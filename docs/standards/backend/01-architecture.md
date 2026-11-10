# 后端架构规范（go-zero 微服务）

## 服务划分

1. 每个业务域一个 RPC 服务（identity 身份、money 资金（资产+支付）、draw 抽奖、base 基础（资格/风控）、marketing 营销+活动、play 玩法、task 任务、catalog 商城、thirdparty 第三方、ops 运营），服务名 `<name>-rpc`；全部服务统一在 `app/<name>` 单树，玩家网关在 `app/gateway`，管理网关在 `app/admin-gateway`。
2. 对外只暴露两个 HTTP 网关 `gateway.api`（玩家）/`admin-gateway.api`（管理），客户端（含前端）不直连 RPC。
3. 网关**无状态**：不连 MySQL/Redis，只做路由转发、鉴权中间件、参数校验与 DTM SAGA 编排。
4. 端口规划（全平台唯一，新增服务登记）：

| 服务 | 端口 | 协议 |
|------|------|------|
| gateway（玩家网关，app/gateway） | 9310 | HTTP |
| identity-rpc | 9311 | gRPC |
| money-rpc（资产+支付合并） | 9312 | gRPC |
| draw-rpc | 9313 | gRPC |
| base-rpc | 9314 | gRPC |
| marketing-rpc（营销+活动合并） | 9315 | gRPC |
| play-rpc | 9316 | gRPC |
| task-rpc | 9317 | gRPC |
| catalog-rpc | 9318 | gRPC |
| thirdparty-rpc | 9319 | gRPC |
| ops-rpc | 9320 | gRPC |
| admin-gateway（管理网关，app/admin-gateway） | 9321 | HTTP |
| DTM Server | 36789 / 36790 | HTTP / gRPC |

5. 新服务按序取 9322、9323……端口与 RPC 服务名（`<name>.rpc`）都必须在 Etcd 中唯一。

## 通信与发现

6. 服务注册与发现用 Etcd；网关通过 `XxxRpc: {Etcd: {Key: xxx.rpc}}` 配置发现下游。
7. 服务间调用只走 gRPC（zrpc client），禁止共享数据库、禁止 HTTP 互调。
8. 网关调下游必须挂 `ClientErrorInterceptor`（gRPC status → CodeError），RPC 服务挂 `ServerErrorInterceptor`（CodeError → gRPC status），保证错误码跨服务不失真。

## 认证

9. 令牌签发与校验集中在身份服务（identity）：登录签 JWT（HS256，Claims 含 `UserId`/`Username`），网关通过 RPC `VerifyToken` 校验后把 `userId` 放入请求上下文。
10. 网关鉴权中间件对受保护路由生效（在 `routes.go` 的受保护路由组声明 `middleware: JwtAuth`）；公开路由（登录、注册、宝箱列表、支付回调）不声明。

## 分布式事务（DTM SAGA）

11. 跨服务写操作（如抽盒购买）必须走 DTM SAGA：网关编排，DTM Server 存储事务状态。
12. 参与事务的每个操作成对声明 `<Action>` / `<Action>Revert` RPC；Revert 必须幂等。
13. 每个业务库建 `dtm_barrier` 表（防悬挂/空补偿），服务 main 中 `dtmcli.SetBarrierTableName("dtm_barrier")`。
14. 事务服务地址用直连目标（`dns:///ip:port`）写入配置，不经过 Etcd 发现。

## 数据与基础设施

15. 一服务一库（`gacha_<域>`），服务只连自己的库；Redis 按需共享，key 带服务前缀。
16. 渠道回调、指标接入等非核心逻辑异步化：不阻塞主流程返回（如第三方 provider 指标走 outbox + 轮询分发），并自行兜底重试。
17. gRPC 反射只在 `DevMode/TestMode` 开启。
