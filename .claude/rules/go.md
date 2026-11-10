---
paths:
  - "**/*.go"
  - "**/*.api"
  - "**/*.proto"
  - "**/*.sql"
---

# Go 后端规则（go-zero 微服务）

## 命名

1. Go 标识符遵循 Go 惯例（导出 CamelCase，非导出 camelCase）；`.api`/`.proto` 字段一律 camelCase（proto 刻意不用 snake_case，保证与 JSON 直通）。
2. 服务命名：`<name>.rpc`（RPC 服务）、`gateway.api`（网关）；配置文件名 `pkg/etcs/<svc>-rpc.yaml`。
3. 请求/响应类型命名 `<Action>Req` / `<Action>Resp`；SAGA 补偿方法成对命名 `<Action>` / `<Action>Revert`。
4. SQL 表名蛇形复数（`users`、`game_bets`）；索引 `uk_<列>`（唯一）、`idx_<列>`（普通）。

## 目录与生成代码

5. 配置文件只放 `pkg/etcs/`；goctl 生成的 `app/<svc>/etc` 目录生成后立即删除，`main.go` 用 `-f ../../pkg/etcs/<svc>-rpc.yaml` 指定配置。
6. 生成文件（`types.go`、`routes.go`、`*.pb.go`）禁止手改；改 `.api`/`.proto` 后用 `./script/goctl.sh <svc>` 重新生成。
7. 公共代码进 `pkg/utils/`（errorx、response、jwt、interceptor、validator 等），禁止在服务内部复制。

## 分层职责

8. 网关 handler 薄：只调用 logic 并写响应（`httpx.OkJsonCtx` / `httpx.ErrorCtx`）。
9. RPC server 层纯委托：`l := logic.NewXxxLogic(ctx, s.svcCtx); return l.Xxx(in)`。
10. Logic 采用 `XxxLogic` 结构体模式：字段含 `ctx`、`svcCtx`、内嵌 `logx.Logger`，构造器 `NewXxxLogic(ctx, svcCtx)`，方法名与端点同名。
11. Model 层：一个表一个结构体（GORM tag + 显式 `TableName()`），一个 `XxxModel` DAO 结构体（`NewXxxModel(db)`），方法 `FindByXxx/FindList/Create/Update/UpdateXxx`，`ctx` 为第一参数。
12. 查询不存在时返回 `(nil, nil)` 而非错误（映射 `gorm.ErrRecordNotFound`）；余额等原子更新用 `gorm.Expr("balance + ?", amount)` 并加 `WHERE` 守卫，`RowsAffected == 0` 表示余额不足。

## 错误与响应

13. 业务错误一律 `errorx.NewCodeError*` 或预定义单例错误（`errorx.ErrXxx`），禁止裸 `errors.New` 返回给调用方。
14. Logic 包装未知错误用 `%w` 并带中文上下文：`fmt.Errorf("查询用户失败: %w", err)`。
15. 网关统一响应信封 `{code, msg, data}` 由 `httpx.SetOkHandler` / `SetErrorHandlerCtx` 集中处理，HTTP 恒 200；handler 内不得自行拼响应结构。
16. 新错误码按 `../../docs/standards/backend/05-errors.md` 的分段表登记。

## 数据与安全

17. 金额在 RPC 边界用 `fmt.Sprintf("%.2f", ...)` 转字符串；时间格式 `"2006-01-02 15:04:05"`。
18. 密码字段 JSON tag 为 `json:"-"`；日志禁止输出令牌、密码、密钥。
19. 每个业务库必须建 `dtm_barrier` 表并在服务 main 里 `dtmcli.SetBarrierTableName("dtm_barrier")`。
20. gRPC 反射只在 `DevMode/TestMode` 下注册。

## 风格

21. 注释中文；逻辑步骤用 `// 1. …`、`// 2. …` 编号注释；模块用 `// ==================== xxx ====================` 分隔。
22. gofmt 标准格式（tab 缩进）；提交前跑 `go test ./...`。
