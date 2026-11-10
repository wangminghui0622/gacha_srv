package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetMemoryMatchSessionByRequestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMemoryMatchSessionByRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchSessionByRequestLogic {
	return &GetMemoryMatchSessionByRequestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMemoryMatchSessionByRequest 按 requestId（= DTM gid）查会话：网关购买端点轮询/幂等重放用。
// paidMinor 取报价 payable（券后实付；报价缺失回退会话购买价）。不存在 → 404（网关继续轮询）。
func (l *GetMemoryMatchSessionByRequestLogic) GetMemoryMatchSessionByRequest(in *play.GetMemoryMatchSessionByRequestReq) (*play.GetMemoryMatchSessionByRequestResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var sess gamemodelnc.MemoryMatchSessions
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
		First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeNotFound, "会话不存在")
		}
		l.Errorf("GetMemoryMatchSessionByRequest failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	paidMinor := sess.PurchasePriceMinor
	// 报价 payable（券后实付）经营销 RPC 读（quote_token="q_"+requestId 确定性派生；缺失回退会话购买价）
	if q, qerr := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: "q_" + in.RequestId}); qerr == nil {
		paidMinor = q.GetPayableFen()
	}
	appearance, err := mmParseAppearance(sess.AppearanceJson)
	if err != nil {
		l.Errorf("GetMemoryMatchSessionByRequest parse appearance failed, sessionId: %d, err: %v", sess.Id, err)
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话外观快照异常")
	}
	return &play.GetMemoryMatchSessionByRequestResp{
		SessionId:       sess.Id,
		State:           play.MemoryMatchSessionState(sess.Status),
		PurchasedRounds: sess.PurchasedRounds,
		PaidMinor:       paidMinor,
		FacePresetId:    appearance.FacePresetID,
		BackPresetId:    appearance.BackPresetID,
	}, nil
}
