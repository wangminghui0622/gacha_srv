// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package task

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/task"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
)

// RequestTaskPlanPublication 申请发布任务计划（task.plan:request_publication；
// 申请人 = 会话主体，不接受客户端传入）。
func RequestTaskPlanPublicationHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version, err := restx.PathUint64AllowZero(r, "version")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req ops.RequestTaskPlanPublicationRequest
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.Version = version
		resp, err := task.NewRequestTaskPlanPublicationLogic(r.Context(), svcCtx).RequestTaskPlanPublication(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
