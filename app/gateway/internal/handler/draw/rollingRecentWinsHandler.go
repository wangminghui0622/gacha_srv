// M12 滚动中奖 feed（首页 marquee）。公开端点，无 JWT（掩码展示）。

package draw

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/draw"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 滚动中奖最近中奖列表（首页 marquee 数据源；M12，公开）
func RollingRecentWinsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RollingRecentWinsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := draw.NewRollingRecentWinsLogic(r.Context(), svcCtx)
		resp, err := l.RollingRecentWins(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
