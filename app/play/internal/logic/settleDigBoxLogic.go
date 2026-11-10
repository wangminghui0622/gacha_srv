package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
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

// errDigRecordDup 事务内哨兵：同 requestId 的挖宝记录已存在（另一 gid 已结算）
var errDigRecordDup = errors.New("dig record duplicated")

type SettleDigBoxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleDigBoxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleDigBoxLogic {
	return &SettleDigBoxLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleDigBox SAGA 05 正向：barrier 事务内
//
//	① 会话 FOR UPDATE（必须 PLAYING）；② 校验格子已由 03 置位（3017）；
//	③ 揭晓奖品（dig_box_cells 固定奖品，空格跳过）落 prizes_json；
//	④ 插挖宝记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	⑤ 满格（位图全 1）→ 会话 COMPLETED（下次购买回收）。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleDigBoxLogic) SettleDigBox(in *play.SettleDigBoxReq) (*play.SettleDigBoxResp, error) {
	if in.RequestId == "" || in.SessionNo == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleDigBox get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleDigBox get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 会话 FOR UPDATE
		var sess gamemodelnc.DigBoxSessions
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_no = ? AND user_id = ?", in.SessionNo, in.UserId).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝会话不存在", errorx.CodeDigSessionInvalid))
			}
			return err
		}
		if sess.Status != digSessionPlaying {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝会话状态不满足结算", errorx.CodeDigSessionInvalid))
		}

		// ② 格子必须已由 03 置位（防绕过锁定直接结算）；格数上限取配置
		var cfg gamemodel.DigBoxConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝箱不存在", errorx.CodeDigBoxNotFound))
			}
			return err
		}
		if !digCellsValid(in.CellIndexes, cfg.CellCount) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=所选格子无效", errorx.CodeDigCellOccupied))
		}
		mask := digCellMask(in.CellIndexes)
		if sess.OccupiedBitmap&mask != mask {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=会话位图与所选格子不一致", errorx.CodeDigSessionInvalid))
		}

		// ③ 揭晓奖品（格行按 cell_index 排序，与 grants #idx 对齐）
		var cells []*gamemodel.DigBoxCells
		if err := db.Where("config_id = ? AND cell_index IN ?", in.ConfigId, in.CellIndexes).
			Order("cell_index ASC").Find(&cells).Error; err != nil {
			return err
		}
		prizesJSON, _ := digPrizesOfCells(l.ctx, l.svcCtx, cells)

		// ④ 插记录（uk request_id 幂等；撞键 = 已结算读回）
		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.DigBoxRecords{
			RequestId:     in.RequestId,
			UserId:        in.UserId,
			ConfigId:      in.ConfigId,
			SessionNo:     in.SessionNo,
			CellsChosen:   digJoinCells(in.CellIndexes),
			PrizesJson:    prizesJSON,
			TotalPriceFen: priceFen,
			Status:        digRecordNormal,
			CreatedAt:     time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errDigRecordDup
			}
			return err
		}
		outRecordId = rec.Id

		// ⑤ 满格 → COMPLETED（下次购买时回收）
		if sess.OccupiedBitmap == (int64(1)<<cfg.CellCount)-1 {
			if err := db.Model(&gamemodelnc.DigBoxSessions{}).Where("id = ?", sess.Id).
				Updates(map[string]interface{}{"status": digSessionCompleted, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, errDigRecordDup) {
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleDigBox aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleDigBox failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleDigBox success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleDigBoxResp{RecordId: outRecordId, Status: "SETTLED"}, nil
}

// readBackRecord 幂等/屏障跳过路径：读已有记录如实返回
func (l *SettleDigBoxLogic) readBackRecord(requestId string, userId int64) (*play.SettleDigBoxResp, error) {
	var rec gamemodelnc.DigBoxRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "挖宝记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d", requestId, rec.Id)
	return &play.SettleDigBoxResp{RecordId: rec.Id, Status: "SETTLED"}, nil
}
