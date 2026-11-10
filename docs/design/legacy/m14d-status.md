# M14d 进展状态表（随实施推进更新）

> 更新于 2026-10-23。M14d = 管理控制台五域：活动版本化 / 条件政策 CEL / 风控案件状态机 /
> 券模板+发券+忠诚度默认值 / 支付路由存管。设计决策见 [m14d-console.md](m14d-console.md)。

## 一、总体里程碑

| 里程碑 | 内容 | 状态 |
|---|---|---|
| M7-M11c | 资金/支付/资格风控/券报价/活动（签到/兑换/转盘/活动宝箱）/挖宝 | ✅ 已提交全绿 |
| M12a-c | 滚动中奖 + 满车 + 扭蛋（场景 18-21） | ✅ 已提交全绿 |
| M13 | 连击链 + 拼团抽（场景 22-23） | ✅ 已提交全绿 |
| M14a | 管理网关 + 运营服务（身份/授权/审计，场景 24） | ✅ 已提交全绿 |
| M14b | 赏池草稿→审批→发布控制台（场景 25） | ✅ 已提交全绿 |
| M14c | 商品目录控制台 catalog-rpc 9316 激活（场景 26） | ✅ 已提交（95c5fce） |
| M14d | 控制台五域（场景 27，run_all 23→24 场景） | ✅ 完成（run_all ×2 全绿，随本批提交） |
| M14e | 任务计划+调度+报表聚合（ops-rpc）+ web_admin 前端全量 + 场景 28 | ✅ 已交付（run_all 25 场景 ×2 全绿，随本批提交） |

## 二、M14d 任务清单

