package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	listCouponTemplatesDefaultPageSize = 20
	listCouponTemplatesMaxPageSize     = 100
)

type ListCouponTemplateVersionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCouponTemplateVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCouponTemplateVersionsLogic {
	return &ListCouponTemplateVersionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListCouponTemplateVersions 设计 D5：游标 (after_template_id, after_version) 分页，
// 回全版本行（不可变版本列表）。
func (l *ListCouponTemplateVersionsLogic) ListCouponTemplateVersions(in *marketing.ListCouponTemplateVersionsRequest) (*marketing.ListCouponTemplateVersionsReply, error) {
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = listCouponTemplatesDefaultPageSize
	}
	if pageSize > listCouponTemplatesMaxPageSize {
		pageSize = listCouponTemplatesMaxPageSize
	}
	// 多取一行判断是否还有下一页
	rows, err := repo.ListCouponTemplateVersions(l.svcCtx.DB,
		int64(in.AfterTemplateId), int64(in.AfterVersion), pageSize+1)
	if err != nil {
		return nil, err
	}
	reply := &marketing.ListCouponTemplateVersionsReply{}
	if len(rows) > pageSize {
		rows = rows[:pageSize]
		last := rows[len(rows)-1]
		reply.NextAfterTemplateId = uint64(last.TemplateID)
		reply.NextAfterVersion = uint64(last.Version)
	}
	templates := make([]*marketing.CouponTemplate, 0, len(rows))
	for i := range rows {
		templates = append(templates, rows[i].ToProto())
	}
	reply.Templates = templates
	return reply, nil
}
