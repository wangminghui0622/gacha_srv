#!/bin/bash
# ============================================================
# 场景25：赏池草稿→发布全链路（M14b，admin gateway 9321 / draw-rpc 9312）
#
# 覆盖：赏池控制台 maker/checker 流程（语义照抄 server_v2 单审批）——
#       CreatePoolDraft（Idempotency-Key 幂等：缺头 400 / 同键同载荷重放
#       返回既有草稿 / 同键不同载荷 409）→ RequestPoolPublication
#       （requester=会话主体）→ 自审批 409 → ApprovePoolPublication
#       （approver=会话主体，≠requester）→ PublishPool（物化
#       draw_boxes/draw_pool_entries + 标记 PUBLISHED，已发布幂等）
#       → 玩家侧 /boxes、/boxes/:boxId 验证物化 → 未审批发布 409
#       → 扭蛋幸运指派（promotion-admin：ASSIGNED/CLEARED/非法奖 400）
#       → 权限负例（403 CONSOLE_FORBIDDEN）→ 审计落库 → 现场清理
#
# 断言打在 kratos 形错误体 {code,message,reason} 与 protojson 成功体上
# （int64=数字、bytes=Base64 字符串、枚举=名字面量）。
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景25：赏池草稿→发布全链路（pool console）"

ADMIN_PASS="admin12345678"   # ops-rpc.yaml Console.InitialAdminPassword（本地开发种子）

DRAFT1='{"box":{"boxName":"E2E赏池·庆典","price":100,"description":"M14b场景25","entries":[{"name":"谢谢参与","weight":60,"quantity":-1},{"name":"金币x50","weight":30,"quantity":5000,"isPrize":true,"displayValueFen":100},{"name":"限定皮肤","weight":10,"quantity":100,"isPrize":true,"displayValueFen":5000,"rollingLimited":true}],"rollingRulesJson":"{\"high_value_threshold_fen\":3000,\"single_draw_excluded_percent\":0,\"user_excluded_percent\":0}"}}'
DRAFT2='{"box":{"boxName":"E2E赏池·纪念","price":50,"description":"未审批发布负例","entries":[{"name":"谢谢参与","weight":90,"quantity":-1},{"name":"金币x10","weight":10,"quantity":100,"isPrize":true,"displayValueFen":100}]}}'

# ---------- 1. SQL 复位管理现场（幂等重跑） ----------
print_step "1. SQL 复位（admin TOTP；e2e 账号/角色；赏池草稿/物化；幸运指派）"
do_mysql "UPDATE gacha_ops.console_accounts SET totp_secret='' WHERE username='admin';"
for u in e2e-pool-maker e2e-pool-approver e2e-pool-noperm e2e-pool-promo; do
    oid=$(do_mysql "SELECT id FROM gacha_ops.console_accounts WHERE username='${u}';")
    if [ -n "$oid" ]; then
        do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${oid}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${oid}; DELETE FROM gacha_ops.console_accounts WHERE id=${oid};"
    fi
done
for c in e2e-pool-maker e2e-pool-approver e2e-pool-noperm; do
    rid=$(do_mysql "SELECT id FROM gacha_ops.roles WHERE code='${c}';")
    if [ -n "$rid" ]; then
        do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${rid}; DELETE FROM gacha_ops.roles WHERE id=${rid};"
    fi
done
do_mysql "DELETE FROM gacha_draw.draw_pool_publication_approvals WHERE pool_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_pool_draft_requests WHERE pool_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_pool_versions WHERE pool_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_pool_heads WHERE pool_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_pool_entries WHERE box_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_boxes WHERE id >= 1000000;"
do_mysql "DELETE FROM gacha_play.gashapon_lucky_assignments WHERE config_id=1 AND user_id=777;"

# ---------- 2. 超管登录 ----------
print_step "2. 超管两步登录（setup TOTP）"
line=$(admin_login_begin admin "$ADMIN_PASS" e2e-fp-admin) || exit 1
ticket=$(echo "$line" | cut -d' ' -f1)
secret=$(echo "$line" | cut -d' ' -f3)
[ "$(echo "$line" | cut -d' ' -f2)" == "true" ] || print_error "复位后应 setupRequired=true，实际: ${line}"
token_admin=$(admin_login_complete admin "$ADMIN_PASS" e2e-fp-admin "$ticket" "$(admin_totp "$secret")") || exit 1
[ -n "$token_admin" ] || print_error "超管第二步未返回 accessToken"