| # | 任务 | 状态 |
|---|---|---|
| 1 | 设计文档 m14d-console.md（D1-D10 决策） | ✅ 完成 |
| 2 | errorx 新码（3035-3037 / 5006-5009 / 8003-8005）+ 05-errors.md 登记 + restx MapError 映射 | ✅ 完成 |
| 3 | DDL 三库：19_gacha_game_m14d.sql（campaign 版本化 + 券模板版本）、19_gacha_base_m14d.sql（政策重建 CEL + 风控状态机）、19_gacha_payment_m14d.sql（路由三表）；已执行 + mysql 验证 | ✅ 完成 |
| 4 | proto 扩展：game（活动+券，照抄 server_v2 campaign/promotion 形状）、base（Claim/Resolve 换 server_v2 risk 形状 + 政策 eligibility 形状 + 案件列表）、payment（路由 payment_route 形状）→ goctl.sh game/base/payment 三生成 | ✅ 完成 |
| 5 | game-rpc：campaignConsole 核心（console↔player 配置转换 + 校验 + 单测）→ 活动 3 RPC（CreateCampaignDraft/PublishCampaign/GetCampaign）+ 券 6 RPC（PublishCouponTemplate/GetCouponTemplate/ListCouponTemplateVersions/GrantCoupons/GetTenantLoyaltyDefaults/PutTenantLoyaltyDefaults）+ kind 常量 3→13 + 新表模型接入 → 重启 game-rpc | ✅ 完成（core 单测绿、模型重生成取新列、repo 层 + 9 logic、wheelTx 多奖品/日库存、campaigns/券模板缓存失效 DeleteCache、种子 kind 13、go vet 干净、game-rpc 已重启） |
| 6 | base-rpc：cel-go 依赖 + eligibilityConsole 核心（config_hash 纯函数 + 单测）+ evaluatePolicyLogic CEL 重写 + 政策 5 RPC（CreatePolicyDraft/CreatePolicyRevision/GetPolicy/ListPolicies/PublishPolicy）+ ListRiskCases + 新 Claim/Resolve（旧两 logic 已编译失败待重写）+ 场景 8 种子改新三表 → 重启 base-rpc | ✅ 完成（cel-go v0.21.0、core 单测绿、CEL 引擎场景解析按 published_at 取最新 PUBLISHED、required_facts 缺失→MISSING_REQUIRED_FACT、ALL/ANY+failure_code 进 reason；risk 状态数值化 1/2/3/4 + risk_score severity 映射 + action=3 + 事件表幂等重放；Claim/Resolve/List 冒烟全过；场景 8/9 单独跑绿；base-rpc 已重启 9314） |
| 7 | payment-rpc：route 三表模型 + Get/PutPaymentRoute（CAS+审计）→ 重启 payment-rpc | ✅ 完成（core ValidatePutPaymentRoute 照抄 server_v2 校验 minus registry + 25 负例单测；repo 三行型 + FOR UPDATE 行锁 + UpdateRoutePolicyCAS + 候选全删全插 + 审计；logic 新建 v1/更新 CAS 8004/首建 old_snapshot NULL；DDL 修 id 缺 AUTO_INCREMENT 并 ALTER 在线表；冒烟 8 项全过——8003/建/查/改 v2/8004/8005×2；冒烟数据已清；payment-rpc 已重启 9313） |
| 8 | admin 网关：desc 5 文件（campaign/eligibility/risk/promotion/payment）→ goctl admin-gateway → svc 客户端 + FailClosedMilestone M14d + adminop 19 操作 + 19 handler/logic → 重启 admin gateway | ✅ 完成（goctl 保留手写 handler 已验证；BaseRpc/PaymentRpc 客户端 + yaml FailClosedMilestone M14d；19 handler/logic 手写照抄 server_v2 契约——claim/resolve OperationId 取 Idempotency-Key 头、PublishCouponTemplate body 即 template、PutLoyalty body 即 defaults、risk List actorId 可选默认 0=all（偏离记录见下）；adminop 2 处路径对齐 server_v2；go vet 绿 + 重启 admin gateway fail-closed 通过；HTTP 冒烟 33/33——19 路由全接线 + 401/400/404/409 映射 + payment 全流程 CAS/审计经网关贯通；冒烟抓出并修复 2 个网关注入 bug：policy.tenantId=1 与 policy.channel/scene 注入；现场已清理） |
| 9 | 场景 27（五域全流程 + 权限负例 + 审计断言）+ run_all.sh/menu.sh 注册 | ✅ 完成（12 步五域全链路：403 权限负例、活动草稿→发布→改版→重复发布 409、转盘多奖品/日库存、兑换码、条件政策 CEL+CAS 版本切换+RETIRED+evalpol 工具直连判定、券模板版本+幂等+冲突+发券+忠诚度默认值、风控案件游标翻页+认领 409+解决+幂等重放、支付路由 CAS+全替换+审计、ops 审计 ≥25 行；run_all 24 场景、menu 27 项；standalone 全绿） |
| 10 | 边界门：run_all ×2 全绿、go build ./...、go test 核心包（game/base/payment core）、npm run build | ✅ 完成（run_all 24 场景 ×2 全绿；go build ./...、go test -count=1 game/base/payment/draw core、npm run build 全绿） |
| 11 | 文档更新（m14-admin-console.md §4.4/§7）+ memory 更新 + 提交（Co-Authored-By，排除 meta/ 清单） | ✅ 已提交（8b3f16f） |

## 三、当前工作树状态

