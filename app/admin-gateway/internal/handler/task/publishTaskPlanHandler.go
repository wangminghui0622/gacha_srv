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

// PublishTaskPlan 发布任务计划（task.plan:publish；POST /task-plans/:version/publish）。
func PublishTaskPlanHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version, err := restx.PathUint64AllowZero(r, "version")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req ops.PublishTaskPlanRequest
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.Version = version
		resp, err := task.NewPublishTaskPlanLogic(r.Context(), svcCtx).PublishTaskPlan(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
