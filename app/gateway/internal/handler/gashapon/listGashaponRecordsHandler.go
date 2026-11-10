// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package gashapon

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/gashapon"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 扭蛋记录（M12c：requestId 精确查询=购买轮询，空=分页列表）
func ListGashaponRecordsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GashaponRecordsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := gashapon.NewListGashaponRecordsLogic(r.Context(), svcCtx)
		resp, err := l.ListGashaponRecords(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
