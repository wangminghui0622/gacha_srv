package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type LockQuoteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockQuoteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockQuoteLogic {
	return &LockQuoteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockQuote SAGA 正向分支 03：只做状态迁移（金额在 QuoteCheckout 已算定）。
// barrier 事务内 FOR UPDATE 锁报价行 + 券行（防并发重复锁）：
//
//	quote OPEN→LOCKED；券 AVAILABLE→LOCKED（locked_by=quote_token）。
//
// 幂等：已 LOCKED 直接成功；券已被本报价锁定亦继续。
//
// 玩法购买 SAGA 的 03 分支（play.Lock<玩法>）经本 RPC sub-call 锁报价：
// sub-call 无 DTM trans info，退化为普通事务（业务幂等兜底）；
// 抽盒 SAGA 由 DTM 直调本 RPC，带 trans info 走 barrier。两种形态共用同一业务闭包。
// 响应回传 newLocked/userId/grossFen/boxId 供玩法锁内校验与幂等判断。
//
// 错误语义：业务失败（过期/状态非法/券被占）→ codes.Aborted（触发补偿）；
//
//	系统错误 → codes.Internal（DTM 重试）。
func (l *LockQuoteLogic) LockQuote(in *marketing.LockQuoteReq) (*marketing.LockQuoteResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 双调用形态：SAGA 分支 03 被 DTM 直调时带 trans info 走 barrier；
	// play.Lock<玩法> 分支内 sub-call 无 trans info，退化为普通事务
	//（幂等由 acquireQuoteLock 的 FOR UPDATE + 状态校验兜底）。
	barrier, err := dtmgrpc.BarrierFromGrpcOrNil(l.ctx)
	if err != nil {
		l.Errorf("LockQuote get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	newLocked := false
	busi := func(db *gorm.DB) error {
		businessRan = true
		// 报价+券锁（共享 helper，LockDigBox 同源）：alreadyLocked=幂等重发
		_, alreadyLocked, err := acquireQuoteLock(db, in.QuoteToken)
		if err != nil {
			return err
		}
		if alreadyLocked {
			l.Infof("LockQuote idempotent: already locked, quoteToken: %s", in.QuoteToken)
			return nil
		}

		// 报价 OPEN→LOCKED（盲更新防并发双锁；0 行=并发竞争，Internal 触发 DTM 重试后幂等收敛）
		result := db.Model(&gamemodelnc.QuoteLocks{}).Where("quote_token = ? AND status = ?", in.QuoteToken, quoteOpen).Update("status", quoteLocked)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return status.Error(codes.Internal, "报价锁定竞争，请重试")
		}
		newLocked = true
		return nil
	}
	if barrier != nil {
		rawDB, err := l.svcCtx.DB.DB()
		if err != nil {
			l.Errorf("LockQuote get raw db failed, err: %v", err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
			return busi(txutil.WithTx(l.ctx, l.svcCtx.DB, tx))
		})
	} else {
		err = l.svcCtx.DB.Transaction(busi)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockQuote aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockQuote failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（正向已执行过）：读库如实返回
		return l.readBackQuote(in.QuoteToken)
	}
	l.Infof("LockQuote success, quoteToken: %s, newLocked: %v", in.QuoteToken, newLocked)
	return l.fillQuoteFields(in.QuoteToken, newLocked)
}

// fillQuoteFields 读报价回填玩法所需字段（userId/grossFen/boxId）
func (l *LockQuoteLogic) fillQuoteFields(quoteToken string, newLocked bool) (*marketing.LockQuoteResp, error) {
	q, err := l.svcCtx.QuoteModel.FindOneByQuoteToken(l.ctx, quoteToken)
	if err != nil {
		l.Errorf("fillQuoteFields failed, quoteToken: %s, err: %v", quoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &marketing.LockQuoteResp{
		QuoteToken: quoteToken,
		Status:     quoteStatusName(q.Status),
		NewLocked:  newLocked,
		UserId:     q.UserId,
		GrossFen:   q.GrossFen,
		BoxId:      q.BoxId,
	}, nil
}

// readBackQuote 屏障跳过时读库如实返回当前状态
func (l *LockQuoteLogic) readBackQuote(quoteToken string) (*marketing.LockQuoteResp, error) {
	q, err := l.svcCtx.QuoteModel.FindOneByQuoteToken(l.ctx, quoteToken)
	if err != nil {
		if errors.Is(err, gamemodelnc.ErrNotFound) {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价不存在", errorx.CodeQuoteNotFound))
		}
		l.Errorf("readBackQuote failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if q.Status != quoteLocked && q.Status != quoteUsed {
		return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价状态不满足锁定", errorx.CodeQuoteStateInvalid))
	}
	l.Infof("readBackQuote: status %d, quoteToken: %s", q.Status, quoteToken)
	return &marketing.LockQuoteResp{
		QuoteToken: quoteToken,
		Status:     quoteStatusName(q.Status),
		NewLocked:  false,
		UserId:     q.UserId,
		GrossFen:   q.GrossFen,
		BoxId:      q.BoxId,
	}, nil
}
