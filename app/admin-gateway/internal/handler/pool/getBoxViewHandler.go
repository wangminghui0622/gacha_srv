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

// GetBoxView 箱子库存与玩法状态（gacha.pool:read；本仓无独立 box-instance，
// 与 GetBox 同样映射 BoxDetail：名称/价格/条目库存/概率公示）。
func GetBoxViewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		boxId, err := restx.PathUint64(r, "boxId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &draw.BoxDetailReq{BoxId: int64(boxId)}
		resp, err := pool.NewGetBoxViewLogic(r.Context(), svcCtx).GetBoxView(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
