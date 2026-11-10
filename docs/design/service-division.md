# 服务拆分重构蓝图（gacha_srv 从 0 重建）

> 决策日期：2026-11-09。用户拍板：**A 平衡版拆分** + **网关维持 SAGA 编排、统一 builder**，随后三轮收敛合并小服务：
> ① risk+eligibility 保持旧仓原样（base-rpc 整体搬迁，不拆）；② promotion+campaign → **marketing-rpc**；③ asset+payment → **money-rpc**。
> 最终：**10 RPC + 2 网关 + DTM**。
> 本文件是整个重构的单一事实源。旧仓 = `C:\Users\Administrator\Desktop\Gacha`（module gacha），新仓 = `C:\Users\Administrator\Desktop\gacha_srv`（module gacha_srv）。
> 原则：**接口不能变**（HTTP 路径、RPC 形状、数据库表结构、前端行为全部不变）；只重排服务归属与目录结构。
> 历史里程碑文档（旧拓扑名）保留在 `docs/design/legacy/`（m14*~m16*、service-mapping），仅供追溯，不作为当前事实。

## 0. 旧仓问题结论（评估摘要）

1. **game-rpc 巨兽**：89 RPC / 20,759 行业务代码（≈全仓 RPC 服务代码 53%），把老项目 promotion/gacha/campaign/task 四个独立服务塞进一个进程，"表都在 gacha_game 一库"是因果倒置（先建了一库，再用"数据跟着域走"固化错误划分）。
2. **网关业务化**：7 个购买 SAGA 组装 + 幂等回查全在玩家网关（4,700 行 logic），依赖 5 个下游 client。
3. **域合并过度**：user-rpc 三合一（identity+wallet+inventory）；base-rpc 二合一（eligibility 营销资格 + risk 安全风控，名字无域含义）；ops-rpc 报表跨库破例。

## 1. 新拆分总表（10 RPC + 2 网关 + DTM）

| 服务 | 端口 | 数据库 | 错误码段 | RPC 数 | 来源（旧仓） | 目录 |
|---|---|---|---|---|---|---|
| gateway（玩家网关） | 9310 | 无 | — | HTTP 路由 46+ | gateway（logic 收敛为统一 saga builder） | app/gateway |
| identity-rpc | 9311 | gacha_identity | 1000-1999 | 13 | user 拆（账号/会话/手机/三方绑定） | app/identity |
| money-rpc | 9312 | gacha_money | 2000-2999 | 18 | asset（user 拆，10）+ payment（8）合并 | app/money |
| draw-rpc | 9313 | gacha_draw | 3000-3999 | 15 | draw 整体（开奖+盒/池+引擎内核） | app/draw |
| base-rpc | 9314 | gacha_base | 4000-4999 | 12 | base 整体搬迁（资格+风控，不拆） | app/base |
| marketing-rpc | 9315 | gacha_marketing | 5000-5999 | 21 | game 拆（promotion 13 + campaign 8 合并） | app/marketing |
| play-rpc | 9316 | gacha_play | 6000-6999 | 61 | game 拆（6 玩法+幸运指派+发奖标记） | app/play |
| task-rpc | 9317 | gacha_task | 7000-7999 | 7 | game 拆（任务/签到/通行证/会员） | app/task |
| catalog-rpc | 9318 | gacha_catalog | 8000-8999 | 9 | catalog 整体 | app/catalog |
| thirdparty-rpc | 9319 | gacha_thirdparty | 9000-9999 | 11 | thirdparty 整体 | app/thirdparty |
| ops-rpc | 9320 | gacha_ops | 10000-10999 | 33 | ops 整体（app_admin/ops，admin-gateway 专用） | app/ops |
| admin-gateway | 9321 | 无 | — | HTTP 路由 | admin gateway 整体 | app/admin-gateway |
| dtm | 36789/36790 | gacha_dtm | — | — | dtm 整体 | app/dtm |

## 2. proto 切分映射（RPC 逐条归属；10 个 proto 统一在 pkg/proto）

