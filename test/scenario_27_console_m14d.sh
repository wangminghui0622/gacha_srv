#!/bin/bash
# ============================================================
# 场景27：M14d 控制台五域全链路（admin gateway 9321 / game 9315 / base 9314 / payment 9313）
#
# 覆盖：SQL 复位 → 超管登录 → 6 角色 + 7 账号（含 noperm 403 负例）
#       → 活动：签到草稿→发布→玩家可见+签到积分 → v2 草稿→草稿上加草稿 409
#         → 发布 v2 → 重复发布 409（closed）→ 转盘草稿发布 slot 物化
#         → 兑换码草稿发布 code 物化+玩家兑换生效 → SQL 终态断言
#       → 条件政策：草稿→List/Get→修订 CAS 409→v2→发布 v1（hash CAS 409→200）
#         → EvaluatePolicy 拒绝/放行（failure_code 透出）→ 发布 v2 → v1 RETIRED + 按 v2 生效
#       → 券：发布模板（同 requestId 幂等/异内容 409）→ List/Get → 发券 ×2（AVAILABLE+7 天过期）
#         → 再发布 v2 → 两版本 → custom_amount 负例 → 发券走 v2 → loyalty defaults Get/Put
#       → 风控：SQL 插 2 案例 → 两页游标 → Claim → 重复 claim 409 → 非认领人 Resolve 409
#         → Resolve → 同 operation_id 幂等重放 → events 2 行 + 状态断言
#       → 支付路由：GET 404 → PUT 建 v1 → PUT CAS 409 → PUT v2 PINNED → audit 2 行断言
#       → 审计抽样（ops_audit_logs 按 operation_id）→ 现场清理
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景27：M14d 控制台五域全链路"

ADMIN_PASS="admin12345678"
OP_PASS="OpPass12345678"
SUF="${RANDOM}${RANDOM}${RANDOM}"
NOW_MS=$(($(date +%s) * 1000))
START_MS=$((NOW_MS - 86400000))
END_MS=$((NOW_MS + 2592000000))

# ---------- 1. SQL 复位（幂等重跑） ----------
print_step "1. SQL 复位（admin TOTP / e2e 账号角色 / 五域业务数据）"
do_mysql "UPDATE gacha_ops.console_accounts SET totp_secret='' WHERE username='admin';"
for u in e2e-m14d-cw e2e-m14d-ew e2e-m14d-rw1 e2e-m14d-rw2 e2e-m14d-pa e2e-m14d-rt e2e-m14d-np; do
    oid=$(do_mysql "SELECT id FROM gacha_ops.console_accounts WHERE username='${u}';")
    if [ -n "$oid" ]; then
        do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${oid}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${oid}; DELETE FROM gacha_ops.console_accounts WHERE id=${oid};"
    fi
done
for r in e2e-27-campaign e2e-27-eligibility e2e-27-risk e2e-27-promo e2e-27-route e2e-27-noperm; do
    rid=$(do_mysql "SELECT id FROM gacha_ops.roles WHERE code='${r}';")
    if [ -n "$rid" ]; then
        do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${rid}; DELETE FROM gacha_ops.roles WHERE id=${rid};"
    fi
done
do_mysql "DELETE FROM gacha_marketing.campaign_versions WHERE campaign_id IN (SELECT id FROM gacha_marketing.campaigns WHERE campaign_key LIKE 'e2e-%-27');"
do_mysql "DELETE FROM gacha_marketing.campaign_wheel_slots WHERE campaign_id IN (SELECT id FROM gacha_marketing.campaigns WHERE campaign_key LIKE 'e2e-%-27');"
do_mysql "DELETE FROM gacha_marketing.campaign_redemption_codes WHERE campaign_id IN (SELECT id FROM gacha_marketing.campaigns WHERE campaign_key LIKE 'e2e-%-27');"
do_mysql "DELETE FROM gacha_marketing.campaigns WHERE campaign_key LIKE 'e2e-%-27';"
do_mysql "DELETE FROM gacha_marketing.coupon_template_versions WHERE template_id=27; DELETE FROM gacha_marketing.coupon_template_heads WHERE template_id=27;"
do_mysql "DELETE FROM gacha_marketing.coupon_templates WHERE template_no='CONSOLE-27'; DELETE FROM gacha_marketing.user_coupons WHERE template_no='CONSOLE-27';"
do_mysql "DELETE FROM gacha_base.eligibility_policy_rules WHERE policy_id IN (SELECT id FROM gacha_base.eligibility_policies WHERE policy_code='e2e-pol-27');"
do_mysql "DELETE FROM gacha_base.eligibility_policy_versions WHERE policy_id IN (SELECT id FROM gacha_base.eligibility_policies WHERE policy_code='e2e-pol-27');"
do_mysql "DELETE FROM gacha_base.eligibility_policies WHERE policy_code='e2e-pol-27';"
do_mysql "DELETE FROM gacha_base.eligibility_decision_logs WHERE decision_no LIKE 'd_e2e-eval-%';"
do_mysql "DELETE FROM dtm_barrier.barrier WHERE gid LIKE 'e2e-eval-gid-%';"
do_mysql "DELETE FROM gacha_base.risk_case_events WHERE case_id LIKE 'rk_e2e27%';"
do_mysql "DELETE FROM gacha_base.risk_cases WHERE case_no LIKE 'rk_e2e27%';"
do_mysql "DELETE FROM gacha_money.payment_route_audits; DELETE FROM gacha_money.payment_route_candidates; DELETE FROM gacha_money.payment_route_policies WHERE tenant_id=1;"
do_mysql "UPDATE gacha_marketing.loyalty_offset_config SET points_per_yuan=10, max_ratio_bps=7000 WHERE id=1;"

