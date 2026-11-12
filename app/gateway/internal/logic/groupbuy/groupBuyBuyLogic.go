package groupbuy

import (
	"context"

	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GroupBuyBuyLogic struct {
	logx.Logger
	ctx       context.Context
	svcCtx    *svc.ServiceContext
	requestId string
}

func NewGroupBuyBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GroupBuyBuyLogic {
	return &GroupBuyBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *GroupBuyBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// GroupBuyBuy 拼团抽下注：5 分支 SAGA，玩法差异见 purchaseMode。
func (l *GroupBuyBuyLogic) GroupBuyBuy(req *types.GroupBuyBuyReq) (*types.GroupBuyBuyResp, error) {
	if req.Count <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "下注数非法")
	}
	gid, userID, alreadyDone, err := saga.RunBuy(l.ctx, l.Logger, l.svcCtx, l.requestId, req,
		purchaseMode{log: l.Logger})
	if err != nil {
		return nil, err
	}
	status := "pending"
	if alreadyDone {
		status = "success"
	}
	l.Infof("group buy submitted, gid: %s, configId: %d, userId: %d, count: %d, status: %s",
		gid, req.ConfigId, userID, req.Count, status)
	return &types.GroupBuyBuyResp{RequestId: gid, Status: status}, nil
}
