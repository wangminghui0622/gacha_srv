# M14e 任务计划/调度 + 报表聚合 + web_admin 前端全量

## Context

M14a-d 已交付：管理网关 + 控制台身份/授权/审计、赏池发布流、商品目录、五域控制台
（活动/政策/券/风控/支付路由），run_all 24 场景。M14e = 最后一程：**任务计划（版本化+
提请/审批/发布）**、**任务进度列表（空数据就位）**、**报表（指标目录/看板保存/发布/查询/
用户盈亏曲线）**、**web_admin 前端全量（~21 视图）**、场景 28、收尾提交。

server_v2 参考：`api/v2/task/task.proto` + `api/v2/reporting/reporting.proto`（控制台 HTTP
映射在 `api/v2/gateway/console/public.proto`）；实现 `app/service/task`、`app/service/reporting`。
全部落 **ops-rpc（9318 / gacha_ops）**——服务划分文档已定：平台层任务计划/报表归 ops-rpc，
不开新服务。

## 已验证的核心设计决策（server_v2 语义逐条提取）

### D1 任务计划：三表 + head 单例行（照抄 000118/000119 迁移）

- `task_plan_head`（singleton_id=1 单行）：current_version / latest_version，乐观锁 CAS
  （`SetTaskPlanHead` 带 current_2/latest_2 条件，updated!=1 → CONFLICT）。
- `task_plan_version`：version PK、status 1 DRAFT/2 PUBLISHED/3 RETIRED、config_hash
  BINARY(32) UK、config_blob MEDIUMBLOB（deterministic proto 字节）、created_at/published_at。
- `task_plan_publication_approval`：plan_version PK、config_hash、requester_id、approver_id
  （0=未审）、status 1 PENDING/2 APPROVED、requested_at/approved_at。
- **启动种子**（EnsureBootstrapPlan）：head 不存在或 latest=0 时插 version 1 PUBLISHED
  （种子 plan 来自配置），current=latest=1。场景 28 前 SQL 复位此三表后重种。

### D2 计划校验与 config_hash（biz/plan.go + reward.go）

- TaskDefinition 校验：id>0、version>0、target>0、period/metric 非 UNSPECIFIED、
  id 不重复；奖励二选一——legacy（reward_template_id 非空 + reward_count 1..1000，无
  structured reward）或 structured（count 1..1000 + oneof coupon{template_id,template_version}
  / inventory{product_id,sku_id,product_version}，且 legacy 字段为空）。pass/member_tier
  同款校验；tasks 按 id 升序存入。
- **config_hash = SHA256(deterministic proto 序列化 canonical plan)**：clone 后清
  version/status/config_hash/created/published 字段再 MarshalOptions{Deterministic:true}。
- **HasStructuredRewards → 发布拒绝**（照抄半成品语义）：任一 task/pass/tier 带 structured
  reward → TASK_PLAN_NOT_READY（"原子发奖持久化未就绪"）；legacy reward_template_id 计划
  可正常发布。

### D3 计划状态机（data/plan.go，全部 FOR UPDATE 串行）

- **CreateTaskPlanDraft**：head 锁 → `latest == expectedLatestVersion` 否则 CONFLICT →
  version=latest+1 → 插 DRAFT → CAS head.latest=version。返回带 version/status/hash/created。
- **GetTaskPlan**：version=0 → latest；无行 NOT_FOUND。
- **ListTaskPlans**：afterVersion 游标、pageSize 默认 20 上限 100、取 pageSize+1 行，
  next_after_version=末条已回行 version（peek 行判 has-more，M14c 同款）。
- **RequestTaskPlanPublication**：head.latest==version（否则 CONFLICT）→ version 行 FOR
  UPDATE 须 DRAFT+hash 匹配 → 已有审批行：同 requester+同 hash → **幂等重放返回 prior**，
  否则 CONFLICT → 无行则插 PENDING（approver=0）。
- **ApproveTaskPlanPublication**：同上前置 → 审批行 hash 匹配 + **approver != requester**
  （否则 CONFLICT）→ 已 APPROVED：同 approver 重放，异 approver CONFLICT → PENDING →
  CAS 更新 APPROVED（approver/approved_at）。
