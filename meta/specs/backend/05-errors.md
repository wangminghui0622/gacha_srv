# 错误处理规范（errorx）

## 错误码分段表

| 段 | 含义 | 示例 |
|----|------|------|
| `0` | 成功 | — |
| `400` | 参数错误（校验失败统一用这个） | `CodeParamError` |
| `401` | 未认证 / 令牌无效 | `CodeUnauthorized` |
| `403` | 无权限 | `CodeForbidden` |
| `404` | 资源不存在 | `CodeNotFound` |
| `408` | 超时 | `CodeTimeout` |
| `429` | 限流 | `CodeRateLimit` |
| `500` | 服务端未知错误 | `CodeServerError` |
| `600-699` | 基础设施：600 DB、601 DB 重复、610 Redis、620 Etcd、630 DTM | `CodeDBError` |
| `1000-1999` | 用户域 | `1001 用户不存在` |
| `2000-2999` | 游戏域 | `2001 余额不足` |
| `3000-3999` | 支付域 | `3001 支付渠道不可用` |
| `4000-4999` | 代理域 | — |
| `5000-5999` | VIP 域 | — |

**新增服务第一件事：在本表登记域段。** 同一错误只允许一个码，删除的错误码不复用。

## 定义方式

1. 核心类型与构造器：

```go
type CodeError struct { Code int; Msg string }
func NewCodeError(code int, msg string) error
func NewCodeErrorf(code int, format string, args ...interface{}) error
```

2. 常用错误定义单例（中文 msg）：`ErrUserNotFound`、`ErrPasswordWrong`、`ErrBalanceNotEnough`、`ErrTokenInvalid`……
3. 判断与归一：`IsCodeError(err)`；`FromError(err)` 把未知错误折叠为 `CodeServerError`（保留原始信息）——**对外出口必须经过它**。

## 跨服务传递（gRPC）

4. RPC 服务出口挂 `ServerErrorInterceptor`：业务码（1000-5999）直接作为 gRPC status code 传出；标准码映射为对应 gRPC 码。
5. 网关调下游挂 `ClientErrorInterceptor`：反向还原为 CodeError，业务码跨服务不失真。

## 适配器

6. `FromMySQLError`：1062 → `CodeDBDuplicate`（提取键名进 msg）、1213 → 死锁重试、1048 → `CodeParamError`、连接错误字符串识别。
7. `WrapRedisError` / `FromRedisError`：`redis.Nil` 视为缓存未命中（返回 nil，不是错误）；其余 → `CodeRedisError`。

## 使用规则

8. Logic 层：预定义错误直接返回；未知错误 `fmt.Errorf("中文上下文: %w", err)` 包装。
9. 禁止：裸 `errors.New` 直接返回给调用方、把驱动/框架原始错误透出网关、错误信息里带 SQL 或敏感数据。
10. 校验器（validator）失败返回 `CodeParamError`，msg 为翻译后的中文第一项，不允许被吞成 500。
