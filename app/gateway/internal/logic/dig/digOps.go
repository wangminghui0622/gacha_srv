package dig

import (
	"context"
	"strconv"
	"strings"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/pkg/utils/errorx"
)

// parseCellsCSV "0,1,2" → []int64（非法片段返回 nil；调用方判空报 400）
func parseCellsCSV(s string) []int64 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil || n < 0 {
			return nil
		}
		out = append(out, n)
	}
	return out
}

// resolveDigCells 挖宝格子解析：takeAll → 会话空位（服务端读取），否则用显式格子。
// 返回 (cells, count)；count<=0 → CodeError。
// D5：会话 COMPLETED（满格）时 takeAll = 重开一轮——解析为全部格子，
// 由 LockDigBox 锁内回收会话（bitmap 清零）后置位，网关无需特判。
func resolveDigCells(ctx context.Context, svcCtx *svc.ServiceContext, userId, boxId int64,
	explicit []int64, takeAll bool) ([]int64, error) {
	if takeAll {
		sess, err := svcCtx.PlayRpc.GetDigBoxSession(ctx, &playClient.GetDigBoxSessionReq{
			UserId: userId,
			BoxId:  boxId,
		})
		if err != nil {
			return nil, errorx.FromError(err)
		}
		var cells []int64
		if sess.HasSession && sess.Status == 2 { // COMPLETED：重开一轮 → 全部格子
			for i := int64(0); i < sess.CellCount; i++ {
				cells = append(cells, i)
			}
			return cells, nil
		}
		for i := int64(0); i < sess.CellCount; i++ {
			if sess.OccupiedBitmap&(1<<i) == 0 {
				cells = append(cells, i)
			}
		}
		if len(cells) == 0 {
			return nil, errorx.NewCodeError(errorx.CodeDigCellOccupied, "该宝箱已被挖完，无法全开")
		}
		return cells, nil
	}
	if len(explicit) == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请选择格子或使用一键全开")
	}
	// 去重校验
	seen := make(map[int64]bool, len(explicit))
	for _, c := range explicit {
		if seen[c] {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "所选格子重复")
		}
		seen[c] = true
	}
	return explicit, nil
}
