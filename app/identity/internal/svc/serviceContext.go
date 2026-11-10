package svc

import (
	"encoding/base64"
	"time"

	"gacha_srv/app/identity/internal/config"
	"gacha_srv/app/identity/internal/phonecrypto"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/thirdparty/thirdpartyClient"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config    config.Config
	DB        *gorm.DB
	UserModel gacha_identity.UserUsersModel
	// M15a 手机号盲索引 / 第三方身份仓库
	PhoneRepo      *repo.PhoneRepo
	ThirdPartyRepo *repo.ThirdPartyRepo
	// ThirdpartyRpc 第三方服务客户端（验证码/微信 code 交换）
	ThirdpartyRpc thirdpartyClient.Thirdparty
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

	// 手机号盲索引 codec：base64 解码密钥，非法配置直接拒绝启动
	encKey, err := base64.StdEncoding.DecodeString(c.PhoneCrypto.EncKey)
	if err != nil {
		panic("phone enc key must be base64: " + err.Error())
	}
	indexKey, err := base64.StdEncoding.DecodeString(c.PhoneCrypto.IndexKey)
	if err != nil {
		panic("phone index key must be base64: " + err.Error())
	}
	phoneCodec, err := phonecrypto.New(encKey, indexKey, c.PhoneCrypto.EncKeyID, c.PhoneCrypto.IndexVersion)
	if err != nil {
		panic("phone crypto init failed: " + err.Error())
	}

	// ClientErrorInterceptor：下游业务码还原为 CodeError，跨服务不失真
	thirdpartyRpc := thirdpartyClient.NewThirdparty(zrpc.MustNewClient(c.ThirdpartyRpc,
		zrpc.WithUnaryClientInterceptor(errorx.ClientErrorInterceptor)))

	return &ServiceContext{
		Config:          c,
		DB:              db,
		UserModel:       gacha_identity.NewUserUsersModel(db),
		PhoneRepo:       repo.NewPhoneRepo(db, phoneCodec),
		ThirdPartyRepo:  repo.NewThirdPartyRepo(db),
		ThirdpartyRpc:   thirdpartyRpc,
	}
}
