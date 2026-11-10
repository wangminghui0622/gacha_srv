package logic

import (
	"context"
	"database/sql"
	"errors"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/app/dtm/client/dtmgrpc"
	drawmodel "gacha_srv/pkg/model/gacha_draw"
	drawmodelnc "gacha_srv/pkg/model_nocache/gacha_draw"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 抽赏记录/吉仓状态（M8：SAGA 补偿语义）
const (
	recordStatusNormal   int64 = 1 // 正常
	recordStatusCanceled int64 = 2 // 已撤销（SAGA 补偿）
	warehouseStatusIn    int64 = 1 // 在仓
	warehouseStatusOut   int64 = 0 // 已出仓（撤销）
)

type SettleDrawRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleDrawRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleDrawRevertLogic {
	return &SettleDrawRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 10/30 完善：库存补偿改为按记录 entry_id 精确定位（旧数据无 id 回退按 name）。

// SettleDrawRevert SAGA 补偿分支：撤销开奖副作用（真补偿）。
//
// M8 起本步不再是末步（07 失败会逆序补偿到这里），v1 空补偿前提失效，必须真实撤销：
//
//	① 记录 → 已撤销（status=2）
//	② 吉仓 → 出仓（status=0，列表不再展示）
//	③ 归还库存：限量中奖条目 remaining+1（UpdateByKeys 走 tx + 延迟双删，同正向）
//
// 空补偿/重复补偿：无记录或记录已撤销 → 跳过，如实返回成功让 DTM 收尾。
// 幂等：barrier（uk gid+branch_id+op+barrier_id）+ 状态盲更新（status=1 才动）。
// 补偿错误统一 Internal：DTM 重试补偿（barrier 保证业务至多执行一次）。
func (l *SettleDrawRevertLogic) SettleDrawRevert(in *draw.SettleDrawReq) (*draw.SettleDrawResp, error) {
	if in.BoxId <= 0 || in.UserId <= 0 || in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=补偿参数无效")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleDrawRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleDrawRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var (
		reverted    bool
		businessRan bool
		winnerEntry *drawmodel.DrawPoolEntries // commit 后延迟双删用
	)
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// 定位记录：无记录 = 空补偿（正向从未成功，无副作用可撤）
		var rec drawmodelnc.DrawRecords
		err := db.Where("request_id = ?", in.RequestId).First(&rec).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Infof("empty compensate: record not found, requestId: %s, skip", in.RequestId)
			return nil
		}
		if err != nil {
			return err
		}
		if rec.Status == recordStatusCanceled {
			l.Infof("repeated compensate: record already canceled, requestId: %s, skip", in.RequestId)
			return nil
		}
		if rec.Status != recordStatusNormal {
			// 未知状态不阻断补偿收尾（防御）
			l.Errorf("compensate skip: unexpected record status %d, requestId: %s", rec.Status, in.RequestId)
			return nil
		}

		// ① 记录 → 已撤销（盲更新：仅 status=1 才动）
		if err := db.Model(&drawmodelnc.DrawRecords{}).Where("id = ? AND status = ?", rec.Id, recordStatusNormal).Update("status", recordStatusCanceled).Error; err != nil {
			return err
		}

		// ② 吉仓 → 出仓（中奖才有仓；盲更新仅 status=1 才动）
		if rec.IsPrize == 1 {
			if err := db.Model(&drawmodelnc.DrawWarehouseItems{}).Where("record_id = ? AND status = ?", rec.Id, warehouseStatusIn).Update("status", warehouseStatusOut).Error; err != nil {
				return err
			}
		}

		// ③ 归还库存：优先按中奖条目 id（记录 entry_id）精确定位；旧数据无 entry_id（=0）
		//    回退按 (box_id, name) 兼容。FOR UPDATE 与并发开奖串行化；找不到条目
		//    （配置已改）→ 记日志跳过，不阻断补偿收尾。
		var e drawmodel.DrawPoolEntries
		var findErr error
		if rec.EntryId > 0 {
			findErr = db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", rec.EntryId).First(&e).Error
		} else {
			findErr = db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("box_id = ? AND name = ? AND is_prize = 1", rec.BoxId, rec.EntryName).First(&e).Error
		}
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			l.Errorf("compensate skip stock: entry not found, entryId: %d, boxId: %d, name: %s", rec.EntryId, rec.BoxId, rec.EntryName)
			return nil
		}
		if findErr != nil {
			return findErr
		}
		if e.Quantity == -1 {
			// 不限量：正向未扣库存，无需归还
			return nil
		}
		e.Remaining += 1
		e.UpdatedAt = time.Now()
		if err := l.svcCtx.EntryModel.UpdateByKeys(l.ctx, db, &e); err != nil {
			return err
		}
		winnerEntry = &e
		reverted = true
		return nil
	})
	if err != nil {
		l.Errorf("SettleDrawRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（重复补偿）：如实返回成功
		l.Infof("SettleDrawRevert barrier-skip (repeated compensate), requestId: %s", in.RequestId)
		return &draw.SettleDrawResp{Success: true}, nil
	}

	// commit 后延迟双删（同正向：清「DEL 后 commit 前」窗口期回填的旧值）；
	// best-effort：失败只记日志，兜底 3 分钟 TTL。
	if winnerEntry != nil {
		threading.GoSafe(func() {
			time.Sleep(500 * time.Millisecond)
			dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if e := l.svcCtx.EntryModel.DeleteCache(dctx, winnerEntry); e != nil {
				l.Errorf("SettleDrawRevert delayed cache delete failed, entryId: %d, err: %v", winnerEntry.Id, e)
			}
		})
	}
	l.Infof("SettleDrawRevert success, requestId: %s, reverted: %v", in.RequestId, reverted)
	return &draw.SettleDrawResp{Success: true}, nil
}
