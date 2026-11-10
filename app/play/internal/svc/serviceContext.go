package svc

import (
	"time"

	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/money/moneyClient"
	"gacha_srv/app/play/internal/config"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config       config.Config
	DB           *gorm.DB
	BarrierModel gamemodelnc.DtmBarrierModel

	// M11 cache 模型：挖宝配置与格子（纯种子配置，运行时只读）
	DigBoxCfgModel  gamemodel.DigBoxConfigsModel
	DigBoxCellModel gamemodel.DigBoxCellsModel

	// M12 cache 模型：满车/扭蛋配置与奖池条目（纯种子配置，运行时只读）
	FullCarCfgModel    gamemodel.FullCarConfigsModel
	FullCarEntryModel  gamemodel.FullCarEntriesModel
	GashaponCfgModel   gamemodel.GashaponConfigsModel
	GashaponEntryModel gamemodel.GashaponEntriesModel

	// M13 cache 模型：连击链/拼团配置与拼团奖池条目（纯种子配置，运行时只读）
	ComboChainCfgModel gamemodel.ComboChainConfigsModel
	GroupBuyCfgModel   gamemodel.GroupBuyConfigsModel
	GroupBuyEntryModel gamemodel.GroupBuyEntriesModel

	// M15b cache 模型：记忆翻牌配置与奖档（纯种子配置，运行时只读）
	MemoryMatchCfgModel   gamemodel.MemoryMatchConfigsModel
	MemoryMatchEntryModel gamemodel.MemoryMatchEntriesModel

	// M11 nocache 模型：行锁资源（FOR UPDATE）与记录（uk request_id 幂等）
	DigSessionModel gamemodelnc.DigBoxSessionsModel
	DigRecordModel  gamemodelnc.DigBoxRecordsModel

	// M12 nocache 模型：满车/扭蛋状态（行锁 FOR UPDATE）与记录（uk request_id 幂等）、幸运指派
	FullCarStateModel       gamemodelnc.FullCarStateModel
	FullCarRecordModel      gamemodelnc.FullCarRecordsModel
	GashaponStateModel      gamemodelnc.GashaponStateModel
	GashaponRecordModel     gamemodelnc.GashaponRecordsModel
	GashaponAssignmentModel gamemodelnc.GashaponLuckyAssignmentsModel
	// PrizeGrantMarkModel SAGA 06 发奖落库标记（uk request_id 幂等；04 补偿退款前置检查）
	PrizeGrantMarkModel gamemodelnc.PrizeGrantMarksModel

	// M13 nocache 模型：连击链/拼团状态（行锁 FOR UPDATE）、记录（uk request_id 幂等）、连击计数
	ComboChainStateModel  gamemodelnc.ComboChainStateModel
	ComboChainRecordModel gamemodelnc.ComboChainRecordsModel
	ComboChainStreakModel gamemodelnc.ComboChainStreaksModel
	GroupBuyStateModel    gamemodelnc.GroupBuyStateModel
	GroupBuyRecordModel   gamemodelnc.GroupBuyRecordsModel

	// M15b nocache 模型：记忆翻牌会话/回合/结算/外观/擂主（行锁 FOR UPDATE + uk 幂等）
	MemoryMatchSessionModel     gamemodelnc.MemoryMatchSessionsModel
	MemoryMatchRoundModel       gamemodelnc.MemoryMatchRoundsModel
	MemoryMatchSettlementModel  gamemodelnc.MemoryMatchSettlementsModel
	MemoryMatchSelModel         gamemodelnc.MemoryMatchAppearanceSelectionsModel
	MemoryMatchEntitleModel     gamemodelnc.MemoryMatchAppearanceEntitlementsModel
	MemoryMatchChampionModel    gamemodelnc.MemoryMatchChampionModel
	MemoryMatchChampionEvModel  gamemodelnc.MemoryMatchChampionEventsModel
	MemoryMatchChampionPayModel gamemodelnc.MemoryMatchChampionPayoutsModel

	// MoneyRpc 资金服务客户端（玩法发奖现金/积分 CreditWallet 入账，走 Etcd；旧 MoneyRpc）
	MoneyRpc moneyClient.Money
	// MarketingRpc 营销服务客户端（锁报价/读报价/券名/发券，走 Etcd；旧同服务直连表）
	MarketingRpc marketingClient.Marketing
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
	marketingRpc := marketingClient.NewMarketing(zrpc.MustNewClient(c.MarketingRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))

	return &ServiceContext{
		Config:       c,
		DB:           db,
		BarrierModel: gamemodelnc.NewDtmBarrierModel(db),

		DigBoxCfgModel:  gamemodel.NewDigBoxConfigsModel(db, c.CacheRedis, cache.WithExpiry(30*time.Second)),
		DigBoxCellModel: gamemodel.NewDigBoxCellsModel(db, c.CacheRedis, cache.WithExpiry(30*time.Second)),

		FullCarCfgModel:    gamemodel.NewFullCarConfigsModel(db, c.CacheRedis, cache.WithExpiry(30*time.Second)),
		FullCarEntryModel:  gamemodel.NewFullCarEntriesModel(db, c.CacheRedis, cache.WithExpiry(30*time.Second)),
		GashaponCfgModel:   gamemodel.NewGashaponConfigsModel(db, c.CacheRedis, cache.WithExpiry(30*time.Second)),
		GashaponEntryModel: gamemodel.NewGashaponEntriesModel(db, c.CacheRedis, cache.WithExpiry(30*time.Second)),

		ComboChainCfgModel: gamemodel.NewComboChainConfigsModel(db, c.CacheRedis),
		GroupBuyCfgModel:   gamemodel.NewGroupBuyConfigsModel(db, c.CacheRedis),
		GroupBuyEntryModel: gamemodel.NewGroupBuyEntriesModel(db, c.CacheRedis),

		MemoryMatchCfgModel:   gamemodel.NewMemoryMatchConfigsModel(db, c.CacheRedis),
		MemoryMatchEntryModel: gamemodel.NewMemoryMatchEntriesModel(db, c.CacheRedis),

		DigSessionModel: gamemodelnc.NewDigBoxSessionsModel(db),
		DigRecordModel:  gamemodelnc.NewDigBoxRecordsModel(db),

		FullCarStateModel:       gamemodelnc.NewFullCarStateModel(db),
		FullCarRecordModel:      gamemodelnc.NewFullCarRecordsModel(db),
		GashaponStateModel:      gamemodelnc.NewGashaponStateModel(db),
		GashaponRecordModel:     gamemodelnc.NewGashaponRecordsModel(db),
		GashaponAssignmentModel: gamemodelnc.NewGashaponLuckyAssignmentsModel(db),
		PrizeGrantMarkModel:     gamemodelnc.NewPrizeGrantMarksModel(db),

		ComboChainStateModel:  gamemodelnc.NewComboChainStateModel(db),
		ComboChainRecordModel: gamemodelnc.NewComboChainRecordsModel(db),
		ComboChainStreakModel: gamemodelnc.NewComboChainStreaksModel(db),
		GroupBuyStateModel:    gamemodelnc.NewGroupBuyStateModel(db),
		GroupBuyRecordModel:   gamemodelnc.NewGroupBuyRecordsModel(db),

		MemoryMatchSessionModel:     gamemodelnc.NewMemoryMatchSessionsModel(db),
		MemoryMatchRoundModel:       gamemodelnc.NewMemoryMatchRoundsModel(db),
		MemoryMatchSettlementModel:  gamemodelnc.NewMemoryMatchSettlementsModel(db),
		MemoryMatchSelModel:         gamemodelnc.NewMemoryMatchAppearanceSelectionsModel(db),
		MemoryMatchEntitleModel:     gamemodelnc.NewMemoryMatchAppearanceEntitlementsModel(db),
		MemoryMatchChampionModel:    gamemodelnc.NewMemoryMatchChampionModel(db),
		MemoryMatchChampionEvModel:  gamemodelnc.NewMemoryMatchChampionEventsModel(db),
		MemoryMatchChampionPayModel: gamemodelnc.NewMemoryMatchChampionPayoutsModel(db),

		MoneyRpc:     moneyRpc,
		MarketingRpc: marketingRpc,
	}
}
