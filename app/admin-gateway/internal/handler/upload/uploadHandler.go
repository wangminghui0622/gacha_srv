package upload

import (
	"io"
	"net/http"
	"strings"

	"gacha_srv/app/admin-gateway/internal/logic/upload"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/pkg/utils/errorx"
)

const maxUploadSize = 10 << 20 // 10 MiB

// UploadHandler 管理后台上传文件到 OSS（gacha.pool:write；multipart/form-data，字段名 file）。
func UploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1)
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "上传内容无效或超过 10MB"))
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "缺少文件字段 file"))
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
		if err != nil {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "读取上传文件失败"))
			return
		}
		if len(data) == 0 {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "文件内容不能为空"))
			return
		}
		if len(data) > maxUploadSize {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "文件不能超过 10MB"))
			return
		}
		contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		if !strings.HasPrefix(contentType, "image/") {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "仅支持上传图片文件"))
			return
		}
		dir := strings.TrimSpace(r.FormValue("dir"))
		if dir == "" {
			dir = "gacha/match"
		}
		resp, err := upload.NewUploadLogic(r.Context(), svcCtx).Upload(&thirdparty.UploadObjectReq{
			Filename:    header.Filename,
			ContentType: contentType,
			Data:        data,
			Dir:         dir,
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
