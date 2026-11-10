# API / Proto 定义规范

## .api 文件（网关）

1. `pkg/api/gateway.api` 只做聚合导入，每个业务模块一个文件放 `pkg/api/desc/`：

```go
import "desc/user.api"
import "desc/payment.api"
```

2. 每个模块文件结构：中文分隔横幅 → `type (...)` 请求/响应结构（每个字段上方一行中文注释）→ `@server` 块 → `service gateway-api`。
3. `@server` 块约定：`prefix: /<项目名>/api/v1`、`group: <域>`、受保护接口 `middleware: JwtAuth`、`tags: <域>`。
4. 端点声明：`@doc` 中文说明、`@handler` 驼峰名、路径 kebab-case、动词 `get/post/put`：

```go
@server (
    prefix:     /<项目名>/api/v1
    group:      user
    middleware: JwtAuth
    tags:   user
)
service gateway-api {
    @doc "获取用户信息"
    @handler GetProfile
    get /user/profile returns (UserInfo)
}
```

5. 字段规则：json tag camelCase；校验 tag 内联（`validate:"required,min=6,max=32"`）；分页 `Page int \`json:"page,default=1"\``、`PageSize int \`json:"pageSize,default=10"\``；金额/时间用 `string`。
6. 共享类型（分页、通用结构）放 `desc/common.api`，该文件同时携带 `syntax = "v1"` 与 `info(...)`（title/desc/author/version）。
7. 校验器全局注册（`httpx.SetValidator`），校验失败返回 400 与中文提示（见 `pkg/utils/validator`）。

## .proto 文件（RPC）

8. 每个服务一个 `pkg/proto/<svc>.proto`：

```proto
syntax = "proto3";

package user;
option go_package = "./user";

service User {
    rpc Register(RegisterReq) returns (RegisterResp);
}
```

9. 包名 = 服务名；`go_package = "./<svc>"`（相对路径）。
10. 字段名**刻意 camelCase**（`userId`、`betAmount`），与 JSON/API 直通，不用 protobuf 惯例 snake_case。
11. 类型约定：金额/余额 `string`；时间 `string`；请求/响应 `<Action>Req` / `<Action>Resp`；布尔结果 `bool success = 1;`；分页响应 `repeated X list = 1; int64 total = 2;`。
12. SAGA 参与服务成对声明：`rpc PlaceBet(PlaceBetReq) returns (PlaceBetResp); rpc PlaceBetRevert(PlaceBetReq) returns (PlaceBetResp);`。

## 代码生成

13. 生成脚本 `script/goctl.sh`，参数 `<all|gateway|user|...>`，固定用 `--style goZero`：

```bash
goctl api go -api "${PKG_DIR}/api/gateway.api" -dir "${APP_DIR}/gateway" --style goZero
goctl rpc protoc user.proto --go_out=... --go-grpc_out=... --zrpc_out=... --style goZero
```

14. 生成后必须执行清理：删除 `app/<svc>/etc`，把配置路径改写为 `../../pkg/etcs/<svc>-rpc.yaml`（脚本中自动化）。
15. swagger JSON 生成后提交到 `pkg/api/`，由网关自定义路由在 `/swagger`、`/swagger.json` 提供。
16. 生成文件头部注释（`DO NOT EDIT` / `Safe to edit`）必须保留，明确哪些文件可以手改（handler 与 logic 可以）。