# ---------- 2. 超管两步登录 ----------
print_step "2. 超管两步登录"
line=$(admin_login_begin admin "$ADMIN_PASS" e2e-fp-admin27) || exit 1
ticket=$(echo "$line" | cut -d' ' -f1)
secret=$(echo "$line" | cut -d' ' -f3)
[ "$(echo "$line" | cut -d' ' -f2)" == "true" ] || { print_error "admin 登录应走 setup"; exit $FAILED; }
token_admin=$(admin_login_complete admin "$ADMIN_PASS" e2e-fp-admin27 "$ticket" "$(admin_totp "$secret")") || exit 1
[ -n "$token_admin" ] || { print_error "超管第二步未返回 accessToken"; exit $FAILED; }

# ---------- 3. 角色创建 + 权限写入 ----------
print_step "3. 角色创建 + 权限写入（resource/action 对齐 adminop 19 操作）"
perm_put() { # perm_put <token> <roleId> <expectedVersion> <permsJson>
    admin_request PUT "/authorization/roles/$2/permissions" "{\"tenantId\":\"1\",\"expectedVersion\":\"$3\",\"permissions\":$4}" "$1"
}
P_ALL='"dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"'

role_cw=$(admin_role_create "$token_admin" e2e-27-campaign "E2E Campaign Writer") || exit 1
out=$(perm_put "$token_admin" "$role_cw" 1 "[{\"resource\":\"campaign.definition\",\"action\":\"write\",${P_ALL}},{\"resource\":\"campaign.definition\",\"action\":\"read\",${P_ALL}},{\"resource\":\"campaign.definition\",\"action\":\"publish\",${P_ALL}}]")
[ "$(admin_status "$out")" == "200" ] || { print_error "campaign 角色权限写入应 200: $(admin_body "$out")"; exit $FAILED; }

role_ew=$(admin_role_create "$token_admin" e2e-27-eligibility "E2E Eligibility Writer") || exit 1
out=$(perm_put "$token_admin" "$role_ew" 1 "[{\"resource\":\"eligibility.policy\",\"action\":\"read\",${P_ALL}},{\"resource\":\"eligibility.policy\",\"action\":\"create\",${P_ALL}},{\"resource\":\"eligibility.policy\",\"action\":\"write\",${P_ALL}},{\"resource\":\"eligibility.policy\",\"action\":\"publish\",${P_ALL}}]")
[ "$(admin_status "$out")" == "200" ] || { print_error "eligibility 角色权限写入应 200: $(admin_body "$out")"; exit $FAILED; }

role_rk=$(admin_role_create "$token_admin" e2e-27-risk "E2E Risk Worker") || exit 1
out=$(perm_put "$token_admin" "$role_rk" 1 "[{\"resource\":\"risk.case\",\"action\":\"list\",${P_ALL}},{\"resource\":\"risk.case\",\"action\":\"claim\",${P_ALL}},{\"resource\":\"risk.case\",\"action\":\"resolve\",${P_ALL}}]")
[ "$(admin_status "$out")" == "200" ] || { print_error "risk 角色权限写入应 200: $(admin_body "$out")"; exit $FAILED; }

role_pa=$(admin_role_create "$token_admin" e2e-27-promo "E2E Promo Admin") || exit 1
out=$(perm_put "$token_admin" "$role_pa" 1 "[{\"resource\":\"promotion.coupon_template\",\"action\":\"read\",${P_ALL}},{\"resource\":\"promotion.coupon_template\",\"action\":\"publish\",${P_ALL}},{\"resource\":\"promotion.user_coupon\",\"action\":\"grant\",${P_ALL}},{\"resource\":\"promotion.tenant_loyalty\",\"action\":\"read\",${P_ALL}},{\"resource\":\"promotion.tenant_loyalty\",\"action\":\"write\",${P_ALL}}]")
[ "$(admin_status "$out")" == "200" ] || { print_error "promo 角色权限写入应 200: $(admin_body "$out")"; exit $FAILED; }

role_rt=$(admin_role_create "$token_admin" e2e-27-route "E2E Route Writer") || exit 1
out=$(perm_put "$token_admin" "$role_rt" 1 "[{\"resource\":\"payment.route\",\"action\":\"read\",${P_ALL}},{\"resource\":\"payment.route\",\"action\":\"write\",${P_ALL}}]")
[ "$(admin_status "$out")" == "200" ] || { print_error "route 角色权限写入应 200: $(admin_body "$out")"; exit $FAILED; }

role_np=$(admin_role_create "$token_admin" e2e-27-noperm "E2E No Perm") || exit 1

# ---------- 4. 账号开通 + 角色绑定 + 登录 ----------
print_step "4. 账号开通 + 角色绑定 + 两步登录"
open_and_login() { # open_and_login <username> <roleId> → 输出 "principalId token"
    local u=$1 r=$2 line pid token
    line=$(admin_open_account "$token_admin" "$u" "$OP_PASS") || exit 1
    pid=$(echo "$line" | cut -d' ' -f1)
    out=$(admin_bind_roles "$token_admin" "$pid" 1 "$r")
    [ "$(admin_status "$out")" == "200" ] || { print_error "$u 绑角色应 200: $(admin_body "$out")"; exit $FAILED; }
    line=$(admin_login_begin "$u" "$OP_PASS" "e2e-fp-${u}") || exit 1
    local tk=$(echo "$line" | cut -d' ' -f1) secret=$(echo "$line" | cut -d' ' -f3)
    [ "$(echo "$line" | cut -d' ' -f2)" == "true" ] || { print_error "$u 登录应走 setup"; exit $FAILED; }
    token=$(admin_login_complete "$u" "$OP_PASS" "e2e-fp-${u}" "$tk" "$(admin_totp "$secret")") || exit 1
    echo "$pid $token"
}
cw_line=$(open_and_login e2e-m14d-cw "$role_cw"); cw_pid=$(echo "$cw_line" | cut -d' ' -f1); token_cw=$(echo "$cw_line" | cut -d' ' -f2)
ew_line=$(open_and_login e2e-m14d-ew "$role_ew"); ew_pid=$(echo "$ew_line" | cut -d' ' -f1); token_ew=$(echo "$ew_line" | cut -d' ' -f2)
rw1_line=$(open_and_login e2e-m14d-rw1 "$role_rk"); rw1_pid=$(echo "$rw1_line" | cut -d' ' -f1); token_rw1=$(echo "$rw1_line" | cut -d' ' -f2)
rw2_line=$(open_and_login e2e-m14d-rw2 "$role_rk"); rw2_pid=$(echo "$rw2_line" | cut -d' ' -f1); token_rw2=$(echo "$rw2_line" | cut -d' ' -f2)
pa_line=$(open_and_login e2e-m14d-pa "$role_pa"); pa_pid=$(echo "$pa_line" | cut -d' ' -f1); token_pa=$(echo "$pa_line" | cut -d' ' -f2)
rt_line=$(open_and_login e2e-m14d-rt "$role_rt"); rt_pid=$(echo "$rt_line" | cut -d' ' -f1); token_rt=$(echo "$rt_line" | cut -d' ' -f2)
np_line=$(open_and_login e2e-m14d-np "$role_np"); np_pid=$(echo "$np_line" | cut -d' ' -f1); token_np=$(echo "$np_line" | cut -d' ' -f2)

