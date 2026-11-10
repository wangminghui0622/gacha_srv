package task

import "gacha_srv/app/ops/ops"

type MemberTier struct {
	Level            uint32
	SpendMinor       int64
	RewardTemplateID string
	RewardCount      uint32
	Reward           *ops.TaskRewardDefinition
}

type memberState struct {
	spend   int64
	claimed map[uint32]struct{}
}

func (b *Board) SetMemberPlan(tiers []MemberTier) error {
	if len(tiers) == 0 || len(tiers) > 50 {
		return ErrInvalidTask
	}
	var previous int64
	seen := map[uint32]struct{}{}
	copied := make([]MemberTier, len(tiers))
	for i, tier := range tiers {
		if tier.Level == 0 || tier.SpendMinor <= previous || !validReward(tier.Reward, tier.RewardTemplateID, tier.RewardCount) {
			return ErrInvalidTask
		}
		if _, ok := seen[tier.Level]; ok {
			return ErrInvalidTask
		}
		seen[tier.Level] = struct{}{}
		previous = tier.SpendMinor
		copied[i] = tier
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.memberTiers = copied
	if b.members == nil {
		b.members = map[uint64]memberState{}
		b.memberEvents = map[string]struct{}{}
	}
	return nil
}

func (b *Board) creditMemberLocked(userID uint64, amount int64, eventID string) {
	if len(b.memberTiers) == 0 || amount <= 0 {
		return
	}
	if _, ok := b.memberEvents[eventID]; ok {
		return
	}
	row := b.members[userID]
	if row.spend > (1<<62)-amount {
		row.spend = 1 << 62
	} else {
		row.spend += amount
	}
	if row.claimed == nil {
		row.claimed = map[uint32]struct{}{}
	}
	b.members[userID] = row
	b.memberEvents[eventID] = struct{}{}
}

func (b *Board) MemberLevel(userID uint64) (uint32, int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	row := b.members[userID]
	return b.memberLevelLocked(userID), row.spend
}

func (b *Board) memberLevelLocked(userID uint64) uint32 {
	row := b.members[userID]
	level := uint32(0)
	for _, tier := range b.memberTiers {
		if row.spend >= tier.SpendMinor && tier.Level > level {
			level = tier.Level
		}
	}
	return level
}

// MemberLevelForTiers 最高满足档（老项目 memberLevelLocked 的纯函数形态；
// 存储模式 task_member.level 由 SaveApply 落库，视图直接读行）
func MemberLevelForTiers(tiers []MemberTier, spend int64) uint32 {
	level := uint32(0)
	for _, tier := range tiers {
		if spend >= tier.SpendMinor && tier.Level > level {
			level = tier.Level
		}
	}
	return level
}

// ClaimMember 会员等级奖励领取（内存模式）
func (b *Board) ClaimMember(userID uint64, level uint32, requestID string) (Claim, error) {
	if userID == 0 || level == 0 || requestID == "" || len(requestID) > 64 {
		return Claim{}, ErrInvalidTask
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var tier MemberTier
	found := false
	for _, item := range b.memberTiers {
		if item.Level == level {
			tier = item
			found = true
			break
		}
	}
	if !found {
		return Claim{}, ErrTaskNotFound
	}
	if existing, ok := b.memberClaims[requestID]; ok {
		return existing, nil
	}
	row := b.members[userID]
	if row.spend < tier.SpendMinor {
		return Claim{}, ErrTaskNotReady
	}
	if row.claimed == nil {
		row.claimed = map[uint32]struct{}{}
	}
	if _, ok := row.claimed[level]; ok {
		return Claim{}, ErrTaskFinished
	}
	row.claimed[level] = struct{}{}
	b.members[userID] = row
	claim := Claim{RewardTemplateID: tier.RewardTemplateID, RewardCount: tier.RewardCount, Claimed: level}
	if b.memberClaims == nil {
		b.memberClaims = map[string]Claim{}
	}
	b.memberClaims[requestID] = claim
	return claim, nil
}
