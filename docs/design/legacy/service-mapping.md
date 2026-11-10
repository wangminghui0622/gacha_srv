# 老项目（server_v2）服务 ↔ 新仓服务对照表

> 老项目 = `F:\workspace\src\server_v2`（Kratos 微服务，21 个进程：17 个 service + engine/draw + gateway/interface + gateway/console + worker/push）。
> 新仓 = `C:\Users\Administrator\Desktop\Gacha`（go-zero + gRPC + Etcd + DTM SAGA，8 个 RPC 服务 + 2 个网关 + 1 个 DTM 实例）。
> 裁决依据（用户原话）："他怎么做的，我们就怎么做，业务功能不要动，服务可以调整，接口不能变"。
> 配套文档：[m14-service-division.md](m14-service-division.md)（控制台功能划分）、[m14-admin-console.md](m14-admin-console.md)（管理控制台设计）。

**一句话结论**：老项目 21 个进程归并成新仓 11 个进程（8 RPC + 2 网关 + DTM）；17 个业务服务 + 抽奖引擎中 12 个完整克隆（含 wallet 并入 user-rpc、engine 并入 draw-rpc、task 两端分落 ops-rpc/game-rpc、integration 全套）、3 个部分克隆（identity/gacha/inventory）、3 个不克隆（delivery/engagement/trade）+ worker/push 不克隆；2 个网关一一对应。**不克隆 ≠ 老项目没做**：trade/delivery/engagement/inventory 在老项目有实现但未接线（两网关无端点）。**§3 A 类五项（2026-10-24 用户拍板）全部已交付（M15a-M15d）**：手机号/微信登录 → user-rpc（M15a）、MemoryMatch → game-rpc（M15b）、task 玩家体系 → game-rpc（M15c）、integration → thirdparty-rpc 9317 激活（M15a 短信验证码+微信登录；M15d 微信小程序全家 + 支付宝 OAuth + 腾讯广告 + provider 指标）；ClaimPrize 维持 SAGA 自动发奖（业务终点一致，记为架构偏离，不做仓库）。

## 1. 总表：server_v2 服务 → 新仓归属

