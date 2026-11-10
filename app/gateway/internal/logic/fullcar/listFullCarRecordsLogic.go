package fullcar

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListFullCarRecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFullCarRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFullCarRecordsLogic {
	return &ListFullCarRecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListFullCarRecords 满车记录：requestId 非空=精确查一条（购买轮询），空=分页列表
func (l *ListFullCarRecordsLogic) ListFullCarRecords(req *types.FullCarRecordsReq) (resp *types.FullCarRecordsResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.ListFullCarRecords(l.ctx, &playClient.ListFullCarRecordsReq{
		UserId:    userId,
		RequestId: req.RequestId,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		l.Errorf("ListFullCarRecords failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	items := make([]types.FullCarRecordItem, 0, len(r.Items))
	for _, it := range r.Items {
		items = append(items, types.FullCarRecordItem{
			RecordId:  it.RecordId,
			RequestId: it.RequestId,
			ConfigId:  it.ConfigId,
			SeatIndex: it.SeatIndex,
			EntryName: it.EntryName,
			IsPrize:   it.IsPrize,
			Price:     it.Price,
			Status:    it.Status,
			CreatedAt: it.CreatedAt,
		})
	}
	return &types.FullCarRecordsResp{Items: items, Total: r.Total}, nil
}
