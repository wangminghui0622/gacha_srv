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

type RequestPoolPublicationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRequestPoolPublicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestPoolPublicationLogic {
	return &RequestPoolPublicationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RequestPoolPublication 提请发布（server_v2 RequestPublication 语义）：
// 版本必须 DRAFT 且 hash 一致 → 锁审批行：同人同 hash 重放幂等返回；
// 已存在他人/他 hash 审批行 → 冲突。新提请插入 PENDING 审批行。
func (l *RequestPoolPublicationLogic) RequestPoolPublication(in *draw.RequestPoolPublicationReq) (*draw.PoolPublicationApproval, error) {
	if err := poolValidateKey(in.GetPoolId(), in.GetVersion(), in.GetExpectedConfigHash()); err != nil {
		return nil, err
	}
	if in.GetRequesterId() <= 0 {
		return nil, errorx.NewCodeError(errorx.CodePoolDraftInvalid, "requesterId 必填")
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
	if err == nil {
		// 同人同 hash 重放 → 幂等返回既有审批行（含已批准状态）
		if approval.RequesterID == in.GetRequesterId() && bytes.Equal(approval.ConfigHash, in.GetExpectedConfigHash()) {
			return l.approvalToResp(approval), nil
		}
		return nil, poolErrPublication()
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errorx.FromMySQLError(err)
	}

	row := &repo.PoolPublicationApproval{
		PoolID:      in.GetPoolId(),
		Version:     in.GetVersion(),
		ConfigHash:  append([]byte(nil), in.GetExpectedConfigHash()...),
		RequesterID: in.GetRequesterId(),
		ApproverID:  0,
		Status:      repo.ApprovalPending,
		RequestedAt: time.Now(),
	}
	if err = repo.InsertPoolPublicationApproval(tx, row); err != nil {
		// 并发同版本提请：回滚后重读，按重放规则返回
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			_ = tx.Rollback()
			if prior, rerr := repo.LockPoolPublicationApproval(l.svcCtx.DB, in.GetPoolId(), in.GetVersion()); rerr == nil {
				if prior.RequesterID == in.GetRequesterId() && bytes.Equal(prior.ConfigHash, in.GetExpectedConfigHash()) {
					return l.approvalToResp(prior), nil
				}
			}
		}
		return nil, errorx.FromMySQLError(err)
	}
	if err = tx.Commit().Error; err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return l.approvalToResp(row), nil
}

// approvalToResp 审批行 → 接口形状（approved = 状态 APPROVED）。
func (l *RequestPoolPublicationLogic) approvalToResp(a *repo.PoolPublicationApproval) *draw.PoolPublicationApproval {
	return &draw.PoolPublicationApproval{
		PoolId:      a.PoolID,
		Version:     a.Version,
		ConfigHash:  a.ConfigHash,
		RequesterId: a.RequesterID,
		ApproverId:  a.ApproverID,
		Approved:    a.Status == repo.ApprovalApproved,
	}
}
