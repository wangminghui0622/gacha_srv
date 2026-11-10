package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"
	"gacha_srv/pkg/utils/txutil"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type LockMemoryMatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockMemoryMatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockMemoryMatchLogic {
	return &LockMemoryMatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockMemoryMatch SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen/boxId 取报价行）；
//	② 配置加载 + 规则快照校验（3020）；
//	③ 外观预设校验（非默认 face 需权益 → 3044）；
//	④ 价格一致（quote.gross == 配置购买价 == 请求总额）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ session 插入 status=PENDING（uk request_id 幂等）。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockMemoryMatchLogic) LockMemoryMatch(in *play.LockMemoryMatchReq) (*play.LockMemoryMatchResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("LockMemoryMatch get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("LockMemoryMatch get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	var outSessionID int64
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 报价只读校验（营销 GetQuote sub-call；quote 不存在 → Aborted）
		quoteResp, err := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{
			QuoteToken: in.QuoteToken,
		})
		if err != nil {
			var ce *errorx.CodeError
			if errors.As(err, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return err
		}
		userId := quoteResp.GetUserId()
		if in.UserId != userId {
			return status.Error(codes.Aborted, "code=400 msg=请求用户与报价不一致")
		}

		// ② 配置 + 规则快照（纯种子配置只读）
		var cfg gamemodel.MemoryMatchConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=玩法不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=玩法不可用", errorx.CodeGameConfigInvalid))
		}
		rules, err := mmConfigToRules(&cfg)
		if err != nil {
			return mmAborted(err)
		}

		// ③ 外观预设校验（0=默认；非默认 face 需权益 → 3044）
		faceID := in.FacePresetId
		if faceID == 0 {
			faceID = cfg.DefaultFacePresetId
		}
		backID := in.BackPresetId
		if backID == 0 {
			backID = cfg.DefaultBackPresetId
		}
		faces, err := mmFacePresets(&cfg)
		if err != nil {
			return mmAborted(err)
		}
		if !memorymatch.PresetExists(faces, int(faceID)) {
			return status.Error(codes.Aborted, "code=400 msg=正面预设不存在")
		}
		backs, err := mmBackPresets(&cfg)
		if err != nil {
			return mmAborted(err)
		}
		if !memorymatch.BackPresetExists(backs, int(backID)) {
			return status.Error(codes.Aborted, "code=400 msg=背面预设不存在")
		}
		if faceID != cfg.DefaultFacePresetId {
			var ent gamemodelnc.MemoryMatchAppearanceEntitlements
			err := db.Where("config_id = ? AND user_id = ? AND preset_id = ?",
				in.ConfigId, userId, faceID).First(&ent).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=外观未拥有", errorx.CodeMemoryMatchAppearanceNotOwned))
			}
			if err != nil {
				return err
			}
		}

		// ④ 价格一致：quote.gross == 配置购买价 == 请求总额；quote.BoxId 绑定配置（M10 报价预留字段）
		if quoteResp.GetGrossFen() != cfg.PurchasePriceMinor || in.TotalPriceFen != cfg.PurchasePriceMinor {
			return status.Error(codes.Aborted, "code=400 msg=购买价格校验不一致")
		}
		if quoteResp.GetBoxId() != 0 && quoteResp.GetBoxId() != in.ConfigId {
			return status.Error(codes.Aborted, "code=400 msg=报价与玩法不匹配")
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，读回）
		lockResp, err := l.svcCtx.MarketingRpc.LockQuote(l.ctx, &marketingClient.LockQuoteReq{
			RequestId:  in.RequestId,
			QuoteToken: in.QuoteToken,
		})
		if err != nil {
			var ce *errorx.CodeError
			if errors.As(err, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return err
		}
		if !lockResp.GetNewLocked() {
			l.Infof("LockMemoryMatch idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			outSessionID = 0
			return nil
		}

		// ⑥ session 插入 PENDING（uk request_id 幂等：撞键 = 重试重放 → 读回）
		appearance := mmAppearance{FacePresetID: faceID, BackPresetID: backID}
		appearanceJSON, _ := json.Marshal(appearance)
		secondEnabled := int64(0)
		if rules.SecondDestinyEnabled {
			secondEnabled = 1
		}
		row := &gamemodelnc.MemoryMatchSessions{
			UserId:               userId,
			ConfigId:             in.ConfigId,
			RequestId:            in.RequestId,
			EngineVersion:        1,
			PurchasedRounds:      rules.RoundsPerPurchase,
			PurchasePriceMinor:   rules.PurchasePriceMinor,
			RulesJson:            mmRulesJSON(rules),
			AppearanceJson:       string(appearanceJSON),
			DestinyPatternId:     0,
			SecondDestinyEnabled: secondEnabled,
			CompletedRounds:      0,
			CurrentRound:         1,
			SkipRequested:        0,
			Status:               mmSessionPending,
			Version:              1,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}
		if err := db.Create(row).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				// 重放：读回已有会话
				var exist gamemodelnc.MemoryMatchSessions
				if rerr := db.Where("request_id = ?", in.RequestId).First(&exist).Error; rerr != nil {
					return rerr
				}
				outSessionID = exist.Id
				return nil
			}
			return err
		}
		outSessionID = row.Id
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockMemoryMatch aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockMemoryMatch failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan || outSessionID == 0 {
		// 屏障级跳过或幂等读回：读库如实返回
		return l.readBackLock(in.RequestId)
	}
	l.Infof("LockMemoryMatch success, requestId: %s, sessionId: %d", in.RequestId, outSessionID)
	return &play.LockMemoryMatchResp{SessionId: outSessionID, Status: "LOCKED"}, nil
}

// readBackLock 屏障跳过/幂等重放时读会话如实返回
func (l *LockMemoryMatchLogic) readBackLock(requestId string) (*play.LockMemoryMatchResp, error) {
	var sess gamemodelnc.MemoryMatchSessions
	if err := l.svcCtx.DB.Where("request_id = ?", requestId).First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.Internal, "会话不存在")
		}
		l.Errorf("readBackLock failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackLock: requestId: %s, sessionId: %d, status: %d", requestId, sess.Id, sess.Status)
	return &play.LockMemoryMatchResp{SessionId: sess.Id, Status: "LOCKED"}, nil
}
