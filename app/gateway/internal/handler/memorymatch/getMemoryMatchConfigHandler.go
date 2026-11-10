// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package memorymatch

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/memorymatch"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 记忆翻牌玩法详情（M15b：配置+奖档+擂主+外观）
func GetMemoryMatchConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MemoryMatchPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := memorymatch.NewGetMemoryMatchConfigLogic(r.Context(), svcCtx)
		resp, err := l.GetMemoryMatchConfig(req.ConfigId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
