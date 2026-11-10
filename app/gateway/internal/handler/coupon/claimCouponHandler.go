// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package coupon
import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/coupon"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 领取券（模板单号 → 用户券）
func ClaimCouponHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ClaimCouponReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := coupon.NewClaimCouponLogic(r.Context(), svcCtx)
		resp, err := l.ClaimCoupon(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
