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

type ListDigRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDigRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDigRecordsLogic {
	return &ListDigRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListDigRecords 挖宝记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListDigRecordsLogic) ListDigRecords(in *play.ListDigRecordsReq) (*play.ListDigRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	// 精确查询（轮询开奖结果）
	if in.RequestId != "" {
		var rec gamemodelnc.DigBoxRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewCodeError(errorx.CodeDigRecordNotFound, "挖宝记录不存在")
			}
			l.Errorf("ListDigRecords query failed, err: %v", err)
			return nil, err
		}
		item, err := l.buildItem(&rec)
		if err != nil {
			return nil, err
		}
		return &play.ListDigRecordsResp{Items: []*play.DigRecordItem{item}, Total: 1}, nil
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
	if err := l.svcCtx.DB.Model(&gamemodelnc.DigBoxRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListDigRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.DigBoxRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListDigRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.DigRecordItem, 0, len(recs))
	for i := range recs {
		item, err := l.buildItem(&recs[i])
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &play.ListDigRecordsResp{Items: items, Total: total}, nil
}

// buildItem 记录行 → 响应条目（prizes_json 解析 + 金额转元）
func (l *ListDigRecordsLogic) buildItem(rec *gamemodelnc.DigBoxRecords) (*play.DigRecordItem, error) {
	prizes, err := digGrantPrizesOfRecord(rec.PrizesJson)
	if err != nil {
		l.Errorf("ListDigRecords parse prizes failed, recordId: %d, err: %v", rec.Id, err)
		return nil, err
	}
	items := make([]*play.PrizeItem, 0, len(prizes))
	for _, p := range prizes {
		items = append(items, digPrizeItemOf(p))
	}
	return &play.DigRecordItem{
		RecordId:    rec.Id,
		RequestId:   rec.RequestId,
		ConfigId:    rec.ConfigId,
		CellsChosen: rec.CellsChosen,
		Prizes:      items,
		Price:       utils.FenToYuan(rec.TotalPriceFen),
		Status:      rec.Status,
		CreatedAt:   rec.CreatedAt.Unix(),
	}, nil
}