- **GetTaskPlanPublicationApproval**：无行 NOT_FOUND。
- **PublishTaskPlan**：head.latest==version → version 行 FOR UPDATE：
  已 PUBLISHED && current==version && hash 同 → **幂等重放**；否则须 DRAFT+hash →
  HasStructuredRewards → NOT_READY → 审批行须 APPROVED+hash+approver>0+approver≠requester
  → RetirePublishedTaskPlans（旧 PUBLISHED 全置 RETIRED）→ CAS 发布行（status=2,
  published_at, hash）→ CAS head（current=latest=version）。

### D4 任务进度列表（空数据就位）

- **ListTasks**（console GET /tasks）：user_id 必填（0 → INVALID）、pageSize 默认 20 上限
  50、period 过滤可选、page_token=task_id 游标。数据 = 已发布计划的 task 定义 × 用户进度。
  **本仓无进度管线**（ApplyProgress/ClaimTask/SignIn/ClaimPass/ClaimMember 玩家侧不在
  M14e 范围，设计文档已定）：进度恒 0/claimed 0/state IN_PROGRESS（target>0 必成立），
  不建 progress 表。返回形状照抄 TaskView{definition,progress,claimed,state}。

### D5 报表五表 + 种子（照抄 000026 迁移）

- `reporting_metric_definition`（metric_code PK、title、standard_metric 1-5、value_kind
  1 COUNT/2 MONEY、supported_grains SET('hour','day')、status、时间戳）；
  `reporting_dimension_definition`；`reporting_metric_dimension`（PK metric+dimension）；
  `reporting_dashboard`（id PK 手工生成、dashboard_code UK、title、status 1 DRAFT/2
  PUBLISHED/3 ARCHIVED、version、published_at）；`reporting_dashboard_widget`（id PK、
  dashboard_id、widget_code UK(dashboard_id)、title、kind 1-4、metric_code、grain、dimension_code、
  grid_x/y/w/h、display_options JSON）。
- 种子：dimension 'all'（全部）；metrics provider.impressions/clicks/conversions/spend/revenue
  （标题 展示量/点击量/转化量/投放花费/投放营收，spend/revenue 为 MONEY）；metric_dimension
  全 ×all。

### D6 看板 CRUD（biz/reporting.go + data/dashboard.go）

- **校验**：dashboard_code ≤64、title 1..128、widgets 1..100；widget code ≤64 唯一、title
  1..128、metric/dimension code 合法、kind 1-4、grain 仅 HOUR/DAY、grid 24 列宽约束
  （w>0、w≤24、x+w≤24、h 1..100、y≤10000）、display_options 合法 JSON object 且 ≤8192、
  **widget 矩形互不重叠**。
- **SaveDashboard**：id=0 → 新 id（雪花序）+version 1+DRAFT；已有 → 必须 DRAFT，CAS
  version 更新、version++，widgets **全量替换**（id 重生成）。create 时 version≠1 → CONFLICT。
- **PublishDashboard**：DRAFT→PUBLISHED，expected_version CAS。
- **GetDashboard**：无行 NOT_FOUND。
- **QueryDashboard**：校验（tenant/dashboard/from<to/max_data_points≤2000）→ 看板必须
  PUBLISHED（否则 NOT_PUBLISHED）→ 逐 widget 按 metric 目录解析（未知 metric →
  INVALID）→ **buckets 读 provider metric 桶——M15d 前本仓无集成管线恒空，M15d（2026-10-25）provider 指标接入后桶由 thirdparty-rpc outbox→ops-rpc 直连灌入，看板查询数据就位**（场景 32 已断言 query 数据点非空）→ points 空数组、
  truncated=false；NUMBER 类 sum 后单点（空）。带 Redis 结果缓存（key=sha256(看板
  id/version/query 参数)），skip_cache 可跳。
- **QueryUserProfitCurve**：**照抄 stub**——校验 tenant/user/grain(HOUR/DAY/MONTH)/from<to
  后恒返回 data_ready=false + 空 points（server_v2 自身就是"投影未从 V1 迁入"空态，
  前端展示空态而非伪造数据；业务功能不要动，不做真实聚合）。

