package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type WriteAuditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewWriteAuditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WriteAuditLogic {
	return &WriteAuditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// WriteAudit 写审计日志（内部 RPC：管理网关鉴权通过后异步调用；只插不删）。
func (l *WriteAuditLogic) WriteAudit(in *ops.WriteAuditRequest) (*ops.WriteAuditReply, error) {
	if in.OperationId == "" || in.RequestId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "操作与请求 ID 必填")
	}
	row := &opsmodel.OpsAuditLogs{
		TenantId: defaultTenant(in.TenantId), AccountId: in.AccountId, OperationId: in.OperationId,
		Resource: in.Resource, Action: in.Action, TargetId: in.TargetId,
		RequestId: in.RequestId, DetailJson: in.DetailJson,
	}
	if err := l.svcCtx.AuditModel.Insert(l.ctx, nil, row); err != nil {
		return nil, err
	}
	return &ops.WriteAuditReply{Written: true}, nil
}
