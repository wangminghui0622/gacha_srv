// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package memorymatch

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/memorymatch"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 记忆翻牌跳过（M15b；AutoResolve 剩余回合+终局结算；已跳过幂等重跑）
func SkipMemoryMatchHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MemoryMatchSessionPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := memorymatch.NewSkipMemoryMatchLogic(r.Context(), svcCtx)
		resp, err := l.SkipMemoryMatch(req.SessionId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
