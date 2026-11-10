package task

import (
	"fmt"

	"gacha_srv/app/ops/ops"
	taskpb "gacha_srv/app/task/task"
)

type PassReward struct {
	Level            uint32
	Track            taskpb.PassTrack
	RewardTemplateID string
	RewardCount      uint32
	Reward           *ops.TaskRewardDefinition
}

type PassSeason struct {
	ID                 uint64
	Version            uint64
	Name               string
	XPPerLevel         int64
	MaxLevel           uint32
	Rewards            []PassReward
	XPPerSpendMinor    int64
	PremiumLine        taskpb.PassPremiumLine
	PremiumMemberLevel uint32
}

type passProgress struct {
	xp      int64
	premium bool
	claimed map[string]struct{}
}

type PassClaim struct {
	Level            uint32
	Track            taskpb.PassTrack
	RewardTemplateID string
	RewardCount      uint32
}

type passUserKey struct {
	userID uint64
	passID uint64
}

func (b *Board) AddPass(season PassSeason) error {
	if season.ID == 0 || season.Version == 0 || season.XPPerLevel <= 0 || season.XPPerSpendMinor <= 0 || season.MaxLevel == 0 || season.MaxLevel > 200 || len(season.Rewards) == 0 {
		return ErrInvalidTask
	}
	seen := map[string]struct{}{}
	premium := false
	for _, reward := range season.Rewards {
		if reward.Level == 0 || reward.Level > season.MaxLevel || !validReward(reward.Reward, reward.RewardTemplateID, reward.RewardCount) || (reward.Track != taskpb.PassTrack_PASS_TRACK_FREE && reward.Track != taskpb.PassTrack_PASS_TRACK_PREMIUM) {
			return ErrInvalidTask
		}
		if reward.Track == taskpb.PassTrack_PASS_TRACK_PREMIUM {
			premium = true
		}
		key := fmt.Sprintf("%d:%d", reward.Level, reward.Track)
		if _, ok := seen[key]; ok {
			return ErrInvalidTask
		}
		seen[key] = struct{}{}
	}
	switch season.PremiumLine {
	case taskpb.PassPremiumLine_PASS_PREMIUM_LINE_UNSPECIFIED:
		if premium || season.PremiumMemberLevel != 0 {
			return ErrInvalidTask
		}
	case taskpb.PassPremiumLine_PASS_PREMIUM_LINE_PAYMENT:
		if !premium || season.PremiumMemberLevel != 0 {
			return ErrInvalidTask
		}
	case taskpb.PassPremiumLine_PASS_PREMIUM_LINE_MEMBER:
		if !premium || season.PremiumMemberLevel == 0 {
			return ErrInvalidTask
		}
	default:
		return ErrInvalidTask
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.passes == nil {
		b.passes = map[uint64]PassSeason{}
		b.passUsers = map[passUserKey]passProgress{}
		b.passEvents = map[string]struct{}{}
		b.passClaims = map[string]PassClaim{}
	}
	if _, ok := b.passes[season.ID]; ok {
		return ErrInvalidTask
	}
	b.passes[season.ID] = season
	return nil
}

func (b *Board) creditPassLocked(userID uint64, metric taskpb.TaskMetric, amount int64, eventID string) {
	if metric != taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR {
		return
	}
	for id, season := range b.passes {
		per := season.XPPerSpendMinor
		if per <= 0 || amount > (1<<62)/per {
			continue
		}
		b.addPassXPLocked(userID, season, "pass:"+fmt.Sprint(id)+":"+eventID, amount*per)
	}
}

// UnlockPass 解锁付费通行证（仅 PAYMENT 线合法，老项目语义；内存模式）
func (b *Board) UnlockPass(userID, passID uint64) error {
	if userID == 0 || passID == 0 {
		return ErrInvalidTask
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	season, ok := b.passes[passID]
	if !ok {
		return ErrTaskNotFound
	}
	if season.PremiumLine != taskpb.PassPremiumLine_PASS_PREMIUM_LINE_PAYMENT {
		return ErrInvalidTask
	}
	row := b.passUsers[passUserKey{userID, passID}]
	row.premium = true
	if row.claimed == nil {
		row.claimed = map[string]struct{}{}
	}
	b.passUsers[passUserKey{userID, passID}] = row
	return nil
}

func (b *Board) addPassXPLocked(userID uint64, season PassSeason, eventID string, xp int64) uint32 {
	if _, ok := b.passEvents[eventID]; ok {
		return PassLevel(season, b.passUsers[passUserKey{userID, season.ID}].xp)
	}
	row := b.passUsers[passUserKey{userID, season.ID}]
	capXP := season.XPPerLevel * int64(season.MaxLevel)
	row.xp += xp
	if row.xp > capXP {
		row.xp = capXP
	}
	if row.claimed == nil {
		row.claimed = map[string]struct{}{}
	}
	b.passUsers[passUserKey{userID, season.ID}] = row
	b.passEvents[eventID] = struct{}{}
	return PassLevel(season, row.xp)
}

// ClaimPass 通行证奖励领取（内存模式）
func (b *Board) ClaimPass(userID, passID uint64, level uint32, track taskpb.PassTrack, requestID string) (PassClaim, error) {
	if userID == 0 || passID == 0 || level == 0 || requestID == "" || len(requestID) > 64 {
		return PassClaim{}, ErrInvalidTask
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	season, ok := b.passes[passID]
	if !ok {
		return PassClaim{}, ErrTaskNotFound
	}
	var reward PassReward
	found := false
	for _, item := range season.Rewards {
		if item.Level == level && item.Track == track {
			reward = item
			found = true
			break
		}
	}
	if !found {
		return PassClaim{}, ErrTaskNotFound
	}
	if existing, ok := b.passClaims[requestID]; ok {
		return existing, nil
	}
	row := b.passUsers[passUserKey{userID, passID}]
	if PassLevel(season, row.xp) < level {
		return PassClaim{}, ErrTaskNotReady
	}
	slot := fmt.Sprintf("%d:%d:%d:%d", userID, passID, level, track)
	if _, ok := row.claimed[slot]; ok {
		return PassClaim{}, ErrTaskFinished
	}
	if reward.Track == taskpb.PassTrack_PASS_TRACK_PREMIUM {
		switch season.PremiumLine {
		case taskpb.PassPremiumLine_PASS_PREMIUM_LINE_PAYMENT:
			if !row.premium {
				return PassClaim{}, ErrTaskNotReady
			}
		case taskpb.PassPremiumLine_PASS_PREMIUM_LINE_MEMBER:
			if b.memberLevelLocked(userID) < season.PremiumMemberLevel {
				return PassClaim{}, ErrTaskNotReady
			}
		default:
			return PassClaim{}, ErrTaskNotReady
		}
	}
	if row.claimed == nil {
		row.claimed = map[string]struct{}{}
	}
	row.claimed[slot] = struct{}{}
	b.passUsers[passUserKey{userID, passID}] = row
	claim := PassClaim{Level: level, Track: track, RewardTemplateID: reward.RewardTemplateID, RewardCount: reward.RewardCount}
	b.passClaims[requestID] = claim
	return claim, nil
}

// PassLevel 线性等级 = xp/XPPerLevel 整除封顶 MaxLevel（老项目 passLevel）
func PassLevel(season PassSeason, xp int64) uint32 {
	if xp <= 0 || season.XPPerLevel <= 0 {
		return 0
	}
	level := xp / season.XPPerLevel
	if level > int64(season.MaxLevel) {
		return season.MaxLevel
	}
	return uint32(level)
}