| server_v2 服务 | 老项目职责 | 新仓归属 | 状态 |
|---|---|---|---|
| identity | 玩家会话/注册/登录、设备密钥、手机号、第三方登录（identity/phone/third_party/user_session）；控制台会话/TOTP 两步登录（console_session） | **拆分**：玩家注册/登录/会话 → **user-rpc**；控制台会话/TOTP/账号 → **ops-rpc**；手机号/微信登录 → **user-rpc** + **thirdparty-rpc**（M15a 已交付）；设备密钥 → 未克隆（范围选择，本仓无设备绑定场景） | ⚠️ 部分克隆 |
| authorization | Authorize（RBAC/CEL/数据范围/字段投影）、权限目录、角色、审计 | **ops-rpc** :9318（M14a 已交付） | ✅ 已克隆 |
| campaign | 活动（签到/兑换码/转盘/活动宝箱）草稿→发布→参与 | **game-rpc** :9315（玩家面 M11，控制台 M14d） | ✅ 已克隆 |
| catalog | SPU/SKU/修订版/快照（商品目录） | **catalog-rpc** :9316（M14c 已交付） | ✅ 已克隆 |
| delivery | 实物发货：创建运单/打包/出库/物流追踪 | 未克隆（老项目有实现但玩家面无端点、仅 kuaidi100 回调接线，见 §3 B 类） | ❌ 不克隆 |
| eligibility | 资格策略（CEL 表达式）草稿/修订/发布 + 运行时评估 | **base-rpc** :9314（玩家面 M7-M10，控制台 M14d） | ✅ 已克隆 |
| engagement | 内容运营（公告等）、推送 token/推送队列/互动埋点 | 未克隆（老项目有实现但两网关均无端点，见 §3 B 类） | ❌ 不克隆 |
| gacha | 抽奖执行/领奖、盒与池（草稿→审批→发布）、滚动中奖 feed、满车、扭蛋、连击链、拼团抽、活动宝箱、幸运指派、记忆翻牌（MemoryMatch） | **拆分**：抽奖执行/池控制台/滚动 feed → **draw-rpc** :9312（滚动 M12a，池控制台 M14b）；满车/扭蛋/连击链/拼团/活动宝箱/幸运指派 → **game-rpc** :9315（M12b-c/M13）；记忆翻牌 → **game-rpc**（M15b 已交付，m15b-memorymatch.md）；独立领奖（ClaimPrize）→ 不实施（SAGA 自动发奖，架构偏离，见 §3 A 类） | ⚠️ 部分克隆 |
| integration | 验证码、微信小程序（token/手机号/二维码）、支付宝 OAuth、广告回调、provider 指标数据接入 | **拆分**：短信验证码/微信 code 交换 → **thirdparty-rpc** :9317（M15a 已交付短信+微信）；**M15d 已交付（2026-10-25）微信小程序全家（token/手机号/二维码/urllink）+ 支付宝 OAuth + 腾讯广告 + provider 指标**（kuaidi100 承接方 delivery 未克隆留占位） | ✅ 已克隆 |
| inventory | 资产发放（GrantAsset）、预约/预留（Reserve/Commit/Release）、提货码（PickupCode） | **拆分**：资产发放 → **user-rpc**（GrantAsset/GrantAssetRevert，SAGA 06 分支发奖）；预约/预留/提货码 → 未克隆（老项目有实现但两网关均无端点，见 §3 B 类） | ⚠️ 部分克隆 |
| payment | 支付授权/捕获/退款 + 渠道路由配置 | **payment-rpc** :9313（玩家面 M7，路由控制台 M14d） | ✅ 已克隆 |
| promotion | 报价（QuoteCheckout/Lock/Release/Commit）+ 优惠券 + 忠诚度默认值 | **game-rpc** :9315（报价+券 M7-M10；模板发布/发券/忠诚度 M14d——券表本就在 gacha_game，数据跟着域走） | ✅ 已克隆 |
| reporting | 指标目录/看板（保存/发布/查询）/盈亏曲线（跨域只读聚合） | **ops-rpc** :9318（M14e，开只读多库连接聚合，见 m14-service-division.md §4） | ✅ 已克隆 |
| risk | 风控评估 + 案件状态机（认领/处理） | **base-rpc** :9314（玩家面 M7-M10，控制台 M14d） | ✅ 已克隆 |
| task | 玩家任务体系（任务/签到/通行证/会员）+ 控制台任务计划（cron 调度） | **拆分**：控制台任务计划（草稿→提请→审批→发布）→ **ops-rpc**（M14e）；玩家任务体系 → **game-rpc**（M15c 已交付，m15c-task-player.md） | ✅ 已克隆 |
| trade | 玩家间交易：挂单/报价/接受/撤销 | 未克隆（老项目有实现但两网关均无端点，见 §3 B 类） | ❌ 不克隆 |
| wallet | 资金：Credit/CreateHold/PostHold/VoidHold/账本 | **user-rpc** :9311（GetBalance/Recharge/CreditWallet/FreezeWallet/PostHold/VoidHold/GetWallet，M7——钱包并入用户服务，未单开 wallet 服务） | ✅ 已克隆（并入） |
| engine/draw | 抽奖引擎（独立进程，确定性抽奖内核） | **并入 draw-rpc** :9312（SettleDraw + internal/core 确定性内核；不开独立引擎进程） | ✅ 已克隆（并入） |
| gateway/interface | 玩家接口网关（HTTP） | **app/gateway** :9310（玩家网关 + DTM SAGA 编排逻辑） | ✅ 已克隆 |
| gateway/console | 控制台网关（HTTP，/console/v2/api） | **app_admin/gateway** :9319（admin 网关，fail-closed 路由校验） | ✅ 已克隆 |
| worker/push | 推送 worker | 未克隆（老项目有实现但随 engagement 未接线，见 §3 B 类） | ❌ 不克隆 |

## 2. 归并与拆分说明（服务调整，接口不变）

