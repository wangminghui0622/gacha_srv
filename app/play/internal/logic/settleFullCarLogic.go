package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// errFullCarRecordDup 事务内哨兵：同 requestId 的满车记录已存在（另一 gid 已结算）
var errFullCarRecordDup = errors.New("full car record duplicated")

type SettleFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleFullCarLogic {
	return &SettleFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleFullCar SAGA 05 正向：barrier 事务内
//
//	① state FOR UPDATE（必须 OPEN）；② 座位校验（seats[seatIndex]==userId）；
//	③ 加权抽 regular 奖（full_car_entries，移植 draw-rpc core.PickPrize）+ 库存-1；
//	④ 插记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	⑤ 余票==0（最后一票已占满）→ 整车结算：确定性 SettleFullCar（seed=gid），
//	   awards 追加落库（含 requestId）、status→DEPARTED。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleFullCarLogic) SettleFullCar(in *play.SettleFullCarReq) (*play.SettleFullCarResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 || in.SeatIndex < 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 幂等重放快路径：同 requestId 记录已存在 → 读回成功（先于业务校验，M11b 教训）
	var existing gamemodelnc.FullCarRecords
	if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&existing).Error; err == nil {
		return l.readBackRecord(in.RequestId, in.UserId)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("SettleFullCar fast path query failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleFullCar get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleFullCar get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outEntry := ""
	outIsPrize := false
	outDeparted := false
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① state FOR UPDATE
		var st gamemodelnc.FullCarState
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车状态不存在", errorx.CodeFullCarStateInvalid))
			}
			return err
		}
		if st.Status != fullCarStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车状态不满足结算", errorx.CodeFullCarStateInvalid))
		}

		// ② 座位必须已由 03 分配（防绕过锁定直接结算）
		seats := fullCarParseSeats(st.SeatsJson)
		if in.SeatIndex >= int64(len(seats)) || seats[in.SeatIndex] != in.UserId {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车座位与请求不一致", errorx.CodeFullCarStateInvalid))
		}

		// 满车配置（纯种子只读）
		var cfg gamemodel.FullCarConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}

		// ③ 加权抽 regular 奖（remaining=0 售罄不参与；-1 不限量恒可抽）
		var entries []gamemodel.FullCarEntries
		if err := db.Where("config_id = ?", in.ConfigId).Find(&entries).Error; err != nil {
			return err
		}
		pool := make([]core.PrizeEntry, 0, len(entries))
		for _, e := range entries {
			if e.Remaining != 0 {
				pool = append(pool, core.PrizeEntry{Id: e.Id, Name: e.Name, Weight: int(e.Weight)})
			}
		}
		if len(pool) == 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车奖池已售罄", errorx.CodeGameConfigInvalid))
		}
		// 确定性抽取：seed=gid（同一 requestId 必抽同一条目，可回放核对）
		picked := core.PickPrizeDeterministic(pool, []byte(in.RequestId), "full-car-regular", 0)
		if picked == nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车奖池已售罄", errorx.CodeGameConfigInvalid))
		}
		// 库存-1（不限量 quantity=-1 跳过）
		if err := db.Model(&gamemodel.FullCarEntries{}).
			Where("id = ? AND quantity != -1 AND remaining > 0", picked.Id).
			Update("remaining", gorm.Expr("remaining - 1")).Error; err != nil {
			return err
		}

		// ④ 插记录（uk request_id 幂等；撞键 = 已结算读回）
		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.FullCarRecords{
			RequestId: in.RequestId,
			UserId:    in.UserId,
			ConfigId:  in.ConfigId,
			SeatIndex: in.SeatIndex,
			EntryName: picked.Name,
			IsPrize:   isPrizeEntryName(entries, picked.Id),
			PriceFen:  priceFen,
			Status:    fullCarRecordNormal,
			CreatedAt: time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errFullCarRecordDup
			}
			return err
		}
		outRecordId = rec.Id
		outEntry = picked.Name
		outIsPrize = rec.IsPrize == 1

		// ⑤ 最后一票 → 整车结算（确定性 seed=gid，可回放核对）
		if st.RemainingTickets == 0 {
			var prizes []core.FullCarSettlementPrize
			if err := json.Unmarshal([]byte(cfg.SettlementPrizesJson), &prizes); err != nil || len(prizes) == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车整车结算奖配置无效", errorx.CodeGameConfigInvalid))
			}
			awards := core.SettleFullCar(seats, prizes, []byte(in.RequestId))
			stored := fullCarAwardsToStored(awards, in.RequestId)
			newAwards := append(fullCarParseAwards(st.AwardsJson), stored...)
			if err := db.Model(&gamemodelnc.FullCarState{}).Where("id = ?", st.Id).
				Updates(map[string]interface{}{
					"awards_json": fullCarAwardsJSON(newAwards),
					"status":      fullCarStateDeparted,
					"updated_at":  time.Now(),
				}).Error; err != nil {
				return err
			}
			outDeparted = true
			l.Infof("SettleFullCar departed, requestId: %s, awards: %d", in.RequestId, len(stored))
		}
		return nil
	})
	if errors.Is(err, errFullCarRecordDup) {
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleFullCar aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleFullCar failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleFullCar success, requestId: %s, recordId: %d, departed: %v", in.RequestId, outRecordId, outDeparted)
	return &play.SettleFullCarResp{
		RecordId:  outRecordId,
		EntryName: outEntry,
		IsPrize:   outIsPrize,
		Departed:  outDeparted,
		Status:    "SETTLED",
	}, nil
}

// isPrizeEntryName 由条目 id 取 is_prize（picked 只带回 id/name/weight）
func isPrizeEntryName(entries []gamemodel.FullCarEntries, id int64) int64 {
	for _, e := range entries {
		if e.Id == id {
			return e.IsPrize
		}
	}
	return 0
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 当前状态如实返回
func (l *SettleFullCarLogic) readBackRecord(requestId string, userId int64) (*play.SettleFullCarResp, error) {
	var rec gamemodelnc.FullCarRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "满车记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	departed := false
	var st gamemodelnc.FullCarState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		departed = st.Status == fullCarStateDeparted
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, departed: %v", requestId, rec.Id, departed)
	return &play.SettleFullCarResp{
		RecordId:  rec.Id,
		EntryName: rec.EntryName,
		IsPrize:   rec.IsPrize == 1,
		Departed:  departed,
		Status:    "SETTLED",
	}, nil
}
