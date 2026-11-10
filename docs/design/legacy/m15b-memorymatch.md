# M15b 设计：MemoryMatch 记忆翻牌（game-rpc 9315 / gacha_game）

> 老项目参考：F:\workspace\src\server_v2 app/service/gacha（biz memory_match*.go 共 8 文件 893 行 + data memory_match*.go 共 8 文件 1816 行 + service 11 RPC + 玩家网关 11 HTTP 端点）。
> 新仓落地：game-rpc 9315 / gacha_game，套 M12 自包含玩法模式（config+entries+state+records + 专用 SAGA）。
> 调研报告结论已并入下文；全量语义以老项目源码为准，本仓实现逐条照抄。

## 核心玩法（照抄，常量不可改）

- 牌堆 54 张卡（id 1..54），9 图案 × 每图案 6 张，`pattern = (card-1)/6 + 1`。
- 初始手牌 = 洗牌后前 **9 张**，draw_position=9；手牌恒 ≤9。
- 回合随机源（服务端可审计）：
  - `roundSeed = SHA256("memory-match/v1/round" ‖ engineSeed(32B，购买时 crypto/rand) ‖ BE64(sessionID) ‖ BE32(roundNo))`；
  - `next() = SHA256(roundSeed ‖ BE64(counter))` 前 8 字节大端，counter 递增；uint64n 拒绝采样；
  - 洗牌 Fisher-Yates；`seed_commitment = SHA256(roundSeed)`；
  - EXACT 发牌每次尝试种子 `SHA256("memory-match/v1/deck-attempt" ‖ roundSeed ‖ BE32(attemptIndex))`，默认 32 次上限 64，取 AutoResolve 后 pair_count 距 [exact_min,exact_max] 最近的牌组（距离 0 即用）；
  - 最终 `final_hash = SHA256("memory-match/v1/final" ‖ r1.ResultHash ‖ … ‖ rN.ResultHash)`。
- 回合推进规则：
  - **ApplyPair**：两卡 1..54、互异、同图案、都在手牌 → 移除两张，pair_count++；配对图案==命运 → available_destiny++（第二命运启用且两命运同图案时额外 +1 的资源加成按老项目 `SecondDestinyOnlyBuffsMatchingDestinyPairs`）；draw_position<54 且手牌<9 → 补一张（补入命运图案 → available_destiny++）。actions 记 `pair:a:b`、`replenish:c`。
  - **ApplyDestinyDraw**：pair_count<max、available_destiny>0、堆未空、手牌<9，且**手牌无同图案对子**（有对必须先配，firstPair 按图案 1..9 扫）→ 抽 1 张，available_destiny--，destiny_draws++；抽到命运图案 → available_destiny++。
  - **Terminal**：pair_count≥max_pairs；或无对子且（destiny 用尽 / 堆空 / 手牌=9）。
  - **AutoResolve**（跳过/兜底）：循环「有对配对，否则命运抽牌」到 Terminal。手动进度与跳过续玩共用同一 opening 校验（deck/初始手牌/承诺/draw_position∈[9,54]/手牌≤9/pair≤max/destiny≤81/actions≤108）。
- 会话状态机：PENDING（本仓新增，支付前）→ AWAITING_DESTINY →(select_destiny)→ PLAYING（second 启用则 → AWAITING_SECOND_DESTINY →(select_second_destiny)→ PLAYING）→(advance_round: completed==purchased)→ SETTLING →(finish_settlement)→ COMPLETED；skip 同事务 skip_requested+SETTLING。命运仅一次、current_round==1 时可选；同值幂等重放、异值冲突。
- 上限常量：rounds∈[1,100]、max_pairs∈[1,27]、pattern_id 1..9、preset 1..100、face 恰 9 张、name≤128、URL≤2048、engine_version=1。

## 决策 D1-D12

