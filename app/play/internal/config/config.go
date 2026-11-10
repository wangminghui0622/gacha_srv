package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	MySQL struct {
		// DataSource 本服务数据库 DSN（gacha_play：六玩法状态/记录/会话合并库）
		DataSource string
	}
	// CacheRedis 缓存模型（玩法配置/奖池条目，30s 短 TTL）
	CacheRedis cache.CacheConf
	// MoneyRpc 资金服务客户端（玩法发奖现金/积分 CreditWallet 入账，走 Etcd；旧 UserRpc）
	MoneyRpc zrpc.RpcClientConf
	// MarketingRpc 营销服务客户端（锁报价/读报价/券名/发券，走 Etcd；旧同服务直连表）
	MarketingRpc zrpc.RpcClientConf
}
