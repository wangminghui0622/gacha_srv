package svc

import (
	"time"

	"gacha_srv/app/task/internal/config"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config config.Config
	// DB 本服务数据库（gacha_task：任务/签到/通行证/会员 8 表，全 raw SQL 无 gorm 模型）
	DB *gorm.DB
	// OpsDB 运营库跨库直读（gacha_ops：task_plan_head/version 计划快照；旧仓 game 同款）
	OpsDB *gorm.DB
}

// openDB 打开连接并设置连接池参数（与其余服务同款：100/20/1h/10m + 优雅关闭）
func openDB(name, dataSource string) *gorm.DB {
	db, err := gorm.Open(mysql.Open(dataSource), &gorm.Config{
		// 开启错误翻译（1062 → gorm.ErrDuplicatedKey），幂等路径依赖此判断
		TranslateError: true,
	})
	if err != nil {
		panic("failed to connect " + name + ": " + err.Error())
	}
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
			logx.Errorf("close sql.DB (%s) failed: %v", name, err)
		}
	})
	return db
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
		DB:     openDB("gacha_task", c.MySQL.DataSource),
		OpsDB:  openDB("gacha_ops", c.OpsMySQL.DataSource),
	}
}
