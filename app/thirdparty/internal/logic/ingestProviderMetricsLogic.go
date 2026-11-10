package logic

import (
	"context"
	"errors"

	"gacha_srv/app/thirdparty/internal/core"
	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngestProviderMetricsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewIngestProviderMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngestProviderMetricsLogic {
	return &IngestProviderMetricsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *IngestProviderMetricsLogic) IngestProviderMetrics(in *thirdparty.IngestProviderMetricsReq) (*thirdparty.IngestProviderMetricsResp, error) {
	// server_v2 IngestProviderMetrics 同款：校验失败 6011、幂等冲突 6012、其余 500
	batch, err := core.BuildBatch(in)
	if err != nil {
		if errors.Is(err, core.ErrInvalidRequest) {
			return nil, errorx.NewCodeError(errorx.CodeMetricInvalidRequest, err.Error())
		}
		return nil, errorx.NewCodeErrorf(errorx.CodeServerError, "%v", err)
	}
	done, replayed, err := l.svcCtx.MetricStore.Ingest(l.ctx, batch)
	if err != nil {
		if errors.Is(err, core.ErrIdempotencyConflict) {
			return nil, errorx.NewCodeError(errorx.CodeMetricConflict, err.Error())
		}
		return nil, errorx.NewCodeErrorf(errorx.CodeServerError, "%v", err)
	}
	return &thirdparty.IngestProviderMetricsResp{
		BatchId:       done.Id,
		AcceptedCount: uint32(done.AcceptedCount),
		StaleCount:    uint32(done.StaleCount),
		Replayed:      replayed,
	}, nil
}
