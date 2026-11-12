package playkit

import (
	"context"
	"errors"
	"fmt"

	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Quote 报价快照（各玩法 lock 只需要这三个字段）。
type Quote struct {
	UserID   int64
	GrossFen int64
	BoxID    int64
}

// LoadQuote 读报价（营销 GetQuote）并把业务错误统一映射为 Aborted。
//
// 所有玩法的 lock 分支都做同一件事：quoteToken → userId/grossFen/boxId；
// quote 不存在 / 业务拒绝 → Aborted（触发 DTM 补偿）；系统错误原样返回（DTM 重试）。
func LoadQuote(ctx context.Context, svcCtx *svc.ServiceContext, quoteToken string) (*Quote, error) {
	resp, err := svcCtx.MarketingRpc.GetQuote(ctx, &marketingClient.GetQuoteReq{QuoteToken: quoteToken})
	if err != nil {
		return nil, mapQuoteErr(err)
	}
	return &Quote{
		UserID:   resp.GetUserId(),
		GrossFen: resp.GetGrossFen(),
		BoxID:    resp.GetBoxId(),
	}, nil
}

// LockQuote 锁报价（营销 LockQuote）；newLocked=false 表示该 requestId 已锁过。
//
// 调用方约定：玩法自身校验全部通过后，最后才锁报价（营销事务独立提交，
// 无法随玩法事务回滚；先锁会让玩法校验失败时留下 OPEN 报价）。
func LockQuote(ctx context.Context, svcCtx *svc.ServiceContext, requestID, quoteToken string) (bool, error) {
	resp, err := svcCtx.MarketingRpc.LockQuote(ctx, &marketingClient.LockQuoteReq{
		RequestId:  requestID,
		QuoteToken: quoteToken,
	})
	if err != nil {
		return false, mapQuoteErr(err)
	}
	return resp.GetNewLocked(), nil
}

// mapQuoteErr 业务错误 → gRPC Aborted（保留 code/msg）；系统错误原样返回。
func mapQuoteErr(err error) error {
	var ce *errorx.CodeError
	if errors.As(err, &ce) {
		return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
	}
	return err
}
