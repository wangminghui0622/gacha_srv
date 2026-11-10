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

// ApproveTaskPlanPublication 审批任务计划发布（task.plan:approve_publication；
// 审批人 = 会话主体，不接受客户端传入）。
func ApproveTaskPlanPublicationHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version, err := restx.PathUint64AllowZero(r, "version")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req ops.ApproveTaskPlanPublicationRequest
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.Version = version
		resp, err := task.NewApproveTaskPlanPublicationLogic(r.Context(), svcCtx).ApproveTaskPlanPublication(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
