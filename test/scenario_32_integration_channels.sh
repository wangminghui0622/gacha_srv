#!/bin/bash
# ============================================================
# 场景32：M15d integration 渠道收尾（thirdparty-rpc 9317 / ops-rpc 报表管线）
#
# 覆盖：SQL 复位（gacha_thirdparty 5 表 + gacha_ops 报表 3 表整表清空——仅 M15d 使用，
#       整表清空最稳）+ 微信/支付宝 mock 启动
#       → 微信全家（token 两次同值+本地缓存（token 计数增量≤1）、
#         phone mock 首次 40001 重试后成功（phone 计数增量=2 且重拉 token）、
#         qr PNG 魔数、urllink 字符串、err/errqr/errlink 负例 6006）
#       → 支付宝（authorization_code 字段断言 union_id=open_id、refresh_token、
#         空 code 9008、err 9008、非法 grant 9008、userinfo 空 stub）
#       → 腾讯广告（同 openid 两次→1 行、异 openid→2 行、空 openid 9010）
#       → 指标（3 样本批 accepted=3 → batch/raw/current/outbox 断言 → 轮询 dispatcher
#         → ops inbox/snapshot/bucket 断言 → 场景内建已发布看板 query 数据点非空
#         → 高修订 accepted 桶=差量、低修订 stale 桶不变 → 同 request_id 重放同批号
#         → 同幂等键异内容 9012 → 非法样本 9011 ×4 → 复位）
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景32：M15d 微信小程序全家 + 支付宝 + 腾讯广告 + provider 指标"

TC=/tmp/thirdpartycli_m15d.exe
ADMIN_PASS="admin12345678"
OP_PASS="OpPass12345678"
PROVIDER=mock
ACCOUNT=acc-32
# 桶锚点 2026-10-23T08:00Z（小时对齐；看板查询窗覆盖）
BUCKET_START="2026-10-23T08:00:00Z"
QRY_FROM="2026-10-23T00:00:00Z"
QRY_TO="2026-10-24T00:00:00Z"

# ---------- 1. 复位 ----------
print_step "1. 复位（integration 5 表 + reporting 3 表整表清空 / mock 启动 / admin e2e-32 残留清理）"
for tbl in integration_metric_batch integration_metric_raw integration_metric_current integration_metric_outbox integration_tx_ad_click; do
    do_mysql "DELETE FROM gacha_thirdparty.${tbl};"
done
for tbl in reporting_metric_inbox reporting_metric_snapshot reporting_metric_bucket; do
    do_mysql "DELETE FROM gacha_ops.${tbl};"
done
do_mysql "UPDATE gacha_ops.console_accounts SET totp_secret='' WHERE username='admin';"
oid=$(do_mysql "SELECT id FROM gacha_ops.console_accounts WHERE username='e2e-m15d';")
if [ -n "$oid" ]; then
    do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${oid}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${oid}; DELETE FROM gacha_ops.console_accounts WHERE id=${oid};"
fi
rid=$(do_mysql "SELECT id FROM gacha_ops.roles WHERE code='e2e-32-rpt';")
if [ -n "$rid" ]; then
    do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${rid}; DELETE FROM gacha_ops.roles WHERE id=${rid};"
fi
do_mysql "DELETE FROM gacha_ops.reporting_dashboard_widget WHERE dashboard_id IN (SELECT id FROM gacha_ops.reporting_dashboard WHERE dashboard_code='e2e-db-32');"
do_mysql "DELETE FROM gacha_ops.reporting_dashboard WHERE dashboard_code='e2e-db-32';"

# 工具编译（多次调用编译成 exe，比 go run 逐次编译快）
go build -o "$TC" "${SCRIPT_DIR}/tools/thirdpartycli/main.go" || { print_error "构建 thirdpartycli 失败"; exit $FAILED; }
go build -o /tmp/wechatmock_m15d.exe "${SCRIPT_DIR}/tools/wechatmock/main.go" || { print_error "构建 wechatmock 失败"; exit $FAILED; }
go build -o /tmp/alipaymock_m15d.exe "${SCRIPT_DIR}/tools/alipaymock/main.go" || { print_error "构建 alipaymock 失败"; exit $FAILED; }

