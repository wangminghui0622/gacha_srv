#!/bin/bash
# ============================================================
# 场景28：M14e 任务计划 + 报表聚合（admin gateway 9321 / ops-rpc 9318）
#
# 覆盖：SQL 复位（task 三表回 bootstrap、报表 e2e 行清空）→ 超管登录
#       → 4 角色 + 4 账号（A 申请人/B 审批发布/C noperm/D SELF 数据范围）
#       → 403 负例（C 读任务/指标、A 无 publish 权限）
#       → 任务计划：草稿 v2 → 过期 latest 409 → v3（异内容）→ Get/List/404
#         → 提请（伪造 requesterId 被会话覆盖/错哈希 409/重放 200）→ 审批读取
#         → B 批准（伪造 approverId 被覆盖）→ A 自批 409 → B 发布 → v1 RETIRED
#         → 错哈希发布 409 → 发布重放 200 → v4 结构化券奖励 → 提请/批准 → 发布 400 NOT_READY
#       → ListTasks：userId=0 400 → 空进度视图（progress 0/IN_PROGRESS）
#         → pageSize=1 游标两页 → period 过滤空
#       → 报表：指标目录 5 项 → 看板创建 v1（AUTO_INCREMENT id）→ CAS v2 → 过期 CAS 409
#         → widget 重叠 400 → 未发布查询 400 → 发布 CAS 409/200 → 查询两结果 → 重放 cacheHit
#         → 不存在 404
#       → 盈亏曲线：空态 {} → 非法粒度 400 → 时间窗 400 → tenant 不符 403
#       → D（SELF）查询看板 403 DATA_SCOPE_DENIED → 审计 14 操作抽样 → 现场清理
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景28：M14e 任务计划 + 报表聚合"

ADMIN_PASS="admin12345678"
OP_PASS="OpPass12345678"
TASK_UID=280000001
# 32 字节零哈希（base64），用于错哈希负例
ZERO_HASH="AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
# first_field <json> <field>：提取首个出现的字段（TaskPlan 顶层 version 在嵌套任务 version 之前；
# ^[{][^{]* 允许首字符 { 但不跨过嵌套对象，避免 greedy 命中任务内的同名字段）
first_field() {
    echo "$1" | sed -n "s/^[{][^{]*\"$2\":\([^,}]*\).*/\1/p" | tr -d '"'
}

# ---------- 1. SQL 复位（幂等重跑） ----------
print_step "1. SQL 复位（admin TOTP / e2e 账号角色 / task 三表回 bootstrap / 报表 e2e 行）"
do_mysql "UPDATE gacha_ops.console_accounts SET totp_secret='' WHERE username='admin';"
for u in e2e-m14e-a e2e-m14e-b e2e-m14e-c e2e-m14e-d; do
    oid=$(do_mysql "SELECT id FROM gacha_ops.console_accounts WHERE username='${u}';")
    if [ -n "$oid" ]; then
        do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${oid}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${oid}; DELETE FROM gacha_ops.console_accounts WHERE id=${oid};"
    fi
done
for r in e2e-28-task e2e-28-approver e2e-28-noperm e2e-28-self; do
    rid=$(do_mysql "SELECT id FROM gacha_ops.roles WHERE code='${r}';")
    if [ -n "$rid" ]; then
        do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${rid}; DELETE FROM gacha_ops.roles WHERE id=${rid};"
    fi
done
# task 计划三表 → bootstrap（v1 PUBLISHED + head 1/1，审批清空）
do_mysql "DELETE FROM gacha_ops.task_plan_publication_approval;"
do_mysql "DELETE FROM gacha_ops.task_plan_version;"
do_mysql "INSERT INTO gacha_ops.task_plan_version (version, status, config_hash, config_blob, created_at, published_at)
          VALUES (1, 2, UNHEX('c4ea9a722bcccc403081206cf2e0ad6c9525f8452f2891da3d6bbecaa4d7fe06'),
                  UNHEX('1a22080110011a0ce6af8fe697a5e799bbe5bd952002280330014206542d424f4f544801'), NOW(3), NOW(3));"
