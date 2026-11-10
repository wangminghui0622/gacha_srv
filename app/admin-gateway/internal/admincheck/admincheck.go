// Package admincheck fail-closed 路由校验（M14 设计 D5，照抄 server_v2 安全语义）：
// 注册路由 ↔ pkg/adminop 操作目录双向比对——任一方向缺失即启动失败，
// 防止"忘了登记鉴权配置的端点裸奔上线"（server_v2 由 proto 注解框架强制，
// go-zero 无此机制，故在启动时显式校验）。
package admincheck

import (
	"fmt"
	"strings"

	"gacha_srv/pkg/adminop"

	"github.com/zeromicro/go-zero/rest"
)

// consoleAPIPrefix 管理控制台路由前缀（与 .api @server prefix 一致）。
const consoleAPIPrefix = "/console/v2/api"

// MustValidateRoutes 双向校验注册路由与操作目录，缺失即 panic 启动失败。
// routes 为 server.Routes()（路径含 /console/v2/api 前缀）；
// targetMilestone 为目标里程碑（如 "M14a"）：目录中 ≤ 该里程碑的操作必须全部注册。
func MustValidateRoutes(routes []rest.Route, targetMilestone string) {
	// 1. 每个注册路由必须在目录中（含方法一致、路径逐段一致；
	//    中间件归属由 .api 静态声明，不在此校验）
	for _, rt := range routes {
		path := strings.TrimPrefix(rt.Path, consoleAPIPrefix)
		if _, _, ok := adminop.Match(rt.Method, path); !ok {
			panic(fmt.Sprintf("fail-closed: 路由未登记操作目录 %s %s（请补 pkg/adminop/operations.go 后重生成）", rt.Method, rt.Path))
		}
	}

	// 2. 目录中 ≤ 目标里程碑的操作必须全部注册（含公开操作）
	registered := map[string]bool{}
	for _, rt := range routes {
		registered[rt.Method+" "+strings.TrimPrefix(rt.Path, consoleAPIPrefix)] = true
	}
	for _, op := range adminop.All() {
		if !adminop.MilestoneAtMost(op, targetMilestone) {
			continue
		}
		if !registered[op.Method+" "+op.Path] {
			panic(fmt.Sprintf("fail-closed: 操作目录已登记但路由未注册 %s %s（%s）", op.Method, consoleAPIPrefix+op.Path, op.ID))
		}
	}
}