# ---------- 5. 403 负例（noperm + 跨域越权） ----------
print_step "5. 403 负例"
out=$(admin_request GET /risk/cases '' "$token_np")
[ "$(admin_status "$out")" == "403" ] || { print_error "noperm 读案件应 403，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "CONSOLE_FORBIDDEN" ] || { print_error "noperm reason 应 CONSOLE_FORBIDDEN，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /eligibility/policies '{"code":"x27","name":"x"}' "$token_cw")
[ "$(admin_status "$out")" == "403" ] || { print_error "campaign-writer 建政策应 403，实际: $(admin_status "$out")"; exit $FAILED; }
print_step "403 负例通过"

# ---------- 6. 活动（campaign） ----------
print_step "6. 活动：签到草稿→发布→玩家签到→v2→409→转盘→兑换码"
SIGN_CFG='{"signIn":{"rewards":[{"day":1,"loyaltyPoints":100},{"day":2,"loyaltyPoints":200},{"day":3,"loyaltyPoints":300}]}}'
out=$(admin_request POST /campaigns/drafts "{\"campaignKey\":\"e2e-sign-27\",\"kind\":1,\"name\":\"E2E签到27\",\"startAtUnixMillis\":${START_MS},\"endAtUnixMillis\":${END_MS},\"config\":${SIGN_CFG}}" "$token_cw")
[ "$(admin_status "$out")" == "200" ] || { print_error "签到草稿应 200: $(admin_body "$out")"; exit $FAILED; }
cid=$(extract_field "$(admin_body "$out")" campaignId)
[ -n "$cid" ] || { print_error "签到草稿应回 campaignId"; exit $FAILED; }
out=$(admin_request GET "/campaigns/${cid}" '' "$token_cw")
[ "$(admin_status "$out")" == "200" ] || { print_error "Get 草稿应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" state)" == "CAMPAIGN_STATE_DRAFT" ] || { print_error "草稿 state 应 DRAFT，实际: $(admin_body "$out")"; exit $FAILED; }

out=$(admin_request POST "/campaigns/${cid}/publish" '{}' "$token_cw")
[ "$(admin_status "$out")" == "200" ] || { print_error "发布应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" publishedVersion)" == "1" ] || { print_error "发布后 publishedVersion 应 1，实际: $(admin_body "$out")"; exit $FAILED; }
[ "$(do_mysql "SELECT published_version FROM gacha_marketing.campaigns WHERE id=${cid};")" == "1" ] || print_error "campaigns.published_version 应为 1"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_marketing.campaign_versions WHERE campaign_id=${cid};")" == "1" ] || print_error "campaign_versions 应 1 行"
[ "$(do_mysql "SELECT state FROM gacha_marketing.campaign_versions WHERE campaign_id=${cid} AND version=1;")" == "2" ] || print_error "版本行 state 应为 2 PUBLISHED"

# 玩家可见 + 签到积分
reg=$(register_user "e2e_m14d_") || exit 1
p1=$(echo "$reg" | cut -d' ' -f1)
p1_name=$(echo "$reg" | cut -d' ' -f2)
token_p1=$(login_by_name "$p1_name") || exit 1
reg2=$(register_user "e2e_m14d_") || exit 1
p2=$(echo "$reg2" | cut -d' ' -f1)
p2_name=$(echo "$reg2" | cut -d' ' -f2)
token_p2=$(login_by_name "$p2_name") || exit 1
clist=$(do_get /campaigns "$token_p1")
echo "$clist" | grep -q '"campaignKey":"e2e-sign-27"' || { print_error "玩家列表应可见 e2e-sign-27: ${clist}"; exit $FAILED; }
KEY_S="k27sign_${SUF}"
resp=$(participate_campaign "$token_p1" "e2e-sign-27" "" "$KEY_S")
[ "$(extract_code "$resp")" == "0" ] || { print_error "玩家签到失败: ${resp}"; exit $FAILED; }
echo "$resp" | grep -q '"points":100' || { print_error "day1 应奖 100 积分: ${resp}"; exit $FAILED; }
pts=$(get_loyalty_points "$p1")
[ "$pts" == "100" ] || { print_error "签到后积分应 100，实际: ${pts}"; exit $FAILED; }

