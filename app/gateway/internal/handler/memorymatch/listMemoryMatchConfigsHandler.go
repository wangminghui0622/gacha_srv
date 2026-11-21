package memorymatch

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/memorymatch"
	"gacha_srv/app/gateway/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 对对碰主题列表（玩家端：只返回上架配置，最新在前）
func ListMemoryMatchConfigsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := memorymatch.NewListMemoryMatchConfigsLogic(r.Context(), svcCtx)
		resp, err := l.ListMemoryMatchConfigs()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
