// e2e 工具：EvaluatePolicy grpc 直连调用（场景 27 条件政策断言用）。
// 用法：go run test/tools/evalpol/main.go <userId> [scene=DRAW] [requestIdSuffix]
// 输出：allowed=<bool> policyNo=<单号> reason=<拒绝原因>
//   - Aborted(code=5001) 是业务拒绝（决策日志已落库），按 allowed=false 输出并退出 0；
//   - 其余 grpc 错误输出 ERR 行并退出 2。
// 调用按 SAGA 分支契约携带 dtm-* trans 元数据（屏障表写入由服务端完成，e2e 清理删除）。
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gacha_srv/app/base/base"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func main() {
	uid, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		fmt.Printf("ERR userId 非法: %v\n", err)
		os.Exit(2)
	}
	scene := "DRAW"
	if len(os.Args) > 2 && os.Args[2] != "" {
		scene = os.Args[2]
	}
	suffix := os.Args[1]
	if len(os.Args) > 3 && os.Args[3] != "" {
		suffix = os.Args[3]
	}

	conn, err := grpc.NewClient("127.0.0.1:9314", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("ERR 连接 base-rpc 失败: %v\n", err)
		os.Exit(2)
	}
	defer conn.Close()
	cli := base.NewBaseClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// EvaluatePolicy 是 SAGA 分支 01：服务端从元数据解析屏障事务信息（缺失→Internal）。
	ctx = metadata.AppendToOutgoingContext(ctx,
		"dtm-gid", "e2e-eval-gid-"+suffix,
		"dtm-trans_type", "saga",
		"dtm-branch_id", "01",
		"dtm-op", "action",
	)

	resp, err := cli.EvaluatePolicy(ctx, &base.EvaluatePolicyReq{
		RequestId: "e2e-eval-" + suffix,
		UserId:    uid,
		BoxId:     0,
		Scene:     scene,
	})
	if err != nil {
		if status.Code(err) == codes.Aborted {
			// 业务拒绝：desc 形如 "code=4001 msg=<failure_code>"
			reason := err.Error()
			if idx := strings.LastIndex(reason, "msg="); idx >= 0 {
				reason = reason[idx+4:]
			}
			fmt.Printf("allowed=false policyNo= reason=%s\n", reason)
			return
		}
		fmt.Printf("ERR code=%v msg=%v\n", status.Code(err), err)
		os.Exit(2)
	}
	fmt.Printf("allowed=true policyNo=%s reason=\n", resp.PolicyNo)
}
