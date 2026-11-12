package gameimpl

import (
	"context"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewGrantMemoryMatchPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantMemoryMatchPrizesLogic {
	return &GrantMemoryMatchPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantMemoryMatchPrizes SAGA 06 正向：空操作。
// 记忆翻牌购买时不发奖——奖品在终局结算事务内同步发放（grant_no="mmr:<sessionID>" 幂等补发，
// 见 mmRetryPendingSettlementGrant），06 分支仅为 SAGA 骨架占位。永不 Aborted。
func (l *GrantMemoryMatchPrizesLogic) GrantMemoryMatchPrizes(in *play.GrantMemoryMatchPrizesReq) (*play.GrantMemoryMatchPrizesResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}
	l.Infof("GrantMemoryMatchPrizes no-op, requestId: %s, sessionId: %d", in.RequestId, in.SessionId)
	return &play.GrantMemoryMatchPrizesResp{Status: "NO_PRIZES_AT_PURCHASE"}, nil
}

func NewGrantMemoryMatchPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantMemoryMatchPrizesRevertLogic {
	return &GrantMemoryMatchPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantMemoryMatchPrizesRevert SAGA 06 补偿：空操作（正向即空操作，无奖可撤）。永不 Aborted。
func (l *GrantMemoryMatchPrizesRevertLogic) GrantMemoryMatchPrizesRevert(in *play.GrantMemoryMatchPrizesReq) (*play.GrantMemoryMatchPrizesResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}
	l.Infof("GrantMemoryMatchPrizesRevert no-op, requestId: %s, sessionId: %d", in.RequestId, in.SessionId)
	return &play.GrantMemoryMatchPrizesResp{Status: "NO_PRIZES_AT_PURCHASE"}, nil
}

// mmGrantOne 单奖幂等发放（grant_no 全 uk 幂等；统一走 playkit.GrantOne）
func mmGrantOne(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, grantNo string, prizeKind, cashFen, points int64, couponTemplateNo string) error {
	return playkit.GrantOne(ctx, svcCtx, userID, grantNo, playkit.Prize{
		Kind:             prizeKind,
		CashFen:          cashFen,
		Points:           points,
		CouponTemplateNo: couponTemplateNo,
	})
}
