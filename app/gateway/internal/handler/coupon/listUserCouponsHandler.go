// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package coupon
import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/coupon"
	"gacha_srv/app/gateway/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 我的券列表（券中心/购买页选择器）
func ListUserCouponsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := coupon.NewListUserCouponsLogic(r.Context(), svcCtx)
		resp, err := l.ListUserCoupons()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
