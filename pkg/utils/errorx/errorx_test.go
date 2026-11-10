package errorx

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCodeErrorRoundTrip(t *testing.T) {
	err := NewCodeError(CodeBalanceNotEnough, "余额不足")
	var ce *CodeError
	if !errors.As(err, &ce) || ce.Code != 2001 {
		t.Fatalf("CodeError round trip failed: %v", err)
	}
	if got := err.Error(); got != "code=2001 msg=余额不足" {
		t.Errorf("Error() = %q", got)
	}
}

// ServerErrorInterceptor：业务码直接作 gRPC status code 传出
func TestServerInterceptorBusinessCode(t *testing.T) {
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, NewCodeError(CodePasswordWrong, "密码错误")
	}
	_, err := ServerErrorInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	s, ok := status.FromError(err)
	if !ok || s.Code() != codes.Code(CodePasswordWrong) {
		t.Fatalf("code = %v, want %d", s, CodePasswordWrong)
	}
	if s.Message() != "密码错误" {
		t.Errorf("message = %q", s.Message())
	}
}

// ServerErrorInterceptor：SAGA 分支的 status.Error(Aborted) 原样透出
func TestServerInterceptorPassthroughStatus(t *testing.T) {
	aborted := status.Error(codes.Aborted, "code=2001 msg=余额不足")
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, aborted
	}
	_, err := ServerErrorInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	if !errors.Is(err, aborted) {
		t.Fatalf("status error not passed through: %v", err)
	}
}

// ServerErrorInterceptor：标准码映射为 gRPC 码
func TestServerInterceptorStandardCode(t *testing.T) {
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, NewCodeError(CodeUnauthorized, "令牌无效或已过期")
	}
	_, err := ServerErrorInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	if s, _ := status.FromError(err); s.Code() != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", s.Code())
	}
}

// ServerErrorInterceptor：未知错误折叠为 Internal
func TestServerInterceptorUnknown(t *testing.T) {
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("boom")
	}
	_, err := ServerErrorInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	if s, _ := status.FromError(err); s.Code() != codes.Internal {
		t.Fatalf("code = %v, want Internal", s.Code())
	}
}

// FromError：gRPC 业务码还原为 CodeError
func TestFromErrorGRPCStatus(t *testing.T) {
	err := status.Error(codes.Code(2001), "宝箱不存在")
	ce, ok := FromError(err).(*CodeError)
	if !ok || ce.Code != 2001 || ce.Msg != "宝箱不存在" {
		t.Fatalf("FromError = %v", FromError(err))
	}
}

// FromError：nil 与普通错误
func TestFromErrorNilAndPlain(t *testing.T) {
	if FromError(nil) != nil {
		t.Error("FromError(nil) should be nil")
	}
	ce, ok := FromError(errors.New("raw")).(*CodeError)
	if !ok || ce.Code != CodeServerError {
		t.Fatalf("FromError(plain) = %v", FromError(errors.New("raw")))
	}
}
