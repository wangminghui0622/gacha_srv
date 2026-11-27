package logic

import "gacha_srv/app/identity/internal/config"

func inviteRewardPoints(c config.Config) int64 {
	if c.Invite.RewardPoints > 0 {
		return c.Invite.RewardPoints
	}
	return 10
}

func inviteRewardValidDays(c config.Config) int64 {
	if c.Invite.RewardValidDays > 0 {
		return c.Invite.RewardValidDays
	}
	return 10
}
