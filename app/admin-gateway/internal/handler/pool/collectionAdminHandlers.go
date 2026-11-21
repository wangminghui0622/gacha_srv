package pool

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/admin-gateway/internal/types"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ListCollectionSetsHandler 图鉴集合列表（gacha.pool:read）。
func ListCollectionSetsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CollectionSetListReq
		if err := httpx.Parse(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := pool.NewListCollectionSetsLogic(r.Context(), svcCtx).ListCollectionSets(&play.ListCollectionSetsReq{
			Keyword:  req.Keyword,
			Status:   req.Status,
			Page:     req.Page,
			PageSize: req.PageSize,
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// CreateCollectionSetHandler 新建图鉴集合（gacha.pool:create）。
func CreateCollectionSetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CollectionSetCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := pool.NewCreateCollectionSetLogic(r.Context(), svcCtx).CreateCollectionSet(&play.CreateCollectionSetReq{
			Code:           req.Code,
			Name:           req.Name,
			AccumulateType: req.AccumulateType,
			CycleType:      req.CycleType,
			Sort:           req.Sort,
			Status:         req.Status,
			ConfigJson:     req.ConfigJson,
			StartAt:        req.StartAt,
			EndAt:          req.EndAt,
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// UpdateCollectionSetHandler 编辑图鉴集合（gacha.pool:write）。
func UpdateCollectionSetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CollectionSetUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := pool.NewUpdateCollectionSetLogic(r.Context(), svcCtx).UpdateCollectionSet(&play.UpdateCollectionSetReq{
			Id:             req.Id,
			Name:           req.Name,
			AccumulateType: req.AccumulateType,
			CycleType:      req.CycleType,
			Sort:           req.Sort,
			Status:         req.Status,
			ConfigJson:     req.ConfigJson,
			StartAt:        req.StartAt,
			EndAt:          req.EndAt,
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// SetCollectionSetStatusHandler 更新图鉴集合状态（gacha.pool:write）。
func SetCollectionSetStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CollectionSetStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := pool.NewSetCollectionSetStatusLogic(r.Context(), svcCtx).SetCollectionSetStatus(&play.SetCollectionSetStatusReq{
			Id:     req.Id,
			Status: req.Status,
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// GetCollectionSetDataHandler 图鉴集合数据概览（gacha.pool:read）。
func GetCollectionSetDataHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var path types.CollectionSetDataReq
		if err := httpx.Parse(r, &path); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := pool.NewGetCollectionSetDataLogic(r.Context(), svcCtx).GetCollectionSetData(&play.GetCollectionSetDataReq{Id: path.Id})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
