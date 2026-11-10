# M14 管理控制台（Admin Console）设计

> 参考实现：server_v2 的 `app/gateway/console` + `web/console`（已完整盘点，只读研究）。
>
> **用户定调（本次评审结论）**：他怎么做的，我们就怎么做——**业务功能不要动，服务可以调整，接口不能变**。
> 即：server_v2 console 的全部业务功能（含 TOTP 两步登录、草稿→审批→发布流、任务计划、商品目录）照抄；
> 后端服务划分按我们的 8+1 服务调整；HTTP 接口路径、请求/响应契约、JWT claims、错误形状与 server_v2 一致。

## 1. 参考实现盘点（server_v2 console 已完成的功能）

**形态**：独立 HTTP 网关（kratos，端口 8101，路径前缀 `/console/v2/api/`）+ Vue3 管理前端（web/console，~22 页面）。

**前端**：登录（用户名/密码 → TOTP challenge 两步验证，首绑含二维码/secret）→ 工作台；运营（任务进度、任务计划：草稿编辑→双人审批→发布）；业务管理（商品 SPU/SKU 与修订发布、赏池详情/编辑器与提请发布/审批、箱子详情+库存视图+指定特殊赏用户、营销券模板/发券/忠诚度、活动、条件政策）；系统（权限管理：角色 CRUD/角色权限（resource/action+数据范围+审批模式+条件+字段白黑名单）/角色成员/主体角色、报表看板、风控案件）；隐藏页（用户盈亏曲线、用户概览）。动态权限路由：登录后按权限树挂载。

**网关**：12 个 HTTP 服务约 70 端点（详见 §6 端点清单，来自 `api/v2/gateway/console/{authorization,payment_route,public}.proto` + `api/v2/identity/console_session.proto`），薄 handler 持 gRPC 客户端直转下游；编译期路由注册表（proto `console_operation` 注解 {operation_id, resource, action, public}，缺注解启动即失败，fail-closed）。

**鉴权链路**（每条请求）：
1. JWT（EdDSA，claims = actor_id/tenant_id/name/roles[]/jti/iss/aud）验签，网关只持公钥
2. Redis 会话吊销检查 `console:session:revoked:<jti>`
3. `X-Request-Id` 头强制要求（缺失 400）
4. 每请求调 authorization 服务 `Authorize`（RoleIds × Resource × Action × attributes → Allowed + DataScope + 字段投影）
5. 响应按 AllowedFields/DeniedFields 投影

**授权模型**：DataScope 六级（SELF/TEAM/DEPARTMENT/ORGANIZATION/CUSTOM/ALL → SQL 列过滤）；字段投影（AllowedFields/DeniedFields）；审批模式（approval_mode）；条件表达式。

**多租户**：控制台账号/RBAC/券模板/忠诚度/支付路由/看板表带 tenant_id；会话即单租户（tenant 从 JWT claims 读）；玩家侧 wallet/trade/inventory 无 tenant 列（user_id 全局唯一、1 用户 1 租户）。

**半成品面（照抄其现状）**：payment 路由管理有网关 proto 无前端页；risk 策略规则无前端配置面；engagement 内容无 console 视图。

## 2. 架构决策

