# Gacha 抽盒平台

抽盒（盲盒）平台：选盒 → 支付 → 按公示概率开奖 → 奖品入吉仓。10 个 RPC 服务 + 2 个网关（玩家/管理）+ DTM SAGA 保证跨服务资金操作最终一致；老项目 server_v2 功能全量克隆（范围与偏离见 `docs/design/service-mapping.md`）。

> 本仓是从旧仓（`../Gacha`，module gacha）重构重建的仓库：结构重组 + 代码迁移适配，不重写业务规则。核心不变式：**接口不变**——HTTP 路径、RPC 形态、数据库表结构、前端行为均不变。拆分决策见 `docs/design/service-division.md`（单一事实源）。

## 📚 文档索引

| 文档 | 说明 |
|------|------|
| `docs/design/service-division.md` | 服务拆分重构蓝图（拓扑/表映射/SAGA 编排/迁移清单，单一事实源） |
| `docs/design/service-mapping.md` | 老项目（server_v2）服务 ↔ 本仓服务对照表（克隆范围与偏离） |
| `docs/design/legacy/` | 旧仓历史里程碑设计文档（旧拓扑名，仅供追溯） |
| `docs/standards/` | 工程规范（通用 / 后端 / 前端） |
| `docs/version` | 工具链版本基线（升级需全量回归） |
| `meta/` | 标准包源文件（CLAUDE.md 副本 / rules / skills / specs / templates / settings 模板） |

## 🚀 快速开始

```bash
# 1) 初始化数据库：按序号执行 script/split/ 下 01_databases.sql（权威建库点，11 库+屏障表）
#    及 02_*~12_*（各库终态结构+种子）。本机 MySQL/Redis 已加密码，凭据见 deploy/dev/ 启动脚本。

# 2) 启动依赖（Redis/Etcd；MySQL 后台常驻）与全部服务（DTM → 10 RPC → 2 网关）
bash deploy/dev/start-infra.sh
bash deploy/dev/start.sh        # 停止：bash deploy/dev/stop.sh

# 3) 一键端到端回归（29 场景；慢场景 5/6/10 用 bash test/menu.sh 单独执行）
bash test/run_all.sh
```

## 🗄️ 服务与端口

| 服务 | 端口 | 协议 | 数据库 | 说明 |
|------|------|------|--------|------|
| gateway（玩家网关） | 9310 | HTTP | — | 路由 / JWT 鉴权 / DTM SAGA 编排（prefix `/gacha/api/v1`） |
| identity-rpc | 9311 | gRPC | gacha_identity | 账号/会话/手机号/三方绑定 |
| money-rpc | 9312 | gRPC | gacha_money | 钱包/资产/冻结（asset+payment 合并） |
| draw-rpc | 9313 | gRPC | gacha_draw | 宝箱 / 开奖 / 吉仓 / 滚动中奖 / 赏池控制台 |
| base-rpc | 9314 | gRPC | gacha_base | 资格策略 + 风控（不拆，整体搬迁） |
| marketing-rpc | 9315 | gRPC | gacha_marketing | 报价/券/活动（promotion+campaign 合并） |
| play-rpc | 9316 | gRPC | gacha_play | 六玩法（挖宝/满车/扭蛋/连击链/拼团/记忆翻牌）+ 幸运指派 |
| task-rpc | 9317 | gRPC | gacha_task | 任务/签到/通行证/会员 |
| catalog-rpc | 9318 | gRPC | gacha_catalog | 商品目录控制台 |
| thirdparty-rpc | 9319 | gRPC | gacha_thirdparty | 手机号/微信/支付宝/微信支付渠道 |
| ops-rpc | 9320 | gRPC | gacha_ops | 控制台报表/权限（管理网关专用） |
| admin-gateway（管理网关） | 9321 | HTTP | — | 控制台路由（prefix `/console/v2/api`）+ 权限校验 |
| dtm | 36789/36790 | HTTP/gRPC | gacha_dtm | 分布式事务状态存储 |

## 🖥️ 前端

| 项目 | 目录 | 说明 |
|------|------|------|
| 管理控制台 | `web_admin/` | Vue 3 + TS + Vite（Ant Design Vue）；请求 `/console/v2/api` → 9321 管理网关；dev 端口 9374 |
| 玩家小程序/H5 | `mini/` | uni-app + Vue 3；请求 `/gacha/api/v1` → 9310 玩家网关；dev 端口 9375；微信开发者工具走查见 `mini/CHECKLIST.md` |
| DTM 管理端 | `app/dtm/admin/` | DTM 自带管理前端 |

## 🔑 工程铁律（摘要，完整见 CLAUDE.md）

- 注释/文档/日志用中文；UI 文案走 i18n；统一响应 `{code, msg, data}`，HTTP 恒 200
- 金额一律两位小数字符串；密钥不入库/不进日志/不进提交（配置文件只写占位符，本机开发凭据由 deploy/dev 脚本注入）
- 一服务一库，服务间只走 RPC；跨服务资金操作必须 DTM SAGA（Aborted=补偿、Internal=重试）
- goctl 生成文件禁止手改；网关路由手写 routes.go；提交前必须通过 lint 与测试
