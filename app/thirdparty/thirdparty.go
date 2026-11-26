package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"gacha_srv/app/thirdparty/internal/config"
	"gacha_srv/app/thirdparty/internal/server"
	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "../../pkg/etcs/thirdparty-rpc.yaml", "the config file")

// resolveConfigFile 本地开发优先读取同名 .local.yaml（gitignored，放 COS/OSS 等真实密钥）。
func resolveConfigFile(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasSuffix(path, ".yaml") {
		localPath := strings.TrimSuffix(path, ".yaml") + ".local.yaml"
		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}
	return path
}

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(resolveConfigFile(*configFile), &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		thirdparty.RegisterThirdpartyServer(grpcServer, server.NewThirdpartyServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	// OSS 图片上传：放宽 gRPC 消息上限（默认 4MB），支持 10MB 以内的后台图片。
	s.AddOptions(grpc.MaxRecvMsgSize(16<<20), grpc.MaxSendMsgSize(16<<20))
	// 业务码跨 gRPC 不失真：CodeError(1000-10999) 直接作 status code 传出；
	// DTM SAGA 分支的 status.Error 原样透出
	s.AddUnaryInterceptors(errorx.ServerErrorInterceptor)
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