| # | 决策 | 说明 |
|---|------|------|
| D1 | **独立管理网关 `app_admin/gateway`，端口 9319，路径前缀 `/console/v2/api/`（照抄）** | 不复用玩家网关 9310；go-zero rest 栈实现（`.api` 生成骨架 + 手写 logic），但**对外接口与 kratos 版完全一致**：路径、方法、请求/响应 JSON 形状、错误体、HTTP 状态码 |
| D2 | **服务划分调整（服务可以调整的落点）** | server_v2 17 服务 → 我们 8+1：console 账号/会话/TOTP（identity）+ authorization 全部 → **ops-rpc 9318**（激活，gacha_ops 库、7000-7999 错误码段已预留）；catalog 商品 → **catalog-rpc 9316**（激活）；gacha 奖池/箱子 → **draw-rpc**（普通盒池）+ **game-rpc**（玩法配置）；campaign/promotion → **game-rpc**；eligibility/risk → **base-rpc**；payment route → **payment-rpc**；task 计划/reporting → **ops-rpc**。RPC 契约（请求/响应字段）与 server_v2 同名同构 |
| D3 | **JWT 照抄** | EdDSA（Ed25519）签发（ops-rpc 持私钥）、admin 网关本地验签（持公钥，配置注入），claims 同构：actor_id/tenant_id/name/roles[]/jti/iss/aud；Redis 会话吊销照抄；X-Request-Id 强制照抄 |
| D4 | **授权照抄** | ops-rpc `Authorize` RPC：RoleIds × Resource × Action × attributes → Allowed + DataScope（六级全量）+ AllowedFields/DeniedFields 投影；角色权限含 data_scope/approval_mode/条件表达式 |
| D5 | **路由注册表 fail-closed（实现机制换 go-zero 方式）** | server_v2 用 proto 注解+代码生成；我们在 admin 网关 Go 代码建 operation registry（operation_id/resource/action/public 全量登记，与 §6 端点一一对应），路由未登记 = 启动失败。对外行为一致 |
| D6 | **审批流照抄** | 草稿（draft/revision）→ 提请发布（requester）→ 审批（approver）→ 发布，贯穿奖池/任务计划/条件政策/活动/商品修订/券模板；审批表带 requester/approver |
| D7 | **TOTP 两步登录照抄** | 登录 → 无 TOTP 则正常签发，有则返回 totpChallenge（loginTicket）+ otpCode 二次提交；首次绑定返回 totpSecret/totpUri（前端 qrcode 展示）；ResetAccountTotp 重置 |
| D8 | **多租户照抄** | 控制台侧表带 tenant_id（默认 1），JWT claims 透传；玩家侧表不加 tenant 列 |
| D9 | **接口信封例外（接口不能变的推论）** | admin 网关**不套** {code,msg,data} 统一信封：成功响应 = proto JSON 原样；错误体照 kratos 形状 `{"code": <http状态码>, "message": "...", "reason": "<错误码>"}`，HTTP 状态码同 server_v2（400/401/403/404…）。此为 CLAUDE.md 统一信封规则的**唯一例外**，在规范文档登记 |
| D10 | **i18n 双语（我们规范，不动业务与接口）** | 管理前端界面文案全量 i18n（zh-CN/en-US）；参考实现硬编码中文，i18n 是纯展示层包装 |

## 3. 功能范围映射（全量照抄，无裁剪）

| server_v2 console 功能 | server_v2 后端 | 我们落点 | 里程碑 |
|---|---|---|---|
| 登录/会话/TOTP/控制台账号 | identity | **ops-rpc**（console_accounts/sessions/TOTP） | M14a |
| 角色/权限/主体绑定/Authorize | authorization | **ops-rpc**（roles/role_permissions/principal_roles） | M14a |
| 权限目录/有效权限 | authorization | **ops-rpc** | M14a |
| 赏池管理（草稿/提请发布/审批/发布） | gacha | **draw-rpc**（池版本化+审批） | M14b |
| 玩法配置管理（挖宝/满车/扭蛋/连击/拼团） | gacha | **game-rpc**（版本化+审批，发布时物化到现有 *_configs 表） | M14b |
| 箱子创建/视图/库存/幸运指定 | gacha | draw-rpc + game-rpc（CreateBox/GetBox/GetBoxView/AssignLuckyUser） | M14b |
| 商品 SPU/SKU + 修订/发布 | catalog | **catalog-rpc 9316 激活**（product/sku/修订草稿/发布快照） | M14c |
| 活动草稿/发布 | campaign | **game-rpc**（campaign 版本化草稿/发布） | M14d |
| 券模板发布/发券/忠诚度默认值 | promotion | **game-rpc**（券模板版本化+grant+loyalty） | M14d |
| 条件政策（创建/修订/发布） | eligibility | **base-rpc**（eligibility_policies/drafts 已有表，补管理 RPC） | M14d |
| 风控案件列表/认领/解决 | risk | **base-rpc**（risk_cases 已有，补 ListRiskCases） | M14d |
| 支付路由策略（有接口无前端页） | payment | **payment-rpc**（照抄半成品：只做接口） | M14d |
| 任务进度列表 | task | **ops-rpc**（任务运行时未建，接口先就位返回空列表） | M14e |
| 任务计划（草稿/提请/审批/发布） | task | **ops-rpc**（task_plan 版本化+审批） | M14e |
| 报表看板（保存/发布/查询）+ 用户盈亏 | reporting | **ops-rpc**（dashboards/指标+聚合查询） | M14e |
| 风险策略规则配置 | risk | 参考实现无前端面（靠种子）→ 照抄：不做前端，仅种子 | — |
| engagement 内容管理 | engagement | 参考实现无 console 视图 → 照抄：无 | — |

