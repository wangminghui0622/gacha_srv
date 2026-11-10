#!/bin/bash
# ============================================================
# 场景24：管理控制台全链路（M14a，admin gateway 9321 / ops-rpc 9318）
#
# 覆盖：健康检查 → 缺 X-Request-Id 400 → 超管两步登录（TOTP setup 绑定）
#       → /session/permissions 通配授权 → 角色 CRUD + 版本冲突 409
#       → 权限写入（未知权限 400 / 合法权限枚举 wire 格式）
#       → 开通控制台账号（不自动绑角色，role_code 契约常量）
#       → 主体角色绑定（版本乐观锁 0 拒绝/冲突/成功）→ 促销管理员两步登录
#       → 越权 403 CONSOLE_FORBIDDEN → 权限清单 → TOTP 重置后再 setup
#       → 登出吊销 401（会话已吊销）→ 无令牌/伪令牌 401
#       → 审计落库（ops_audit_logs 按 request_id 断言）→ 现场清理
#
# 断言打在 kratos 形错误体 {code,message,reason} 与 protojson 成功体上
# （D9 信封例外：HTTP 状态码 = code；成功体 camelCase、uint64 为字符串）。
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景24：管理控制台全链路（admin console）"

ADMIN_PASS="admin12345678"   # ops-rpc.yaml Console.InitialAdminPassword（本地开发种子）

# ---------- 1. SQL 复位管理现场（幂等重跑） ----------
print_step "1. SQL 复位管理现场（admin TOTP 清空走 setup；清 e2e 残留）"
do_mysql "UPDATE gacha_ops.console_accounts SET totp_secret='' WHERE username='admin';"
e2e_oid=$(do_mysql "SELECT id FROM gacha_ops.console_accounts WHERE username='e2e-console-op';")
if [ -n "$e2e_oid" ]; then
    do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${e2e_oid}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${e2e_oid}; DELETE FROM gacha_ops.console_accounts WHERE id=${e2e_oid};"
fi
e2e_rid=$(do_mysql "SELECT id FROM gacha_ops.roles WHERE code='e2e-console-role';")
if [ -n "$e2e_rid" ]; then
    do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${e2e_rid}; DELETE FROM gacha_ops.roles WHERE id=${e2e_rid};"
fi

# ---------- 2. 健康检查 + 缺 X-Request-Id ----------
print_step "2. 健康检查 + 缺 X-Request-Id 拒绝"
out=$(admin_request GET /health)
[ "$(admin_status "$out")" == "200" ] || print_error "健康检查应 200，实际: $(admin_status "$out")"
[ "$(admin_body "$out")" == '{"status":"ok"}' ] || print_error "健康检查体应为 {\"status\":\"ok\"}，实际: $(admin_body "$out")"
out=$(curl -s -w '\n%{http_code}' -X POST -H "Content-Type: application/json" -d '{}' "${ADMIN_URL}/session/login")
[ "$(admin_status "$out")" == "400" ] || print_error "缺 X-Request-Id 应 400，实际: $(admin_status "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "CONSOLE_REQUEST_ID_MISSING" ] || print_error "缺 X-Request-Id reason 应 CONSOLE_REQUEST_ID_MISSING，实际: $(admin_body "$out")"

