package gameimpl

import (
	"context"
	"errors"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

func NewGetFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFullCarLogic {
	return &GetFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetFullCar 满车详情：配置 + 座位 + 余票 + 状态；整车奖仅在 DEPARTED 后可见。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *GetFullCarLogic) GetFullCar(in *play.GetFullCarReq) (*play.GetFullCarResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	cfg, err := l.svcCtx.FullCarCfgModel.FindOne(l.ctx, in.ConfigId)
	if err != nil {
		if errors.Is(err, gamemodel.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "满车配置不存在")
		}
		l.Errorf("GetFullCar query config failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	var st gamemodelnc.FullCarState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "满车状态不存在")
		}
		l.Errorf("GetFullCar query state failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	seats := fullCarParseSeats(st.SeatsJson)
	seatItems := make([]*play.FullCarSeatItem, 0, len(seats))
	for i, u := range seats {
		seatItems = append(seatItems, &play.FullCarSeatItem{SeatIndex: int64(i), UserId: u})
	}

	resp := &play.GetFullCarResp{
		ConfigId:         cfg.Id,
		Name:             cfg.Name,
		Price:            utils.FenToYuan(cfg.PriceFen),
		SeatCount:        cfg.SeatCount,
		Seats:            seatItems,
		RemainingTickets: st.RemainingTickets,
		Status:           st.Status,
	}
	if st.Status == fullCarStateDeparted {
		resp.Awards = l.awardItems(fullCarParseAwards(st.AwardsJson))
	}
	return resp, nil
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

	if in.RequestId != "" {
		var rec gamemodelnc.FullCarRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {

				return &play.ListFullCarRecordsResp{Items: []*play.FullCarRecordItem{}, Total: 0}, nil
			}
			l.Errorf("ListFullCarRecords query failed, err: %v", err)
			return nil, err
		}
		return &play.ListFullCarRecordsResp{Items: []*play.FullCarRecordItem{l.buildItem(&rec)}, Total: 1}, nil
	}

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