# ---------- 3. 角色 + 权限（maker/checker + 无权限对照） ----------
print_step "3. 角色与权限：maker（create/read/request/approve/publish）、approver（read/approve）、noperm（无池权限）"
maker_role=$(admin_role_create "$token_admin" e2e-pool-maker "E2E Pool Maker") || exit 1
approver_role=$(admin_role_create "$token_admin" e2e-pool-approver "E2E Pool Approver") || exit 1
noperm_role=$(admin_role_create "$token_admin" e2e-pool-noperm "E2E No Perm") || exit 1
# maker 特意持有 approve_publication：自审批应被 draw-rpc 业务规则
# （requester==approver → 409）拒绝，而不是被权限中间件 403 拦住
maker_perms='[{"resource":"gacha.pool","action":"create","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"},{"resource":"gacha.pool","action":"read","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"},{"resource":"gacha.pool","action":"request_publication","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"},{"resource":"gacha.pool","action":"approve_publication","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"},{"resource":"gacha.pool","action":"publish","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"}]'
approver_perms='[{"resource":"gacha.pool","action":"read","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"},{"resource":"gacha.pool","action":"approve_publication","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"}]'
out=$(admin_request PUT "/authorization/roles/${maker_role}/permissions" "{\"tenantId\":\"1\",\"expectedVersion\":\"1\",\"permissions\":${maker_perms}}" "$token_admin")
[ "$(admin_status "$out")" == "200" ] || print_error "maker 权限写入应 200，实际: $(admin_body "$out")"
out=$(admin_request PUT "/authorization/roles/${approver_role}/permissions" "{\"tenantId\":\"1\",\"expectedVersion\":\"1\",\"permissions\":${approver_perms}}" "$token_admin")
[ "$(admin_status "$out")" == "200" ] || print_error "approver 权限写入应 200，实际: $(admin_body "$out")"

# ---------- 4. 开通账号 + 绑定角色 ----------
print_step "4. 开通账号 maker/approver/noperm/promo 并绑角色（promo 绑 promotion-admin）"
maker_line=$(admin_open_account "$token_admin" e2e-pool-maker "MakerPass1234") || exit 1
approver_line=$(admin_open_account "$token_admin" e2e-pool-approver "ApproverPass1234") || exit 1
noperm_line=$(admin_open_account "$token_admin" e2e-pool-noperm "NoPermPass1234") || exit 1
promo_line=$(admin_open_account "$token_admin" e2e-pool-promo "PromoPass1234") || exit 1
maker_id=$(echo "$maker_line" | cut -d' ' -f1)
approver_id=$(echo "$approver_line" | cut -d' ' -f1)
noperm_id=$(echo "$noperm_line" | cut -d' ' -f1)
promo_id=$(echo "$promo_line" | cut -d' ' -f1)
out=$(admin_bind_roles "$token_admin" "$maker_id" 1 "$maker_role")
[ "$(admin_status "$out")" == "200" ] || print_error "maker 绑角色应 200，实际: $(admin_body "$out")"
out=$(admin_bind_roles "$token_admin" "$approver_id" 1 "$approver_role")
[ "$(admin_status "$out")" == "200" ] || print_error "approver 绑角色应 200，实际: $(admin_body "$out")"
out=$(admin_bind_roles "$token_admin" "$noperm_id" 1 "$noperm_role")
[ "$(admin_status "$out")" == "200" ] || print_error "noperm 绑角色应 200，实际: $(admin_body "$out")"
promo_rid=$(promo_role_id) || exit 1
[ -n "$promo_rid" ] || { print_error "未查到种子角色 promotion-admin"; exit 1; }
out=$(admin_bind_roles "$token_admin" "$promo_id" 1 "$promo_rid")
[ "$(admin_status "$out")" == "200" ] || print_error "promo 绑 promotion-admin(${promo_rid}) 应 200，实际: $(admin_body "$out")"

