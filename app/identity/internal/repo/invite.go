package repo

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"gorm.io/gorm"
)

const inviteAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// InviteRecordRow 邀请关系行。
type InviteRecordRow struct {
	InviteeUserID int64      `gorm:"column:invitee_user_id"`
	InviterUserID int64      `gorm:"column:inviter_user_id"`
	InviteCode    string     `gorm:"column:invite_code"`
	RewardPoints  int64      `gorm:"column:reward_points"`
	Status        int32      `gorm:"column:status"`
	BoundAt       time.Time  `gorm:"column:bound_at"`
	RewardAt      *time.Time `gorm:"column:reward_at"`
	ExpiresAt     time.Time  `gorm:"column:expires_at"`
}

// InviteStats 邀请统计。
type InviteStats struct {
	InvitedCount  int64 `gorm:"column:invited_count"`
	RewardPoints  int64 `gorm:"column:reward_points"`
	PendingPoints int64 `gorm:"column:pending_points"`
}

// InviteRecordView 邀请记录（带被邀请人展示信息）。
type InviteRecordView struct {
	InviteeUserID int64      `gorm:"column:invitee_user_id"`
	PublicID      string     `gorm:"column:public_id"`
	Nickname      string     `gorm:"column:nickname"`
	RewardPoints  int64      `gorm:"column:reward_points"`
	Status        int32      `gorm:"column:status"`
	BoundAt       time.Time  `gorm:"column:bound_at"`
	RewardAt      *time.Time `gorm:"column:reward_at"`
	ExpiresAt     time.Time  `gorm:"column:expires_at"`
}

func randomInviteCode(length int) (string, error) {
	if length <= 0 {
		length = 12
	}
	out := make([]byte, length)
	max := big.NewInt(int64(len(inviteAlphabet)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = inviteAlphabet[n.Int64()]
	}
	return string(out), nil
}

// FindInviteCode 查询用户邀请码；不存在返回 sql.ErrNoRows。
func FindInviteCode(db *gorm.DB, userID int64) (string, error) {
	var code string
	err := db.Raw("SELECT code FROM user_invite_codes WHERE user_id = ?", userID).Row().Scan(&code)
	if err != nil {
		return "", err
	}
	return code, nil
}

// EnsureInviteCode 确保用户有邀请码；首次访问时生成。
func EnsureInviteCode(db *gorm.DB, userID int64, length int) (string, error) {
	if code, err := FindInviteCode(db, userID); err == nil {
		return code, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	for i := 0; i < 8; i++ {
		code, err := randomInviteCode(length)
		if err != nil {
			return "", err
		}
		if err := db.Exec(
			"INSERT INTO user_invite_codes (user_id, code) VALUES (?, ?)",
			userID, code,
		).Error; err != nil {
			// 并发下用户码已存在，或随机码撞唯一键；回查用户码后重试
			if existing, findErr := FindInviteCode(db, userID); findErr == nil {
				return existing, nil
			}
			continue
		}
		return code, nil
	}
	return "", fmt.Errorf("generate invite code: too many collisions")
}

// FindInviterByCode 按邀请码查邀请人；不存在返回 sql.ErrNoRows。
func FindInviterByCode(db *gorm.DB, code string) (int64, error) {
	var userID int64
	err := db.Raw("SELECT user_id FROM user_invite_codes WHERE code = ?", code).Row().Scan(&userID)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

// GetInviteRecord 查询被邀请人的邀请关系；不存在返回 nil, nil。
func GetInviteRecord(db *gorm.DB, inviteeUserID int64) (*InviteRecordRow, error) {
	var rows []InviteRecordRow
	if err := db.Raw(
		"SELECT invitee_user_id, inviter_user_id, invite_code, reward_points, status, bound_at, reward_at, expires_at FROM user_invite_records WHERE invitee_user_id = ?",
		inviteeUserID,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// CreateInviteRecord 创建邀请关系，并同步 user_profiles.parent_user_id（仅在为空时写入）。
func CreateInviteRecord(db *gorm.DB, record InviteRecordRow) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			`INSERT INTO user_invite_records
			 (invitee_user_id, inviter_user_id, invite_code, reward_points, status, bound_at, expires_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			record.InviteeUserID, record.InviterUserID, record.InviteCode,
			record.RewardPoints, record.Status, record.BoundAt, record.ExpiresAt,
		).Error; err != nil {
			return err
		}
		return tx.Exec(
			`INSERT INTO user_profiles (user_id, parent_user_id) VALUES (?, ?)
			 ON DUPLICATE KEY UPDATE parent_user_id = IF(parent_user_id = 0, VALUES(parent_user_id), parent_user_id)`,
			record.InviteeUserID, record.InviterUserID,
		).Error
	})
}

// ListPendingInvites 查询邀请人名下待领取的邀请记录。
func ListPendingInvites(db *gorm.DB, inviterUserID int64) ([]InviteRecordRow, error) {
	var rows []InviteRecordRow
	if err := db.Raw(
		`SELECT invitee_user_id, inviter_user_id, invite_code, reward_points, status, bound_at, reward_at, expires_at
		 FROM user_invite_records
		 WHERE inviter_user_id = ? AND status = 0
		 ORDER BY bound_at ASC`,
		inviterUserID,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// MarkInviteStatus 更新邀请记录状态。
func MarkInviteStatus(db *gorm.DB, inviteeUserID int64, status int32, rewardAt *time.Time) error {
	if rewardAt == nil {
		return db.Exec(
			"UPDATE user_invite_records SET status = ? WHERE invitee_user_id = ?",
			status, inviteeUserID,
		).Error
	}
	return db.Exec(
		"UPDATE user_invite_records SET status = ?, reward_at = ? WHERE invitee_user_id = ?",
		status, *rewardAt, inviteeUserID,
	).Error
}

// GetInviteStats 统计邀请人数、已发放和待领取吉祥值。
func GetInviteStats(db *gorm.DB, inviterUserID int64) (InviteStats, error) {
	var stats InviteStats
	err := db.Raw(
		`SELECT COUNT(*) AS invited_count,
		        COALESCE(SUM(CASE WHEN status = 1 THEN reward_points ELSE 0 END), 0) AS reward_points,
		        COALESCE(SUM(CASE WHEN status = 0 AND expires_at > NOW() THEN reward_points ELSE 0 END), 0) AS pending_points
		 FROM user_invite_records
		 WHERE inviter_user_id = ?`,
		inviterUserID,
	).Scan(&stats).Error
	return stats, err
}

// ListInviteRecords 查询邀请记录（最多 200 条）。
func ListInviteRecords(db *gorm.DB, inviterUserID int64) ([]InviteRecordView, error) {
	var rows []InviteRecordView
	err := db.Raw(
		`SELECT r.invitee_user_id, r.reward_points, r.status, r.bound_at, r.reward_at, r.expires_at,
		        COALESCE(u.public_id, '') AS public_id, COALESCE(u.nickname, '') AS nickname
		 FROM user_invite_records r
		 LEFT JOIN user_users u ON u.id = r.invitee_user_id
		 WHERE r.inviter_user_id = ?
		 ORDER BY r.bound_at DESC
		 LIMIT 200`,
		inviterUserID,
	).Scan(&rows).Error
	return rows, err
}
