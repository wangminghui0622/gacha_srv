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

// GetTaskPlan 读取任务计划（task.plan:read；GET /task-plans/:version；version=0 → 最新）。
func GetTaskPlanHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version, err := restx.PathUint64AllowZero(r, "version")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &ops.GetTaskPlanRequest{Version: version}
		resp, err := task.NewGetTaskPlanLogic(r.Context(), svcCtx).GetTaskPlan(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
