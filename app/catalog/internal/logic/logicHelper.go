// Package logic 商品目录 RPC 逻辑（M14c）：参数校验 → core 纯函数 → repo 事务，
// 错误统一映射 errorx 4000 段（4001 不存在 / 4002 冲突 / 4003 校验失败）。
package logic

import (
	"errors"

	"gacha_srv/app/catalog/internal/repo"
	"gacha_srv/pkg/utils/errorx"
)

// mapRepoErr repo 哨兵错误 → errorx 业务码；其余折叠（数据库错误/未知错误）。
func mapRepoErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repo.ErrNotFound):
		return errorx.NewCodeError(errorx.CodeCatalogNotFound, "商品不存在")
	case errors.Is(err, repo.ErrConflict):
		return errorx.NewCodeError(errorx.CodeCatalogConflict, "商品版本或状态冲突")
	default:
		return errorx.FromError(err)
	}
}

// mapVariantErr 变体/修订冲突 → 4002。
func mapVariantErr(err error) error {
	return errorx.NewCodeError(errorx.CodeCatalogConflict, "商品版本或状态冲突")
}

// mapIPRepoErr IP 数据访问错误 → 业务码。
func mapIPRepoErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repo.ErrNotFound):
		return errorx.NewCodeError(errorx.CodeCatalogNotFound, "IP 不存在")
	case errors.Is(err, repo.ErrIPExists):
		return errorx.NewCodeError(errorx.CodeCatalogInvalid, "IP 名称已存在")
	default:
		return errorx.FromError(err)
	}
}