- **D1 服务归属**：全部落 game-rpc 9315 / gacha_game；不开新服务。引擎纯函数落 `game/rpc/internal/core/memorymatch/`，逻辑落 `game/rpc/internal/logic/`。
- **D2 池→配置**（偏离 1）：老项目 purchase 按 pool_id/pool_version（池版本携带 MemoryMatchRules）；本仓玩法配置自治（M12 满车/扭蛋 config_id 先例），`memory_match_configs` 一行一玩法，purchase 入参 `configId`。外观/规则/奖档全在配置内。
- **D3 11 端点全量 + 1 本仓新增**（接口照抄，路径按本仓 /gacha/api/v1 风格）：

| 方法/路径 | 组 | 说明 |
|---|---|---|
| POST /memory-match/purchase | JWT | 购买（Idempotency-Key 强制）；body{configId, facePresetId?, backPresetId?, userCouponId?, paymentMethod}；返回 {sessionId, purchasedRounds, paidMinor, state, appearance} |
| GET /memory-match/appearance?configId= | JWT | 读外观（用户已存选择或默认） |
| PUT /memory-match/appearance | JWT | 存外观 {configId, facePresetId, backPresetId} |
| GET /memory-match/sessions/:sessionId | JWT | 会话进度（settlement 已发奖则含 totalPairCount） |
| POST /memory-match/sessions/:sessionId/destiny | JWT | 选命运 {patternId} |
| POST /memory-match/sessions/:sessionId/second-destiny | JWT | 选第二命运 {patternId} |
| GET /memory-match/sessions/:sessionId/rounds/:roundNo | JWT | 回合视图（现场派生校验 + 行持久化） |
| POST /memory-match/sessions/:sessionId/rounds/:roundNo/pair | JWT | 配对 {firstCard, secondCard} |
| POST /memory-match/sessions/:sessionId/rounds/:roundNo/destiny-draw | JWT | 命运抽牌 |
| POST /memory-match/sessions/:sessionId/rounds/:roundNo/complete | JWT | 完成回合（Terminal 后；末回合触发终局结算+发奖+擂主） |
| POST /memory-match/sessions/:sessionId/skip | JWT | 跳过（AutoResolve 剩余回合 + 终局结算；已跳过幂等重跑） |
| GET /memory-match/configs/:configId | JWT | **本仓新增第 12 端点**：玩法详情（配置+奖档+擂主+外观；前端购买页数据源，老项目由 purchase 入参池信息承担） |

购买响应同步返回会话（老项目同步购买同款形状）：Submit 后网关按 requestId 轮询
**GetMemoryMatchSessionByRequest**（本仓新增第 18 个内部 RPC，偏离 5：老项目同步购买无此需求）
至会话建立（≤10s；EXPIRED=事务已补偿→报错换键重试）。

- **D4 购买 = 7 分支 SAGA（仅 WALLET）**：01 base.EvaluatePolicy(scene=MEMORY_MATCH) → 02 RiskEvaluate → 03 game.LockMemoryMatch（barrier 事务：配置加载+快照 rules/appearance + 券报价锁（scene=CHECKOUT_SCENE_MEMORY_MATCH_PURCHASE，M10 已预留枚举）+ session 行插入 status=PENDING、uk request_id；Release 补偿置 EXPIRED）→ 04 payment.AuthorizeWalletPayment（payable=0 免单零腿，M11b 先例）→ 05 game.SettleMemoryMatch（PENDING→AWAITING_DESTINY：engine_seed crypto/rand + 聚合字段落库；Revert 回 PENDING）→ 06 game.GrantMemoryMatchPrizes/空补偿（购买时无奖，成功空操作，dig 先例）→ 07 payment.CaptureWalletPayment。幂等重放快路径先于业务解析；WithRetryLimit(10)；Submit 失败 2s GetRecord 兜底。
  - **SAGA 载荷 sessionId 约定**：05 分支提交时 sessionId 未知（会话在 03 才插入）→ 载荷传 0，settle 分支按 uk request_id 推导会话；`in.SessionId>0` 时才做一致性校验。05/06 的参数校验不要求 sessionId>0。
  - **外卡偏离（偏离 2）**：WECHAT/ALIPAY 不支持——本仓 payment-rpc 无外部渠道适配层（老项目外卡依赖 Kafka 支付事件 + 真实商户凭据，dev 环境同样无法完成）；接口保留 payment_method 字段，非 WALLET → **3038**。
  - **TotalPriceFen 校验**：LockMemoryMatch 锁内校验 `quote.gross == 配置购买价 == totalPriceFen`（原价；M13 拼团同款）；券抵扣体现在报价 payable/cash，04/07 按 payable 冻结/捕获。
