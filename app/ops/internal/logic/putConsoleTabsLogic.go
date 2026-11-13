package logic

import (
	"context"
	"strings"

	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	consoleTabsMaxCount      = 50
	consoleTabPathMaxLength  = 512
	consoleTabTitleMaxLength = 128
)

type PutConsoleTabsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPutConsoleTabsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutConsoleTabsLogic {
	return &PutConsoleTabsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PutConsoleTabs 覆盖保存账号页签，并返回服务端归一化后的结果。
func (l *PutConsoleTabsLogic) PutConsoleTabs(in *ops.PutConsoleTabsRequest) (*ops.ConsoleTabs, error) {
	if in.AccountId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "account_id 必填")
	}
	tabs, err := normalizeConsoleTabs(in.Tabs)
	if err != nil {
		return nil, err
	}
	activePath := strings.TrimSpace(in.ActivePath)
	if !containsConsoleTab(tabs, activePath) {
		activePath = tabs[0].Path
	}
	tenantID := defaultTenant(in.TenantId)
	if err = l.svcCtx.TabStore.Put(l.ctx, tenantID, in.AccountId, &repo.TabRecord{
		Tabs:       tabs,
		ActivePath: activePath,
	}); err != nil {
		return nil, err
	}
	out := &ops.ConsoleTabs{ActivePath: activePath, Tabs: make([]*ops.ConsoleTab, 0, len(tabs))}
	for _, tab := range tabs {
		out.Tabs = append(out.Tabs, &ops.ConsoleTab{Path: tab.Path, TitleKey: tab.TitleKey})
	}
	return out, nil
}

func normalizeConsoleTabs(in []*ops.ConsoleTab) ([]repo.TabItem, error) {
	if len(in) > consoleTabsMaxCount {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "页签数量不能超过 50 个")
	}
	tabs := make([]repo.TabItem, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, item := range in {
		if item == nil {
			continue
		}
		path := strings.TrimSpace(item.Path)
		if !validConsoleTabPath(path) {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "页签路径无效")
		}
		if _, ok := seen[path]; ok {
			continue
		}
		titleKey := strings.TrimSpace(item.TitleKey)
		if titleKey == "" {
			titleKey = "nav.dashboard"
		}
		if len(titleKey) > consoleTabTitleMaxLength {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "页签标题无效")
		}
		seen[path] = struct{}{}
		tabs = append(tabs, repo.TabItem{Path: path, TitleKey: titleKey})
	}
	if len(tabs) == 0 {
		tabs = append(tabs, repo.TabItem{Path: "/dashboard", TitleKey: "nav.dashboard"})
	}
	return tabs, nil
}

func validConsoleTabPath(path string) bool {
	if path == "" || len(path) > consoleTabPathMaxLength || !strings.HasPrefix(path, "/") {
		return false
	}
	if strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\\\r\n\t?#") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

func containsConsoleTab(tabs []repo.TabItem, path string) bool {
	for _, tab := range tabs {
		if tab.Path == path {
			return true
		}
	}
	return false
}
