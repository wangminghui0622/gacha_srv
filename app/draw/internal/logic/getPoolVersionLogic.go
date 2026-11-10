package logic

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPoolVersionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPoolVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPoolVersionLogic {
	return &GetPoolVersionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 赏池版本详情（server_v2 GetPoolVersion：按 (pool_id, version) 读取，
// 含草稿与已发布；行状态覆盖密封值）。
func (l *GetPoolVersionLogic) GetPoolVersion(in *draw.GetPoolVersionReq) (*draw.BoxDraft, error) {
	if in.PoolId <= 0 || in.Version <= 0 {
		return nil, poolErrNotFound()
	}
	return poolGetVersion(l.svcCtx.DB, in.PoolId, in.Version)
}