### 2.1 user.proto（23 RPC）→ identity.proto（13）+ money.proto 资产段（10）

**identity.proto**（service Identity）：Register、Login、VerifyToken、GetProfile、RequestPhoneLoginCode、LoginWithPhoneCode、LoginWithWeChatCode、SetUserPhone、FindUserPhoneExact、SearchUserPhones、LinkThirdPartyIdentity、ResolveUserByThirdParty、GetThirdPartyIdentity

**money.proto 资产段**（service Money）：GetBalance、Recharge、CreditWallet、FreezeWallet、PostHold、VoidHold、GetWallet、GrantAsset、GrantAssetRevert、CheckGrantsByRequestId

### 2.2 payment.proto（8 RPC）→ money.proto 支付段

AuthorizeWalletPayment、CancelPayment、CaptureWalletPayment、CaptureWalletPaymentRevert、Refund、GetPayment、GetPaymentRoute、PutPaymentRoute

### 2.3 game.proto（89 RPC）→ marketing.proto（21）+ play.proto（61）+ task.proto（7）

**marketing.proto**（service Marketing = promotion 13 + campaign 8）：
QuoteCheckout、LockQuote、ReleaseQuote、CommitQuote、ClaimCoupon、ListUserCoupons、ListCouponTemplates、PublishCouponTemplate、GetCouponTemplate、ListCouponTemplateVersions、GrantCoupons、GetTenantLoyaltyDefaults、PutTenantLoyaltyDefaults；
ListCampaigns、CampaignPreview、CampaignParticipate、ListCampaignBoxEntitlements、GetCampaignBoxEntitlement、CreateCampaignDraft、PublishCampaign、GetCampaign

**play.proto**（service Play，61 RPC）：
- 挖宝（9）：ListDigBoxes、GetDigBoxSession、LockDigBox、ReleaseDigBox、SettleDigBox、SettleDigBoxRevert、GrantDigPrizes、GrantDigPrizesRevert、ListDigRecords
- 满车（8）：GetFullCar、LockFullCar、ReleaseFullCar、SettleFullCar、SettleFullCarRevert、GrantFullCarPrizes、GrantFullCarPrizesRevert、ListFullCarRecords
- 扭蛋（9）：GetGashapon、LockGashapon、ReleaseGashapon、SettleGashapon、SettleGashaponRevert、GrantGashaponPrizes、GrantGashaponPrizesRevert、ListGashaponRecords、AssignGashaponLuckyUser
- 连击链（8）：GetComboChain、LockComboChain、ReleaseComboChain、SettleComboChain、SettleComboChainRevert、GrantComboChainPrizes、GrantComboChainPrizesRevert、ListComboChainRecords
- 拼团（8）：GetGroupBuy、LockGroupBuy、ReleaseGroupBuy、SettleGroupBuy、SettleGroupBuyRevert、GrantGroupBuyPrizes、GrantGroupBuyPrizesRevert、ListGroupBuyRecords
- 记忆翻牌（19）：GetMemoryMatchConfig、GetMemoryMatchAppearance、SetMemoryMatchAppearance、SelectMemoryMatchDestiny、SelectMemoryMatchSecondDestiny、GetMemoryMatchSession、GetMemoryMatchSessionByRequest、GetMemoryMatchRound、ApplyMemoryMatchPair、ApplyMemoryMatchDestinyDraw、CompleteMemoryMatchRound、SkipMemoryMatch、LockMemoryMatch、LockMemoryMatchRevert、SettleMemoryMatch、SettleMemoryMatchRevert、GrantMemoryMatchPrizes、GrantMemoryMatchPrizesRevert、CheckPrizesGranted

**task.proto**（service Task）：
ListTasks、SignInTask、ClaimTask、ClaimPassReward、ClaimMemberReward、ApplyTaskProgress、UnlockPass

### 2.4 整体搬迁（service 名不变）：draw.proto、catalog.proto、thirdparty.proto