# ---------- 5. 三方登录 ----------
print_step "5. maker/approver/noperm/promo 两步登录（setup TOTP）"
token_maker=$(admin_login e2e-pool-maker "MakerPass1234") || exit 1
token_approver=$(admin_login e2e-pool-approver "ApproverPass1234") || exit 1
token_noperm=$(admin_login e2e-pool-noperm "NoPermPass1234") || exit 1
token_promo=$(admin_login e2e-pool-promo "PromoPass1234") || exit 1
[ -n "$token_maker" ] && [ -n "$token_approver" ] && [ -n "$token_noperm" ] && [ -n "$token_promo" ] || print_error "四方登录应全部拿到 token"

# ---------- 6. 权限负例 ----------
print_step "6. 权限负例：越权 403 CONSOLE_FORBIDDEN"
out=$(admin_request GET '/authorization/roles?tenantId=1' '' "$token_maker")
[ "$(admin_status "$out")" == "403" ] || print_error "maker 读角色应 403，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "CONSOLE_FORBIDDEN" ] || print_error "越权 reason 应 CONSOLE_FORBIDDEN，实际: $(admin_body "$out")"
out=$(admin_request POST /gacha/pools/drafts "$DRAFT1" "$token_approver")
[ "$(admin_status "$out")" == "403" ] || print_error "approver 无 create 建草稿应 403，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request POST /gacha/pools/drafts "$DRAFT1" "$token_noperm")
[ "$(admin_status "$out")" == "403" ] || print_error "noperm 建草稿应 403，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request GET /gacha/pools '' "$token_noperm")
[ "$(admin_status "$out")" == "403" ] || print_error "noperm 读版本列表应 403，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request POST /gacha/gashapons/1/lucky-assignment '{"prizeIndex":2,"userId":777}' "$token_maker")
[ "$(admin_status "$out")" == "403" ] || print_error "maker 无 assign_lucky 指派应 403，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request GET /gacha/pools '' "$token_maker")
[ "$(admin_status "$out")" == "200" ] || print_error "maker 读版本列表应 200，实际: $(admin_status "$out") $(admin_body "$out")"

# ---------- 7. 创建草稿（幂等键三态） ----------
print_step "7. 创建草稿：缺 Idempotency-Key 400 → 首建 200 → 同键同载荷重放幂等 → 同键异载荷 409"
out=$(admin_request POST /gacha/pools/drafts "$DRAFT1" "$token_maker")
[ "$(admin_status "$out")" == "400" ] || print_error "缺 Idempotency-Key 应 400，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "IDEMPOTENCY_KEY_REQUIRED" ] || print_error "缺头 reason 应 IDEMPOTENCY_KEY_REQUIRED，实际: $(admin_body "$out")"
out=$(admin_request POST /gacha/pools/drafts "$DRAFT1" "$token_maker" "e2e-pool-draft-1")
[ "$(admin_status "$out")" == "200" ] || print_error "建草稿应 200，实际: $(admin_status "$out") $(admin_body "$out")"
body=$(admin_body "$out")
pool1=$(extract_field "$body" poolId)
hash1=$(extract_field "$body" configHash)
[ "$pool1" -ge 1000000 ] || print_error "新池 poolId 应 ≥1000000，实际: ${pool1}"
[ "$(extract_field "$body" version)" == "1" ] || print_error "首版应为 1，实际: $(extract_field "$body" version)"
[ "$(extract_field "$body" status)" == "POOL_STATUS_DRAFT" ] || print_error "新草稿应 DRAFT，实际: $(extract_field "$body" status)"
[ "${#hash1}" -ge 40 ] || print_error "configHash 应为 Base64(SHA256)，实际: ${hash1}"
out=$(admin_request POST /gacha/pools/drafts "$DRAFT1" "$token_maker" "e2e-pool-draft-1")
[ "$(admin_status "$out")" == "200" ] || print_error "同键同载荷重放应 200，实际: $(admin_status "$out") $(admin_body "$out")"
replay=$(admin_body "$out")
[ "$(extract_field "$replay" poolId)" == "$pool1" ] || print_error "重放应返回同一 poolId，实际: $(extract_field "$replay" poolId)"
[ "$(extract_field "$replay" configHash)" == "$hash1" ] || print_error "重放应返回同一 configHash"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_draw.draw_pool_versions WHERE pool_id=${pool1};")" == "1" ] || print_error "重放不应新增版本行"
out=$(admin_request POST /gacha/pools/drafts "$DRAFT2" "$token_maker" "e2e-pool-draft-1")
[ "$(admin_status "$out")" == "409" ] || print_error "同键异载荷应 409，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "POOL_DRAFT_REQUEST_CONFLICT" ] || print_error "同键异载荷 reason 应 POOL_DRAFT_REQUEST_CONFLICT，实际: $(admin_body "$out")"
[ -z "$(do_mysql "SELECT id FROM gacha_draw.draw_boxes WHERE id=${pool1};")" ] || print_error "草稿阶段不应物化 draw_boxes"

