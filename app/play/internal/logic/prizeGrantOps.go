package logic

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/svc"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"

	"gorm.io/gorm"
)

// ══════════════ SAGA 06 发奖落库标记（prize_grant_marks）══════════════
// 高1 修复（发奖与退款不对称）：五条发奖链（gashapon/dig/fullcar/groupbuy/combochain）
// 的 06 发奖循环成功后按 gid 写标记（uk request_id 幂等，重试/重放安全）；
// 04 补偿（CancelPayment）退款前经 CheckPrizesGranted 查标记——已发奖则不退钱
// （资产已发放不追回、钱保持扣除、冻结保持可恢复），用户不能白拿奖品；
// 反之退款成功则奖不追（标记不存在=未发奖，正常退款）。

// insertPrizeGrantMark 按 gid 写发奖落库标记（uk request_id；撞键=幂等成功）
func insertPrizeGrantMark(ctx context.Context, svcCtx *svc.ServiceContext, requestId string, userId int64) error {
	err := svcCtx.PrizeGrantMarkModel.Insert(ctx, nil, &gamemodelnc.PrizeGrantMarks{
		RequestId: requestId,
		UserId:    userId,
	})
	if err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		return err
	}
	return nil
}
