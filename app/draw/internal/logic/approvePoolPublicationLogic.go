package logic

import (
	"bytes"
	"context"
	"errors"
	"time"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/repo"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ApprovePoolPublicationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApprovePoolPublicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovePoolPublicationLogic {
	return &ApprovePoolPublicationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ApprovePoolPublication 审批发布（server_v2 ApprovePublication 语义）：
// 版本 DRAFT + hash 一致 + 审批行存在（未提请 2009）→ 审批人 ≠ 申请人（自批 2009）
// → 同人重复批准幂等返回；CAS PENDING→APPROVED。
func (l *ApprovePoolPublicationLogic) ApprovePoolPublication(in *draw.ApprovePoolPublicationReq) (*draw.PoolPublicationApproval, error) {
	if err := poolValidateKey(in.GetPoolId(), in.GetVersion(), in.GetExpectedConfigHash()); err != nil {
		return nil, err
	}
	if in.GetApproverId() <= 0 {
		return nil, errorx.NewCodeError(errorx.CodePoolDraftInvalid, "approverId 必填")
	}

	tx := l.svcCtx.DB.Begin()
	if tx.Error != nil {
		return nil, errorx.FromMySQLError(tx.Error)
	}
	defer func() { _ = tx.Rollback() }()

	version, err := repo.LockPoolVersion(tx, in.GetPoolId(), in.GetVersion())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, poolErrNotFound()
		}
		return nil, errorx.FromMySQLError(err)
	}
	if version.Status != repo.StatusDraft {
		return nil, poolErrConflict()
	}
	if !bytes.Equal(version.ConfigHash, in.GetExpectedConfigHash()) {
		return nil, poolErrConflict()
	}

	approval, err := repo.LockPoolPublicationApproval(tx, in.GetPoolId(), in.GetVersion())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, poolErrPublication()
		}
		return nil, errorx.FromMySQLError(err)
	}
	if approval.Status == repo.ApprovalApproved {
		// 同人同 hash 重复批准 → 幂等返回
		if approval.ApproverID == in.GetApproverId() && bytes.Equal(approval.ConfigHash, in.GetExpectedConfigHash()) {
			return l.approvalToResp(approval), nil
		}
		return nil, poolErrPublication()
	}
	if approval.RequesterID == in.GetApproverId() {
		return nil, poolErrPublication()
	}

	ok, err := repo.ApprovePoolPublication(tx, in.GetPoolId(), in.GetVersion(),
		in.GetExpectedConfigHash(), in.GetApproverId(), time.Now())
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	if !ok {
		return nil, poolErrPublication()
	}
	if err = tx.Commit().Error; err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	approval.ApproverID = in.GetApproverId()
	approval.Status = repo.ApprovalApproved
	now := time.Now()
	approval.ApprovedAt = &now
	return l.approvalToResp(approval), nil
}

// approvalToResp 审批行 → 接口形状（approved = 状态 APPROVED）。
func (l *ApprovePoolPublicationLogic) approvalToResp(a *repo.PoolPublicationApproval) *draw.PoolPublicationApproval {
	return &draw.PoolPublicationApproval{
		PoolId:      a.PoolID,
		Version:     a.Version,
		ConfigHash:  a.ConfigHash,
		RequesterId: a.RequesterID,
		ApproverId:  a.ApproverID,
		Approved:    a.Status == repo.ApprovalApproved,
	}
}