### D7 网关绑定（照抄 task_http.go/reporting_http.go）

- **requester/approver 取会话主体**：body 里的 requesterId/approverId 被网关覆盖为
  ClaimsFrom().ActorID；tenant_id 取会话主体 tenant（body 与会话不符 → 403
  TENANT_MISMATCH，本仓 M14a 会话模型固定 tenant=1，沿用 D7 规则：网关注入 tenant_id=1）。
- **QueryDashboard 要求 data scope ORG/ALL**（decision 非 ORG/ALL → 403）。
- 操作 id 以 proto 注解为准（kratos 的 tr.Operation() 覆盖 hand 常量）：console.task.list/
  createplandraft/getplan/listplans/requestplanpublication/approveplanpublication/
  getplanpublicationapproval/publishplan；console.reporting.queryuserprofit/metrics/save/
  get/publish/query。

### D8 错误码（ops 域 7000 段续排，05-errors.md 登记）

- 7016 TASK_PLAN_NOT_FOUND 404 / 7017 TASK_PLAN_CONFLICT 409 / 7018 TASK_PLAN_INVALID 400
  / 7019 TASK_PLAN_NOT_READY 400（FailedPrecondition→kratos 400）
- 7020 DASHBOARD_NOT_FOUND 404 / 7021 DASHBOARD_CONFLICT 409 / 7022 DASHBOARD_INVALID 400
  / 7023 DASHBOARD_NOT_PUBLISHED 400

### D9 权限种子（照抄 000120 语义）

- task.plan × {read,write,request_publication,approve_publication,publish} + task.progress
  read + reporting.metric {read,query} + reporting.dashboard {read,write,publish,query} →
  promotion-admin 角色（幂等 INSERT ... NOT EXISTS）；adminop 目录 14 操作同步登记。

### D10 幂等与重放

- Request/Approve 的"同参重放返回 prior"与 Publish 的"已发布重放"都**不依赖 Idempotency-Key**
  （server_v2 原样）；场景断言按语义写。

## DDL — 20_gacha_ops_m14e.sql（幂等，init_all.sql 聚合）

- 三张 task_plan_* 表（D1 形状，MySQL 8 无 CHECK 约束用应用层校验替代 server_v2 的
  CHECK/FK）+ 五张 reporting_* 表（D5 形状）+ 种子（dimension/metric/metric_dimension +
  bootstrap plan version 1：**无任务空计划**——EnsureBootstrapPlan 校验要求 tasks+passes+
  tiers 至少一个，本仓种子用 1 个 legacy 任务"每日登录"（period DAILY、metric LOGIN、
  target 1、reward_template_id 占位 T-BOOT、count 1）保证快照合法且进度恒 IN_PROGRESS）。
  场景 28 开头 SQL 复位 task 三表并重种。

## proto 扩展（ops.proto；`bash script/goctl.sh ops`）

- task 域消息照抄 task.proto：TaskPeriod/TaskMetric/TaskProgressState/TaskDefinition/TaskView/
  ListTasksRequest/ListTasksReply/TaskPlanStatus/TaskPlan/CreateTaskPlanDraftRequest/
  GetTaskPlanRequest/ListTaskPlansRequest/ListTaskPlansReply/PublishTaskPlanRequest/
  RequestTaskPlanPublicationRequest/ApproveTaskPlanPublicationRequest/
  GetTaskPlanPublicationApprovalRequest/TaskPlanPublicationApproval +
  TaskRewardDefinition{CouponRewardReference,InventoryRewardReference}（reward_definition.proto）。
  **不进** player 侧 RPC（ApplyProgress/ClaimTask/SignIn/ClaimPass/UnlockPass/ClaimMember）。
- reporting 域消息照抄 reporting.proto：DashboardStatus/WidgetKind/MetricValueKind/TimeGrain/
  MetricDefinition/DashboardWidget/Dashboard/Save/Publish/Get/Query 全家/WidgetDataPoint/
  WidgetQueryResult/QueryDashboardReply/UserProfitCurvePoint/QueryUserProfitCurve* +
  StandardMetric 枚举（1-5，来自 integration/metrics.proto，内联进 ops.proto）。
  **不进** Ingest/ListProviderMetricBuckets。
