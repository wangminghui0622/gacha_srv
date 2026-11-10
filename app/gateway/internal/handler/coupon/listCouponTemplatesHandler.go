// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package coupon
import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/coupon"
	"gacha_srv/app/gateway/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 可领券模板列表（券中心领取页）
func ListCouponTemplatesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := coupon.NewListCouponTemplatesLogic(r.Context(), svcCtx)
		resp, err := l.ListCouponTemplates()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
