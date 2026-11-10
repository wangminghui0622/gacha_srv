package campaign

import (
	"context"

	marketingClient "gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CampaignParticipateLogic struct {
	logx.Logger
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	idemKey string // 客户端 Idempotency-Key 头（强制：缺失/超长 400）
}

func NewCampaignParticipateLogic(ctx context.Context, svcCtx *svc.ServiceContext, idemKey string) *CampaignParticipateLogic {
	return &CampaignParticipateLogic{
		Logger:  logx.WithContext(ctx),
		ctx:     ctx,
		svcCtx:  svcCtx,
		idemKey: idemKey,
	}
}

// CampaignParticipate 统一参与（签到/兑换码/转盘）：强制 Idempotency-Key，
// 网络重试同 key 重放返回首次结果（发奖幂等补发 PENDING）。
func (l *CampaignParticipateLogic) CampaignParticipate(req *types.CampaignParticipateReq) (resp *types.CampaignParticipateResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	if l.idemKey == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "缺少 Idempotency-Key 请求头")
	}
	if len(l.idemKey) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "Idempotency-Key 超长（最多64字符）")
	}
	r, err := l.svcCtx.MarketingRpc.CampaignParticipate(l.ctx, &marketingClient.CampaignParticipateReq{
		UserId:      userId,
		RequestId:   l.idemKey,
		CampaignKey: req.CampaignKey,
		Code:        req.Code,
	})
	if err != nil {
		l.Errorf("CampaignParticipate failed, campaignKey: %s, err: %v", req.CampaignKey, err)
		return nil, errorx.FromError(err)
	}
	return &types.CampaignParticipateResp{
		RequestId: r.RequestId,
		Replayed:  r.Replayed,
		Streak:    r.Streak,
		SpinsLeft: r.SpinsLeft,
		Prizes:    prizeItemsOf(r.Prizes),
	}, nil
}
