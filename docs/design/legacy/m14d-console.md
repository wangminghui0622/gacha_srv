# M14d 活动/条件政策/券/风控/支付路由控制台

## Context

M14 管理控制台第 4 步：克隆 server_v2 剩余 5 个 console 服务面共 19 条路由——
ConsoleCampaignHTTPService（3）、ConsoleEligibilityHTTPService（5）、ConsoleRiskHTTPService（3）、
ConsolePromotionHTTPService（6）、ConsolePaymentRouteService（2）。
业务功能照抄（版本化活动草稿→发布、CEL 条件政策版本化发布、风险案件状态机、券模板不可变版本
+ 发券、支付路由 CAS 存管），接口契约不变（/console/v2/api/... 路径、camelCase protojson、
JWT claims、kratos 形错误体），服务划分按计划：campaign+promotion→game-rpc、eligibility+risk→
base-rpc、payment route→payment-rpc。无 SAGA、无 DTM（全部单库事务）。

## 已验证的核心设计决策（server_v2 语义逐条提取）

- **D1 Campaign 版本化（head+version 两表，照抄）**：现有 `campaigns` 表即 head（加
  `published_version` 列），新 `campaign_versions` 表（campaign_id+version 主键，state/name/
  config_json/start_at/end_at）。**CreateDraft**：CampaignId==0 → 按 (campaign_key) 查 head，
  存在 → 冲突；插 head（published_version=0）+ 插 v1 DRAFT。CampaignId!=0 → 锁 head FOR UPDATE；
  key 非空且不符 → 冲突；kind 不符 → 冲突；latest version state==DRAFT → 冲突（草稿上不能再加草稿）；
  v=latest+1。**Publish**：锁 head；latest 非 DRAFT → closed 冲突（server_v2 无重放幂等分支，
  已发布再发 → closed）；物化到 campaigns 行（player 形状 config_json、status=1、published_version=v，
  `UPDATE ... WHERE published_version=旧值` CAS）；单事务。**Get**：published_version>0 读该版本行，
  =0 读 latest 版本行（未发布草稿读最新版）。
- **D2 kind 与配置双形态**：campaigns.kind 迁移 3→13（PRIZE_WHEEL=13 对齐 server_v2 枚举，
  M11a 常量 campaignWheel 同步改 13，种子 SQL 同步改）。version.config_json 存 **console 形状**
  （server_v2 protojson：SignInConfig{rewards[{day,loyaltyPoints,prizes}]} /
  RedemptionConfig{codes[{code,loyaltyPoints,maxRedemptions,perUserLimit,prizes}]} /
  PrizeWheelConfig{slots[{weight,prizes,dailyStock}],dailyLimit}）；**发布物化转 player 形状**：
  签到 → `{"rewards":[{"prizes":[{kind:1,cashFen}|{kind:2,points}]}]}`（day 连续从 1，Prize.kind
  CASH=1→现金、LOYALTY_POINT=4→积分，其余 PrizeKind 本里程碑不支持报校验错）；
  兑换码 → 逐 code upsert campaign_redemption_codes（prize_config JSON 数组、max/per_user）；
  转盘 → 逐 slot upsert campaign_wheel_slots + config `{"dailyLimit":N}`。
  校验（照抄 NormalizeDraft + kind 校验）：tenant_id>0、key 匹配 `^[a-z][a-z0-9_.-]{1,63}$`
  （新建必填）、name 1..128、窗口 end>start>0、config 非空；签到 day 从 1 连续且 loyalty_points≥0；
  转盘 slots 非空、weight>0、daily_limit>0；兑换码 codes 非空、code 非空、per_user_limit≤max_redemptions
  （max>0）。
