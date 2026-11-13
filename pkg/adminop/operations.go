// Package adminop 管理控制台操作目录：权限目录（ops-rpc ListPermissionCatalog /
// PutRolePermissions 校验）与管理网关路由注册表（fail-closed）的单一事实源。
// 存放于 pkg/adminop（共享包目录，与 pkg/api_admin、pkg/etcs 同树，
// 被 app/ops 与 app/admin-gateway 共同引用）。
// operation_id/resource/action 照抄 server_v2 console proto 的 console_operation 注解。
package adminop

import (
	"sort"
	"strings"
)

// Operation 一条控制台操作（一个 HTTP 端点）。
type Operation struct {
	ID       string // operation_id（照抄 server_v2）
	Resource string // 资源（照抄）
	Action   string // 动作（照抄）
	Public   bool   // true = 免鉴权（登录/刷新/健康检查等）
	Method   string // HTTP 方法
	Path     string // HTTP 路径（/console/v2/api 之后的部分，含路径参数占位）
	Title    string // 中文标题（权限目录展示用）
}

// All 全量操作目录（M14 范围：会话/权限/宝箱奖池/商品/活动/条件政策/营销/风控/支付路由/任务/报表）。
// 管理网关启动时按此表逐路由校验：缺失登记 = 启动失败（fail-closed，照抄 server_v2）。
func All() []Operation {
	return []Operation{
		// ── 会话（ConsoleSessionHTTPService）──
		{ID: "console.session.login", Public: true, Method: "POST", Path: "/session/login", Title: "控制台登录"},
		{ID: "console.session.refresh", Public: true, Method: "POST", Path: "/session/refresh", Title: "刷新令牌"},
		{ID: "console.session.logout", Public: true, Method: "POST", Path: "/session/logout", Title: "退出登录"},
		{ID: "console.session.permissions", Public: true, Method: "GET", Path: "/session/permissions", Title: "当前有效权限"},
		{ID: "console.session.tabs.get", Public: true, Method: "GET", Path: "/session/tabs", Title: "读取账号页签"},
		{ID: "console.session.tabs.put", Public: true, Method: "PUT", Path: "/session/tabs", Title: "保存账号页签"},
		{ID: "console.session.openaccount", Resource: "identity.console_account", Action: "create", Method: "POST", Path: "/session/accounts", Title: "开通控制台账号"},
		{ID: "console.session.resetaccounttotp", Resource: "identity.console_account", Action: "reset_totp", Method: "POST", Path: "/session/accounts/:principalId/totp-reset", Title: "重置账号 TOTP"},
		// ── 权限（ConsoleAuthorizationService）──
		{ID: "console.authorization.permission_catalog.read", Resource: "authorization.role", Action: "read", Method: "GET", Path: "/authorization/permissions", Title: "查看权限目录"},
		{ID: "console.authorization.role.list", Resource: "authorization.role", Action: "read", Method: "GET", Path: "/authorization/roles", Title: "查看角色列表"},
		{ID: "console.authorization.role.get", Resource: "authorization.role", Action: "read", Method: "GET", Path: "/authorization/roles/:roleId", Title: "查看角色"},
		{ID: "console.authorization.role.create", Resource: "authorization.role", Action: "write", Method: "POST", Path: "/authorization/roles", Title: "创建角色"},
		{ID: "console.authorization.role.update", Resource: "authorization.role", Action: "write", Method: "PUT", Path: "/authorization/roles/:roleId", Title: "修改角色"},
		{ID: "console.authorization.role.permissions.put", Resource: "authorization.role", Action: "write", Method: "PUT", Path: "/authorization/roles/:roleId/permissions", Title: "配置角色权限"},
		{ID: "console.authorization.role.members.list", Resource: "authorization.role", Action: "read", Method: "GET", Path: "/authorization/roles/:roleId/members", Title: "查看角色成员"},
		{ID: "console.authorization.principal.roles.get", Resource: "authorization.role", Action: "read", Method: "GET", Path: "/authorization/principals/:principalId/roles", Title: "查看主体角色"},
		{ID: "console.authorization.principal.roles.put", Resource: "authorization.role", Action: "write", Method: "PUT", Path: "/authorization/principals/:principalId/roles", Title: "配置主体角色"},
		// ── 赏池/箱子（ConsolePoolHTTPService，resource: gacha.pool）──
		// createbox 不实现：server_v2 的 box-instance 概念本仓无对应物（我们的箱子即 draw_boxes，由赏池发布物化）
		{ID: "console.pool.getbox", Resource: "gacha.pool", Action: "read", Method: "GET", Path: "/gacha/boxes/:boxId", Title: "读取箱子"},
		{ID: "console.pool.getboxview", Resource: "gacha.pool", Action: "read", Method: "GET", Path: "/gacha/boxes/:boxId/view", Title: "箱子库存与玩法状态"},
		{ID: "console.pool.list", Resource: "gacha.pool", Action: "read", Method: "GET", Path: "/gacha/pools", Title: "赏池版本列表"},
		{ID: "console.pool.create", Resource: "gacha.pool", Action: "create", Method: "POST", Path: "/gacha/pools/drafts", Title: "创建赏池草稿"},
		{ID: "console.pool.publish", Resource: "gacha.pool", Action: "publish", Method: "POST", Path: "/gacha/pools/:poolId/versions/:version/publish", Title: "发布赏池"},
		{ID: "console.pool.requestpublication", Resource: "gacha.pool", Action: "request_publication", Method: "POST", Path: "/gacha/pools/:poolId/versions/:version/publication-request", Title: "提请发布赏池"},
		{ID: "console.pool.approvepublication", Resource: "gacha.pool", Action: "approve_publication", Method: "POST", Path: "/gacha/pools/:poolId/versions/:version/approval", Title: "审批赏池发布"},
		{ID: "console.pool.get", Resource: "gacha.pool", Action: "read", Method: "GET", Path: "/gacha/pools/:poolId/versions/:version", Title: "读取赏池版本"},
		// ── 统一玩赏池（所有玩法共用；resource 同 gacha.pool）──
		{ID: "console.playpool.list", Resource: "gacha.pool", Action: "read", Method: "GET", Path: "/gacha/play-pools", Title: "统一玩赏池列表"},
		{ID: "console.playpool.get", Resource: "gacha.pool", Action: "read", Method: "GET", Path: "/gacha/play-pools/:poolId/versions/:version", Title: "读取统一玩赏池版本"},
		{ID: "console.playpool.create", Resource: "gacha.pool", Action: "create", Method: "POST", Path: "/gacha/play-pools/drafts", Title: "创建统一玩赏池草稿"},
		{ID: "console.playpool.publish", Resource: "gacha.pool", Action: "publish", Method: "POST", Path: "/gacha/play-pools/:poolId/versions/:version/publish", Title: "发布统一玩赏池"},
		// assign_lucky 路径适配：server_v2 是 /gacha/boxes/:boxId/lucky-assignment，
		// 本仓幸运指派实体是扭蛋配置（gashapon_configs），不是箱子
		{ID: "console.pool.assignluckyuser", Resource: "gacha.pool", Action: "assign_lucky", Method: "POST", Path: "/gacha/gashapons/:configId/lucky-assignment", Title: "指定幸运用户"},
		// ── 商品（ConsoleCatalogHTTPService，resource: catalog.product）──
		{ID: "console.catalog.ip.list", Resource: "catalog.product", Action: "read", Method: "GET", Path: "/catalog/ips", Title: "IP 列表"},
		{ID: "console.catalog.ip.create", Resource: "catalog.product", Action: "create", Method: "POST", Path: "/catalog/ips", Title: "创建 IP"},
		{ID: "console.catalog.ip.update", Resource: "catalog.product", Action: "write", Method: "PUT", Path: "/catalog/ips/:ipId", Title: "编辑 IP"},
		{ID: "console.catalog.brand.list", Resource: "catalog.product", Action: "read", Method: "GET", Path: "/catalog/brands", Title: "品牌列表"},
		{ID: "console.catalog.brand.create", Resource: "catalog.product", Action: "create", Method: "POST", Path: "/catalog/brands", Title: "创建品牌"},
		{ID: "console.catalog.brand.update", Resource: "catalog.product", Action: "write", Method: "PUT", Path: "/catalog/brands/:brandId", Title: "编辑品牌"},
		{ID: "console.catalog.series.list", Resource: "catalog.product", Action: "read", Method: "GET", Path: "/catalog/series", Title: "系列列表"},
		{ID: "console.catalog.series.create", Resource: "catalog.product", Action: "create", Method: "POST", Path: "/catalog/series", Title: "创建系列"},
		{ID: "console.catalog.series.update", Resource: "catalog.product", Action: "write", Method: "PUT", Path: "/catalog/series/:seriesId", Title: "编辑系列"},
		{ID: "console.catalog.list", Resource: "catalog.product", Action: "read", Method: "GET", Path: "/catalog/products", Title: "商品列表"},
		{ID: "console.catalog.create", Resource: "catalog.product", Action: "create", Method: "POST", Path: "/catalog/products", Title: "创建商品"},
		{ID: "console.catalog.get", Resource: "catalog.product", Action: "read", Method: "GET", Path: "/catalog/products/:productId", Title: "查看商品"},
		{ID: "console.catalog.publish", Resource: "catalog.product", Action: "publish", Method: "POST", Path: "/catalog/products/:productId/publish", Title: "发布商品"},
		{ID: "console.catalog.saverevision", Resource: "catalog.product", Action: "write", Method: "POST", Path: "/catalog/products/:productId/revision", Title: "保存商品修订"},
		{ID: "console.catalog.savevariantrevision", Resource: "catalog.product", Action: "write", Method: "POST", Path: "/catalog/products/:productId/variant-revision", Title: "保存 SKU 修订"},
		{ID: "console.catalog.getrevision", Resource: "catalog.product", Action: "read", Method: "GET", Path: "/catalog/products/:productId/revision", Title: "查看商品修订"},
		{ID: "console.catalog.publishrevision", Resource: "catalog.product", Action: "publish", Method: "POST", Path: "/catalog/products/:productId/revision/publish", Title: "发布商品修订"},
		// ── 活动（ConsoleCampaignHTTPService，resource: campaign.definition）──
		{ID: "console.campaign.createdraft", Resource: "campaign.definition", Action: "write", Method: "POST", Path: "/campaigns/drafts", Title: "创建活动草稿"},
		{ID: "console.campaign.publish", Resource: "campaign.definition", Action: "publish", Method: "POST", Path: "/campaigns/:campaignId/publish", Title: "发布活动"},
		{ID: "console.campaign.get", Resource: "campaign.definition", Action: "read", Method: "GET", Path: "/campaigns/:campaignId", Title: "查看活动"},
		// ── 条件政策（ConsoleEligibilityHTTPService，resource: eligibility.policy）──
		{ID: "console.eligibility.list", Resource: "eligibility.policy", Action: "read", Method: "GET", Path: "/eligibility/policies", Title: "条件政策列表"},
		{ID: "console.eligibility.get", Resource: "eligibility.policy", Action: "read", Method: "GET", Path: "/eligibility/policies/:policyId/versions/:version", Title: "条件政策详情"},
		{ID: "console.eligibility.create", Resource: "eligibility.policy", Action: "create", Method: "POST", Path: "/eligibility/policies", Title: "创建条件政策"},
		{ID: "console.eligibility.createrevision", Resource: "eligibility.policy", Action: "write", Method: "POST", Path: "/eligibility/policies/:policyId/revisions", Title: "创建条件政策修订"},
		{ID: "console.eligibility.publish", Resource: "eligibility.policy", Action: "publish", Method: "POST", Path: "/eligibility/policies/:policyId/versions/:version/publish", Title: "发布条件政策"},
		// ── 营销（ConsolePromotionHTTPService）──
		{ID: "console.promotion.listcoupontemplates", Resource: "promotion.coupon_template", Action: "read", Method: "GET", Path: "/promotion/tenants/:tenantId/coupon-templates", Title: "优惠券模板列表"},
		{ID: "console.promotion.getcoupontemplate", Resource: "promotion.coupon_template", Action: "read", Method: "GET", Path: "/promotion/tenants/:tenantId/coupon-templates/:templateId", Title: "查看券模板"},
		{ID: "console.promotion.publishcoupontemplate", Resource: "promotion.coupon_template", Action: "publish", Method: "POST", Path: "/promotion/tenants/:tenantId/coupon-templates", Title: "发布券模板"},
		{ID: "console.promotion.grantcoupons", Resource: "promotion.user_coupon", Action: "grant", Method: "POST", Path: "/promotion/tenants/:tenantId/coupon-grants", Title: "发放优惠券"},
		{ID: "console.promotion.getloyalty", Resource: "promotion.tenant_loyalty", Action: "read", Method: "GET", Path: "/promotion/tenants/:tenantId/loyalty-defaults", Title: "查看忠诚度默认值"},
		{ID: "console.promotion.putloyalty", Resource: "promotion.tenant_loyalty", Action: "write", Method: "PUT", Path: "/promotion/tenants/:tenantId/loyalty-defaults", Title: "配置忠诚度默认值"},
		// ── 风控（ConsoleRiskHTTPService，resource: risk.case）──
		{ID: "console.risk.list", Resource: "risk.case", Action: "list", Method: "GET", Path: "/risk/cases", Title: "风控案件列表"},
		{ID: "console.risk.claim", Resource: "risk.case", Action: "claim", Method: "POST", Path: "/risk/cases/:caseId/claim", Title: "认领风控案件"},
		{ID: "console.risk.resolve", Resource: "risk.case", Action: "resolve", Method: "POST", Path: "/risk/cases/:caseId/resolve", Title: "解决风控案件"},
		// ── 支付路由（ConsolePaymentRouteService，resource: payment.route）──
		{ID: "console.paymentroute.getpaymentroute", Resource: "payment.route", Action: "read", Method: "GET", Path: "/payment/routes", Title: "读取支付路由"},
		{ID: "console.paymentroute.putpaymentroute", Resource: "payment.route", Action: "write", Method: "PUT", Path: "/payment/routes", Title: "配置支付路由"},
		// ── 任务（ConsoleTaskHTTPService）──
		{ID: "console.task.list", Resource: "task.progress", Action: "read", Method: "GET", Path: "/tasks", Title: "任务进度列表"},
		{ID: "console.task.createplandraft", Resource: "task.plan", Action: "write", Method: "POST", Path: "/task-plans/drafts", Title: "创建任务计划草稿"},
		{ID: "console.task.listplans", Resource: "task.plan", Action: "read", Method: "GET", Path: "/task-plans", Title: "任务计划列表"},
		{ID: "console.task.getplan", Resource: "task.plan", Action: "read", Method: "GET", Path: "/task-plans/:version", Title: "读取任务计划"},
		{ID: "console.task.requestplanpublication", Resource: "task.plan", Action: "request_publication", Method: "POST", Path: "/task-plans/:version/publication-request", Title: "申请发布任务计划"},
		{ID: "console.task.approveplanpublication", Resource: "task.plan", Action: "approve_publication", Method: "POST", Path: "/task-plans/:version/approval", Title: "审批任务计划"},
		{ID: "console.task.getplanpublicationapproval", Resource: "task.plan", Action: "read", Method: "GET", Path: "/task-plans/:version/approval", Title: "读取任务计划审批"},
		{ID: "console.task.publishplan", Resource: "task.plan", Action: "publish", Method: "POST", Path: "/task-plans/:version/publish", Title: "发布任务计划"},
		// ── 报表（ConsoleReportingHTTPService）──
		{ID: "console.reporting.queryuserprofit", Resource: "reporting.metric", Action: "read", Method: "GET", Path: "/analytics/users/:userId/profit-curve", Title: "用户盈亏曲线"},
		{ID: "console.reporting.metrics", Resource: "reporting.metric", Action: "read", Method: "GET", Path: "/reporting/metrics", Title: "指标定义列表"},
		{ID: "console.reporting.save", Resource: "reporting.dashboard", Action: "write", Method: "POST", Path: "/reporting/dashboards", Title: "保存看板"},
		{ID: "console.reporting.get", Resource: "reporting.dashboard", Action: "read", Method: "GET", Path: "/reporting/dashboards/:dashboardId", Title: "读取看板"},
		{ID: "console.reporting.publish", Resource: "reporting.dashboard", Action: "publish", Method: "POST", Path: "/reporting/dashboards/:dashboardId/publish", Title: "发布看板"},
		{ID: "console.reporting.query", Resource: "reporting.dashboard", Action: "query", Method: "POST", Path: "/reporting/dashboards/:dashboardId/query", Title: "查询看板"},
		// ── 系统（ConsoleSystemService）──
		{ID: "console.system.health", Public: true, Method: "GET", Path: "/health", Title: "健康检查"},
	}
}