- **D5 会话/回合/结算表**（nocache，FOR UPDATE + version 乐观锁，冲突 → **3039**）：
  - `memory_match_sessions`：uk request_id（= DTM gid）；engine_seed CHAR(64)（hex，购买时 crypto/rand 32B；偏离 BINARY(32)：hex 与 result_hash 展示一致）、engine_version、purchased_rounds、purchase_price_minor、rules_json（快照）、appearance_json、destiny_pattern_id、second_destiny_enabled、second_destiny_pattern_id、completed_rounds、current_round、skip_requested、final_result_hash CHAR(64)、champion_credit_user_id、champion_share_minor（D9 回扣标记）、status（1 PENDING/2 AWAITING_DESTINY/3 AWAITING_SECOND_DESTINY/4 PLAYING/5 SETTLING/6 COMPLETED/7 EXPIRED）、version。
    - 状态 5 SETTLING 本仓不使用：终局结算在 CompleteRound/Skip 单事务内瞬时完成，落库直接 COMPLETED（D7）。
  - `memory_match_rounds`：uk(session_id,round_no)；state（1 READY/2 PLAYING/3 RESOLVED/4 SKIPPED；READY 未使用——回合在选命运时即开局）、state_json（deck/initial_hand/seed_commitment/draw_position/available_destiny/destiny_draws/pair_count/actions）、pair_count、result_json、result_hash CHAR(64)、version。
  - `memory_match_settlements`：session_id PK；result_hash、reward_json、status 1 待发奖/2 已发奖。
- **D6 引擎纯函数 + 单测**：移植 6 个 biz 测试文件全部用例（OneDestinyAndSequentialPaidRounds、SkipStillRequiresFinalResults、OptionalSecondDestinyOnlyOnce、RoundReplayAndLimits、SkipMatchesSequentialAutoResolution、SecondDestinyOnlyBuffsMatchingDestinyPairs、DeckStrategiesAndChampion、ManualMoveThenSkipResumesSameRound、CannotSettleUnfinishedRound、SelectMemoryMatchAwardByTotalPairCount、AppearancePresetSelectionBindsBackAndBackground、RequireOwnedMemoryMatchAppearanceAllowsDefaults、RejectsUngrantedPreset）。
- **D7 终局结算+同步发奖（偏离 3：去 kernel effect 异步队列）**：CompleteRound/Skip 在**同一本地事务**内完成——校验 1..purchased 全部回合 RESOLVED/SKIPPED 并逐回合 Verify（老项目同款）→ total/final_hash → 插 settlement(status=1 待发奖) → 发奖：
  - 按 total_pair_count **精确匹配** entries 档位（uk(config_id,total_pair_count)，发布/种子时保证档位唯一）；无匹配档 → 无奖（正常完成，老项目同款）；
  - 匹配档 FOR UPDATE 扣 remaining（≥1 否则 **3045**，整事务回滚可重试）；
  - **发奖走 PENDING→补发两级（偏离 3 细化）**：结算事务只落 settlement(status=1)+会话 COMPLETED+final_result_hash；事务提交后同步调 mmRetryPendingSettlementGrant 发奖（现金 CreditWallet(kind1)/积分 kind4/券本地插，grant_no=`"mmr:<sessionID>"` 幂等）并置 status=2；发奖失败（跨库 RPC 不可回滚本地事务）→ settlement 留 PENDING，CompleteRound/Skip 重放快路径补发（幂等重放快路径先于业务解析）。