- **D3 Eligibility 全量换 server_v2 模型（版本化+CEL）**：重建三表
  `eligibility_policies`（**head**：id/policy_code uk/name/current_version/latest_version）、
  `eligibility_policy_versions`（policy_id+version 主键，scene/status 1DRAFT/2PUBLISHED/3RETIRED/
  combination 1ALL/2ANY/required_facts JSON/config_hash BINARY(32)/published_at，
  uk(policy_id,config_hash)）、`eligibility_policy_rules`（policy_id+policy_version+rule_id 主键，
  expression/failure_code/sort_order）。旧三表（drafts/policies/rules 事实键模型）**DROP**，
  种子改为 4 条 head+version 空规则 PUBLISHED 策略（P-DISCOVER-1 等，current_version=1）。
  **CreateDraft**：code 匹配 `^[a-z][a-z0-9_.-]{1,63}$`、name 非空 → v1 DRAFT。
  **CreateRevision**：锁 head；latest_version==expected_latest_version 否则冲突；v=latest+1；
  同步改 head.name。**PublishPolicy**：锁 head；version 行存在且 DRAFT 且 config_hash==
  expected_config_hash 否则冲突；RETIRE 旧 PUBLISHED（status=3）；该行→PUBLISHED+published_at；
  head.current_version=v。**config_hash 计算**（照抄 prepareVersion）：克隆 PolicyVersion 后
  PolicyId/Version/Status/ConfigHash 置零、required_facts 排序，proto Deterministic marshal
  → SHA256。**ListPolicies**：游标 after_policy_id、page_size 默认 20 封顶 100，回 head+
  latest version；**GetPolicy**：version=0 读 latest。
  **评估引擎换 CEL**（cel-go）：evaluatePolicyLogic 重写——scene 解析规则 = 所有
  current_version 为 PUBLISHED 且 scene 匹配的 head 中 published_at 最新者（server_v2 评估方
  传入 policy 对象、无 scene 解析；本仓 SAGA 分支只传 scene，此为内部解析规则，多政策同 scene
  取最新发布）；无生效策略 = 放行（保持现状）。facts map={user_id,box_id}（策略请求无金额）；
  required_facts 缺失 → 拒绝（reason=MISSING_REQUIRED_FACT）；ALL=全部通过、ANY=至少一条；
  failed_rule 的 failure_code 进 reason。拒绝语义/SAGA Aborted 码不变（场景 8 断言保持）。
  场景 8 种子重写：P-DRAW-DENY 改插新三表（expression=`facts.user_id != <uid>`——规则通过即放行，
  目标用户不满足即拒，published_at=NOW() 晚于种子 → 解析命中）。
- **D4 Risk 案件状态机（照抄）**：risk_cases 状态数值化 1 OPEN/2 IN_REVIEW/3 RESOLVED/4 DISMISSED
  （ALTER 幂等迁移：OPEN→1/REVIEWING→2/APPROVED→3/DISMISSED→4），加列 assigned_actor_id
  BIGINT DEFAULT 0、resolution_code VARCHAR(64) DEFAULT ''、decision_id VARCHAR(64) DEFAULT ''、
  risk_score INT DEFAULT 0、action TINYINT DEFAULT 3（本仓案件全部 CHALLENGE，分数由 severity
  映射 LOW 30/MEDIUM 60/HIGH 90）；新 `risk_case_events` 表（event_id=operation_id uk，case_id/
  event_type/from_status/to_status/actor_id/resolution_code/occurred_at）。**Claim**：FOR UPDATE
  锁案件；已存在同 operation_id 事件 → 字段全同幂等重放、异同冲突；状态必须 OPEN；置
  IN_REVIEW+assigned_actor_id=actor_id+插事件。**Resolve**：状态必须 IN_REVIEW 且
  assigned_actor_id==actor_id（非认领人冲突）；target RESOLVED/DISMISSED、resolution_code 1..64
  非空；插事件。**ListCases**：游标 (before_created_at_unix_millis, before_case_id)（首查空），
  page_size 默认 20 封顶 100；回 caseId=case_no/decisionId/sceneCode=scene/userId/deviceId=0/
  action=CHALLENGE/riskScore/status/assignedActorId/createdAtUnixMillis。Claim/Resolve RPC
  换 server_v2 形状（caseId/actorId/operationId/tenantId——本仓无调用者，直接替换 M7 旧形状）。