## 4. 数据模型（新增表，全部幂等 DDL）

### 4.1 M14a — gacha_ops 控制台地基（`16_gacha_ops_admin.sql`）
- `console_accounts`（nocache）：id, tenant_id, username(uk), password_hash, nickname, status(1 启用/2 停用), totp_secret(NULL 未绑), created_at, updated_at
- `console_sessions`（nocache）：jti(uk), account_id, tenant_id, refresh_token_hash, expires_at, revoked_at(NULL), created_at；Redis 吊销键照抄 `console:session:revoked:<jti>`
- `roles`（nocache）：id, tenant_id, name, description, created_by, created_at, updated_at
- `role_permissions`（nocache）：role_id, resource, action, data_scope(SELF/TEAM/DEPARTMENT/ORGANIZATION/CUSTOM/ALL), approval_mode, conditions_json, allowed_fields, denied_fields —— (role_id, resource, action) uk
- `principal_roles`（nocache）：account_id, role_id —— uk(account_id, role_id)
- `ops_audit_logs`（nocache，我们的增强，不动参考功能）：id, tenant_id, account_id, operation_id, resource, action, target_id, request_id, detail_json, created_at

种子：admin/admin123（bcrypt，tenant_id=1）+ `super_admin` 角色（resource=* action=* DataScope=ALL）。

### 4.2 M14b — 赏池草稿→审批→发布（✅ 已交付，语义照抄 server_v2）
- draw 侧 `17_gacha_draw_m14b.sql`（幂等）：
  - `draw_pool_heads`（pool_id 分配器，AUTO_INCREMENT 起 1000000；next_version）
  - `draw_pool_versions`（pool_id+version 主键，status DRAFT/PUBLISHED，config_hash BINARY(32)，config_json 完整草稿，published_at）
  - `draw_pool_draft_requests`（idempotency_key 主键：同键同载荷重放返回既有草稿，同键异载荷 409）
  - `draw_pool_publication_approvals`（pool_id+version 主键，requester_id/approver_id/status PENDING→APPROVED；**单审批，审批人≠申请人**——server_v2 通用审批表为遗留未使用表，不实现）
  - 无 `box_instances`：server_v2 box-instance 概念本仓无对应物（createbox 操作不提供），幸运指派走 gashapon 配置
- 发布 = 域库**单事务**物化 draw_boxes（upsert 保 created_at）+ draw_pool_entries（整体替换，remaining=quantity）+ CAS 标记 PUBLISHED；已发布同哈希幂等返回。**无跨服务写 → 无 SAGA → 不需要管理侧 DTM**（原草案 gacha_dtm_admin + 第二实例已撤销，见 m14-service-division.md §3）
- game 侧幸运指派：M12 D4 的 AssignGashaponLuckyUser RPC 经管理网关开放 HTTP（POST /gacha/gashapons/:configId/lucky-assignment），无需新表；玩法配置（dig/full-car/gashapon 等）版本化+审批挪到 M14d 与 campaign/券同批（服务划分调整见 m14-service-division.md）

