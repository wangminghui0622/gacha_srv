package svc

import (
	"time"

	"gacha_srv/app/draw/internal/config"
	drawmodel "gacha_srv/pkg/model/gacha_draw"
	drawmodelnc "gacha_srv/pkg/model_nocache/gacha_draw"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config         config.Config
	DB             *gorm.DB
	BoxModel       drawmodel.DrawBoxesModel       // 缓存模型：静态宝箱（10 分钟 TTL）
	EntryModel     drawmodel.DrawPoolEntriesModel // 缓存模型：奖池条目（3 分钟短 TTL + 延迟双删）
	RecordModel    drawmodelnc.DrawRecordsModel   // nocache：写多、按用户查
	WarehouseModel drawmodelnc.DrawWarehouseItemsModel
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

	return &ServiceContext{
		Config:  c,
		DB:      db,
		BoxModel: drawmodel.NewDrawBoxesModel(db, c.CacheRedis, cache.WithExpiry(10*time.Minute)),
		EntryModel: drawmodel.NewDrawPoolEntriesModel(db, c.CacheRedis, cache.WithExpiry(3*time.Minute)),
		RecordModel:    drawmodelnc.NewDrawRecordsModel(db),
		WarehouseModel: drawmodelnc.NewDrawWarehouseItemsModel(db),
	}
}
