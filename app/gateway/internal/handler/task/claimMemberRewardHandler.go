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

// 会员奖励领取（M15c，POST /membership/claim）
func ClaimMemberRewardHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body types.ClaimMemberBodyReq
		if err := httpx.Parse(r, &body); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := task.NewClaimMemberRewardLogic(r.Context(), svcCtx)
		resp, err := l.ClaimMemberReward(&body)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
