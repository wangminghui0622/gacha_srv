package logic

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ListFullCarRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFullCarRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFullCarRecordsLogic {
	return &ListFullCarRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListFullCarRecords 满车记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListFullCarRecordsLogic) ListFullCarRecords(in *play.ListFullCarRecordsReq) (*play.ListFullCarRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	// 精确查询（轮询开奖结果）
	if in.RequestId != "" {
		var rec gamemodelnc.FullCarRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewCodeError(errorx.CodeFullCarStateInvalid, "满车记录不存在")
			}
			l.Errorf("ListFullCarRecords query failed, err: %v", err)
			return nil, err
		}
		return &play.ListFullCarRecordsResp{Items: []*play.FullCarRecordItem{l.buildItem(&rec)}, Total: 1}, nil
	}

	// 分页列表
	page, pageSize := in.Page, in.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := l.svcCtx.DB.Model(&gamemodelnc.FullCarRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListFullCarRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.FullCarRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListFullCarRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.FullCarRecordItem, 0, len(recs))
	for i := range recs {
		items = append(items, l.buildItem(&recs[i]))
	}
	return &play.ListFullCarRecordsResp{Items: items, Total: total}, nil
}

// buildItem 记录行 → 响应条目（金额转元）
func (l *ListFullCarRecordsLogic) buildItem(rec *gamemodelnc.FullCarRecords) *play.FullCarRecordItem {
	return &play.FullCarRecordItem{
		RecordId:  rec.Id,
		RequestId: rec.RequestId,
		ConfigId:  rec.ConfigId,
		SeatIndex: rec.SeatIndex,
		EntryName: rec.EntryName,
		IsPrize:   rec.IsPrize == 1,
		Price:     utils.FenToYuan(rec.PriceFen),
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt.Unix(),
	}
}
