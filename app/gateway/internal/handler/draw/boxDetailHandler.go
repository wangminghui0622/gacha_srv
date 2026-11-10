// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package draw

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/draw"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 宝箱详情（概率公示）
func BoxDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BoxDetailPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := draw.NewBoxDetailLogic(r.Context(), svcCtx)
		resp, err := l.BoxDetail(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
