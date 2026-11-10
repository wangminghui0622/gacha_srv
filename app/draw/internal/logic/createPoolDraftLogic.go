package logic

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/core"
	"gacha_srv/app/draw/internal/repo"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

type CreatePoolDraftLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePoolDraftLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePoolDraftLogic {
	return &CreatePoolDraftLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreatePoolDraft 创建赏池草稿（server_v2 CreatePoolDraft 语义）：
// 校验 → 幂等键重放 → 分配/锁定版本头 → 密封（version+config_hash）→ 落版本行
// + 幂等记录 → CAS 推进版本头。poolId=0 由服务端分配新池 id（本仓适配：
// server_v2 要求调用方携带非零 pool_id）。
func (l *CreatePoolDraftLogic) CreatePoolDraft(in *draw.CreatePoolDraftReq) (*draw.BoxDraft, error) {
	box := in.GetBox()
	key := strings.TrimSpace(in.GetIdempotencyKey())
	if box == nil || key == "" || len(key) > 128 {
		return nil, errorx.NewCodeError(errorx.CodePoolDraftInvalid, "草稿配置与幂等键必填（幂等键不超过 128 字符）")
	}
	if box.PoolId < 0 || box.Version != 0 || len(box.ConfigHash) != 0 || box.PublishedAtUnixMillis != 0 {
		return nil, errorx.NewCodeError(errorx.CodePoolDraftInvalid, "草稿不能携带版本号/配置哈希/发布时间")
	}
	if box.Status != draw.PoolStatus_POOL_STATUS_UNSPECIFIED && box.Status != draw.PoolStatus_POOL_STATUS_DRAFT {
		return nil, errorx.NewCodeError(errorx.CodePoolDraftInvalid, "草稿状态非法")
	}
	if err := core.ValidateDraft(box); err != nil {
		return nil, errorx.NewCodeError(errorx.CodePoolDraftInvalid, "草稿校验失败："+err.Error())
	}
	clone := proto.Clone(box).(*draw.BoxDraft)
	clone.Status = draw.PoolStatus_POOL_STATUS_DRAFT
	reqHash, err := core.DraftRequestHash(clone)
	if err != nil {
		return nil, errorx.FromError(err)
	}

	// 幂等重放：同 key 同载荷 → 返回既有草稿；同 key 不同载荷 → 冲突
	if prior, err := repo.GetPoolDraftRequest(l.svcCtx.DB, key); err == nil {
		if prior.PoolID != clone.PoolId && clone.PoolId != 0 {
			return nil, errorx.NewCodeError(errorx.CodePoolDraftKeyConflict, "幂等键已用于其他赏池")
		}
		if !bytes.Equal(prior.RequestHash, reqHash) {
			return nil, errorx.NewCodeError(errorx.CodePoolDraftKeyConflict, "幂等键已存在且载荷不同")
		}
		return l.getDraft(prior.PoolID, prior.Version)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errorx.FromMySQLError(err)
	}

	tx := l.svcCtx.DB.Begin()
	if tx.Error != nil {
		return nil, errorx.FromMySQLError(tx.Error)
	}
	defer func() { _ = tx.Rollback() }()

	poolID, err := repo.EnsurePoolHead(tx, clone.PoolId)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	version, err := repo.LockPoolHead(tx, poolID)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	clone.PoolId = poolID
	if err = core.SealPoolDraft(clone, version); err != nil {
		return nil, errorx.NewCodeError(errorx.CodePoolDraftInvalid, "草稿密封失败："+err.Error())
	}
	cfgJSON, err := core.MarshalDraft(clone)
	if err != nil {
		return nil, errorx.FromError(err)
	}
	if err = repo.InsertPoolVersion(tx, &repo.PoolVersion{
		PoolID: poolID, Version: version, Status: repo.StatusDraft,
		ConfigHash: clone.ConfigHash, ConfigJSON: cfgJSON, CreatedAt: time.Now(),
	}); err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	if err = repo.InsertPoolDraftRequest(tx, &repo.PoolDraftRequest{
		IdempotencyKey: key, PoolID: poolID, Version: version,
		RequestHash: reqHash, CreatedAt: time.Now(),
	}); err != nil {
		// 并发同 key：回滚后按重放返回
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			_ = tx.Rollback()
			if prior, rerr := repo.GetPoolDraftRequest(l.svcCtx.DB, key); rerr == nil {
				return l.getDraft(prior.PoolID, prior.Version)
			}
		}
		return nil, errorx.FromMySQLError(err)
	}
	ok, err := repo.AdvancePoolHead(tx, poolID, version, version+1)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	if !ok {
		return nil, errorx.NewCodeError(errorx.CodePoolVersionConflict, "赏池版本号并发冲突")
	}
	if err = tx.Commit().Error; err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return clone, nil
}

// getDraft 按 (pool_id, version) 读取并反序列化版本行。
func (l *CreatePoolDraftLogic) getDraft(poolID, version int64) (*draw.BoxDraft, error) {
	row, err := repo.GetPoolVersion(l.svcCtx.DB, poolID, version)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodePoolVersionNotFound, "赏池版本不存在")
		}
		return nil, errorx.FromMySQLError(err)
	}
	return poolDecode(row)
}
