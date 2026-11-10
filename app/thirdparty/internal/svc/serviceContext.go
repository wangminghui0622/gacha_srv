package svc

import (
	"errors"

	"gacha_srv/app/thirdparty/internal/alipay"
	"gacha_srv/app/thirdparty/internal/config"
	"gacha_srv/app/thirdparty/internal/outbox"
	"gacha_srv/app/thirdparty/internal/repo"
	"gacha_srv/app/thirdparty/internal/sms"
	"gacha_srv/app/thirdparty/internal/tencentad"
	"gacha_srv/app/thirdparty/internal/wechat"
	"gacha_srv/app/ops/opsClient"
	thirdpartymodel "gacha_srv/pkg/model_nocache/gacha_thirdparty"
	"gacha_srv/pkg/utils/errorx"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config  config.Config
	Redis   *redis.Client
	DB      *gorm.DB
	Captcha *sms.CaptchaService
	Wechat  *wechat.Gateway
	// M15d gacha_thirdparty 模型（provider 指标接入 + 腾讯广告点击 + outbox）
	MetricBatchModel   thirdpartymodel.IntegrationMetricBatchModel
	MetricRawModel     thirdpartymodel.IntegrationMetricRawModel
	MetricCurrentModel thirdpartymodel.IntegrationMetricCurrentModel
	MetricOutboxModel  thirdpartymodel.IntegrationMetricOutboxModel
	TxAdClickModel     thirdpartymodel.IntegrationTxAdClickModel
	// M15d 编排层（D2：指标 Ingest 存储编排 + 单实例 outbox 直投 ops-rpc 调度器）
	MetricStore      *repo.MetricStore
	Alipay           *alipay.OAuthClient
	TencentAd        *tencentad.Service
	MetricDispatcher *outbox.Dispatcher
}

// TencentAdConfigured 腾讯广告是否可用（server_v2 TxAdClick 的 nil 守卫同款语义：
// 无任何账户配置 ⇒ 6009；RecordClick 本身只依赖去重落库，不触账户）。
func (s *ServiceContext) TencentAdConfigured() bool {
	return len(s.Config.TencentAd.Accounts) > 0
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 内建 Redis 段（Host/Type 走 go-zero 校验），客户端直连 go-redis v9
	rdb := redis.NewClient(&redis.Options{Addr: c.Redis.Host, Password: c.Redis.Pass})
	proc.AddShutdownListener(func() {
		if err := rdb.Close(); err != nil {
			logx.Errorf("close redis failed: %v", err)
		}
	})

	// M15d：gacha_thirdparty（指标/广告点击一致性优先，无缓存）
	db, err := gorm.Open(mysql.Open(c.MySQL.DataSource), &gorm.Config{
		// 开启错误翻译（1062 → gorm.ErrDuplicatedKey），与 ops-rpc 同款
		TranslateError: true,
	})
	if err != nil {
		panic("failed to init gacha_thirdparty db: " + err.Error())
	}

	sender, err := sms.NewSender(sms.SMSConfig{
		DryRun: c.Sms.DryRun,
		Aliyun: sms.AliyunSMS{
			Enabled:         c.Sms.Aliyun.Enabled,
			AccessKeyID:     c.Sms.Aliyun.AccessKeyID,
			AccessKeySecret: c.Sms.Aliyun.AccessKeySecret,
			SignName:        c.Sms.Aliyun.SignName,
			TemplateCode:    c.Sms.Aliyun.TemplateCode,
		},
		Tencent: sms.TencentSMS{
			Enabled:         c.Sms.Tencent.Enabled,
			AccessKeyID:     c.Sms.Tencent.AccessKeyID,
			AccessKeySecret: c.Sms.Tencent.AccessKeySecret,
			SignName:        c.Sms.Tencent.SignName,
			TemplateID:      c.Sms.Tencent.TemplateID,
			AppID:           c.Sms.Tencent.AppID,
		},
	})
	if err != nil {
		panic("failed to init sms sender: " + err.Error())
	}

	// M15d：支付宝 OAuth（懒装配：appid/私钥缺失 ⇒ nil ⇒ logic 映射 6007，server_v2 同款）
	alipayClient, err := alipay.NewOAuthClient(alipay.Config{
		AppID:           c.Alipay.AppID,
		AppPrivateKey:   c.Alipay.AppPrivateKey,
		AlipayPublicKey: c.Alipay.AlipayPublicKey,
		Production:      c.Alipay.Production,
		APIBase:         c.Alipay.APIBase,
	})
	if err != nil && !errors.Is(err, alipay.ErrNotConfigured) {
		panic("failed to init alipay oauth: " + err.Error())
	}
	if errors.Is(err, alipay.ErrNotConfigured) {
		alipayClient = nil
	}

	batchModel := thirdpartymodel.NewIntegrationMetricBatchModel(db)
	outboxModel := thirdpartymodel.NewIntegrationMetricOutboxModel(db)

	// M15d D2：指标事件 outbox 单实例轮询 → ops-rpc ApplyMetricEvent（替代老项目 Kafka）
	opsRpc := opsClient.NewOps(zrpc.MustNewClient(c.OpsRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	dispatcher, err := outbox.NewDispatcher(outboxModel, outbox.NewRpcPublisher(opsRpc), outbox.DefaultOptions())
	if err != nil {
		panic("failed to init metric dispatcher: " + err.Error())
	}

	return &ServiceContext{
		Config: c,
		Redis:  rdb,
		DB:     db,
		Captcha: sms.NewCaptchaService(
			sms.NewRedisCaptchaStore(rdb),
			sender,
		),
		Wechat: wechat.NewGateway(wechat.Config{
			Mini: wechat.MiniProgramConfig{
				AppID:       c.Wechat.Mini.AppID,
				AppSecret:   c.Wechat.Mini.AppSecret,
				APIBase:     c.Wechat.Mini.APIBase,
				DefaultPage: c.Wechat.Mini.DefaultPage,
			},
			App: wechat.MobileAppConfig{
				AppID:     c.Wechat.App.AppID,
				AppSecret: c.Wechat.App.AppSecret,
				OAuthBase: c.Wechat.App.OAuthBase,
			},
		}, wechat.NewTokenStore(rdb)),
		MetricBatchModel:   batchModel,
		MetricRawModel:     thirdpartymodel.NewIntegrationMetricRawModel(db),
		MetricCurrentModel: thirdpartymodel.NewIntegrationMetricCurrentModel(db),
		MetricOutboxModel:  outboxModel,
		TxAdClickModel:     thirdpartymodel.NewIntegrationTxAdClickModel(db),
		MetricStore:        repo.NewMetricStore(db, batchModel),
		Alipay:             alipayClient,
		TencentAd:          tencentad.NewService(repo.NewTxAdClickStore(thirdpartymodel.NewIntegrationTxAdClickModel(db))),
		MetricDispatcher:   dispatcher,
	}
}