### 2.5 ops.proto：旧仓 pkg/proto_admin/ops.proto 原样并入 pkg/proto/ops.proto（33 RPC；旧仓仅此一个 ops 服务 app_admin/ops → 新仓 app/ops）

> 新仓目录布局为**单树**：全部服务与两个网关都在 `app/` 下（app/{identity,money,draw,base,marketing,play,task,catalog,thirdparty,ops,gateway,admin-gateway,dtm}），旧仓 app/ 与 app_admin/ 两棵树不再保留。

## 3. 数据库拆表映射（表名 → 新库）

### 3.1 gacha_user（12 表）→ gacha_identity（5）+ gacha_money（9）

- **gacha_identity**：user_users、user_phones、user_phone_grams、user_third_parties、dtm_barrier
- **gacha_money**：wallet_accounts、wallet_holds、wallet_ledgers、user_balances、user_balance_logs、assets、asset_reservations、pickup_codes、dtm_barrier

### 3.2 gacha_payment（7 表）→ gacha_money（整体并入）

payment_orders、payment_legs、payment_refunds、payment_route_policies、payment_route_candidates、payment_route_audits、dtm_barrier（并入后与 3.1 的 barrier 去重为一张）

### 3.3 gacha_base（8 表）→ gacha_base（不拆，整体搬迁）

eligibility_policies、eligibility_policy_rules、eligibility_policy_versions、eligibility_decision_logs、risk_rules、risk_cases、risk_case_events、dtm_barrier

### 3.4 gacha_game（57 表）→ gacha_marketing（16）+ gacha_task（9）+ gacha_play（32）

- **gacha_marketing**：campaigns、campaign_versions、campaign_box_entitlements、campaign_participations、campaign_prize_grants、campaign_redemption_codes、campaign_redemption_users、campaign_sign_ins、campaign_wheel_daily、campaign_wheel_slots、coupon_template_heads、coupon_template_versions、coupon_templates、loyalty_offset_config、quote_locks、user_coupons、dtm_barrier
- **gacha_task**：task_claim、task_event、task_member、task_member_claim、task_pass_claim、task_pass_progress、task_progress、task_sign_in、dtm_barrier
- **gacha_play**：dig_box_cells、dig_box_configs、dig_box_records、dig_box_sessions、full_car_configs、full_car_entries、full_car_records、full_car_state、gashapon_configs、gashapon_entries、gashapon_lucky_assignments、gashapon_records、gashapon_state、combo_chain_configs、combo_chain_records、combo_chain_state、combo_chain_streaks、group_buy_configs、group_buy_entries、group_buy_records、group_buy_state、memory_match_configs、memory_match_entries、memory_match_sessions、memory_match_rounds、memory_match_settlements、memory_match_appearance_entitlements、memory_match_appearance_selections、memory_match_champion、memory_match_champion_events、memory_match_champion_payouts、prize_grant_marks、dtm_barrier

### 3.5 不拆库（整体迁移）：gacha_draw、gacha_catalog、gacha_thirdparty、gacha_ops、gacha_dtm

### 3.6 跨域数据适配点（一服务一库，玩法域触碰券表改走 RPC）

- **注册建余额行（identity→money 跨库写）→ 懒初始化**：旧仓 user 的 registerLogic 在注册事务里同库 INSERT user_balances 初始 0 行；拆库后 identity 不能再写 gacha_money。改为 money 侧读写路径 upsert 兜底（getBalance/recharge/creditWallet 对 uk_user_id 冲突时忽略/按用户建行），registerLogic 只建账号。已实施：identity registerLogic 移除余额行写入；money 侧待实施。
- user_coupons / coupon_templates 归 gacha_marketing；旧仓 play 域逻辑直连这两张表的位置（grantDigPrizesLogic、releaseDigBox/releaseFullCar/releaseGashapon/releaseGroupBuy/releaseComboChain、memorymatchOps）迁移时改为调 marketing 内部 RPC：发券走 GrantCoupons，释放退还新增内部 RPC（表结构不变，调用链重排）。
- task_plan_head / task_plan_version 归 gacha_ops；旧仓 game-rpc 跨库直读（M15c task 计划快照，OpsMySQL 配置）违反一服务一库，task-rpc 改为经 ops-rpc 新增内部 RPC 读取（task-rpc.yaml 已预置 OpsRpc 客户端）。