# v2 草稿 → 草稿上加草稿 409 → 发布 v2 → 重复发布 409
out=$(admin_request POST /campaigns/drafts "{\"campaignId\":${cid},\"kind\":1,\"name\":\"E2E签到27v2\",\"startAtUnixMillis\":${START_MS},\"endAtUnixMillis\":${END_MS},\"config\":${SIGN_CFG}}" "$token_cw")
[ "$(admin_status "$out")" == "200" ] || { print_error "v2 草稿应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "2" ] || { print_error "v2 草稿 version 应 2，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /campaigns/drafts "{\"campaignId\":${cid},\"kind\":1,\"name\":\"E2E签到27v3\",\"startAtUnixMillis\":${START_MS},\"endAtUnixMillis\":${END_MS},\"config\":${SIGN_CFG}}" "$token_cw")
[ "$(admin_status "$out")" == "409" ] || { print_error "草稿上加草稿应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "CAMPAIGN_CONFLICT" ] || { print_error "草稿冲突 reason 应 CAMPAIGN_CONFLICT，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST "/campaigns/${cid}/publish" '{}' "$token_cw")
[ "$(admin_status "$out")" == "200" ] || { print_error "发布 v2 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" publishedVersion)" == "2" ] || { print_error "发布 v2 后 publishedVersion 应 2"; exit $FAILED; }
out=$(admin_request POST "/campaigns/${cid}/publish" '{}' "$token_cw")
[ "$(admin_status "$out")" == "409" ] || { print_error "重复发布应 409（closed），实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "CAMPAIGN_CONFLICT" ] || { print_error "重复发布 reason 应 CAMPAIGN_CONFLICT"; exit $FAILED; }

# 转盘草稿 → 发布 → slot 物化
WHEEL_CFG='{"prizeWheel":{"slots":[{"weight":50,"prizes":[{"kind":4,"amount":10}],"dailyStock":100},{"weight":50,"prizes":[{"kind":1,"amount":5}]}],"dailyLimit":5}}'
out=$(admin_request POST /campaigns/drafts "{\"campaignKey\":\"e2e-wheel-27\",\"kind\":13,\"name\":\"E2E转盘27\",\"startAtUnixMillis\":${START_MS},\"endAtUnixMillis\":${END_MS},\"config\":${WHEEL_CFG}}" "$token_cw")
[ "$(admin_status "$out")" == "200" ] || { print_error "转盘草稿应 200: $(admin_body "$out")"; exit $FAILED; }
wid=$(extract_field "$(admin_body "$out")" campaignId)
out=$(admin_request POST "/campaigns/${wid}/publish" '{}' "$token_cw")
[ "$(admin_status "$out")" == "200" ] || { print_error "转盘发布应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_marketing.campaign_wheel_slots WHERE campaign_id=${wid};")" == "2" ] || print_error "转盘 slot 物化应 2 行"
[ "$(do_mysql "SELECT kind FROM gacha_marketing.campaigns WHERE id=${wid};")" == "13" ] || print_error "campaigns.kind 应为 13"

# 兑换码草稿 → 发布 → code 物化 + 玩家兑换
CODE_CFG='{"redemption":{"codes":[{"code":"E2E27-CODE-A","loyaltyPoints":50,"maxRedemptions":10,"perUserLimit":1},{"code":"E2E27-CODE-B","maxRedemptions":1,"perUserLimit":1,"prizes":[{"kind":4,"amount":80}]}]}}'
out=$(admin_request POST /campaigns/drafts "{\"campaignKey\":\"e2e-code-27\",\"kind\":2,\"name\":\"E2E兑换码27\",\"startAtUnixMillis\":${START_MS},\"endAtUnixMillis\":${END_MS},\"config\":${CODE_CFG}}" "$token_cw")
[ "$(admin_status "$out")" == "200" ] || { print_error "兑换码草稿应 200: $(admin_body "$out")"; exit $FAILED; }
rcid=$(extract_field "$(admin_body "$out")" campaignId)
out=$(admin_request POST "/campaigns/${rcid}/publish" '{}' "$token_cw")
[ "$(admin_status "$out")" == "200" ] || { print_error "兑换码发布应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_marketing.campaign_redemption_codes WHERE campaign_id=${rcid};")" == "2" ] || print_error "兑换码物化应 2 行"
KEY_R="k27code_${SUF}"
resp=$(participate_campaign "$token_p1" "e2e-code-27" "E2E27-CODE-A" "$KEY_R")
[ "$(extract_code "$resp")" == "0" ] || { print_error "玩家兑换应成功: ${resp}"; exit $FAILED; }
pts=$(get_loyalty_points "$p1")
[ "$pts" == "150" ] || { print_error "兑换后积分应 150（100+50），实际: ${pts}"; exit $FAILED; }
print_step "活动域断言通过：版本行 2/物化 slot 2/code 2/积分 150"

# ---------- 7. 条件政策（eligibility） ----------
print_step "7. 条件政策：草稿→修订→发布（CAS）→评估→v2 生效"
POL_BODY_1="{\"code\":\"e2e-pol-27\",\"name\":\"E2E政策27\",\"scene\":3,\"combination\":1,\"rules\":[{\"ruleId\":\"r1\",\"expression\":\"facts.user_id != ${p1}\",\"failureCode\":\"E2E_USER_DENY\"}],\"requiredFacts\":[\"user_id\"]}"
out=$(admin_request POST /eligibility/policies "$POL_BODY_1" "$token_ew")
[ "$(admin_status "$out")" == "200" ] || { print_error "政策草稿应 200: $(admin_body "$out")"; exit $FAILED; }
pol_id=$(extract_field "$(admin_body "$out")" policyId)
[ -n "$pol_id" ] || { print_error "政策应回 policyId"; exit $FAILED; }
out=$(admin_request GET /eligibility/policies '' "$token_ew")
echo "$(admin_body "$out")" | grep -q '"code":"e2e-pol-27"' || { print_error "政策列表应含 e2e-pol-27"; exit $FAILED; }
out=$(admin_request GET "/eligibility/policies/${pol_id}/versions/1" '' "$token_ew")
[ "$(admin_status "$out")" == "200" ] || { print_error "Get 政策 v1 应 200: $(admin_body "$out")"; exit $FAILED; }
hash_v1=$(extract_field "$(admin_body "$out")" configHash)
[ -n "$hash_v1" ] || { print_error "Get v1 应回 configHash"; exit $FAILED; }

out=$(admin_request POST "/eligibility/policies/${pol_id}/revisions" "{\"name\":\"E2E政策27v2\",\"scene\":3,\"combination\":1,\"rules\":[{\"ruleId\":\"r1\",\"expression\":\"facts.user_id != ${p2}\",\"failureCode\":\"E2E_DENY2\"}],\"requiredFacts\":[\"user_id\"],\"expectedLatestVersion\":99}" "$token_ew")
[ "$(admin_status "$out")" == "409" ] || { print_error "修订 expectedLatestVersion 不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "POLICY_CONFLICT" ] || { print_error "修订冲突 reason 应 POLICY_CONFLICT"; exit $FAILED; }
out=$(admin_request POST "/eligibility/policies/${pol_id}/revisions" "{\"name\":\"E2E政策27v2\",\"scene\":3,\"combination\":1,\"rules\":[{\"ruleId\":\"r1\",\"expression\":\"facts.user_id != ${p2}\",\"failureCode\":\"E2E_DENY2\"}],\"requiredFacts\":[\"user_id\"],\"expectedLatestVersion\":1}" "$token_ew")
[ "$(admin_status "$out")" == "200" ] || { print_error "修订 v2 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "2" ] || { print_error "修订后版本应 2"; exit $FAILED; }

out=$(admin_request POST "/eligibility/policies/${pol_id}/versions/1/publish" '{"expectedConfigHash":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}' "$token_ew")
[ "$(admin_status "$out")" == "409" ] || { print_error "发布 hash 不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST "/eligibility/policies/${pol_id}/versions/1/publish" "{\"expectedConfigHash\":\"${hash_v1}\"}" "$token_ew")
[ "$(admin_status "$out")" == "200" ] || { print_error "发布 v1 应 200: $(admin_body "$out")"; exit $FAILED; }

ev=$(go run "${SCRIPT_DIR}/tools/evalpol/main.go" "$p1" DRAW "p1-${SUF}")
echo "$ev" | grep -q "allowed=false" || { print_error "uid1 评估应拒绝: ${ev}"; exit $FAILED; }
echo "$ev" | grep -q "E2E_USER_DENY" || { print_error "拒绝 reason 应含 failure_code E2E_USER_DENY: ${ev}"; exit $FAILED; }
ev2=$(go run "${SCRIPT_DIR}/tools/evalpol/main.go" "$p2" DRAW "p2-${SUF}")
echo "$ev2" | grep -q "allowed=true" || { print_error "uid2 评估应放行: ${ev2}"; exit $FAILED; }

hash_v2=$(extract_field "$(admin_body "$(admin_request GET "/eligibility/policies/${pol_id}/versions/2" '' "$token_ew")")" configHash)
out=$(admin_request POST "/eligibility/policies/${pol_id}/versions/2/publish" "{\"expectedConfigHash\":\"${hash_v2}\"}" "$token_ew")
[ "$(admin_status "$out")" == "200" ] || { print_error "发布 v2 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(do_mysql "SELECT status FROM gacha_base.eligibility_policy_versions WHERE policy_id=${pol_id} AND version=1;")" == "3" ] || print_error "v1 应 RETIRED(3)"
[ "$(do_mysql "SELECT current_version FROM gacha_base.eligibility_policies WHERE id=${pol_id};")" == "2" ] || print_error "head.current_version 应 2"
ev=$(go run "${SCRIPT_DIR}/tools/evalpol/main.go" "$p1" DRAW "p1b-${SUF}")
echo "$ev" | grep -q "allowed=true" || { print_error "v2 生效后 uid1 应放行: ${ev}"; exit $FAILED; }
ev2=$(go run "${SCRIPT_DIR}/tools/evalpol/main.go" "$p2" DRAW "p2b-${SUF}")
echo "$ev2" | grep -q "allowed=false" || { print_error "v2 生效后 uid2 应拒绝: ${ev2}"; exit $FAILED; }
echo "$ev2" | grep -q "E2E_DENY2" || { print_error "v2 拒绝 reason 应含 E2E_DENY2: ${ev2}"; exit $FAILED; }
print_step "条件政策断言通过：CAS 409×2/评估按版本切换/RETIRED"

# ---------- 8. 券（promotion） ----------
print_step "8. 券模板：发布（幂等/冲突）→ 发券 → v2 → loyalty"
TPL_BODY='{"templateId":"27","kind":1,"amountMinor":500,"currency":"CNY","funder":1,"validityDays":7}'
TPL_RID="e2e-tpl27-${SUF}"
out=$(admin_request_rid "$TPL_RID" POST /promotion/tenants/1/coupon-templates "$TPL_BODY" "$token_pa")
[ "$(admin_status "$out")" == "200" ] || { print_error "发布券模板应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "1" ] || { print_error "券模板版本应 1"; exit $FAILED; }
out=$(admin_request_rid "$TPL_RID" POST /promotion/tenants/1/coupon-templates "$TPL_BODY" "$token_pa")
[ "$(admin_status "$out")" == "200" ] || { print_error "同 requestId 重放应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "1" ] || { print_error "重放应回原版本 1"; exit $FAILED; }
out=$(admin_request_rid "$TPL_RID" POST /promotion/tenants/1/coupon-templates '{"templateId":"27","kind":1,"amountMinor":999,"currency":"CNY","funder":1,"validityDays":7}' "$token_pa")
[ "$(admin_status "$out")" == "409" ] || { print_error "同 requestId 异内容应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "COUPON_TEMPLATE_CONFLICT" ] || { print_error "券冲突 reason 应 COUPON_TEMPLATE_CONFLICT"; exit $FAILED; }
out=$(admin_request GET '/promotion/tenants/1/coupon-templates' '' "$token_pa")
echo "$(admin_body "$out")" | grep -q '"templateId":"27"' || { print_error "券列表应含模板 27"; exit $FAILED; }
out=$(admin_request GET '/promotion/tenants/1/coupon-templates/27' '' "$token_pa")
[ "$(extract_field "$(admin_body "$out")" amountMinor)" == "500" ] || { print_error "Get 模板 amountMinor 应 500"; exit $FAILED; }

out=$(admin_request_rid "e2e-grant27-a-${SUF}" POST '/promotion/tenants/1/coupon-grants' "{\"userId\":\"${p1}\",\"templateId\":\"27\",\"count\":2}" "$token_pa")
[ "$(admin_status "$out")" == "200" ] || { print_error "发券应 200: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q 'USER_COUPON_STATUS_AVAILABLE' || { print_error "发券回复应 AVAILABLE"; exit $FAILED; }
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE template_no='CONSOLE-27' AND user_id=${p1} AND status=1;")" == "2" ] || print_error "user_coupons 应 2 张 AVAILABLE"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE template_no='CONSOLE-27' AND user_id=${p1} AND expires_at > NOW() + INTERVAL 6 DAY AND expires_at < NOW() + INTERVAL 8 DAY;")" == "2" ] || print_error "过期时间应 now+7 天"

TPL2_BODY='{"templateId":"27","kind":1,"amountMinor":800,"currency":"CNY","funder":1,"validityDays":7}'
out=$(admin_request_rid "e2e-tpl27b-${SUF}" POST /promotion/tenants/1/coupon-templates "$TPL2_BODY" "$token_pa")
[ "$(admin_status "$out")" == "200" ] || { print_error "再发布 v2 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "2" ] || { print_error "v2 版本应 2"; exit $FAILED; }
out=$(admin_request GET '/promotion/tenants/1/coupon-templates' '' "$token_pa")
nv=$(echo "$(admin_body "$out")" | grep -o '"templateId":"27"' | wc -l | tr -d ' ')
[ "$nv" == "2" ] || { print_error "列表应含模板 27 两版本，实际 ${nv}"; exit $FAILED; }
[ "$(do_mysql "SELECT value FROM gacha_marketing.coupon_templates WHERE template_no='CONSOLE-27';")" == "800" ] || print_error "coupon_templates 物化 value 应为 800（v2）"
out=$(admin_request_rid "e2e-grant27-neg-${SUF}" POST '/promotion/tenants/1/coupon-grants' "{\"userId\":\"${p1}\",\"templateId\":\"27\",\"count\":1,\"customAmountMinor\":100}" "$token_pa")
[ "$(admin_status "$out")" == "400" ] || { print_error "FIXED_MINOR 带 customAmountMinor 应 400，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "COUPON_INVALID" ] || { print_error "负例 reason 应 COUPON_INVALID"; exit $FAILED; }
out=$(admin_request_rid "e2e-grant27-b-${SUF}" POST '/promotion/tenants/1/coupon-grants' "{\"userId\":\"${p1}\",\"templateId\":\"27\",\"count\":1}" "$token_pa")
[ "$(admin_status "$out")" == "200" ] || { print_error "v2 发券应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE template_no='CONSOLE-27' AND user_id=${p1} AND status=1;")" == "3" ] || print_error "累计应 3 张券"

out=$(admin_request GET '/promotion/tenants/1/loyalty-defaults' '' "$token_pa")
[ "$(extract_field "$(admin_body "$out")" loyaltyPointsPerMajor)" == "10" ] || { print_error "loyalty 默认应 10，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request PUT '/promotion/tenants/1/loyalty-defaults' '{"loyaltyPointsPerMajor":15,"maxGrossBps":8000}' "$token_pa")
[ "$(admin_status "$out")" == "200" ] || { print_error "PUT loyalty 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" loyaltyPointsPerMajor)" == "15" ] || { print_error "PUT 回读应 15"; exit $FAILED; }
[ "$(do_mysql "SELECT points_per_yuan FROM gacha_marketing.loyalty_offset_config WHERE id=1;")" == "15" ] || print_error "loyalty_offset_config.points_per_yuan 应 15"
[ "$(do_mysql "SELECT max_ratio_bps FROM gacha_marketing.loyalty_offset_config WHERE id=1;")" == "8000" ] || print_error "loyalty_offset_config.max_ratio_bps 应 8000"
do_mysql "UPDATE gacha_marketing.loyalty_offset_config SET points_per_yuan=10, max_ratio_bps=7000 WHERE id=1;"
print_step "券域断言通过：幂等/冲突/两版本/发券 3 张/物化 800/loyalty 落库复原"

# ---------- 9. 风控（risk） ----------
print_step "9. 风控：SQL 插案例 → 两页游标 → Claim → 冲突 → Resolve → 幂等"
# 游标断言要求 OPEN 案件恰为 A/B 两行：清掉本场景内真实产生的案件
# （步骤 6 转盘抽奖的风控评估会为 p1 落一条 OPEN 案，run_all 里其时间戳会落在
# A(NOW) 与 B(NOW-10min) 之间，污染第二页游标——独立跑时步长 >10 分钟才侥幸通过）。
do_mysql "DELETE FROM gacha_base.risk_case_events WHERE case_id NOT LIKE 'rk_e2e27%';"
do_mysql "DELETE FROM gacha_base.risk_cases WHERE case_no NOT LIKE 'rk_e2e27%';"
do_mysql "INSERT INTO gacha_base.risk_cases (case_no, rule_no, scene, user_id, box_id, amount_fen, status, decision_id, risk_score, action, assigned_actor_id, created_at)
          VALUES ('rk_e2e27B', 'E2E-27', 'DRAW', ${p1}, 0, 100, 1, 'E2E27B', 60, 3, 0, NOW() - INTERVAL 10 MINUTE),
                 ('rk_e2e27A', 'E2E-27', 'DRAW', ${p1}, 0, 100, 1, 'E2E27A', 60, 3, 0, NOW());"
out=$(admin_request GET '/risk/cases?pageSize=1' '' "$token_rw1")
[ "$(admin_status "$out")" == "200" ] || { print_error "案件列表应 200: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"caseId":"rk_e2e27A"' || { print_error "第 1 页应含 rk_e2e27A（最新）"; exit $FAILED; }
cur_ms=$(extract_field "$(admin_body "$out")" nextBeforeCreatedAtUnixMillis)
cur_id=$(extract_field "$(admin_body "$out")" nextBeforeCaseId)
[ -n "$cur_ms" ] || { print_error "第 1 页应回游标"; exit $FAILED; }
out=$(admin_request GET "/risk/cases?pageSize=1&beforeCreatedAtUnixMillis=${cur_ms}&beforeCaseId=${cur_id}" '' "$token_rw1")
echo "$(admin_body "$out")" | grep -q '"caseId":"rk_e2e27B"' || { print_error "第 2 页应含 rk_e2e27B，实际: $(admin_body "$out")"; exit $FAILED; }

KEY_C1="k27claim1_${SUF}"; KEY_C2="k27claim2_${SUF}"
out=$(admin_request POST /risk/cases/rk_e2e27A/claim '' "$token_rw1" "$KEY_C1")
[ "$(admin_status "$out")" == "200" ] || { print_error "认领应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" status)" == "RISK_CASE_STATUS_IN_REVIEW" ] || { print_error "认领后状态应 IN_REVIEW，实际: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" actorId)" == "${rw1_pid}" ] || { print_error "认领 actorId 应为 rw1(${rw1_pid})"; exit $FAILED; }
out=$(admin_request POST /risk/cases/rk_e2e27A/claim '' "$token_rw1" "$KEY_C2")
[ "$(admin_status "$out")" == "409" ] || { print_error "重复认领应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "RISK_CASE_CONFLICT" ] || { print_error "重复认领 reason 应 RISK_CASE_CONFLICT"; exit $FAILED; }

KEY_R3="k27res3_${SUF}"; KEY_R4="k27res4_${SUF}"
out=$(admin_request POST /risk/cases/rk_e2e27A/resolve '{"status":3,"resolutionCode":"closed-ok"}' "$token_rw2" "$KEY_R3")
[ "$(admin_status "$out")" == "409" ] || { print_error "非认领人处置应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /risk/cases/rk_e2e27A/resolve '{"status":3,"resolutionCode":"closed-ok"}' "$token_rw1" "$KEY_R4")
[ "$(admin_status "$out")" == "200" ] || { print_error "认领人处置应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" status)" == "RISK_CASE_STATUS_RESOLVED" ] || { print_error "处置后状态应 RESOLVED"; exit $FAILED; }
out=$(admin_request POST /risk/cases/rk_e2e27A/resolve '{"status":3,"resolutionCode":"closed-ok"}' "$token_rw1" "$KEY_R4")
[ "$(admin_status "$out")" == "200" ] || { print_error "同 operation_id 重放应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_base.risk_case_events WHERE case_id='rk_e2e27A';")" == "2" ] || print_error "events 应 2 行（CLAIMED+RESOLVED）"
[ "$(do_mysql "SELECT status FROM gacha_base.risk_cases WHERE case_no='rk_e2e27A';")" == "3" ] || print_error "risk_cases.status 应 3 RESOLVED"
[ "$(do_mysql "SELECT assigned_actor_id FROM gacha_base.risk_cases WHERE case_no='rk_e2e27A';")" == "${rw1_pid}" ] || print_error "assigned_actor_id 应为 rw1"
print_step "风控断言通过：游标两页/409×2/幂等重放/events 2 行"

# ---------- 10. 支付路由（payment） ----------
print_step "10. 支付路由：GET 404 → PUT v1 → CAS 409 → PUT v2 PINNED → audit"
out=$(admin_request GET '/payment/routes?channel=1&scene=1' '' "$token_rt")
[ "$(admin_status "$out")" == "404" ] || { print_error "GET 无行应 404，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "PAYMENT_ROUTE_NOT_FOUND" ] || { print_error "GET reason 应 PAYMENT_ROUTE_NOT_FOUND"; exit $FAILED; }
RT_CREATE='{"channel":1,"scene":1,"policy":{"mode":1},"candidates":[{"provider":1,"merchantAccountId":"10001","priority":1,"weight":100,"enabled":true},{"provider":2,"merchantAccountId":"10002","priority":2,"weight":100,"enabled":true}],"expectedVersion":0}'
out=$(admin_request PUT /payment/routes "$RT_CREATE" "$token_rt")
[ "$(admin_status "$out")" == "200" ] || { print_error "PUT 新建应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "1" ] || { print_error "新建 version 应 1"; exit $FAILED; }
rt_pid_id=$(extract_field "$(admin_body "$out")" policyId)
out=$(admin_request GET '/payment/routes?channel=1&scene=1' '' "$token_rt")
echo "$(admin_body "$out")" | grep -q 'PAYMENT_ROUTE_MODE_AUTO' || { print_error "GET 应回 AUTO v1"; exit $FAILED; }
out=$(admin_request PUT /payment/routes "{\"channel\":1,\"scene\":1,\"policy\":{\"policyId\":${rt_pid_id},\"mode\":1,\"version\":99},\"candidates\":[{\"provider\":1,\"merchantAccountId\":\"10001\",\"priority\":1,\"weight\":100,\"enabled\":true}],\"expectedVersion\":99}" "$token_rt")
[ "$(admin_status "$out")" == "409" ] || { print_error "PUT CAS 不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "PAYMENT_ROUTE_VERSION_CONFLICT" ] || { print_error "CAS reason 应 PAYMENT_ROUTE_VERSION_CONFLICT"; exit $FAILED; }
RT_V2="{\"channel\":1,\"scene\":1,\"policy\":{\"policyId\":${rt_pid_id},\"mode\":2,\"pinnedProvider\":1,\"pinnedMerchantAccountId\":\"10001\",\"version\":1},\"candidates\":[{\"provider\":1,\"merchantAccountId\":\"10001\",\"priority\":1,\"weight\":100,\"enabled\":true}],\"expectedVersion\":1}"
out=$(admin_request PUT /payment/routes "$RT_V2" "$token_rt")
[ "$(admin_status "$out")" == "200" ] || { print_error "PUT v2 PINNED 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "2" ] || { print_error "v2 version 应 2"; exit $FAILED; }
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_money.payment_route_policies WHERE tenant_id=1;")" == "1" ] || print_error "policies 应 1 行"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_money.payment_route_candidates;")" == "1" ] || print_error "candidates 应全量替换为 1 行"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_money.payment_route_audits;")" == "2" ] || print_error "audits 应 2 行"
[ "$(do_mysql "SELECT MIN(old_version) FROM gacha_money.payment_route_audits;")" == "0" ] || print_error "audit 首行 old_version 应 0"
[ "$(do_mysql "SELECT MAX(new_version) FROM gacha_money.payment_route_audits;")" == "2" ] || print_error "audit 末行 new_version 应 2"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_money.payment_route_audits WHERE operator_user_id=${rt_pid};")" == "2" ] || print_error "audit operator 应为 route-writer(${rt_pid})"
print_step "支付路由断言通过：v1/v2 CAS/audit 2 行/candidates 替换"

# ---------- 11. 审计抽样（ops_audit_logs） ----------
print_step "11. 审计抽样（ops_audit_logs 按 operation_id）"
audit_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_ops.ops_audit_logs WHERE request_id LIKE 'e2e-rid-%';")
[ "$audit_rows" -ge 25 ] || print_error "审计行数应 ≥25，实际: ${audit_rows}"
# 券模板/发券调用显式传了自定义请求 ID（e2e-tpl27-*/e2e-grant27-*，也是幂等键），
# 审计行的 request_id 随之存该值——断言前缀按调用实际取值，不能用统一的 e2e-rid-%（从零重建库无历史残留凑数）
for op in console.campaign.createdraft console.campaign.publish console.campaign.get \
          console.eligibility.create console.eligibility.createrevision console.eligibility.publish \
          console.risk.list console.risk.claim console.risk.resolve \
          console.promotion.publishcoupontemplate console.promotion.grantcoupons console.promotion.putloyalty \
          console.paymentroute.getpaymentroute console.paymentroute.putpaymentroute; do
    case "$op" in
        console.promotion.publishcoupontemplate) prefix='e2e-tpl27%' ;;
        console.promotion.grantcoupons)          prefix='e2e-grant27%' ;;
        *)                                       prefix='e2e-rid-%' ;;
    esac
    [ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='${op}' AND request_id LIKE '${prefix}' LIMIT 1;")" ] || print_error "审计应含 ${op} 行（prefix=${prefix}）"
done
print_step "审计抽样通过：14 类操作落库"

# ---------- 12. 现场清理 ----------
print_step "12. 现场清理（e2e 账号/角色/五域业务数据）"
for u in e2e-m14d-cw e2e-m14d-ew e2e-m14d-rw1 e2e-m14d-rw2 e2e-m14d-pa e2e-m14d-rt e2e-m14d-np; do
    oid=$(do_mysql "SELECT id FROM gacha_ops.console_accounts WHERE username='${u}';")
    if [ -n "$oid" ]; then
        do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${oid}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${oid}; DELETE FROM gacha_ops.console_accounts WHERE id=${oid};"
    fi
done
for r in e2e-27-campaign e2e-27-eligibility e2e-27-risk e2e-27-promo e2e-27-route e2e-27-noperm; do
    rid=$(do_mysql "SELECT id FROM gacha_ops.roles WHERE code='${r}';")
    if [ -n "$rid" ]; then
        do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${rid}; DELETE FROM gacha_ops.roles WHERE id=${rid};"
    fi
done
for u in "$p1" "$p2"; do
    do_mysql "DELETE FROM gacha_marketing.campaign_participations WHERE user_id=${u}; DELETE FROM gacha_marketing.campaign_sign_ins WHERE user_id=${u}; DELETE FROM gacha_marketing.campaign_prize_grants WHERE user_id=${u}; DELETE FROM gacha_marketing.user_coupons WHERE user_id=${u} AND template_no='CONSOLE-27';"
done
do_mysql "DELETE FROM gacha_marketing.campaign_versions WHERE campaign_id IN (SELECT id FROM gacha_marketing.campaigns WHERE campaign_key LIKE 'e2e-%-27');"
do_mysql "DELETE FROM gacha_marketing.campaign_wheel_slots WHERE campaign_id IN (SELECT id FROM gacha_marketing.campaigns WHERE campaign_key LIKE 'e2e-%-27');"
do_mysql "DELETE FROM gacha_marketing.campaign_redemption_codes WHERE campaign_id IN (SELECT id FROM gacha_marketing.campaigns WHERE campaign_key LIKE 'e2e-%-27');"
do_mysql "DELETE FROM gacha_marketing.campaigns WHERE campaign_key LIKE 'e2e-%-27';"
do_mysql "DELETE FROM gacha_marketing.coupon_template_versions WHERE template_id=27; DELETE FROM gacha_marketing.coupon_template_heads WHERE template_id=27;"
do_mysql "DELETE FROM gacha_marketing.coupon_templates WHERE template_no='CONSOLE-27';"
do_mysql "DELETE FROM gacha_base.eligibility_policy_rules WHERE policy_id IN (SELECT id FROM gacha_base.eligibility_policies WHERE policy_code='e2e-pol-27');"
do_mysql "DELETE FROM gacha_base.eligibility_policy_versions WHERE policy_id IN (SELECT id FROM gacha_base.eligibility_policies WHERE policy_code='e2e-pol-27');"
do_mysql "DELETE FROM gacha_base.eligibility_policies WHERE policy_code='e2e-pol-27';"
do_mysql "DELETE FROM gacha_base.eligibility_decision_logs WHERE decision_no LIKE 'd_e2e-eval-%';"
do_mysql "DELETE FROM dtm_barrier.barrier WHERE gid LIKE 'e2e-eval-gid-%';"
do_mysql "DELETE FROM gacha_base.risk_case_events WHERE case_id LIKE 'rk_e2e27%';"
do_mysql "DELETE FROM gacha_base.risk_cases WHERE case_no LIKE 'rk_e2e27%';"
do_mysql "DELETE FROM gacha_money.payment_route_audits; DELETE FROM gacha_money.payment_route_candidates; DELETE FROM gacha_money.payment_route_policies WHERE tenant_id=1;"

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}√ 场景27 M14d 控制台五域全链路 通过${NC}"
    exit 0
else
    echo -e "${RED}× 场景27 失败${NC}"
    exit 1
fi
