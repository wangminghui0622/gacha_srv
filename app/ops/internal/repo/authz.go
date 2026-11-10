package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/ops"
	adminop "gacha_srv/pkg/adminop"

	"gorm.io/gorm"
)

// rulePriority 角色权限行的统一优先级（照抄 server_v2 authorization_rule：1000）。
const rulePriority = 1000

// AuthzStore 授权规则读取（角色权限行 → core.AuthzRule 物化）。
type AuthzStore struct {
	db *gorm.DB
}

// NewAuthzStore 构造授权规则存储。
func NewAuthzStore(db *gorm.DB) *AuthzStore {
	return &AuthzStore{db: db}
}

// LoadRulesForRoles 加载启用角色的全部权限行，物化为授权规则。
// 规则 ID 照抄 server_v2："rbac.<role_code>.<resource>.<action>"。
func (s *AuthzStore) LoadRulesForRoles(ctx context.Context, tenantID uint64, roleCodes []string) ([]*core.AuthzRule, error) {
	if len(roleCodes) == 0 {
		return nil, nil
	}
	var rows []struct {
		Code          string
		Resource      string
		Action        string
		DataScope     int64
		ApprovalMode  int64
		ConditionExpr string
		AllowedFields string
		DeniedFields  string
	}
	err := s.db.WithContext(ctx).
		Table("`role_permissions` AS p").
		Select("r.code, p.resource, p.action, p.data_scope, p.approval_mode, p.condition_expr, p.allowed_fields, p.denied_fields").
		Joins("JOIN `roles` AS r ON r.id = p.role_id AND r.enabled = 1 AND r.tenant_id = ?", tenantID).
		Where("r.code IN ?", roleCodes).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	rules := make([]*core.AuthzRule, 0, len(rows))
	for _, row := range rows {
		allowed, err := parseFields(row.AllowedFields)
		if err != nil {
			return nil, fmt.Errorf("角色 %s 权限 %s/%s 白名单字段无效: %w", row.Code, row.Resource, row.Action, err)
		}
		denied, err := parseFields(row.DeniedFields)
		if err != nil {
			return nil, fmt.Errorf("角色 %s 权限 %s/%s 黑名单字段无效: %w", row.Code, row.Resource, row.Action, err)
		}
		rules = append(rules, &core.AuthzRule{
			RuleID:    fmt.Sprintf("rbac.%s.%s.%s", row.Code, row.Resource, row.Action),
			RoleCodes: []string{row.Code}, Resource: row.Resource, Action: row.Action,
			Condition: row.ConditionExpr,
			DataScope: ops.DataScope(row.DataScope), AllowedFields: allowed, DeniedFields: denied,
			ApprovalMode: ops.ApprovalMode(row.ApprovalMode), Priority: rulePriority,
		})
	}
	return rules, nil
}

func parseFields(raw string) ([]string, error) {
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var fields []string
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// EnsureConsolePermission 校验权限目录（pkg/adminop 共享目录），未知拒绝。
func EnsureConsolePermission(resource, action string) error {
	if !adminop.IsConsolePermission(resource, action) {
		return fmt.Errorf("未知权限 %s/%s", resource, action)
	}
	return nil
}