### 3.7 配置统一（pkg/etcs）

- 12 个 yaml：10 个 `<svc>-rpc.yaml`（identity 9311 / money 9312 / draw 9313 / base 9314 / marketing 9315 / play 9316 / task 9317 / catalog 9318 / thirdparty 9319 / ops 9320）+ `gateway-api.yaml`（9310）+ `admin-gateway-api.yaml`（9321）。
- 生成骨架 main 的默认配置路径手改为 `-f ../../pkg/etcs/<svc>-rpc.yaml`（goctl 模板默认 etc/<svc>.yaml，重新生成后需再次修正）。
- DTM 直连地址只保留 SAGA 分支目标：base/marketing/money/draw/play。
- DevServer 端口按服务序 9390-9401。

## 4. SAGA 编排（网关统一 builder）

7 个购买 SAGA 分支目标服务变更（service 名重映射，RPC 名与请求形状不变）：

| 分支 | 旧 | 新 |
|---|---|---|
| 01 资格 | base.Base/EvaluatePolicy | 不变 |
| 02 风控 | base.Base/RiskEvaluate | 不变 |
| 03 锁报价 | game.Game/LockQuote | marketing.Marketing/LockQuote |
| 04 建单冻结 | payment.Payment/AuthorizeWalletPayment | money.Money/AuthorizeWalletPayment |
| 05 开奖/结算 | draw.Draw/SettleDraw 或 game.Game/Settle<玩法> | draw 不变；玩法→ play.Play/Settle<玩法> |
| 06 发奖 | user.User/GrantAsset | money.Money/GrantAsset |
| 07 捕获 | payment.Payment/CaptureWalletPayment | money.Money/CaptureWalletPayment |

补偿链 [空 → 空 → ReleaseQuote → CancelPayment → SettleRevert → GrantAssetRevert → 空]：04/06/07 三分支同服务（money），支付↔钱包的旧跨服务调用变本地事务；释放玩法/退款等涉及券的流程，play 侧释放步骤改经网关 SAGA 调 marketing。

**报价锁跨服务适配（玩法 SAGA，2026-11-10 落地）**：旧仓玩法分支 03 在自身事务内直锁 quote_locks 表（自中止随事务回滚，报价保持 OPEN）；拆服务后改经营销 RPC sub-call（营销事务独立提交、无法随玩法事务回滚）。适配铁律两条：
1. **锁后置**：play.Lock<玩法> 先经 marketing.GetQuote 只读取 userId/grossFen/boxId 完成全部玩法校验，**最后一步**才调 marketing.LockQuote——玩法自身校验失败回滚时从未触碰营销锁，自中止后报价保持 OPEN，与旧仓事务内回滚的最终状态一致。
2. **barrier 可选**：marketing.LockQuote/ReleaseQuote 双调用形态——抽盒 SAGA 由 DTM 直调（有 trans info 走 barrier）；play 分支内 sub-call 无 trans info，`dtmgrpc.BarrierFromGrpcOrNil` 返回 nil 时退化为普通事务（幂等由 FOR UPDATE + 状态校验兜底，businessRan 恒 true）。

- 网关内新建 `app/gateway/internal/saga/builder.go`：通用 SAGA 组装器（分支列表 + 重试 + 幂等回查模板），7 个流程共用，消除现有 sagaOps.go ×7 的拷贝粘贴。
- 直连地址配置：`DtmServer.Target` 及各 RPC 直连（BaseRpcDirect 不变、PromotionRpcDirect → MarketingRpcDirect、AssetRpcDirect → MoneyRpcDirect 等）在 etcs 里重命名。

## 5. 迁移执行清单（阶段状态跟踪）