1. **wallet 并入 user-rpc**：本仓玩家注册/登录（identity 玩家面）与资金账户都属"用户域"，合并进 gacha_user 单库，少一个服务全套基建；server_v2 wallet.proto 的 RPC 形状全部保留在 user.proto。
2. **authorization + console_session + task 控制台 + reporting → ops-rpc**：server_v2 的 authorization 服务同构于本仓 ops-rpc（平台层）；task/reporting 两个"管理平台能力"归并理由见 m14-service-division.md §4（报表本身就是跨域只读聚合，任务计划执行器调任意域 RPC）。task 玩家执行端（任务/签到/通行证/会员）属玩家域 → game-rpc（M15c）。
3. **campaign + promotion → game-rpc**：两类表在 M8/M11 已建在 gacha_game，拆服务要跨库搬表，违背"数据跟着域走"。
4. **eligibility + risk → base-rpc**：两类表已在 gacha_base，运行时 RPC（EvaluatePolicy/RiskEvaluate/Claim/Resolve）已在 base.proto，控制台 CRUD 自然同服务。
5. **gacha 拆分 draw-rpc + game-rpc**：抽奖/盒/池/滚动（gacha_draw 域库）与玩法（满车/扭蛋/连击链/拼团/活动宝箱，gacha_game 域库）分库分服务；幸运指派按 M12 D4 走 game-rpc 的 AssignGashaponLuckyUser。
6. **engine/draw 不开独立进程**：抽奖确定性内核以 draw-rpc internal/core 纯函数库形式落地（server_v2 的 kuji_kernel/full_car/gashapon 判定逻辑同款移植 + 单测）。
7. **approval 通用表不实现**：server_v2 authorization 的 approval_request/action 为遗留未使用表（已查证），发布审批在域库单事务完成，无管理侧 SAGA（m14-service-division.md §3）。

## 3. 未克隆清单（及老项目实际状态，已逐服务核实源码/网关接线）

分三类——**"老项目没做"不是主要理由**：多数服务老项目都有实现，区别只在有没有接线到两个网关。

### A 类：老项目完整实现且已接线（✅ 2026-10-24 用户拍板全部补克隆，M15a-M15d 已全部交付）

| server_v2 功能 | 老项目状态 | 新仓归属（2026-10-24 已定） | 注记 |
|---|---|---|---|
| task 玩家任务体系（任务/签到/通行证/会员） | 1215 行实现+374 行测试；玩家网关有 5 个端点（/v2/api/tasks 列表、签到、领任务、通行证、会员） | **game-rpc** :9315（gacha_game） | 老项目 task 一个服务管两端：控制台计划已在 ops-rpc（M14e），玩家执行端落 game-rpc（玩家域在 app/、平台域在 app_admin/）；每日签到与 campaign 签到类活动是老项目两个独立功能，克隆时分开；**M15c 已交付（2026-10-25）**，见 m15c-task-player.md |
| gacha MemoryMatch（记忆翻牌） | 玩家网关全套 11 个端点（购买/外观/命运/回合/配对/完成/跳过）+ biz 实现 | **game-rpc** :9315（gacha_game） | 盒玩法，套 M12 自包含玩法模式（config/state/records + 专用 SAGA + 网关端点 + 前端页）；**M15b 已交付（2026-10-25）**，见 m15b-memorymatch.md |
| gacha ClaimPrize（抽中→仓库→手动领奖） | 玩家网关有 /v2/api/gacha/prizes/claim 端点 | **不实施**：维持 SAGA 自动发奖 | 用户拍板（2026-10-24）：业务终点一致（玩家最终拿到奖品），记为架构偏离；若未来要仓库体验，背包+领奖 RPC 落 user-rpc，届时改全部玩法 SAGA 的 06 分支 |
| identity 手机号/微信登录 | 玩家网关有 3 个登录端点（验证码/手机号/微信），integration 支撑 | **user-rpc** :9311（账号/会话）+ **thirdparty-rpc** :9317（发码/微信接口） | 老项目 identity 管账号、integration 管渠道，结构照旧；**M15a 已交付（2026-10-24）**，见 m15a-auth-thirdparty.md |
| integration 全套（验证码/微信小程序/支付宝 OAuth/腾讯广告/kuaidi100） | alipay/sms/tencentad/wechat/kd100 子包 + service 层全实现（含测试） | **thirdparty-rpc** :9317 激活（6000 段，M14c catalog 同款激活模式） | **M15a 已交付短信+微信（2026-10-24）**；**M15d 已交付（2026-10-25）微信小程序全家（token/手机号/二维码/urllink）+ 支付宝 OAuth（userinfo 同老项目空 stub）+ 腾讯广告 + provider 指标接入**（gacha_thirdparty 库 5 表 + gacha_ops 报表 3 表，outbox+轮询分发→ops 直连，无 Kafka），见 m15d-integration-channels.md；kuaidi100 承接方 delivery 未克隆留占位 |

