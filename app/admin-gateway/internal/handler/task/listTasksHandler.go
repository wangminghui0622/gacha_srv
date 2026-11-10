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

// ListTasks 任务进度列表（task.progress:read；GET /tasks）。
// 查询参数与 server_v2 kratos 绑定一致：userId/tenantId/period（枚举名）/
// pageSize/pageToken；tenant_id 仅授权属性，进度不按租户拆分。
func ListTasksHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, err := restx.QueryUint64(r, "userId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		tenantId, err := restx.QueryUint64(r, "tenantId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		period, err := restx.QueryEnum(r, "period", ops.TaskPeriod_value)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &ops.ListTasksRequest{
			UserId:    userId,
			TenantId:  tenantId,
			Period:    ops.TaskPeriod(period),
			PageSize:  pageSize,
			PageToken: restx.QueryString(r, "pageToken"),
		}
		resp, err := task.NewListTasksLogic(r.Context(), svcCtx).ListTasks(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
