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

// GetTaskPlanPublicationApproval 读取任务计划发布审批（task.plan:read；
// GET /task-plans/:version/approval；无审批行 → 404，是正常空态）。
func GetTaskPlanPublicationApprovalHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version, err := restx.PathUint64AllowZero(r, "version")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &ops.GetTaskPlanPublicationApprovalRequest{Version: version}
		resp, err := task.NewGetTaskPlanPublicationApprovalLogic(r.Context(), svcCtx).GetTaskPlanPublicationApproval(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
