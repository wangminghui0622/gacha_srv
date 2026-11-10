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

// ListTaskPlans 任务计划列表（task.plan:read；GET /task-plans；afterVersion 游标）。
func ListTaskPlansHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		afterVersion, err := restx.QueryUint64(r, "afterVersion")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &ops.ListTaskPlansRequest{AfterVersion: afterVersion, PageSize: pageSize}
		resp, err := task.NewListTaskPlansLogic(r.Context(), svcCtx).ListTaskPlans(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
