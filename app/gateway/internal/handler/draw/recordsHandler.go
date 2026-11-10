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

// 抽赏记录（requestId 精确查询或分页列表）
func RecordsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RecordsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := draw.NewRecordsLogic(r.Context(), svcCtx)
		resp, err := l.Records(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
