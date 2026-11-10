package logic

import (
	"context"
	"errors"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/svc"
	drawmodelnc "gacha_srv/pkg/model_nocache/gacha_draw"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetRecordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRecordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRecordLogic {
	return &GetRecordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetRecord 按请求单号查询抽赏记录（网关购买轮询用）。
// 普通 RPC（非 SAGA 分支）：业务错误直接返回 CodeError，ServerErrorInterceptor 透出业务码。
// 归属校验：记录 userId 与查询入参不符时同样按不存在处理，不泄露他人记录。
func (l *GetRecordLogic) GetRecord(in *draw.GetRecordReq) (*draw.GetRecordResp, error) {
	if in.RequestId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "requestId不能为空")
	}
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}

	rec, err := l.svcCtx.RecordModel.FindOneByRequestId(l.ctx, in.RequestId)
	if err != nil {
		if errors.Is(err, drawmodelnc.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeRecordNotFound, "抽赏记录不存在")
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeRecordNotFound, "抽赏记录不存在")
		}
		l.Errorf("GetRecord failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, errorx.FromMySQLError(err)
	}
	if rec.UserId != in.UserId {
		// 记录归属不符：按不存在处理（404 语义，防枚举他人单号）
		return nil, errorx.NewCodeError(errorx.CodeRecordNotFound, "抽赏记录不存在")
	}

	names, err := boxNames(l.ctx, l.svcCtx.DB)
	if err != nil {
		l.Errorf("GetRecord boxNames failed, err: %v", err)
		return nil, errorx.FromMySQLError(err)
	}
	return &draw.GetRecordResp{
		Record: &draw.RecordItem{
			RecordId:  rec.Id,
			RequestId: rec.RequestId,
			BoxId:     rec.BoxId,
			BoxName:   names[rec.BoxId],
			EntryName: rec.EntryName,
			IsPrize:   rec.IsPrize == 1,
			Price:     utils.FenToYuan(rec.Price),
			CreatedAt: rec.CreatedAt.Unix(),
			Status:    int32(rec.Status),
		},
	}, nil
}
