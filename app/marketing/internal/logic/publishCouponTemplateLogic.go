package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/marketing/internal/core"
	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

type PublishCouponTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishCouponTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishCouponTemplateLogic {
	return &PublishCouponTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PublishCouponTemplate 设计 D5：request_id 幂等（同号同内容重放返回原版本、异同冲突）；
// 锁 head（无则建）→ version=next_version → 插不可变版本行 → head CAS 前进；
// 同时物化 player coupon_templates 行（template_no=CONSOLE-<templateId>）。
func (l *PublishCouponTemplateLogic) PublishCouponTemplate(in *marketing.PublishCouponTemplateRequest) (*marketing.CouponTemplate, error) {
	if in.RequestId == "" || len(in.RequestId) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "requestId 须非空且 ≤64 字符")
	}
	t := in.Template
	if t == nil {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "template 不能为空")
	}
	if t.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "tenantId 须为 1（单租户）")
	}
	if t.TemplateId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "templateId 必须大于 0")
	}
	if err := core.ValidateCouponTemplate(t); err != nil {
		return nil, errorx.NewCodeErrorf(errorx.CodeCouponInvalid, "%v", err)
	}

	var versionRow *repo.CouponTemplateVersionRow
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		// 幂等重放：同 request_id 已发布 → 内容全同返回原版本、异同冲突
		existing, err := repo.GetCouponTemplateVersionByRequestID(tx, in.RequestId)
		if err == nil {
			if couponTemplateContentEqual(t, existing) {
				versionRow = existing
				return nil
			}
			return errorx.NewCodeError(errorx.CodeCouponTemplateConflict, "requestId 已存在且内容不同")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// DRAW_REWARD：引用的券模板须已发布（latest 存在）
		if t.Kind == marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_DRAW_REWARD {
			if _, err := repo.GetLatestCouponTemplateVersion(tx, int64(t.RewardTemplateId)); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errorx.NewCodeError(errorx.CodeCouponInvalid, "rewardTemplateId 指向的券模板未发布")
				}
				return err
			}
		}
		if err := repo.EnsureCouponTemplateHead(tx, int64(t.TemplateId)); err != nil {
			return err
		}
		head, err := repo.LockCouponTemplateHead(tx, int64(t.TemplateId))
		if err != nil {
			return err
		}
		row := &repo.CouponTemplateVersionRow{
			TemplateID:        int64(t.TemplateId),
			Version:           head.NextVersion,
			Kind:              int32(t.Kind),
			AmountMinor:       t.AmountMinor,
			PercentBps:        t.PercentBps,
			MinGrossMinor:     t.MinGrossMinor,
			Currency:          t.Currency,
			Funder:            int32(t.Funder),
			Scene:             int32(t.Scene),
			MinUnitPriceMinor: t.MinUnitPriceMinor,
			MaxUnitPriceMinor: t.MaxUnitPriceMinor,
			MinItemCount:      int32(t.MinItemCount),
			MaxItemCount:      int32(t.MaxItemCount),
			TargetBusinessKey: t.TargetBusinessKey,
			ValidityDays:      int32(t.ValidityDays),
			RewardTemplateID:  int64(t.RewardTemplateId),
			RewardCount:       int32(t.RewardCount),
			RequestID:         in.RequestId,
			PublishedAt:       time.Now(),
		}
		if err := repo.InsertCouponTemplateVersion(tx, row); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errorx.NewCodeError(errorx.CodeCouponTemplateConflict, "requestId 已存在")
			}
			return err
		}
		ok, err := repo.AdvanceCouponTemplateHeadCAS(tx, row.TemplateID, head.NextVersion, head.NextVersion+1)
		if err != nil {
			return err
		}
		if !ok {
			return errorx.NewCodeError(errorx.CodeCouponTemplateConflict, "模板头版本已被并发推进")
		}
		// 物化 player 行（kind 枚举与 player 表同值；value=amount_minor 或 percent_bps）
		value := t.AmountMinor
		if t.Kind == marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_PERCENT_BPS {
			value = int64(t.PercentBps)
		}
		if err := repo.UpsertPlayerCouponTemplate(tx, &repo.PlayerCouponTemplateRow{
			TemplateNo: core.ConsoleTemplateNo(t.TemplateId),
			Name:       "console",
			Kind:       int32(t.Kind),
			Value:      value,
			MinGross:   t.MinGrossMinor,
		}); err != nil {
			return err
		}
		versionRow = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	// raw upsert 绕过缓存模型：commit 后手动失效 player 券模板缓存（best-effort，兜底靠 30s TTL）
	templateNo := core.ConsoleTemplateNo(t.TemplateId)
	if id, e := repo.GetPlayerCouponTemplateID(l.svcCtx.DB, templateNo); e == nil {
		if e := l.svcCtx.TemplateModel.DeleteCache(l.ctx, &gamemodel.CouponTemplates{
			Id:         id,
			TemplateNo: templateNo,
		}); e != nil {
			l.Errorf("PublishCouponTemplate DeleteCache failed, templateNo: %s, err: %v", templateNo, e)
		}
	} else {
		l.Errorf("PublishCouponTemplate get player row id failed, templateNo: %s, err: %v", templateNo, e)
	}
	return versionRow.ToProto(), nil
}

// couponTemplateContentEqual 重放内容比较：模板 ID/版本/状态/租户为幂等键归属，
// 不参与内容比较（归一化后逐字段 proto.Equal）。
func couponTemplateContentEqual(req *marketing.CouponTemplate, stored *repo.CouponTemplateVersionRow) bool {
	a := proto.Clone(req).(*marketing.CouponTemplate)
	a.TemplateId, a.Version, a.Status, a.TenantId = 0, 0, 0, 0
	b := stored.ToProto()
	b.TemplateId, b.Version, b.Status, b.TenantId = 0, 0, 0, 0
	return proto.Equal(a, b)
}