- ops-rpc service 追加：ListTasks、CreateTaskPlanDraft、GetTaskPlan、ListTaskPlans、
  RequestTaskPlanPublication、ApproveTaskPlanPublication、GetTaskPlanPublicationApproval、
  PublishTaskPlan、ListMetricDefinitions、SaveDashboard、PublishDashboard、GetDashboard、
  QueryDashboard、QueryUserProfitCurve（14 RPC）。

## admin gateway 14 路由（pkg/api_admin/desc/{task,reporting}.api + goctl admin-gateway）

| 方法/路径 | 操作 | RPC |
|---|---|---|
| GET /tasks | console.task.list (read) | ListTasks |
| POST /task-plans/drafts | console.task.createplandraft (write) | CreateTaskPlanDraft |
| GET /task-plans/:version | console.task.getplan (read) | GetTaskPlan |
| GET /task-plans | console.task.listplans (read) | ListTaskPlans |
| POST /task-plans/:version/publication-request | console.task.requestplanpublication (request_publication) | RequestTaskPlanPublication |
| POST /task-plans/:version/approval | console.task.approveplanpublication (approve_publication) | ApproveTaskPlanPublication |
| GET /task-plans/:version/approval | console.task.getplanpublicationapproval (read) | GetTaskPlanPublicationApproval |
| POST /task-plans/:version/publish | console.task.publishplan (publish) | PublishTaskPlan |
| GET /analytics/users/:userId/profit-curve | console.reporting.queryuserprofit (read) | QueryUserProfitCurve |
| GET /reporting/metrics | console.reporting.metrics (read) | ListMetricDefinitions |
| POST /reporting/dashboards | console.reporting.save (write) | SaveDashboard |
| GET /reporting/dashboards/:dashboardId | console.reporting.get (read) | GetDashboard |
| POST /reporting/dashboards/:dashboardId/publish | console.reporting.publish (publish) | PublishDashboard |
| POST /reporting/dashboards/:dashboardId/query | console.reporting.query (query) | QueryDashboard |

## web_admin 前端全量（M14e 主战场）

- 工程抄 web/（Vite+vue-tsc+i18n 双语+AntDV）；route catalog 照抄
  `web/console/src/navigation/routeCatalog.ts`：动态权限路由（allows/grants）、菜单分组
  operations/business/governance、隐藏路由（用户概览/盈亏/详情页）。
- 视图 21 个照抄：dashboard 工作台、operations/TaskProgressView、operations/TaskPlanView
  （编辑器+提请+审批+发布）、analytics/UserProfitCurveView（data_ready=false 空态）、
  users/UserOverviewView、catalog/ProductLookupView+ProductDetailView+SkuDetailView+
  ProductEditor、gacha/PoolLookupView+PoolDetailView+PoolEditor+BoxManagement+BoxDetailView、
  promotion/CouponManagementView、campaign/CampaignManagementView、eligibility/
  EligibilityPolicyView、authorization/RoleManagementView、reporting/ReportingDashboardView、
  risk/RiskCaseView、login/LoginView（TOTP 两步+首绑二维码）+ ConsoleLayout（侧边菜单/
  水印/退出）。
- 手写 typed services（我们栈无 protoc-gen-typescript-http）：axios 封装、X-Request-Id
  自动注入、401 清 Cookie 跳登录、401→silentNotFound 特例（GET approval 无行 404 是正常
  空态——server_v2 web/console/src/api/task.ts 同款）。
- i18n zh-CN/en-US 双语全量（与参考实现的刻意差异之一）。

## 测试 — test/scenario_28_task_reporting.sh（run_all 25 场景）

1. SQL 复位：task 三表重种（head 1/1、version 1 PUBLISHED 种子）、reporting 五表清
   e2e 行（metric 目录保留）、审计计数基线。
