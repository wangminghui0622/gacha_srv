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

// 连击链详情（M13：当前车进度/全局奖槽/共享奖档位/我的连击计数）
func GetComboChainHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ComboChainPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := combochain.NewGetComboChainLogic(r.Context(), svcCtx)
		resp, err := l.GetComboChain(req.ConfigId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
