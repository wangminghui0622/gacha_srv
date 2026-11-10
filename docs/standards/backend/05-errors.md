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
| `1000-1999` | 身份域（identity 9311） | `1001 用户不存在` `1002 密码错误` `1003 用户名已存在` `1004 令牌无效` `1011 未绑定手机号登录失败（验证码错归 9001）` `1012 手机号已被其他用户绑定` `1013 第三方身份未绑定` `1014 手机号片段搜索范围过大` |
| `2000-2999` | 资金域（money 9312，资产+支付合并） | `2001 余额不足` `2002 金额无效` `2003 冻结单不存在` `2004 冻结单业务不匹配` `2005 支付单不存在` `2006 支付单状态不满足操作` `2007 支付路由策略不存在` `2008 支付路由版本冲突` `2009 支付路由校验失败` |
| `3000-3999` | 抽奖域（draw 9313） | `3001 宝箱不存在` `3002 宝箱已售罄` `3003 宝箱已下架` `3004 抽赏记录不存在` `3005 滚动中奖规则无效` `3006 赏池草稿无效（校验不通过）` `3007 赏池版本不存在` `3008 赏池版本冲突（哈希不符/状态不符）` `3009 赏池发布审批冲突（自批/未批准/审批行不符）` `3010 草稿幂等键冲突（同键异载荷）` |
| `4000-4999` | 基础域（base 9314，资格/风控） | `4001 资格策略拒绝` `4002 风控拒绝` `4003 风控案例不存在` `4004 风控案例状态不满足操作` `4006 条件政策不存在` `4007 条件政策版本冲突（expected_latest_version/config_hash 不符）` `4008 条件政策校验失败` `4009 风控案例参数无效` |
| `5000-5999` | 营销域（marketing 9315，营销+活动合并） | `5001 券模板不存在` `5002 券不可用` `5003 报价不存在或已过期` `5004 报价状态不满足操作` `5005 活动不存在` `5006 活动未开始或已结束` `5007 活动参数无效` `5008 兑换码无效` `5009 兑换码已兑完` `5010 该用户兑换次数已达上限` `5011 转盘今日次数已用完` `5012 转盘奖池已空` `5013 活动宝箱资格不存在` `5014 活动宝箱资格不可用` `5015 活动状态/版本冲突（closed、草稿上加草稿、key/kind 不符）` `5016 券模板冲突（同 request_id 异内容、head 版本 CAS 失败）` `5017 发券/券模板参数无效` |
| `6000-6999` | 玩法域（play 9316） | `6001 挖宝箱不存在` `6002 所选格子已被挖开` `6003 挖宝会话状态不满足操作` `6004 挖宝价格校验不一致` `6005 挖宝记录不存在` `6006 玩法配置缺失或 JSON 无效` `6007 满车已出发` `6008 座位已占` `6009 座位越界` `6010 满车状态不满足操作` `6011 扭蛋余票不足或已结算` `6012 扭蛋配置变更` `6013 幸运指派无效` `6014 抽数非法` `6015 连击链当前车已满或已结算` `6016 连击链当前车重复购买` `6017 连击链配置变更` `6018 拼团抽余位不足或已结算` `6019 拼团抽下注数非法` `6020 拼团抽配置变更` `6021 支付方式暂不支持（记忆翻牌仅 WALLET）` `6022 会话状态冲突（命运已选异值/回合不匹配/版本冲突）` `6023 回合未完成（complete 时未 Terminal）` `6024 回合已结束（pair/destiny-draw 时已 Terminal）` `6025 命运抽牌不可用（有对子未配/资源耗尽/堆空/手牌满）` `6026 第二命运未启用` `6027 外观未拥有` `6028 奖档售罄` |
| `7000-7999` | 任务域（task 9317） | `7001 任务未达标` `7002 任务已领完` `7003 任务参数无效` `7004 通行证领取条件未满足` `7005 通行证槽位已领` `7006 会员消费未达档位` `7007 会员档已领` `7008 任务状态冲突（乐观锁失败）` `7009 进度事件非法` |
| `8000-8999` | 商城域（catalog 9318） | `8001 商品不存在（含修订草稿不存在）` `8002 商品版本或状态冲突（版本不符/状态不符/草稿内容不符）` `8003 商品校验失败（code/title/option/sku/币种/组合非法）` |
| `9000-9999` | 第三方域（thirdparty 9319） | `9001 验证码无效或已过期（一次性消费）` `9002 验证码发送过于频繁（60s/24h 10 次频控）` `9003 短信通道未配置且非 DryRun` `9004 手机号格式非法` `9005 微信未配置（appid/secret 缺失）` `9006 微信 API 错误（errcode≠0/openid 缺失）` `9007 支付宝 OAuth 未配置（appid/私钥缺失）` `9008 支付宝 API 错误（含入参非法/空响应）` `9009 腾讯广告未配置` `9010 广告点击记录失败（openid 缺失等）` `9011 指标接入请求非法（校验失败）` `9012 指标接入幂等冲突（同键异内容）` |
| `10000-10999` | 运营域（ops 9320，M14 管理控制台） | `10001 控制台账号不存在` `10002 无权限（Authorize 拒绝）` `10003 控制台令牌无效/已吊销` `10004 角色不存在` `10005 角色编码已存在` `10006 账号已停用` `10007 缺少 X-Request-Id` `10008 用户名或密码错误` `10009 TOTP 验证码无效` `10010 登录票据无效或已过期` `10011 角色/账号角色版本冲突（并发修改）` `10012 未知权限（不在权限目录）` `10013 账号已存在` `10014 refresh 令牌无效（含指纹/家族不符）` `10015 缺少 Idempotency-Key 头` `10016 任务计划（或审批行）不存在` `10017 计划版本冲突` `10018 任务计划校验失败` `10019 计划含 structured reward 发布拒绝` `10020 看板不存在` `10021 看板版本/状态冲突` `10022 看板校验失败` `10023 看板未发布` `10024 租户与会话不符（TENANT_MISMATCH）` `10025 数据范围不足（DATA_SCOPE_DENIED）` `10026 指标事件非法（event_id/tenant 缺失）` |