# ---------- 8. 版本读取 200/404 ----------
print_step "8. 版本读取 200/404"
out=$(admin_request GET "/gacha/pools/${pool1}/versions/1" '' "$token_maker")
[ "$(admin_status "$out")" == "200" ] || print_error "读版本应 200，实际: $(admin_status "$out") $(admin_body "$out")"
vbody=$(admin_body "$out")
echo "$vbody" | grep -q '"boxName":"E2E赏池·庆典"' || print_error "版本应含 boxName，实际: ${vbody}"
[ "$(extract_field "$vbody" configHash)" == "$hash1" ] || print_error "版本 configHash 应与创建一致"
out=$(admin_request GET "/gacha/pools/${pool1}/versions/999" '' "$token_maker")
[ "$(admin_status "$out")" == "404" ] || print_error "读不存在版本应 404，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "POOL_VERSION_NOT_FOUND" ] || print_error "不存在版本 reason 应 POOL_VERSION_NOT_FOUND，实际: $(admin_body "$out")"

# ---------- 9. 提请发布（requester=会话主体） ----------
print_step "9. 提请发布：哈希守卫 409 → 首请 200 → 同请求重放幂等"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/publication-request" '{"expectedConfigHash":"AAAA"}' "$token_maker")
[ "$(admin_status "$out")" == "409" ] || print_error "哈希不符提请应 409，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "POOL_VERSION_CONFLICT" ] || print_error "哈希不符 reason 应 POOL_VERSION_CONFLICT，实际: $(admin_body "$out")"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/publication-request" "{\"expectedConfigHash\":\"${hash1}\"}" "$token_maker")
[ "$(admin_status "$out")" == "200" ] || print_error "提请发布应 200，实际: $(admin_status "$out") $(admin_body "$out")"
req_body=$(admin_body "$out")
echo "$req_body" | grep -q '"approved":true' && print_error "提请后不应已批准，实际: ${req_body}"
[ "$(extract_field "$req_body" requesterId)" == "$maker_id" ] || print_error "requester 应为 maker 会话主体 ${maker_id}，实际: ${req_body}"
[ -z "$(extract_field "$req_body" approverId)" ] || print_error "提请后 approverId 应为 0（protojson 省略），实际: ${req_body}"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/publication-request" "{\"expectedConfigHash\":\"${hash1}\"}" "$token_maker")
[ "$(admin_status "$out")" == "200" ] || print_error "同请求重放应 200，实际: $(admin_status "$out") $(admin_body "$out")"
echo "$(admin_body "$out")" | grep -q '"approved":true' && print_error "重放应返回既有待审批行（不应已批准）"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_draw.draw_pool_publication_approvals WHERE pool_id=${pool1} AND version=1;")" == "1" ] || print_error "审批表应恰 1 行"