### 4.3 M14c — gacha_catalog 商品目录（`18_gacha_catalog_m14c.sql`，catalog-rpc 激活，✅ 已交付）
照抄 server_v2 catalog 表（列名照抄，SQL 文件号为 18 非 16）：`products`（code/title/status 1 DRAFT/2 ACTIVE/3 ARCHIVED/version）、`product_options`/`option_values`（规格）、`skus`（sku_code/售价/币种/履约模式）、`sku_selections`（SKU-选项值关联）、`product_revision_drafts`（草稿 blob，base_version/target_version CHECK target=base+1）、`product_published_snapshots`（发布快照，玩家只读已发布版）。
关键语义（照抄 server_v2 query.sql）：首发 `SET status=?,version=version+1`——**head 版本前进，首发后即 v2，首个快照是 v2，不存在 v1 快照**；ACTIVE 商品读快照、草稿商品读 live 表；变体修订草稿用 ValueID=0+ValueIndex 表示新规格值（发布时回填 AUTO_INCREMENT ID，区别于 server_v2 前置 Snowflake ID 的实现优化）；修订发布把被删 SKU 归档 status=3。

### 4.4 M14d — 活动/券版本化 + 支付路由（✅ 已交付）
- game 侧：`campaign_versions`（campaign_id, version, config_json, status）＋审批表（campaign 草稿/发布流）；`coupon_template_versions`＋审批表（券模板发布流）；`tenant_loyalty_defaults`（tenant_id uk, config_json）
- payment 侧：`payment_route_policies`（tenant_id, route_config_json, updated_by, updated_at）
- base 侧：无新表（eligibility_policies/drafts/rules、risk_cases 已有，只补 RPC）
- **已记录偏离（业务/接口不变，实现或半成品语义按 M14 裁决处理）**：
  1. **支付路由不做 provider registry 校验**（D6 半成品）：server_v2 ValidatePutPaymentRoute 里
     对候选 provider 的 registry 存在性/权重合法性校验依赖其支付通道注册表，本仓支付-rpc 无该
     注册表 → 仅做路由形状/枚举/权重/唯一性校验（core 单测 25 负例覆盖）；policies 表只存
     route_config_json 半成品字段，无独立 provider/candidate 列。
  2. **风控案件列表 actorId 过滤偏离**：server_v2 ListRiskCases 的 actor 过滤走其 IAM 服务
     按会话主体查询；本仓按会话主体 id 过滤（actorId=0 视为不限），业务语义对齐。
  3. **DDL 修复**：19_gacha_payment_m14d.sql 首版路由三表 id 漏 AUTO_INCREMENT，
     执行后 ALTER 在线表补齐（幂等 DDL 已改，历史库已修）。
  4. **restx.WriteProto json.Compact 归一**：protobuf v1.36.11 起 protojson 单行输出按
     **二进制哈希随机**在逗号后插空格（官方"输出不稳定"契约），每轮编译 wire 输出漂移，
     违反"接口不能变"逐字契约 → 管理网关写响应统一 json.Compact 归一，输出与 server_v2
     kratos 紧凑编码逐字一致（D9 的工程化补强）。

### 4.5 M14e — 任务计划 + 报表
- ops 侧：`task_plans`（plan_id, version, config_blob, status DRAFT/PUBLISHED）、`task_plan_publication_approvals`（plan_id, version, requester, approver, status, decided_at）
- ops 侧：`metric_definitions`（metric_key, name, aggregation, source）、`dashboards`（id, tenant_id, name, config_json, status DRAFT/PUBLISHED）、`dashboard_widgets`（id, dashboard_id, metric_key, title, position_json, params_json）
- **已记录偏离（业务/接口不变，实现按 M14 裁决处理）**：
  1. **看板 id 用 AUTO_INCREMENT**：server_v2 看板 id 为雪花；本仓 `dashboards.id`/
     `dashboard_widgets.id` 用 MySQL AUTO_INCREMENT 手工分配（20_gacha_ops_m14e.sql
     注释已记），接口契约不变（uint64 字符串）。
  2. **D8 补两码**：实现期新增 7024 TENANT_MISMATCH（403，会话租户与查询目标不符）
     与 7025 DATA_SCOPE_DENIED（403，SELF 数据范围拒绝跨主体查询），05-errors.md 已登记。
  3. **D9 修正 reporting.metric 只读**：实现中 reporting.metric 仅 read 一个 action
     （queryuserprofit 与 metrics 两端点都挂 read），设计 D9 的 {read,query} 修正为
     {read}——指标目录无创建/修改端点，adminop 目录同步只登记 read。
  4. **前端两刻意差异**（见 §5）：API 客户端手写 typed services（本栈无
     protoc-gen-typescript-http）；界面文案 i18n 双语（zh-CN/en-US，D10）。

