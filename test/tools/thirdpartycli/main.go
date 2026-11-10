// e2e 工具：thirdparty-rpc（9317）8 个 M15d RPC 的 grpc 直连调用（场景 32 用）。
// 用法：go run test/tools/thirdpartycli/main.go <cmd> [args...]
//   wechat-token <tenantId>                      → access_token=<v>
//   wechat-phone <tenantId> <phoneCode>          → phone_number=<v> country_code=<v>
//   wechat-qr <tenantId> <scene> [page] [width]  → qr_bytes=<n> qr_magic=<hex 前8字节>
//   wechat-urllink <tenantId> <path> [query]     → url_link=<v>
//   alipay-token <grant=auth|refresh> <code|token> → user_id=<v> access_token=<v> refresh_token=<v> open_id=<v> union_id=<v>
//   alipay-userinfo <accessToken>                → open_id=<v> avatar=<v> nick_name=<v>
//   tx-ad-click <tenantId> <openId> [clickId]    → ok
//   ingest <tenantId> <requestId> <idemKey> <provider> <account> <metric=1..5> <value> <revision> [currency] [dimension] [seconds] [objectId]
//      → batch_id=<v> accepted=<v> stale=<v> replayed=<bool>
//   ingest-json <file>：任意请求体 JSON（多样本批/负例），输出同上
// 业务错误（业务码即 gRPC status code，1000-8999 段）按 `code=NNNN msg=...`
// 打印并退出 1（场景 grep 断言）；传输错误打印 ERR 并退出 2。
// 目标地址：$THIRDPARTY_RPC_ADDR，默认 127.0.0.1:9317。
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"gacha_srv/app/thirdparty/thirdparty"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: thirdpartycli <cmd> [args...]")
		os.Exit(2)
	}
	addr := os.Getenv("THIRDPARTY_RPC_ADDR")
	if addr == "" {
		addr = "127.0.0.1:9317"
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("ERR 连接 thirdparty-rpc 失败: %v\n", err)
		os.Exit(2)
	}
	defer conn.Close()
	cli := thirdparty.NewThirdpartyClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	run(ctx, cli, os.Args[1], os.Args[2:])
}

// printErr 业务错误码即 gRPC status code（ServerErrorInterceptor：CodeError → codes.Code(业务码)）：
// 业务码段输出 code=XXXX msg=... 一行并退出 1（场景可 grep code=）；传输错误打印 ERR 并退出 2。
func printErr(err error) {
	if s, ok := status.FromError(err); ok {
		c := int(s.Code())
		if c >= 1000 && c <= 8999 {
			fmt.Printf("code=%d msg=%s\n", c, s.Message())
			os.Exit(1)
		}
	}
	fmt.Printf("ERR %v\n", err)
	os.Exit(2)
}

