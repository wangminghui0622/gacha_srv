package logic

import (
	"context"

	"gacha_srv/app/play/internal/repo"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchGlobalRulesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMemoryMatchGlobalRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchGlobalRulesLogic {
	return &GetMemoryMatchGlobalRulesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- 对对碰管理后台（全局规则/牌堆模板/外观池）----
func (l *GetMemoryMatchGlobalRulesLogic) GetMemoryMatchGlobalRules(in *play.GetMemoryMatchGlobalRulesReq) (*play.GetMemoryMatchGlobalRulesResp, error) {
	store := repo.NewMemoryMatchAdminStore(l.svcCtx.DB)
	row, err := store.GetGlobalRules(l.ctx)
	if err != nil {
		return nil, mmDBError(err)
	}
	return &play.GetMemoryMatchGlobalRulesResp{Rules: mmGlobalRuleToProto(row)}, nil
}
