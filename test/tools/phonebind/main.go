// e2e 工具：M15a 手机号/第三方身份 grpc 直连调用（场景 29 断言用）。
// 用法：
//   go run test/tools/phonebind/main.go captcha <tenant> <phone>          → 发验证码（user-rpc → thirdparty-rpc）
//   go run test/tools/phonebind/main.go devcode <tenant> <phone>          → DryRun 下从 Redis 读码（须先 captcha）
//   go run test/tools/phonebind/main.go login <tenant> <phone> <code>     → 验证码登录 → userId/token
//   go run test/tools/phonebind/main.go set <tenant> <user> <phone>       → 绑定/改绑手机号
//   go run test/tools/phonebind/main.go find <tenant> <phone>             → 精确查绑定 → userId
//   go run test/tools/phonebind/main.go search <tenant> <fragment>        → 片段搜索 → 掩码命中列表
//   go run test/tools/phonebind/main.go link <tenant> <user> <kind> <openId> [unionId]
//   go run test/tools/phonebind/main.go resolve <tenant> <kind> <openId>  → 按 openid 查用户
//   go run test/tools/phonebind/main.go get <tenant> <user> <kind>        → 查用户第三方身份
// kind: 1=微信小程序 2=微信App。业务错误输出 code=XXXX msg=... 行并退出 1，
// 标准 gRPC 码映射回项目标准码（400/401/403/404/408/429，同 errorx.mapGRPCCode 语义），其余 grpc 错误退出 2。
package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"gacha_srv/app/identity/identity"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	conn, err := grpc.NewClient("127.0.0.1:9311", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("ERR 连接 user-rpc 失败: %v\n", err)
		os.Exit(2)
	}
	defer conn.Close()
	cli := identity.NewIdentityClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := os.Args[1]
	switch cmd {
	case "clear":
		// 清频控+验证码（开发/场景测试复位用；生产无此入口）
		tenant, err1 := strconv.ParseInt(os.Args[2], 10, 64)
		if err1 != nil {
			fmt.Printf("ERR tenant 非法: %v\n", err1)
			os.Exit(2)
		}
		rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: os.Getenv("REDIS_PASS")})
		defer rdb.Close()
		for _, candidate := range []string{os.Args[3], "+86" + os.Args[3]} {
			sum := md5.Sum([]byte(candidate))
			_ = rdb.Del(ctx, fmt.Sprintf("luckybox:sms:captcha:%d:%s", tenant, hex.EncodeToString(sum[:]))).Err()
			_ = rdb.Del(ctx, fmt.Sprintf("luckybox:sms:captcha:frequency:%s", hex.EncodeToString(sum[:]))).Err()
		}
		fmt.Println("cleared")
	case "captcha", "devcode", "login":
		if len(os.Args) < 4 {
			usage()
			os.Exit(2)
		}
		tenant, err1 := strconv.ParseInt(os.Args[2], 10, 64)
		if err1 != nil {
			fmt.Printf("ERR tenant 非法: %v\n", err1)
			os.Exit(2)
		}
		phone := os.Args[3]
		switch cmd {
		case "captcha":
			resp, err := cli.RequestPhoneLoginCode(ctx, &identity.RequestPhoneLoginCodeReq{TenantId: tenant, Phone: phone})
			if err != nil {
				fail(err)
			}
			fmt.Printf("expiresAt=%d\n", resp.ExpiresAtUnix)
		case "devcode":
			code, err := readDevCode(ctx, tenant, phone)
			if err != nil {
				fmt.Printf("ERR 读 Redis 验证码失败: %v\n", err)
				os.Exit(2)
			}
			fmt.Printf("code=%s\n", code)
		case "login":
			if len(os.Args) < 5 {
				usage()
				os.Exit(2)
			}
			resp, err := cli.LoginWithPhoneCode(ctx, &identity.LoginWithPhoneCodeReq{
				TenantId: tenant, Phone: phone, Code: os.Args[4],
			})
			if err != nil {
				fail(err)
			}
			fmt.Printf("userId=%d token=%s\n", resp.UserId, resp.Token)
		}
	case "set", "find", "search":
		tenant, err1 := strconv.ParseInt(os.Args[2], 10, 64)
		if err1 != nil {
			fmt.Printf("ERR tenant 非法: %v\n", err1)
			os.Exit(2)
		}
		switch cmd {
		case "set":
			uid, err2 := strconv.ParseInt(os.Args[3], 10, 64)
			if err2 != nil {
				fmt.Printf("ERR user 非法: %v\n", err2)
				os.Exit(2)
			}
			_, err := cli.SetUserPhone(ctx, &identity.SetUserPhoneReq{TenantId: tenant, UserId: uid, Phone: os.Args[4]})
			if err != nil {
				fail(err)
			}
			fmt.Printf("userId=%d\n", uid)
		case "find":
			resp, err := cli.FindUserPhoneExact(ctx, &identity.FindUserPhoneExactReq{TenantId: tenant, Phone: os.Args[3]})
			if err != nil {
				fail(err)
			}
			fmt.Printf("userId=%d\n", resp.UserId)
		case "search":
			resp, err := cli.SearchUserPhones(ctx, &identity.SearchUserPhonesReq{TenantId: tenant, Fragment: os.Args[3]})
			if err != nil {
				fail(err)
			}
			fmt.Printf("hits=%d\n", len(resp.Hits))
			for _, hit := range resp.Hits {
				fmt.Printf("hit userId=%d masked=%s\n", hit.UserId, hit.MaskedPhone)
			}
		}
	case "link", "resolve", "get":
		tenant, err1 := strconv.ParseInt(os.Args[2], 10, 64)
		if err1 != nil {
			fmt.Printf("ERR tenant 非法: %v\n", err1)
			os.Exit(2)
		}
		switch cmd {
		case "link":
			uid, err2 := strconv.ParseInt(os.Args[3], 10, 64)
			if err2 != nil {
				fmt.Printf("ERR user 非法: %v\n", err2)
				os.Exit(2)
			}
			kind, err3 := strconv.ParseInt(os.Args[4], 10, 64)
			if err3 != nil || (kind != 1 && kind != 2) {
				fmt.Printf("ERR kind 非法（1=小程序 2=App）: %v\n", err3)
				os.Exit(2)
			}
			union := os.Args[5]
			if len(os.Args) > 6 {
				union = os.Args[6]
			}
			_, err := cli.LinkThirdPartyIdentity(ctx, &identity.LinkThirdPartyIdentityReq{
				TenantId: tenant, UserId: uid, Kind: identity.ThirdPartyKind(kind), OpenId: os.Args[5], UnionId: union,
			})
			if err != nil {
				fail(err)
			}
			fmt.Printf("linked openId=%s\n", os.Args[5])
		case "resolve":
			kind, err3 := strconv.ParseInt(os.Args[3], 10, 64)
			if err3 != nil || (kind != 1 && kind != 2) {
				fmt.Printf("ERR kind 非法（1=小程序 2=App）: %v\n", err3)
				os.Exit(2)
			}
			resp, err := cli.ResolveUserByThirdParty(ctx, &identity.ResolveUserByThirdPartyReq{
				TenantId: tenant, Kind: identity.ThirdPartyKind(kind), OpenId: os.Args[4],
			})
			if err != nil {
				fail(err)
			}
			fmt.Printf("userId=%d\n", resp.UserId)
		case "get":
			uid, err2 := strconv.ParseInt(os.Args[3], 10, 64)
			if err2 != nil {
				fmt.Printf("ERR user 非法: %v\n", err2)
				os.Exit(2)
			}
			kind, err3 := strconv.ParseInt(os.Args[4], 10, 64)
			if err3 != nil || (kind != 1 && kind != 2) {
				fmt.Printf("ERR kind 非法（1=小程序 2=App）: %v\n", err3)
				os.Exit(2)
			}
			resp, err := cli.GetThirdPartyIdentity(ctx, &identity.GetThirdPartyIdentityReq{
				TenantId: tenant, UserId: uid, Kind: identity.ThirdPartyKind(kind),
			})
			if err != nil {
				fail(err)
			}
			fmt.Printf("openId=%s unionId=%s\n", resp.OpenId, resp.UnionId)
		}
	default:
		usage()
		os.Exit(2)
	}
}

