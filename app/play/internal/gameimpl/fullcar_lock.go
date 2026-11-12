package gameimpl

import (
	"context"
	"errors"
	"fmt"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/core"
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

func NewLockFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockFullCarLogic {
	return &LockFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockFullCar SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen 取报价行）；
//	② state FOR UPDATE（uk config_id 串行化同车购买；种子缺失防御建行）；
//	③ 座位分配（显式 seatIndex 或首个空位；越界 3023 / 已占 3022 / 满车 3021）；
//	④ 价格一致（3018 复用：quote.gross == 单价×1）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ 占座 + 余票-1。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockFullCarLogic) LockFullCar(in *play.LockFullCarReq) (*play.LockFullCarResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	assignedSeat := int32(-1)
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockFullCar", func(db *gorm.DB) error {

		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

		// 满车配置（纯种子配置只读，不走缓存锁）
		var cfg gamemodel.FullCarConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 || cfg.SeatCount < 2 || cfg.SeatCount > 1000 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车配置不可用", errorx.CodeGameConfigInvalid))
		}

		// ② state FOR UPDATE（uk(config_id) 串行化同车并发购买）
		var st gamemodelnc.FullCarState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {

			st = gamemodelnc.FullCarState{
				ConfigId:         in.ConfigId,
				SeatsJson:        fullCarMarshalSeats(make([]int64, cfg.SeatCount)),
				AwardsJson:       "[]",
				RemainingTickets: cfg.SeatCount,
				Status:           fullCarStateOpen,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			if err := db.Create(&st).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {

					if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
		} else if err != nil {
			return err
		}
		if st.Status != fullCarStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车状态不满足锁定", errorx.CodeFullCarStateInvalid))
		}
		if st.RemainingTickets <= 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车已出发", errorx.CodeFullCarDeparted))
		}
		seats := fullCarParseSeats(st.SeatsJson)
		if len(seats) != int(cfg.SeatCount) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车座位数据异常", errorx.CodeGameConfigInvalid))
		}

		requested := in.SeatIndex
		if requested >= cfg.SeatCount {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=座位越界", errorx.CodeFullCarSeatInvalid))
		}

		if core.FullCarContainsUser(seats, userId) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=同一用户已在车内，不可重复占座", errorx.CodeFullCarSeatOccupied))
		}
		var seat int32
		if requested >= 0 {
			if seats[requested] != 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=座位已被占用", errorx.CodeFullCarSeatOccupied))
			}
			seat = int32(requested)
		} else {
			seat = -1
			for i, u := range seats {
				if u == 0 {
					seat = int32(i)
					break
				}
			}
			if seat < 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车已出发", errorx.CodeFullCarDeparted))
			}
		}

		if quote.GrossFen != cfg.PriceFen || in.TotalPriceFen != cfg.PriceFen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockFullCar idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			assignedSeat = l.seatOfUser(db, in.ConfigId, userId)
			return nil
		}

		seats[seat] = userId
		if err := db.Model(&gamemodelnc.FullCarState{}).Where("id = ?", st.Id).
			Updates(map[string]interface{}{
				"seats_json":        fullCarMarshalSeats(seats),
				"remaining_tickets": st.RemainingTickets - 1,
				"status":            fullCarStateOpen,
				"updated_at":        time.Now(),
			}).Error; err != nil {
			return err
		}
		assignedSeat = seat
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockFullCar aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockFullCar failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {

		return l.readBackLock(in.ConfigId, in.QuoteToken)
	}
	l.Infof("LockFullCar success, quoteToken: %s, seat: %d", in.QuoteToken, assignedSeat)
	return &play.LockFullCarResp{SeatIndex: int64(assignedSeat), Status: "LOCKED"}, nil
}

// readBackLock 屏障跳过时读库如实返回（报价经营销 RPC 读 userId）
func (l *LockFullCarLogic) readBackLock(configId int64, quoteToken string) (*play.LockFullCarResp, error) {
	q, err := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: quoteToken})
	if err != nil {
		var ce *errorx.CodeError
		if errors.As(err, &ce) && ce.Code == errorx.CodeQuoteNotFound {
			return nil, status.Error(codes.Internal, "报价不存在")
		}
		l.Errorf("readBackLock failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	seat := l.seatOfUser(l.svcCtx.DB, configId, q.GetUserId())
	l.Infof("readBackLock: configId: %d, seat: %d", configId, seat)
	return &play.LockFullCarResp{SeatIndex: int64(seat), Status: "LOCKED"}, nil
}
