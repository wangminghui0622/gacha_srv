package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	MySQL struct {
		// DataSource 本服务数据库 DSN（gacha_ops）
		DataSource string
	}
	// CacheRedis 业务 Redis：登录票据（TOTP challenge，5 分钟）与
	// 会话吊销键 console:session:revoked:<jti>（模型全 nocache，不建模型缓存）
	CacheRedis cache.CacheConf
	// Console 控制台令牌与会话参数（照抄 server_v2，网关持公钥本地验签）
	Console ConsoleConf
}

// ConsoleConf 控制台会话/初始化账号配置。
type ConsoleConf struct {
	// PrivateKeyBase64 Ed25519 私钥（PKCS8 DER 的 base64，32 字节；仅本服务持有）
	PrivateKeyBase64 string
	// PublicKeyBase64 Ed25519 公钥（32 字节 base64；部署时写入管理网关配置）
	PublicKeyBase64 string
	Issuer          string
	Audience        string
	// AccessTtlSeconds access 令牌有效期（默认 900）
	AccessTtlSeconds int64
	// RefreshTtlSeconds refresh 令牌有效期（默认 604800）
	RefreshTtlSeconds int64
	// 首次启动自动开通的超级管理员（仅本地开发；生产部署首登改密）
	InitialAdminUsername    string
	InitialAdminPassword    string
	InitialAdminDisplayName string
	InitialAdminRoleCode    string
	// SkipTotp 跳过 TOTP（仅本地开发调试；生产必须 false）
	SkipTotp bool
}
