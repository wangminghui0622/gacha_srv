// e2e 工具：微信本地 mock server（场景 29/32 用，thirdparty-rpc yaml
// Wechat.Mini.APIBase / App.OAuthBase 指向 http://127.0.0.1:9410）。
// 契约：
//   - /sns/jscode2session、/sns/oauth2/access_token：openid=code 本身；code=err → errcode=40029
//   - /cgi-bin/token：appid=err → errcode=40013；否则 {access_token:"dev-mini-token",expires_in:7200}
//   - /wxa/business/getuserphonenumber：code=err → errcode=40029；code 前缀 retry- →
//     首次 errcode=40001（触发 40001 清 token 重试流）、二次成功；成功回 13800001111/86
//   - /wxa/getwxacodeunlimit：scene=errqr → errcode=43002 JSON；否则 PNG 魔数图片
//   - /wxa/generate_urllink：path=errlink → errcode=41002；否则回 url_link
//   - GET /__stats 各端点调用计数；GET /__reset 清零（场景断言缓存/重试用）
// 端口占用时自动退出（run_all 串行场景间不泄漏进程）。
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
)

// 端点调用计数（场景断言：token 本地缓存 → token_calls 不增；40001 重试 → phone_calls>=2）
var stats struct {
	sync.Mutex
	token, phone, qr, urllink int
	retrySeen                  map[string]bool // retry-* code 是否已失败过一次（40001 重试流）
}

func main() {
	addr := "127.0.0.1:9410"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("wechatmock listen %s failed: %v\n", addr, err)
		os.Exit(1)
	}
	http.HandleFunc("/sns/jscode2session", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("js_code")
		writeJSON(w, exchangeReply(code, "dev-union-1", "dev-sk-"+code))
	})
	http.HandleFunc("/sns/oauth2/access_token", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		reply := exchangeReply(code, "dev-union-app", "")
		reply["access_token"] = "dev-at-" + code
		writeJSON(w, reply)
	})
	http.HandleFunc("/cgi-bin/token", func(w http.ResponseWriter, r *http.Request) {
		stats.Lock()
		stats.token++
		stats.Unlock()
		if r.URL.Query().Get("appid") == "err" {
			writeJSON(w, map[string]any{"errcode": 40013, "errmsg": "invalid appid"})
			return
		}
		writeJSON(w, map[string]any{"access_token": "dev-mini-token", "expires_in": 7200})
	})
	http.HandleFunc("/wxa/business/getuserphonenumber", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Code string `json:"code"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		stats.Lock()
		stats.phone++
		retried := stats.retrySeen[body.Code]
		if stats.retrySeen == nil {
			stats.retrySeen = map[string]bool{}
		}
		stats.retrySeen[body.Code] = true
		stats.Unlock()
		switch {
		case body.Code == "err":
			writeJSON(w, map[string]any{"errcode": 40029, "errmsg": "invalid code"})
		case len(body.Code) >= len("retry-") && body.Code[:len("retry-")] == "retry-" && !retried:
			// 同一 code 首次 40001、二次成功（客户端清 token 重试一次后成功）
			writeJSON(w, map[string]any{"errcode": 40001, "errmsg": "access_token expired"})
		default:
			writeJSON(w, phoneReply(body.Code))
		}
	})
	http.HandleFunc("/wxa/getwxacodeunlimit", func(w http.ResponseWriter, r *http.Request) {
		stats.Lock()
		stats.qr++
		stats.Unlock()
		var body struct {
			Scene string `json:"scene"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Scene == "errqr" {
			writeJSON(w, map[string]any{"errcode": 43002, "errmsg": "invalid scene"})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes())
	})
	http.HandleFunc("/wxa/generate_urllink", func(w http.ResponseWriter, r *http.Request) {
		stats.Lock()
		stats.urllink++
		stats.Unlock()
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Path == "errlink" {
			writeJSON(w, map[string]any{"errcode": 41002, "errmsg": "invalid path"})
			return
		}
		writeJSON(w, map[string]any{"errcode": 0, "errmsg": "ok", "url_link": "https://wxaurl.cn/dev-link"})
	})
	http.HandleFunc("/__stats", func(w http.ResponseWriter, r *http.Request) {
		stats.Lock()
		defer stats.Unlock()
		writeJSON(w, map[string]any{"token": stats.token, "phone": stats.phone, "qr": stats.qr, "urllink": stats.urllink})
	})
	http.HandleFunc("/__reset", func(w http.ResponseWriter, r *http.Request) {
		stats.Lock()
		stats.token, stats.phone, stats.qr, stats.urllink = 0, 0, 0, 0
		stats.retrySeen = map[string]bool{}
		stats.Unlock()
		writeJSON(w, map[string]any{"ok": true})
	})
	fmt.Printf("wechatmock listening on %s\n", addr)
	if err := http.Serve(ln, nil); err != nil {
		fmt.Printf("wechatmock stopped: %v\n", err)
		os.Exit(1)
	}
}

// exchangeReply openid=code；code=err → 微信 errcode（→ thirdparty 报 9006）
func exchangeReply(code, unionID, sessionKey string) map[string]any {
	if code == "err" {
		return map[string]any{"errcode": 40029, "errmsg": "invalid code"}
	}
	reply := map[string]any{"openid": code, "unionid": unionID}
	if sessionKey != "" {
		reply["session_key"] = sessionKey
	}
	return reply
}

// phoneReply 手机号成功应答：purePhoneNumber 优先字段照抄微信（场景断言用）
func phoneReply(code string) map[string]any {
	phone := "13800001111"
	if len(code) == 11 {
		phone = code
	}
	return map[string]any{"errcode": 0, "errmsg": "ok", "phone_info": map[string]any{
		"phoneNumber": phone, "purePhoneNumber": phone, "countryCode": "86", "watermark": map[string]any{"timestamp": 1, "appid": "dev"},
	}}
}

// pngBytes PNG 魔数 + 最小合法头（场景断言前 8 字节魔数）
func pngBytes() []byte {
	return []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 13, 'I', 'H', 'D', 'R', 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89}
}

func writeJSON(w http.ResponseWriter, v map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
