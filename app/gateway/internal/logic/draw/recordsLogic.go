package draw

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type RecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecordsLogic {
	return &RecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Records 抽赏记录：
//   - requestId 有值：精确查一条（购买轮询用），未找到透出 2004，
//     前端以此继续轮询（开奖失败无记录，配合超时与余额刷新兜底）
//   - 否则：分页列表
func (l *RecordsLogic) Records(req *types.RecordsReq) (resp *types.RecordsListResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}

	if req.RequestId != "" {
		r, err := l.svcCtx.DrawRpc.GetRecord(l.ctx, &drawClient.GetRecordReq{
			RequestId: req.RequestId,
			UserId:    userId,
		})
		if err != nil {
			return nil, errorx.FromError(err)
		}
		return &types.RecordsListResp{List: []types.DrawRecordItem{toRecordItem(r.Record)}, Total: 1}, nil
	}

	r, err := l.svcCtx.DrawRpc.ListRecords(l.ctx, &drawClient.ListRecordsReq{
		UserId:   userId,
		Page:     int64(req.Page),
		PageSize: int64(req.PageSize),
	})
	if err != nil {
		l.Errorf("Records list failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	items := make([]types.DrawRecordItem, 0, len(r.Records))
	for _, rec := range r.Records {
		items = append(items, toRecordItem(rec))
	}
	return &types.RecordsListResp{List: items, Total: r.Total}, nil
}

// toRecordItem draw.RecordItem → 网关类型
func toRecordItem(rec *drawClient.RecordItem) types.DrawRecordItem {
	return types.DrawRecordItem{
		RecordId:  rec.RecordId,
		RequestId: rec.RequestId,
		BoxId:     rec.BoxId,
		BoxName:   rec.BoxName,
		EntryName: rec.EntryName,
		IsPrize:   rec.IsPrize,
		Status:    int64(rec.Status),
		Price:     rec.Price,
		CreatedAt: rec.CreatedAt,
	}
}
