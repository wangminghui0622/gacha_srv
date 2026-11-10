# M14 管理后台服务划分计划

> 目标：管理控制台功能与 server_v2 完全一致，接口（路径/JSON 格式/JWT/错误体）完全一致，
> 服务划分按本仓现状调整为最合理状态（用户裁决："他怎么做的，我们就怎么做，业务功能不要动，
> 服务可以调整，接口不能变"）。
> 配套文档：docs/design/m14-admin-console.md（D1-D10 设计裁决，网关/身份/授权已按此落地）。

## 0. 划分原则

1. **服务数量不增不减**：8 个业务 rpc（user/draw/payment/base/game/catalog/thirdparty/ops）
   + 2 个网关（player 9310 / admin 9319）+ 1 个 DTM 实例（36789/36790）不变；
   控制台功能"就近归并"到既有服务，只追加"控制台 RPC"与表，玩家面接口与玩家链路零改动。
2. **三层结构**：
   - 边缘层 `admin-gateway :9319`：路由/中间件/fail-closed 校验/字段投影/审计触发，无业务。
   - 平台层 `ops-rpc :9318`（gacha_ops）：控制台账号/会话/TOTP/授权/审计（M14a 已交付）+
     **任务计划/调度 + 报表聚合**（server_v2 的 task、reporting 两个服务归并于此——
     两者都是"管理平台能力"，不是业务域能力）。
   - 业务层：各域 rpc 持有业务数据，追加控制台 CRUD/草稿/发布/复核 RPC。
3. **数据跟着域走**：业务配置表留在各自域库（campaign/券在 gacha_game、资格/风控在
   gacha_base 等，现状已如此），不搬迁；只有管理平台自身的表进 gacha_ops。
4. **审批→发布在域库单事务完成，不走 SAGA**：已查证 server_v2 源码——通用审批表
   （authorization_approval_request/action）是未使用遗留表；赏池发布审批在**域库**
   （gacha_pool_publication_approval），PublishPool 在一个 MySQL 事务内完成
   「版本行校验 + 审批行校验 + 物化线上表 + 标记 PUBLISHED」，无跨服务写，
   因此不需要管理侧 SAGA、不需要第二个 DTM 实例（§3 撤销说明）。

## 1. server_v2 控制台功能组 → 本仓归属总表

| server_v2 功能组 | server_v2 服务 | 本仓归属 | 状态 |
|---|---|---|---|
| 控制台账号/会话/TOTP 两步登录 | authorization 域 | ops-rpc :9318（gacha_ops） | ✅ M14a 已交付 |
| 授权（角色/主体/规则/策略 CEL/数据范围/字段投影/决策日志） | authorization | ops-rpc :9318 | ✅ M14a 已交付 |
| 操作审计 | authorization | ops-rpc :9318（ops_audit_logs） | ✅ M14a 已交付 |
| 通用审批（approval_request/action 通用表） | authorization | **不实现**——server_v2 遗留未使用表；发布审批在域库（见 §5） | ✅ 已查证撤销 |
| 宝箱/奖池/版本/滚动规则控制台 | draw/box 域 | draw-rpc :9312（gacha_draw，池草稿/版本/发布审批四表） | ✅ M14b 已交付 |
| 幸运指派控制台 | draw/box 域 | game-rpc :9315——M12 D4 预留的 AssignGashaponLuckyUser RPC 经管理网关开放 HTTP | ✅ M14b 已挂路由 |
| SPU/SKU/修订版/快照（商品目录） | catalog | catalog-rpc :9316（gacha_catalog）——占位 stub 激活，启用 4000 错误码段 | M14c 激活 |
| 活动定义（签到/兑换码/转盘/活动宝箱） | campaign | game-rpc :9315（gacha_game，campaigns 等表 M11 已在） | M14d 新增 |
| 优惠券/忠诚度（promotion） | promotion | game-rpc :9315（coupon_templates/loyalty_offset_config 在 gacha_game） | M14d 新增 |
| 资格策略（草稿/发布/版本） | eligibility | base-rpc :9314（eligibility_policy_* 在 gacha_base） | M14d 新增 |
| 风控规则/案例复核 | risk | base-rpc :9314（risk_rules/risk_cases 在 gacha_base；Claim/Resolve RPC 已有） | M14d 新增 |
| 支付路由配置 | payment | payment-rpc :9313（gacha_payment，加 payment_routes 表+CRUD） | M14d 新增 |
| 用户管理（查询/详情/资产只读） | user | user-rpc :9311（gacha_user，加只读控制台 RPC） | M14d 新增 |
| 任务计划（cron 调度/执行日志） | task | ops-rpc :9318 新增 task_plans/task_runs 表 + 调度器（理由见 §4） | M14e 新增 |
| 报表/分析（跨域聚合） | reporting | ops-rpc :9318 只读多库连接聚合（理由见 §4） | M14e 新增 |
| 管理侧全局事务 | （dtm） | **不需要**（审批→发布为域库单事务，已查证，见 §3） | ✅ 已撤销 |

