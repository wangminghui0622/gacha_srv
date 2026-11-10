# API / Proto 定义规范

## .api 文件（网关）

1. `pkg/api/gateway.api` 只做聚合导入，每个业务模块一个文件放 `pkg/api/desc/`：

```go
import "desc/identity.api"
import "desc/draw.api"
```

2. 每个模块文件结构：中文分隔横幅 → `type (...)` 请求/响应结构（每个字段上方一行中文注释）→ `@server` 块 → `service gateway-api`。
3. `@server` 块约定：`prefix: /gacha/api/v1`、`group: <域>`、受保护接口 `middleware: JwtAuth`、`tags: <域>`。
4. 端点声明：`@doc` 中文说明、`@handler` 驼峰名、路径 kebab-case、动词 `get/post/put`：

```go
@server (
    prefix:     /gacha/api/v1
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

5. 字段规则：json tag camelCase；校验 tag 内联；分页 `Page int \`json:"page,default=1" form:"page,default=1"\``、`PageSize int \`json:"pageSize,default=10" form:"pageSize,default=10"\``；金额/时间用 `string`。**GET 查询参数必须同时写 form 标签**（go-zero httpx 的查询绑定走 form tag，json 标签只负责响应序列化；漏写 form 则查询参数静默不生效）。校验 tag 约定：`min`/`max` 只用于字符串/切片（翻译为「长度」语义），**数值字段用 `gte`/`lte`/`gt`/`lt`**；字符串格式用自定义 tag `name`（中文/字母/数字/@_!&*）、密码用 `passwordFormat`；金额字符串比较用 `dgt=0`/`dgte=…`（decimal 精确比较）。
6. 共享类型（分页、通用结构）放 `desc/common.api`，该文件同时携带 `syntax = "v1"` 与 `info(...)`（title/desc/author/version）。
7. 校验器全局注册（`httpx.SetValidator`），校验失败返回 400 与中文提示（见 `pkg/utils/validator`）。实现参考 slots_platform（go-playground/validator/v10 + universal-translator 中文翻译 + 字段友好名映射 + 自定义校验器）；新增接口字段若想获得中文错误提示，在 `fieldFriendlyNames` 登记友好名。

### 现状与事实源（M8 起：routes.go 是事实源，.api 冻结为早期登记）

**网关路由的事实源是 routes.go，不是 .api**。玩家网关 M8-M15d 新增端点、管理网关 M14d/M14e 的 task 计划与 reporting 控制台全部直接手写 routes.go；.api 文件保留为早期接口登记，**不再同步**。`script/goctl.sh` 的 `gateway`/`admin-gateway` 分支已加硬闸（默认拒绝重生成，`FORCE_GATEWAY_REGENERATE=1` 可越过）——重生成会把 routes.go 覆盖回 .api 定义面，手写路由全部丢失。

**新增端点流程（替代上节第 1-4 条的 .api 流程）**：手写 routes.go 路由 + handler（+ svc 客户端）；不改 .api。新增 RPC 仍照常改 `pkg/proto/*.proto` 后跑 `bash script/goctl.sh <svc>`（RPC 生成不受影响）。

- **玩家网关** `app/gateway/internal/handler/routes.go`：57 条路由条目（56 个路径）。.api 登记 10 条（user 5 + draw 5，M7 面），**其余 47 条条目（46 个路径）手写无登记**：

  | 域（handler 包） | 手写路径 |
  |---|---|
  | user（M15a 登录） | /auth/phone/code、/auth/phone/login、/auth/wechat/login |
  | draw（M10/M12a） | /box/quote、/rolling-draw/recent-wins |
  | game | /coupon-templates、/coupons、/coupons/claim |
  | campaign | /campaigns、/campaigns/:campaignKey/preview、/campaigns/participate、/campaign-boxes、/campaign-boxes/:entitlementNo/open |
  | dig | /dig-boxes、/dig-boxes/:boxId/quote、/dig-boxes/:boxId/session、/dig-boxes/:boxId/buy、/dig-boxes/records |
  | fullcar | /full-car/:configId、/full-car/:configId/buy、/full-car/records |
  | gashapon | /gashapon/:configId、/gashapon/:configId/buy、/gashapon/records |
  | combochain | /combo-chain/:configId、/combo-chain/:configId/buy、/combo-chain/records |
  | groupbuy | /group-buy/:configId、/group-buy/:configId/buy、/group-buy/records |
  | memorymatch | /memory-match/configs/:configId、/memory-match/appearance、/memory-match/purchase、/memory-match/sessions/:sessionId/{destiny,second-destiny,skip}、/memory-match/sessions/:sessionId/rounds/:roundNo/{pair,destiny-draw,complete} |
  | task | /tasks、/tasks/:taskId/sign-in、/tasks/:taskId/claim、/tasks/passes/:passId/claim、/membership/claim |

- **管理网关** `app/admin-gateway/internal/handler/routes.go`：66 条路由条目，与 `pkg/adminop/operations.go` 双向一致（fail-closed 启动校验）。.api 登记 52 条，**14 条手写无登记**（M14e）：

  | 域 | 方法 | 路径 |
  |---|---|---|
  | reporting | GET | /reporting/metrics、/reporting/dashboards/:dashboardId、/analytics/users/:userId/profit-curve |
  | reporting | POST | /reporting/dashboards、/reporting/dashboards/:dashboardId/publish、/reporting/dashboards/:dashboardId/query |
  | task 计划 | GET | /tasks、/task-plans、/task-plans/:version、/task-plans/:version/approval |
  | task 计划 | POST | /task-plans/drafts、/task-plans/:version/publication-request、/task-plans/:version/approval、/task-plans/:version/publish |

## .proto 文件（RPC）

8. 每个服务一个 `pkg/proto/<svc>.proto`：

```proto
syntax = "proto3";

package identity;
option go_package = "./identity";

service Identity {
    rpc Register(RegisterReq) returns (RegisterResp);
}
```

9. 包名 = 服务名；`go_package = "./<svc>"`（相对路径）。
10. 字段名**刻意 camelCase**（`userId`、`betAmount`），与 JSON/API 直通，不用 protobuf 惯例 snake_case。
11. 类型约定：金额/余额 `string`；时间 `string`；请求/响应 `<Action>Req` / `<Action>Resp`；布尔结果 `bool success = 1;`；分页响应 `repeated X list = 1; int64 total = 2;`。
12. SAGA 参与服务成对声明：`rpc PlaceBet(PlaceBetReq) returns (PlaceBetResp); rpc PlaceBetRevert(PlaceBetReq) returns (PlaceBetResp);`。

## 代码生成

13. 生成脚本 `script/goctl.sh`，参数 `<all|identity|money|...>`，固定用 `--style goZero`。**`gateway`/`admin-gateway` 分支已加硬闸默认拒绝执行**（见上节「现状与事实源」），日常只跑 `<svc>`（RPC）分支：

```bash
goctl rpc protoc identity.proto --go_out=... --go-grpc_out=... --zrpc_out=... --style goZero
```

14. 生成后必须执行清理：删除 `app/<svc>/etc`，把配置路径改写为 `../../pkg/etcs/<svc>-rpc.yaml`（脚本中自动化）。
15. swagger JSON 生成后提交到 `pkg/api/`，由网关自定义路由在 `/swagger`、`/swagger.json` 提供。
16. 生成文件头部注释（`DO NOT EDIT` / `Safe to edit`）必须保留，明确哪些文件可以手改（handler 与 logic 可以）。
