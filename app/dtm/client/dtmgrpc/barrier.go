/*
 * Copyright (c) 2021 yedf. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

package dtmgrpc

import (
	"context"

	"gacha_srv/app/dtm/client/dtmcli"
	"gacha_srv/app/dtm/client/dtmgrpc/dtmgimp"
)

// BarrierFromGrpc generate a Barrier from grpc context
func BarrierFromGrpc(ctx context.Context) (*dtmcli.BranchBarrier, error) {
	tb := dtmgimp.TransBaseFromGrpc(ctx)
	return dtmcli.BarrierFrom(tb.TransType, tb.Gid, tb.BranchID, tb.Op)
}

// BarrierFromGrpcOrNil 从 gRPC metadata 提取 DTM trans info 构造 barrier：
// 作为 SAGA 分支被 DTM 直调时 trans info 完整，返回 barrier；
// 作为普通内部 RPC 被其他分支 sub-call 时（如 play.Lock<玩法> 内调 marketing.LockQuote）
// 无 trans info，返回 (nil, nil)，调用方退化为普通事务——
// 幂等由业务侧 FOR UPDATE + 状态校验兜底，而非 barrier 表。
func BarrierFromGrpcOrNil(ctx context.Context) (*dtmcli.BranchBarrier, error) {
	tb := dtmgimp.TransBaseFromGrpc(ctx)
	if tb.Gid == "" {
		return nil, nil
	}
	return dtmcli.BarrierFrom(tb.TransType, tb.Gid, tb.BranchID, tb.Op)
}
