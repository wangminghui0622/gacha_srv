// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package dig

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/dig"
	"gacha_srv/app/gateway/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 挖宝箱列表（M11b）
func ListDigBoxesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := dig.NewListDigBoxesLogic(r.Context(), svcCtx)
		resp, err := l.ListDigBoxes()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
