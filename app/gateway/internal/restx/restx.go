// Package restx 统一响应信封（规范 common/api-conventions）：
// {code, msg, data}，HTTP 状态码恒 200，错误只通过 code 表达。
// 网关启动时调用一次 MustSetup 注册 Ok/Error 处理器；业务代码不得自行拼信封。
package restx

import (
	"context"
	"net/http"

	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// envelope 统一响应信封（data 非 0 时省略）
type envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// MustSetup 注册成功/错误处理器（幂等：重复调用只是覆盖为相同实现）
func MustSetup() {
	// 成功：{code:0, msg:"success", data:resp}
	httpx.SetOkHandler(func(ctx context.Context, v any) any {
		return envelope{Code: errorx.CodeSuccess, Msg: "success", Data: v}
	})
	// 失败：FromError 归一（CodeError 原样、gRPC 业务码还原、未知折叠 500），
	// HTTP 恒 200；内部错误详情只进日志不进响应体（由调用方记日志）
	httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
		ce, _ := errorx.FromError(err).(*errorx.CodeError)
		return http.StatusOK, envelope{Code: ce.Code, Msg: ce.Msg}
	})
}
