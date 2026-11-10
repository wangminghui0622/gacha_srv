package svc

import (
	"time"

	"gacha_srv/app/draw/drawClient"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/money/internal/config"
	"gacha_srv/app/play/playClient"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config config.Config
	DB     *gorm.DB
	// 资产域模型（旧 gacha_user 钱包表，拆分后并入 gacha_money）
	WalletAccountModel gacha_money.WalletAccountsModel
	WalletHoldModel    gacha_money.WalletHoldsModel
	WalletLedgerModel  gacha_money.WalletLedgersModel
	AssetModel         gacha_money.AssetsModel
	// 支付域模型（旧 gacha_payment 表，拆分后并入 gacha_money）
	PaymentOrderModel  gacha_money.PaymentOrdersModel
	PaymentLegModel    gacha_money.PaymentLegsModel
	PaymentRefundModel gacha_money.PaymentRefundsModel
	// DrawRpc 抽奖服务客户端（GrantAsset 反查开奖记录，走 Etcd）
	DrawRpc drawClient.Draw
	// MarketingRpc 营销服务客户端（07 捕获内 sub-call CommitQuote 核销报价，走 Etcd）
	MarketingRpc marketingClient.Marketing
	// PlayRpc 玩法服务客户端（CancelPayment 发奖落库前置检查，走 Etcd）
	PlayRpc playClient.Play
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

	// ClientErrorInterceptor：下游业务码还原为 CodeError，跨服务不失真（规范 05-errors 规则 5）
	drawRpc := drawClient.NewDraw(zrpc.MustNewClient(c.DrawRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	marketingRpc := marketingClient.NewMarketing(zrpc.MustNewClient(c.MarketingRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))
	playRpc := playClient.NewPlay(zrpc.MustNewClient(c.PlayRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))

	return &ServiceContext{
		Config:             c,
		DB:                 db,
		WalletAccountModel: gacha_money.NewWalletAccountsModel(db),
		WalletHoldModel:    gacha_money.NewWalletHoldsModel(db),
		WalletLedgerModel:  gacha_money.NewWalletLedgersModel(db),
		AssetModel:         gacha_money.NewAssetsModel(db),
		PaymentOrderModel:  gacha_money.NewPaymentOrdersModel(db),
		PaymentLegModel:    gacha_money.NewPaymentLegsModel(db),
		PaymentRefundModel: gacha_money.NewPaymentRefundsModel(db),
		DrawRpc:            drawRpc,
		MarketingRpc:       marketingRpc,
		PlayRpc:            playRpc,
	}
}
