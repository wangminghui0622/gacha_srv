// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/pkg/utils/errorx"
)

// pathTenantID 路径 :tenantId 一致性校验：非 1 → 400（单租户固定 1，D7）。
func pathTenantID(r *http.Request) error {
	t, err := restx.PathUint64(r, "tenantId")
	if err != nil {
		return err
	}
	if t != 1 {
		return errorx.NewCodeError(errorx.CodeParamError, "路径参数 tenantId 必须为 1")
	}
	return nil
}