// ByID 返回 operation_id → Operation 索引。
func ByID() map[string]Operation {
	m := map[string]Operation{}
	for _, op := range All() {
		m[op.ID] = op
	}
	return m
}

// IsConsolePermission 校验 resource/action 是否在权限目录内
// （照抄 server_v2 PutRolePermissions 的校验：未知权限拒绝）。
func IsConsolePermission(resource, action string) bool {
	for _, op := range All() {
		if op.Resource == resource && op.Action == action {
			return true
		}
	}
	return false
}

// resourceNames 资源中文名。
var resourceNames = map[string]string{
	"authorization.role":        "权限管理",
	"identity.console_account":  "控制台账号",
	"catalog.product":           "商品",
	"gacha.pool":                "赏池与箱子",
	"campaign.definition":       "活动",
	"eligibility.policy":        "条件政策",
	"promotion.coupon_template": "优惠券模板",
	"promotion.tenant_loyalty":  "忠诚度默认值",
	"promotion.user_coupon":     "用户券",
	"risk.case":                 "风控案件",
	"payment.route":             "支付路由",
	"task.plan":                 "任务计划",
	"task.progress":             "任务进度",
	"reporting.metric":          "指标",
	"reporting.dashboard":       "报表看板",
}

// actionNames 动作中文名。
var actionNames = map[string]string{
	"read": "查看", "write": "修改", "create": "创建", "publish": "发布",
	"request_publication": "提请发布", "approve_publication": "审批发布",
	"assign_lucky": "指定幸运用户", "reset_totp": "重置 TOTP", "list": "列表",
	"claim": "认领", "resolve": "解决", "grant": "发放", "query": "查询",
}

