package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/repo"
	"gacha_srv/app/base/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ClaimRiskCaseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimRiskCaseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimRiskCaseLogic {
	return &ClaimRiskCaseLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ClaimRiskCase 风控案例认领（D4 状态机）：OPEN → IN_REVIEW。
// 事务内行锁案件；同 operation_id 事件已存在 → 字段全同幂等重放、异同冲突；
// 状态非 OPEN → 409 RISK_CASE_CONFLICT。
func (l *ClaimRiskCaseLogic) ClaimRiskCase(in *base.ClaimRiskCaseRequest) (*base.RiskCaseTransition, error) {
	if in.CaseId == "" || in.OperationId == "" || in.ActorId <= 0 || (in.TenantId != 0 && in.TenantId != 1) {
		return nil, errorx.NewCodeError(errorx.CodeRiskCaseInvalid, "caseId/operationId/actorId 不能为空且 tenantId 须为 1")
	}

	var transition *base.RiskCaseTransition
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		c, err := repo.LockRiskCaseByCaseNo(tx, in.CaseId)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.NewCodeError(errorx.CodeRiskCaseNotFound, "风控案例不存在")
		}
		if err != nil {
			return err
		}
		// 幂等重放判断：同 operation_id 事件已存在
		ev, err := repo.GetRiskCaseEvent(tx, in.OperationId)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 首次：状态必须 OPEN
			if c.Status != repo.RiskCaseStatusOpen {
				return errorx.NewCodeError(errorx.CodeRiskCaseStateInvalid, "风控案例已认领或已结案")
			}
			occurredAt := time.Now()
			c.Status = repo.RiskCaseStatusInReview
			c.AssignedActorID = int64(in.ActorId)
			if err := repo.UpdateRiskCaseStatus(tx, &repo.RiskCaseRow{
				ID: c.ID, Status: c.Status, AssignedActorID: c.AssignedActorID,
			}); err != nil {
				return err
			}
			if err := repo.InsertRiskCaseEvent(tx, &repo.RiskCaseEventRow{
				EventID:    in.OperationId,
				CaseID:     in.CaseId,
				EventType:  repo.RiskEventTypeClaimed,
				FromStatus: repo.RiskCaseStatusOpen,
				ToStatus:   repo.RiskCaseStatusInReview,
				ActorID:    int64(in.ActorId),
				OccurredAt: occurredAt,
			}); err != nil {
				return err
			}
			transition = &base.RiskCaseTransition{
				CaseId:               in.CaseId,
				Status:               base.RiskCaseStatus_RISK_CASE_STATUS_IN_REVIEW,
				ActorId:              in.ActorId,
				OccurredAtUnixMillis: occurredAt.UnixMilli(),
			}
			return nil
		}
		if err != nil {
			return err
		}
		// 重放：字段全同 → 幂等返回同迁移；异同 → 冲突
		if ev.CaseID == in.CaseId && ev.EventType == repo.RiskEventTypeClaimed &&
			ev.FromStatus == repo.RiskCaseStatusOpen &&
			ev.ToStatus == repo.RiskCaseStatusInReview &&
			ev.ActorID == int64(in.ActorId) {
			transition = &base.RiskCaseTransition{
				CaseId:               in.CaseId,
				Status:               base.RiskCaseStatus_RISK_CASE_STATUS_IN_REVIEW,
				ActorId:              in.ActorId,
				OccurredAtUnixMillis: ev.OccurredAt.UnixMilli(),
			}
			return nil
		}
		return errorx.NewCodeError(errorx.CodeRiskCaseStateInvalid, "同 operation_id 异参数冲突")
	})
	if err != nil {
		if errorx.IsCodeError(err) {
			return nil, err
		}
		l.Errorf("ClaimRiskCase failed, caseId: %s, err: %v", in.CaseId, err)
		return nil, errorx.FromMySQLError(err)
	}
	l.Infof("ClaimRiskCase success, caseId: %s, actorId: %d", in.CaseId, in.ActorId)
	return transition, nil
}
