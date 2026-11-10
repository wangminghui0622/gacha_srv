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

// 扭蛋详情（M12c：特别奖进度/余票/参与人数/单价）
func GetGashaponHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GashaponPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := gashapon.NewGetGashaponLogic(r.Context(), svcCtx)
		resp, err := l.GetGashapon(req.ConfigId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
