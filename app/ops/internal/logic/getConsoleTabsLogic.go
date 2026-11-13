package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConsoleTabsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConsoleTabsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConsoleTabsLogic {
	return &GetConsoleTabsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetConsoleTabs 读取账号页签；从未保存过时返回空列表（前端会补仪表盘）。
func (l *GetConsoleTabsLogic) GetConsoleTabs(in *ops.GetConsoleTabsRequest) (*ops.ConsoleTabs, error) {
	if in.AccountId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "account_id 必填")
	}
	tenantID := defaultTenant(in.TenantId)
	record, err := l.svcCtx.TabStore.Get(l.ctx, tenantID, in.AccountId)
	if err != nil {
		return nil, err
	}
	out := &ops.ConsoleTabs{Tabs: []*ops.ConsoleTab{}}
	if record == nil {
		return out, nil
	}
	out.ActivePath = record.ActivePath
	for _, tab := range record.Tabs {
		out.Tabs = append(out.Tabs, &ops.ConsoleTab{Path: tab.Path, TitleKey: tab.TitleKey})
	}
	return out, nil
}