# ---------- 10. 审批（单审批，审批人 ≠ 申请人） ----------
print_step "10. 审批：自审批 409 → 哈希不符 409 → approver 批准 200 → 重批幂等"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/approval" "{\"expectedConfigHash\":\"${hash1}\"}" "$token_maker")
[ "$(admin_status "$out")" == "409" ] || print_error "自审批应 409，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "POOL_PUBLICATION_CONFLICT" ] || print_error "自审批 reason 应 POOL_PUBLICATION_CONFLICT，实际: $(admin_body "$out")"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/approval" '{"expectedConfigHash":"AAAA"}' "$token_approver")
[ "$(admin_status "$out")" == "409" ] || print_error "审批哈希不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/approval" "{\"expectedConfigHash\":\"${hash1}\"}" "$token_approver")
[ "$(admin_status "$out")" == "200" ] || print_error "批准应 200，实际: $(admin_status "$out") $(admin_body "$out")"
app_body=$(admin_body "$out")
[ "$(extract_field "$app_body" approved)" == "true" ] || print_error "批准后 approved 应为 true，实际: ${app_body}"
[ "$(extract_field "$app_body" approverId)" == "$approver_id" ] || print_error "approver 应为会话主体 ${approver_id}，实际: ${app_body}"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/approval" "{\"expectedConfigHash\":\"${hash1}\"}" "$token_approver")
[ "$(admin_status "$out")" == "200" ] || print_error "同人重批应 200 幂等，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" approved)" == "true" ] || print_error "重批应返回既有已批准行"
[ "$(do_mysql "SELECT status FROM gacha_draw.draw_pool_publication_approvals WHERE pool_id=${pool1} AND version=1;")" == "2" ] || print_error "审批行 status 应为 2(APPROVED)"

