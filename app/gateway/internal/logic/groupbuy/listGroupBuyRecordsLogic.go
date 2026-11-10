package groupbuy

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListGroupBuyRecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListGroupBuyRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGroupBuyRecordsLogic {
	return &ListGroupBuyRecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListGroupBuyRecords 拼团记录：requestId 非空=精确查一条（购买轮询），空=分页列表
func (l *ListGroupBuyRecordsLogic) ListGroupBuyRecords(req *types.GroupBuyRecordsReq) (resp *types.GroupBuyRecordsResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.ListGroupBuyRecords(l.ctx, &playClient.ListGroupBuyRecordsReq{
		UserId:    userId,
		RequestId: req.RequestId,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		l.Errorf("ListGroupBuyRecords failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	items := make([]types.GroupBuyRecordItem, 0, len(r.Items))
	for _, it := range r.Items {
		awards := make([]types.DigPrizeItem, 0, len(it.Awards))
		for _, a := range it.Awards {
			awards = append(awards, digPrizeItemOf(a))
		}
		items = append(items, types.GroupBuyRecordItem{
			RecordId:  it.RecordId,
			RequestId: it.RequestId,
			ConfigId:  it.ConfigId,
			Count:     it.Count,
			Awards:    awards,
			Price:     it.Price,
			Status:    it.Status,
			CreatedAt: it.CreatedAt,
		})
	}
	return &types.GroupBuyRecordsResp{Items: items, Total: r.Total}, nil
}
