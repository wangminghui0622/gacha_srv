# 部署与配置规范

## 配置（pkg/etcs/）

1. 每个服务一个 yaml：`gateway-api.yaml`、`<svc>-rpc.yaml`；命名与启动参数 `-f ../../pkg/etcs/<svc>-rpc.yaml` 一致。
2. 网关配置：`Name: gateway.api`、`Host/Port: 9310`（管理网关 `admin-gateway.api` 9321）、每个下游一段 `XxxRpc: {Etcd: {Hosts, Key: xxx.rpc}}`、`DtmServer`、直连目标 `XxxRpcTarget: "dns:///ip:port"`（供 DTM 使用）、业务可调参数注释中文。
3. RPC 配置：`Name: <svc>.rpc`、`ListenOn: 0.0.0.0:<端口>`、`Etcd`、`MySQL.DataSource`（DSN 带 `charset=utf8mb4&parseTime=True&loc=Local`）、`RedisConf`、`JwtAuth`、`DtmServer`、`Log: {ServiceName: <svc>-rpc, Mode: console, Level: info}`。
4. **密钥只放占位符**：`"your-secret-key-change-in-production"`、`PaymentCallbackSecret: "change-me-..."`；生产替换走发布流程，禁止把真实密钥提交进仓库。
5. 配置结构体（`internal/config/config.go`）内嵌 go-zero conf 类型，自定义块用匿名结构体并注释中文。
6. **本机基础设施带密码**（2026-10-26 起）：MySQL root 与 Redis 均设开发密码——DSN 写 `root:${MYSQL_PASS}@tcp(...)` 占位、Redis 块 `Pass` 占位，**具体值不入库、不落文档**，统一由 `deploy/swarm/env.sh` 配置；MySQL 密码存服务端数据目录、Redis requirepass 存其启动配置 redis.conf，随重启持久；测试脚本用 `DB_PASS`/`REDIS_PASS` 环境变量（默认同值可覆盖）；DSN 禁止回退空密码。

## 构建与启停（deploy/）

7. 部署脚本已统一为 `deploy/swarm/`（env.sh 环境配置 / bootstrap.sh 一键部署 / build-binaries.sh 交叉编译+生成配置 / build-push.sh 构建推送 / release.sh 滚动更新回滚 / teardown.sh 清理），原 `deploy/linux`、`deploy/windows`、`deploy/dev` 三套脚本已废弃（2026-11-11）。
8. 构建：`go build -o bin/<name>`；Linux 交叉编译固定 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`；产物 `bin/gateway`、`bin/identity-rpc`……（Windows 加 `.exe`）；`bin/`、`logs/` 必须 gitignore。
8a. **已知现象**：前端 `npm install` 后，`go build/test/vet ./...` 会一并编译 `web_admin/node_modules` 内第三方 JS 包的 Go 移植文件（如 flatted，纯标准库、无副作用）；若未来某依赖的 Go 移植引入外部依赖导致后端构建失败，在该前端目录放一个独立 `go.mod` 划出 module 边界即可（root 的 `./...` 不进独立 module 目录）。
9. 启动：从项目根目录执行（保证相对配置路径解析），`nohup <bin> -f <配置> > logs/<名>.log 2>&1 &`；启动顺序固定 **DTM → 各 RPC → 网关**，间隔留探活时间；停止先 `kill -15` 再 `kill -9`。
10. 新增服务必须同步更新两套脚本的服务列表。

## 反向代理（nginx）

11. 生产 nginx 单配置文件：80 端口 301 跳 HTTPS；443 挂 TLS。
12. location 顺序遵循最长前缀匹配原则（注释说明）：
    - `/gacha/api/` → 玩家网关 9310
    - `/swagger`、`/swagger.json` → 玩家网关 9310
    - `/gacha/` → 前端静态资源
13. 与前端约定的部署子路径必须三方一致：nginx location、网关路由 prefix、前端 `vite.config.ts` 的 `base` / router history `base`。

## 环境与发布

14. 不做配置文件分环境拆分；差异只体现在密钥值（占位符 vs 生产注入），发布流程负责替换。
15. 工具链版本（goctl、protoc、gRPC、DTM、Go）记录在 `docs/version`，升级需全量回归。