# ---------- 3. 超管两步登录（setup 绑定 TOTP） ----------
print_step "3. 超管两步登录（首次 setup：挑战携带密钥与 otpauth URI）"
line=$(admin_login_begin admin "$ADMIN_PASS" e2e-fp-admin) || exit 1
ticket=$(echo "$line" | cut -d' ' -f1)
setup=$(echo "$line" | cut -d' ' -f2)
secret=$(echo "$line" | cut -d' ' -f3)
[ "$setup" == "true" ] || print_error "复位后应 setupRequired=true，实际: ${setup}"
[ ${#secret} -eq 32 ] || print_error "setup 挑战应返回 32 位 Base32 密钥，实际: ${secret}"
token_admin=$(admin_login_complete admin "$ADMIN_PASS" e2e-fp-admin "$ticket" "$(admin_totp "$secret")") || exit 1
[ -n "$token_admin" ] || print_error "超管第二步未返回 accessToken"

# ---------- 4. 权限清单 + 角色列表（授权链） ----------
print_step "4. /session/permissions 通配授权 + 角色列表"
perm=$(admin_body "$(admin_request GET /session/permissions '' "$token_admin")")
[ "$(admin_status "$(admin_request GET /session/permissions '' "$token_admin")")" == "200" ] || print_error "权限清单应 200，实际: ${perm}"
echo "$perm" | grep -q '"resource":"\*","action":"\*"' || print_error "超管应有 */* 通配授权，实际: ${perm}"
roles=$(admin_roles "$token_admin")
echo "$roles" | grep -q '"code":"super_admin"' || print_error "角色列表应含 super_admin，实际: ${roles}"
echo "$roles" | grep -q '"code":"promotion-admin"' || print_error "角色列表应含 promotion-admin，实际: ${roles}"

# ---------- 5. 角色 CRUD + 版本乐观锁 ----------
print_step "5. 角色 CRUD + 版本冲突"
role_id=$(admin_role_create "$token_admin" e2e-console-role "E2E Console Role") || exit 1
out=$(admin_request POST /authorization/roles "{\"tenantId\":\"1\",\"code\":\"e2e-console-role\",\"name\":\"Dup\"}" "$token_admin")
[ "$(admin_status "$out")" == "409" ] || print_error "角色重码应 409，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "CONSOLE_ROLE_CODE_EXISTS" ] || print_error "重码 reason 应 CONSOLE_ROLE_CODE_EXISTS，实际: $(admin_body "$out")"
out=$(admin_request PUT "/authorization/roles/${role_id}" "{\"tenantId\":\"1\",\"name\":\"E2E v2\",\"expectedVersion\":\"99\"}" "$token_admin")
[ "$(admin_status "$out")" == "409" ] || print_error "角色版本冲突应 409，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "CONSOLE_VERSION_CONFLICT" ] || print_error "冲突 reason 应 CONSOLE_VERSION_CONFLICT，实际: $(admin_body "$out")"
out=$(admin_request PUT "/authorization/roles/${role_id}" "{\"tenantId\":\"1\",\"name\":\"E2E v2\",\"expectedVersion\":\"1\"}" "$token_admin")
[ "$(admin_status "$out")" == "200" ] || print_error "正确版本更新应 200，实际: $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" version)" == "2" ] || print_error "更新后版本应为 2，实际: $(admin_body "$out")"

# ---------- 6. 权限写入（未知权限拒绝 + 合法枚举 wire 格式） ----------
print_step "6. 权限写入：未知权限 400、合法权限枚举（DATA_SCOPE_ALL/APPROVAL_MODE_NONE）"
out=$(admin_request PUT "/authorization/roles/${role_id}/permissions" "{\"tenantId\":\"1\",\"expectedVersion\":\"2\",\"permissions\":[{\"resource\":\"catalog.spu\",\"action\":\"read\",\"dataScope\":\"DATA_SCOPE_ALL\",\"approvalMode\":\"APPROVAL_MODE_NONE\"}]}" "$token_admin")
[ "$(admin_status "$out")" == "400" ] || print_error "未知权限应 400，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "CONSOLE_UNKNOWN_PERMISSION" ] || print_error "未知权限 reason 应 CONSOLE_UNKNOWN_PERMISSION，实际: $(admin_body "$out")"
out=$(admin_request PUT "/authorization/roles/${role_id}/permissions" "{\"tenantId\":\"1\",\"expectedVersion\":\"2\",\"permissions\":[{\"resource\":\"promotion.coupon_template\",\"action\":\"read\",\"dataScope\":\"DATA_SCOPE_ALL\",\"approvalMode\":\"APPROVAL_MODE_NONE\"}]}" "$token_admin")
[ "$(admin_status "$out")" == "200" ] || print_error "合法权限写入应 200，实际: $(admin_body "$out")"
echo "$(admin_body "$out")" | grep -q '"dataScope":"DATA_SCOPE_ALL"' || print_error "权限回读应含枚举名字面量 DATA_SCOPE_ALL，实际: $(admin_body "$out")"

# ---------- 7. 开通控制台账号（不自动绑角色） ----------
print_step "7. 开通账号 e2e-console-op（role_code 契约常量，不自动绑角色）"
op_line=$(admin_open_account "$token_admin" e2e-console-op "OpPass12345678") || exit 1
op_id=$(echo "$op_line" | cut -d' ' -f1)
op_role=$(echo "$op_line" | cut -d' ' -f2)
[ "$op_role" == "promotion-admin" ] || print_error "开通回复 role_code 应为 promotion-admin，实际: ${op_role}"
[ "$(do_mysql "SELECT role_version FROM gacha_ops.console_accounts WHERE id=${op_id};")" == "1" ] || print_error "新账号 role_version 应为 1（server_v2 DEFAULT 1），实际: $(do_mysql "SELECT role_version FROM gacha_ops.console_accounts WHERE id=${op_id};")"
[ -z "$(do_mysql "SELECT account_id FROM gacha_ops.principal_roles WHERE account_id=${op_id};")" ] || print_error "开通不应自动绑角色"

# ---------- 8. 主体角色绑定（乐观锁：0 拒绝/冲突/成功） ----------
print_step "8. 绑定主体角色：expectedVersion=0 拒绝 → 版本冲突 409 → 绑定成功"
promo_rid=$(promo_role_id) || exit 1
[ -n "$promo_rid" ] || { print_error "未查到种子角色 promotion-admin"; exit 1; }
out=$(admin_bind_roles "$token_admin" "$op_id" 0 "$promo_rid")
[ "$(admin_status "$out")" == "400" ] || print_error "expectedVersion=0 应 400，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_bind_roles "$token_admin" "$op_id" 2 "$promo_rid")
[ "$(admin_status "$out")" == "409" ] || print_error "错误版本应 409，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_bind_roles "$token_admin" "$op_id" 1 "$promo_rid")
[ "$(admin_status "$out")" == "200" ] || print_error "绑定应 200，实际: $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" version)" == "2" ] || print_error "绑定后版本应为 2，实际: $(admin_body "$out")"
[ "$(do_mysql "SELECT role_id FROM gacha_ops.principal_roles WHERE account_id=${op_id};")" == "$promo_rid" ] || print_error "principal_roles 应写入 promotion-admin(${promo_rid})"

