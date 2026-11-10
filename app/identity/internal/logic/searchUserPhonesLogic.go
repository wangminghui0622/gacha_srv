package logic

import (
	"context"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchUserPhonesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchUserPhonesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchUserPhonesLogic {
	return &SearchUserPhonesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SearchUserPhones 片段搜索（4 位起；候选超 500 → 1014；掩码 3-4-4 返回）
func (l *SearchUserPhonesLogic) SearchUserPhones(in *identity.SearchUserPhonesReq) (*identity.SearchUserPhonesResp, error) {
	if in.TenantId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenant required")
	}
	results, err := l.svcCtx.PhoneRepo.Search(l.ctx, in.TenantId, in.Fragment)
	if err != nil {
		return nil, mapPhoneRepoError(err)
	}
	hits := make([]*identity.PhoneSearchHit, 0, len(results))
	for _, hit := range results {
		hits = append(hits, &identity.PhoneSearchHit{UserId: hit.UserID, MaskedPhone: hit.MaskedPhone})
	}
	return &identity.SearchUserPhonesResp{Hits: hits}, nil
}
