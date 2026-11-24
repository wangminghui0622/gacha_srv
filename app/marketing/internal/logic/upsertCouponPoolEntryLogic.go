package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type UpsertCouponPoolEntryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertCouponPoolEntryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertCouponPoolEntryLogic {
	return &UpsertCouponPoolEntryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpsertCouponPoolEntry 创建/编辑券池记录（id=0 创建，id>0 编辑）。
// 引用的模板必须是真实券模板（kind 1-4），且已有最新版本。
func (l *UpsertCouponPoolEntryLogic) UpsertCouponPoolEntry(in *marketing.UpsertCouponPoolEntryRequest) (*marketing.CouponPoolEntry, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "tenantId 须为 1（单租户）")
	}
	e := in.Entry
	if e == nil {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "entry 不能为空")
	}
	if e.TemplateId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "templateId 必须大于 0")
	}
	if e.Id == 0 && (e.Count == 0 || e.Count > 1000) {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "张数须在 1-1000 之间")
	}
	if e.DrawCount == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "满抽次数必须大于 0")
	}
	if e.StartAtUnixMillis < 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "生效开始时间不能为负数")
	}
	tpl, err := repo.GetLatestCouponTemplateVersion(l.svcCtx.DB, int64(e.TemplateId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeCouponTemplateNotFound, "引用的优惠券模板不存在")
		}
		return nil, err
	}
	if tpl.Kind < repo.PlayerTplKindFixedMinor || tpl.Kind > repo.PlayerTplKindCustomMinor {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "引用的模板不是有效优惠券模板")
	}

	row := &repo.CouponPoolEntryRow{
		TemplateID:     int64(e.TemplateId),
		Count:          int32(e.Count),
		RemainingCount: int32(e.Count),
		DrawCount:      int32(e.DrawCount),
		StartAt:        e.StartAtUnixMillis,
		Enabled:        boolToInt32(e.Enabled),
	}
	if e.Id == 0 {
		now := time.Now()
		row.CreatedAt = now
		row.UpdatedAt = now
		if err := repo.InsertCouponPoolEntry(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	} else {
		existing, err := repo.GetCouponPoolEntry(l.svcCtx.DB, int64(e.Id))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewCodeError(errorx.CodeCouponPoolNotFound, "券池记录不存在")
			}
			return nil, err
		}
		row.ID = existing.ID
		if err := repo.UpdateCouponPoolEntry(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	}
	saved, err := repo.GetCouponPoolEntry(l.svcCtx.DB, row.ID)
	if err != nil {
		return nil, err
	}
	return saved.ToProto(), nil
}
