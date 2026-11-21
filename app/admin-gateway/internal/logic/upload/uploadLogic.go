package upload

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"

	"github.com/zeromicro/go-zero/core/logx"
)

type UploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadLogic {
	return &UploadLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UploadLogic) Upload(req *thirdparty.UploadObjectReq) (*thirdparty.UploadObjectResp, error) {
	return l.svcCtx.ThirdpartyRpc.UploadObject(l.ctx, req)
}
