// Package errorx 错误码与错误类型（规范见 docs/standards/backend/05-errors.md）
//
// 分段表：0 成功；400-429 标准码；500 未知；600-699 基础设施；
// 业务码段 CodeBusinessMin~CodeBusinessMax 按服务序号×1000 划分（10 RPC 服务）：
// 1000 身份 / 2000 资金 / 3000 抽奖 / 4000 基础 / 5000 营销 /
// 6000 玩法 / 7000 任务 / 8000 商城 / 9000 第三方 / 10000 运营
// （新仓 10 服务重排：旧仓 8 段按 service-division.md §1 重映射，常量名保持不变，
// 数值按新服务段重排；新增服务先在 05-errors.md 登记域段）。
//
// 跨服务传递：ServerErrorInterceptor 把业务码（CodeBusinessMin~CodeBusinessMax）
// 直接作为 gRPC status code 传出，ClientErrorInterceptor 反向还原——业务码跨服务不失真。
// DTM SAGA 分支例外：分支只认 codes.Aborted（触发补偿）/codes.Internal（重试），
// 分支 logic 直接返回 status.Error，拦截器原样透出。
package errorx

import (
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// 业务码段边界（服务序号×1000：1 身份 2 资金 3 抽奖 4 基础 5 营销
// 6 玩法 7 任务 8 商城 9 第三方 10 运营；拦截器按此判定业务码直传/还原）
const (
	CodeBusinessMin = 1000  // 业务码段下限（含）
	CodeBusinessMax = 10999 // 业务码段上限（含）
)

// 标准码
const (
	CodeSuccess      = 0
	CodeParamError   = 400 // 参数错误（校验失败统一用这个）
	CodeUnauthorized = 401 // 未认证 / 令牌无效
	CodeForbidden    = 403 // 无权限
	CodeNotFound     = 404 // 资源不存在
	CodeTimeout      = 408 // 超时
	CodeRateLimit    = 429 // 限流
	CodeServerError  = 500 // 服务端未知错误
)

// 基础设施
const (
	CodeDBError     = 600 // 数据库错误
	CodeDBDuplicate = 601 // 数据库唯一键冲突
	CodeRedisError  = 610
	CodeEtcdError   = 620
	CodeDTMError    = 630
)

// 身份域 1000-1999（identity-rpc；旧仓用户域余额/冻结码迁往资金域 2000 段）
const (
	CodeUserNotFound     = 1001 // 用户不存在
	CodePasswordWrong    = 1002 // 密码错误
	CodeUsernameExists   = 1003 // 用户名已存在
	CodeTokenInvalid     = 1004 // 令牌无效
	// M15a 手机号/微信登录（验证码收发/微信接口错误走 9000 段 thirdparty）
	CodePhoneLoginInvalid   = 1011 // 未绑定手机号登录失败（server_v2 ErrInvalidPhoneLogin；验证码错归 9001）
	CodePhoneInUse          = 1012 // 手机号已被其他用户绑定（exact_token 唯一键仲裁）
	CodeThirdPartyNotLinked = 1013 // 第三方身份未绑定（server_v2 ErrThirdPartyNotLinked）
	CodePhoneSearchTooBroad = 1014 // 手机号片段搜索范围过大（候选超 500）
)

// 资金域 2000-2999（money-rpc = 旧 asset 域 + payment 域合并，两段并入后按域内顺序重排）
const (
	CodeBalanceNotEnough = 2001 // 余额不足（旧 1005）
	CodeAmountInvalid    = 2002 // 金额无效（旧 1006）
	CodeHoldNotFound     = 2003 // 冻结单不存在（旧 1009）
	CodeHoldBizMismatch  = 2004 // 冻结单业务信息不匹配（旧 1010）
	// 支付段（旧 8000 段并入）
	CodePaymentOrderNotFound = 2005 // 支付单不存在（旧 8001）
	CodePaymentStateInvalid  = 2006 // 支付单状态不满足操作（旧 8002）
	// M14d 控制台（支付路由存管）
	CodePaymentRouteNotFound        = 2007 // 支付路由策略不存在（旧 8003）
	CodePaymentRouteVersionConflict = 2008 // 支付路由版本冲突（expected_version CAS 失败，旧 8004）
	CodePaymentRouteInvalid         = 2009 // 支付路由校验失败（channel/scene/mode/candidates，旧 8005）
)

// 抽奖域 3000-3999（draw-rpc；旧仓游戏域 2000 段平移）
const (
	CodeBoxNotFound             = 3001 // 宝箱不存在（旧 2001）
	CodeBoxSoldOut              = 3002 // 宝箱已售罄（无任何可抽中奖条目）
	CodeBoxOffline              = 3003 // 宝箱已下架
	CodeRecordNotFound          = 3004 // 抽赏记录不存在
	CodeRollingRuleInvalid      = 3005 // 滚动中奖规则无效（percent 越界等）
	CodePoolDraftInvalid        = 3006 // 赏池草稿校验失败（M14b 控制台：字段/滚动规则非法）
	CodePoolVersionNotFound     = 3007 // 赏池版本不存在
	CodePoolVersionConflict     = 3008 // 赏池版本冲突（config_hash/状态不满足操作）
	CodePoolPublicationConflict = 3009 // 赏池发布审批冲突（自批/未提请/状态错）
	CodePoolDraftKeyConflict    = 3010 // 草稿幂等键冲突（同 key 不同载荷）
)

// 基础域 4000-4999（base-rpc 资格+风控；旧仓 5000 段平移）
const (
	CodePolicyDenied         = 4001 // 资格策略拒绝（旧 5001）
	CodeRiskRejected         = 4002 // 风控拒绝
	CodeRiskCaseNotFound     = 4003 // 风控案例不存在
	CodeRiskCaseStateInvalid = 4004 // 风控案例状态不满足操作
	// M14d 控制台（eligibility+risk 同属 base 域）
	CodePolicyNotFound  = 4006 // 条件政策不存在
	CodePolicyConflict  = 4007 // 条件政策版本冲突（expected_latest_version/config_hash 不符）
	CodePolicyInvalid   = 4008 // 条件政策校验失败（code/name/表达式非法/非 bool 输出）
	CodeRiskCaseInvalid = 4009 // 风控案例参数无效（case_id/operation_id/resolution_code）
)

// 营销域 5000-5999（marketing-rpc = 旧 promotion 13 + campaign 8；旧仓玩法域 3000 段拆出）
const (
	CodeCouponTemplateNotFound = 5001 // 券模板不存在（旧 3001）
	CodeCouponUnavailable      = 5002 // 券不可用（状态/归属/门槛/与积分互斥）
	CodeQuoteNotFound          = 5003 // 报价不存在或已过期
	CodeQuoteStateInvalid      = 5004 // 报价状态不满足操作

	CodeCampaignNotFound     = 5005 // 活动不存在
	CodeCampaignNotInWindow  = 5006 // 活动未开始或已结束
	CodeCampaignParamInvalid = 5007 // 活动参数无效（kind 与参数不匹配）
	CodeRedemptionInvalid    = 5008 // 兑换码无效
	CodeRedemptionExhausted  = 5009 // 兑换码已兑完
	CodeRedemptionUserLimit  = 5010 // 该用户兑换次数已达上限
	CodeWheelDailyLimit      = 5011 // 转盘今日次数已用完
	CodeWheelEmpty           = 5012 // 转盘奖池已空
	CodeEntitlementNotFound  = 5013 // 活动宝箱资格不存在
	CodeEntitlementUnavail   = 5014 // 活动宝箱资格不可用（状态/归属/箱型不符）

	// M14d 控制台（campaign+promotion 同属 marketing 域）
	CodeCampaignConflict       = 5015 // 活动状态/版本冲突（closed、草稿上加草稿、key/kind 不符）
	CodeCouponTemplateConflict = 5016 // 券模板冲突（同 request_id 异内容、head 版本 CAS 失败）
	CodeCouponInvalid          = 5017 // 发券/券模板参数无效（金额/bps/funder/custom bounds/区间）
)

// 玩法域 6000-6999（play-rpc 6 玩法+幸运指派+发奖标记；旧仓玩法域 3000 段拆出）
const (
	CodeDigBoxNotFound       = 6001 // 挖宝箱不存在（旧 3015）
	CodeDigCellOccupied      = 6002 // 所选格子已被挖开
	CodeDigSessionInvalid    = 6003 // 挖宝会话状态不满足操作
	CodeDigPriceMismatch     = 6004 // 挖宝价格校验不一致（满车/扭蛋价格校验复用）
	CodeDigRecordNotFound    = 6005 // 挖宝记录不存在

	CodeGameConfigInvalid     = 6006 // 玩法配置缺失或 JSON 无效
	CodeFullCarDeparted       = 6007 // 满车已出发（余票 0）
	CodeFullCarSeatOccupied   = 6008 // 座位已占
	CodeFullCarSeatInvalid    = 6009 // 座位越界
	CodeFullCarStateInvalid   = 6010 // 满车状态不满足操作
	CodeGashaponNoTickets     = 6011 // 扭蛋余票不足或已结算
	CodeGashaponConfigChanged = 6012 // 扭蛋配置变更（sent 长度与特别奖数不匹配）
	CodeLuckyAssignInvalid    = 6013 // 幸运指派无效（非 LUCKY 奖或用户未参与）
	CodeGashaponCountInvalid  = 6014 // 抽数非法（count<=0 或 >余票）

	CodeComboChainNoTickets     = 6015 // 连击链当前车已满或已结算（余票不足/状态不满足）
	CodeComboChainDupBuy        = 6016 // 连击链当前车重复购买（每用户每车限 1 票）
	CodeComboChainConfigChanged = 6017 // 连击链配置变更（参与人数与配置不符/全局奖缺失）
	CodeGroupBuyNoSlots         = 6018 // 拼团抽余位不足或已结算
	CodeGroupBuyCountInvalid    = 6019 // 拼团抽下注数非法（count<=0）
	CodeGroupBuyConfigChanged   = 6020 // 拼团抽配置变更（lucky 已发数与配置不匹配）

	// M15b 记忆翻牌 MemoryMatch
	CodeMemoryMatchPaymentUnsupported = 6021 // 支付方式暂不支持（本仓仅 WALLET）
	CodeMemoryMatchConflict           = 6022 // 会话状态冲突（命运已选异值/回合不匹配/版本冲突，server_v2 ErrKernelConflict）
	CodeMemoryMatchRoundNotFinished   = 6023 // 回合未完成（complete 时未 Terminal）
	CodeMemoryMatchRoundFinished      = 6024 // 回合已结束（pair/destiny-draw 时已 Terminal）
	CodeMemoryMatchDestinyUnavailable = 6025 // 命运抽牌不可用（有对子未配/资源耗尽/堆空/手牌满）
	CodeMemoryMatchSecondDestinyOff   = 6026 // 第二命运未启用
	CodeMemoryMatchAppearanceNotOwned = 6027 // 外观未拥有（非默认 preset 需权益）
	CodeMemoryMatchPrizeExhausted     = 6028 // 奖档售罄（结算发奖库存不足，整事务回滚可重试）
)

// 任务域 7000-7999（task-rpc 任务/签到/通行证/会员；旧仓玩法域 3000 段拆出，M15c）
const (
	CodeTaskNotReady         = 7001 // 任务未达标（progress 不足/签到条件未满足，server_v2 ErrTaskNotReady）
	CodeTaskFinished         = 7002 // 任务已领完（CycleLimit 或 Finished，server_v2 ErrTaskFinished）
	CodeTaskInvalid          = 7003 // 任务参数无效（server_v2 ErrInvalidTask）
	CodePassNotReady         = 7004 // 通行证领取条件未满足（等级未达/未解锁 premium/会员等级不足）
	CodePassSlotClaimed      = 7005 // 通行证槽位已领
	CodeMemberNotReady       = 7006 // 会员消费未达档位
	CodeMemberClaimed        = 7007 // 会员档已领
	CodeTaskStateConflict    = 7008 // 任务状态冲突（乐观锁失败）
	CodeApplyProgressInvalid = 7009 // 进度事件非法（SIGN_IN 拒收/metric 非法/amount<=0）
)

// 商城域 8000-8999（catalog-rpc，M14c 管理控制台；旧仓 4000 段平移）
const (
	CodeCatalogNotFound = 8001 // 商品不存在（含修订草稿不存在）
	CodeCatalogConflict = 8002 // 商品版本或状态冲突（版本不符/状态不符/草稿内容不符）
	CodeCatalogInvalid  = 8003 // 商品校验失败（code/title/option/sku/币种/组合非法）
)

// 第三方域 9000-9999（thirdparty-rpc；旧仓 6000 段平移）
const (
	CodeCaptchaInvalid      = 9001 // 验证码无效或已过期（一次性消费后重放）
	CodeCaptchaBusy         = 9002 // 验证码发送过于频繁（60s 重发/24h 超 10 次频控）
	CodeSmsDisabled         = 9003 // 短信通道未配置且非 DryRun
	CodePhoneInvalid        = 9004 // 手机号格式非法（非大陆 11 位手机号）
	CodeWechatNotConfigured = 9005 // 微信未配置（appid/secret 缺失）
	CodeWechatAPIError      = 9006 // 微信 API 错误（errcode≠0 / openid 缺失）
	// M15d integration 渠道收尾（微信小程序全家/支付宝/腾讯广告/provider 指标）
	CodeAlipayNotConfigured    = 9007 // 支付宝 OAuth 未配置（appid/私钥缺失）
	CodeAlipayAPIError         = 9008 // 支付宝 API 错误（含入参非法/空响应）
	CodeTencentAdNotConfigured = 9009 // 腾讯广告未配置
	CodeTencentAdRecordFailed  = 9010 // 广告点击记录失败（openid 缺失等）
	CodeMetricInvalidRequest   = 9011 // 指标接入请求非法（校验失败）
	CodeMetricConflict         = 9012 // 指标接入幂等冲突（同键异内容）
)

// 运营域 10000-10999（ops-rpc，M14 管理控制台；旧仓 7000 段平移）
const (
	CodeConsoleAccountNotFound     = 10001 // 控制台账号不存在
	CodeConsoleForbidden           = 10002 // 无权限（Authorize 拒绝）
	CodeConsoleTokenInvalid        = 10003 // 控制台令牌无效/已吊销
	CodeConsoleRoleNotFound        = 10004 // 角色不存在
	CodeConsoleRoleCodeExists      = 10005 // 角色编码已存在
	CodeConsoleAccountDisabled     = 10006 // 账号已停用
	CodeConsoleRequestIdMissing    = 10007 // 缺少 X-Request-Id
	CodeConsoleLoginInvalid        = 10008 // 用户名或密码错误
	CodeConsoleTotpInvalid         = 10009 // TOTP 验证码无效
	CodeConsoleLoginTicketInvalid  = 10010 // 登录票据无效或已过期
	CodeConsoleRoleVersionConflict = 10011 // 角色版本冲突（并发修改）
	CodeConsoleUnknownPermission   = 10012 // 未知权限（不在权限目录）
	CodeConsoleAccountExists       = 10013 // 账号已存在
	CodeConsoleRefreshInvalid      = 10014 // refresh 令牌无效（含指纹/家族不符）
	CodeConsoleIdempotencyRequired = 10015 // 缺少 Idempotency-Key 头（server_v2 IDEMPOTENCY_KEY_REQUIRED）
	// M14e 任务计划/报表控制台
	CodeTaskPlanNotFound      = 10016 // 任务计划（或审批行）不存在
	CodeTaskPlanConflict      = 10017 // 计划版本冲突（head CAS / 审批冲突）
	CodeTaskPlanInvalid       = 10018 // 任务计划校验失败
	CodeTaskPlanNotReady      = 10019 // 计划含 structured reward，原子发奖未就绪（发布拒绝）
	CodeDashboardNotFound     = 10020 // 看板不存在
	CodeDashboardConflict     = 10021 // 看板版本/状态冲突（CAS 失败）
	CodeDashboardInvalid      = 10022 // 看板校验失败（widget/网格/指标）
	CodeDashboardNotPublished = 10023 // 看板未发布（查询拒绝）
	// M14e 网关守卫（server_v2 kratos reason 同款）
	CodeConsoleTenantMismatch  = 10024 // 租户与会话不符（TENANT_MISMATCH）
	CodeConsoleDataScopeDenied = 10025 // 数据范围不足（DATA_SCOPE_DENIED）
	// M15d 指标事件（ApplyMetricEvent 内部 RPC）
	CodeMetricEventInvalid = 10026 // 指标事件非法（event_id/tenant 缺失）
)

// CodeError 业务错误：对外出口必须经过 FromError 归一
type CodeError struct {
	Code int
	Msg  string
}

func (e *CodeError) Error() string {
	return fmt.Sprintf("code=%d msg=%s", e.Code, e.Msg)
}

// NewCodeError 创建业务错误
func NewCodeError(code int, msg string) error {
	return &CodeError{Code: code, Msg: msg}
}

// NewCodeErrorf 创建业务错误（格式化消息）
func NewCodeErrorf(code int, format string, args ...any) error {
	return &CodeError{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// IsCodeError 判断是否为业务错误
func IsCodeError(err error) bool {
	var ce *CodeError
	return errors.As(err, &ce)
}

// FromError 归一化：业务错误原样返回；gRPC 业务码还原；其余折叠为 500（保留原始信息）
func FromError(err error) error {
	if err == nil {
		return nil
	}
	var ce *CodeError
	if errors.As(err, &ce) {
		return ce
	}
	var se interface{ GRPCStatus() *status.Status }
	if errors.As(err, &se) {
		s := se.GRPCStatus()
		if s.Code() >= CodeBusinessMin && s.Code() <= CodeBusinessMax {
			return &CodeError{Code: int(s.Code()), Msg: s.Message()}
		}
	}
	return &CodeError{Code: CodeServerError, Msg: err.Error()}
}

// FromMySQLError 把 MySQL/GORM 驱动错误折叠为基础设施码（见 05-errors 规则 6）
func FromMySQLError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return &CodeError{Code: CodeDBDuplicate, Msg: "数据已存在（唯一键冲突）"}
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		switch me.Number {
		case 1062:
			return &CodeError{Code: CodeDBDuplicate, Msg: "数据已存在（唯一键冲突）"}
		case 1213:
			return &CodeError{Code: CodeDBError, Msg: "死锁，请重试"}
		}
	}
	return &CodeError{Code: CodeDBError, Msg: "数据库错误"}
}

// mapStandardCode 标准码 → gRPC 码（ServerErrorInterceptor 用）
func mapStandardCode(code int) codes.Code {
	switch code {
	case CodeParamError:
		return codes.InvalidArgument
	case CodeUnauthorized:
		return codes.Unauthenticated
	case CodeForbidden:
		return codes.PermissionDenied
	case CodeNotFound:
		return codes.NotFound
	case CodeTimeout:
		return codes.DeadlineExceeded
	case CodeRateLimit:
		return codes.ResourceExhausted
	default:
		return codes.Internal
	}
}

// mapGRPCCode gRPC 码 → 标准码（ClientErrorInterceptor 用）
func mapGRPCCode(c codes.Code) int {
	switch c {
	case codes.InvalidArgument:
		return CodeParamError
	case codes.Unauthenticated:
		return CodeUnauthorized
	case codes.PermissionDenied:
		return CodeForbidden
	case codes.NotFound:
		return CodeNotFound
	case codes.DeadlineExceeded:
		return CodeTimeout
	case codes.ResourceExhausted:
		return CodeRateLimit
	default:
		return CodeServerError
	}
}
