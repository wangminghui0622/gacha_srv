package pool

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/admin-gateway/internal/types"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// QueryPlayPoolReferencesHandler 引用查询（商品ID/池子ID 反查奖池）。
func QueryPlayPoolReferencesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.QueryPlayPoolReferencesReq
		if err := httpx.Parse(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := pool.NewQueryPlayPoolReferencesLogic(r.Context(), svcCtx).QueryPlayPoolReferences(&play.QueryPlayPoolReferencesReq{
			RefType:  req.RefType,
			RefId:    req.RefId,
			Page:     req.Page,
			PageSize: req.PageSize,
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
