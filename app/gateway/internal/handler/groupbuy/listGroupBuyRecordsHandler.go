// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package groupbuy

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/groupbuy"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 拼团记录（M13：requestId 精确查询=购买轮询，空=分页列表）
func ListGroupBuyRecordsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GroupBuyRecordsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := groupbuy.NewListGroupBuyRecordsLogic(r.Context(), svcCtx)
		resp, err := l.ListGroupBuyRecords(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
