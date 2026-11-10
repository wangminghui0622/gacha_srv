package logic

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/core"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/app/dtm/client/dtmgrpc"
	drawmodel "gacha_srv/pkg/model/gacha_draw"
	drawmodelnc "gacha_srv/pkg/model_nocache/gacha_draw"
	"gacha_srv/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// errRecordDup 事务内哨兵：同 requestId 的抽赏记录已存在（另一 gid 已成功开奖）
var errRecordDup = errors.New("draw record duplicated")

type SettleDrawLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleDrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleDrawLogic {
	return &SettleDrawLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 10/30 完善：抽赏记录新增 EntryId（=中奖条目 id），补偿按 id 精确定位库存，避免同名条目还错行。

// SettleDraw SAGA 正向分支：按公示概率开奖 + 扣条目库存 + 落记录 + 中奖入吉仓。
//
// 错误语义（SAGA 分支铁律）：
//   - 业务失败（宝箱不存在/下架/售罄、参数无效）→ codes.Aborted，DTM 触发退款补偿
//   - 系统错误 → codes.Internal，DTM 退避重试
//
// 缓存一致性（条目写路径）：FOR UPDATE 行锁读刻意不走缓存模型
// （锁必须作用在事务连接上，缓存读方法无 tx 参数）；
// 写 = UpdateByKeys（Save 走 tx + 删条目键与 box 列表键）→ commit → 延迟双删，
// 兜底 3 分钟短 TTL。
func (l *SettleDrawLogic) SettleDraw(in *draw.SettleDrawReq) (*draw.SettleDrawResp, error) {
	if in.BoxId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=宝箱ID无效")
	}
	if in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=用户ID无效")
	}
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=requestId不能为空")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleDraw get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleDraw get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var (
		out struct {
			recordId  int64
			prizeName string
			isPrize   bool
		}
		businessRan bool
		winnerEntry *drawmodel.DrawPoolEntries // commit 后延迟双删用
	)
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① FOR UPDATE 锁宝箱行：校验存在与上架；价格取锁行（不信外部传参）
		var b drawmodel.DrawBoxes
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", in.BoxId).First(&b).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, "code=3001 msg=宝箱不存在")
			}
			return err
		}
		if b.Status != 1 {
			return status.Error(codes.Aborted, "code=3003 msg=宝箱已下架")
		}
		// 实付价格：M8 网关传箱价，M10 起传报价实付（券/积分抵扣后）；缺省回退锁行价格。
		// price=0 合法：券全抵/免单券（M10 免单订单实付为 0）
		priceFen := b.Price
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		// ② M12 滚动剔除：滚动盒（rolling_rules_json 非空）解析规则，
		//    无效 → Aborted 2005；本仓 SettleDraw 恒单抽，单抽剔除按开关生效
		singlePercent := 0
		rollingBox := b.RollingRulesJson != nil && *b.RollingRulesJson != ""
		var rules *core.RollingRules
		if rollingBox {
			var err error
			rules, err = core.ParseRollingRules(*b.RollingRulesJson)
			if err != nil {
				return status.Error(codes.Aborted, "code=3005 msg="+err.Error())
			}
			if rules.SingleDrawExclusionEnabled {
				singlePercent = rules.SingleDrawExcludedPercent
			}
		}
		// ③ FOR UPDATE 锁本箱全部条目：并发开奖串行化，remaining 读改原子
		var entries []drawmodel.DrawPoolEntries
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("box_id = ?", in.BoxId).Order("id ASC").Find(&entries).Error; err != nil {
			return err
		}

		// 滚动剔除集合：限量条目按 id 升序取前缀的并集（entries 已按 id ASC 锁定读）
		var rollingExcluded map[int64]struct{}
		if rollingBox {
			rollingExcluded = core.ExcludeRollingSet(rollingEntries(entries), singlePercent, rules.UserExcludedPercent)
		}
		// ④ 可抽条目：remaining>0（quantity=-1 的谢谢参与天然满足）；
		//    被滚动剔除的条目不参与本次抽取；可抽中奖条目必须存在，
		//    否则整箱售罄（含全剔除）→ Aborted 退款
		drawable := make([]core.PrizeEntry, 0, len(entries))
		prizeDrawable := false
		for i := range entries {
			e := &entries[i]
			if e.Remaining == 0 {
				continue // 售罄条目不参与抽取
			}
			if _, ok := rollingExcluded[e.Id]; ok {
				continue // 滚动剔除的限量条目不参与本次抽取
			}
			drawable = append(drawable, core.PrizeEntry{Id: e.Id, Name: e.Name, Weight: int(e.Weight)})
			if e.IsPrize == 1 {
				prizeDrawable = true
			}
		}
		if !prizeDrawable {
			return status.Error(codes.Aborted, "code=3002 msg=宝箱已售罄")
		}

		// ⑤ 确定性开奖（SHA256 拒绝采样 + 审计哈希，字节级对齐 server_v2 Rust 引擎）。
		// seed 用 crypto/rand 生成 32 字节，audit_hash 覆盖 seed/奖池/结果，可离线复算核对。
		seed := make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			return err
		}
		detEntries := make([]core.DeterministicEntry, 0, len(drawable))
		for i := range drawable {
			d := &drawable[i]
			var rem int64
			for j := range entries {
				if entries[j].Id == d.Id {
					rem = entries[j].Remaining
					break
				}
			}
			detEntries = append(detEntries, core.DeterministicEntry{
				EntryID:   uint64(d.Id),
				Weight:    uint64(d.Weight),
				Remaining: uint32(rem), // -1 → 0xFFFFFFFF（谢谢参与不限量）
			})
		}
		outcomes, auditHex, derr := core.DrawDeterministic(detEntries, seed, uint64(b.Id), 1, in.RequestId, 1, false)
		if derr != nil {
			return status.Error(codes.Internal, fmt.Sprintf("code=500 msg=抽取失败: %v", derr))
		}
		pickedID := int64(outcomes[0].EntryID)
		var winner *drawmodel.DrawPoolEntries
		for i := range entries {
			if entries[i].Id == pickedID {
				winner = &entries[i]
				break
			}
		}
		if winner == nil {
			return status.Error(codes.Internal, "抽取失败：中选条目不在奖池")
		}
		seedHex := hex.EncodeToString(seed)

		// ⑥ 扣库存：行已锁，remaining>=1 必然成立（保留检查属防御性编程）；
		//    谢谢参与（quantity=-1）不扣。UpdateByKeys = Save 走 tx + 删缓存键。
		if winner.IsPrize == 1 && winner.Quantity != -1 {
			if winner.Remaining < 1 {
				return status.Error(codes.Aborted, "code=3002 msg=宝箱已售罄")
			}
			winner.Remaining -= 1
			winner.UpdatedAt = time.Now()
			if err := l.svcCtx.EntryModel.UpdateByKeys(l.ctx, db, winner); err != nil {
				return err
			}
			winnerEntry = winner
		}

		// ⑦ 插抽赏记录（uk_request_id 幂等：同 requestId 撞键 = 已开奖）；
		//    status=1 正常（SAGA 补偿置 2 已撤销）
		rec := &drawmodelnc.DrawRecords{
			RequestId: in.RequestId,
			UserId:    in.UserId,
			BoxId:     b.Id,
			EntryName: winner.Name,
			EntryId:   winner.Id,
			IsPrize:   winner.IsPrize,
			Price:     priceFen,
			SeedHex:   seedHex,
			AuditHash: auditHex,
			Status:    recordStatusNormal,
			CreatedAt: time.Now(),
		}
		if err := l.svcCtx.RecordModel.Insert(l.ctx, db, rec); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errRecordDup
			}
			return err
		}

		// ⑧ 中奖入吉仓（谢谢参与只记录、不落仓）
		if winner.IsPrize == 1 {
			if err := l.svcCtx.WarehouseModel.Insert(l.ctx, db, &drawmodelnc.DrawWarehouseItems{
				UserId:    in.UserId,
				RecordId:  rec.Id,
				BoxId:     b.Id,
				ItemName:  winner.Name,
				Status:    1,
				CreatedAt: time.Now(),
			}); err != nil {
				return err
			}
		}
		out.recordId = rec.Id
		out.prizeName = winner.Name
		out.isPrize = winner.IsPrize == 1
		return nil
	})
	if errors.Is(err, errRecordDup) {
		// 同 requestId 已开奖（另一 gid 已成功）：返回已有记录，幂等成功
		return l.readBackRecord(in.RequestId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleDraw aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleDraw failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
		return l.readBackRecord(in.RequestId)
	}

	// ⑨ commit 后延迟双删：清掉「DEL 后 commit 前」窗口期被并发读回填的旧值。
	// best-effort：失败只记日志，兜底 3 分钟 TTL。
	if winnerEntry != nil {
		threading.GoSafe(func() {
			time.Sleep(500 * time.Millisecond)
			dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if e := l.svcCtx.EntryModel.DeleteCache(dctx, winnerEntry); e != nil {
				l.Errorf("SettleDraw delayed cache delete failed, entryId: %d, err: %v", winnerEntry.Id, e)
			}
		})
	}
	l.Infof("SettleDraw success, requestId: %s, prize: %s, isPrize: %v, recordId: %d",
		in.RequestId, out.prizeName, out.isPrize, out.recordId)
	return &draw.SettleDrawResp{
		Success:   true,
		PrizeName: out.prizeName,
		IsPrize:   out.isPrize,
		RecordId:  out.recordId,
	}, nil
}

// readBackRecord 幂等/屏障跳过路径：读已有记录如实返回
func (l *SettleDrawLogic) readBackRecord(requestId string) (*draw.SettleDrawResp, error) {
	rec, err := l.svcCtx.RecordModel.FindOneByRequestId(l.ctx, requestId)
	if err != nil {
		if errors.Is(err, drawmodelnc.ErrNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "抽赏记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackRecord: requestId: %s, prize: %s, recordId: %d", requestId, rec.EntryName, rec.Id)
	return &draw.SettleDrawResp{
		Success:   true,
		PrizeName: rec.EntryName,
		IsPrize:   rec.IsPrize == 1,
		RecordId:  rec.Id,
	}, nil
}

// rollingEntries 模型条目 → 滚动判定条目（调用方保证按 entry id 升序传入）
func rollingEntries(entries []drawmodel.DrawPoolEntries) []core.RollingEntry {
	out := make([]core.RollingEntry, 0, len(entries))
	for i := range entries {
		e := &entries[i]
		out = append(out, core.RollingEntry{
			Id:         e.Id,
			Limited:    e.RollingLimited,
			DisplayVal: e.DisplayValueFen,
		})
	}
	return out
}
