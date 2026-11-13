package gameimpl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
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

	var outRecordId int64
	outEntry := ""
	outIsPrize := false
	outDeparted := false
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleFullCar", func(db *gorm.DB) error {

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

		seats := fullCarParseSeats(st.SeatsJson)
		seat := in.SeatIndex
		if seat < 0 {

			seat = -1
			for i, u := range seats {
				if u == in.UserId {
					seat = int64(i)
					break
				}
			}
		}
		if seat < 0 || seat >= int64(len(seats)) || seats[seat] != in.UserId {
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

		picked := core.PickPrizeDeterministic(pool, []byte(in.RequestId), "full-car-regular", 0)
		if picked == nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车奖池已售罄", errorx.CodeGameConfigInvalid))
		}

		if err := db.Model(&gamemodel.FullCarEntries{}).
			Where("id = ? AND quantity != -1 AND remaining > 0", picked.Id).
			Update("remaining", gorm.Expr("remaining - 1")).Error; err != nil {
			return err
		}

		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		var pickedSku int64
		for _, e := range entries {
			if e.Id == picked.Id {
				pickedSku = e.SkuId
				break
			}
		}
		rec := &gamemodelnc.FullCarRecords{
			RequestId: in.RequestId,
			UserId:    in.UserId,
			ConfigId:  in.ConfigId,
			SeatIndex: seat,
			EntryName: picked.Name,
			SkuId:     pickedSku,
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

func NewSettleFullCarRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleFullCarRevertLogic {
	return &SettleFullCarRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleFullCarRevert SAGA 05 补偿：只撤销本分支造成的影响（dig 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② regular 奖库存回补（按记录的 entry_name 定位，quantity=-1 不限量跳过）；
//	③ awards_json 按 requestId 删除本 gid 触发的整车奖；DEPARTED→OPEN。
//
// 座位释放与余票+1 由 03 补偿（ReleaseFullCar）唯一负责，本分支不碰余票。
func (l *SettleFullCarRevertLogic) SettleFullCarRevert(in *play.SettleFullCarReq) (*play.SettleFullCarResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleFullCarRevert", func(db *gorm.DB) error {

		// ① 记录 FOR UPDATE → 2 已撤销（盲更新，幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.FullCarRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == fullCarRecordNormal {
				if err := db.Model(&gamemodelnc.FullCarRecords{}).
					Where("id = ? AND status = ?", rec.Id, fullCarRecordNormal).
					Update("status", fullCarRecordCanceled).Error; err != nil {
					return err
				}
			}

			if err := db.Model(&gamemodel.FullCarEntries{}).
				Where("config_id = ? AND name = ? AND quantity != -1", rec.ConfigId, rec.EntryName).
				Update("remaining", gorm.Expr("remaining + 1")).Error; err != nil {
				return err
			}
		}

		// ③ awards 按 requestId 删除 + DEPARTED→OPEN
		var st gamemodelnc.FullCarState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			all := fullCarParseAwards(st.AwardsJson)
			kept := make([]fullCarStoredAward, 0, len(all))
			removed := 0
			for _, a := range all {
				if a.RequestId == in.RequestId {
					removed++
					continue
				}
				kept = append(kept, a)
			}
			updates := map[string]interface{}{"updated_at": time.Now()}
			if removed > 0 {
				updates["awards_json"] = fullCarAwardsJSON(kept)
			}
			if st.Status == fullCarStateDeparted {
				updates["status"] = fullCarStateOpen
			}
			if len(updates) > 1 {
				if err := db.Model(&gamemodelnc.FullCarState{}).Where("id = ?", st.Id).
					Updates(updates).Error; err != nil {
					return err
				}
			}
			if removed > 0 {
				l.Infof("SettleFullCarRevert awards removed, requestId: %s, removed: %d", in.RequestId, removed)
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleFullCarRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleFullCarRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.FullCarRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleFullCarRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleFullCarResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}
