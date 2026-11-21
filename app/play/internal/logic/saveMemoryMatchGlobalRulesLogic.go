package logic

import (
	"context"
	"time"

	"gacha_srv/app/play/internal/repo"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveMemoryMatchGlobalRulesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveMemoryMatchGlobalRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveMemoryMatchGlobalRulesLogic {
	return &SaveMemoryMatchGlobalRulesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SaveMemoryMatchGlobalRulesLogic) SaveMemoryMatchGlobalRules(in *play.SaveMemoryMatchGlobalRulesReq) (*play.SaveMemoryMatchGlobalRulesResp, error) {
	if in == nil || in.Rules == nil {
		return nil, mmValidateGlobalRules(nil)
	}
	if err := mmValidateGlobalRules(in.Rules); err != nil {
		return nil, err
	}
	store := repo.NewMemoryMatchAdminStore(l.svcCtx.DB)
	old, _ := store.GetGlobalRules(l.ctx)
	now := time.Now()
	row := &repo.MemoryMatchGlobalRuleRow{
		ID:                   1,
		Name:                 in.Rules.Name,
		PairProbabilityMode:  in.Rules.PairProbabilityMode,
		InitialHandCount:     in.Rules.InitialHandCount,
		PairSize:             in.Rules.PairSize,
		ReplenishCount:       in.Rules.ReplenishCount,
		SecondDestinyEnabled: in.Rules.SecondDestinyEnabled,
		DestinyDoubleEnabled: in.Rules.DestinyDoubleEnabled,
		DestinyIncome:        in.Rules.DestinyIncome,
		DoubleDestinyBuff:    in.Rules.DoubleDestinyBuff,
		DestinyReplenishCost: in.Rules.DestinyReplenishCost,
		ResumeEnabled:        in.Rules.ResumeEnabled,
		HostModeEnabled:      in.Rules.HostModeEnabled,
		HostMinPairs:         in.Rules.HostMinPairs,
		HostShareBps:         in.Rules.HostShareBps,
		RevenueBase:          in.Rules.RevenueBase,
		LedgerType:           in.Rules.LedgerType,
		Status:               in.Rules.Status,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if old != nil {
		row.CreatedAt = old.CreatedAt
	}
	if err := store.SaveGlobalRules(l.ctx, row); err != nil {
		return nil, mmDBError(err)
	}
	if err := store.SyncFollowingConfigs(l.ctx, row); err != nil {
		return nil, mmDBError(err)
	}
	return &play.SaveMemoryMatchGlobalRulesResp{Rules: mmGlobalRuleToProto(row)}, nil
}
