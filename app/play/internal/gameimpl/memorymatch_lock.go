package gameimpl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

	var outSessionID int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockMemoryMatch", func(db *gorm.DB) error {

		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID
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

		if quote.GrossFen != cfg.PurchasePriceMinor || in.TotalPriceFen != cfg.PurchasePriceMinor {
			return status.Error(codes.Aborted, "code=400 msg=购买价格校验不一致")
		}
		if quote.BoxID != 0 && quote.BoxID != in.ConfigId {
			return status.Error(codes.Aborted, "code=400 msg=报价与玩法不匹配")
		}

		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockMemoryMatch idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			outSessionID = 0
			return nil
		}

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

func NewLockMemoryMatchRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockMemoryMatchRevertLogic {
	return &LockMemoryMatchRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockMemoryMatchRevert SAGA 03 补偿：会话 PENDING/AWAITING_DESTINY → EXPIRED（盲撤销幂等）；
// 已开始游玩（AWAITING_SECOND/PLAYING/COMPLETED/EXPIRED）→ 跳过不打扰。报价释放由 payment 04 补偿负责。
func (l *LockMemoryMatchRevertLogic) LockMemoryMatchRevert(in *play.LockMemoryMatchReq) (*play.LockMemoryMatchResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outSessionID int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "lockMemoryMatchRevert", func(db *gorm.DB) error {

		var sess gamemodelnc.MemoryMatchSessions
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&sess).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		outSessionID = sess.Id
		if sess.Status == mmSessionPending || sess.Status == mmSessionAwaitingDestiny {
			if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).
				Where("id = ? AND status = ?", sess.Id, sess.Status).
				Updates(map[string]interface{}{
					"status":     mmSessionExpired,
					"version":    gorm.Expr("version + 1"),
					"updated_at": time.Now(),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockMemoryMatchRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("LockMemoryMatchRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan || outSessionID == 0 {
		var sess gamemodelnc.MemoryMatchSessions
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&sess).Error; err == nil {
			outSessionID = sess.Id
		}
	}
	l.Infof("LockMemoryMatchRevert success, requestId: %s, sessionId: %d", in.RequestId, outSessionID)
	return &play.LockMemoryMatchResp{SessionId: outSessionID, Status: "RELEASED"}, nil
}
