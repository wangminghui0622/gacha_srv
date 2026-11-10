package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	MySQL struct {
		// DataSource 本服务数据库 DSN（gacha_catalog）
		DataSource string
	}
}