**归并理由（服务调整说明）**：
- server_v2 的 authorization 服务同时管控制台身份、授权、审计——本仓 ops-rpc 本来
  就是它的 go-zero 对应物，同构，不是新归并。
- campaign + promotion 归 game-rpc：两类表已在 gacha_game（M11 建 campaign 六表、
  08 建 coupon_templates/loyalty_offset_config），拆服务要跨库搬表，违背"数据跟着域走"。
- eligibility + risk 归 base-rpc：两类表已在 gacha_base（07），运行时 RPC（EvaluatePolicy/
  RiskEvaluate/ClaimRiskCase/ResolveRiskCase）已在 base.proto，控制台 CRUD 自然同服务。
- task + reporting 归 ops-rpc：见 §4 论证。

## 2. 各服务控制台 RPC 增量清单

### ops-rpc :9318（gacha_ops）——平台层
已有 17 个 RPC（M14a：登录两步 TOTP/刷新/登出/权限/账号/重置 TOTP/Authorize/角色 CRUD/
主体角色/审计）。新增：
- **任务计划**：CreateTaskPlan / UpdateTaskPlan / ListTaskPlans / RunTaskNow /
  ListTaskRuns（§4）
- **报表**：按报表类型一个 RPC（如 DrawSummaryReport / PaymentSummaryReport / …，只读聚合）
- 表：task_plans / task_runs（新增 18_gacha_ops_platform.sql，幂等）
- ~~审批引擎（通用 maker-checker + approval_requests/approval_actions）~~：撤销——
  server_v2 通用审批表未使用，发布审批在域库（§5），M14b 已按域库单审批落地。

### draw-rpc :9312（gacha_draw）——M14b ✅ 已交付
表：draw_pool_heads（pool_id 分配器，AUTO_INCREMENT 起 1000000）/ draw_pool_versions
（DRAFT→PUBLISHED 版本链，config_json+config_hash）/ draw_pool_draft_requests
（Idempotency-Key 幂等，同键异载荷冲突）/ draw_pool_publication_approvals
（单审批：PENDING→APPROVED，审批人≠申请人）。
RPC：CreatePoolDraft / ListPoolVersions / GetPoolVersion / RequestPoolPublication /
ApprovePoolPublication / PublishPool / ListPublishedPools。
发布 = 域库单事务物化 draw_boxes（upsert 保 created_at）+ draw_pool_entries
（整体替换）+ CAS 标记 PUBLISHED；已发布同哈希幂等返回，无 SAGA 分支。

### catalog-rpc :9316（gacha_catalog）——M14c 激活
新增表：catalog_spus / catalog_skus / catalog_revisions / catalog_snapshots。
新增 RPC：SPU/SKU CRUD、SaveRevision、PublishSnapshot/PublishSnapshotRevert。
错误码启用 4000 段（proto 占位注释已预留）。发布语义按 server_v2 catalog 查证后定
（若同款为域库单事务则同样不开 SAGA）。

### game-rpc :9315（gacha_game）——M14d
已有 campaign/coupon/box 玩法运行 RPC（玩家面不动）。新增控制台 RPC：
- campaign 定义 CRUD + 发布（campaigns/campaign_sign_ins/campaign_redemption_codes/
  campaign_wheel_slots 等已有表的草稿→发布）
- 券模板 CRUD（coupon_templates）、忠诚度配置（loyalty_offset_config）
- 幸运指派：SetGashaponLuckyAssignments / ListGashaponLuckyAssignments
  （M12 D4 已实现 AssignGashaponLuckyUser，补列表/批量）

### base-rpc :9314（gacha_base）——M14d
已有表 eligibility_policy_drafts/policies/rules/decision_logs、risk_rules/risk_cases。
新增 RPC：资格策略草稿 CRUD / 发布 / 版本列表；风控规则 CRUD；
风险案例列表（Claim/Resolve 已有）。

### payment-rpc :9313（gacha_payment）——M14d
新增表 payment_routes（渠道路由配置）+ CRUD RPC（配置类，按 server_v2 控制台语义定）。

### user-rpc :9311（gacha_user）——M14d
新增只读 RPC：SearchUsers / GetUserDetail（含余额/资产/风险案例只读视图）。

### thirdparty、dtm
无控制台功能。

## 3. 已撤销决策：管理侧 DTM（gacha_dtm_admin + 第二实例）

**原草案**假设"审批→发布"是跨服务全局事务（ops-rpc 审批引擎 + 域库发布，两库两服务），
故计划独立 store `gacha_dtm_admin` + 第二个 dtmsvr 实例（46789/46790）。

**查证 server_v2 源码后撤销**：
- authorization_approval_request / authorization_approval_action 是**未使用的遗留表**，
  池发布审批不在其中；
