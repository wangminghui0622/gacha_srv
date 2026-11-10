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

type ListRiskCasesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRiskCasesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRiskCasesLogic {
	return &ListRiskCasesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListRiskCases 案件列表（D4）：游标 (before_created_at, before_case_id) 倒序分页，
// actorId/status 过滤（0=全部），pageSize 默认 20 封顶 100。
func (l *ListRiskCasesLogic) ListRiskCases(in *base.ListRiskCasesRequest) (*base.ListRiskCasesReply, error) {
	if in.TenantId != 0 && in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeRiskCaseInvalid, "tenantId 须为 1")
	}
	if in.Status > base.RiskCaseStatus_RISK_CASE_STATUS_DISMISSED {
		return nil, errorx.NewCodeError(errorx.CodeRiskCaseInvalid, "status 非法")
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var beforeCreatedAt time.Time
	if in.BeforeCreatedAtUnixMillis > 0 {
		beforeCreatedAt = time.UnixMilli(in.BeforeCreatedAtUnixMillis)
	}
	rows, err := repo.ListRiskCasePage(l.svcCtx.DB, int64(in.ActorId), int64(in.Status),
		beforeCreatedAt, in.BeforeCaseId, pageSize)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &base.ListRiskCasesReply{}, nil
	}
	if err != nil {
		l.Errorf("ListRiskCases query failed, err: %v", err)
		return nil, errorx.FromMySQLError(err)
	}

	reply := &base.ListRiskCasesReply{}
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	for i := range rows {
		r := &rows[i]
		reply.Cases = append(reply.Cases, &base.RiskCaseSummary{
			CaseId:             r.CaseNo,
			DecisionId:         r.DecisionID,
			SceneCode:          r.Scene,
			UserId:             uint64(r.UserID),
			DeviceId:           0, // 本仓无设备维度
			Action:             base.RiskDecisionAction_RISK_DECISION_ACTION_CHALLENGE,
			RiskScore:          int32(r.RiskScore),
			Status:             base.RiskCaseStatus(r.Status),
			AssignedActorId:    uint64(r.AssignedActorID),
			CreatedAtUnixMillis: r.CreatedAt.UnixMilli(),
		})
	}
	if hasMore {
		last := rows[len(rows)-1]
		reply.NextBeforeCreatedAtUnixMillis = last.CreatedAt.UnixMilli()
		reply.NextBeforeCaseId = last.CaseNo
	}
	return reply, nil
}
