package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	// Redis 走 go-zero 内建段（RpcServerConf.Redis，Host/Type 必填）：
	// 验证码存储与频控（server_v2 integration/sms RedisCaptchaStore）+ 微信小程序 access_token store
	// MySQL：M15d 起 gacha_thirdparty（provider 指标接入 + 腾讯广告点击 + outbox 投递）
	MySQL struct {
		DataSource string
	}
	Sms struct {
		// DryRun 开发模式：不真实发短信（验证码仍写 Redis，可读取调试）
		DryRun bool
		Aliyun struct {
			Enabled         bool
			AccessKeyID     string
			AccessKeySecret string
			SignName        string
			TemplateCode    string
		}
		Tencent struct {
			Enabled         bool
			AccessKeyID     string
			AccessKeySecret string
			SignName        string
			TemplateID      string
			AppID           string
		}
	}
	Wechat struct {
		Mini struct {
			AppID     string
			AppSecret string
			// APIBase jscode2session/token/手机号/二维码/urllink 基础地址（空=微信官方；场景测试指向 mock server）
			APIBase string
			// DefaultPage 小程序码未传 page 时的兜底页
			DefaultPage string
		}
		App struct {
			AppID     string
			AppSecret string
			// OAuthBase oauth2 access_token 基础地址（空=微信官方；场景测试指向 mock server）
			OAuthBase string
		}
	}
	// Alipay 支付宝 OAuth（D3：不引 gopay，crypto/rsa 手写 RSA2；公钥空=跳过验签开发模式）
	Alipay struct {
		AppID           string
		AppPrivateKey   string
		AlipayPublicKey string
		Production      bool
		// APIBase 支付宝网关基础地址（空=openapi.alipay.com；场景测试指向本地 alipaymock）
		APIBase string
	}
	// TencentAd 腾讯广告（M15d 只克隆 RecordClick 去重落库；ReportAction 死代码不搬）
	TencentAd struct {
		Accounts  []TencentAdAccount
		MiniAppID string
	}
	// OpsRpc 运营服务客户端（M15d 指标事件 outbox 直投 ApplyMetricEvent，走 Etcd）
	OpsRpc zrpc.RpcClientConf
}

// TencentAdAccount 广告账户（server_v2 tencentad.Account 同款字段）。
type TencentAdAccount struct {
	AccountID       int64
	AccessToken     string
	UserActionSetID int64
}