## 5. 前端（`web_admin/` 新 Vite 项目，Vue3+TS+AntDV，与 server_v2 web/console 页面 1:1）

- 工程：抄我们 web/ 的工程配置（Vite+vue-tsc+i18n 双语）；路由/状态/API 客户端按 server_v2 console 结构（Pinia session store、动态权限路由 installDynamicRoutes、axios 封装、X-Request-Id 自动注入、401 → 清 Cookie 跳登录）
- 页面全量（~22 页）：登录（TOTP 两步验证+首绑二维码）、工作台、任务进度、任务计划（编辑器+双人审批）、用户盈亏、用户概览、SPU 详情/SKU 详情/商品管理、赏池管理/赏池详情（编辑器+提请发布+审批）、箱子详情（库存+幸运指定）、优惠券管理（模板列表/发布/发券/忠诚度）、活动、条件政策、权限管理（角色 CRUD/权限配置/成员）、报表看板、风控案件 + ConsoleLayout（侧边菜单/水印/退出）
- 与参考实现的两个刻意差异（都不动业务与接口）：界面文案 i18n 双语（D10）；API 客户端手写 typed services（我们栈无 protoc-gen-typescript-http，契约按 §6 手写）

## 6. 端点清单（照抄，路径与契约不变；operation_id/resource/action 同 server_v2）

**会话（ConsoleSessionHTTPService，public：login/refresh/logout/permissions）**
- POST `/console/v2/api/session/login`（public）· POST `/session/refresh`（public）· POST `/session/logout`（public）· GET `/session/permissions`（public）
- POST `/session/accounts`（OpenAccount，resource: identity.console_account, create）· POST `/session/accounts/{principal_id}/totp-reset`（reset_totp）

**权限（ConsoleAuthorizationService，resource: authorization.role）**
- GET `/authorization/permissions` · GET `/authorization/roles` · GET `/authorization/roles/{role_id}` · POST `/authorization/roles` · PUT `/authorization/roles/{role_id}` · PUT `/authorization/roles/{role_id}/permissions` · GET `/authorization/roles/{role_id}/members` · GET `/authorization/principals/{principal_id}/roles` · PUT `/authorization/principals/{principal_id}/roles`

**赏池/箱子（ConsolePoolHTTPService，resource: gacha.pool）**
- POST `/gacha/boxes`（createbox）· GET `/gacha/boxes/{box_id}`（getbox）· GET `/gacha/boxes/{box_id}/view`（getboxview）· POST `/gacha/boxes/{box_id}/lucky-assignment`（assignluckyuser）
- GET `/gacha/pools`（list）· POST `/gacha/pools/drafts`（create）· POST `/gacha/pools/{pool_id}/versions/{version}/publish`（publish）· POST `/gacha/pools/{pool_id}/versions/{version}/publication-request`（requestpublication）· POST `/gacha/pools/{pool_id}/versions/{version}/approval`（approvepublication）· GET `/gacha/pools/{pool_id}/versions/{version}`（get）

**商品（ConsoleCatalogHTTPService，resource: catalog.product）**
- GET `/catalog/products`（list）· POST `/catalog/products`（create）· GET `/catalog/products/{product_id}`（get）· POST `/catalog/products/{product_id}/publish`（publish）· POST `/catalog/products/{product_id}/revision`（saverevision）· POST `/catalog/products/{product_id}/variant-revision`（savevariantrevision）· GET `/catalog/products/{product_id}/revision`（getrevision）· POST `/catalog/products/{product_id}/revision/publish`（publishrevision）

**活动（ConsoleCampaignHTTPService，resource: campaign.definition）**
- POST `/campaigns/drafts`（createdraft）· POST `/campaigns/{campaign_id}/publish`（publish）· GET `/campaigns/{campaign_id}`（get）

