// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package fullcar

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/fullcar"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 满车记录（M12b：requestId 精确查询=购买轮询，空=分页列表）
func ListFullCarRecordsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.FullCarRecordsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := fullcar.NewListFullCarRecordsLogic(r.Context(), svcCtx)
		resp, err := l.ListFullCarRecords(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
