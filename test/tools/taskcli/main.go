// e2e 工具：M15c task 玩家体系 grpc 直连调用（场景 31 断言用）。
// ApplyTaskProgress / UnlockPass 是内部 RPC（无 HTTP 路由，D4），场景经此工具直连 task-rpc 9317。
// 用法：
//   go run test/tools/taskcli/main.go apply <userId> <eventId> <metric> <amount>  → 进度事件（幂等）
//   go run test/tools/taskcli/main.go unlock-pass <userId> <passId>                → 付费线通行证解锁 premium
// metric: 1 DRAW_COUNT / 2 DRAW_SPEND_MINOR / 3 LOGIN / 4 RECHARGE_MINOR / 5 SIGN_IN。
// 业务错误输出 code=XXXX msg=... 行并退出 1（ServerErrorInterceptor：CodeError → gRPC status code），
// 其余 grpc 错误退出 2。
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"gacha_srv/app/task/task"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	conn, err := grpc.NewClient("127.0.0.1:9317", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("ERR 连接 game-rpc 失败: %v\n", err)
		os.Exit(2)
	}
	defer conn.Close()
	cli := task.NewTaskClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch os.Args[1] {
	case "apply":
		if len(os.Args) != 6 {
			usage()
			os.Exit(2)
		}
		uid, err1 := strconv.ParseInt(os.Args[2], 10, 64)
		metric, err2 := strconv.ParseInt(os.Args[4], 10, 32)
		amount, err3 := strconv.ParseInt(os.Args[5], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			usage()
			os.Exit(2)
		}
		_, err := cli.ApplyTaskProgress(ctx, &task.ApplyTaskProgressReq{
			UserId: uid, EventId: os.Args[3], Metric: task.TaskMetric(metric), Amount: amount,
		})
		if err != nil {
			fail(err)
		}
		fmt.Println("OK")
	case "unlock-pass":
		if len(os.Args) != 4 {
			usage()
			os.Exit(2)
		}
		uid, err1 := strconv.ParseInt(os.Args[2], 10, 64)
		passId, err2 := strconv.ParseInt(os.Args[3], 10, 64)
		if err1 != nil || err2 != nil {
			usage()
			os.Exit(2)
		}
		_, err := cli.UnlockPass(ctx, &task.UnlockPassReq{UserId: uid, PassId: passId})
		if err != nil {
			fail(err)
		}
		fmt.Println("OK")
	default:
		usage()
		os.Exit(2)
	}
}

// fail 业务错误码即 gRPC status code（ServerErrorInterceptor：CodeError → codes.Code(业务码)）：
// 业务码段输出 code=XXXX msg=... 一行并退出 1；其余基础设施错误输出 ERR 并退出 2
func fail(err error) {
	if st, ok := status.FromError(err); ok {
		c := int(st.Code())
		if c >= 1000 && c <= 10999 {
			fmt.Printf("code=%d msg=%s\n", c, st.Message())
			os.Exit(1)
		}
	}
	fmt.Printf("ERR code=%v msg=%v\n", status.Code(err), err)
	os.Exit(2)
}

func usage() {
	fmt.Println("用法见文件头注释")
}
