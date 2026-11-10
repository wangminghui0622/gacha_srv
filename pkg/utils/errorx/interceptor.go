// 拦截器：业务码跨 gRPC 不失真（规范 05-errors 规则 4/5）
package errorx

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ServerErrorInterceptor RPC 服务出口：
//   - 业务码（CodeBusinessMin~CodeBusinessMax）直接作为 gRPC status code 传出；
//   - 标准码映射为对应 gRPC 码；
//   - DTM SAGA 分支直接返回的 status.Error（Aborted/Internal）原样透出；
//   - 其余未知错误折叠为 Internal（避免驱动原始错误透出）。
func ServerErrorInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	if err == nil {
		return resp, nil
	}
	// 已是 gRPC status 错误（SAGA 分支语义）→ 原样透出
	var se interface{ GRPCStatus() *status.Status }
	if errors.As(err, &se) {
		return nil, err
	}
	var ce *CodeError
	if errors.As(err, &ce) {
		if ce.Code >= CodeBusinessMin && ce.Code <= CodeBusinessMax {
			return nil, status.Error(codes.Code(ce.Code), ce.Msg)
		}
		return nil, status.Error(mapStandardCode(ce.Code), ce.Msg)
	}
	return nil, status.Error(codes.Internal, err.Error())
}

// ClientErrorInterceptor 网关调下游：gRPC 状态反向还原为 CodeError
func ClientErrorInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err == nil {
		return nil
	}
	s, _ := status.FromError(err)
	code := int(s.Code())
	if code >= CodeBusinessMin && code <= CodeBusinessMax {
		return NewCodeError(code, s.Message())
	}
	return NewCodeError(mapGRPCCode(s.Code()), s.Message())
}
