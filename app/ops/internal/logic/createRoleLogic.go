package logic

import (
	"context"
	"errors"

	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type CreateRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRoleLogic {
	return &CreateRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateRole 创建角色（code 唯一，初始 enabled，version=1）。
func (l *CreateRoleLogic) CreateRole(in *ops.CreateRoleRequest) (*ops.Role, error) {
	tenantID := defaultTenant(in.TenantId)
	code := trimNonEmpty(in.Code)
	name := trimNonEmpty(in.Name)
	if code == "" || len(code) > 64 || name == "" || len(name) > 128 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "角色编码(1-64)/名称(1-128)必填")
	}
	_, err := l.svcCtx.RolesModel.FindOneByTenantIdCode(l.ctx, tenantID, code)
	if err == nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleRoleCodeExists, "角色编码已存在")
	}
	if !repo.IsNotFound(err) {
		return nil, err
	}
	role := &opsmodel.Roles{TenantId: tenantID, Code: code, Name: name, Enabled: 1, Version: 1, CreatedBy: 0}
	if err = l.svcCtx.RolesModel.Insert(l.ctx, nil, role); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, errorx.NewCodeError(errorx.CodeConsoleRoleCodeExists, "角色编码已存在")
		}
		return nil, err
	}
	return roleToProto(role, nil), nil
}
