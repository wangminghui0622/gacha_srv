package main

import (
	"flag"
	"fmt"

	"gacha_srv/app/play/internal/config"
	"gacha_srv/app/play/internal/server"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "../../pkg/etcs/play-rpc.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		play.RegisterPlayServer(grpcServer, server.NewPlayServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	// 业务码跨 gRPC 不失真：CodeError(1000-10999) 直接作 status code 传出；
	// DTM SAGA 分支的 status.Error 原样透出
	s.AddUnaryInterceptors(errorx.ServerErrorInterceptor)
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
