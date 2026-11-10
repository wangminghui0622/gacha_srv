package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	MySQL struct {
		// DataSource 本服务数据库 DSN（gacha_draw）
		DataSource string
	}
	// CacheRedis 缓存模型专用 Redis（与业务 Redis 分开，见 draw-rpc.yaml）
	CacheRedis cache.CacheConf
}
