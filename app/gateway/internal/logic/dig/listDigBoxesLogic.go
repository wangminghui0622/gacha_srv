package dig

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListDigBoxesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListDigBoxesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDigBoxesLogic {
	return &ListDigBoxesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListDigBoxes 挖宝箱列表（M11b）
func (l *ListDigBoxesLogic) ListDigBoxes() (resp *types.DigBoxListResp, err error) {
	if _, ok := middleware.UserIdFromCtx(l.ctx); !ok {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.ListDigBoxes(l.ctx, &playClient.ListDigBoxesReq{})
	if err != nil {
		l.Errorf("ListDigBoxes failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	resp = &types.DigBoxListResp{Items: make([]types.DigBoxItem, 0, len(r.Items))}
	for _, it := range r.Items {
		resp.Items = append(resp.Items, types.DigBoxItem{
			BoxId:     it.BoxId,
			Name:      it.Name,
			Price:     it.Price,
			CellCount: it.CellCount,
		})
	}
	return resp, nil
}
