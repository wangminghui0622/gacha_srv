// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/promotion"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"
)

func CreateFortuneRewardHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var reward marketing.FortuneReward
		if err := restx.ParseProto(r, &reward); err != nil {
			restx.Error(w, r, err)
			return
		}
		reward.Id = 0
		resp, err := promotion.NewUpsertFortuneRewardLogic(r.Context(), svcCtx).UpsertFortuneReward(&marketing.UpsertFortuneRewardRequest{
			RequestId: restx.RequestIdFrom(r.Context()), TenantId: 1, Reward: &reward,
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