// ResourceName 资源中文名（未知资源返回原码）。
func ResourceName(resource string) string {
	if n, ok := resourceNames[resource]; ok {
		return n
	}
	return resource
}

// ActionName 动作中文名（未知动作返回原码）。
func ActionName(action string) string {
	if n, ok := actionNames[action]; ok {
		return n
	}
	return action
}

// CatalogResources 权限目录（resource → actions，按名称排序，供 ListPermissionCatalog 用）。
func CatalogResources() []struct {
	Code    string
	Name    string
	Actions []string
} {
	byResource := map[string]map[string]struct{}{}
	for _, op := range All() {
		if op.Resource == "" {
			continue
		}
		if byResource[op.Resource] == nil {
			byResource[op.Resource] = map[string]struct{}{}
		}
		byResource[op.Resource][op.Action] = struct{}{}
	}
	codes := make([]string, 0, len(byResource))
	for code := range byResource {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	out := make([]struct {
		Code    string
		Name    string
		Actions []string
	}, 0, len(codes))
	for _, code := range codes {
		actions := make([]string, 0, len(byResource[code]))
		for action := range byResource[code] {
			actions = append(actions, action)
		}
		sort.Strings(actions)
		out = append(out, struct {
			Code    string
			Name    string
			Actions []string
		}{Code: code, Name: ResourceName(code), Actions: actions})
	}
	return out
}

// ── 里程碑归组（M14a-e 交付节奏；管理网关 fail-closed 校验按里程碑推进）──

// milestoneOrder 里程碑先后序（M14a 会话/权限 → M14e 任务/报表）。
var milestoneOrder = []string{"M14a", "M14b", "M14c", "M14d", "M14e"}

// MilestoneOf 操作所属里程碑（按路径前缀归组）。
func MilestoneOf(op Operation) string {
	switch {
	case strings.HasPrefix(op.Path, "/session/"),
		strings.HasPrefix(op.Path, "/authorization/"),
		op.Path == "/health":
		return "M14a"
	case strings.HasPrefix(op.Path, "/gacha/"):
		return "M14b"
	case strings.HasPrefix(op.Path, "/catalog/"):
		return "M14c"
	case strings.HasPrefix(op.Path, "/campaigns/"),
		strings.HasPrefix(op.Path, "/eligibility/"),
		strings.HasPrefix(op.Path, "/promotion/"),
		strings.HasPrefix(op.Path, "/risk/"),
		strings.HasPrefix(op.Path, "/payment/"):
		return "M14d"
	default: // /tasks、/task-plans、/analytics、/reporting
		return "M14e"
	}
}

// MilestoneAtMost 目标里程碑（含）是否已覆盖操作所属里程碑。
func MilestoneAtMost(op Operation, target string) bool {
	targetIdx, opIdx := -1, -1
	for i, m := range milestoneOrder {
		if m == target {
			targetIdx = i
		}
		if m == MilestoneOf(op) {
			opIdx = i
		}
	}
	if targetIdx == -1 {
		return false // 未知目标里程碑 → 不强制（fail-closed 由网关显式校验）
	}
	return opIdx != -1 && opIdx <= targetIdx
}

// Match 按 HTTP 方法与路径匹配目录操作（":param" 段取实参），未命中返回 ok=false。
func Match(method, path string) (Operation, map[string]string, bool) {
	segs := splitPath(path)
	for _, op := range All() {
		if op.Method != method {
			continue
		}
		params, ok := matchSegments(op.Path, segs)
		if ok {
			return op, params, true
		}
	}
	return Operation{}, nil, false
}

// matchSegments 逐段比较：目录段 ":name" 匹配任意实参并记录。
func matchSegments(catalogPath string, segs []string) (map[string]string, bool) {
	catalogSegs := splitPath(catalogPath)
	if len(catalogSegs) != len(segs) {
		return nil, false
	}
	params := map[string]string{}
	for i, want := range catalogSegs {
		if strings.HasPrefix(want, ":") {
			params[strings.TrimPrefix(want, ":")] = segs[i]
			continue
		}
		if want != segs[i] {
			return nil, false
		}
	}
	return params, true
}

func splitPath(p string) []string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
