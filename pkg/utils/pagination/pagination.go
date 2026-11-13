// Package pagination 控制台列表分页参数的统一归一化。
package pagination

// MaxPageSize 单页最大条数（所有管理端列表一致）。
const MaxPageSize uint32 = 100

// Normalize 归一化页码和每页条数：page 最小为 1，pageSize 为 0 时使用 fallback，超过上限则封顶。
func Normalize(page, pageSize, fallback uint32) (uint32, uint32) {
	if page == 0 {
		page = 1
	}
	return page, NormalizeSize(pageSize, fallback)
}

// NormalizeSize 归一化每页条数（游标分页只有 pageSize 时使用）。
func NormalizeSize(pageSize, fallback uint32) uint32 {
	if fallback == 0 {
		fallback = 20
	}
	if pageSize == 0 {
		pageSize = fallback
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return pageSize
}
