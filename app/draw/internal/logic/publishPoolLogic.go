package logic

import (
	"bytes"
	"context"
	"errors"
	"time"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/repo"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"
	drawmodel "gacha_srv/pkg/model/gacha_draw"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type PublishPoolLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishPoolLogic {
	return &PublishPoolLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PublishPool 发布（server_v2 PublishPool 语义）：
// 版本 DRAFT + hash 一致 + 审批行 APPROVED 且 approver≠requester → 单事务内
// 物化线上表（draw_boxes upsert + draw_pool_entries 整体替换）→ CAS 标记 PUBLISHED。
// 已发布同 hash 重放 → 幂等返回（不重复物化）。
func (l *PublishPoolLogic) PublishPool(in *draw.PublishPoolReq) (*draw.BoxDraft, error) {
	if err := poolValidateKey(in.GetPoolId(), in.GetVersion(), in.GetExpectedConfigHash()); err != nil {
		return nil, err
	}

	tx := l.svcCtx.DB.Begin()
	if tx.Error != nil {
		return nil, errorx.FromMySQLError(tx.Error)
	}
	defer func() { _ = tx.Rollback() }()

	row, err := repo.LockPoolVersion(tx, in.GetPoolId(), in.GetVersion())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, poolErrNotFound()
		}
		return nil, errorx.FromMySQLError(err)
	}
	if row.Status == repo.StatusPublished {
		// 已发布：同 hash 幂等返回
		if bytes.Equal(row.ConfigHash, in.GetExpectedConfigHash()) {
			_ = tx.Rollback()
			return poolGetVersion(l.svcCtx.DB, in.GetPoolId(), in.GetVersion())
		}
		return nil, poolErrConflict()
	}
	if row.Status != repo.StatusDraft {
		return nil, poolErrConflict()
	}
	if !bytes.Equal(row.ConfigHash, in.GetExpectedConfigHash()) {
		return nil, poolErrConflict()
	}

	approval, err := repo.LockPoolPublicationApproval(tx, in.GetPoolId(), in.GetVersion())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, poolErrPublication()
		}
		return nil, errorx.FromMySQLError(err)
	}
	if approval.Status != repo.ApprovalApproved || approval.ApproverID == 0 ||
		approval.ApproverID == approval.RequesterID ||
		!bytes.Equal(approval.ConfigHash, in.GetExpectedConfigHash()) {
		return nil, poolErrPublication()
	}

	box, err := poolDecode(row)
	if err != nil {
		return nil, err
	}
	if err = l.materialize(tx, box); err != nil {
		return nil, err
	}
	now := time.Now()
	ok, err := repo.MarkPoolVersionPublished(tx, in.GetPoolId(), in.GetVersion(), in.GetExpectedConfigHash(), now)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	if !ok {
		return nil, poolErrConflict()
	}
	if err = tx.Commit().Error; err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	box.Status = draw.PoolStatus_POOL_STATUS_PUBLISHED
	box.PublishedAtUnixMillis = now.UnixMilli()
	return box, nil
}

// materialize 物化线上表（tx 内，缓存键随 ExecCtx 失效）：
// draw_boxes 按 pool_id upsert（新建插行 / 既有保 created_at 更新）；
// draw_pool_entries 整体替换（旧行全删 + 新行 remaining=quantity 全新库存）。
func (l *PublishPoolLogic) materialize(tx *gorm.DB, box *draw.BoxDraft) error {
	now := time.Now()
	var rolling *string
	if box.RollingRulesJson != "" {
		r := box.RollingRulesJson
		rolling = &r
	}
	boxRow := &drawmodel.DrawBoxes{
		Id:               box.PoolId,
		BoxName:          box.BoxName,
		Price:            box.Price,
		Description:      box.Description,
		Status:           1,
		RollingRulesJson: rolling,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// upsert：先无锁查既有行（锁已由版本行兜底串行化发布），决定插 or 更新
	var existing drawmodel.DrawBoxes
	err := tx.Where("`id` = ?", box.PoolId).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if err = l.svcCtx.BoxModel.Insert(l.ctx, tx, boxRow); err != nil {
			return errorx.FromMySQLError(err)
		}
	case err != nil:
		return errorx.FromMySQLError(err)
	default:
		boxRow.CreatedAt = existing.CreatedAt
		if err = l.svcCtx.BoxModel.Update(l.ctx, tx, boxRow); err != nil {
			return errorx.FromMySQLError(err)
		}
	}

	entries := make([]drawmodel.DrawPoolEntries, 0, len(box.Entries))
	for _, e := range box.Entries {
		if e == nil {
			continue
		}
		isPrize := int64(0)
		if e.IsPrize {
			isPrize = 1
		}
		rollingLimited := int64(0)
		if e.RollingLimited {
			rollingLimited = 1
		}
		entries = append(entries, drawmodel.DrawPoolEntries{
			BoxId:           box.PoolId,
			Name:            e.Name,
			Weight:          int64(e.Weight),
			Quantity:        int64(e.Quantity),
			Remaining:       int64(e.Quantity),
			IsPrize:         isPrize,
			DisplayValueFen: e.DisplayValueFen,
			RollingLimited:  rollingLimited,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}
	if err = l.svcCtx.EntryModel.ReplaceByBoxId(l.ctx, tx, box.PoolId, entries); err != nil {
		return errorx.FromMySQLError(err)
	}
	return nil
}