2. 登录两账号（A 计划申请方 + B 审批方，均带 task.plan 权限；另 C 无权限）。
3. 任务计划：A 建草稿 v2（expectedLatestVersion=1，含 legacy 任务）→ 重复
   expectedLatestVersion=1 → 409 → v3 用正确 latest → Get v3 / List 含 v3 → 提请
   （body 伪造 requesterId 被会话覆盖断言）→ 同参重放 200 prior → B 审批 → 审批后
   A 自审 409（approver==requester）→ B 发布（expectedConfigHash）→ v3 PUBLISHED、
   v1 RETIRED、head current=3 → 重放发布 200 → **structured reward 计划**：A 建 v4
   （带 coupon structured reward）→ 提请 → B 审批 → 发布 → 400 TASK_PLAN_NOT_READY。
4. 任务进度：GET /tasks?userId=<玩家> → 已发布计划任务进度 0/IN_PROGRESS；user 缺失
   → 400；page token 游标形状断言。
5. 报表：GET /reporting/metrics 5 指标 → POST /reporting/dashboards 建看板 v1（2 widget
   不重叠 + 1 重叠负例 400）→ Save 同 id CAS v2 → 未发布查询 → 400 DASHBOARD_NOT_PUBLISHED
   → 发布（版本不符 409 → 正确版本）→ 查询 200 results 2 条 points 空 → 重放缓存命中。
6. 盈亏曲线：GET /analytics/users/:userId/profit-curve?grain=DAY&from&to → 200
   dataReady=false + points 空。
7. 权限负例：C 访问 task-plans → 403；审计 ≥N 行 + 新 operation_ids。
8. 现场清理镜像复位。

## 实施顺序

1. docs/design/m14e-console.md（本文件）→ m14d-status.md 开 M14e 段。
2. 20_gacha_ops_m14e.sql（八表+种子）→ 执行 + mysql 验证。
3. ops.proto task/reporting 消息+14 RPC → `bash script/goctl.sh ops` → 模型脚本接入
   （task/reporting 八表 gacha_ops 模型）。
4. errorx 7016-7023 + restx MapError + 05-errors.md 登记。
5. ops-rpc 实现：internal/core/planConsole.go（校验+hash 纯函数+单测）+
   internal/core/dashboardConsole.go（看板校验纯函数+单测）+ 14 logic + repo → 重启
   ops-rpc。
6. admin 网关：desc task/reporting 2 文件 → goctl admin-gateway → adminop 14 操作 +
   14 handler/logic（requester/approver/tenant 注入、Query 数据域守卫）→
   FailClosedMilestone M14e → 重启 admin gateway。
7. 场景 28 + run_all/menu 注册（25 场景/28 项）→ standalone 绿。
8. **web_admin 全量前端**（工程+登录/TOTP+布局+路由权限+21 视图+typed services+i18n
   双语）→ npm run build 绿。
9. 边界门：run_all ×2、go build ./...、go test 核心包（ops 新增 core 单测）、
   npm run build ×2（web+web_admin）。
10. 文档收尾（m14-admin-console.md §4.5/§7 + 05-errors.md）+ memory 更新 + 提交
    （Co-Authored-By，排除 meta/ 清单）。

## 风险与边界

1. task_plan_head CAS 与 version FOR UPDATE 双重串行化（M14b 池发布同款），并发冲突面小。
2. config_hash 用 deterministic 二进制 proto——不受 protojson 编译漂移影响（M14d 教训）。
3. bootstrap 种子任务 reward_template_id 用占位符（T-BOOT 无真实模板）——ListTasks 只回
   定义不核验模板存在性（server_v2 同款：进度列表不 join 模板表）。
4. 看板查询数据来源：M15d（2026-10-25）provider 指标接入后由集成管线灌桶（场景 32 断言
   非空），M15d 前恒空是**照抄半成品**：接口/校验/缓存全功能，数据层留空，
   与 server_v2 在本仓等价形态一致。
5. 场景 28 审批人≠申请人断言走 CHECK 语义（approver=requester → CONFLICT）。
6. 永远只跑 `bash script/goctl.sh ops` 与 `goctl admin-gateway` 生成（gateway 手写 logic
   不跑 goctl gateway）。
7. web_admin 是全新 Vite 工程：build 与 web/ 独立，run_all 不碰前端。
