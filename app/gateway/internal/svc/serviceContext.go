// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package svc

import (
	"gacha_srv/app/base/baseClient"
	"gacha_srv/app/draw/drawClient"
	"gacha_srv/app/gateway/internal/config"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/identity/identityClient"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/money/moneyClient"
	"gacha_srv/app/play/playClient"
	"gacha_srv/app/task/taskClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config  config.Config
	JwtAuth rest.Middleware
	// JwtAuthOptional 可选认证（M15c GET /tasks 公开浏览：带令牌校验，无令牌放行）
	JwtAuthOptional rest.Middleware

	// 服务拆分后下游客户端：旧 UserRpc → IdentityRpc（账号/会话）+ MoneyRpc（钱包/资产/支付）；
	// 旧 GameRpc → MarketingRpc（报价/券/活动）+ PlayRpc（6 玩法）+ TaskRpc（任务/签到/会员）
	IdentityRpc  identityClient.Identity
	MoneyRpc     moneyClient.Money
	DrawRpc      drawClient.Draw
	BaseRpc      baseClient.Base
	MarketingRpc marketingClient.Marketing
	PlayRpc      playClient.Play
	TaskRpc      taskClient.Task
}

func NewServiceContext(c config.Config) *ServiceContext {
	// ClientErrorInterceptor：下游业务码还原为 CodeError，跨服务不失真（规范 05-errors 规则 5）
	identityRpc := identityClient.NewIdentity(zrpc.MustNewClient(c.IdentityRpc, zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	moneyRpc := moneyClient.NewMoney(zrpc.MustNewClient(c.MoneyRpc, zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	drawRpc := drawClient.NewDraw(zrpc.MustNewClient(c.DrawRpc, zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	baseRpc := baseClient.NewBase(zrpc.MustNewClient(c.BaseRpc, zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	marketingRpc := marketingClient.NewMarketing(zrpc.MustNewClient(c.MarketingRpc, zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	playRpc := playClient.NewPlay(zrpc.MustNewClient(c.PlayRpc, zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	taskRpc := taskClient.NewTask(zrpc.MustNewClient(c.TaskRpc, zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	jwtAuth := middleware.NewJwtAuthMiddleware(identityRpc)
	return &ServiceContext{
		Config:          c,
		JwtAuth:         jwtAuth.Handle,
		JwtAuthOptional: jwtAuth.OptionalHandle,
		IdentityRpc:     identityRpc,
		MoneyRpc:        moneyRpc,
		DrawRpc:         drawRpc,
		BaseRpc:         baseRpc,
		MarketingRpc:    marketingRpc,
		PlayRpc:         playRpc,
		TaskRpc:         taskRpc,
	}
}
