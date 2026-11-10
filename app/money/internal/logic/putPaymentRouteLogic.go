package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/money/internal/core"
	"gacha_srv/app/money/internal/repo"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// tenantIDConsole 控制台单租户固定 1（D7）
const tenantIDConsole = int64(1)

type PutPaymentRouteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPutPaymentRouteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutPaymentRouteLogic {
	return &PutPaymentRouteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ── M14d 控制台：支付路由存管（半成品，无消费方；形状照抄 server_v2 payment_route）──
// D6：校验照抄 server_v2 ValidateRoutePolicy（registry 检查按设计偏离省略）→ 行锁 →
// 新建（version=1）/ 更新（policy_id==行 id && expected_version==行 version，否则 2008；
// CAS UPDATE 双保险）→ 候选全量替换 → 审计快照（首建 old_snapshot=NULL）。
func (l *PutPaymentRouteLogic) PutPaymentRoute(in *money.PutPaymentRouteRequest) (*money.PutPaymentRouteReply, error) {
	if err := core.ValidatePutPaymentRoute(in); err != nil {
		return nil, errorx.NewCodeError(errorx.CodePaymentRouteInvalid, err.Error())
	}
	var policyID int64
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		row, err := repo.LockRoutePolicy(tx, tenantIDConsole, int32(in.Policy.Channel), int32(in.Policy.Scene))
		var (
			oldSnap     *string
			oldVersion  int64
			newVersion  int64
			newSnapRows []repo.PaymentRouteCandidateRow
		)
		switch {
		case err == nil:
			// 更新：policy_id==行 id 且 expected_version==行 version（校验已保证两者非 0）
			if uint64(row.ID) != in.Policy.PolicyId || uint64(row.Version) != in.ExpectedVersion {
				return errorx.NewCodeError(errorx.CodePaymentRouteVersionConflict,
					"路由版本冲突：policy_id/expected_version 与当前行不符")
			}
			oldCands, err := repo.ListRouteCandidates(tx, row.ID)
			if err != nil {
				return err
			}
			snap, err := snapshotOf(row, oldCands)
			if err != nil {
				return err
			}
			oldSnap = &snap
			oldVersion, newVersion = row.Version, row.Version+1
			ok, err := repo.UpdateRoutePolicyCAS(tx, row.ID, oldVersion, newVersion,
				int32(in.Policy.Mode), int32(in.Policy.PinnedProvider),
				in.Policy.PinnedMerchantAccountId, int64(in.OperatorUserId))
			if err != nil {
				return err
			}
			if !ok {
				return errorx.NewCodeError(errorx.CodePaymentRouteVersionConflict, "路由版本冲突：并发修改")
			}
			policyID = row.ID
		case errors.Is(err, gorm.ErrRecordNotFound):
			// 新建（校验已保证 policyId==0 && expectedVersion==0）
			nr := &repo.PaymentRoutePolicyRow{
				TenantID:                tenantIDConsole,
				Channel:                 int32(in.Policy.Channel),
				Scene:                   int32(in.Policy.Scene),
				Mode:                    int32(in.Policy.Mode),
				PinnedProvider:          int32(in.Policy.PinnedProvider),
				PinnedMerchantAccountID: in.Policy.PinnedMerchantAccountId,
				Version:                 1,
				UpdatedBy:               int64(in.OperatorUserId),
			}
			if err := repo.InsertRoutePolicy(tx, nr); err != nil {
				return err
			}
			policyID, newVersion = nr.ID, 1
		default:
			return err
		}

		// 候选全量替换（删旧插新；priority 取请求值）
		if err := repo.DeleteRouteCandidates(tx, policyID); err != nil {
			return err
		}
		candRows := make([]repo.PaymentRouteCandidateRow, 0, len(in.Candidates))
		for _, c := range in.Candidates {
			candRows = append(candRows, repo.PaymentRouteCandidateRow{
				PolicyID:          policyID,
				Provider:          int32(c.Provider),
				MerchantAccountID: c.MerchantAccountId,
				Priority:          int32(c.Priority),
				Weight:            int32(c.Weight),
				Enabled:           c.Enabled,
			})
		}
		if err := repo.InsertRouteCandidates(tx, candRows); err != nil {
			return err
		}
		newSnapRows = candRows

		// 审计（old_snapshot 首建为 NULL）
		newSnap, err := snapshotOf(&repo.PaymentRoutePolicyRow{
			ID:                      policyID,
			TenantID:                tenantIDConsole,
			Channel:                 int32(in.Policy.Channel),
			Scene:                   int32(in.Policy.Scene),
			Mode:                    int32(in.Policy.Mode),
			PinnedProvider:          int32(in.Policy.PinnedProvider),
			PinnedMerchantAccountID: in.Policy.PinnedMerchantAccountId,
			Version:                 newVersion,
			UpdatedBy:               int64(in.OperatorUserId),
		}, newSnapRows)
		if err != nil {
			return err
		}
		return repo.InsertRouteAudit(tx, &repo.PaymentRouteAuditRow{
			PolicyID:       policyID,
			TenantID:       tenantIDConsole,
			OldVersion:     oldVersion,
			NewVersion:     newVersion,
			OperatorUserID: int64(in.OperatorUserId),
			OldSnapshot:    oldSnap,
			NewSnapshot:    newSnap,
			ChangedAt:      time.Now(),
		})
	})
	if err != nil {
		return nil, err
	}

	// 回读组装回复（含候选自增 ID 与最新版本）
	row, err := repo.GetRoutePolicy(l.svcCtx.DB, tenantIDConsole, int32(in.Policy.Channel), int32(in.Policy.Scene))
	if err != nil {
		return nil, err
	}
	cands, err := repo.ListRouteCandidates(l.svcCtx.DB, row.ID)
	if err != nil {
		return nil, err
	}
	return &money.PutPaymentRouteReply{
		Policy:     policyRecordOf(row),
		Candidates: candidateRecordsOf(cands),
	}, nil
}
