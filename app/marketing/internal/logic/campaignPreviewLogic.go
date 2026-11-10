package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CampaignPreviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCampaignPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CampaignPreviewLogic {
	return &CampaignPreviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CampaignPreview 参与预览：kind 分发（签到下一日奖励 / 兑换码奖品 / 转盘槽位），无写入。
// 错误语义：业务失败 CodeError（拦截器转 gRPC 业务码），系统错误 codes.Internal。
func (l *CampaignPreviewLogic) CampaignPreview(in *marketing.CampaignPreviewReq) (*marketing.CampaignPreviewResp, error) {
	if in.UserId <= 0 || in.CampaignKey == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	campaign, err := l.svcCtx.CampaignModel.FindOneByCampaignKey(l.ctx, in.CampaignKey)
	if isNotFound(err) {
		return nil, errorx.NewCodeError(errorx.CodeCampaignNotFound, "活动不存在")
	}
	if err != nil {
		l.Errorf("CampaignPreview find campaign failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if campaign.Status != 1 {
		return nil, errorx.NewCodeError(errorx.CodeCampaignNotFound, "活动已停用")
	}
	if !inWindow(campaign.StartAt, campaign.EndAt) {
		return nil, errorx.NewCodeError(errorx.CodeCampaignNotInWindow, "活动未开始或已结束")
	}

	resp := &marketing.CampaignPreviewResp{CampaignKey: in.CampaignKey, Kind: campaign.Kind, SpinsLeft: -1, SpinDailyLimit: -1}
	switch campaign.Kind {
	case campaignSignIn:
		if err := l.previewSignIn(campaign, in.UserId, resp); err != nil {
			return nil, err
		}
	case campaignRedeemCode:
		if err := l.previewRedeem(campaign, in.UserId, in.Code, resp); err != nil {
			return nil, err
		}
	case campaignWheel:
		if err := l.previewWheel(campaign, in.UserId, resp); err != nil {
			return nil, err
		}
	default:
		return nil, errorx.NewCodeError(errorx.CodeCampaignParamInvalid, "未知活动类型")
	}
	return resp, nil
}

// previewSignIn 签到预览：今日是否已签、下次连续天数与对应阶梯奖励
func (l *CampaignPreviewLogic) previewSignIn(campaign *gamemodel.Campaigns, userId int64, resp *marketing.CampaignPreviewResp) error {
	cfg, _, err := parseCampaignConfig(campaignSignIn, campaign.ConfigJson)
	if err != nil {
		return err
	}
	row, err := l.svcCtx.CampaignSignInModel.FindOneByCampaignIdUserId(l.ctx, campaign.Id, userId)
	if err != nil && !isNotFound(err) {
		l.Errorf("CampaignPreview sign-in query failed, err: %v", err)
		return status.Error(codes.Internal, err.Error())
	}
	// 无签到记录或断签 → 下次为第 1 天；昨日签过 → streak+1；今日已签 → 维持当前
	nextStreak := int64(1)
	if err == nil {
		resp.Streak = row.Streak
		if row.LastSignedOn != nil {
			switch localDate(*row.LastSignedOn) {
			case localToday():
				resp.AlreadyParticipated = true
				nextStreak = row.Streak
			case localYesterday():
				nextStreak = row.Streak + 1
			}
		}
	}
	resp.NextStreak = nextStreak
	day := cfg.Rewards[(nextStreak-1)%int64(len(cfg.Rewards))]
	for _, p := range day.Prizes {
		resp.PreviewPrizes = append(resp.PreviewPrizes, prizeItemOf(l.ctx, l.svcCtx, p))
	}
	return nil
}

// previewRedeem 兑换码预览：code 非空时校验并返回奖品与兑换次数
func (l *CampaignPreviewLogic) previewRedeem(campaign *gamemodel.Campaigns, userId int64, code string, resp *marketing.CampaignPreviewResp) error {
	if code == "" {
		return nil // 未输入码：仅返回活动基础信息
	}
	row, err := l.svcCtx.RedeemCodeModel.FindOneByCode(l.ctx, code)
	if isNotFound(err) || (err == nil && row.Status != 1) {
		return errorx.NewCodeError(errorx.CodeRedemptionInvalid, "兑换码无效")
	}
	if err != nil {
		l.Errorf("CampaignPreview redeem query failed, err: %v", err)
		return status.Error(codes.Internal, err.Error())
	}
	if row.CampaignId != campaign.Id {
		return errorx.NewCodeError(errorx.CodeRedemptionInvalid, "兑换码不属于该活动")
	}
	resp.RedemptionUserLimit = row.PerUserLimit
	if userRow, uerr := l.svcCtx.RedeemUserModel.FindOneByCampaignIdCodeUserId(l.ctx, campaign.Id, code, userId); uerr == nil {
		resp.RedemptionTimes = userRow.Times
	} else if !isNotFound(uerr) {
		l.Errorf("CampaignPreview redeem user query failed, err: %v", uerr)
		return status.Error(codes.Internal, uerr.Error())
	}
	prizes, perr := parsePrizeConfig(row.PrizeConfig)
	if perr != nil {
		return perr
	}
	for _, p := range prizes {
		resp.PreviewPrizes = append(resp.PreviewPrizes, prizeItemOf(l.ctx, l.svcCtx, p))
	}
	return nil
}

// previewWheel 转盘预览：今日剩余次数 + 槽位（remaining=0 不参与抽取，仍展示）。
// M14d 注：console 多奖品槽的单奖品列=首奖品，预览按首奖品展示（响应形状不变）。
func (l *CampaignPreviewLogic) previewWheel(campaign *gamemodel.Campaigns, userId int64, resp *marketing.CampaignPreviewResp) error {
	_, wcfg, err := parseCampaignConfig(campaignWheel, campaign.ConfigJson)
	if err != nil {
		return err
	}
	resp.SpinDailyLimit = wcfg.DailyLimit
	resp.SpinsLeft = wcfg.DailyLimit
	if row, werr := l.svcCtx.WheelDailyModel.FindOneByCampaignIdUserIdSpinDate(l.ctx, campaign.Id, userId, localTodayTime()); werr == nil {
		left := wcfg.DailyLimit - row.SpinCount
		if left < 0 {
			left = 0
		}
		resp.SpinsLeft = left
	} else if !isNotFound(werr) {
		l.Errorf("CampaignPreview wheel daily query failed, err: %v", werr)
		return status.Error(codes.Internal, werr.Error())
	}
	var slots []gamemodelnc.CampaignWheelSlots
	if err := l.svcCtx.DB.Where("campaign_id = ?", campaign.Id).Order("slot_index").Find(&slots).Error; err != nil {
		l.Errorf("CampaignPreview wheel slots query failed, err: %v", err)
		return status.Error(codes.Internal, err.Error())
	}
	for i := range slots {
		s := &slots[i]
		resp.Slots = append(resp.Slots, &marketing.WheelSlotItem{
			SlotIndex: s.SlotIndex,
			Label:     s.Label,
			Weight:    s.Weight,
			Remaining: s.Remaining,
			Prize: prizeItemOf(l.ctx, l.svcCtx, prizeCfg{
				Kind:             s.PrizeKind,
				CashFen:          s.CashFen,
				Points:           s.PointsAmount,
				CouponTemplateNo: s.CouponTemplateNo,
			}),
		})
	}
	return nil
}
