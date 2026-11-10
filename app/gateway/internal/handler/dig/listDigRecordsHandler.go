// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package dig

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/dig"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 挖宝记录（requestId 精确查询或分页列表，M11b）
func ListDigRecordsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DigRecordsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := dig.NewListDigRecordsLogic(r.Context(), svcCtx)
		resp, err := l.ListDigRecords(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
