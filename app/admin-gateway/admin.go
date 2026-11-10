// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package main

import (
	"flag"
	"fmt"

	"gacha_srv/app/admin-gateway/internal/admincheck"
	"gacha_srv/app/admin-gateway/internal/config"
	"gacha_srv/app/admin-gateway/internal/handler"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

// 配置统一放 pkg/etcs/（规范 08-deploy-config），启动：
// cd app/admin-gateway && go run admin.go -f ../../pkg/etcs/admin-gateway-api.yaml
var configFile = flag.String("f", "../../pkg/etcs/admin-gateway-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	// fail-closed（M14 设计 D5）：注册路由 ↔ 操作目录双向校验，缺失即启动失败
	admincheck.MustValidateRoutes(server.Routes(), c.Console.FailClosedMilestone)

	fmt.Printf("Starting admin gateway at %s:%d (prefix /console/v2/api)...\n", c.Host, c.Port)
	server.Start()
}