do_mysql "UPDATE gacha_ops.task_plan_head SET current_version=1, latest_version=1, updated_at=NOW(3) WHERE singleton_id=1;"
do_mysql "DELETE FROM gacha_ops.reporting_dashboard_widget WHERE dashboard_id IN (SELECT id FROM gacha_ops.reporting_dashboard WHERE dashboard_code LIKE 'e2e-%');"
do_mysql "DELETE FROM gacha_ops.reporting_dashboard WHERE dashboard_code LIKE 'e2e-%';"

# ---------- 2. 超管两步登录 ----------
print_step "2. 超管两步登录"
line=$(admin_login_begin admin "$ADMIN_PASS" e2e-fp-admin28) || exit 1
ticket=$(echo "$line" | cut -d' ' -f1)
secret=$(echo "$line" | cut -d' ' -f3)
[ "$(echo "$line" | cut -d' ' -f2)" == "true" ] || { print_error "admin 登录应走 setup"; exit $FAILED; }
token_admin=$(admin_login_complete admin "$ADMIN_PASS" e2e-fp-admin28 "$ticket" "$(admin_totp "$secret")") || exit 1
[ -n "$token_admin" ] || { print_error "超管第二步未返回 accessToken"; exit $FAILED; }

# ---------- 3. 角色创建 + 权限写入 ----------
print_step "3. 角色创建 + 权限写入（resource/action 对齐 M14e 14 操作）"
perm_put() { # perm_put <token> <roleId> <expectedVersion> <permsJson>
    admin_request PUT "/authorization/roles/$2/permissions" "{\"tenantId\":\"1\",\"expectedVersion\":\"$3\",\"permissions\":$4}" "$1"
}
P_ALL='"dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"'

role_task=$(admin_role_create "$token_admin" e2e-28-task "E2E Task Requester") || exit 1
out=$(perm_put "$token_admin" "$role_task" 1 "[{\"resource\":\"task.plan\",\"action\":\"read\",${P_ALL}},{\"resource\":\"task.plan\",\"action\":\"write\",${P_ALL}},{\"resource\":\"task.plan\",\"action\":\"request_publication\",${P_ALL}},{\"resource\":\"task.plan\",\"action\":\"approve_publication\",${P_ALL}},{\"resource\":\"task.progress\",\"action\":\"read\",${P_ALL}},{\"resource\":\"reporting.metric\",\"action\":\"read\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"read\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"write\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"publish\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"query\",${P_ALL}}]")
[ "$(admin_status "$out")" == "200" ] || { print_error "task 角色权限写入应 200: $(admin_body "$out")"; exit $FAILED; }

role_apr=$(admin_role_create "$token_admin" e2e-28-approver "E2E Plan Approver") || exit 1
out=$(perm_put "$token_admin" "$role_apr" 1 "[{\"resource\":\"task.plan\",\"action\":\"read\",${P_ALL}},{\"resource\":\"task.plan\",\"action\":\"approve_publication\",${P_ALL}},{\"resource\":\"task.plan\",\"action\":\"publish\",${P_ALL}},{\"resource\":\"reporting.metric\",\"action\":\"read\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"read\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"query\",${P_ALL}}]")
[ "$(admin_status "$out")" == "200" ] || { print_error "approver 角色权限写入应 200: $(admin_body "$out")"; exit $FAILED; }

role_np=$(admin_role_create "$token_admin" e2e-28-noperm "E2E No Perm") || exit 1

role_self=$(admin_role_create "$token_admin" e2e-28-self "E2E Self Scope") || exit 1
out=$(perm_put "$token_admin" "$role_self" 1 "[{\"resource\":\"reporting.dashboard\",\"action\":\"query\",\"dataScope\":\"DATA_SCOPE_SELF\",\"approvalMode\":\"APPROVAL_MODE_NONE\"}]")
[ "$(admin_status "$out")" == "200" ] || { print_error "self 角色权限写入应 200: $(admin_body "$out")"; exit $FAILED; }

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
a_line=$(open_and_login e2e-m14e-a "$role_task"); a_pid=$(echo "$a_line" | cut -d' ' -f1); token_a=$(echo "$a_line" | cut -d' ' -f2)
b_line=$(open_and_login e2e-m14e-b "$role_apr"); b_pid=$(echo "$b_line" | cut -d' ' -f1); token_b=$(echo "$b_line" | cut -d' ' -f2)
c_line=$(open_and_login e2e-m14e-c "$role_np"); c_pid=$(echo "$c_line" | cut -d' ' -f1); token_c=$(echo "$c_line" | cut -d' ' -f2)
d_line=$(open_and_login e2e-m14e-d "$role_self"); d_pid=$(echo "$d_line" | cut -d' ' -f1); token_d=$(echo "$d_line" | cut -d' ' -f2)

