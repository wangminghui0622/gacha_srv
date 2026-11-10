// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package main

import (
	"flag"
	"fmt"

	"gacha_srv/app/gateway/internal/config"
	"gacha_srv/app/gateway/internal/handler"
	"gacha_srv/app/gateway/internal/restx"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/pkg/utils/validator"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 配置统一放 pkg/etcs/（规范 08-deploy-config），启动：
// cd app/gateway && go run gateway.go -f ../../pkg/etcs/gateway-api.yaml
var configFile = flag.String("f", "../../pkg/etcs/gateway-api.yaml", "the config file")

func main() {
	flag.Parse()

	// 统一响应信封 {code, msg, data}：HTTP 恒 200，错误只通过 code 表达
	restx.MustSetup()

	httpx.SetValidator(validator.New())

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
