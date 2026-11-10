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

// 记忆翻牌会话进度（M15b）
func GetMemoryMatchSessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MemoryMatchSessionPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := memorymatch.NewGetMemoryMatchSessionLogic(r.Context(), svcCtx)
		resp, err := l.GetMemoryMatchSession(req.SessionId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
