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

type ListComboChainRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListComboChainRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListComboChainRecordsLogic {
	return &ListComboChainRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListComboChainRecords 连击链记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 每条记录附本单触发的结算奖（state.awards_json 按 requestId 过滤，展示/轮询揭晓）。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListComboChainRecordsLogic) ListComboChainRecords(in *play.ListComboChainRecordsReq) (*play.ListComboChainRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	// 精确查询（轮询开奖结果）
	if in.RequestId != "" {
		var rec gamemodelnc.ComboChainRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 3029 语义复用：尚无记录（轮询方吞错继续等）
				return nil, errorx.NewCodeError(errorx.CodeComboChainNoTickets, "连击链记录不存在")
			}
			l.Errorf("ListComboChainRecords query failed, err: %v", err)
			return nil, err
		}
		return &play.ListComboChainRecordsResp{Items: []*play.ComboChainRecordItem{l.buildItem(&rec)}, Total: 1}, nil
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
	if err := l.svcCtx.DB.Model(&gamemodelnc.ComboChainRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListComboChainRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.ComboChainRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListComboChainRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.ComboChainRecordItem, 0, len(recs))
	for i := range recs {
		items = append(items, l.buildItem(&recs[i]))
	}
	return &play.ListComboChainRecordsResp{Items: items, Total: total}, nil
}

// buildItem 记录行 → 响应条目（金额转元；结算奖按 requestId 过滤自 state.awards_json）
func (l *ListComboChainRecordsLogic) buildItem(rec *gamemodelnc.ComboChainRecords) *play.ComboChainRecordItem {
	item := &play.ComboChainRecordItem{
		RecordId:  rec.Id,
		RequestId: rec.RequestId,
		ConfigId:  rec.ConfigId,
		CarIndex:  rec.CarIndex,
		Price:     utils.FenToYuan(rec.PriceFen),
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt.Unix(),
	}
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		if state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson); err == nil {
			for _, a := range comboChainAwardsOfRequest(state.Awards, rec.RequestId) {
				item.Awards = append(item.Awards, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
			}
		}
	}
	return item
}
