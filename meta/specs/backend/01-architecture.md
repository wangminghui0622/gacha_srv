# 后端架构规范（go-zero 微服务）

## 服务划分

1. 每个业务域一个 RPC 服务（用户、游戏、支付、代理、VIP、运营……），服务名 `<name>.rpc`。
2. 对外只暴露一个 HTTP 网关 `gateway.api`，客户端（含前端）不直连 RPC。
3. 网关**无状态**：不连 MySQL/Redis，只做路由转发、鉴权中间件、参数校验与 DTM SAGA 编排。
4. 端口规划（全平台唯一，新增服务登记）：

| 服务 | 端口 | 协议 |
|------|------|------|
| Gateway API | 8888 | HTTP |
| User RPC | 6001 | gRPC |
| Game RPC | 6002 | gRPC |
| Payment RPC | 6003 | gRPC |
| Agent RPC | 6004 | gRPC |
| VIP RPC | 6005 | gRPC |
| Ops RPC | 6006 | gRPC |
| DTM Server | 36789 / 36790 | HTTP / gRPC |

5. 新服务按序取 6007、6008……端口与 RPC 服务名（`<name>.rpc`）都必须在 Etcd 中唯一。

## 通信与发现

6. 服务注册与发现用 Etcd；网关通过 `XxxRpc: {Etcd: {Key: xxx.rpc}}` 配置发现下游。
7. 服务间调用只走 gRPC（zrpc client），禁止共享数据库、禁止 HTTP 互调。
8. 网关调下游必须挂 `ClientErrorInterceptor`（gRPC status → CodeError），RPC 服务挂 `ServerErrorInterceptor`（CodeError → gRPC status），保证错误码跨服务不失真。

## 认证

9. 令牌签发与校验集中在用户服务：登录签 JWT（HS256，Claims 含 `UserId`/`Username`），网关通过 RPC `VerifyToken` 校验后把 `userId` 放入请求上下文。
10. 网关鉴权中间件对受保护路由生效（在 `.api` 的 `@server` 块声明 `middleware: JwtAuth`）；公开路由（登录、注册、公告、支付回调）不声明。

## 分布式事务（DTM SAGA）

11. 跨服务写操作（如下注扣款）必须走 DTM SAGA：网关编排，DTM Server 存储事务状态。
12. 参与事务的每个操作成对声明 `<Action>` / `<Action>Revert` RPC；Revert 必须幂等。
13. 每个业务库建 `dtm_barrier` 表（防悬挂/空补偿），服务 main 中 `dtmcli.SetBarrierTableName("dtm_barrier")`。
14. 事务服务地址用直连目标（`dns:///ip:port`）写入配置，不经过 Etcd 发现。

## 数据与基础设施

15. 一服务一库（`<项目>_<域>`），服务只连自己的库；Redis 按需共享，key 带服务前缀。
16. 三级佣金、通知等非核心逻辑异步化：不阻塞主流程返回（`_ = distributor.Distribute(...)` 并自行兜底重试）。
17. gRPC 反射只在 `DevMode/TestMode` 开启。