- **D5 券模板不可变版本 + 发券**：新 `coupon_template_heads`（template_id 主键 client 指定、
  next_version）、`coupon_template_versions`（template_id+version 主键，全字段 + request_id uk +
  published_at）。**PublishCouponTemplate**：request_id 必填；同 request_id 重放 → 内容全同
  幂等返回原版本、异同冲突；锁 head（无则建）→ version=next_version → 插版本行 → head 前进
  CAS；同时物化 `coupon_templates` 行（template_no=`CONSOLE-<templateId>`，kind 映射
  FIXED_MINOR→1/PERCENT_BPS→2/FREE→3/CUSTOM_MINOR→4/DRAW_REWARD→5，value=amount_minor 或
  percent_bps，min_gross=min_gross_minor，status=1）——玩家侧折扣计算读该表不变。
  校验（照抄）：tenant/template_id/currency 3 字母；min/max 区间合法；amount 按 kind
  （FIXED>0 且 bps=0、PERCENT 0<bps≤10000 且 amount=0、FREE 全 0、CUSTOM amount>0 且 bps=0）；
  funder PLATFORM/MERCHANT；DRAW_REWARD 需 reward_template_id 已发布且 ≠自身。**ListCouponTemplates**
  游标 (after_template_id, after_version)，回全版本行（不可变版本列表）。**GetCouponTemplate** 读
  指定版本（console 无此路径的版本参数——GET 路径仅 template_id，取最新版）。**GrantCoupons**：
  request_id+tenant+user+template_id+count 必填；读 LatestPublished 版本；CUSTOM_MINOR 需
  0<custom≤amount_minor、其余 custom 必须 0；逐张铸 user_coupons（coupon_no=`gc_<requestId>#<n>`
  uk 幂等、template_no=CONSOLE-<id>、expires_at=now+validity_days、AVAILABLE）。
  **Loyalty defaults**：Get/Put 读写 loyalty_offset_config id=1（loyalty_points_per_major↔
  points_per_yuan、max_gross_bps↔max_ratio_bps；tenant_id 单租户固定 1）。
- **D6 Payment route 存管（半成品照抄，无消费方）**：新三表 `payment_route_policies`
  （uk(tenant_id,channel,scene)，id/mode/pinned_provider/pinned_merchant_account_id/version/
  updated_by）、`payment_route_candidates`（policy_id/merchant_account_id/provider/priority/
  weight/enabled）、`payment_route_audits`（policy_id/old_version/new_version/operator_user_id/
  old_snapshot/new_snapshot JSON/changed_at）。**Get**：无行 → 404 ROUTE_POLICY_NOT_FOUND。
  **Put**：校验照抄（policy+≥1 candidate≤16；channel∈{WECHAT,ALIPAY}、scene∈{APP,MINI_PROGRAM,H5}；
  mode AUTO/PINNED；新建 policy_id==0 且 expected_version==0，更新 policy_id==行 id 且
  expected_version==行 version；AUTO 不得带 pinned；candidate provider∈{NATIVE_WECHAT,
  NATIVE_ALIPAY}（本仓适配器固定，registry.GetConfigured 检查省略——实现可优化）、
  merchant_account_id 1..64、weight 1..10000、(provider,merchant) 去重、≥1 enabled、PINNED 时
  pinned 必须在 enabled 中）；FOR UPDATE 锁 policy 行 CAS version（不符 409）→ 更新/插入 +
  全量替换 candidates + audit 快照行（old_snapshot 为旧整包 JSON）。**偏离记录**：server_v2 校验
  enabled candidate 必须命中 provider 注册表；本仓无注册表，provider 白名单即校验。
- **D7 单租户 tenant 约定**：全部 console proto 保留 tenant_id 字段（wire 兼容），网关统一
  填 1（server_v2 取 principal.TenantID，本仓单租户固定 1）；operator_user_id/actor_id 取
  JWT 主体 operator id；审计由 ConsoleAuthorize 中间件按 operation_id 自动落。
- **D8 错误码按既有域分段追加（gateway 映射 404/409/400，信封形状不变）**：
  game 域 3000 段追加 `3035 CodeCampaignConflict`（活动状态/版本冲突：closed、草稿上加草稿、
  key/kind 不符）→ 409 CAMPAIGN_CONFLICT、`3036 CodeCouponTemplateConflict`（同 request_id 异
  内容/head 版本 CAS 失败）→ 409 COUPON_TEMPLATE_CONFLICT、`3037 CodeCouponInvalid`（发券/券
  模板参数无效）→ 400 COUPON_INVALID；复用 3005→404 CAMPAIGN_NOT_FOUND、3007→400
  CAMPAIGN_INVALID、3001→404 COUPON_TEMPLATE_NOT_FOUND。base 域 5000 段追加 `5006
  CodePolicyNotFound`→404 POLICY_NOT_FOUND、`5007 CodePolicyConflict`→409 POLICY_CONFLICT、
  `5008 CodePolicyInvalid`→400 POLICY_INVALID、`5009 CodeRiskCaseInvalid`→400 RISK_CASE_INVALID；
  复用 5003→404 RISK_CASE_NOT_FOUND、5004→409 RISK_CASE_CONFLICT。payment 域 8000 段追加
  `8003 CodePaymentRouteNotFound`→404 PAYMENT_ROUTE_NOT_FOUND、`8004
  CodePaymentRouteVersionConflict`→409 PAYMENT_ROUTE_VERSION_CONFLICT、`8005
  CodePaymentRouteInvalid`→400 PAYMENT_ROUTE_INVALID。