- 池发布审批在**域库** gacha_pool_publication_approval（本仓 draw_pool_publication_approvals）；
- PublishPool 在**一个 MySQL 事务**内完成：锁版本行（DRAFT+哈希校验）→ 锁审批行
  （APPROVED+审批人≠申请人+哈希校验）→ 物化线上表 → CAS 标记 PUBLISHED。
  所有写都在 gacha_draw 单库，**没有跨服务写，不需要 SAGA**；
- 单审批（审批人≠申请人），不是双人审批。

**结论**：管理侧不新增 DTM 库/实例；17 号 SQL 编号让位给 `17_gacha_draw_m14b.sql`
（池草稿四表）；DTM 部署（36789/36790 单实例）保持不变。

## 4. task 与 reporting 归 ops-rpc 的论证

- **任务计划（task）**：任务本质是"运营动作"——按 cron 调度触发业务动作
  （发券、重置签到、过期处理）。执行器需要 Redis 分布式锁（ops-rpc 已有）并调域 RPC
  （game/user 等，任意 rpc 都能调）。放 ops-rpc 平台层，task_plans/task_runs 进 gacha_ops，
  与 server_v2 的 task 服务接口一致（照抄其 proto），仅服务位置不同。
  备选：新建 task-rpc——多一个服务的 etcd/redis/dtm/model 全套基建，收益仅为形式对齐，
  故不取。
- **报表（reporting）**：server_v2 reporting 服务本身就是跨域只读聚合。
  本仓由 ops-rpc 开只读多库连接（gacha_user/draw/payment/game 等）直接聚合，
  域服务不增加报表 RPC（避免玩家服务为后台报表加负载）；接口照抄 server_v2
  reporting proto，仅宿主不同。

## 5. 审批→发布流（server_v2 语义照抄，域库单事务）

```
控制台(admin gateway) → draw-rpc（gacha_draw 单库，无 SAGA）
  1. CreatePoolDraft            → 校验模式规则 + 密封（config_hash=SHA256）
                                   + draw_pool_versions 插 DRAFT + 幂等表
                                   （同 Idempotency-Key 同载荷重放返回既有草稿；
                                     同键异载荷 409）
  2. RequestPoolPublication     → 锁版本行（DRAFT+哈希匹配）+ 审批行 PENDING
                                   （requester=会话主体；同请求重放幂等）
  3. ApprovePoolPublication     → 审批行 CAS PENDING→APPROVED
                                   （approver=会话主体；审批人≠申请人，自批 409；
                                     同人重批幂等）
  4. PublishPool                → 版本行+审批行双校验（哈希守卫）→ 同一事务物化
                                   draw_boxes（upsert 保 created_at）+
                                   draw_pool_entries（整体替换，remaining=quantity）
                                   + CAS DRAFT→PUBLISHED；
                                   已发布同哈希幂等返回，不再物化
```

单审批（审批人 ≠ 申请人），无 SAGA 分支/补偿——所有写在 gacha_draw 一个事务内，
天然原子。后续里程碑（M14c/M14d）的发布流按各自 server_v2 语义查证后照抄，
若同样是域库单事务则不开 SAGA。

## 6. 里程碑划分（每边界 run_all 全绿 ×2 + build/test/前端构建）

| 里程碑 | 内容 | 新增场景 |
|---|---|---|
| **M14a** ✅ 已交付 | 管理网关 + 身份/授权/审计（D1-D5, D7, D9） | ✅ 场景 24 |
| **M14b** ✅ 已交付 | 17_gacha_draw_m14b.sql 池草稿四表；draw-rpc 7 个池 RPC；管理网关 9 路由（pool.api + fail-closed M14b）；16 号 SQL promotion-admin 种子（gacha.pool/assign_lucky）；撤销 DTM-admin 决策（§3） | ✅ 场景 25 |
| **M14c** | catalog-rpc 激活（SPU/SKU/修订/快照/发布，4000 段） | 26 商品目录 |
| **M14d** | game（campaign/券/忠诚度）、base（资格/风控）、payment（路由）、user（只读）控制台 RPC + 管理网关路由 | 27 业务配置 |
| **M14e** | 任务计划+调度、报表聚合、前端 web_admin 全量（Vue3+AntDV+i18n、动态权限路由、审批中心、各配置页） | 28 任务报表 |

## 7. 明确不做

- 不开新业务服务；玩家面接口、玩家 SAGA 链路零改动（控制台 RPC 只追加，不重构）。
- 不搬迁既有表（campaign/券留 gacha_game、资格/风控留 gacha_base）。
- 不实现 server_v2 遗留未使用的通用审批表（authorization_approval_request/action）。
- 不新增管理侧 DTM 库/实例（§3 已查证撤销）。
- 接口照抄约定不变：/console/v2/api/ 前缀、camelCase protojson、JWT claims、
  kratos 形错误体（D9）、X-Request-Id（D1）、fail-closed 启动校验（D5）。
- 任务/报表的 server_v2 接口形状逐字段照抄，仅宿主服务位置不同（本计划核心调整）。
