package logic

import (
	"context"
	"strings"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const maxUploadSize = 10 << 20 // 10 MiB

type UploadObjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUploadObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadObjectLogic {
	return &UploadObjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UploadObject 上传文件到对象存储（COS/OSS，管理后台图片等；返回可公开访问的 URL）。
func (l *UploadObjectLogic) UploadObject(in *thirdparty.UploadObjectReq) (*thirdparty.UploadObjectResp, error) {
	if in == nil || len(in.Data) == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "文件内容不能为空")
	}
	if len(in.Data) > maxUploadSize {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "文件不能超过 10MB")
	}
	if l.svcCtx.Storage == nil || !l.svcCtx.Storage.Enabled() {
		return nil, errorx.NewCodeError(errorx.CodeOSSNotConfigured, "对象存储未配置")
	}
	contentType := strings.TrimSpace(in.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	key := l.svcCtx.Storage.ObjectKey(in.Filename, in.Dir)
	publicURL, err := l.svcCtx.Storage.Upload(l.ctx, key, in.Data, contentType)
	if err != nil {
		l.Errorf("UploadObject failed, key: %s, size: %d, err: %v", key, len(in.Data), err)
		return nil, errorx.NewCodeError(errorx.CodeOSSUploadFailed, "对象存储上传失败")
	}
	return &thirdparty.UploadObjectResp{
		ObjectKey: key,
		Url:       publicURL,
		Size:      int64(len(in.Data)),
	}, nil
}
