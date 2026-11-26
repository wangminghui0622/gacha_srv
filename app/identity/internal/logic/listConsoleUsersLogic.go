package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"
	"gacha_srv/pkg/utils/publicid"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

const (
	listConsoleUsersDefaultPageSize = 15
	listConsoleUsersMaxPageSize     = 100
)

type ListConsoleUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListConsoleUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConsoleUsersLogic {
	return &ListConsoleUsersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListConsoleUsersLogic) ListConsoleUsers(in *identity.ListConsoleUsersReq) (*identity.ListConsoleUsersResp, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenantId 须为 1（单租户）")
	}
	page := int(in.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = listConsoleUsersDefaultPageSize
	}
	if pageSize > listConsoleUsersMaxPageSize {
		pageSize = listConsoleUsersMaxPageSize
	}

	var phoneUserID int64
	if phone := strings.TrimSpace(in.Phone); phone != "" {
		id, err := l.svcCtx.PhoneRepo.FindExact(l.ctx, in.TenantId, phone)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &identity.ListConsoleUsersResp{Items: []*identity.ConsoleUser{}, Total: 0}, nil
			}
			return nil, err
		}
		phoneUserID = id
	}

	query := l.svcCtx.DB.WithContext(l.ctx).
		Model(&gacha_identity.UserUsers{}).
		Joins("LEFT JOIN user_profiles p ON p.user_id = user_users.id")

	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		switch {
		case strings.HasPrefix(keyword, publicid.UserPrefix):
			// 对外 ID 以库中 public_id 为准，不再依赖可逆解码（支持密钥轮换）
			query = query.Where("(user_users.public_id = ? OR user_users.nickname LIKE ? OR user_users.username LIKE ?)", keyword, like, like)
		default:
			if id, err := strconv.ParseInt(keyword, 10, 64); err == nil && id > 0 {
				query = query.Where("(user_users.id = ? OR user_users.nickname LIKE ? OR user_users.username LIKE ?)", id, like, like)
			} else {
				query = query.Where("(user_users.nickname LIKE ? OR user_users.username LIKE ?)", like, like)
			}
		}
	}
	if phoneUserID > 0 {
		query = query.Where("user_users.id = ?", phoneUserID)
	}
	if identityName := strings.TrimSpace(in.Identity); identityName != "" {
		query = query.Where("COALESCE(p.identity, '用户') = ?", identityName)
	}
	if in.Status > 0 {
		query = query.Where("COALESCE(p.status, 1) = ?", in.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var users []gacha_identity.UserUsers
	if err := query.Order("user_users.id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&users).Error; err != nil {
		return nil, err
	}

	userIDs := make([]int64, 0, len(users))
	for i := range users {
		userIDs = append(userIDs, users[i].Id)
	}
	profiles, err := repo.FetchUserProfiles(l.svcCtx.DB.WithContext(l.ctx), userIDs)
	if err != nil {
		return nil, err
	}

	items := make([]*identity.ConsoleUser, 0, len(users))
	for i := range users {
		user := &users[i]
		phone, err := l.svcCtx.PhoneRepo.Get(l.ctx, in.TenantId, user.Id)
		if err != nil {
			return nil, err
		}
		profile, ok := profiles[user.Id]
		if !ok {
			profile = repo.UserProfile{
				UserID:       user.Id,
				Status:       1,
				LuckyDisplay: "初始化",
				RebateRate:   10,
				Identity:     "用户",
			}
		}
		if profile.Status == 0 {
			profile.Status = 1
		}
		if profile.LuckyDisplay == "" {
			profile.LuckyDisplay = "初始化"
		}
		if profile.Identity == "" {
			profile.Identity = "用户"
		}
		items = append(items, &identity.ConsoleUser{
			UserId:              user.Id,
			PublicUserId:        publicUserIdOfUser(user),
			Username:            user.Username,
			Nickname:            user.Nickname,
			Phone:               phone,
			AvatarUrl:           profile.AvatarURL,
			Status:              profile.Status,
			LuckyDisplay:        profile.LuckyDisplay,
			RegisterIp:          profile.RegisterIP,
			RegisterDevice:      profile.RegisterDevice,
			RebateRate:          profile.RebateRate,
			Identity:            profile.Identity,
			ParentUserId:        profile.ParentUserID,
			ReissueLuckPoints:   profile.ReissueLuckPoints == 1,
			CreatedAtUnixMillis: user.CreatedAt.UnixMilli(),
			UpdatedAtUnixMillis: user.UpdatedAt.UnixMilli(),
		})
	}
	return &identity.ListConsoleUsersResp{Items: items, Total: uint64(total)}, nil
}