# ---------- 9. 促销管理员两步登录 → 越权 403 + 权限清单 ----------
print_step "9. e2e-console-op 两步登录（setup）→ 越权 403 → 权限清单"
op_line=$(admin_login_begin e2e-console-op "OpPass12345678" e2e-fp-op) || exit 1
op_ticket=$(echo "$op_line" | cut -d' ' -f1)
op_secret=$(echo "$op_line" | cut -d' ' -f3)
[ "$(echo "$op_line" | cut -d' ' -f2)" == "true" ] || print_error "新账号登录应走 setup，实际: ${op_line}"
token_op=$(admin_login_complete e2e-console-op "OpPass12345678" e2e-fp-op "$op_ticket" "$(admin_totp "$op_secret")") || exit 1
out=$(admin_request GET '/authorization/roles?tenantId=1' '' "$token_op")
[ "$(admin_status "$out")" == "403" ] || print_error "促销管理员读角色应 403，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "CONSOLE_FORBIDDEN" ] || print_error "越权 reason 应 CONSOLE_FORBIDDEN，实际: $(admin_body "$out")"
op_perm=$(admin_body "$(admin_request GET /session/permissions '' "$token_op")")
echo "$op_perm" | grep -q '"resource":"promotion.coupon_template","action":"read"' || print_error "op 权限应含 promotion.coupon_template/read，实际: ${op_perm}"
echo "$op_perm" | grep -q '"resource":"authorization.role"' && print_error "op 权限不应含 authorization.role，实际: ${op_perm}"

# ---------- 10. TOTP 重置后再 setup ----------
print_step "10. 管理员重置 op TOTP → 下次登录重新 setup"
out=$(admin_reset_totp "$token_admin" "$op_id")
[ "$(admin_status "$out")" == "200" ] || print_error "重置 TOTP 应 200，实际: $(admin_body "$out")"
op_line2=$(admin_login_begin e2e-console-op "OpPass12345678" e2e-fp-op2) || exit 1
[ "$(echo "$op_line2" | cut -d' ' -f2)" == "true" ] || print_error "重置后登录应重新 setup，实际: ${op_line2}"
# 现场复原：op 重新绑定 TOTP（避免残留 setup 态干扰后续步骤）
op_ticket2=$(echo "$op_line2" | cut -d' ' -f1)
op_secret2=$(echo "$op_line2" | cut -d' ' -f3)
token_op2=$(admin_login_complete e2e-console-op "OpPass12345678" e2e-fp-op2 "$op_ticket2" "$(admin_totp "$op_secret2")") || exit 1
[ -n "$token_op2" ] || print_error "重置后完成 setup 登录应拿到 token"

# ---------- 11. 登出吊销 + 无令牌/伪令牌 ----------
print_step "11. 登出吊销 401 → 无令牌 401 → 伪令牌 401"
out=$(admin_logout "$token_op2")
[ "$(admin_status "$out")" == "200" ] || print_error "登出应 200，实际: $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" revoked)" == "true" ] || print_error "登出回复 revoked 应为 true，实际: $(admin_body "$out")"
out=$(admin_request GET /session/permissions '' "$token_op2")
[ "$(admin_status "$out")" == "401" ] || print_error "吊销后应 401，实际: $(admin_status "$out") $(admin_body "$out")"
echo "$(admin_body "$out")" | grep -q "会话已吊销" || print_error "吊销后 message 应含'会话已吊销'，实际: $(admin_body "$out")"
out=$(admin_request GET '/authorization/roles?tenantId=1')
[ "$(admin_status "$out")" == "401" ] || print_error "无令牌应 401，实际: $(admin_status "$out")"
out=$(admin_request GET '/authorization/roles?tenantId=1' '' "fake.token.value")
[ "$(admin_status "$out")" == "401" ] || print_error "伪令牌应 401，实际: $(admin_status "$out")"

# ---------- 12. 审计落库断言 ----------
print_step "12. 审计落库（ops_audit_logs 按 request_id 断言 authorization.role.write）"
audit_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_ops.ops_audit_logs WHERE request_id LIKE 'e2e-rid-%';")
[ "$audit_rows" -ge 10 ] || print_error "审计行数应 ≥10，实际: ${audit_rows}"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.authorization.role.create' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.authorization.role.create 行"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.authorization.role.update' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.authorization.role.update 行"

# ---------- 13. 现场清理 ----------
print_step "13. 现场清理（e2e 账号/角色/会话）"
do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${op_id}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${op_id}; DELETE FROM gacha_ops.console_accounts WHERE id=${op_id};"
do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${role_id}; DELETE FROM gacha_ops.roles WHERE id=${role_id};"

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}场景24通过：管理控制台全链路（登录/TOTP/授权/角色/账号/审计/吊销）${NC}"
    exit 0
else
    echo -e "${RED}场景24失败${NC}"
    exit 1
fi
