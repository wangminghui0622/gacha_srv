package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListCampaignsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCampaignsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCampaignsLogic {
	return &ListCampaignsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListCampaigns 活动列表 + 我的状态（每类活动带该用户侧边状态：签到连续天数/转盘剩余次数）。
// 列表页低频，gorm 直查（campaigns 缓存模型服务单条热路径）。
func (l *ListCampaignsLogic) ListCampaigns(in *marketing.ListCampaignsReq) (*marketing.ListCampaignsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var campaigns []gamemodel.Campaigns
	if err := l.svcCtx.DB.Where("status = ?", 1).Order("id").Find(&campaigns).Error; err != nil {
		l.Errorf("ListCampaigns query failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	items := make([]*marketing.CampaignItem, 0, len(campaigns))
	for i := range campaigns {
		c := &campaigns[i]
		item := &marketing.CampaignItem{
			CampaignKey:    c.CampaignKey,
			Name:           c.Name,
			Kind:           c.Kind,
			StartAt:        c.StartAt.Unix(),
			EndAt:          c.EndAt.Unix(),
			InWindow:       inWindow(c.StartAt, c.EndAt),
			SpinsLeft:      -1,
			SpinDailyLimit: -1,
		}
		switch c.Kind {
		case campaignSignIn:
			if row, err := l.svcCtx.CampaignSignInModel.FindOneByCampaignIdUserId(l.ctx, c.Id, in.UserId); err == nil {
				item.SignedToday = row.LastSignedOn != nil && localDate(*row.LastSignedOn) == localToday()
				item.Streak = row.Streak
				item.TotalDays = row.TotalDays
			} else if !isNotFound(err) {
				l.Errorf("ListCampaigns sign-in query failed, err: %v", err)
				return nil, status.Error(codes.Internal, err.Error())
			}
		case campaignWheel:
			_, wcfg, err := parseCampaignConfig(campaignWheel, c.ConfigJson)
			if err != nil {
				l.Errorf("ListCampaigns wheel config invalid, campaignKey: %s, err: %v", c.CampaignKey, err)
				return nil, err
			}
			item.SpinDailyLimit = wcfg.DailyLimit
			item.SpinsLeft = wcfg.DailyLimit
			if row, err := l.svcCtx.WheelDailyModel.FindOneByCampaignIdUserIdSpinDate(l.ctx, c.Id, in.UserId, localTodayTime()); err == nil {
				left := wcfg.DailyLimit - row.SpinCount
				if left < 0 {
					left = 0
				}
				item.SpinsLeft = left
			} else if !isNotFound(err) {
				l.Errorf("ListCampaigns wheel daily query failed, err: %v", err)
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		items = append(items, item)
	}
	return &marketing.ListCampaignsResp{Items: items}, nil
}
