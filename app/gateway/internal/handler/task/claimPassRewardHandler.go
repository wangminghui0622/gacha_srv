// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package task

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/task"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 通行证奖励领取（M15c，POST /tasks/passes/:passId/claim）
func ClaimPassRewardHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ClaimPassPathReq
		if err := httpx.ParsePath(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		var body types.ClaimPassBodyReq
		if err := httpx.Parse(r, &body); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := task.NewClaimPassRewardLogic(r.Context(), svcCtx)
		resp, err := l.ClaimPassReward(&req, &body)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