# mock 重启（端口占用先杀；alipaymock 从工具目录起以加载 dev_public.pem 验签）
for port in 9410 9411; do
    mock_pid=$(netstat -ano | grep "127.0.0.1:${port}" | grep LISTENING | awk '{print $5}' | head -1)
    [ -n "$mock_pid" ] && taskkill //F //PID "$mock_pid" 2>/dev/null
done
sleep 1
/tmp/wechatmock_m15d.exe > /tmp/wechatmock_m15d.log 2>&1 &
(cd "${SCRIPT_DIR}/tools/alipaymock" && /tmp/alipaymock_m15d.exe > /tmp/alipaymock_m15d.log 2>&1 &)
for i in $(seq 1 20); do
    netstat -ano | grep -q "127.0.0.1:9410.*LISTENING" && break
    sleep 0.5
done
for i in $(seq 1 20); do
    netstat -ano | grep -q "127.0.0.1:9411.*LISTENING" && break
    sleep 0.5
done
curl -s http://127.0.0.1:9410/__reset >/dev/null

# ---------- 2. 微信全家 ----------
print_step "2. 微信全家：token 缓存 / phone 40001 重试 / qr 魔数 / urllink"
# token 两次同值；两次调用 token 计数增量≤1（本地缓存命中；跨 run 预热时增量=0）
tok_before=$(curl -s http://127.0.0.1:9410/__stats | grep -o '"token":[0-9]*' | cut -d: -f2)
t1=$("$TC" wechat-token 1) || exit $FAILED
t2=$("$TC" wechat-token 1) || exit $FAILED
tok_after=$(curl -s http://127.0.0.1:9410/__stats | grep -o '"token":[0-9]*' | cut -d: -f2)
echo "$t1" | grep -q "access_token=dev-mini-token" || { print_error "token 应回 dev-mini-token: ${t1}"; exit $FAILED; }
echo "$t2" | grep -q "access_token=dev-mini-token" || { print_error "二次 token 应同值: ${t2}"; exit $FAILED; }
[ $((tok_after - tok_before)) -le 1 ] || { print_error "两次 token 调用 HTTP 增量应≤1（本地缓存），实际: ${tok_before}→${tok_after}"; exit $FAILED; }

# phone：retry-1 首次 40001 → 清 token 重试成功；phone 计数增量=2，token 重拉增量=1
phone_before=$(curl -s http://127.0.0.1:9410/__stats | grep -o '"phone":[0-9]*' | cut -d: -f2)
tok_before2=$(curl -s http://127.0.0.1:9410/__stats | grep -o '"token":[0-9]*' | cut -d: -f2)
out=$("$TC" wechat-phone 1 retry-1) || exit $FAILED
tok_after2=$(curl -s http://127.0.0.1:9410/__stats | grep -o '"token":[0-9]*' | cut -d: -f2)
phone_after=$(curl -s http://127.0.0.1:9410/__stats | grep -o '"phone":[0-9]*' | cut -d: -f2)
echo "$out" | grep -q "phone_number=13800001111 country_code=86" || { print_error "phone 应回 13800001111/86，实际: ${out}"; exit $FAILED; }
[ $((phone_after - phone_before)) -eq 2 ] || { print_error "40001 重试 phone 调用应恰 2 次，实际: ${phone_before}→${phone_after}"; exit $FAILED; }
[ $((tok_after2 - tok_before2)) -eq 1 ] || { print_error "40001 后应重拉 token 恰 1 次，实际: ${tok_before2}→${tok_after2}"; exit $FAILED; }

# 负例：phone err / qr errqr / urllink errlink → 6006
out=$("$TC" wechat-phone 1 err)
echo "$out" | grep -q "code=6006" || { print_error "phone errcode 应 6006，实际: ${out}"; exit $FAILED; }
out=$("$TC" wechat-qr 1 s32 "" 0)
echo "$out" | grep -q "qr_magic=89504e470d0a1a0a" || { print_error "qr 应回 PNG 魔数，实际: ${out}"; exit $FAILED; }
[ "$(echo "$out" | sed 's/.*qr_bytes=\([0-9]*\).*/\1/')" -ge 8 ] || { print_error "qr 字节数应≥8，实际: ${out}"; exit $FAILED; }
out=$("$TC" wechat-qr 1 errqr "" 0)
echo "$out" | grep -q "code=6006" || { print_error "qr errqr 应 6006，实际: ${out}"; exit $FAILED; }
out=$("$TC" wechat-urllink 1 pages/index/index "scene=1")
echo "$out" | grep -q "url_link=https://wxaurl.cn/dev-link" || { print_error "urllink 应回 dev-link，实际: ${out}"; exit $FAILED; }
out=$("$TC" wechat-urllink 1 errlink "")
echo "$out" | grep -q "code=6006" || { print_error "urllink errlink 应 6006，实际: ${out}"; exit $FAILED; }
print_step "微信全家断言通过"

# ---------- 3. 支付宝 ----------
print_step "3. 支付宝：authorization_code/refresh_token 字段断言 → 负例 9008 → userinfo 空 stub"
out=$("$TC" alipay-token auth e2e-code-1)
echo "$out" | grep -q "user_id=dev-alipay-user-e2e-code-1" || { print_error "user_id 应含指纹: ${out}"; exit $FAILED; }
echo "$out" | grep -q "access_token=dev-alipay-at-auth-e2e-code-1" || { print_error "access_token 错误: ${out}"; exit $FAILED; }
echo "$out" | grep -q "refresh_token=dev-alipay-rt-auth-e2e-code-1" || { print_error "refresh_token 错误: ${out}"; exit $FAILED; }
open_id=$(echo "$out" | sed 's/.*open_id=\([^ ]*\).*/\1/')
union_id=$(echo "$out" | sed 's/.*union_id=\([^ ]*\).*/\1/')
[ "$open_id" == "dev-alipay-open-auth-e2e-code-1" ] || { print_error "open_id 应含指纹: ${out}"; exit $FAILED; }
[ "$union_id" == "$open_id" ] || { print_error "union_id 应=open_id: ${out}"; exit $FAILED; }
out=$("$TC" alipay-token refresh e2e-rt-1)
echo "$out" | grep -q "access_token=dev-alipay-at-refresh-e2e-rt-1" || { print_error "refresh 换 token 错误: ${out}"; exit $FAILED; }
out=$("$TC" alipay-token auth "")
echo "$out" | grep -q "code=9008" || { print_error "空 code 应 9008，实际: ${out}"; exit $FAILED; }
out=$("$TC" alipay-token auth err)
echo "$out" | grep -q "code=9008" || { print_error "err code 应 9008，实际: ${out}"; exit $FAILED; }
out=$("$TC" alipay-token bogus x)
echo "$out" | grep -q "code=9008" || { print_error "非法 grant 应 9008，实际: ${out}"; exit $FAILED; }
out=$("$TC" alipay-userinfo dev-at-1)
echo "$out" | grep -q "open_id= avatar= nick_name=" || { print_error "userinfo 应空 stub，实际: ${out}"; exit $FAILED; }
print_step "支付宝断言通过"

# ---------- 4. 腾讯广告 ----------
print_step "4. 腾讯广告：同 openid 去重 / 异 openid 落行 / 空 openid 9010"
out=$("$TC" tx-ad-click 1 e2e-openid-a click-1)
echo "$out" | grep -q "^ok$" || { print_error "点击应 ok: ${out}"; exit $FAILED; }
out=$("$TC" tx-ad-click 1 e2e-openid-a click-2)
echo "$out" | grep -q "^ok$" || { print_error "同 openid 重复点击应 ok（跳过）: ${out}"; exit $FAILED; }
rows=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_tx_ad_click WHERE wechat_open_id='e2e-openid-a';")
[ "$rows" == "1" ] || { print_error "同 openid 两次应 1 行，实际: ${rows}"; exit $FAILED; }
out=$("$TC" tx-ad-click 1 e2e-openid-b click-3)
echo "$out" | grep -q "^ok$" || { print_error "异 openid 点击应 ok: ${out}"; exit $FAILED; }
rows=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_tx_ad_click;")
[ "$rows" == "2" ] || { print_error "两个 openid 应 2 行，实际: ${rows}"; exit $FAILED; }
out=$("$TC" tx-ad-click 1 "")
echo "$out" | grep -q "code=9010" || { print_error "空 openid 应 9010，实际: ${out}"; exit $FAILED; }
rows=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_tx_ad_click;")
[ "$rows" == "2" ] || { print_error "空 openid 不应落行，实际: ${rows}"; exit $FAILED; }
print_step "腾讯广告断言通过"

# ---------- 5. 指标管线 ----------
print_step "5. 指标：3 样本批 accepted=3 → 落库断言 → dispatcher 轮询 → ops 三表"
cat > /tmp/ingest_3s.json <<'EOF'
{"tenantId":"1","requestId":"e2e-rq-3s","idempotencyKey":"e2e-idem-3s","providerCode":"mock","externalAccountId":"acc-32","samples":[
 {"providerMetricCode":"impressions","standardMetric":"STANDARD_METRIC_IMPRESSIONS","externalObjectId":"obj-3s-1","bucketStart":"2026-10-23T08:00:00Z","bucketSeconds":"3600","dimensionKey":"all","value":"50","sourceRevision":"5"},
 {"providerMetricCode":"clicks","standardMetric":"STANDARD_METRIC_CLICKS","externalObjectId":"obj-3s-2","bucketStart":"2026-10-23T08:00:00Z","bucketSeconds":"3600","dimensionKey":"all","value":"20","sourceRevision":"5"},
 {"providerMetricCode":"spend","standardMetric":"STANDARD_METRIC_SPEND","externalObjectId":"obj-3s-3","bucketStart":"2026-10-23T08:00:00Z","bucketSeconds":"3600","dimensionKey":"all","value":"500","currency":"CNY","sourceRevision":"5"}
]}
EOF
out=$("$TC" ingest-json /tmp/ingest_3s.json)
echo "$out" | grep -q "accepted=3 stale=0 replayed=false" || { print_error "3 样本批应 accepted=3，实际: ${out}"; exit $FAILED; }
batch_3s=$(echo "$out" | sed 's/batch_id=\([0-9]*\).*/\1/')
[ -n "$batch_3s" ] && [ "$batch_3s" -gt 0 ] || { print_error "应回 batch_id>0，实际: ${out}"; exit $FAILED; }
batch_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_metric_batch WHERE request_id='e2e-rq-3s' AND accepted_count=3 AND stale_count=0;")
[ "$batch_rows" == "1" ] || { print_error "batch 行应 1（accepted=3/stale=0），实际: ${batch_rows}"; exit $FAILED; }
raw_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_metric_raw WHERE batch_id=${batch_3s};")
[ "$raw_rows" == "3" ] || { print_error "raw 应 3 行，实际: ${raw_rows}"; exit $FAILED; }
cur_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_metric_current;")
[ "$cur_rows" == "3" ] || { print_error "current 应 3 行，实际: ${cur_rows}"; exit $FAILED; }
ob_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_metric_outbox;")
[ "$ob_rows" == "3" ] || { print_error "outbox 应 3 行，实际: ${ob_rows}"; exit $FAILED; }

# 轮询 dispatcher：outbox 全部 status=3（500ms 间隔 dispatcher + 15s 超时）
wait_published() { # wait_published <期望 published 行数> [超时秒]
    local want=$1 timeout=${2:-15} i
    for i in $(seq 1 $((timeout * 2))); do
        [ "$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_metric_outbox WHERE status=3;")" == "$want" ] && return 0
        sleep 0.5
    done
    print_error "outbox published 未达 ${want}: $(do_mysql "SELECT status,COUNT(*) FROM gacha_thirdparty.integration_metric_outbox GROUP BY status;")"
    return 1
}
wait_published 3 || exit $FAILED
inbox_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_ops.reporting_metric_inbox;")
[ "$inbox_rows" == "3" ] || { print_error "ops inbox 应 3 行，实际: ${inbox_rows}"; exit $FAILED; }
snap_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_ops.reporting_metric_snapshot;")
[ "$snap_rows" == "3" ] || { print_error "ops snapshot 应 3 行，实际: ${snap_rows}"; exit $FAILED; }
bucket_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_ops.reporting_metric_bucket;")
[ "$bucket_rows" == "3" ] || { print_error "ops bucket 应 3 行，实际: ${bucket_rows}"; exit $FAILED; }
imp_val=$(do_mysql "SELECT value FROM gacha_ops.reporting_metric_bucket WHERE external_account_id='acc-32' AND standard_metric=1;")
[ "$imp_val" == "50" ] || { print_error "impressions 桶应 50，实际: ${imp_val}"; exit $FAILED; }
spend_val=$(do_mysql "SELECT value FROM gacha_ops.reporting_metric_bucket WHERE external_account_id='acc-32' AND standard_metric=4 AND currency='CNY';")
[ "$spend_val" == "500" ] || { print_error "spend 桶应 500，实际: ${spend_val}"; exit $FAILED; }
print_step "指标管线断言通过（batch/raw/current/outbox → ops inbox/snapshot/bucket）"

# ---------- 6. 看板查询 ----------
print_step "6. 看板：超管建角色/账号 → 建看板发布 → query 数据点非空"
line=$(admin_login_begin admin "$ADMIN_PASS" e2e-fp-admin32) || exit 1
ticket=$(echo "$line" | cut -d' ' -f1)
secret=$(echo "$line" | cut -d' ' -f3)
token_admin=$(admin_login_complete admin "$ADMIN_PASS" e2e-fp-admin32 "$ticket" "$(admin_totp "$secret")") || exit 1
[ -n "$token_admin" ] || { print_error "超管登录失败"; exit $FAILED; }
P_ALL='"dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"'
role_rpt=$(admin_role_create "$token_admin" e2e-32-rpt "E2E Reporting") || exit 1
out=$(admin_request PUT "/authorization/roles/${role_rpt}/permissions" "{\"tenantId\":\"1\",\"expectedVersion\":\"1\",\"permissions\":[{\"resource\":\"reporting.metric\",\"action\":\"read\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"read\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"write\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"publish\",${P_ALL}},{\"resource\":\"reporting.dashboard\",\"action\":\"query\",${P_ALL}}]}" "$token_admin")
[ "$(admin_status "$out")" == "200" ] || { print_error "rpt 角色权限写入应 200: $(admin_body "$out")"; exit $FAILED; }
line=$(admin_open_account "$token_admin" e2e-m15d "$OP_PASS") || exit 1
pid=$(echo "$line" | cut -d' ' -f1)
out=$(admin_bind_roles "$token_admin" "$pid" 1 "$role_rpt")
[ "$(admin_status "$out")" == "200" ] || { print_error "绑角色应 200: $(admin_body "$out")"; exit $FAILED; }
line=$(admin_login_begin e2e-m15d "$OP_PASS" e2e-fp-m15d) || exit 1
tk=$(echo "$line" | cut -d' ' -f1)
secret2=$(echo "$line" | cut -d' ' -f3)
token_u=$(admin_login_complete e2e-m15d "$OP_PASS" e2e-fp-m15d "$tk" "$(admin_totp "$secret2")") || exit 1
[ -n "$token_u" ] || { print_error "e2e-m15d 登录失败"; exit $FAILED; }

W1='{"widgetCode":"w1","title":"展示量","kind":2,"metricCode":"provider.impressions","grain":"TIME_GRAIN_HOUR","dimensionCode":"all","gridX":"0","gridY":"0","gridWidth":"24","gridHeight":"12","displayOptionsJson":"{}"}'
out=$(admin_request POST /reporting/dashboards "{\"dashboardCode\":\"e2e-db-32\",\"title\":\"E2E指标32\",\"widgets\":[${W1}]}" "$token_u")
[ "$(admin_status "$out")" == "200" ] || { print_error "看板创建应 200: $(admin_body "$out")"; exit $FAILED; }
db_id=$(extract_field "$(admin_body "$out")" dashboardId)
[ -n "$db_id" ] || { print_error "看板应回 dashboardId"; exit $FAILED; }
out=$(admin_request POST "/reporting/dashboards/${db_id}/publish" '{"expectedVersion":"1"}' "$token_u")
[ "$(admin_status "$out")" == "200" ] || { print_error "发布应 200: $(admin_body "$out")"; exit $FAILED; }
[ "$(extract_field "$(admin_body "$out")" status)" == "DASHBOARD_STATUS_PUBLISHED" ] || { print_error "发布后应 PUBLISHED"; exit $FAILED; }
QRY="{\"providerCode\":\"${PROVIDER}\",\"externalAccountId\":\"${ACCOUNT}\",\"from\":\"${QRY_FROM}\",\"to\":\"${QRY_TO}\",\"maxDataPoints\":\"100\",\"skipCache\":true}"
out=$(admin_request POST "/reporting/dashboards/${db_id}/query" "$QRY" "$token_u")
[ "$(admin_status "$out")" == "200" ] || { print_error "看板查询应 200: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"widgetCode":"w1"' || { print_error "查询应含 w1，实际: $(admin_body "$out")"; exit $FAILED; }
echo "$(admin_body "$out")" | grep -q '"bucketStart"' || { print_error "查询数据点应非空（bucketStart），实际: $(admin_body "$out")"; exit $FAILED; }
print_step "看板查询断言通过（provider 指标桶 → 看板数据点）"

# ---------- 7. 仲裁/重放/冲突/非法 ----------
print_step "7. 指标仲裁：高修订差量 / 低修订 stale / 重放同批号 / 幂等冲突 9012 / 非法 9011×4"
out=$("$TC" ingest 1 e2e-rq-arb-a e2e-idem-arb-a "$PROVIDER" acc-arb 1 100 1)
echo "$out" | grep -q "accepted=1 stale=0" || { print_error "arb-a 应 accepted=1，实际: ${out}"; exit $FAILED; }
batch_arb_a=$(echo "$out" | sed 's/batch_id=\([0-9]*\).*/\1/')
wait_published 4 || exit $FAILED
val=$(do_mysql "SELECT value FROM gacha_ops.reporting_metric_bucket WHERE external_account_id='acc-arb' AND standard_metric=1;")
[ "$val" == "100" ] || { print_error "arb-a 桶应 100，实际: ${val}"; exit $FAILED; }
out=$("$TC" ingest 1 e2e-rq-arb-b e2e-idem-arb-b "$PROVIDER" acc-arb 1 300 3)
echo "$out" | grep -q "accepted=1 stale=0" || { print_error "arb-b 应 accepted=1，实际: ${out}"; exit $FAILED; }
wait_published 5 || exit $FAILED
val=$(do_mysql "SELECT value FROM gacha_ops.reporting_metric_bucket WHERE external_account_id='acc-arb' AND standard_metric=1;")
[ "$val" == "300" ] || { print_error "arb-b 桶应差量到 300，实际: ${val}"; exit $FAILED; }
out=$("$TC" ingest 1 e2e-rq-arb-c e2e-idem-arb-c "$PROVIDER" acc-arb 1 999 2)
echo "$out" | grep -q "accepted=0 stale=1" || { print_error "arb-c（低修订）应 stale=1，实际: ${out}"; exit $FAILED; }
sleep 2
val=$(do_mysql "SELECT value FROM gacha_ops.reporting_metric_bucket WHERE external_account_id='acc-arb' AND standard_metric=1;")
[ "$val" == "300" ] || { print_error "低修订不应改桶（仍 300），实际: ${val}"; exit $FAILED; }
stale_rows=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_metric_raw WHERE external_object_id='obj-1' AND is_stale=1;")
[ "$stale_rows" == "1" ] || { print_error "stale raw 应 1 行，实际: ${stale_rows}"; exit $FAILED; }
ob_total=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_metric_outbox;")
[ "$ob_total" == "5" ] || { print_error "stale 不应发事件（outbox 仍 5），实际: ${ob_total}"; exit $FAILED; }
cur=$(do_mysql "SELECT CONCAT(value,':',source_revision) FROM gacha_thirdparty.integration_metric_current WHERE external_account_id='acc-arb' AND standard_metric=1;")
[ "$cur" == "300:3" ] || { print_error "current 应 300:3，实际: ${cur}"; exit $FAILED; }

# 重放：同 request_id 同内容 → 同批号 replayed=true，不新增行
out=$("$TC" ingest 1 e2e-rq-arb-a e2e-idem-arb-a "$PROVIDER" acc-arb 1 100 1)
echo "$out" | grep -q "replayed=true" || { print_error "同 request_id 重放应 replayed=true，实际: ${out}"; exit $FAILED; }
echo "$out" | grep -q "batch_id=${batch_arb_a}" || { print_error "重放应同批号 ${batch_arb_a}，实际: ${out}"; exit $FAILED; }
raw_total=$(do_mysql "SELECT COUNT(*) FROM gacha_thirdparty.integration_metric_raw;")
[ "$raw_total" == "6" ] || { print_error "重放不应增 raw（仍 6），实际: ${raw_total}"; exit $FAILED; }

# 冲突：同幂等键异内容 → 9012
out=$("$TC" ingest 1 e2e-rq-arb-x e2e-idem-arb-a "$PROVIDER" acc-arb 1 777 9)
echo "$out" | grep -q "code=9012" || { print_error "同幂等键异内容应 9012，实际: ${out}"; exit $FAILED; }

# 非法样本 → 9011（seconds 非法 / 计数带币种 / 负值 / 槽位重复）
out=$("$TC" ingest 1 e2e-rq-bad1 e2e-idem-bad1 "$PROVIDER" "$ACCOUNT" 1 1 1 "" all 1800)
echo "$out" | grep -q "code=9011" || { print_error "seconds=1800 应 9011，实际: ${out}"; exit $FAILED; }
out=$("$TC" ingest 1 e2e-rq-bad2 e2e-idem-bad2 "$PROVIDER" "$ACCOUNT" 1 1 1 CNY)
echo "$out" | grep -q "code=9011" || { print_error "计数带币种应 9011，实际: ${out}"; exit $FAILED; }
out=$("$TC" ingest 1 e2e-rq-bad3 e2e-idem-bad3 "$PROVIDER" "$ACCOUNT" 1 -5 1)
echo "$out" | grep -q "code=9011" || { print_error "负值应 9011，实际: ${out}"; exit $FAILED; }
cat > /tmp/ingest_dup.json <<'EOF'
{"tenantId":"1","requestId":"e2e-rq-bad4","idempotencyKey":"e2e-idem-bad4","providerCode":"mock","externalAccountId":"acc-32","samples":[
 {"providerMetricCode":"impressions","standardMetric":"STANDARD_METRIC_IMPRESSIONS","externalObjectId":"obj-dup","bucketStart":"2026-10-23T08:00:00Z","bucketSeconds":"3600","dimensionKey":"all","value":"1","sourceRevision":"1"},
 {"providerMetricCode":"impressions","standardMetric":"STANDARD_METRIC_IMPRESSIONS","externalObjectId":"obj-dup","bucketStart":"2026-10-23T08:00:00Z","bucketSeconds":"3600","dimensionKey":"all","value":"1","sourceRevision":"1"}
]}
EOF
out=$("$TC" ingest-json /tmp/ingest_dup.json)
echo "$out" | grep -q "code=9011" || { print_error "槽位重复应 9011，实际: ${out}"; exit $FAILED; }
print_step "仲裁/重放/冲突/非法断言通过"

# ---------- 8. 现场清理 ----------
print_step "8. 现场清理（8 表整表清空 / admin e2e-32 / mock 进程）"
for tbl in integration_metric_batch integration_metric_raw integration_metric_current integration_metric_outbox integration_tx_ad_click; do
    do_mysql "DELETE FROM gacha_thirdparty.${tbl};"
done
for tbl in reporting_metric_inbox reporting_metric_snapshot reporting_metric_bucket; do
    do_mysql "DELETE FROM gacha_ops.${tbl};"
done
do_mysql "DELETE FROM gacha_ops.reporting_dashboard_widget WHERE dashboard_id IN (SELECT id FROM gacha_ops.reporting_dashboard WHERE dashboard_code='e2e-db-32');"
do_mysql "DELETE FROM gacha_ops.reporting_dashboard WHERE dashboard_code='e2e-db-32';"
oid=$(do_mysql "SELECT id FROM gacha_ops.console_accounts WHERE username='e2e-m15d';")
if [ -n "$oid" ]; then
    do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${oid}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${oid}; DELETE FROM gacha_ops.console_accounts WHERE id=${oid};"
fi
rid=$(do_mysql "SELECT id FROM gacha_ops.roles WHERE code='e2e-32-rpt';")
if [ -n "$rid" ]; then
    do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${rid}; DELETE FROM gacha_ops.roles WHERE id=${rid};"
fi
for port in 9410 9411; do
    mock_pid=$(netstat -ano | grep "127.0.0.1:${port}" | grep LISTENING | awk '{print $5}' | head -1)
    [ -n "$mock_pid" ] && taskkill //F //PID "$mock_pid" 2>/dev/null
done

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}√ 场景32 M15d integration 渠道收尾 通过${NC}"
    exit 0
else
    echo -e "${RED}× 场景32 失败${NC}"
    exit 1
fi
