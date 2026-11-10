// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package draw

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/draw"
	"gacha_srv/app/gateway/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 宝箱列表
func BoxListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := draw.NewBoxListLogic(r.Context(), svcCtx)
		resp, err := l.BoxList()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
