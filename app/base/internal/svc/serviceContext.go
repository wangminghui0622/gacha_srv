package svc

import (
	"time"

	"gacha_srv/app/base/internal/config"
	basemodelnc "gacha_srv/pkg/model_nocache/gacha_base"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config        config.Config
	DB            *gorm.DB
	DecisionModel basemodelnc.EligibilityDecisionLogsModel
	RiskRuleModel basemodelnc.RiskRulesModel
	RiskCaseModel basemodelnc.RiskCasesModel
	RiskCaseEventModel basemodelnc.RiskCaseEventsModel
	BarrierModel  basemodelnc.DtmBarrierModel
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
		Config:            c,
		DB:                db,
		DecisionModel:     basemodelnc.NewEligibilityDecisionLogsModel(db),
		RiskRuleModel:     basemodelnc.NewRiskRulesModel(db),
		RiskCaseModel:     basemodelnc.NewRiskCasesModel(db),
		RiskCaseEventModel: basemodelnc.NewRiskCaseEventsModel(db),
		BarrierModel:      basemodelnc.NewDtmBarrierModel(db),
	}
}