### B 类：老项目有实现但未接线（两个网关都无端点——老项目自己也没做成可用业务）

| server_v2 服务 | 老项目状态 |
|---|---|
| trade（玩家间交易） | 327 行实现+205 行测试（含过期清扫器），但玩家网关与控制台网关均无端点 |
| inventory（预约/预留/提货码） | 134 行实现+209 行测试，两网关均无端点（gacha 服务内另有 inventory_grant 发奖逻辑，已克隆为 SAGA 发奖） |
| engagement（内容/推送/互动） | 213 行实现+75 行测试，两网关均无端点 |
| delivery（创建运单/打包/发货） | 183 行实现+149 行测试，玩家面无业务端点；仅 kuaidi100 物流回调在玩家网关接线（InterfaceCallbackService） |
| worker/push（推送 worker） | 9 个 Go 文件独立 worker，无 HTTP 面，随 engagement 一起未接线 |

### C 类：老项目真没做

- 无——未克隆的服务全部有实现代码（proto 定义的 RPC 都有 service 层实现）。

## 4. 反向索引：新仓服务 → 老项目来源

| 新仓服务 | 端口 | 数据库 | 错误码段 | 来源（server_v2） |
|---|---|---|---|---|
| gateway（玩家网关） | 9310 | 无（无状态） | — | gateway/interface + 玩家 SAGA 编排（自研） |
| user-rpc | 9311 | gacha_user | 1000-1999 | identity（玩家会话，M15a 加手机号/微信登录）+ wallet + inventory（GrantAsset） |
| draw-rpc | 9312 | gacha_draw | 2000-2999 | gacha（抽奖/盒/池控制台/滚动 feed）+ engine/draw（确定性内核） |
| payment-rpc | 9313 | gacha_payment | 8000-8999 | payment |
| base-rpc | 9314 | gacha_base | 5000-5999 | eligibility + risk |
| game-rpc | 9315 | gacha_game | 3000-3999 | campaign + promotion + gacha（满车/扭蛋/连击链/拼团/活动宝箱/幸运指派/记忆翻牌 M15b）+ task（玩家任务体系 M15c） |
| catalog-rpc | 9316 | gacha_catalog | 4000-4999 | catalog |
| thirdparty-rpc（M15a 已激活） | 9317 | gacha_thirdparty（M15d 建库：指标 4 表含 outbox + 广告点击表） | 6000-6999 | integration（M15a 短信+微信；M15d 微信小程序全家+支付宝 OAuth+腾讯广告+provider 指标，kuaidi100 留占位） |
| ops-rpc | 9318 | gacha_ops | 7000-7999 | authorization + identity（console_session）+ task（控制台任务计划）+ reporting |
| admin gateway | 9319 | 无（无状态） | — | gateway/console |
| dtm | 36789/36790 | gacha_dtm | — | 无对应（自研 SAGA 基础设施，见 §5） |

## 5. 本仓自研基础设施（老项目没有）

- **DTM SAGA**（app/dtm 单实例 36789/36790 + gacha_dtm 库）：老项目 Kratos 无分布式事务组件；本仓所有购买类流程（抽盒/挖宝/满车/扭蛋/连击/拼团/记忆翻牌）为 7 分支 SAGA（策略→风控→锁定→冻结→结算→发奖→捕获），带屏障表 + 幂等键 + 补偿。
- **errorx 统一错误码段**：按服务总类序号 ×1000 分段（见上表），全仓登记于 docs/standards/backend/05-errors.md；DTM SAGA 分支用 status.Error(Aborted) 触发补偿，非 SAGA RPC 用 errorx.NewCodeError。
- **管理网关 fail-closed 启动校验**：admin gateway 启动时校验路由 ↔ pkg/adminop/operations.go 目录双向一致（老项目无此机制）。
- **双网关**：玩家 9310 与管理 9319 分离（老项目同为两个 gateway，结构一致）。
