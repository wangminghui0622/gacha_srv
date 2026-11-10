package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	MySQL struct {
		// DataSource 本服务数据库 DSN（gacha_money：钱包/资产/支付单合并库）
		DataSource string
	}
	// DrawRpc 抽奖服务客户端（GrantAsset 反查开奖记录，走 Etcd）
	DrawRpc zrpc.RpcClientConf
	// MarketingRpc 营销服务客户端（07 捕获内 sub-call CommitQuote 核销报价，走 Etcd）
	MarketingRpc zrpc.RpcClientConf
	// PlayRpc 玩法服务客户端（CancelPayment 发奖落库前置检查 CheckPrizesGranted，走 Etcd）
	PlayRpc zrpc.RpcClientConf
}
