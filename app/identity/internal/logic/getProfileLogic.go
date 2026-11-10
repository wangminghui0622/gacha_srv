package logic

import (
	"context"
	"errors"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProfileLogic {
	return &GetProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetProfile 查询用户资料（不返回密码哈希）
func (l *GetProfileLogic) GetProfile(in *identity.GetProfileReq) (*identity.GetProfileResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	u, err := l.svcCtx.UserModel.FindOne(l.ctx, in.UserId)
	if err != nil {
		if errors.Is(err, gacha_identity.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeUserNotFound, "用户不存在")
		}
		l.Errorf("GetProfile failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.FromMySQLError(err)
	}
	return &identity.GetProfileResp{
		UserId:   u.Id,
		Username: u.Username,
		Nickname: u.Nickname,
	}, nil
}