**条件政策（ConsoleEligibilityHTTPService，resource: eligibility.policy）**
- GET `/eligibility/policies`（list）· GET `/eligibility/policies/{policy_id}/versions/{version}`（get）· POST `/eligibility/policies`（create）· POST `/eligibility/policies/{policy_id}/revisions`（createrevision）· POST `/eligibility/policies/{policy_id}/versions/{version}/publish`（publish）

**营销（ConsolePromotionHTTPService，resource: promotion.coupon_template/tenant_loyalty/user_coupon）**
- GET `/promotion/coupon-templates`（listcoupontemplates）· GET `/promotion/tenants/{tenant_id}/coupon-templates/{template_id}`（getcoupontemplate）· POST `/promotion/tenants/{template.tenant_id}/coupon-templates`（publishcoupontemplate）· POST `/promotion/tenants/{tenant_id}/coupon-grants`（grantcoupons）· GET `/promotion/tenants/{tenant_id}/loyalty-defaults`（getloyalty）· PUT `/promotion/tenants/{defaults.tenant_id}/loyalty-defaults`（putloyalty）

**风控（ConsoleRiskHTTPService，resource: risk.case）**
- GET `/risk/cases`（list）· POST `/risk/cases/{case_id}/claim`（claim）· POST `/risk/cases/{case_id}/resolve`（resolve）

**支付路由（ConsolePaymentRouteService，resource: payment.route）**
- GetPaymentRoute · PutPaymentRoute（路径照 payment_route.proto 注册）

**任务（ConsoleTaskHTTPService，resource: task.plan/task.progress）**
- GET `/tasks`（list）· POST `/task-plans/drafts`（createplandraft）· GET `/task-plans`（listplans）· GET `/task-plans/{version}`（getplan）· POST `/task-plans/{version}/publication-request`（requestplanpublication）· POST `/task-plans/{version}/approval`（approveplanpublication）· GET `/task-plans/{version}/approval`（getplanpublicationapproval）· POST `/task-plans/{version}/publish`（publishplan）

**报表（ConsoleReportingHTTPService，resource: reporting.metric/dashboard）**
- GET `/reporting/metrics`（metrics）· POST `/reporting/dashboards`（save）· GET `/reporting/dashboards/{dashboard_id}`（get）· POST `/reporting/dashboards/{dashboard_id}/publish`（publish）· POST `/reporting/dashboards/{dashboard_id}/query`（query）· GET `/analytics/users/{user_id}/profit-curve`（queryuserprofit）

**系统（ConsoleSystemService）**：GET `/console/v2/api/health`（public）

## 7. 内部里程碑

### M14a —— 控制台地基（会话/TOTP/RBAC + 管理网关 + 前端骨架）
1. `16_gacha_ops_admin.sql` + ops-rpc 9318 激活（模板抄 draw-rpc；Ed25519 私钥签发 JWT、Redis 吊销、TOTP RFC6238 + QR URI）
2. ops-rpc RPC 全集：Login/Refresh/Logout/OpenAccount/ResetAccountTotp/Permissions + PermissionCatalog/ListRoles/GetRole/CreateRole/UpdateRole/PutRolePermissions/ListRoleMembers/GetPrincipalRoles/PutPrincipalRoles + Authorize（六级 DataScope+字段投影+条件）+ RegisterOperations
3. admin 网关 `app_admin/gateway`：`pkg/api_admin/gateway.api`（§6 会话/权限端点，路径照抄）+ operation registry（fail-closed）+ 中间件链（X-Request-Id → JWT 本地验签 → Redis 吊销 → Authorize → 响应投影）+ kratos 兼容错误体（D9）
4. errorx 7000-7999 段登记 + 05-errors.md 记接口信封例外
5. 前端 web_admin/ 工程 + 登录（两步验证+首绑二维码）+ ConsoleLayout + 动态权限路由 + 权限管理页 + 账号管理页
6. **场景 24** ✅（已交付）：登录→TOTP 绑定→二次登录→权限目录→建角色/权限→建账号→绑角色→越权 403→吊销 401→审计留痕

