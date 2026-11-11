# CLAUDE.md — 工程规范总纲

> 本文件由标准包生成。所有团队成员与 AI 助手共同遵守。
> 个人偏好（不入库）写 `CLAUDE.local.md`。
> 语言相关规则在 `.claude/rules/go.md`、`.claude/rules/web.md` 中（按文件路径自动加载）；详细规范在 `docs/standards/` 下，按需查阅。
> **标准包源文件（install.sh/rules/skills/specs/templates/settings 模板/本文件副本）已统一收进 `meta/`**（见 `meta/README.md`）；本文件与 `meta/CLAUDE.md` 内容一致，改任何一份必须同步另一份。

## 项目概览

Gacha 抽盒平台后端（重构版）：选盒 → 支付 → 按公示概率开奖 → 奖品入吉仓。**10 个 RPC 服务 + 2 个网关（玩家/管理）+ DTM SAGA**；功能与接口全量克隆自旧仓 `C:\Users\Administrator\Desktop\Gacha`（对照见 `docs/design/service-division.md`），业务规则参考 `F:\workspace\src\server_v2`（老项目 Lucky Box，接口照抄），工程模式参考 `D:\dtm_shop`（go-zero + DTM）。

- 部署于 `/gacha/` 子路径（网关 prefix `/gacha/api/v1`）；服务端口 9310-9321、DTM 36789/36790，见 `docs/standards/backend/01-architecture.md`
- 后端：Go + go-zero 微服务（HTTP 网关 + gRPC + Etcd + DTM SAGA + GORM）
- 前端：三个 Vue 3 + TypeScript + Vite 项目——`web_admin`（管理控制台，请求 `/console/v2/api` 走 9321 管理网关，dev 端口 9374）、`mini`（玩家小程序/H5，请求 `/gacha/api/v1` 走 9310 玩家网关，dev 端口 9375）、`app/dtm/admin`（DTM 管理端）
- 数据库：gacha_identity / gacha_money / gacha_draw / gacha_base / gacha_marketing / gacha_play / gacha_task / gacha_catalog / gacha_thirdparty / gacha_ops / gacha_dtm（一服务一库 + 每业务库 dtm_barrier），初始化脚本 `script/split/01_databases.sql`（权威建库点）+ `02_*`~`12_*` 各库脚本；本机 MySQL/Redis 已加密码，开发环境凭据见 `deploy/swarm/env.sh`（不入本文件、不重复散落）
- 服务拓扑：gateway 9310（玩家网关）、identity 9311、money 9312（资产+支付合并）、draw 9313、base 9314、marketing 9315（营销+活动合并）、play 9316（玩法）、task 9317、catalog 9318、thirdparty 9319、ops 9320、admin-gateway 9321（管理网关）

## 命令

| 操作 | 命令 |
|------|------|
| 后端代码生成 | `./script/goctl.sh <svc\|all>`（RPC 从 pkg/proto 生成；`gateway`/`admin-gateway` 已加硬闸禁跑——路由手写 routes.go） |
| 模型代码生成 | `script/model/*.sh`（gorm-zero 缓存 / 无缓存两套） |
| 后端构建 | `go build ./...` |
| 后端单测 | `go test ./...` |
| 后端端到端 | `bash test/run_all.sh`（依赖本机 MySQL/Redis/Etcd 已启动） |
| 本地起停 | `bash deploy/swarm/bootstrap.sh`（停止 `bash deploy/swarm/teardown.sh`） |
| 前端安装/开发/构建 | `cd web_admin && npm install / npm run dev / npm run build`（含 tsc 类型检查）；`cd mini && npm install / npm run dev:h5 / npm run build:h5`；`cd app/dtm/admin && npm install / npm run dev / npm run build` |
| 前端 lint | `cd web_admin && npm run lint`；其余前端项目同法 |

## 通用硬规则

1. 代码注释、文档、日志用中文；面向用户的 UI 文案一律走 i18n，禁止硬编码。
2. 接口统一响应 `{code, msg, data}`，`code == 0` 为成功，HTTP 状态码恒为 200。见 `docs/standards/common/api-conventions.md`。
3. 错误码按服务分段（identity 1000-1999、money 2000-2999、draw 3000-3999、base 4000-4999、marketing 5000-5999、play 6000-6999、task 7000-7999、catalog 8000-8999、thirdparty 9000-9999、ops 10000-10999），新服务先登记错误码段再写代码。见 `docs/standards/backend/05-errors.md`。
4. 金额一律用字符串（两位小数）传输，禁止 float 传递金额。
5. 密钥、令牌不入库、不进日志、不进提交；配置文件中密钥只写占位符（环境凭据统一由 `deploy/swarm/env.sh` 配置，每台机器独立）。
6. goctl 生成文件禁止手改（handler/logic 除外）；RPC 接口变更改 `pkg/proto/*.proto` 后 `bash script/goctl.sh <svc>` 重新生成；**网关路由直接手写 routes.go，不改 .api、禁跑 goctl gateway**（见 `docs/standards/backend/03-api-proto.md`）。
7. 提交前必须通过 lint 与测试；分支、提交信息遵循 `docs/standards/common/git-workflow.md`。

## 架构要点

- 网关（gateway/admin-gateway）无状态：不连 MySQL/Redis，只做路由、鉴权中间件与 DTM SAGA 编排。
- 一服务一库，服务间只通过 RPC/HTTP 通信，禁止跨服务直连数据库（特例：task 服务照旧仓直读 gacha_ops 计划快照，见 `docs/design/service-division.md`）。
- 涉及跨服务资金操作的接口必须用 DTM SAGA，Action/Revert 成对定义；分支错误语义：业务失败 codes.Aborted（触发补偿）、系统错误 codes.Internal（DTM 重试）。
- 只读热数据（宝箱/奖池条目）走 gorm-zero 缓存模型：写路径「事务内删缓存 → commit 后延迟双删」+ 短 TTL 兜底；资金/流水等一致性优先的表不用缓存（FOR UPDATE 行锁）。参考 `D:\dtm_shop` 的 stock/payment 服务（新仓对应 money 服务）。
- 玩法域（play）与营销域（marketing）跨服务协作走 RPC：报价锁 LockQuote/ReleaseQuote、发券 GrantPlayCoupon、券名 GetCouponTemplateByNo；券/锁状态以 marketing 服务为准，玩法侧不再本地建模。
- proto 唯一来源 `pkg/proto/`（10 个服务 proto）；API 定义 `pkg/api/`（gateway.api）与 `pkg/api_admin/`（admin-gateway.api）仅登记路由，路由事实源是各网关 `internal/handler/routes.go`。

## 规范索引

| 范围 | 路径 | 内容 |
|------|------|------|
| 通用 | `docs/standards/common/` | git 规范、API 接口约定、评审清单 |
| 后端 | `docs/standards/backend/` | 架构、目录布局、api-proto、编码风格、错误处理、数据库、测试、部署配置 |
| 前端 | `docs/standards/frontend/` | 架构、目录布局、组件、状态与路由、services 层、i18n 与样式、工程质量 |

需要某方面细节时，按本索引阅读对应规范文档，不要凭记忆猜测。
