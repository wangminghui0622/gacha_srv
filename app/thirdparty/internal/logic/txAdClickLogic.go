package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/internal/tencentad"
	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TxAdClickLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTxAdClickLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TxAdClickLogic {
	return &TxAdClickLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TxAdClickLogic) TxAdClick(in *thirdparty.TxAdClickReq) (*thirdparty.TxAdClickResp, error) {
	// server_v2 TxAdClick 同款语义：服务未装配/账户未配置 6009，记录失败（openid 缺失等）6010
	if l.svcCtx.TencentAd == nil || !l.svcCtx.TencentAdConfigured() {
		return nil, errorx.NewCodeError(errorx.CodeTencentAdNotConfigured, "腾讯广告未配置")
	}
	err := l.svcCtx.TencentAd.RecordClick(l.ctx, tencentad.AdClickMessage{
		TenantID:     uint64(in.GetTenantId()),
		AccountID:    in.GetAccountId(),
		ClickID:      in.GetClickId(),
		ClickTime:    in.GetClickTime(),
		WechatOpenID: in.GetWechatOpenId(),
		DeviceOsType: in.GetDeviceOsType(),
	})
	if err != nil {
		return nil, errorx.NewCodeErrorf(errorx.CodeTencentAdRecordFailed, "tx ad click: %v", err)
	}
	return &thirdparty.TxAdClickResp{}, nil
}
