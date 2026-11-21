package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gacha_srv/app/play/internal/repo"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"
)

func mmTimeString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func mmGlobalRuleToProto(row *repo.MemoryMatchGlobalRuleRow) *play.MemoryMatchGlobalRules {
	if row == nil {
		return nil
	}
	return &play.MemoryMatchGlobalRules{
		Id:                   row.ID,
		Name:                 row.Name,
		PairProbabilityMode:  row.PairProbabilityMode,
		InitialHandCount:     row.InitialHandCount,
		PairSize:             row.PairSize,
		ReplenishCount:       row.ReplenishCount,
		SecondDestinyEnabled: row.SecondDestinyEnabled,
		DestinyDoubleEnabled: row.DestinyDoubleEnabled,
		DestinyIncome:        row.DestinyIncome,
		DoubleDestinyBuff:    row.DoubleDestinyBuff,
		DestinyReplenishCost: row.DestinyReplenishCost,
		ResumeEnabled:        row.ResumeEnabled,
		HostModeEnabled:      row.HostModeEnabled,
		HostMinPairs:         row.HostMinPairs,
		HostShareBps:         row.HostShareBps,
		RevenueBase:          row.RevenueBase,
		LedgerType:           row.LedgerType,
		Status:               row.Status,
		CreatedAt:            mmTimeString(row.CreatedAt),
		UpdatedAt:            mmTimeString(row.UpdatedAt),
	}
}

func mmDeckTemplateToProto(row *repo.MemoryMatchDeckTemplateRow) *play.MemoryMatchDeckTemplate {
	if row == nil {
		return nil
	}
	return &play.MemoryMatchDeckTemplate{
		Id:         row.ID,
		Name:       row.Name,
		Status:     row.Status,
		TotalCards: row.TotalCards,
		Weight:     row.Weight,
		ConfigJson: row.ConfigJSON,
		CreatedAt:  mmTimeString(row.CreatedAt),
		UpdatedAt:  mmTimeString(row.UpdatedAt),
	}
}

func mmAppearanceToProto(row *repo.MemoryMatchAppearanceRow) *play.MemoryMatchAppearanceItem {
	if row == nil {
		return nil
	}
	return &play.MemoryMatchAppearanceItem{
		Id:             row.ID,
		Type:           row.AppearanceType,
		Name:           row.Name,
		ImageUrl:       row.ImageURL,
		BackImageUrl:   row.BackImageURL,
		Status:         row.Status,
		Free:           row.Free,
		ExchangePoints: row.ExchangePoints,
		FunctionType:   row.FunctionType,
		FunctionConfig: row.FunctionConfig,
		Weight:         row.Weight,
		CreatedAt:      mmTimeString(row.CreatedAt),
		UpdatedAt:      mmTimeString(row.UpdatedAt),
	}
}

func mmNormalizePage(page, pageSize int64) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return int(page), int(pageSize)
}

func mmValidateGlobalRules(in *play.MemoryMatchGlobalRules) error {
	if in == nil {
		return errorx.NewCodeError(errorx.CodeParamError, "规则不能为空")
	}
	switch {
	case in.InitialHandCount < 1 || in.InitialHandCount > 54:
		return errorx.NewCodeError(errorx.CodeParamError, "初始手牌数必须在 1-54 之间")
	case in.PairSize < 2 || in.PairSize > 54:
		return errorx.NewCodeError(errorx.CodeParamError, "几张成碰必须在 2-54 之间")
	case in.ReplenishCount < 0 || in.ReplenishCount > 54:
		return errorx.NewCodeError(errorx.CodeParamError, "碰后补牌数必须在 0-54 之间")
	case in.HostShareBps < 0 || in.HostShareBps > 10000:
		return errorx.NewCodeError(errorx.CodeParamError, "播主收益万分比必须在 0-10000 之间")
	case in.Status != 0 && in.Status != 1:
		return errorx.NewCodeError(errorx.CodeParamError, "状态只能是 0 或 1")
	}
	for _, value := range []int64{
		in.SecondDestinyEnabled, in.DestinyDoubleEnabled, in.ResumeEnabled, in.HostModeEnabled,
	} {
		if value != 0 && value != 1 {
			return errorx.NewCodeError(errorx.CodeParamError, "开关字段只能是 0 或 1")
		}
	}
	if in.DestinyIncome < 0 || in.DoubleDestinyBuff < 0 || in.DestinyReplenishCost < 0 {
		return errorx.NewCodeError(errorx.CodeParamError, "天命收益/消耗不能为负")
	}
	return nil
}

func mmValidateDeckTemplate(in *play.MemoryMatchDeckTemplate) error {
	if in == nil {
		return errorx.NewCodeError(errorx.CodeParamError, "牌堆不能为空")
	}
	if strings.TrimSpace(in.Name) == "" {
		return errorx.NewCodeError(errorx.CodeParamError, "牌堆名称不能为空")
	}
	if in.Status != 0 && in.Status != 1 {
		return errorx.NewCodeError(errorx.CodeParamError, "状态只能是 0 或 1")
	}
	if in.TotalCards < 1 || in.TotalCards > 54 {
		return errorx.NewCodeError(errorx.CodeParamError, "总牌数必须在 1-54 之间")
	}
	if in.Weight < 0 {
		return errorx.NewCodeError(errorx.CodeParamError, "权重不能为负")
	}
	if config := strings.TrimSpace(in.ConfigJson); config != "" && !json.Valid([]byte(config)) {
		return errorx.NewCodeError(errorx.CodeParamError, "牌堆扩展配置不是合法 JSON")
	}
	return nil
}

func mmValidateAppearance(in *play.MemoryMatchAppearanceItem) error {
	if in == nil {
		return errorx.NewCodeError(errorx.CodeParamError, "外观不能为空")
	}
	if in.Type != 1 && in.Type != 2 {
		return errorx.NewCodeError(errorx.CodeParamError, "外观类型只能是 1（卡面）或 2（卡背）")
	}
	if strings.TrimSpace(in.Name) == "" {
		return errorx.NewCodeError(errorx.CodeParamError, "外观名称不能为空")
	}
	if in.Status != 0 && in.Status != 1 {
		return errorx.NewCodeError(errorx.CodeParamError, "状态只能是 0 或 1")
	}
	if in.Free != 0 && in.Free != 1 {
		return errorx.NewCodeError(errorx.CodeParamError, "免费标记只能是 0 或 1")
	}
	if in.Weight < 0 {
		return errorx.NewCodeError(errorx.CodeParamError, "权重不能为负")
	}
	if points := strings.TrimSpace(in.ExchangePoints); points != "" {
		value, err := strconv.ParseFloat(points, 64)
		if err != nil || value < 0 {
			return errorx.NewCodeError(errorx.CodeParamError, "兑换积分必须是非负数字")
		}
	}
	return nil
}

func mmDBError(err error) error {
	if err == nil {
		return nil
	}
	var codeErr *errorx.CodeError
	if errors.As(err, &codeErr) {
		return err
	}
	return errorx.NewCodeError(errorx.CodeDBError, fmt.Sprintf("数据库错误: %v", err))
}
