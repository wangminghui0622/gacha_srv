// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package svc

import (
	"crypto/ed25519"
	"encoding/base64"

	"gacha_srv/app/base/baseClient"
	"gacha_srv/app/catalog/catalogClient"
	"gacha_srv/app/draw/drawClient"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/money/moneyClient"
	"gacha_srv/app/ops/opsClient"
	"gacha_srv/app/play/playClient"
	"gacha_srv/app/admin-gateway/internal/config"
	"gacha_srv/app/admin-gateway/internal/middleware"
	"gacha_srv/pkg/utils/errorx"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config      config.Config
	OpsRpc      opsClient.Ops
	DrawRpc     drawClient.Draw
	MarketingRpc marketingClient.Marketing
	PlayRpc     playClient.Play
	CatalogRpc  catalogClient.Catalog
	BaseRpc     baseClient.Base
	MoneyRpc    moneyClient.Money
	Rdb         *redis.Client
	PubKey      ed25519.PublicKey

	RequestId        rest.Middleware
	ConsoleAuth      rest.Middleware
	ConsoleAuthorize rest.Middleware
}

func NewServiceContext(c config.Config) *ServiceContext {
	// Ed25519 公钥（本地验签控制台 JWT；与 ops-rpc 的 Console.PrivateKeyBase64 配对）
	pub, err := base64.StdEncoding.DecodeString(c.Console.PublicKeyBase64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		panic("Console.PublicKeyBase64 无效：需要 32 字节 Ed25519 公钥（base64）")
	}
	if c.Console.Issuer == "" || c.Console.Audience == "" {
		panic("Console.Issuer/Audience 必填（与 ops-rpc 签发配置一致）")
	}

	// 运营服务客户端（会话/权限/审计；业务码跨服务还原为 CodeError）
	opsRpc := opsClient.NewOps(zrpc.MustNewClient(c.OpsRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	// 抽奖/玩法服务客户端（M14b 赏池控制台；同样还原业务码）
	drawRpc := drawClient.NewDraw(zrpc.MustNewClient(c.DrawRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	// 营销服务客户端（活动/券模板/忠诚度默认值控制台；同样还原业务码）
	marketingRpc := marketingClient.NewMarketing(zrpc.MustNewClient(c.MarketingRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	// 玩法服务客户端（幸运指派：扭蛋 LUCKY 奖指定用户；同样还原业务码）
	playRpc := playClient.NewPlay(zrpc.MustNewClient(c.PlayRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	// 商城服务客户端（M14c 商品目录控制台；同样还原业务码）
	catalogRpc := catalogClient.NewCatalog(zrpc.MustNewClient(c.CatalogRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	// 基础服务客户端（M14d 条件政策/风控案件控制台；同样还原业务码）
	baseRpc := baseClient.NewBase(zrpc.MustNewClient(c.BaseRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	// 资金服务客户端（M14d 支付路由存管控制台；同样还原业务码）
	moneyRpc := moneyClient.NewMoney(zrpc.MustNewClient(c.MoneyRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))

	// 会话吊销检查 Redis（console:session:revoked:<jti>）
	if len(c.CacheRedis) == 0 {
		panic("管理网关需要配置 CacheRedis（会话吊销检查）")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.CacheRedis[0].Host,
		Password: c.CacheRedis[0].Pass,
		DB:       0,
	})

	auth := middleware.NewConsoleAuthMiddleware(ed25519.PublicKey(pub), c.Console.Issuer, c.Console.Audience, rdb)

	return &ServiceContext{
		Config:           c,
		OpsRpc:           opsRpc,
		DrawRpc:          drawRpc,
		MarketingRpc:     marketingRpc,
		PlayRpc:          playRpc,
		CatalogRpc:       catalogRpc,
		BaseRpc:          baseRpc,
		MoneyRpc:         moneyRpc,
		Rdb:              rdb,
		PubKey:           ed25519.PublicKey(pub),
		RequestId:        middleware.NewRequestIdMiddleware().Handle,
		ConsoleAuth:      auth.Handle,
		ConsoleAuthorize: middleware.NewConsoleAuthorizeMiddleware(opsRpc, auth).Handle,
	}
}
