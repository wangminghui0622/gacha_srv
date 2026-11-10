package logic

import (
	"errors"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/core"
	"gacha_srv/app/draw/internal/repo"
	"gacha_srv/pkg/utils/errorx"

	"gorm.io/gorm"
)

// poolStatusFromRow 版本行状态 → 枚举。
func poolStatusFromRow(s uint8) draw.PoolStatus {
	if s == repo.StatusPublished {
		return draw.PoolStatus_POOL_STATUS_PUBLISHED
	}
	return draw.PoolStatus_POOL_STATUS_DRAFT
}

// poolDecode 版本行 → 草稿（config_json protojson 反序列化）。
// 行状态/发布时间覆盖密封值（config_json 密封时为 DRAFT；发布后行状态 PUBLISHED）。
func poolDecode(row *repo.PoolVersion) (*draw.BoxDraft, error) {
	box, err := core.UnmarshalDraft(row.ConfigJSON)
	if err != nil {
		return nil, errorx.FromError(err)
	}
	box.Status = poolStatusFromRow(row.Status)
	if row.PublishedAt != nil {
		box.PublishedAtUnixMillis = row.PublishedAt.UnixMilli()
	} else {
		box.PublishedAtUnixMillis = 0
	}
	return box, nil
}

// poolGetVersion 按 (pool_id, version) 读取版本（无锁读）。
func poolGetVersion(db *gorm.DB, poolID, version int64) (*draw.BoxDraft, error) {
	row, err := repo.GetPoolVersion(db, poolID, version)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodePoolVersionNotFound, "赏池版本不存在")
		}
		return nil, errorx.FromMySQLError(err)
	}
	return poolDecode(row)
}

// poolValidateKey 校验 (pool_id, version, expected_hash) 请求参数；非法返回 2006。
func poolValidateKey(poolID, version int64, hash []byte) error {
	if poolID <= 0 || version <= 0 || len(hash) == 0 {
		return errorx.NewCodeError(errorx.CodePoolDraftInvalid, "poolId/version/configHash 必填且合法")
	}
	return nil
}

// clampPageSize 游标分页页大小（server_v2：默认 20，上限 100）。
func clampPageSize(n int32) int {
	if n <= 0 {
		return 20
	}
	if n > 100 {
		return 100
	}
	return int(n)
}

// poolErrVersionNotFound/Conflict 统一映射（repo 层行缺失/状态不符 → 业务码）。
func poolErrNotFound() error {
	return errorx.NewCodeError(errorx.CodePoolVersionNotFound, "赏池版本不存在")
}

func poolErrConflict() error {
	return errorx.NewCodeError(errorx.CodePoolVersionConflict, "赏池版本冲突：配置已变更或状态不满足操作")
}

func poolErrPublication() error {
	return errorx.NewCodeError(errorx.CodePoolPublicationConflict, "发布审批冲突：未提请/已批准/自批/状态不满足")
}
