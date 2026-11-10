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

type GetDigBoxSessionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDigBoxSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDigBoxSessionLogic {
	return &GetDigBoxSessionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetDigBoxSession 挖宝会话（位图/已挖数/状态；无会话 hasSession=false）
func (l *GetDigBoxSessionLogic) GetDigBoxSession(req *types.DigSessionReq) (resp *types.DigBoxSessionResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.GetDigBoxSession(l.ctx, &playClient.GetDigBoxSessionReq{
		UserId: userId,
		BoxId:  req.BoxId,
	})
	if err != nil {
		l.Errorf("GetDigBoxSession failed, boxId: %d, err: %v", req.BoxId, err)
		return nil, errorx.FromError(err)
	}
	return &types.DigBoxSessionResp{
		HasSession:     r.HasSession,
		SessionNo:      r.SessionNo,
		OccupiedBitmap: r.OccupiedBitmap,
		TotalCellsPaid: r.TotalCellsPaid,
		Status:         r.Status,
		CellCount:      r.CellCount,
	}, nil
}
