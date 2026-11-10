package logic

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/svc"
	drawmodelnc "gacha_srv/pkg/model_nocache/gacha_draw"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRecordsLogic {
	return &ListRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListRecords 抽赏记录分页（按用户、倒序；写多读少，nocache 直查）
func (l *ListRecordsLogic) ListRecords(in *draw.ListRecordsReq) (*draw.ListRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	page := in.Page
	if page < 1 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	if err := l.svcCtx.DB.WithContext(l.ctx).Model(&drawmodelnc.DrawRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListRecords count failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.FromMySQLError(err)
	}
	var recs []drawmodelnc.DrawRecords
	if err := l.svcCtx.DB.WithContext(l.ctx).Where("user_id = ?", in.UserId).
		Order("id DESC").Limit(int(pageSize)).Offset(int((page - 1) * pageSize)).Find(&recs).Error; err != nil {
		l.Errorf("ListRecords failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.FromMySQLError(err)
	}
	names, err := boxNames(l.ctx, l.svcCtx.DB)
	if err != nil {
		l.Errorf("ListRecords boxNames failed, err: %v", err)
		return nil, errorx.FromMySQLError(err)
	}

	items := make([]*draw.RecordItem, 0, len(recs))
	for _, r := range recs {
		items = append(items, &draw.RecordItem{
			RecordId:  r.Id,
			RequestId: r.RequestId,
			BoxId:     r.BoxId,
			BoxName:   names[r.BoxId],
			EntryName: r.EntryName,
			IsPrize:   r.IsPrize == 1,
			Price:     utils.FenToYuan(r.Price),
			CreatedAt: r.CreatedAt.Unix(),
			Status:    int32(r.Status),
		})
	}
	return &draw.ListRecordsResp{Records: items, Total: total}, nil
}