| # | 阶段 | 内容 | 状态 |
|---|---|---|---|
| 1 | 骨架 | go.mod（module gacha_srv）、CLAUDE.md、目录 | ✅ |
| 2 | proto 重切 | 10 个 proto（identity/money/draw/base/marketing/play/task/catalog/thirdparty/ops）protoc 校验通过 | ✅ |
| 3 | SQL 拆库 | script/split_sql.go 按 §3 重切 + 01_databases.sql（11 库+全局屏障库） | ✅ |
| 4 | 模型迁移 | pkg/model、model_nocache 按 10 业务库重组 + 生成脚本适配 | ✅ |
| 5 | 脚手架 | goctl.sh 适配新服务名，生成 10 RPC 骨架 + 2 网关 | ✅ |
| 6 | pkg 基础 | utils/errorx（10 错误码段重分配）、jwtutil、money、txutil、interceptor | ✅ |
| 7 | 业务迁移 | 逐服务迁移 logic：identity、money（asset+payment）、draw、base、marketing（promotion+campaign）、play、task、catalog、thirdparty、ops | ✅ |
| 8 | 网关 | 玩家网关 routes.go + 统一 saga builder；管理网关 | ✅ |
| 9 | 前端 | web_admin、mini 整体搬迁 | ✅（2026-11-10 入仓；适配点仅管理网关 dev 代理端口 9319→9321，请求路径 /console/v2/api、/gacha/api/v1 零改动；web_admin build+lint、mini type-check+build:h5+build:mp-weixin 全过。lint 首次真跑暴露两处配置缺陷并修复：eslint globalIgnores/.oxlintrc.json ignorePatterns 未忽略 node_modules（旧仓从未 npm install 故未暴露）、no-unused-vars 补 create-vue 的 `^_` 前缀豁免约定） |
| 10 | 测试 | 单测迁移 + test/run_all.sh 32 场景适配新端口/服务 | ✅（2026-11-10 回归 29/29 全过；慢场景 5/6/10 单独执行全过——场景 5 千抽概率核对；场景 6 SettleDraw 阻塞 72s、场景 10 CaptureWalletPayment 两段锁接力 130s，均验证 Internal→DTM 退避重试→幂等收敛，扣款/捕获恰一次） |
| 11 | 全量验证 | go build ./...、go test ./...、e2e 全过、前端 build、lint | ✅（2026-11-10 完成：go build/vet/test 全绿；e2e 29/29 + 慢场景 3/3；web_admin build+lint、mini type-check+build:h5+build:mp-weixin 全过） |

## 6. 迁移技术要点

- **import 前缀**：全局替换 `"gacha/` → `"gacha_srv/`（Go 文件）；module gacha → gacha_srv。
- **服务 client 目录**：app/<svc>/<svc>Client 改名规则——旧 gameClient → marketingClient/playClient/taskClient、userClient → identityClient/moneyClient、paymentClient → moneyClient（迁移时按 logic 实际调用逐个改）。
- **错误码常量**：errorx 里 CodeXxx 值按新段重排（identity 1000、money 2000、draw 3000、base 4000、marketing 5000、play 6000、task 7000、catalog 8000、thirdparty 9000、ops 10000）；常量名可保持语义（如 CodeUserNotFound 留在 identity 段；payment 旧段 8xxx 并入 money 段后按域内顺序重排）。
- **HTTP 接口不变**：玩家网关路径 /gacha/api/v1/* 与管理网关 /admin/api/v1/* 全部保持，前端零改动。
- **Etcd key**：identity.rpc、money.rpc、draw.rpc、base.rpc、marketing.rpc、play.rpc、task.rpc、catalog.rpc、thirdparty.rpc、ops.rpc。
- **dtm_barrier**：每个业务库保留 dtm_barrier 表（拆库后 10 个业务库各一张），服务 main 中 SetBarrierTableName。
- **验证标准**：`go build ./...`、`go test ./...`、`bash test/run_all.sh` 32 场景全过、`npm run build`（web_admin/mini）。
