# 后端目录布局规范

## 顶层结构

```
<project>/
├── app/<svc>/                  # 每个服务一个目录
├── pkg/
│   ├── api/                    # 网关接口定义：gateway.api + desc/*.api + swagger json
│   ├── proto/                  # RPC 接口定义：每个服务一个 .proto
│   ├── etcs/                   # 所有服务运行时配置（唯一配置位置）
│   └── utils/                  # 跨服务公共代码
├── script/                     # goctl 代码生成脚本、SQL 初始化脚本
├── deploy/{linux,windows}/     # 构建/启动/停止脚本、nginx 配置
├── test/                       # e2e 测试场景
└── docs/                       # 中文文档
```

## 单服务结构（app/<svc>/）

```
app/user/
├── user.go                     # main：flag 配置路径、注册拦截器、启动
├── user/                       # protoc 生成（.pb.go），禁止手改
├── userClient/                 # 生成的 gRPC client 包装，禁止手改
└── internal/
    ├── config/config.go        # Config 结构体（内嵌 go-zero conf 类型）
    ├── server/<svc>Server.go   # 仅 RPC 服务：薄委托到 logic
    ├── handler/                # 仅网关：按 API 组分子包 + routes.go（生成）
    ├── logic/                  # 每个端点一个 <x>Logic.go
    ├── middleware/             # 仅网关：jwtauth、cors、logger、rateLimit
    ├── model/                  # GORM 模型 + Model DAO
    ├── svc/serviceContext.go   # 依赖注入容器
    └── types/types.go          # 仅网关：goctl 生成，禁止手改
```

## 规则

1. **配置唯一位置** `pkg/etcs/`：删除 goctl 生成的 `app/<svc>/etc`，`main.go` 固定写法：

```go
var configFile = flag.String("f", "../../pkg/etcs/identity-rpc.yaml", "the config file")
```

2. **生成文件红线**：`types.go`、`routes.go`、`*.pb.go`、`*Client/` 禁止手改；新增端点改 `.api`/`.proto` 后重新生成（`./script/goctl.sh <svc>`）。
3. 网关额外路由（swagger 服务等）走 `custom_routes.go` 单独注册，避免被 goctl 覆盖。
4. 公共代码只进 `pkg/utils/`；两个服务用到的同一段代码就应下沉，禁止复制。
5. SQL 脚本放 `script/split/` 编号文件；构建/启停脚本按 OS 放 `deploy/`。
6. 新服务创建后同步：`pkg/etcs/<svc>-rpc.yaml`、`deploy/*` 脚本服务列表、`README` 端口表。
