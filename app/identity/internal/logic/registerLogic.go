package logic

import (
	"context"
	"errors"
	"regexp"
	"time"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Register 注册：仅创建用户账号。余额行由 money 侧懒初始化
// （getBalance/recharge/creditWallet 对 user_balances upsert 兜底），
// 避免 identity 跨库写 gacha_money，见 docs/design/service-division.md §3.6。
func (l *RegisterLogic) Register(in *identity.RegisterReq) (*identity.RegisterResp, error) {
	if !usernameRe.MatchString(in.Username) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户名需 3-32 位字母数字下划线")
	}
	if len(in.Password) < 6 || len(in.Password) > 32 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "密码长度需 6-32 位")
	}
	nickname := in.Nickname
	if nickname == "" {
		nickname = in.Username
	}
	if len(nickname) > 32 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "昵称最长 32 字")
	}

	// bcrypt 加密存储，不落明文
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("Register bcrypt failed: %v", err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "密码加密失败")
	}

	now := time.Now()
	var userId int64
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		u := &gacha_identity.UserUsers{
			Username:  in.Username,
			Password:  string(hash),
			Nickname:  nickname,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := l.svcCtx.UserModel.Insert(l.ctx, tx, u); err != nil {
			// uk_username 冲突（含并发竞态）→ 用户名已存在
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errorx.NewCodeError(errorx.CodeUsernameExists, "用户名已存在")
			}
			return err
		}
		userId = u.Id
		return nil
	})
	if err != nil {
		if errorx.IsCodeError(err) {
			return nil, err
		}
		l.Errorf("Register failed, username: %s, err: %v", in.Username, err)
		return nil, errorx.FromMySQLError(err)
	}
	l.Infof("Register success, username: %s, userId: %d", in.Username, userId)
	return &identity.RegisterResp{UserId: userId}, nil
}
