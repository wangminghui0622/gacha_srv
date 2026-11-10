package logic

import (
	"context"

	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckGrantsByRequestIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckGrantsByRequestIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckGrantsByRequestIdLogic {
	return &CheckGrantsByRequestIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CheckGrantsByRequestId SAGA 04 补偿（CancelPayment）退款前置检查（同服务本地直调）：
// 按 gid 查 draw 域发放资产（asset_no="a_"+requestId）。存在且未撤销
// （state != canceled，含已消耗）⇒ 视为已发放，payment 补偿不得退钱——
// GrantAssetRevert 对已消耗资产仅防御跳过，撤销不掉时退款会让用户白拿奖品。
// 幂等：纯读查询，重复调用安全；系统故障返回 error 供 DTM 重试补偿。
func (l *CheckGrantsByRequestIdLogic) CheckGrantsByRequestId(in *money.CheckGrantsByRequestIdReq) (*money.CheckGrantsByRequestIdResp, error) {
	if in.RequestId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var n int64
	err := l.svcCtx.DB.WithContext(l.ctx).Model(&gacha_money.Assets{}).
		Where("asset_no = ? AND state <> ?", "a_"+in.RequestId, assetStateCanceled).
		Count(&n).Error
	if err != nil {
		return nil, err
	}
	return &money.CheckGrantsByRequestIdResp{Granted: n > 0}, nil
}
