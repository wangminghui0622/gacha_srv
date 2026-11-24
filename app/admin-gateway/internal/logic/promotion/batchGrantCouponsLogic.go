// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/task/taskClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	batchGrantDefaultMaxUsers = 50
	batchGrantHardMaxUsers    = 200
	batchGrantPageSize        = 100
	batchGrantMaxErrSamples   = 20
)

type BatchGrantCouponsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchGrantCouponsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchGrantCouponsLogic {
	return &BatchGrantCouponsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BatchGrantCoupons 按累计消费区间筛选会员用户并批量发券。
// 单次最多处理 200 人，库存不足时提前停止；每人发券使用 <requestId>:<userId> 幂等。
func (l *BatchGrantCouponsLogic) BatchGrantCoupons(in *marketing.BatchGrantCouponsRequest) (*marketing.BatchGrantCouponsReply, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "tenantId 须为 1（单租户）")
	}
	if in.PoolEntryId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "poolEntryId 必须大于 0")
	}
	if in.PerUserCount == 0 || in.PerUserCount > 1000 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "每人张数须在 1-1000 之间")
	}
	if in.MinSpendMinor < 0 || in.MaxSpendMinor < 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "消费区间不能为负数")
	}
	if in.MaxSpendMinor > 0 && in.MaxSpendMinor < in.MinSpendMinor {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "最高消费不能小于最低消费")
	}
	maxUsers := int(in.MaxUsers)
	if maxUsers <= 0 {
		maxUsers = batchGrantDefaultMaxUsers
	}
	if maxUsers > batchGrantHardMaxUsers {
		maxUsers = batchGrantHardMaxUsers
	}

	entry, err := l.svcCtx.MarketingRpc.GetCouponPoolEntry(l.ctx, &marketing.GetCouponPoolEntryRequest{
		TenantId: 1,
		Id:       in.PoolEntryId,
	})
	if err != nil {
		return nil, err
	}
	maxByStock := int(entry.RemainingCount) / int(in.PerUserCount)
	if maxByStock <= 0 {
		return &marketing.BatchGrantCouponsReply{StoppedReason: "stock_exhausted", RemainingCount: entry.RemainingCount}, nil
	}
	target := maxUsers
	if target > maxByStock {
		target = maxByStock
	}

	userIDs, err := l.listAudience(in.MinSpendMinor, in.MaxSpendMinor, target)
	if err != nil {
		return nil, err
	}
	reply := &marketing.BatchGrantCouponsReply{
		MatchedUsers:  uint32(len(userIDs)),
		FailedUserIds: make([]uint64, 0, batchGrantMaxErrSamples),
	}
	if len(userIDs) == 0 {
		reply.StoppedReason = "no_users"
		reply.RemainingCount = entry.RemainingCount
		return reply, nil
	}
	if len(userIDs) >= target {
		if target < maxUsers {
			reply.StoppedReason = "stock_limited"
		} else {
			reply.StoppedReason = "max_users"
		}
	}

	batchID := in.RequestId
	if batchID == "" {
		batchID = fmt.Sprintf("batch-%d", time.Now().UnixNano())
	}
	for _, userID := range userIDs {
		_, grantErr := l.svcCtx.MarketingRpc.GrantCoupons(l.ctx, &marketing.GrantCouponsRequest{
			RequestId:         fmt.Sprintf("%s:%d", batchID, userID),
			TenantId:          1,
			UserId:            uint64(userID),
			TemplateId:        entry.TemplateId,
			Count:             in.PerUserCount,
			CustomAmountMinor: in.CustomAmountMinor,
			Source:            marketing.CouponGrantSource_COUPON_GRANT_SOURCE_CONSOLE,
			PoolEntryId:       in.PoolEntryId,
			IgnoreStartAt:     true,
		})
		if grantErr != nil {
			reply.FailedUsers++
			if len(reply.FailedUserIds) < batchGrantMaxErrSamples {
				reply.FailedUserIds = append(reply.FailedUserIds, uint64(userID))
			}
			if isStockUnavailable(grantErr) {
				reply.StoppedReason = "stock_exhausted"
				break
			}
			continue
		}
		reply.GrantedUsers++
	}

	if latest, e := l.svcCtx.MarketingRpc.GetCouponPoolEntry(l.ctx, &marketing.GetCouponPoolEntryRequest{TenantId: 1, Id: in.PoolEntryId}); e == nil {
		reply.RemainingCount = latest.RemainingCount
	} else {
		l.Errorf("BatchGrantCoupons reload pool entry failed, entryId: %d, err: %v", in.PoolEntryId, e)
	}
	return reply, nil
}

func (l *BatchGrantCouponsLogic) listAudience(minSpendMinor, maxSpendMinor int64, limit int) ([]int64, error) {
	ids := make([]int64, 0, limit)
	var after int64
	for len(ids) < limit {
		pageSize := batchGrantPageSize
		if remain := limit - len(ids); remain < pageSize {
			pageSize = remain
		}
		resp, err := l.svcCtx.TaskRpc.ListMemberUserIds(l.ctx, &taskClient.ListMemberUserIdsReq{
			MinSpendMinor: minSpendMinor,
			MaxSpendMinor: maxSpendMinor,
			AfterUserId:   after,
			Limit:         uint32(pageSize),
		})
		if err != nil {
			return nil, err
		}
		if len(resp.UserIds) == 0 {
			break
		}
		ids = append(ids, resp.UserIds...)
		if resp.NextAfterUserId == 0 {
			break
		}
		after = resp.NextAfterUserId
	}
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids, nil
}

func isStockUnavailable(err error) bool {
	var ce *errorx.CodeError
	return errors.As(err, &ce) && ce.Code == errorx.CodeCouponUnavailable
}