# ---------- 5. 403 负例 ----------
print_step "5. 403 负例"
out=$(admin_request GET /task-plans '' "$token_c")
[ "$(admin_status "$out")" == "403" ] || { print_error "noperm 读任务计划应 403，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "CONSOLE_FORBIDDEN" ] || { print_error "noperm reason 应 CONSOLE_FORBIDDEN，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request GET /reporting/metrics '' "$token_c")
[ "$(admin_status "$out")" == "403" ] || { print_error "noperm 读指标应 403，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /task-plans/3/publish '{"expectedConfigHash":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}' "$token_a")
[ "$(admin_status "$out")" == "403" ] || { print_error "A 无 publish 权限应 403，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
print_step "403 负例通过"

# ---------- 6. 任务计划生命周期 ----------
print_step "6. 任务计划：草稿 v2 → 过期 409 → v3 → Get/List → 提请 → 审批 → 发布 → v4 NOT_READY"
# v2 草稿（单任务签到，内容异于 v1 避免 config_hash 唯一键冲突）
V2_TASKS='[{"taskId":"2","version":"1","name":"每日签到","period":"TASK_PERIOD_DAILY","metric":"TASK_METRIC_SIGN_IN","target":"1","rewardTemplateId":"T-SIGN","rewardCount":"1"}]'
out=$(admin_request POST /task-plans/drafts "{\"expectedLatestVersion\":\"1\",\"tasks\":${V2_TASKS}}" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "草稿 v2 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(first_field "$(admin_body "$out")" version)" == "2" ] || { print_error "草稿 v2 version 应 2，实际: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" status)" == "TASK_PLAN_STATUS_DRAFT" ] || { print_error "草稿 v2 status 应 DRAFT"; exit $FAILED; }
hash_v2=$(extract_field "$(admin_body "$out")" configHash)
[ -n "$hash_v2" ] || { print_error "草稿 v2 应回 configHash"; exit $FAILED; }
out=$(admin_request POST /task-plans/drafts "{\"expectedLatestVersion\":\"1\",\"tasks\":${V2_TASKS}}" "$token_a")
[ "$(admin_status "$out")" == "409" ] || { print_error "过期 latest 建草稿应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "TASK_PLAN_CONFLICT" ] || { print_error "过期 latest reason 应 TASK_PLAN_CONFLICT"; exit $FAILED; }

# v3 草稿（两任务：登录 target=2 + 签到，内容异于 v1/v2）
V3_TASKS='[{"taskId":"1","version":"1","name":"每日登录","period":"TASK_PERIOD_DAILY","metric":"TASK_METRIC_LOGIN","target":"2","rewardTemplateId":"T-BOOT","rewardCount":"1"},{"taskId":"2","version":"1","name":"每日签到","period":"TASK_PERIOD_DAILY","metric":"TASK_METRIC_SIGN_IN","target":"1","rewardTemplateId":"T-SIGN","rewardCount":"1"}]'
out=$(admin_request POST /task-plans/drafts "{\"expectedLatestVersion\":\"2\",\"tasks\":${V3_TASKS}}" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "草稿 v3 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(first_field "$(admin_body "$out")" version)" == "3" ] || { print_error "草稿 v3 version 应 3，实际: $(admin_body "$out")"; exit $FAILED; }
hash_v3=$(extract_field "$(admin_body "$out")" configHash)
[ -n "$hash_v3" ] || { print_error "草稿 v3 应回 configHash"; exit $FAILED; }

out=$(admin_request GET /task-plans/2 '' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "Get v2 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" status)" == "TASK_PLAN_STATUS_DRAFT" ] || { print_error "Get v2 status 应 DRAFT"; exit $FAILED; }
out=$(admin_request GET /task-plans/0 '' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "Get v0（最新）应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(first_field "$(admin_body "$out")" version)" == "3" ] || { print_error "Get v0 应回最新 v3，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request GET /task-plans/999 '' "$token_a")
[ "$(admin_status "$out")" == "404" ] || { print_error "Get 不存在版本应 404，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "TASK_PLAN_NOT_FOUND" ] || { print_error "404 reason 应 TASK_PLAN_NOT_FOUND"; exit $FAILED; }
out=$(admin_request GET '/task-plans?afterVersion=2' '' "$token_a")
echo "$(admin_body "$out")" | grep -q '"version":"3"' || { print_error "列表 afterVersion=2 应含 v3，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request GET /task-plans/2/approval '' "$token_a")
[ "$(admin_status "$out")" == "404" ] || { print_error "v2 无审批行应 404（正常空态），实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }

# 提请发布：过期版本 409 → 伪造 requesterId 被会话覆盖 → 错哈希 409 → 重放 200
out=$(admin_request POST /task-plans/2/publication-request "{\"expectedConfigHash\":\"${hash_v2}\"}" "$token_a")
[ "$(admin_status "$out")" == "409" ] || { print_error "提请非最新版本应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
REQ3="{\"expectedConfigHash\":\"${hash_v3}\",\"requesterId\":\"424242\"}"
out=$(admin_request POST /task-plans/3/publication-request "$REQ3" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "提请 v3 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" requesterId)" == "${a_pid}" ] || { print_error "提请申请人应为会话主体 A(${a_pid})，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /task-plans/3/publication-request "{\"expectedConfigHash\":\"${ZERO_HASH}\"}" "$token_a")
[ "$(admin_status "$out")" == "409" ] || { print_error "错哈希提请应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "TASK_PLAN_CONFLICT" ] || { print_error "错哈希 reason 应 TASK_PLAN_CONFLICT"; exit $FAILED; }
out=$(admin_request POST /task-plans/3/publication-request "$REQ3" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "同提请重放应 200: $(admin_body "$out")"; exit $FAILED; }

out=$(admin_request GET /task-plans/3/approval '' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "读取审批应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" requesterId)" == "${a_pid}" ] || { print_error "审批行申请人应 A(${a_pid})"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"approverId"' && { print_error "待审批 approverId 应省略（0）"; exit $FAILED; }
out=$(admin_request GET /task-plans/0/approval '' "$token_a")
[ "$(admin_status "$out")" == "400" ] || { print_error "version=0 读审批应 400，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }

# B 批准（伪造 approverId 被会话覆盖）→ A 自批 409
APR3="{\"expectedConfigHash\":\"${hash_v3}\",\"approverId\":\"99999\"}"
out=$(admin_request POST /task-plans/3/approval "$APR3" "$token_b")
[ "$(admin_status "$out")" == "200" ] || { print_error "B 批准应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" approved)" == "true" ] || { print_error "批准后 approved 应 true，实际: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" approverId)" == "${b_pid}" ] || { print_error "审批人应为会话主体 B(${b_pid})，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /task-plans/3/approval "{\"expectedConfigHash\":\"${hash_v3}\"}" "$token_a")
[ "$(admin_status "$out")" == "409" ] || { print_error "A 自批应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }

# B 发布 v3：v1 RETIRED → 错哈希 409 → 重放 200
out=$(admin_request POST /task-plans/3/publish "{\"expectedConfigHash\":\"${hash_v3}\"}" "$token_b")
[ "$(admin_status "$out")" == "200" ] || { print_error "发布 v3 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" status)" == "TASK_PLAN_STATUS_PUBLISHED" ] || { print_error "发布后 status 应 PUBLISHED，实际: $(admin_body "$out")"; exit $FAILED; }
[ "$(do_mysql "SELECT status FROM gacha_ops.task_plan_version WHERE version=1;")" == "3" ] || print_error "v1 应 RETIRED(3)"
[ "$(do_mysql "SELECT current_version FROM gacha_ops.task_plan_head WHERE singleton_id=1;")" == "3" ] || print_error "head.current_version 应 3"
out=$(admin_request POST /task-plans/3/publish "{\"expectedConfigHash\":\"${ZERO_HASH}\"}" "$token_b")
[ "$(admin_status "$out")" == "409" ] || { print_error "错哈希发布应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /task-plans/3/publish "{\"expectedConfigHash\":\"${hash_v3}\"}" "$token_b")
[ "$(admin_status "$out")" == "200" ] || { print_error "发布重放应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(do_mysql "SELECT approver_id FROM gacha_ops.task_plan_publication_approval WHERE plan_version=3;")" == "${b_pid}" ] || print_error "审批行 approver 应 B(${b_pid})"

# v4 结构化券奖励：提请 → 批准 → 发布 400 NOT_READY（DRAFT 保留，head 不变）
V4_TASKS='[{"taskId":"4","version":"1","name":"扭蛋券任务","period":"TASK_PERIOD_DAILY","metric":"TASK_METRIC_LOGIN","target":"1","reward":{"count":"1","coupon":{"templateId":"1","templateVersion":"1"}}}]'
out=$(admin_request POST /task-plans/drafts "{\"expectedLatestVersion\":\"3\",\"tasks\":${V4_TASKS}}" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "草稿 v4 应 200: $(admin_body "$out")"; exit $FAILED; }
hash_v4=$(extract_field "$(admin_body "$out")" configHash)
out=$(admin_request POST /task-plans/4/publication-request "{\"expectedConfigHash\":\"${hash_v4}\"}" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "提请 v4 应 200: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /task-plans/4/approval "{\"expectedConfigHash\":\"${hash_v4}\"}" "$token_b")
[ "$(admin_status "$out")" == "200" ] || { print_error "批准 v4 应 200: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /task-plans/4/publish "{\"expectedConfigHash\":\"${hash_v4}\"}" "$token_b")
[ "$(admin_status "$out")" == "400" ] || { print_error "结构化奖励发布应 400 NOT_READY，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "TASK_PLAN_NOT_READY" ] || { print_error "reason 应 TASK_PLAN_NOT_READY，实际: $(admin_body "$out")"; exit $FAILED; }
[ "$(do_mysql "SELECT status FROM gacha_ops.task_plan_version WHERE version=4;")" == "1" ] || print_error "v4 应保留 DRAFT(1)"
[ "$(do_mysql "SELECT current_version FROM gacha_ops.task_plan_head WHERE singleton_id=1;")" == "3" ] || print_error "NOT_READY 后 head.current_version 应仍为 3"
print_step "任务计划断言通过：CAS 409×2/会话主体绑定/重放/发布 CAS/审批行/结构化拒绝"

# ---------- 7. ListTasks 空进度视图 ----------
print_step "7. ListTasks：userId=0 400 → 空进度 → 游标两页 → period 过滤空"
out=$(admin_request GET '/tasks?userId=0' '' "$token_a")
[ "$(admin_status "$out")" == "400" ] || { print_error "userId=0 应 400，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "TASK_PLAN_INVALID" ] || { print_error "reason 应 TASK_PLAN_INVALID"; exit $FAILED; }
out=$(admin_request GET "/tasks?userId=${TASK_UID}" '' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "ListTasks 应 200: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"taskId":"1"' || { print_error "任务视图应含 taskId 1，实际: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"taskId":"2"' || { print_error "任务视图应含 taskId 2（v3 已发布两任务）"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"state":"TASK_PROGRESS_STATE_IN_PROGRESS"' || { print_error "空进度管线 state 应恒 IN_PROGRESS"; exit $FAILED; }
out=$(admin_request GET "/tasks?userId=${TASK_UID}&pageSize=1" '' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "pageSize=1 应 200: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"taskId":"1"' || { print_error "第 1 页应含 taskId 1"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"taskId":"2"' && { print_error "第 1 页不应含 taskId 2"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" nextPageToken)" == "1" ] || { print_error "第 1 页 nextPageToken 应 1，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request GET "/tasks?userId=${TASK_UID}&pageSize=1&pageToken=1" '' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "第 2 页应 200: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"taskId":"2"' || { print_error "第 2 页应含 taskId 2，实际: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"nextPageToken"' && { print_error "末页不应回 nextPageToken"; exit $FAILED; }
out=$(admin_request GET "/tasks?userId=${TASK_UID}&period=TASK_PERIOD_WEEKLY" '' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "period 过滤应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(admin_body "$out")" == "{}" ] || { print_error "WEEKLY 过滤应空任务（protojson 空数组省略 → {}），实际: $(admin_body "$out")"; exit $FAILED; }
print_step "ListTasks 断言通过：空进度/IN_PROGRESS/游标两页/过滤空"

# ---------- 8. 报表：指标 + 看板 CRUD/CAS/发布/查询 ----------
print_step "8. 报表：指标目录 5 项 → 看板 v1 → CAS v2 → 重叠 400 → 发布 → 查询/缓存"
out=$(admin_request GET /reporting/metrics '' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "指标目录应 200: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"metricCode":"provider.impressions"' || { print_error "指标目录应含 provider.impressions"; exit $FAILED; }
nm=$(echo "$(admin_body "$out")" | grep -o '"metricCode"' | wc -l | tr -d ' ')
[ "$nm" == "5" ] || { print_error "指标目录应恰 5 项，实际: ${nm}"; exit $FAILED; }

W1='{"widgetCode":"w1","title":"展示量","kind":1,"metricCode":"provider.impressions","grain":"TIME_GRAIN_DAY","dimensionCode":"all","gridX":"0","gridY":"0","gridWidth":"12","gridHeight":"12","displayOptionsJson":"{}"}'
W2='{"widgetCode":"w2","title":"点击量","kind":2,"metricCode":"provider.clicks","grain":"TIME_GRAIN_DAY","dimensionCode":"all","gridX":"12","gridY":"0","gridWidth":"12","gridHeight":"12","displayOptionsJson":"{}"}'
out=$(admin_request POST /reporting/dashboards "{\"dashboardCode\":\"e2e-db-28\",\"title\":\"E2E报表28\",\"widgets\":[${W1},${W2}]}" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "看板创建应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "1" ] || { print_error "看板 v1 version 应 1，实际: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" status)" == "DASHBOARD_STATUS_DRAFT" ] || { print_error "看板 v1 status 应 DRAFT"; exit $FAILED; }
db_id=$(extract_field "$(admin_body "$out")" dashboardId)
[ -n "$db_id" ] || { print_error "看板应回 dashboardId"; exit $FAILED; }

out=$(admin_request POST /reporting/dashboards "{\"dashboardId\":\"${db_id}\",\"expectedVersion\":\"1\",\"dashboardCode\":\"e2e-db-28\",\"title\":\"E2E报表28v2\",\"widgets\":[${W1},${W2}]}" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "看板 CAS v2 应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "2" ] || { print_error "CAS 后 version 应 2，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST /reporting/dashboards "{\"dashboardId\":\"${db_id}\",\"expectedVersion\":\"1\",\"dashboardCode\":\"e2e-db-28\",\"title\":\"E2E报表28stale\",\"widgets\":[${W1},${W2}]}" "$token_a")
[ "$(admin_status "$out")" == "409" ] || { print_error "过期 CAS 应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "DASHBOARD_CONFLICT" ] || { print_error "过期 CAS reason 应 DASHBOARD_CONFLICT"; exit $FAILED; }
W3='{"widgetCode":"w3","title":"重叠","kind":1,"metricCode":"provider.impressions","grain":"TIME_GRAIN_DAY","dimensionCode":"all","gridX":"6","gridY":"0","gridWidth":"12","gridHeight":"12","displayOptionsJson":"{}"}'
out=$(admin_request POST /reporting/dashboards "{\"dashboardId\":\"${db_id}\",\"expectedVersion\":\"2\",\"dashboardCode\":\"e2e-db-28\",\"title\":\"E2E报表28ov\",\"widgets\":[${W1},${W3}]}" "$token_a")
[ "$(admin_status "$out")" == "400" ] || { print_error "widget 重叠应 400，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "DASHBOARD_INVALID" ] || { print_error "重叠 reason 应 DASHBOARD_INVALID"; exit $FAILED; }

QRY='{"providerCode":"e2e","externalAccountId":"acc-28","from":"2026-10-01T00:00:00Z","to":"2026-10-02T00:00:00Z","maxDataPoints":"100"}'
out=$(admin_request POST "/reporting/dashboards/${db_id}/query" "$QRY" "$token_a")
[ "$(admin_status "$out")" == "400" ] || { print_error "未发布查询应 400，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "DASHBOARD_NOT_PUBLISHED" ] || { print_error "未发布 reason 应 DASHBOARD_NOT_PUBLISHED"; exit $FAILED; }
out=$(admin_request POST "/reporting/dashboards/${db_id}/publish" '{"expectedVersion":"1"}' "$token_a")
[ "$(admin_status "$out")" == "409" ] || { print_error "发布 CAS 不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST "/reporting/dashboards/${db_id}/publish" '{"expectedVersion":"2"}' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "发布应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" status)" == "DASHBOARD_STATUS_PUBLISHED" ] || { print_error "发布后 status 应 PUBLISHED，实际: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" version)" == "3" ] || { print_error "发布后 version 应 3（+1）"; exit $FAILED; }
out=$(admin_request GET "/reporting/dashboards/${db_id}" '' "$token_a")
[ "$(extract_field "$(admin_body "$out")" status)" == "DASHBOARD_STATUS_PUBLISHED" ] || { print_error "Get 应回 PUBLISHED"; exit $FAILED; }
out=$(admin_request GET /reporting/dashboards/999999999 '' "$token_a")
[ "$(admin_status "$out")" == "404" ] || { print_error "Get 不存在看板应 404，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "DASHBOARD_NOT_FOUND" ] || { print_error "404 reason 应 DASHBOARD_NOT_FOUND"; exit $FAILED; }

out=$(admin_request POST "/reporting/dashboards/${db_id}/query" "$QRY" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "查询应 200: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"widgetCode":"w1"' || { print_error "查询结果应含 w1，实际: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"widgetCode":"w2"' || { print_error "查询结果应含 w2"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"dashboardCode":"e2e-db-28"' || { print_error "查询响应应含 dashboard"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"cacheHit"' && { print_error "首查不应回 cacheHit"; exit $FAILED; }
out=$(admin_request POST "/reporting/dashboards/${db_id}/query" "$QRY" "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "重放查询应 200: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"cacheHit":true' || { print_error "重放应 cacheHit=true，实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request POST "/reporting/dashboards/${db_id}/query" "$QRY" "$token_d")
[ "$(admin_status "$out")" == "403" ] || { print_error "SELF 数据范围查询应 403，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "DATA_SCOPE_DENIED" ] || { print_error "reason 应 DATA_SCOPE_DENIED，实际: $(admin_body "$out")"; exit $FAILED; }
print_step "报表断言通过：5 指标/CAS 409×2/重叠 400/发布 CAS/查询两结果/cacheHit/SELF 403"

# ---------- 9. 用户盈亏曲线（空态 stub） ----------
print_step "9. 盈亏曲线：空态 {} → 非法粒度 400 → 时间窗 400 → tenant 不符 403"
out=$(admin_request GET "/analytics/users/${TASK_UID}/profit-curve?grain=TIME_GRAIN_DAY&from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z" '' "$token_a")
[ "$(admin_status "$out")" == "200" ] || { print_error "盈亏曲线应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(admin_body "$out")" == "{}" ] || { print_error "空态应回字面 {}（dataReady=false 省略），实际: $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request GET "/analytics/users/${TASK_UID}/profit-curve?grain=WEEK&from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z" '' "$token_a")
[ "$(admin_status "$out")" == "400" ] || { print_error "非法粒度应 400，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
out=$(admin_request GET "/analytics/users/${TASK_UID}/profit-curve?grain=TIME_GRAIN_DAY&from=2026-10-02T00:00:00Z&to=2026-10-01T00:00:00Z" '' "$token_a")
[ "$(admin_status "$out")" == "400" ] || { print_error "时间窗倒置应 400，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "DASHBOARD_INVALID" ] || { print_error "时间窗 reason 应 DASHBOARD_INVALID"; exit $FAILED; }
out=$(admin_request GET "/analytics/users/${TASK_UID}/profit-curve?tenantId=999&grain=TIME_GRAIN_DAY&from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z" '' "$token_a")
[ "$(admin_status "$out")" == "403" ] || { print_error "tenantId 不符应 403，实际: $(admin_status "$out") $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" reason)" == "TENANT_MISMATCH" ] || { print_error "reason 应 TENANT_MISMATCH，实际: $(admin_body "$out")"; exit $FAILED; }
print_step "盈亏曲线断言通过：空态/400×2/403"

# ---------- 10. 审计抽样（ops_audit_logs） ----------
print_step "10. 审计抽样（ops_audit_logs 按 operation_id）"
audit_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_ops.ops_audit_logs WHERE request_id LIKE 'e2e-rid-%' AND (operation_id LIKE 'console.task.%' OR operation_id LIKE 'console.reporting.%');")
[ "$audit_rows" -ge 18 ] || print_error "M14e 审计行数应 ≥18，实际: ${audit_rows}"
for op in console.task.list console.task.createplandraft console.task.listplans console.task.getplan \
          console.task.requestplanpublication console.task.approveplanpublication \
          console.task.getplanpublicationapproval console.task.publishplan \
          console.reporting.queryuserprofit console.reporting.metrics console.reporting.save \
          console.reporting.get console.reporting.publish console.reporting.query; do
    [ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='${op}' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 ${op} 行"
done
print_step "审计抽样通过：14 类 M14e 操作落库"

# ---------- 11. 现场清理 ----------
print_step "11. 现场清理（e2e 账号/角色/task 三表回 bootstrap/报表 e2e 行）"
for u in e2e-m14e-a e2e-m14e-b e2e-m14e-c e2e-m14e-d; do
    oid=$(do_mysql "SELECT id FROM gacha_ops.console_accounts WHERE username='${u}';")
    if [ -n "$oid" ]; then
        do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${oid}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${oid}; DELETE FROM gacha_ops.console_accounts WHERE id=${oid};"
    fi
done
for r in e2e-28-task e2e-28-approver e2e-28-noperm e2e-28-self; do
    rid=$(do_mysql "SELECT id FROM gacha_ops.roles WHERE code='${r}';")
    if [ -n "$rid" ]; then
        do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${rid}; DELETE FROM gacha_ops.roles WHERE id=${rid};"
    fi
done
do_mysql "DELETE FROM gacha_ops.task_plan_publication_approval;"
do_mysql "DELETE FROM gacha_ops.task_plan_version;"
do_mysql "INSERT INTO gacha_ops.task_plan_version (version, status, config_hash, config_blob, created_at, published_at)
          VALUES (1, 2, UNHEX('c4ea9a722bcccc403081206cf2e0ad6c9525f8452f2891da3d6bbecaa4d7fe06'),
                  UNHEX('1a22080110011a0ce6af8fe697a5e799bbe5bd952002280330014206542d424f4f544801'), NOW(3), NOW(3));"
do_mysql "UPDATE gacha_ops.task_plan_head SET current_version=1, latest_version=1, updated_at=NOW(3) WHERE singleton_id=1;"
do_mysql "DELETE FROM gacha_ops.reporting_dashboard_widget WHERE dashboard_id IN (SELECT id FROM gacha_ops.reporting_dashboard WHERE dashboard_code LIKE 'e2e-%');"
do_mysql "DELETE FROM gacha_ops.reporting_dashboard WHERE dashboard_code LIKE 'e2e-%';"

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}√ 场景28 M14e 任务计划 + 报表聚合 通过${NC}"
    exit 0
else
    echo -e "${RED}× 场景28 失败${NC}"
    exit 1
fi