- **D9 fail-closed 里程碑推进**：gateway-api.yaml `FailClosedMilestone: M14d`；adminop 登记
  19 个操作（console.campaign.createdraft/publish/get、console.eligibility.list/get/create/
  createrevision/publish、console.risk.list/claim/resolve、console.promotion.listcoupontemplates/
  getcoupontemplate/publishcoupontemplate/grantcoupons/getloyalty/putloyalty、
  console.paymentroute.getpaymentroute/putpaymentroute），MilestoneOf=M14d，与路由双向闭合。
- **D10 无 SAGA**：全部单库事务；claim 无 body（HTTP 无体路由，handler 不 ParseProto）。

## DDL — 19_gacha_game_m14d.sql / 19_gacha_base_m14d.sql / 19_gacha_payment_m14d.sql（幂等，init_all.sql 聚合）

- **game**：campaigns 加 published_version INT UNSIGNED DEFAULT 0（条件 ALTER）+ 种子 kind
  3→13 迁移 + campaign_versions 表 + coupon_template_heads/coupon_template_versions 表。
- **base**：DROP 旧 eligibility_policy_drafts/eligibility_policies/eligibility_policy_rules →
  CREATE 新三表（D3）+ 种子 4 条空规则策略（head+version，current_version=1，published_at 置
  初始化时刻）+ risk_cases 条件 ALTER（状态数值化迁移+新列）+ risk_case_events 表。
- **payment**：payment_route_policies/payment_route_candidates/payment_route_audits 三表。
- 全部 InnoDB utf8mb4；无种子外键跨库。

## proto 扩展（game.proto / base.proto / payment.proto；`bash script/goctl.sh game|base|payment`）

- **game.proto**：枚举 CampaignKind（SIGN_IN=1/REDEMPTION_CODE=2/PRIZE_WHEEL=13，注释其余
  21 种后续里程碑）、CampaignState（DRAFT=1/PUBLISHED=2）、PrizeKind（CASH=1/LOYALTY_POINT=4）、
  Prize/SignInReward/SignInConfig/RedemptionCodeSpec/RedemptionConfig/WeightedSlot/
  PrizeWheelConfig/CampaignConfig（sign_in=1/redemption=2/prize_wheel=13 字段号照抄，其余 kind
  未实现不定义）、Campaign/CreateCampaignDraftRequest/PublishCampaignRequest/GetCampaignRequest
  （字段名编号照抄 server_v2：tenant_id/campaign_id/campaign_key/kind/name/state/version/
  start_at_unix_millis/end_at_unix_millis/config/published_version）+ RPC CreateCampaignDraft/
  PublishCampaign/GetCampaign。promotion console 消息（CouponTemplate 全字段照抄 +
  DiscountFunder/CheckoutScene 枚举、PublishCouponTemplateRequest/GetCouponTemplateRequest/
  ListCouponTemplatesRequest/Reply、CouponGrantSource 枚举、UserCoupon、
  GrantCouponsRequest/Reply、TenantLoyaltyDefaults/GetTenantLoyaltyDefaultsRequest/
  PutTenantLoyaltyDefaultsRequest）+ RPC ListCouponTemplateVersions/GetCouponTemplate/
  PublishCouponTemplate/GrantCoupons/GetTenantLoyaltyDefaults/PutTenantLoyaltyDefaults。
- **base.proto**：EligibilityScene/PolicyStatus/RuleCombination/EligibilityRule/PolicyVersion/
  EligibilityPolicy/EligibilityPolicyRecord/CreatePolicyDraftRequest/CreatePolicyRevisionRequest/
  GetPolicyRequest/ListPoliciesRequest/Reply/PublishPolicyRequest（照抄字段名编号）+ RPC
  CreatePolicyDraft/CreatePolicyRevision/GetPolicy/ListPolicies/PublishPolicy；risk：RiskCaseStatus/
  RiskCaseEventType/RiskCaseSummary/ListRiskCasesRequest/Reply/ClaimRiskCaseRequest/
  ResolveRiskCaseRequest/RiskCaseTransition + RPC ListRiskCases；ClaimRiskCase/ResolveRiskCase
  **替换**为 server_v2 形状（无内部调用者）。
