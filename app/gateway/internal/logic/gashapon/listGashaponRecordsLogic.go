package gashapon

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListGashaponRecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListGashaponRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGashaponRecordsLogic {
	return &ListGashaponRecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListGashaponRecords 扭蛋记录：requestId 非空=精确查一条（购买轮询），空=分页列表
func (l *ListGashaponRecordsLogic) ListGashaponRecords(req *types.GashaponRecordsReq) (resp *types.GashaponRecordsResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.ListGashaponRecords(l.ctx, &playClient.ListGashaponRecordsReq{
		UserId:    userId,
		RequestId: req.RequestId,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		l.Errorf("ListGashaponRecords failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	items := make([]types.GashaponRecordItem, 0, len(r.Items))
	for _, it := range r.Items {
		entries := make([]types.GashaponEntryOutcome, 0, len(it.Entries))
		for _, e := range it.Entries {
			entries = append(entries, types.GashaponEntryOutcome{Name: e.Name, IsPrize: e.IsPrize})
		}
		items = append(items, types.GashaponRecordItem{
			RecordId:  it.RecordId,
			RequestId: it.RequestId,
			ConfigId:  it.ConfigId,
			Count:     it.Count,
			Entries:   entries,
			Price:     it.Price,
			Status:    it.Status,
			CreatedAt: it.CreatedAt,
		})
	}
	return &types.GashaponRecordsResp{Items: items, Total: r.Total}, nil
}
