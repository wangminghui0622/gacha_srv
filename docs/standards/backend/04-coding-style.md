# Go 编码风格规范

## 注释

1. 全部中文。导出符号必须有文档注释；逻辑分步用编号注释；模块分隔用横幅：

```go
// ==================== 用户模块 ====================

// 1. 查找用户
// 2. 验证密码
```

2. 生成文件的 goctl 头注释保留，不得删除或伪造。

## Logic 层

3. 每个端点一个 `<Action>Logic` 文件，统一模式：

```go
type LoginLogic struct {
    ctx    context.Context
    svcCtx *svc.ServiceContext
    logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
    return &LoginLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *LoginLogic) Login(in *user.LoginReq) (*user.LoginResp, error) { ... }
```

4. Server 层只做纯委托，不写业务。
5. 未知错误包装用 `%w` + 中文上下文：`fmt.Errorf("查询用户失败: %w", err)`；预定义业务错误直接返回单例（`errorx.ErrPasswordWrong`）。

## Model 层

6. 一个表一个结构体：GORM tag 完整（`primaryKey;autoIncrement`、`size:50;not null;uniqueIndex`、`type:decimal(12,2)`、`autoCreateTime`），json tag camelCase，敏感字段 `json:"-"`，显式 `TableName()` 返回蛇形复数表名。
7. DAO 结构体 `XxxModel{db *gorm.DB}` + 构造器 `NewXxxModel(db)`；方法 `FindByXxx/FindList/Create/Update/UpdateXxx`；`ctx` 第一参数。
8. 不存在即 `(nil, nil)`：把 `gorm.ErrRecordNotFound` 映射为 nil。
9. 金额原子更新用表达式 + 守卫：

```go
result := m.db.WithContext(ctx).Model(&User{}).
    Where("id = ? AND balance + ? >= 0", uid, amount).
    Update("balance", gorm.Expr("balance + ?", amount))
// RowsAffected == 0 → 余额不足
```

## 数据与日志

10. 金额在 RPC 边界格式化 `fmt.Sprintf("%.2f", u.Balance)`；时间 `"2006-01-02 15:04:05"`。
11. 日志禁止输出令牌、密码、密钥；错误日志带上下文（用户 id、订单号）。
12. 数据库错误先过 `errorx.FromMySQLError` / `WrapRedisError` 再返回，禁止直接透出驱动错误。

## 格式与工具

13. gofmt 标准格式（tab）；提交前 `go test ./...` 通过。
14. 工具链版本固定并记录在 `docs/version`（goctl、protoc、protoc-gen-go 等），升级需整体验证。