- **payment.proto**：PaymentChannel/PaymentProvider/PaymentRouteMode/PaymentScene/
  PaymentRoutePolicy/PaymentRouteCandidate/GetPaymentRoutePolicyRequest/Reply/
  PutPaymentRoutePolicyRequest/Reply + RPC GetPaymentRoute/PutPaymentRoute。
- goctl 生成；禁跑 gateway。

## admin gateway 19 路由（pkg/api_admin/desc/{campaign,eligibility,risk,promotion,payment}.api + goctl admin-gateway）

| 方法/路径 | 操作 | RPC |
|---|---|---|
| POST /campaigns/drafts | console.campaign.createdraft (write) | CreateCampaignDraft |
| POST /campaigns/:campaignId/publish | console.campaign.publish (publish) | PublishCampaign |
| GET /campaigns/:campaignId | console.campaign.get (read) | GetCampaign |
| GET /eligibility/policies | console.eligibility.list (read) | ListPolicies |
| GET /eligibility/policies/:policyId/versions/:version | console.eligibility.get (read) | GetPolicy |
| POST /eligibility/policies | console.eligibility.create (create) | CreatePolicyDraft |
| POST /eligibility/policies/:policyId/revisions | console.eligibility.createrevision (write) | CreatePolicyRevision |
| POST /eligibility/policies/:policyId/versions/:version/publish | console.eligibility.publish (publish) | PublishPolicy |
| GET /risk/cases | console.risk.list (list) | ListRiskCases |
| POST /risk/cases/:caseId/claim | console.risk.claim (claim) | ClaimRiskCase |
| POST /risk/cases/:caseId/resolve | console.risk.resolve (resolve) | ResolveRiskCase |
| GET /promotion/tenants/:tenantId/coupon-templates | console.promotion.listcoupontemplates (read) | ListCouponTemplateVersions |
| GET /promotion/tenants/:tenantId/coupon-templates/:templateId | console.promotion.getcoupontemplate (read) | GetCouponTemplate |
| POST /promotion/tenants/:tenantId/coupon-templates | console.promotion.publishcoupontemplate (publish) | PublishCouponTemplate |
| POST /promotion/tenants/:tenantId/coupon-grants | console.promotion.grantcoupons (grant) | GrantCoupons |
| GET /promotion/tenants/:tenantId/loyalty-defaults | console.promotion.getloyalty (read) | GetTenantLoyaltyDefaults |
| PUT /promotion/tenants/:tenantId/loyalty-defaults | console.promotion.putloyalty (write) | PutTenantLoyaltyDefaults |
| GET /payment/routes | console.paymentroute.getpaymentroute (read) | GetPaymentRoute |
| PUT /payment/routes | console.paymentroute.putpaymentroute (write) | PutPaymentRoute |

PublishCouponTemplate 路由 body 取 "template" 字段（照抄 kratos body:"template"）→ handler
从 body 提 template 字段；PutLoyalty body 取 "defaults"；其余 body:"*"。claim 无 body。
handler 统一 tenant_id=1（路径 :tenantId 参数一致性校验：非 1 → 400）。

## 测试 — test/scenario_27_console_m14d.sh（run_all 24 场景）

- SQL 复位（admin TOTP、e2e 账号/角色、活动版本/券模板版本/政策/案件/支付路由表清空）→
  登录 → 角色（campaign-writer 全写、eligibility-writer、risk-worker、promo-admin、route-writer、
  noperm）→ 403 负例。
- **活动**：创建签到草稿（3 天阶梯）→ Get 草稿 state=DRAFT → 发布 → published_version=1 →
  玩家 ListCampaigns 可见 + Participate 签到积分到账（SQL 断言）→ campaignId 加草稿版 v2 →
  v2 未发布再加草稿 409 → 发布 v2 → 重复发布 409（closed）→ 转盘草稿（slots 2 个）→ 发布 →
  转盘 slot 物化断言（campaign_wheel_slots）→ 兑换码草稿（codes 2 条）→ 发布 → 兑换码表物化 +
  玩家兑换生效 → SQL 终态断言（版本行/物化行/kind=13）。
