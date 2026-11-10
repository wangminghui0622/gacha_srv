// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"net/http"

	"gacha_srv/app/play/play"
	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// AssignGashaponLuckyUser 指定幸运用户（gacha.pool:assign_lucky；
// 扭蛋 LUCKY 奖指派，userId=0 清除；configId 走路径）。
func AssignGashaponLuckyUserHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configId, err := restx.PathUint64(r, "configId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req play.AssignGashaponLuckyUserReq
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.ConfigId = int64(configId)
		resp, err := pool.NewAssignGashaponLuckyUserLogic(r.Context(), svcCtx).AssignGashaponLuckyUser(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