- 已改未提交（M14d 全部落地文件）：
  - pkg/proto/{game,base,payment}.proto + 三服务 goctl 生成代码 + pkg/utils/errorx/errorx.go
  - app/game/rpc：internal/core/{campaignConsole,couponConsole}.go + 单测、internal/repo/{campaignConsole,couponConsole}.go、
    9 个 logic、campaignOps.go（kind 13）、campaignParticipateLogic.go（wheelTx 多奖品/日库存）、
    pkg/model/gacha_game/{campaigns,coupon_templates}_model.go（DeleteCache）
  - app/base/rpc：internal/core/{eligibilityConsole,riskConsole}.go + 单测、evaluatePolicyLogic.go CEL 重写、
    政策 5 logic + ListRiskCases + 新 Claim/Resolve、场景 8 种子改新三表
  - app/payment/rpc：route 三表模型 + repo（FOR UPDATE + CAS + 审计）+ 2 logic + internal/core ValidatePutPaymentRoute
  - app_admin/gateway：internal/restx/restx.go（M14d 错误映射 + **WriteProto json.Compact 归一**，见下）、
    svc 客户端 + FailClosedMilestone M14d + adminop 19 操作 + desc 5 文件 + goctl admin-gateway 生成 + 19 handler/logic
  - test/scenario_27_console_m14d.sh（新增）+ test/tools/evalpol/main.go（新增）+ run_all.sh/menu.sh（场景 27 注册 + 计数标签修正）
  - script/split/19_gacha_{game,base,payment}_m14d.sql（新增，现并入 06/07/05 按库终态快照）+ 13_gacha_game_m11.sql（转盘种子 kind 3→13，现并入 07_gacha_game.sql）
  - docs/design/{m14d-console.md,m14d-status.md} + docs/standards/backend/05-errors.md + docs/design/m14-admin-console.md
- **restx WriteProto json.Compact 归一（重要修复）**：protobuf v1.36.11 protojson 单行输出按
  **二进制哈希随机**在逗号后插空格（官方"输出不稳定"契约），每次重新编译都会漂移 →
  场景 24 的紧凑 grep 断言随编译翻绿翻红。json.Compact 归一后 wire 输出逐字稳定，
  与 server_v2 kratos 紧凑输出一致。ConfigHashOf（Deterministic 二进制 proto SHA256）与
  couponTemplateContentEqual（字段比较）不受影响；player 网关用 encoding/json 不受影响。
- 后台运行中：player gateway、game/base/payment/draw/catalog/ops 五 rpc、admin gateway
  （均已重启至 M14d 代码；admin gateway 已用新 restx 重启并现场验证紧凑输出）
- .smoke_risk/ 为一次性冒烟脚本目录，**不得提交**

## 四、M14e 交付记录（2026-10-24，✅ 已交付）

设计决策见 [m14e-console.md](m14e-console.md)；偏离记录见 [m14-admin-console.md](m14-admin-console.md) §4.5。

- **后端**：ops.proto task/reporting 14 RPC（goctl ops 生成）；ops-rpc internal/core/{planConsole,dashboardConsole} 校验+hash 纯函数（单测绿）+ 14 logic + repo（task 三表 bootstrap/versioned/CAS + reporting 五表/看板物化/聚合查询 + 盈亏曲线聚合 SQL）；errorx 7016-7025（含实现期新增 7024 TENANT_MISMATCH/7025 DATA_SCOPE_DENIED）+ 05-errors.md 登记；admin 网关 task/reporting 9 路由（fail-closed M14e）+ adminop 14 操作；场景 28（测试+run_all/menu 注册，run_all 23→25 场景计数同步）。
- **前端 web_admin（22 页整批交付）**：工程/路由/状态/访问控制/布局/共享组件 + 登录（TOTP 两步+首绑二维码）、工作台、任务进度、任务计划（编辑器+审批）、用户盈亏（echarts 曲线）、用户概览、商品管理/SPU/SKU、赏池管理/详情/编辑器（审批流）、箱子详情（幸运指定）、优惠券管理、活动、条件政策、权限管理、报表看板、风控案件；手写 typed services（10 个 api 模块）+ i18n 双语（zh-CN/en-US）全量。
- **边界门**：run_all 25 场景 ×2 全绿；`go build ./...` 绿；`go test ./app/game/rpc/internal/core/... ./app/draw/rpc/internal/core/... ./app_admin/ops/rpc/internal/core/...` 绿；`npm run build`（web + web_admin 含 vue-tsc）绿。
- **收尾**：m14-admin-console.md §4.5 偏离 + §7 标 ✅；提交（Co-Authored-By，排除 meta/ 清单，.smoke_risk/ 不提交）；memory 更新。

## 五、遗留（M14e 之后）

- 待用户指定下一里程碑（参考 server_v2 盘点中尚未克隆的功能：控制台账号管理页、玩家侧运营位等）。
