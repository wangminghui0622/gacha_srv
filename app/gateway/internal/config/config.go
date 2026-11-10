// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf

	// 下游 RPC 客户端（网关自身调用走 Etcd 服务发现；服务拆分后 5 → 7，见 service-division.md §1）
	IdentityRpc  zrpc.RpcClientConf
	MoneyRpc     zrpc.RpcClientConf
	DrawRpc      zrpc.RpcClientConf
	BaseRpc      zrpc.RpcClientConf
	MarketingRpc zrpc.RpcClientConf
	PlayRpc      zrpc.RpcClientConf
	TaskRpc      zrpc.RpcClientConf

	// DTM Server gRPC 地址（SAGA 事务提交用）
	DtmServer DtmConf

	// 各 RPC 服务直连地址（供 DTM 回调，DTM 不接 Etcd，格式 host:port；
	// 只保留 SAGA 分支目标：base/marketing/money/draw/play，见 service-division.md §3.7）
	BaseRpcDirect      string
	MarketingRpcDirect string
	MoneyRpcDirect     string
	DrawRpcDirect      string
	PlayRpcDirect      string
}

type DtmConf struct {
	Target string
}
