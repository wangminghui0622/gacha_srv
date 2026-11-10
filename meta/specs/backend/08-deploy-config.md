# 部署与配置规范

## 配置（pkg/etcs/）

1. 每个服务一个 yaml：`gateway-api.yaml`、`<svc>-rpc.yaml`；命名与启动参数 `-f ../../pkg/etcs/<svc>-rpc.yaml` 一致。
2. 网关配置：`Name: gateway.api`、`Host/Port: 8888`、每个下游一段 `XxxRpc: {Etcd: {Hosts, Key: xxx.rpc}}`、`DtmServer`、直连目标 `XxxRpcTarget: "dns:///ip:port"`（供 DTM 使用）、业务可调参数注释中文。
3. RPC 配置：`Name: <svc>.rpc`、`ListenOn: 0.0.0.0:<端口>`、`Etcd`、`MySQL.DataSource`（DSN 带 `charset=utf8mb4&parseTime=True&loc=Local`）、`RedisConf`、`JwtAuth`、`DtmServer`、`Log: {ServiceName: <svc>-rpc, Mode: console, Level: info}`。
4. **密钥只放占位符**：`"your-secret-key-change-in-production"`、`PaymentCallbackSecret: "change-me-..."`；生产替换走发布流程，禁止把真实密钥提交进仓库。
5. 配置结构体（`internal/config/config.go`）内嵌 go-zero conf 类型，自定义块用匿名结构体并注释中文。

## 构建与启停（deploy/）

6. `deploy/linux/*.sh` 与 `deploy/windows/*.bat` 成对维护，行为一致；统一参数 `{all|<服务名>|stop}`，用 case/goto 分发，中文 echo 带 ✅/√/× 标记。
7. 构建：`go build -o bin/<name>`；Linux 交叉编译固定 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`；产物 `bin/gateway`、`bin/identity-rpc`……（Windows 加 `.exe`）；`bin/`、`logs/` 必须 gitignore。
8. 启动：从项目根目录执行（保证相对配置路径解析），`nohup <bin> -f <配置> > logs/<名>.log 2>&1 &`；启动顺序固定 **DTM → 各 RPC → 网关**，间隔留探活时间；停止先 `kill -15` 再 `kill -9`。
9. 新增服务必须同步更新两套脚本的服务列表。

## 反向代理（nginx）

10. 生产 nginx 单配置文件：80 端口 301 跳 HTTPS；443 挂 TLS。
11. location 顺序遵循最长前缀匹配原则（注释说明）：
    - `/<项目>/api/` → 网关 8888
    - `/swagger`、`/swagger.json` → 网关 8888
    - `/<项目>/` → 前端静态资源
12. 与前端约定的部署子路径必须三方一致：nginx location、网关路由 prefix、前端 `vite.config.ts` 的 `base` / router history `base`。

## 环境与发布

13. 不做配置文件分环境拆分；差异只体现在密钥值（占位符 vs 生产注入），发布流程负责替换。
14. 工具链版本（goctl、protoc、gRPC、DTM、Go）记录在 `docs/version`，升级需全量回归。
