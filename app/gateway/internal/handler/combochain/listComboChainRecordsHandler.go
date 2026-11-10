// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package combochain

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/combochain"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 连击链记录（M13：requestId 精确查询=购买轮询，空=分页列表）
func ListComboChainRecordsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ComboChainRecordsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := combochain.NewListComboChainRecordsLogic(r.Context(), svcCtx)
		resp, err := l.ListComboChainRecords(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
