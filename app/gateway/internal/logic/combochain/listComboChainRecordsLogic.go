package combochain

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListComboChainRecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListComboChainRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListComboChainRecordsLogic {
	return &ListComboChainRecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListComboChainRecords 连击链记录：requestId 非空=精确查一条（购买轮询），空=分页列表
func (l *ListComboChainRecordsLogic) ListComboChainRecords(req *types.ComboChainRecordsReq) (resp *types.ComboChainRecordsResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.ListComboChainRecords(l.ctx, &playClient.ListComboChainRecordsReq{
		UserId:    userId,
		RequestId: req.RequestId,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		l.Errorf("ListComboChainRecords failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	items := make([]types.ComboChainRecordItem, 0, len(r.Items))
	for _, it := range r.Items {
		awards := make([]types.DigPrizeItem, 0, len(it.Awards))
		for _, a := range it.Awards {
			awards = append(awards, prizeItemToView(a))
		}
		items = append(items, types.ComboChainRecordItem{
			RecordId:  it.RecordId,
			RequestId: it.RequestId,
			ConfigId:  it.ConfigId,
			CarIndex:  it.CarIndex,
			Awards:    awards,
			Price:     it.Price,
			Status:    it.Status,
			CreatedAt: it.CreatedAt,
		})
	}
	return &types.ComboChainRecordsResp{Items: items, Total: r.Total}, nil
}
