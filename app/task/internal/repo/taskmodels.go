package repo

import "time"

// M15c task 玩家任务体系 8 表行（gacha_game；联合主键 goctl 不支持 → 手写 gorm 结构体，
// M14d 先例）。全部 nocache：行锁 FOR UPDATE + request_id/槽位幂等键写入。
// 事务由 logic 层开启并传入 tx；本文件只声明行形状。

type (
	// TaskProgressRow task_progress 表行：PK(user_id,task_id,period_key)
	TaskProgressRow struct {
		UserID       uint64    `gorm:"column:user_id;primaryKey"`
		TaskID       uint64    `gorm:"column:task_id;primaryKey"`
		PeriodKey    string    `gorm:"column:period_key;primaryKey"`
		Amount       int64     `gorm:"column:amount"`
		ClaimedCount uint32    `gorm:"column:claimed_count"`
		UpdatedAt    time.Time `gorm:"column:updated_at"`
	}

	// TaskEventRow task_event 表行：PK(event_id) 事件幂等去重
	TaskEventRow struct {
		EventID   string    `gorm:"column:event_id;primaryKey"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}

	// TaskClaimRow task_claim 表行：PK(request_id) 领取幂等重放原样回
	TaskClaimRow struct {
		RequestID        string    `gorm:"column:request_id;primaryKey"`
		UserID           uint64    `gorm:"column:user_id"`
		TaskID           uint64    `gorm:"column:task_id"`
		RewardTemplateID string    `gorm:"column:reward_template_id"`
		RewardCount      uint32    `gorm:"column:reward_count"`
		ClaimedCount     uint32    `gorm:"column:claimed_count"`
		CreatedAt        time.Time `gorm:"column:created_at"`
	}

	// TaskSignInRow task_sign_in 表行：PK(user_id,task_id)；乐观锁 WHERE last_date AND streak
	TaskSignInRow struct {
		UserID    uint64    `gorm:"column:user_id;primaryKey"`
		TaskID    uint64    `gorm:"column:task_id;primaryKey"`
		LastDate  string    `gorm:"column:last_date"`
		Streak    uint32    `gorm:"column:streak"`
		UpdatedAt time.Time `gorm:"column:updated_at"`
	}

	// TaskPassProgressRow task_pass_progress 表行：PK(user_id,pass_id)
	TaskPassProgressRow struct {
		UserID    uint64    `gorm:"column:user_id;primaryKey"`
		PassID    uint64    `gorm:"column:pass_id;primaryKey"`
		XP        int64     `gorm:"column:xp"`
		Premium   uint8     `gorm:"column:premium"`
		UpdatedAt time.Time `gorm:"column:updated_at"`
	}

	// TaskPassClaimRow task_pass_claim 表行：PK(request_id) + UK(user_id,pass_id,level,track)
	TaskPassClaimRow struct {
		RequestID        string    `gorm:"column:request_id;primaryKey"`
		UserID           uint64    `gorm:"column:user_id"`
		PassID           uint64    `gorm:"column:pass_id"`
		Level            uint32    `gorm:"column:level"`
		Track            uint8     `gorm:"column:track"`
		RewardTemplateID string    `gorm:"column:reward_template_id"`
		RewardCount      uint32    `gorm:"column:reward_count"`
		CreatedAt        time.Time `gorm:"column:created_at"`
	}

	// TaskMemberRow task_member 表行：PK(user_id)；spend 只增不减（饱和 1<<62）
	TaskMemberRow struct {
		UserID     uint64    `gorm:"column:user_id;primaryKey"`
		SpendMinor int64     `gorm:"column:spend_minor"`
		Level      uint32    `gorm:"column:level"`
		UpdatedAt  time.Time `gorm:"column:updated_at"`
	}

	// TaskMemberClaimRow task_member_claim 表行：PK(request_id) + UK(user_id,level)
	TaskMemberClaimRow struct {
		RequestID        string    `gorm:"column:request_id;primaryKey"`
		UserID           uint64    `gorm:"column:user_id"`
		Level            uint32    `gorm:"column:level"`
		RewardTemplateID string    `gorm:"column:reward_template_id"`
		RewardCount      uint32    `gorm:"column:reward_count"`
		CreatedAt        time.Time `gorm:"column:created_at"`
	}
)

func (TaskProgressRow) TableName() string     { return "task_progress" }
func (TaskEventRow) TableName() string        { return "task_event" }
func (TaskClaimRow) TableName() string        { return "task_claim" }
func (TaskSignInRow) TableName() string       { return "task_sign_in" }
func (TaskPassProgressRow) TableName() string { return "task_pass_progress" }
func (TaskPassClaimRow) TableName() string    { return "task_pass_claim" }
func (TaskMemberRow) TableName() string       { return "task_member" }
func (TaskMemberClaimRow) TableName() string  { return "task_member_claim" }
