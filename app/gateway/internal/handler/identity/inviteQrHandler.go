package identity

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/skip2/go-qrcode"
)

var inviteCodePattern = regexp.MustCompile(`^[A-Za-z0-9]{4,32}$`)

// InviteQrHandler 邀请二维码图片（公开）：GET /invite/qrcode?code=xxxx
func InviteQrHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if !inviteCodePattern.MatchString(code) {
			http.Error(w, "invalid invite code", http.StatusBadRequest)
			return
		}
		content := "https://jixiangshang.app/invite?code=" + url.QueryEscape(code)
		png, err := qrcode.Encode(content, qrcode.High, 512)
		if err != nil {
			http.Error(w, "qrcode encode failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(png)
	}
}