业务码段边界统一用 `errorx.CodeBusinessMin`(1000) / `errorx.CodeBusinessMax`(10999) 常量，
域段按 1000 递增划分；拦截器按该边界判定业务码直传/还原。

**新增服务第一件事：在本表登记域段。** 同一错误只允许一个码，删除的错误码不复用。

### 管理网关错误体（D9 信封例外）

管理控制台网关（app/admin-gateway 9321）**不使用**播放器网关的恒 200 `{code,msg,data}` 信封：
- 错误体为 kratos 形 `{"code":<HTTP状态码>,"message":"...","reason":"<机器码>"}`，HTTP 状态码 = code；
- 10000-10999 业务码映射：10001→404 CONSOLE_ACCOUNT_NOT_FOUND、10002→403 CONSOLE_FORBIDDEN、
  10003→401 CONSOLE_TOKEN_INVALID、10004→404 CONSOLE_ROLE_NOT_FOUND、10005→409 CONSOLE_ROLE_CODE_EXISTS、
  10006→403 CONSOLE_ACCOUNT_DISABLED、10007→400 CONSOLE_REQUEST_ID_MISSING、10008→401 CONSOLE_LOGIN_INVALID、
  10009→401 CONSOLE_TOTP_INVALID、10010→401 CONSOLE_LOGIN_TICKET_INVALID、10011→409 CONSOLE_VERSION_CONFLICT、
  10012→400 CONSOLE_UNKNOWN_PERMISSION、10013→409 CONSOLE_ACCOUNT_EXISTS、10014→401 CONSOLE_REFRESH_INVALID、
  10015→400 IDEMPOTENCY_KEY_REQUIRED；
  域码透传（M14b 赏池控制台）：3006→400 POOL_DRAFT_INVALID、3007→404 POOL_VERSION_NOT_FOUND、
  3008→409 POOL_VERSION_CONFLICT、3009→409 POOL_PUBLICATION_CONFLICT、3010→409 POOL_DRAFT_REQUEST_CONFLICT、
  6013→400 LUCKY_ASSIGN_INVALID（assign-lucky 经 pool 控制台暴露）；
  域码透传（M14c 商品目录控制台）：8001→404 PRODUCT_NOT_FOUND、8002→409 PRODUCT_CONFLICT、
  8003→400 PRODUCT_INVALID；
  域码透传（M14d 控制台）：5005→404 CAMPAIGN_NOT_FOUND、5015→409 CAMPAIGN_CONFLICT、
  5007→400 CAMPAIGN_INVALID、5001→404 COUPON_TEMPLATE_NOT_FOUND、5016→409 COUPON_TEMPLATE_CONFLICT、
  5017→400 COUPON_INVALID、4006→404 POLICY_NOT_FOUND、4007→409 POLICY_CONFLICT、
  4008→400 POLICY_INVALID、4003→404 RISK_CASE_NOT_FOUND、4004→409 RISK_CASE_CONFLICT、
  4009→400 RISK_CASE_INVALID、2007→404 PAYMENT_ROUTE_NOT_FOUND、2008→409
  PAYMENT_ROUTE_VERSION_CONFLICT、2009→400 PAYMENT_ROUTE_INVALID；
  域码透传（M14e 任务计划/报表控制台）：10016→404 TASK_PLAN_NOT_FOUND、10017→409
  TASK_PLAN_CONFLICT、10018→400 TASK_PLAN_INVALID、10019→400 TASK_PLAN_NOT_READY、
  10020→404 DASHBOARD_NOT_FOUND、10021→409 DASHBOARD_CONFLICT、10022→400 DASHBOARD_INVALID、
  10023→400 DASHBOARD_NOT_PUBLISHED、10024→403 TENANT_MISMATCH、10025→403 DATA_SCOPE_DENIED；
  5xx 时 message 一律 "内部错误"（详情只进日志）；gRPC Unavailable→503、超时→504。
- 成功体为 protojson（camelCase、uint64 为 JSON 字符串、枚举为名字面量），与 server_v2 kratos 输出逐字一致。
- 映射实现：app/admin-gateway/internal/restx/restx.go `MapError`/`Error`。

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

4. RPC 服务出口挂 `ServerErrorInterceptor`：业务码（`CodeBusinessMin`~`CodeBusinessMax`，1000-10999）直接作为 gRPC status code 传出；标准码映射为对应 gRPC 码。
5. 网关调下游挂 `ClientErrorInterceptor`：反向还原为 CodeError，业务码跨服务不失真。

## 适配器

6. `FromMySQLError`：1062 → `CodeDBDuplicate`（提取键名进 msg）、1213 → 死锁重试、1048 → `CodeParamError`、连接错误字符串识别。
7. `WrapRedisError` / `FromRedisError`：`redis.Nil` 视为缓存未命中（返回 nil，不是错误）；其余 → `CodeRedisError`。

## 使用规则

8. Logic 层：预定义错误直接返回；未知错误 `fmt.Errorf("中文上下文: %w", err)` 包装。
9. 禁止：裸 `errors.New` 直接返回给调用方、把驱动/框架原始错误透出网关、错误信息里带 SQL 或敏感数据。
10. 校验器（validator）失败返回 `CodeParamError`，msg 为翻译后的中文第一项，不允许被吞成 500。
