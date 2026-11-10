// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf

	// 运营服务客户端（会话/权限/审计，走 Etcd 服务发现）
	OpsRpc zrpc.RpcClientConf
	// 抽奖服务客户端（赏池控制台：草稿/版本/发布 + 箱子读取；M14b）
	DrawRpc zrpc.RpcClientConf
	// 营销服务客户端（活动/券模板/忠诚度默认值控制台；M11）
	MarketingRpc zrpc.RpcClientConf
	// 玩法服务客户端（幸运指派：扭蛋 LUCKY 奖指定用户；M14b）
	PlayRpc zrpc.RpcClientConf
	// 商城服务客户端（商品目录控制台：SPU/SKU 草稿/发布/修订；M14c）
	CatalogRpc zrpc.RpcClientConf
	// 基础服务客户端（条件政策/风控案件控制台；M14d）
	BaseRpc zrpc.RpcClientConf
	// 资金服务客户端（支付路由存管控制台；M14d）
	MoneyRpc zrpc.RpcClientConf

	// 控制台令牌本地验签（不依赖 ops-rpc 在线）
	Console ConsoleConf

	// 会话吊销检查 Redis
	CacheRedis cache.CacheConf
}

// ConsoleConf 控制台令牌验签配置。
type ConsoleConf struct {
	// Ed25519 公钥 base64（32 字节）
	PublicKeyBase64 string
	Issuer          string
	Audience        string
	// fail-closed 路由校验目标里程碑（如 M14a）
	FailClosedMilestone string
}
