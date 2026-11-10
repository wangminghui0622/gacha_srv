package svc

import (
	"time"

	"gacha_srv/app/marketing/internal/config"
	"gacha_srv/app/money/moneyClient"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config config.Config
	DB     *gorm.DB

	TemplateModel   gamemodel.CouponTemplatesModel // 缓存模型：券模板（30s TTL，发布低频）
	CouponModel     gamemodelnc.UserCouponsModel   // 行锁资源（FOR UPDATE），nocache
	QuoteModel      gamemodelnc.QuoteLocksModel    // 行锁资源，nocache
	LoyaltyCfgModel gamemodelnc.LoyaltyOffsetConfigModel
	BarrierModel    gamemodelnc.DtmBarrierModel

	// M11 cache 模型：活动配置纯种子，运行时只读（30s TTL）
	CampaignModel gamemodel.CampaignsModel

	// M11 nocache 模型：行锁资源（FOR UPDATE）与幂等键写入
	CampaignSignInModel gamemodelnc.CampaignSignInsModel
	RedeemCodeModel     gamemodelnc.CampaignRedemptionCodesModel
	RedeemUserModel     gamemodelnc.CampaignRedemptionUsersModel
	ParticipationModel  gamemodelnc.CampaignParticipationsModel
	PrizeGrantModel     gamemodelnc.CampaignPrizeGrantsModel
	WheelSlotModel      gamemodelnc.CampaignWheelSlotsModel
	WheelDailyModel     gamemodelnc.CampaignWheelDailyModel
	EntitlementModel    gamemodelnc.CampaignBoxEntitlementsModel

	// MoneyRpc 资金服务客户端（QuoteCheckout 读积分余额、活动发奖 CreditWallet，走 Etcd；旧 UserRpc）
	MoneyRpc moneyClient.Money
}

func NewServiceContext(c config.Config) *ServiceContext {
	db, err := gorm.Open(mysql.Open(c.MySQL.DataSource), &gorm.Config{
		// 开启错误翻译（1062 → gorm.ErrDuplicatedKey），幂等路径依赖此判断
		TranslateError: true,
	})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}

	// 连接池参数：防止高并发打满 MySQL 连接数
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get underlying sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	// 进程退出时优雅关闭连接池
	proc.AddShutdownListener(func() {
		if err := sqlDB.Close(); err != nil {
			logx.Errorf("close sql.DB failed: %v", err)
		}
	})

	// ClientErrorInterceptor：下游业务码还原为 CodeError，跨服务不失真
	moneyRpc := moneyClient.NewMoney(zrpc.MustNewClient(c.MoneyRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))

	return &ServiceContext{
		Config:          c,
		DB:              db,
		TemplateModel:   gamemodel.NewCouponTemplatesModel(db, c.CacheRedis, cache.WithExpiry(30*time.Second)),
		CouponModel:     gamemodelnc.NewUserCouponsModel(db),
		QuoteModel:      gamemodelnc.NewQuoteLocksModel(db),
		LoyaltyCfgModel: gamemodelnc.NewLoyaltyOffsetConfigModel(db),
		BarrierModel:    gamemodelnc.NewDtmBarrierModel(db),

		CampaignModel: gamemodel.NewCampaignsModel(db, c.CacheRedis, cache.WithExpiry(30*time.Second)),

		CampaignSignInModel: gamemodelnc.NewCampaignSignInsModel(db),
		RedeemCodeModel:     gamemodelnc.NewCampaignRedemptionCodesModel(db),
		RedeemUserModel:     gamemodelnc.NewCampaignRedemptionUsersModel(db),
		ParticipationModel:  gamemodelnc.NewCampaignParticipationsModel(db),
		PrizeGrantModel:     gamemodelnc.NewCampaignPrizeGrantsModel(db),
		WheelSlotModel:      gamemodelnc.NewCampaignWheelSlotsModel(db),
		WheelDailyModel:     gamemodelnc.NewCampaignWheelDailyModel(db),
		EntitlementModel:    gamemodelnc.NewCampaignBoxEntitlementsModel(db),

		MoneyRpc: moneyRpc,
	}
}
