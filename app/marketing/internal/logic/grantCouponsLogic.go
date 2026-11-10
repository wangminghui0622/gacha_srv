package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/marketing/internal/core"
	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// errGrantReplay 并发同 requestId 时事务内回退标记：commit 无意义，出事务后重读已发批次。
var errGrantReplay = errors.New("grant replay")

type GrantCouponsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantCouponsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantCouponsLogic {
	return &GrantCouponsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantCoupons 设计 D5：读 LatestPublished 版本；CUSTOM_MINOR 须 0<custom≤amount_minor、
// 其余 custom 必须 0；逐张铸 user_coupons（coupon_no=gc_<requestId>#<n> uk 幂等）。
func (l *GrantCouponsLogic) GrantCoupons(in *marketing.GrantCouponsRequest) (*marketing.GrantCouponsReply, error) {
	if err := core.ValidateGrantCoupons(in); err != nil {
		return nil, errorx.NewCodeErrorf(errorx.CodeCouponInvalid, "%v", err)
	}
	latest, err := repo.GetLatestCouponTemplateVersion(l.svcCtx.DB, int64(in.TemplateId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeCouponTemplateNotFound, "券模板不存在")
		}
		return nil, err
	}
	tpl := latest.ToProto()
	if err := core.CheckCustomAmount(tpl, in.CustomAmountMinor); err != nil {
		return nil, errorx.NewCodeErrorf(errorx.CodeCouponInvalid, "%v", err)
	}

	prefix := core.GrantCouponNo(in.RequestId, 0)
	prefix = prefix[:len(prefix)-1] // gc_<requestId>#（去序号）

	// 幂等重放：同 requestId 已发过 → 数量一致返回原批、不一致冲突
	existing, err := repo.ListUserCouponsByPrefix(l.svcCtx.DB, int64(in.UserId), prefix)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		if len(existing) != int(in.Count) {
			return nil, errorx.NewCodeError(errorx.CodeCouponTemplateConflict, "requestId 已存在且发券数量不同")
		}
		return couponsReplyOf(in, existing, uint64(latest.Version)), nil
	}

	var rows []repo.UserCouponGrantRow
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var expiresAt *time.Time
		if tpl.ValidityDays > 0 {
			t := time.Now().Add(time.Duration(tpl.ValidityDays) * 24 * time.Hour)
			expiresAt = &t
		}
		templateNo := core.ConsoleTemplateNo(in.TemplateId)
		for i := uint32(0); i < in.Count; i++ {
			row := repo.UserCouponGrantRow{
				CouponNo:          core.GrantCouponNo(in.RequestId, int(i)),
				UserID:            int64(in.UserId),
				TemplateNo:        templateNo,
				CustomAmountMinor: in.CustomAmountMinor,
				ExpiresAt:         expiresAt,
			}
			if err := repo.InsertUserCouponGrant(tx, &row); err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					// 并发同 requestId：事务内读不到未提交行，回退标记，出事务后重读
					return errGrantReplay
				}
				return err
			}
			rows = append(rows, row)
		}
		return nil
	})
	if errors.Is(err, errGrantReplay) {
		existing, e := repo.ListUserCouponsByPrefix(l.svcCtx.DB, int64(in.UserId), prefix)
		if e != nil {
			return nil, e
		}
		if len(existing) != int(in.Count) {
			return nil, errorx.NewCodeError(errorx.CodeCouponTemplateConflict, "requestId 已存在且发券数量不同")
		}
		return couponsReplyOf(in, existing, uint64(latest.Version)), nil
	}
	if err != nil {
		return nil, err
	}
	return couponsReplyOf(in, rows, uint64(latest.Version)), nil
}

// couponsReplyOf 铸券行 → console UserCoupon 响应（sourceId=幂等来源键 coupon_no）。
// 本仓 user_coupons 无 template_version 列（结构性偏离记录），统一回当前 latest 版本号。
func couponsReplyOf(req *marketing.GrantCouponsRequest, rows []repo.UserCouponGrantRow, templateVersion uint64) *marketing.GrantCouponsReply {
	coupons := make([]*marketing.UserCoupon, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		var expiresMillis int64
		if r.ExpiresAt != nil {
			expiresMillis = r.ExpiresAt.UnixMilli()
		}
		coupons = append(coupons, &marketing.UserCoupon{
			Id:                  uint64(r.ID),
			TenantId:            1,
			UserId:              uint64(r.UserID),
			TemplateId:          req.TemplateId,
			TemplateVersion:     templateVersion,
			Status:              marketing.UserCouponStatus_USER_COUPON_STATUS_AVAILABLE,
			CustomAmountMinor:   r.CustomAmountMinor,
			ExpiresAtUnixMillis: expiresMillis,
			Source:              req.Source,
			SourceId:            r.CouponNo,
		})
	}
	return &marketing.GrantCouponsReply{Coupons: coupons}
}
