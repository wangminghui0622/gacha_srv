package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"

	"github.com/zeromicro/go-zero/core/logx"
)

// defaultTenant 未指定租户时回落 1（单租户本地部署；JWT 通常已带 tenant_id）。
func defaultTenant(in uint64) uint64 {
	if in == 0 {
		return 1
	}
	return in
}

// toTokens TokenPair → proto 会话令牌。
func toTokens(pair *core.TokenPair) *ops.ConsoleSessionTokens {
	return &ops.ConsoleSessionTokens{
		AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, TokenType: "Bearer",
		AccessExpiresAtUnix: pair.AccessExpiresAt.Unix(), RefreshExpiresAtUnix: pair.RefreshExpiresAt.Unix(),
		SessionId: pair.SessionID,
	}
}

// loadRoleWithPerms 加载角色 + 权限行（不存在返回 repo.ErrNotFound 语义：ok=false）。
func loadRoleWithPerms(ctx context.Context, s *svc.ServiceContext, tenantID, roleID uint64) (*opsmodel.Roles, []*opsmodel.RolePermissions, bool, error) {
	role, err := s.RolesModel.FindOne(ctx, roleID)
	if repo.IsNotFound(err) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	if role.TenantId != tenantID {
		return nil, nil, false, nil
	}
	var perms []*opsmodel.RolePermissions
	err = s.DB.WithContext(ctx).Model(&opsmodel.RolePermissions{}).
		Where("`role_id` = ?", roleID).Order("`resource`, `action`").Find(&perms).Error
	if err != nil {
		return nil, nil, false, err
	}
	return role, perms, true, nil
}

// roleToProto 角色 + 权限行 → proto Role。
func roleToProto(role *opsmodel.Roles, perms []*opsmodel.RolePermissions) *ops.Role {
	out := &ops.Role{
		Id: role.Id, TenantId: role.TenantId, Code: role.Code, Name: role.Name,
		Enabled: role.Enabled == 1, Version: role.Version,
	}
	for _, p := range perms {
		out.Permissions = append(out.Permissions, &ops.RolePermission{
			Resource: p.Resource, Action: p.Action, DataScope: ops.DataScope(p.DataScope),
			ApprovalMode: ops.ApprovalMode(p.ApprovalMode), Condition: p.ConditionExpr,
			AllowedFields: parseFieldList(p.AllowedFields), DeniedFields: parseFieldList(p.DeniedFields),
		})
	}
	return out
}

func parseFieldList(raw string) []string {
	if raw == "" || raw == "null" {
		return nil
	}
	var fields []string
	_ = json.Unmarshal([]byte(raw), &fields)
	return fields
}

// attrToAny AttributeValue → Go 值（CEL 求值用）。
func attrToAny(v *ops.AttributeValue) any {
	if v == nil {
		return nil
	}
	switch val := v.Value.(type) {
	case *ops.AttributeValue_IntValue:
		return val.IntValue
	case *ops.AttributeValue_StringValue:
		return val.StringValue
	case *ops.AttributeValue_BoolValue:
		return val.BoolValue
	case *ops.AttributeValue_StringList:
		return val.StringList.GetValues()
	}
	return nil
}

// attrsToMap proto map → map[string]any。
func attrsToMap(in map[string]*ops.AttributeValue) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = attrToAny(v)
	}
	return out
}

// accountRoleCodes 服务端权威解析账号角色编码（不信 JWT 快照）。
func accountRoleCodes(ctx context.Context, s *svc.ServiceContext, tenantID, accountID uint64) []string {
	account, err := repo.NewAccountStore(s.DB).FindByID(ctx, tenantID, accountID)
	if err != nil {
		return nil
	}
	return account.RoleCodes
}

// writeAudit 尽力而为写审计（失败只记日志，不阻断主流程）。
func writeAudit(ctx context.Context, s *svc.ServiceContext, row *opsmodel.OpsAuditLogs) {
	if row == nil {
		return
	}
	if err := s.AuditModel.Insert(context.Background(), nil, row); err != nil {
		logx.WithContext(ctx).Errorf("审计写入失败 operation=%s: %v", row.OperationId, err)
	}
}

// stringsToJSON 简单字段数组序列化（写 role_permissions 用）。
func stringsToJSON(fields []string) string {
	if len(fields) == 0 {
		return "[]"
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// nowTime 时间注入点（测试可替换）。
var nowTime = time.Now

// trimNonEmpty 去空格。
func trimNonEmpty(s string) string {
	return strings.TrimSpace(s)
}
