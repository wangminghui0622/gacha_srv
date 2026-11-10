// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"net/http"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// GetBox 读取箱子（gacha.pool:read；本仓箱子即 draw_boxes，映射 BoxDetail）。
func GetBoxHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		boxId, err := restx.PathUint64(r, "boxId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &draw.BoxDetailReq{BoxId: int64(boxId)}
		resp, err := pool.NewGetBoxLogic(r.Context(), svcCtx).GetBox(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