### M14b —— 赏池草稿→审批→发布（✅ 已交付）
1. draw 侧池管理四表（4.2）+ 池 RPC 全集（CreatePoolDraft/ListPoolVersions/GetPoolVersion/RequestPoolPublication/ApprovePoolPublication/PublishPool/ListPublishedPools；CreateBox 不提供——box-instance 概念本仓无对应物，AssignLuckyUser 走 game-rpc 既有 RPC）
2. admin 网关 §6 赏池/箱子端点（9 路由，fail-closed M14b）+ promotion-admin 种子（gacha.pool/assign_lucky）
3. **场景 25** ✅：建草稿（缺 Idempotency-Key 400/同键重放幂等/同键异载荷 409）→提请（requester=会话主体）→自批 409→审批（单审批）→发布（物化 draw_boxes/entries，玩家 /boxes 断言）→重放幂等→未审批发布 409→幸运指派（ASSIGNED/CLEARED/非法奖 400）→权限负例 403→审计留痕。run_all 22 场景 ×2 全绿

### M14c —— 商品目录（catalog-rpc 激活，✅ 已交付）
1. catalog-rpc 9316 激活（4.3 表，业务域服务落 app/catalog/rpc——app_admin 只放纯管理服务 gateway/ops-rpc）+ 商品 RPC 全集（CreateProduct/GetProduct/ListProducts/PublishProduct/GetPublishedProduct/SaveProductRevision/SaveProductVariantRevision/GetProductRevision/PublishProductRevision，内部实现优化但契约与语义照抄）
2. admin 网关 §6 商品端点（8 路由，fail-closed M14c）+ errorx 4001-4003（404 PRODUCT_NOT_FOUND/409 PRODUCT_CONFLICT/400 PRODUCT_INVALID）+ console.catalog.* 权限点/审计；前端商品管理/SPU 详情/SKU 详情页随 M14e web_admin 整批交付
3. **场景 26** ✅：建 SPU+规格+SKU（校验负例 400）→草稿 live 读→游标分页→首发 v2（重放幂等/版本不符 409）→修订（同内容幂等/异内容 409）→发布修订 v3→变体修订（新增规格值+新 SKU、删旧 SKU）→发布 v4（被删 SKU 归档）→SQL 终态断言（3 快照/草稿删除/归档）→权限负例 403→审计留痕。run_all 23 场景 ×2 全绿

### M14d —— 活动/条件政策/券/风控/支付路由（✅ 已交付）
1. game-rpc：campaign 草稿/发布（版本化）+ 券模板发布流 + GrantCoupons + loyalty defaults；base-rpc：eligibility 管理 RPC（list/get/create/revision/publish）+ ListRiskCases；payment-rpc：Get/PutPaymentRoute（照抄半成品，只做接口无前端页）
2. admin 网关 §6 对应端点 + 前端活动/条件政策/优惠券管理/风控案件页（页面随 M14e web_admin 整批交付）
3. **场景 27** ✅：活动草稿→发布→玩家参与生效→v2 改版发布（草稿叠草稿 409/重复发布 409）；券模板 v1/v2 发布→发券幂等/冲突→用户持有+忠诚度默认值；资格政策修订 CAS→发布→evalpol 判定翻转+旧版 RETIRED；风控案件游标翻页 list/claim/resolve 状态机+幂等；支付路由 CAS 全替换+审计；权限负例 403+审计 ≥25 行。standalone 全绿；run_all 24 场景 ×2 全绿（边界门通过，含场景 27 风控游标抗污染修复：步骤 9 播种前清非测试案件——转盘抽奖风控评估会真实落 OPEN 案）