- **条件政策**：创建政策草稿（e2e-pol-27，DRAW，rules=[facts.user_id==<uid> 表达式]）→
  List/Get → CreateRevision expected_latest_version 不符 409 → 修订 v2 → Publish v1（expected_config_hash
  不符 409 → 正确 hash 200）→ EvaluatePolicy(DRAW, uid) 拒绝 + reason 含 failure_code →
  EvaluatePolicy(DRAW, 其他用户) 放行 → Publish v2 → v1 RETIRED 断言 + 评估按 v2 生效。
- **券**：PublishCouponTemplate（templateId 27，FIXED_MINOR 500/currency CNY/validityDays 7）→
  同 requestId 重放幂等 → 同 requestId 异内容 409 → List/Get → GrantCoupons ×2 → user_coupons
  AVAILABLE + expires_at=+7 天断言 → 再发布 v2（提额 800）→ List 两版本 → Grant 走 v2
  （custom_amount 校验负例）→ loyalty defaults Get/Put 断言 loyalty_offset_config 落库。
- **风控**：SQL 插 OPEN 案例 → List（两页游标）→ Claim（actor 1）→ 重复 claim 409 →
  Resolve 非认领人（actor 2）409 → Resolve（actor 1，RESOLVED）→ 同 operation_id 重放幂等 →
  events 表 2 行断言 + 状态断言。
- **支付路由**：GET 404 → PUT 新建（policy_id=0，AUTO，2 candidates）→ GET 回 v1 →
  PUT expected_version 不符 409 → PUT v2（PINNED）→ audit 2 行 + candidates 替换断言。
- 审计落库（ops_audit_logs 按 operation_id 断言 19 类操作抽样）→ 现场清理。
- run_all.sh/menu.sh 注册 scenario_27。

## 实施顺序

1. errorx 3035-3037/5006-5009/8003-8005 + 05-errors.md 登记 + restx MapError 映射。
2. DDL 三文件 + init_all.sql + mysql 执行（game/base/payment 库）。
3. proto 扩展（game/base/payment）→ goctl.sh 三生成 → 手动实现 logic（goctl 保留已有 logic 文件）。
4. game-rpc：core/campaignConsole.go（console↔player 配置转换 + 校验，纯函数单测）→
   campaign 3 RPC + promotion 6 RPC（repo 层 D1/D5 事务流）→ 重启 game-rpc。
5. base-rpc：core/eligibilityConsole.go（config_hash 计算纯函数单测）+ CEL 引擎
   evaluatePolicyLogic 重写（cel-go，go.mod 加依赖）→ 5+3 RPC → 场景 8 种子改新三表 →
   重启 base-rpc。
6. payment-rpc：route 三表模型 + Get/Put logic（CAS+audit）→ 重启 payment-rpc。
7. 网关：desc 5 文件 → goctl admin-gateway → svc/config/yaml 加 GameRpc 复用（已有）/
   BaseRpc/PaymentRpc 客户端 → FailClosedMilestone M14d → adminop 19 操作 → 19 handler+logic →
   重启 admin gateway。
8. 场景 27 → run_all 24 场景 ×2 全绿。
9. go build ./... + go test 核心包（game/base/payment core）+ npm run build → 文档更新
   （m14-admin-console.md §4.4/§7）→ 提交（Co-Authored-By，排除清单）。

## 风险与边界

1. 活动发布物化与 player 逻辑解耦：player 只读 campaigns/campaign_wheel_slots/
   campaign_redemption_codes 行，console 版本行独立——M11a 场景 13-15 零改动（仅 kind 常量
   3→13 与种子同步，行为不变）。
2. Eligibility 换引擎是 M14d 最大风险点：评估拒绝的 SAGA 行为（1310? Aborted 码、原因透出、
   决策日志 policy_no）保持逐字节兼容；场景 8 种子改写后断言不变；run_all 回归兜底。
3. CEL 表达式编译失败/非法（ValidatePolicy 时校验 bool 输出）→ 4013 拒绝入库。
4. 场景 27 全确定性（CEL 固定表达式、无随机），断言用状态/版本/行数。
5. 不在本里程碑做：任务/报表控制台（M14e）、web_admin 前端（M14e）、支付路由消费方
   （半成品存管，与 server_v2 同款）、CampaignConfig 其余 26 种 kind。
6. 永远只跑 `bash script/goctl.sh game|base|payment` 与 `bash script/goctl.sh admin-gateway`，
   不跑玩家 gateway。
