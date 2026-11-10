// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBoxViewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// GetBoxView 箱子库存与玩法状态（gacha.pool:read；本仓无独立 box-instance，
// 与 GetBox 同样映射 BoxDetail：名称/价格/条目库存/概率公示）。
func NewGetBoxViewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBoxViewLogic {
	return &GetBoxViewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetBoxViewLogic) GetBoxView(req *draw.BoxDetailReq) (*draw.BoxDetailResp, error) {
	return l.svcCtx.DrawRpc.BoxDetail(l.ctx, req)
}