# ---------- 11. 发布（物化 + 已发布幂等） ----------
print_step "11. 发布：哈希不符 409 → 发布 200（物化）→ 重放幂等（不再物化）"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/publish" '{"expectedConfigHash":"AAAA"}' "$token_maker")
[ "$(admin_status "$out")" == "409" ] || print_error "发布哈希不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/publish" "{\"expectedConfigHash\":\"${hash1}\"}" "$token_maker")
[ "$(admin_status "$out")" == "200" ] || print_error "发布应 200，实际: $(admin_status "$out") $(admin_body "$out")"
pub_body=$(admin_body "$out")
[ "$(extract_field "$pub_body" status)" == "POOL_STATUS_PUBLISHED" ] || print_error "发布后状态应 PUBLISHED，实际: ${pub_body}"
[ "$(extract_field "$pub_body" publishedAtUnixMillis)" -gt 0 ] || print_error "发布时间应 >0，实际: ${pub_body}"
[ "$(do_mysql "SELECT status FROM gacha_draw.draw_pool_versions WHERE pool_id=${pool1} AND version=1;")" == "2" ] || print_error "版本行 status 应为 2(PUBLISHED)"
[ -n "$(do_mysql "SELECT published_at FROM gacha_draw.draw_pool_versions WHERE pool_id=${pool1} AND version=1 AND published_at IS NOT NULL;")" ] || print_error "版本行 published_at 应为非空"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_draw.draw_boxes WHERE id=${pool1} AND status=1;")" == "1" ] || print_error "draw_boxes 应物化 1 行"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_draw.draw_pool_entries WHERE box_id=${pool1};")" == "3" ] || print_error "draw_pool_entries 应物化 3 行"
[ "$(do_mysql "SELECT remaining FROM gacha_draw.draw_pool_entries WHERE box_id=${pool1} AND \`name\`='金币x50';")" == "5000" ] || print_error "金币x50 remaining 应为 5000"
[ "$(do_mysql "SELECT remaining FROM gacha_draw.draw_pool_entries WHERE box_id=${pool1} AND \`name\`='谢谢参与';")" == "-1" ] || print_error "谢谢参与 remaining 应为 -1"
out=$(admin_request POST "/gacha/pools/${pool1}/versions/1/publish" "{\"expectedConfigHash\":\"${hash1}\"}" "$token_maker")
[ "$(admin_status "$out")" == "200" ] || print_error "已发布重放应 200 幂等，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" status)" == "POOL_STATUS_PUBLISHED" ] || print_error "重放应返回已发布状态"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_draw.draw_boxes WHERE id=${pool1};")" == "1" ] || print_error "重放不应重复物化"

# ---------- 12. 玩家侧验证物化 ----------
print_step "12. 玩家侧验证：/boxes 列表 + /boxes/:boxId 概率公示"
boxes=$(do_get /boxes)
echo "$boxes" | grep -q '"boxName":"E2E赏池·庆典"' || print_error "玩家宝箱列表应含新池，实际: ${boxes}"
detail=$(do_get "/boxes/${pool1}")
[ -n "$detail" ] || print_error "玩家宝箱详情应为非空"
echo "$detail" | grep -q '"boxName":"E2E赏池·庆典"' || print_error "玩家详情应含 boxName，实际: ${detail}"
echo "$detail" | grep -q '"name":"限定皮肤","weight":10' || print_error "玩家详情应含限定皮肤条目，实际: ${detail}"
echo "$detail" | grep -q '"probability":"10.00%"' || print_error "限定皮肤概率应 10.00%，实际: ${detail}"
echo "$detail" | grep -q '"price":"1.00"' || print_error "价格应为 1.00 元，实际: ${detail}"

# ---------- 13. 未审批发布负例（第二个草稿） ----------
print_step "13. 草稿2：提请后未经审批直接发布应 409，且不物化"
out=$(admin_request POST /gacha/pools/drafts "$DRAFT2" "$token_maker" "e2e-pool-draft-2")
[ "$(admin_status "$out")" == "200" ] || print_error "草稿2 创建应 200，实际: $(admin_status "$out") $(admin_body "$out")"
pool2=$(extract_field "$(admin_body "$out")" poolId)
hash2=$(extract_field "$(admin_body "$out")" configHash)
[ "$pool2" -gt "$pool1" ] || print_error "草稿2 poolId 应大于草稿1，实际: ${pool2}"
out=$(admin_request POST "/gacha/pools/${pool2}/versions/1/publication-request" "{\"expectedConfigHash\":\"${hash2}\"}" "$token_maker")
[ "$(admin_status "$out")" == "200" ] || print_error "草稿2 提请应 200，实际: $(admin_body "$out")"
out=$(admin_request POST "/gacha/pools/${pool2}/versions/1/publish" "{\"expectedConfigHash\":\"${hash2}\"}" "$token_maker")
[ "$(admin_status "$out")" == "409" ] || print_error "未审批发布应 409，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "POOL_PUBLICATION_CONFLICT" ] || print_error "未审批发布 reason 应 POOL_PUBLICATION_CONFLICT，实际: $(admin_body "$out")"
[ -z "$(do_mysql "SELECT id FROM gacha_draw.draw_boxes WHERE id=${pool2};")" ] || print_error "未审批发布不应物化 draw_boxes"
[ "$(do_mysql "SELECT status FROM gacha_draw.draw_pool_versions WHERE pool_id=${pool2};")" == "1" ] || print_error "草稿2 版本行应仍为 DRAFT"

# ---------- 13b. 版本列表游标分页（两池两版本行） ----------
print_step "13b. 版本列表游标分页（pageSize=1 两页）"
out=$(admin_request GET '/gacha/pools?pageSize=1' '' "$token_maker")
[ "$(admin_status "$out")" == "200" ] || print_error "列表应 200，实际: $(admin_status "$out") $(admin_body "$out")"
lbody=$(admin_body "$out")
[ "$(echo "$lbody" | grep -c '"poolId"')" == "1" ] || print_error "pageSize=1 应只回 1 行，实际: ${lbody}"
next_pool=$(extract_field "$lbody" nextAfterPoolId)
[ "$next_pool" -gt 0 ] || print_error "首页应给出 nextAfterPoolId，实际: ${next_pool}"
next_ver=$(extract_field "$lbody" nextAfterVersion)
out=$(admin_request GET "/gacha/pools?afterPoolId=${next_pool}&afterVersion=${next_ver}&pageSize=1" '' "$token_maker")
[ "$(admin_status "$out")" == "200" ] || print_error "末页请求应 200，实际: $(admin_status "$out") $(admin_body "$out")"
lbody=$(admin_body "$out")
[ "$(echo "$lbody" | grep -c '"poolId"')" == "1" ] || print_error "末页应回最后 1 行，实际: ${lbody}"
[ -z "$(extract_field "$lbody" nextAfterPoolId)" ] || print_error "末页 nextAfterPoolId 应为 0（protojson 省略），实际: ${lbody}"

# ---------- 14. 扭蛋幸运指派（promotion-admin：gacha.pool/assign_lucky） ----------
print_step "14. 幸运指派：ASSIGNED → SQL 落库 → 重放幂等 → 非法奖 400 → 清除 CLEARED"
out=$(admin_request POST /gacha/gashapons/1/lucky-assignment '{"prizeIndex":2,"userId":777}' "$token_promo")
[ "$(admin_status "$out")" == "200" ] || print_error "指派应 200，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" status)" == "ASSIGNED" ] || print_error "指派回复应为 ASSIGNED，实际: $(admin_body "$out")"
[ "$(do_mysql "SELECT user_id FROM gacha_play.gashapon_lucky_assignments WHERE config_id=1 AND prize_index=2;")" == "777" ] || print_error "指派应落库 user_id=777"
out=$(admin_request POST /gacha/gashapons/1/lucky-assignment '{"prizeIndex":2,"userId":777}' "$token_promo")
[ "$(admin_status "$out")" == "200" ] || print_error "重复指派应 200 幂等，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" status)" == "ASSIGNED" ] || print_error "重复指派回复应为 ASSIGNED"
out=$(admin_request POST /gacha/gashapons/1/lucky-assignment '{"prizeIndex":1,"userId":777}' "$token_promo")
[ "$(admin_status "$out")" == "400" ] || print_error "非 LUCKY 奖指派应 400，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "LUCKY_ASSIGN_INVALID" ] || print_error "非法奖 reason 应 LUCKY_ASSIGN_INVALID，实际: $(admin_body "$out")"
out=$(admin_request POST /gacha/gashapons/1/lucky-assignment '{"prizeIndex":2,"userId":0}' "$token_promo")
[ "$(admin_status "$out")" == "200" ] || print_error "清除指派应 200，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" status)" == "CLEARED" ] || print_error "清除回复应为 CLEARED，实际: $(admin_body "$out")"
[ -z "$(do_mysql "SELECT user_id FROM gacha_play.gashapon_lucky_assignments WHERE config_id=1 AND prize_index=2;")" ] || print_error "清除后指派行应删除"

# ---------- 15. 审计落库断言 ----------
print_step "15. 审计落库（ops_audit_logs 按 request_id 断言 pool 操作）"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.pool.create' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.pool.create 行"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.pool.requestpublication' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.pool.requestpublication 行"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.pool.approvepublication' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.pool.approvepublication 行"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.pool.publish' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.pool.publish 行"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.pool.assignluckyuser' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.pool.assignluckyuser 行"

# ---------- 16. 现场清理 ----------
print_step "16. 现场清理（e2e 账号/角色；赏池草稿/物化；幸运指派）"
for pair in "$maker_id e2e-pool-maker" "$approver_id e2e-pool-approver" "$noperm_id e2e-pool-noperm" "$promo_id e2e-pool-promo"; do
    set -- $pair
    do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=$1; DELETE FROM gacha_ops.principal_roles WHERE account_id=$1; DELETE FROM gacha_ops.console_accounts WHERE id=$1;"
done
for pair in "$maker_role e2e-pool-maker" "$approver_role e2e-pool-approver" "$noperm_role e2e-pool-noperm"; do
    set -- $pair
    do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=$1; DELETE FROM gacha_ops.roles WHERE id=$1;"
done
do_mysql "DELETE FROM gacha_draw.draw_pool_publication_approvals WHERE pool_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_pool_draft_requests WHERE pool_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_pool_versions WHERE pool_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_pool_heads WHERE pool_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_pool_entries WHERE box_id >= 1000000;"
do_mysql "DELETE FROM gacha_draw.draw_boxes WHERE id >= 1000000;"
do_mysql "DELETE FROM gacha_play.gashapon_lucky_assignments WHERE config_id=1 AND user_id=777;"

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}场景25通过：赏池草稿→发布全链路（幂等/单审批/物化/指派/审计）${NC}"
    exit 0
else
    echo -e "${RED}场景25失败${NC}"
    exit 1
fi
