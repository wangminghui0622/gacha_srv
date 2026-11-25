package logic

import (
	"context"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserBindingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserBindingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserBindingsLogic {
	return &GetUserBindingsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserBindingsLogic) GetUserBindings(in *identity.GetUserBindingsReq) (*identity.UserBindings, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenantId 须为 1（单租户）")
	}
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID不能为空")
	}
	if _, err := l.svcCtx.UserModel.FindOne(l.ctx, in.UserId); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeUserNotFound, "用户不存在")
	}
	payments, err := repo.ListPaymentBindings(l.svcCtx.DB.WithContext(l.ctx), in.UserId)
	if err != nil {
		return nil, err
	}
	realnames, err := repo.ListRealnameBindings(l.svcCtx.DB.WithContext(l.ctx), in.UserId)
	if err != nil {
		return nil, err
	}
	resp := &identity.UserBindings{
		Payments:  make([]*identity.PaymentBinding, 0, len(payments)),
		Realnames: make([]*identity.RealnameBinding, 0, len(realnames)),
	}
	for i := range payments {
		row := &payments[i]
		resp.Payments = append(resp.Payments, &identity.PaymentBinding{
			Id:                  row.ID,
			CreatedAtUnixMillis: row.CreatedAt.UnixMilli(),
			UserId:              row.UserID,
			Name:                row.Name,
			PaymentAccount:      row.PaymentAccount,
		})
	}
	for i := range realnames {
		row := &realnames[i]
		resp.Realnames = append(resp.Realnames, &identity.RealnameBinding{
			Id:                  row.ID,
			CreatedAtUnixMillis: row.CreatedAt.UnixMilli(),
			UserId:              row.UserID,
			Name:                row.Name,
			IdCard:              row.IDCard,
			Phone:               row.Phone,
		})
	}
	return resp, nil
}
