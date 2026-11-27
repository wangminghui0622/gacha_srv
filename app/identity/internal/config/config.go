package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	MySQL struct {
		// DataSource 本服务数据库 DSN（gacha_identity）
		DataSource string
	}
	JwtAuth struct {
		// AccessSecret JWT 签名密钥（占位符，生产替换走发布流程）
		AccessSecret string
		// AccessExpire 令牌有效期（秒）
		AccessExpire int64
	}
	// PhoneCrypto 手机号盲索引密钥（base64 32B，EncKey/IndexKey 必须不同；生产替换走发布流程）
	PhoneCrypto struct {
		EncKey       string
		IndexKey     string
		EncKeyID     string
		IndexVersion int64
	}
	// ThirdpartyRpc 第三方服务客户端（发码/微信 code 交换，走 Etcd）
	ThirdpartyRpc zrpc.RpcClientConf
	// MoneyRpc 钱包服务（邀请奖励吉祥值入账）
	MoneyRpc zrpc.RpcClientConf
	// Invite 邀请奖励配置（0 时使用默认值：10 吉祥值 / 10 天）
	Invite struct {
		RewardPoints    int64
		RewardValidDays int64
	}
}