func run(ctx context.Context, cli thirdparty.ThirdpartyClient, cmd string, args []string) {
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	argInt := func(i int) int64 {
		n, _ := strconv.ParseInt(arg(i), 10, 64)
		return n
	}
	switch cmd {
	case "wechat-token":
		resp, err := cli.GetWeChatMiniProgramAccessToken(ctx, &thirdparty.GetWeChatMiniProgramAccessTokenReq{TenantId: argInt(0)})
		if err != nil {
			printErr(err)
			return
		}
		fmt.Printf("access_token=%s\n", resp.GetAccessToken())
	case "wechat-phone":
		resp, err := cli.GetWeChatMiniProgramPhoneNumber(ctx, &thirdparty.GetWeChatMiniProgramPhoneNumberReq{TenantId: argInt(0), PhoneCode: arg(1)})
		if err != nil {
			printErr(err)
			return
		}
		fmt.Printf("phone_number=%s country_code=%s\n", resp.GetPhoneNumber(), resp.GetCountryCode())
	case "wechat-qr":
		width := uint32(430)
		if arg(3) != "" {
			width = uint32(argInt(3))
		}
		resp, err := cli.GenerateWeChatMiniProgramQrCode(ctx, &thirdparty.GenerateWeChatMiniProgramQrCodeReq{
			TenantId: argInt(0), Scene: arg(1), Page: arg(2), Width: width,
		})
		if err != nil {
			printErr(err)
			return
		}
		png := resp.GetImagePng()
		magic := ""
		if len(png) >= 8 {
			magic = fmt.Sprintf("%x", png[:8])
		}
		fmt.Printf("qr_bytes=%d qr_magic=%s\n", len(png), magic)
	case "wechat-urllink":
		resp, err := cli.GenerateWeChatMiniProgramUrlLink(ctx, &thirdparty.GenerateWeChatMiniProgramUrlLinkReq{
			TenantId: argInt(0), Path: arg(1), Query: arg(2),
		})
		if err != nil {
			printErr(err)
			return
		}
		fmt.Printf("url_link=%s\n", resp.GetUrlLink())
	case "alipay-token":
		grant := thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_UNSPECIFIED
		if arg(0) == "auth" {
			grant = thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_AUTHORIZATION_CODE
		} else if arg(0) == "refresh" {
			grant = thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_REFRESH_TOKEN
		}
		resp, err := cli.GetAliPayOauthToken(ctx, &thirdparty.GetAliPayOauthTokenReq{GrantType: grant, AuthCode: arg(1), RefreshToken: arg(1)})
		if err != nil {
			printErr(err)
			return
		}
		fmt.Printf("user_id=%s access_token=%s refresh_token=%s open_id=%s union_id=%s\n",
			resp.GetUserId(), resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetOpenId(), resp.GetUnionId())
	case "alipay-userinfo":
		resp, err := cli.GetAliPayUserInfo(ctx, &thirdparty.GetAliPayUserInfoReq{AccessToken: arg(0)})
		if err != nil {
			printErr(err)
			return
		}
		fmt.Printf("open_id=%s avatar=%s nick_name=%s\n", resp.GetOpenId(), resp.GetAvatar(), resp.GetNickName())
	case "tx-ad-click":
		_, err := cli.TxAdClick(ctx, &thirdparty.TxAdClickReq{
			TenantId: argInt(0), WechatOpenId: arg(1), ClickId: arg(2),
		})
		if err != nil {
			printErr(err)
			return
		}
		fmt.Println("ok")
	case "ingest":
		metric := thirdparty.StandardMetric(argInt(5))
		currency := arg(8)
		if currency == "" && (metric == thirdparty.StandardMetric_STANDARD_METRIC_SPEND || metric == thirdparty.StandardMetric_STANDARD_METRIC_REVENUE) {
			currency = "CNY"
		}
		dimension := arg(9)
		if dimension == "" {
			dimension = "all"
		}
		seconds := uint32(argInt(10))
		if seconds == 0 {
			seconds = 3600
		}
		objectID := arg(11)
		if objectID == "" {
			objectID = "obj-1"
		}
		// 桶起点：固定锚点 2026-10-23T08:00Z（对齐小时），revision 不同仍同槽（仲裁测试）
		resp, err := cli.IngestProviderMetrics(ctx, &thirdparty.IngestProviderMetricsReq{
			TenantId: argInt(0), RequestId: arg(1), IdempotencyKey: arg(2),
			ProviderCode: arg(3), ExternalAccountId: arg(4),
			Samples: []*thirdparty.ProviderMetricSample{{
				ProviderMetricCode: "impressions", StandardMetric: metric, ExternalObjectId: objectID,
				BucketStart: timestamppb.New(time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)),
				BucketSeconds: seconds, DimensionKey: dimension,
				Value: argInt(6), Currency: currency, SourceRevision: uint64(argInt(7)),
			}},
		})
		if err != nil {
			printErr(err)
			return
		}
		fmt.Printf("batch_id=%d accepted=%d stale=%d replayed=%v\n",
			resp.GetBatchId(), resp.GetAcceptedCount(), resp.GetStaleCount(), resp.GetReplayed())
	case "ingest-json":
		// 任意请求体（多样本/负例），文件路径经 os.Args 传入
		raw, err := os.ReadFile(arg(0))
		if err != nil {
			fmt.Printf("ERR 读取 %s: %v\n", arg(0), err)
			os.Exit(2)
		}
		req := new(thirdparty.IngestProviderMetricsReq)
		if err = protojson.Unmarshal(raw, req); err != nil {
			fmt.Printf("ERR JSON 解析: %v\n", err)
			os.Exit(2)
		}
		resp, err := cli.IngestProviderMetrics(ctx, req)
		if err != nil {
			printErr(err)
			return
		}
		fmt.Printf("batch_id=%d accepted=%d stale=%d replayed=%v\n",
			resp.GetBatchId(), resp.GetAcceptedCount(), resp.GetStaleCount(), resp.GetReplayed())
	default:
		fmt.Printf("ERR 未知命令 %s\n", cmd)
		os.Exit(2)
	}
}
