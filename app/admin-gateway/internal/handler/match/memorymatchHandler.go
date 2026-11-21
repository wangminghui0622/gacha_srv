package match

import (
	"net/http"
	"strconv"
	"strings"

	"gacha_srv/app/admin-gateway/internal/logic/match"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"
)

// 读取对对碰全局玩法规则（gacha.pool:read）。
func GetMemoryMatchGlobalRulesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := match.NewGetMemoryMatchGlobalRulesLogic(r.Context(), svcCtx).
			GetMemoryMatchGlobalRules(&play.GetMemoryMatchGlobalRulesReq{})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// 保存对对碰全局玩法规则（gacha.pool:write）。
func SaveMemoryMatchGlobalRulesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req play.SaveMemoryMatchGlobalRulesReq
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := match.NewSaveMemoryMatchGlobalRulesLogic(r.Context(), svcCtx).SaveMemoryMatchGlobalRules(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// 牌堆模板列表（gacha.pool:read）。
func ListMemoryMatchDeckTemplatesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := restx.QueryUint32(r, "page")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		status, err := queryInt64Default(r, "status", -1)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &play.ListMemoryMatchDeckTemplatesReq{
			Keyword:  restx.QueryString(r, "keyword"),
			Status:   status,
			Page:     int64(page),
			PageSize: int64(pageSize),
		}
		resp, err := match.NewListMemoryMatchDeckTemplatesLogic(r.Context(), svcCtx).ListMemoryMatchDeckTemplates(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// 保存牌堆模板（gacha.pool:write）。
func SaveMemoryMatchDeckTemplateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req play.SaveMemoryMatchDeckTemplateReq
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := match.NewSaveMemoryMatchDeckTemplateLogic(r.Context(), svcCtx).SaveMemoryMatchDeckTemplate(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// 删除牌堆模板（gacha.pool:write）。
func DeleteMemoryMatchDeckTemplateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := restx.PathUint64(r, "id")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := match.NewDeleteMemoryMatchDeckTemplateLogic(r.Context(), svcCtx).
			DeleteMemoryMatchDeckTemplate(&play.DeleteMemoryMatchDeckTemplateReq{Id: int64(id)})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// 外观池列表（gacha.pool:read）。
func ListMemoryMatchAppearancesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := restx.QueryUint32(r, "page")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		typeFilter, err := queryInt64Default(r, "type", -1)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		status, err := queryInt64Default(r, "status", -1)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &play.ListMemoryMatchAppearancesReq{
			Type:     typeFilter,
			Status:   status,
			Keyword:  restx.QueryString(r, "keyword"),
			Page:     int64(page),
			PageSize: int64(pageSize),
		}
		resp, err := match.NewListMemoryMatchAppearancesLogic(r.Context(), svcCtx).ListMemoryMatchAppearances(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// 保存外观（gacha.pool:write）。
func SaveMemoryMatchAppearanceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req play.SaveMemoryMatchAppearanceReq
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := match.NewSaveMemoryMatchAppearanceLogic(r.Context(), svcCtx).SaveMemoryMatchAppearance(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// 删除外观（gacha.pool:write）。
func DeleteMemoryMatchAppearanceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := restx.PathUint64(r, "id")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := match.NewDeleteMemoryMatchAppearanceLogic(r.Context(), svcCtx).
			DeleteMemoryMatchAppearance(&play.DeleteMemoryMatchAppearanceReq{Id: int64(id)})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

func queryInt64Default(r *http.Request, name string, def int64) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return def, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, errorx.NewCodeError(errorx.CodeParamError, name+" 必须是整数")
	}
	return value, nil
}
