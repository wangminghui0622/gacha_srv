package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/marketing/internal/core"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CampaignParticipateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCampaignParticipateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CampaignParticipateLogic {
	return &CampaignParticipateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CampaignParticipate 统一参与入口：kind 分发 + idem_key 幂等重放 + PENDING 补发（计划 D3）。
// 事务内只写本域（参与行 + 计数 + PENDING 流水），事务外幂等外部发放；
// 发放失败 → codes.Internal，客户端同 Idempotency-Key 重试 → 重放只补发 PENDING。
// 错误语义：业务失败 CodeError（拦截器转 gRPC 业务码跨服务不失真），系统错误 codes.Internal。
func (l *CampaignParticipateLogic) CampaignParticipate(in *marketing.CampaignParticipateReq) (*marketing.CampaignParticipateResp, error) {
	if in.UserId <= 0 || in.CampaignKey == "" || in.RequestId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	if len(in.RequestId) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "幂等键超长（最多64字符）")
	}
	idemKey := in.RequestId

	// ① 幂等重放：撞 uk idem_key 读回结果 + 补发 PENDING（重试/重复提交收敛）
	if p, err := l.svcCtx.ParticipationModel.FindOneByIdemKey(l.ctx, idemKey); err == nil {
		if p.UserId != in.UserId {
			return nil, errorx.NewCodeError(errorx.CodeCampaignParamInvalid, "幂等键已被其他用户使用")
		}
		if err := grantPrizesPending(l.ctx, l.svcCtx, idemKey); err != nil {
			l.Errorf("CampaignParticipate replay grants failed, idemKey: %s, err: %v", idemKey, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		r := unmarshalResult(p.ResultJson)
		l.Infof("CampaignParticipate replayed, idemKey: %s, campaignKey: %s", idemKey, in.CampaignKey)
		return &marketing.CampaignParticipateResp{
			RequestId: in.RequestId,
			IdemKey:   idemKey,
			Replayed:  true,
			Streak:    r.Streak,
			SpinsLeft: r.SpinsLeft,
			Prizes:    r.Prizes,
		}, nil
	} else if !isNotFound(err) {
		l.Errorf("CampaignParticipate replay query failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ② 活动与窗口校验
	campaign, err := l.svcCtx.CampaignModel.FindOneByCampaignKey(l.ctx, in.CampaignKey)
	if isNotFound(err) || (err == nil && campaign.Status != 1) {
		return nil, errorx.NewCodeError(errorx.CodeCampaignNotFound, "活动不存在")
	}
	if err != nil {
		l.Errorf("CampaignParticipate find campaign failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !inWindow(campaign.StartAt, campaign.EndAt) {
		return nil, errorx.NewCodeError(errorx.CodeCampaignNotInWindow, "活动未开始或已结束")
	}

	// ③ 事务内 kind 分发：计数/状态迁移 + 参与行 + PENDING 流水（原子）
	var (
		streak, spinsLeft int64 = 0, -1
		prizes            []prizeCfg
	)
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var derr error
		switch campaign.Kind {
		case campaignSignIn:
			streak, prizes, derr = l.signInTx(tx, campaign, in.UserId)
		case campaignRedeemCode:
			prizes, derr = l.redeemTx(tx, campaign, in.UserId, in.Code)
		case campaignWheel:
			spinsLeft, prizes, derr = l.wheelTx(tx, campaign, in.UserId, idemKey)
		default:
			derr = errorx.NewCodeError(errorx.CodeCampaignParamInvalid, "未知活动类型")
		}
		if derr != nil {
			return derr
		}
		// 券奖品模板在参与时校验（发放期模板必须可用；重放路径不再校验）
		for _, p := range prizes {
			if p.Kind != prizeCoupon {
				continue
			}
			tpl, terr := l.svcCtx.TemplateModel.FindOneByTemplateNo(l.ctx, p.CouponTemplateNo)
			if isNotFound(terr) || (terr == nil && tpl.Status != templatePublished) {
				return errorx.NewCodeError(errorx.CodeCouponTemplateNotFound, "券模板不存在或未发布")
			}
			if terr != nil {
				return terr
			}
		}
		resultJSON, merr := marshalResult(&participateResult{Streak: streak, SpinsLeft: spinsLeft,
			Prizes: prizeItemsOf(l.ctx, l.svcCtx, prizes)})
		if merr != nil {
			return merr
		}
		if _, ierr := insertParticipationAndGrants(tx, idemKey, campaign.Id, in.UserId, campaign.Kind, prizes, resultJSON); ierr != nil {
			return ierr
		}
		return nil
	})
	if err != nil {
		// CodeError 原样返回（拦截器转 gRPC 业务码）；其余系统错误折叠 Internal
		if errorx.IsCodeError(err) {
			l.Errorf("CampaignParticipate aborted, campaignKey: %s, reason: %v", in.CampaignKey, err)
			return nil, err
		}
		l.Errorf("CampaignParticipate tx failed, campaignKey: %s, err: %v", in.CampaignKey, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ④ 事务外幂等发放（失败 Internal → 客户端重试补发）
	if err := grantPrizesPending(l.ctx, l.svcCtx, idemKey); err != nil {
		l.Errorf("CampaignParticipate grants failed, idemKey: %s, err: %v", idemKey, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	l.Infof("CampaignParticipate ok, idemKey: %s, campaignKey: %s, kind: %d, prizes: %d",
		idemKey, in.CampaignKey, campaign.Kind, len(prizes))
	return &marketing.CampaignParticipateResp{
		RequestId: in.RequestId,
		IdemKey:   idemKey,
		Replayed:  false,
		Streak:    streak,
		SpinsLeft: spinsLeft,
		Prizes:    prizeItemsOf(l.ctx, l.svcCtx, prizes),
	}, nil
}

// signInTx 签到：FOR UPDATE 锁行；同日重复无新奖励（日期幂等兜底）；
// 昨日签过 streak+1，否则重置为 1；阶梯 (streak-1)%len 取当日奖励。
func (l *CampaignParticipateLogic) signInTx(tx *gorm.DB, campaign *gamemodel.Campaigns, userId int64) (int64, []prizeCfg, error) {
	cfg, _, err := parseCampaignConfig(campaignSignIn, campaign.ConfigJson)
	if err != nil {
		return 0, nil, err
	}
	var row gamemodelnc.CampaignSignIns
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("campaign_id = ? AND user_id = ?", campaign.Id, userId).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil, err
	}
	// 同日重复：无新奖励（并发下 uk 插入撞键 → tx 回滚 → 客户端同 key 重放）
	if row.LastSignedOn != nil && localDate(*row.LastSignedOn) == localToday() {
		return row.Streak, nil, nil
	}
	streak := int64(1)
	if row.LastSignedOn != nil && localDate(*row.LastSignedOn) == localYesterday() {
		streak = row.Streak + 1
	}
	prizes := cfg.Rewards[(streak-1)%int64(len(cfg.Rewards))].Prizes
	now := time.Now()
	if row.Id == 0 {
		row = gamemodelnc.CampaignSignIns{
			CampaignId:   campaign.Id,
			UserId:       userId,
			Streak:       streak,
			TotalDays:    1,
			LastSignedOn: &now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return 0, nil, err
		}
	} else {
		if err := tx.Model(&row).Updates(map[string]interface{}{
			"streak": streak, "total_days": gorm.Expr("total_days + 1"), "last_signed_on": now,
		}).Error; err != nil {
			return 0, nil, err
		}
	}
	return streak, prizes, nil
}

// redeemTx 兑换码：FOR UPDATE 锁码行 → 用户计数 → 盲更新总次数防超发（WHERE redeemed_count < max）
func (l *CampaignParticipateLogic) redeemTx(tx *gorm.DB, campaign *gamemodel.Campaigns, userId int64, code string) ([]prizeCfg, error) {
	if code == "" {
		return nil, errorx.NewCodeError(errorx.CodeCampaignParamInvalid, "兑换码不能为空")
	}
	var codeRow gamemodelnc.CampaignRedemptionCodes
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("code = ?", code).First(&codeRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errorx.NewCodeError(errorx.CodeRedemptionInvalid, "兑换码无效")
	}
	if err != nil {
		return nil, err
	}
	if codeRow.Status != 1 || codeRow.CampaignId != campaign.Id {
		return nil, errorx.NewCodeError(errorx.CodeRedemptionInvalid, "兑换码无效")
	}

	// 用户计数（FOR UPDATE + 缺失则建行；uk 撞键=并发先到，回滚后客户端重放收敛）
	var userRow gamemodelnc.CampaignRedemptionUsers
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("campaign_id = ? AND code = ? AND user_id = ?", campaign.Id, code, userId).First(&userRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now()
		userRow = gamemodelnc.CampaignRedemptionUsers{
			CampaignId: campaign.Id,
			Code:       code,
			UserId:     userId,
			Times:      0,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := tx.Create(&userRow).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if userRow.Times >= codeRow.PerUserLimit {
		return nil, errorx.NewCodeError(errorx.CodeRedemptionUserLimit, "该用户兑换次数已达上限")
	}
	if err := tx.Model(&userRow).Update("times", gorm.Expr("times + 1")).Error; err != nil {
		return nil, err
	}

	// 总量盲更新：并发防超发（0 行=已兑完）
	result := tx.Model(&gamemodelnc.CampaignRedemptionCodes{}).
		Where("code = ? AND redeemed_count < max_redemptions", code).
		Update("redeemed_count", gorm.Expr("redeemed_count + 1"))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, errorx.NewCodeError(errorx.CodeRedemptionExhausted, "兑换码已兑完")
	}

	prizes, err := parsePrizeConfig(codeRow.PrizeConfig)
	if err != nil {
		return nil, err
	}
	return prizes, nil
}

// wheelTx 转盘：FOR UPDATE 锁日计数与全部槽位（同一事务内抽奖+扣减，行锁防并发超抽）；
// 只在 remaining≠0 且日库存未耗尽的槽中加权，全无 → 5012。
// M14d 扩展：槽 prize_config 非空按全量奖品数组发放（console 多奖品槽）；
// daily_stock>0 的槽按日限额（daily_spent/daily_stock_date 跨日重置）。
func (l *CampaignParticipateLogic) wheelTx(tx *gorm.DB, campaign *gamemodel.Campaigns, userId int64, idemKey string) (int64, []prizeCfg, error) {
	_, wcfg, err := parseCampaignConfig(campaignWheel, campaign.ConfigJson)
	if err != nil {
		return 0, nil, err
	}
	// 日计数行
	var daily gamemodelnc.CampaignWheelDaily
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("campaign_id = ? AND user_id = ? AND spin_date = ?", campaign.Id, userId, localTodayTime()).
		First(&daily).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now()
		daily = gamemodelnc.CampaignWheelDaily{
			CampaignId: campaign.Id,
			UserId:     userId,
			SpinDate:   localTodayTime(),
			SpinCount:  0,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := tx.Create(&daily).Error; err != nil {
			return 0, nil, err
		}
	} else if err != nil {
		return 0, nil, err
	}
	if daily.SpinCount >= wcfg.DailyLimit {
		return 0, nil, errorx.NewCodeError(errorx.CodeWheelDailyLimit, "转盘今日次数已用完")
	}

	// 槽位行锁 + 加权抽取（日库存：daily_stock>0 时当日已中数达上限不参与，跨日重置）
	var slots []gamemodelnc.CampaignWheelSlots
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("campaign_id = ?", campaign.Id).Order("slot_index").Find(&slots).Error; err != nil {
		return 0, nil, err
	}
	today := localToday()
	slotAvailable := func(s *gamemodelnc.CampaignWheelSlots) bool {
		if s.Remaining == 0 {
			return false
		}
		if s.DailyStock <= 0 { // -1 不限 / 0 无日库存语义（M11 种子）
			return true
		}
		if s.DailyStockDate == nil || localDate(*s.DailyStockDate) != today {
			return true // 从未计数或跨日（抽中时重置）
		}
		return s.DailySpent < s.DailyStock
	}
	var totalWeight int64
	for i := range slots {
		if slotAvailable(&slots[i]) {
			totalWeight += slots[i].Weight
		}
	}
	if totalWeight <= 0 {
		return 0, nil, errorx.NewCodeError(errorx.CodeWheelEmpty, "转盘奖池已空")
	}
	// 确定性选槽：seed=幂等键 idemKey（同一参与幂等键必中同槽，可回放核对）
	roll := int64(core.FullCarRandom([]byte(idemKey), "campaign-wheel", 0) % uint64(totalWeight))
	var picked *gamemodelnc.CampaignWheelSlots
	for i := range slots {
		if !slotAvailable(&slots[i]) {
			continue
		}
		roll -= slots[i].Weight
		if roll < 0 {
			picked = &slots[i]
			break
		}
	}
	if picked == nil {
		picked = &slots[len(slots)-1]
	}
	if picked.Remaining > 0 {
		if err := tx.Model(picked).Update("remaining", gorm.Expr("remaining - 1")).Error; err != nil {
			return 0, nil, err
		}
	}
	if picked.DailyStock > 0 {
		if picked.DailyStockDate != nil && localDate(*picked.DailyStockDate) == today {
			if err := tx.Model(picked).Update("daily_spent", gorm.Expr("daily_spent + 1")).Error; err != nil {
				return 0, nil, err
			}
		} else {
			// 跨日重置：计数归 1，日期换今天
			if err := tx.Model(picked).Updates(map[string]interface{}{
				"daily_spent":      1,
				"daily_stock_date": localTodayTime(),
			}).Error; err != nil {
				return 0, nil, err
			}
		}
	}
	if err := tx.Model(&daily).Update("spin_count", gorm.Expr("spin_count + 1")).Error; err != nil {
		return 0, nil, err
	}
	spinsLeft := wcfg.DailyLimit - (daily.SpinCount + 1)

	// M14d：prize_config 非空 → 全量奖品数组发放；否则单奖品列（M11 种子/兜底）
	if picked.PrizeConfig != "" {
		prizes, err := parsePrizeConfig(picked.PrizeConfig)
		if err != nil {
			return 0, nil, err
		}
		return spinsLeft, prizes, nil
	}
	return spinsLeft, []prizeCfg{{
		Kind:             picked.PrizeKind,
		CashFen:          picked.CashFen,
		Points:           picked.PointsAmount,
		CouponTemplateNo: picked.CouponTemplateNo,
	}}, nil
}

// prizeItemsOf 配置奖品数组 → 响应 PrizeItem 数组
func prizeItemsOf(ctx context.Context, svcCtx *svc.ServiceContext, prizes []prizeCfg) []*marketing.PrizeItem {
	items := make([]*marketing.PrizeItem, 0, len(prizes))
	for _, p := range prizes {
		items = append(items, prizeItemOf(ctx, svcCtx, p))
	}
	return items
}