- **D8 外观 + 权益**：config 内 face_presets/back_presets JSON（preset 校验同老项目）。非默认 **face** preset 需拥有权益——新表 `memory_match_appearance_entitlements`（uk(config_id,user_id,preset_id)），存在即拥有（**偏离 4**：老项目走 campaign 通用权益状态机，本仓权益发放链路不在本里程碑，表行由 SQL/未来活动插入；无行 → **3044**）。默认 preset 恒可用；**back preset 无权益门**（老项目 RequireOwnedMemoryMatchAppearanceAllowsDefaults 语义：仅 face 需权益，back 自由选择）。UPSERT `memory_match_appearance_selections`。
- **D9 擂主 champion**：champion_min_total_pairs>0 且 share_bps∈(0,10000]（配置校验）；每次购买价×bps/10000 计入现任 pending_income（`memory_match_champion` uk config_id）；total≥min 且（无擂主或严格大于现任）→ 换擂：异用户 → 现任 pending 全额 CreditWallet（grant_no=`"mm-champion:<displacedSessionID>"` 幂等）；同用户只更新。`memory_match_champion_events` uk session_id 防重。
  - **打款账本 `memory_match_champion_payouts`**（session_id PK, user_id, amount_minor, grant_no, status 1 PENDING/2 PAID）：换擂打款先插 PENDING 行、事务后 mmRetryChampionPayouts 补发（与 D7 同款两级）；打款失败留 PENDING 重试，绝不丢款。
  - **购买分成回扣竞态守卫**：session 记 champion_credit_user_id+champion_share_minor；Revert 回扣仅当现任擂主仍是记账用户才扣（max(0,pending-share) 兜底）；擂主已易主 → 接受泄漏，绝不偷扣新擂主。
- **D10 errorx**（game 域，05-errors.md 登记）：`3038 支付方式暂不支持（仅 WALLET）` `3039 会话状态冲突（命运已选异值/回合不匹配/版本冲突）` `3040 回合未完成（complete 时未 Terminal）` `3041 回合已结束（pair/destiny-draw 时已 Terminal）` `3042 命运抽牌不可用（有对子未配/资源耗尽/堆空/手牌满）` `3043 第二命运未启用` `3044 外观未拥有` `3045 奖档售罄（结算发奖库存不足）`；会话/回合不存在 → 404（本仓 gorm.ErrRecordNotFound 惯例）；入参校验 → 400。
- **D11 场景 30**（test/scenario_30_memory_match.sh + run_all/menu 注册，26→27 场景）：注册 A/B 充值 → A 购买（3 轮×max 3 对，100 分；WECHAT → 3038；同 key 重放 → 同 session）→ 命运（异值 → 3039）→ 手打回合（错对 → 400、已终局 pair → 3041、有对时 destiny-draw → 3042、未终局 complete → 3040）→ 3 轮打完终局（total 断言 = 手打确定值、奖励发放断言：钱包/券 mysql 直查、settlement status=2、重放 complete 幂等）→ B 购买 + skip（AutoResolve、final_hash 返回、奖励+擂主：B total≥5 换擂 A 获 pending 打款断言）→ 外观（默认 OK、非默认无权益 3044 → SQL 插权益 → OK）→ 现场复位。
- **D12 前端**：pages/MemoryMatch/index.vue（购买页：价格/外观选择/券/购买 → 牌局页：手牌 9 张图案展示、命运选择、点两张配对、命运抽牌按钮、可用命运数/配对数进度、回合完成/跳过按钮、结算弹窗、擂主展示）+ services/memoryMatch.ts（11 API）+ 路由 + MainLayout 菜单 + i18n 双语全量。

## DDL — 22_gacha_game_m15b.sql（幂等，init_all.sql 聚合）

- `memory_match_configs`（cache）：id、name、rounds_per_purchase、max_pair_count、purchase_price_minor、wallet_kind、second_destiny_enabled、deck_strategy、pattern_weights_json、exact_min_pair_count、exact_max_pair_count、exact_attempt_limit、face_presets_json、back_presets_json、default_face_preset_id、default_back_preset_id、champion_min_total_pairs、champion_share_bps、status。种子 1 号「记忆翻牌·命运牌局」：3 轮×3 对、100 分、kind1、second 开、DYNAMIC 权重 9 图案、champion min=5 bps=500。
- `memory_match_entries`（cache）：config_id、name、total_pair_count、weight、prize_kind、cash_fen、points_amount、coupon_template_no、quantity、remaining、is_prize；uk(config_id,total_pair_count)。种子：9 对→限定皮肤（现金 5000 分）、6 对→头像框（积分 500）、3 对→金币（现金 100）、0 对→谢谢参与（is_prize=0）。
- `memory_match_sessions` / `memory_match_rounds` / `memory_match_settlements` / `memory_match_appearance_selections` / `memory_match_appearance_entitlements` / `memory_match_champion` / `memory_match_champion_events` / `memory_match_champion_payouts`（nocache，形状见 D5/D8/D9）。
- 模型脚本：game_cache.sh += memory_match_configs memory_match_entries；game_nocache.sh += 其余 7 表；接入 serviceContext.go。

