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

// 记忆翻牌命运抽牌（M15b；有对子未配/资源耗尽 → 3042）
func ApplyMemoryMatchDestinyDrawHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MemoryMatchRoundPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := memorymatch.NewApplyMemoryMatchDestinyDrawLogic(r.Context(), svcCtx)
		resp, err := l.ApplyMemoryMatchDestinyDraw(req.SessionId, req.RoundNo)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