// readDevCode DryRun 模式下验证码只在 Redis（不真发短信）：key 与 store 一致。
// 尝试原始号与 +86 归一化两种 md5（send 前归一化）。
func readDevCode(ctx context.Context, tenant int64, phone string) (string, error) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: os.Getenv("REDIS_PASS")})
	defer rdb.Close()
	for _, candidate := range []string{phone, "+86" + phone} {
		sum := md5.Sum([]byte(candidate))
		key := fmt.Sprintf("luckybox:sms:captcha:%d:%s", tenant, hex.EncodeToString(sum[:]))
		code, err := rdb.Get(ctx, key).Result()
		if err == nil {
			return code, nil
		}
	}
	return "", fmt.Errorf("captcha key not found")
}

// fail 业务错误码即 gRPC status code（ServerErrorInterceptor：CodeError → codes.Code(业务码)）：
// 业务码段输出 code=XXXX msg=... 一行并退出 1；标准 gRPC 码（如 InvalidArgument）
// 映射回项目标准码输出；其余基础设施错误输出 ERR 并退出 2
func fail(err error) {
	if st, ok := status.FromError(err); ok {
		c := int(st.Code())
		if c >= 1000 && c <= 10999 {
			fmt.Printf("code=%d msg=%s\n", c, st.Message())
			os.Exit(1)
		}
		if std, ok := mapStandard(st.Code()); ok {
			fmt.Printf("code=%d msg=%s\n", std, st.Message())
			os.Exit(1)
		}
	}
	fmt.Printf("ERR code=%v msg=%v\n", status.Code(err), err)
	os.Exit(2)
}

// mapStandard 标准 gRPC 码 → 项目标准码（与 errorx.mapGRPCCode 一致）
func mapStandard(c codes.Code) (int, bool) {
	switch c {
	case codes.InvalidArgument:
		return 400, true
	case codes.Unauthenticated:
		return 401, true
	case codes.PermissionDenied:
		return 403, true
	case codes.NotFound:
		return 404, true
	case codes.DeadlineExceeded:
		return 408, true
	case codes.ResourceExhausted:
		return 429, true
	}
	return 0, false
}

func usage() {
	fmt.Println("用法见文件头注释")
}