## proto 扩展（`bash script/goctl.sh game`，禁跑 gateway）

game.proto 新增枚举 MemoryMatchSessionState/MemoryMatchRoundState/DeckStrategy + 消息 + 18 RPC：
- SAGA：LockMemoryMatch / LockMemoryMatchRevert（release=补偿）、SettleMemoryMatch / SettleMemoryMatchRevert、GrantMemoryMatchPrizes / GrantMemoryMatchPrizesRevert（空补偿）
- 玩法：GetMemoryMatchConfig（含外观 presets + 擂主）、SetMemoryMatchAppearance、GetMemoryMatchAppearance、SelectMemoryMatchDestiny、SelectMemoryMatchSecondDestiny、GetMemoryMatchSession、GetMemoryMatchSessionByRequest（偏离 5，网关购买轮询）、GetMemoryMatchRound、ApplyMemoryMatchPair、ApplyMemoryMatchDestinyDraw、CompleteMemoryMatchRound、SkipMemoryMatch
- 响应复用 PrizeItem 表达奖档/结算奖。

## 网关端点（/gacha/api/v1，手加 routes/types/handler/logic，禁跑 goctl gateway）

见 D3 表；SAGA 编排 logic 落 app/gateway/internal/logic/memorymatch/memoryMatchBuyLogic.go（7 分支骨架同 M12 fullCarBuy）。

## 测试

- 单测：`go test ./app/game/rpc/internal/core/memorymatch/...`（D6 全用例移植）。
- 场景 30（D11）+ run_all/menu 注册。
- 边界门：run_all 27 场景 ×2 全绿、go build ./...、go test 核心包、npm run build ×2。

## 偏离记录汇总（m15b）

1. **D2**：pool_id/pool_version → config_id（玩法配置自治，M12 先例）。
2. **D4**：外卡 WECHAT/ALIPAY 不实现（3038）——本仓 payment-rpc 无外部渠道层；老项目 dev 同样不可用；接口保留字段。
3. **D7**：kernel effect 异步队列 → 结算同事务落 PENDING、事务后同步补发（mmRetryPendingSettlementGrant 幂等；业务终点一致：玩家拿到奖品；无 Kafka）。
4. **D8**：外观权益发放链路（campaign 发 PAIR_CARD_APPEARANCE）不在本里程碑；权益表由 SQL/未来活动插入。仅 face preset 需权益（back 自由选择，老项目语义）。
5. **D3**：购买异步 SAGA → 新增内部 RPC GetMemoryMatchSessionByRequest（网关按 requestId 轮询会话建立，购买响应保持老项目同步形状）；另新增 GET /memory-match/configs/:configId 第 12 端点（前端配置数据源）。

## 实施顺序

1. 22_gacha_game_m15b.sql + 模型脚本更新+跑 → serviceContext 接入。
2. errorx 3038-3046 + 05-errors.md 登记。
3. game.proto +18 RPC → `bash script/goctl.sh game`。
4. core/memorymatch 引擎移植 + 单测全绿。
5. 18 logic（lock/settle/grant/revert + 12 玩法 RPC）。
6. 网关 11 端点 + memoryMatchBuy SAGA。
7. 前端 MemoryMatch 页 + services + i18n。
8. 场景 30 → run_all 27 场景 ×2 全绿。
9. 边界门（build/test/npm ×2）+ 提交（Co-Authored-By，排除清单）+ memory 更新。
