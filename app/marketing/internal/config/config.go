package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	MySQL struct {
		// DataSource 本服务数据库 DSN（gacha_marketing：报价锁/券/忠诚度/活动合并库）
		DataSource string
	}
	// CacheRedis 缓存模型（券模板/活动配置，30s 短 TTL）
	CacheRedis cache.CacheConf
	// MoneyRpc 资金服务客户端（QuoteCheckout 读积分余额、活动发奖 CreditWallet，走 Etcd；旧 UserRpc）
	MoneyRpc zrpc.RpcClientConf
}
