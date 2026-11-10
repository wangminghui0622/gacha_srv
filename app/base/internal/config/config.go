package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	MySQL struct {
		// DataSource 本服务数据库 DSN（gacha_base）
		DataSource string
	}
	// BizRedis 业务 Redis：场景→生效策略单号映射（gacha:base:policy:scene:{scene}，30s TTL）
	// 命名避开 RpcServerConf 内嵌的 Redis 键（冲突）
	BizRedis redis.RedisConf
	// CacheRedis 缓存模型专用 Redis（与业务 Redis 分开，见 base-rpc.yaml）
	CacheRedis cache.CacheConf
}