### M14e —— 任务计划 + 报表 + 收尾（✅ 已交付）
1. ops-rpc：task_plans 版本化+审批（草稿/提请/审批/发布）+ tasks 进度列表（空数据就位）；reporting：metric_definitions/dashboards 保存/发布/查询 + 用户盈亏曲线（聚合 user/draw 库只读 SQL）
2. admin 网关 §6 任务/报表端点 + web_admin 前端 22 页整批交付（任务进度/任务计划编辑器+审批/报表看板/用户概览/盈亏曲线/商品/赏池/箱子/券/活动/条件政策/权限管理/风控案件等，手写 typed services + i18n 双语）
3. **场景 28** ✅：任务计划草稿 v2（expectedLatestVersion 过期 409）→ v3 Get/List → 提请（伪造 requester 被会话主体覆盖、同参重放 200）→ B 审批（A 自审 409）→ 发布（expectedConfigHash CAS）→ structured reward v4 NOT_READY 400；ListTasks（userId 缺 400/空进度/游标两页/period 过滤空）；看板 v1 保存（widget 重叠 400）→ CAS v2 → 未发布查询 400 → 发布（版本不符 409×2）→ 查询 2 结果 points 空 + cacheHit + SELF 403；盈亏曲线空态 {} → 非法粒度 400 → 时间窗 400 → tenant 不符 403；权限负例 403 + 审计 14 类操作落库。run_all 25 场景 ×2 全绿
4. 收尾：`go build ./...`、`go test`（ops/draw/game/base/payment/catalog 核心包）、`npm run build`（web + web_admin 含 vue-tsc）、§4.5 偏离记录、提交（Co-Authored-By，meta 排除清单）、更新 memory

## 8. 验证（验收标准）

1. 每边界 `bash test/run_all.sh` 全绿；终态 25 场景连跑两遍全绿
2. `go build ./...`、`go test ./app_admin/ops/rpc/internal/... ./app/draw/rpc/internal/core/... ./app/game/rpc/internal/core/... ./app/base/rpc/internal/core/... ./app/catalog/rpc/internal/core/... ./app/payment/rpc/internal/core/...`、`npm run build`（web + web_admin，含 vue-tsc）全绿
3. 冒烟：admin 登录（TOTP 两步）→ 建角色/账号 → 赏池草稿→审批→发布 → 玩家侧抽奖生效 → 越权 403 → 审计留痕 → 盈亏曲线渲染
4. dev 走查管理端全部页面（~22 页），i18n 双语；玩家侧回归无影响

## 9. 风险与边界

1. 管理 RPC 与玩家 RPC 共存：管理 RPC 命名带 Admin 前缀（RPC 契约形状照抄 server_v2，服务内方法名可调）；玩家网关不挂管理路由；admin 网关路由手写（禁跑 goctl gateway 的老规矩不变，新增生成目标 `ops`/`catalog`/`admin-gateway`）
2. 配置发布物化：审批通过→发布 = 事务内写现有配置表 + 删缓存（「事务内删缓存 → commit 后延迟双删」既有约定），玩家侧缓存模型生效
3. Authorize 每请求一次 RPC：ops-rpc 内权限结果短 TTL 缓存 + 角色/权限变更失效
4. 审计日志异步写：不阻塞主请求，尽力而为
5. admin 网关无状态不连 DB（网关铁律）：数据全部经 ops-rpc/业务 RPC；本地验签只需公钥配置（D3）
6. 种子密码 admin123 仅本地开发；生产文档注明首登改密
7. TOTP 依赖 `github.com/pquerna/otp`（web 前端 qrcode 已具备依赖）；密钥不入库明文——totp_secret 加密存储

## 10. 实施顺序

1. 设计评审（本文档）→ 用户确认
2. M14a：SQL → proto_admin → goctl 生成 ops + admin-gateway → logic → 网关中间件链 → 前端骨架 → 场景 24 → run_all 绿
3. M14b：draw/game 版本化表 + 管理 RPC → 网关端点 → 前端赏池/箱子页 → 场景 25 → run_all 绿
4. M14c：catalog-rpc 激活 → 网关端点 → 前端商品页 → 场景 26 → run_all 绿
5. M14d：game/base/payment 管理 RPC → 网关端点 → 前端活动/政策/券/风控页 → 场景 27 → run_all 绿
6. M14e：ops 任务计划/报表 → 网关端点 → 前端任务/报表页 → 场景 28 → 全量边界门 → 提交 → memory
