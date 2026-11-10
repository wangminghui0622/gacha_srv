package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	MySQL struct {
		// DataSource 本服务数据库 DSN（gacha_task：任务/签到/通行证/会员 8 表）
		DataSource string
	}
	OpsMySQL struct {
		// DataSource 运营库跨库直读 DSN（gacha_ops：task_plan_head/version 计划快照；
		// 旧仓 game 服务同款直读，新仓照抄保留）
		DataSource string
	}
}
