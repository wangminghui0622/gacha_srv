#!/bin/bash
# ============================================================
# 公共配置和工具函数（被各场景脚本 source 引用）
#
# 响应统一信封 {code,msg,data}：所有断言打在信封上（code、data.xxx），
# 不允许只检查 HTTP 200（规范 07-testing 规则 7）。
# JSON 解析走 sed（本机无 python/jq 依赖）。
# ============================================================

# ---------- 依赖探测（缺失时直接报错退出，避免测试"静默失败"） ----------
_MISSING_DEPS=""
command -v curl >/dev/null 2>&1 || _MISSING_DEPS="${_MISSING_DEPS} curl"
MYSQL_BIN="${MYSQL_BIN:-/f/workspace/bin/mysql-8.0.46/bin/mysql.exe}"
if [ ! -x "${MYSQL_BIN}" ]; then
    _MISSING_DEPS="${_MISSING_DEPS} mysql(设置 MYSQL_BIN 指向 mysql.exe)"
fi
REDIS_CLI="${REDIS_CLI:-/f/workspace/bin/redis-8.10.1/redis-cli.exe}"
export REDIS_PASS="${REDIS_PASS:-we7redis}"
if [ ! -x "${REDIS_CLI}" ]; then
    _MISSING_DEPS="${_MISSING_DEPS} redis-cli(设置 REDIS_CLI 指向 redis-cli.exe)"
fi
if [ -n "${_MISSING_DEPS}" ]; then
    echo "缺少依赖:${_MISSING_DEPS}" >&2
    exit 1
fi

# 网关地址（可用环境变量 BASE_URL 覆盖；默认本地网关 9310，前缀 /gacha/api/v1）
BASE_URL="${BASE_URL:-http://127.0.0.1:9310/gacha/api/v1}"

# 数据库连接参数：与 pkg/etcs/*.yaml 的 DataSource 保持一致（root / we7root）。
# 仅用于查询/重置网关未暴露 HTTP 接口的数据（如奖池剩余库存）。
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
export DB_PASS="${DB_PASS:-we7root}"

# 断言失败标志：print_error 会置 1，场景脚本结尾统一 exit $FAILED
FAILED=0

# 颜色输出
export GREEN='\033[0;32m'
export BLUE='\033[0;34m'
export YELLOW='\033[1;33m'
export RED='\033[0;31m'
export NC='\033[0m' # No Color

# ---------- HTTP 请求 ----------

# do_post <url> <json> [token] [idemKey]：POST JSON；可选 Bearer 令牌与 Idempotency-Key 头
# JSON 走临时文件传输：内联 -d 的中文会被 Windows argv 转成 GBK 破坏 UTF-8（触发校验失败）
do_post() {
    local url=$1 json=$2 token=${3:-} key=${4:-}
    local tmp
    tmp=$(mktemp)
    printf '%s' "$json" > "$tmp"
    local args=(-s -X POST -H "Content-Type: application/json")
    [ -n "$token" ] && args+=(-H "Authorization: Bearer ${token}")
    [ -n "$key" ] && args+=(-H "Idempotency-Key: ${key}")
    curl "${args[@]}" --data-binary "@${tmp}" "${BASE_URL}${url}"
    rm -f "$tmp"
}

# do_put <url> <json> [token]：PUT；可选 Bearer 令牌
do_put() {
    local url=$1 json=$2 token=${3:-}
    local tmp
    tmp=$(mktemp)
    printf '%s' "$json" > "$tmp"
    local args=(-s -X PUT -H "Content-Type: application/json")
    [ -n "$token" ] && args+=(-H "Authorization: Bearer ${token}")
    curl "${args[@]}" --data-binary "@${tmp}" "${BASE_URL}${url}"
    rm -f "$tmp"
}

# do_get <url> [token]：GET；可选 Bearer 令牌
do_get() {
    local url=$1 token=${2:-}
    if [ -n "$token" ]; then
        curl -s -H "Authorization: Bearer ${token}" "${BASE_URL}${url}"
    else
        curl -s "${BASE_URL}${url}"
    fi
}

# ---------- 信封解析（sed 实现；响应为紧凑 JSON，字段唯一出现） ----------

# extract_field <json> <field>：提取信封内字段值（字符串去引号）
extract_field() {
    echo "$1" | sed -n "s/.*\"$2\":\([^,}]*\).*/\1/p" | tr -d '"'
}

# extract_code <json>：提取信封 code（断言必打在信封上）
extract_code() {
    echo "$1" | sed -n 's/^.*"code":\(-\?[0-9]*\).*$/\1/p'
}

# ---------- 用户 ----------

# register_user <前缀>：注册唯一用户（用户名=前缀+时间戳+随机数，防同秒撞名），输出 "userId username"
register_user() {
    local name="${1}$(date +%s)${RANDOM}"
    local resp
    resp=$(do_post /user/register "{\"username\":\"${name}\",\"password\":\"pass123456\",\"nickname\":\"e2e测试\"}")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "注册失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" userId) ${name}"
}

# login_by_name <username>：登录固定 e2e 密码（pass123456），输出 token
login_by_name() {
    local name=$1
    local resp
    resp=$(do_post /user/login "{\"username\":\"${name}\",\"password\":\"pass123456\"}")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "登录失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" token)"
}

# ---------- 业务操作 ----------

# buy_box <token> <boxId> [idemKey] [userCouponId] [useLoyalty]：购买开奖（SAGA 提交），输出 requestId
# M10：可选券（userCouponId>0 时传入）/积分抵扣（useLoyalty=1 时传入）
buy_box() {
    local token=$1 box_id=$2 key=${3:-} coupon=${4:-0} loyalty=${5:-0}
    local json resp
    json="{\"boxId\":${box_id}"
    [ "$coupon" -gt 0 ] 2>/dev/null && json="${json},\"userCouponId\":${coupon}"
    [ "$loyalty" == "1" ] && json="${json},\"useLoyaltyOffset\":true"
    json="${json}}"
    resp=$(do_post /box/buy "$json" "$token" "$key")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "购买失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" requestId)"
}

# poll_record <token> <requestId> [timeoutSec=20]：轮询抽赏记录直到出现
# 输出: 命中记录的完整 JSON；超时无输出且返回 1。
# 注: 开奖失败（余额不足/售罄退款）不会产生记录——轮询必然超时，
#     此时以「余额未变/无记录」作为失败断言（v1 无失败记录表）。
poll_record() {
    local token=$1 rid=$2 timeout=${3:-20}
    local i=0 resp code
    while [ "$i" -lt $((timeout * 2)) ]; do
        resp=$(do_get "/draw/records?requestId=${rid}" "$token")
        code=$(extract_code "$resp")
        if [ "$code" == "0" ]; then
            echo "$resp"
            return 0
        fi
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# get_balance <token>：GET /user/balance，输出余额（元字符串，如 "100.00"）
get_balance() {
    local resp
    resp=$(do_get /user/balance "$1")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "查询余额失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" balance)"
}

# fen_of <元字符串>：把 "100.00" 转整数分（10# 前缀防前导零八进制歧义）
fen_of() {
    local fen
    fen=$(echo "$1" | sed 's/\.//')
    echo $((10#${fen}))
}

# wait_balance <token> <期望元字符串> [timeoutSec=20]：轮询余额直到等于期望值
# （SAGA 退款/扣款是异步的，用它替代固定 sleep；超时返回 1）
wait_balance() {
    local token=$1 expect=$2 timeout=${3:-20}
    local i=0 b
    while [ "$i" -lt $((timeout * 2)) ]; do
        b=$(get_balance "$token") || return 1
        if [ "$(fen_of "$b")" == "$(fen_of "$expect")" ]; then
            echo "$b"
            return 0
        fi
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# ---------- MySQL 直查（只查 HTTP 未暴露的数据：奖池库存 / 记录计数） ----------

# do_mysql <SQL>：输出查询结果（-N -B）；失败无输出返回非 0
do_mysql() {
    "${MYSQL_BIN}" --connect-timeout=5 -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" -p"${DB_PASS}" \
        --default-character-set=utf8mb4 -N -B -e "$1" 2>/dev/null
}

# get_entry_remaining <boxId> <entryName>：奖池条目剩余库存（-1=不限量）
get_entry_remaining() {
    do_mysql "SELECT remaining FROM gacha_draw.draw_pool_entries WHERE box_id=$1 AND \`name\`='$2';"
}

# count_records <requestId>：按全局事务单号数抽赏记录
count_records() {
    do_mysql "SELECT COUNT(*) FROM gacha_draw.draw_records WHERE request_id='$1';"
}

# ---------- M7 钱包断言（wallet_accounts / wallet_ledgers） ----------

# get_wallet_available <userId> <kind>：钱包可用余额（分；kind 1 CASH 2 BONUS 3 REDEEM_POINT 4 LOYALTY_POINT）
get_wallet_available() {
    do_mysql "SELECT available FROM gacha_money.wallet_accounts WHERE user_id=$1 AND wallet_kind=$2;"
}

# get_wallet_held <userId> <kind>：钱包冻结余额（分）
get_wallet_held() {
    do_mysql "SELECT held FROM gacha_money.wallet_accounts WHERE user_id=$1 AND wallet_kind=$2;"
}

# count_wallet_ledgers <where片段>：按条件数流水（片段为 SQL 拼串，仅测试脚本内部使用）
count_wallet_ledgers() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_ledgers WHERE $1;"
}

# count_wallet_holds <where片段>：按条件数冻结单
count_wallet_holds() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_holds WHERE $1;"
}

# get_ledger_field <where片段> <字段>：取单条流水字段值（如 amount/status/biz_type）
get_ledger_field() {
    do_mysql "SELECT $2 FROM gacha_money.wallet_ledgers WHERE $1 LIMIT 1;"
}

# ---------- M8 支付断言（gacha_money：payment_orders / payment_legs） ----------

# payment_no <requestId>：支付单号确定性派生（"P"+requestId）
payment_no() {
    echo "P$1"
}

# get_payment_status <requestId>：支付单状态（1 CREATED 2 AWAITING 3 PAID 4 REFUNDING 5 REFUNDED 6 CANCELED）
get_payment_status() {
    do_mysql "SELECT status FROM gacha_money.payment_orders WHERE payment_no='$(payment_no "$1")';"
}

# wait_payment_paid <requestId> <maxWaitSec>：轮询支付单至 PAID(3)，成功返回 0
wait_payment_paid() {
    local rid=$1 max=${2:-20} i=0 st
    while [ "$i" -lt "$max" ]; do
        st=$(get_payment_status "$rid")
        [ "$st" == "3" ] && return 0
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# wait_payment_status <requestId> <expectStatus> <maxWaitSec>：轮询支付单至指定状态，成功返回 0
wait_payment_status() {
    local rid=$1 expect=$2 max=${3:-20} i=0 st
    while [ "$i" -lt "$max" ]; do
        st=$(get_payment_status "$rid")
        [ "$st" == "$expect" ] && return 0
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# count_payment_orders <requestId>：按支付单号数支付单（幂等：恰 1 条）
count_payment_orders() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.payment_orders WHERE payment_no='$(payment_no "$1")';"
}

# get_payment_leg_status <requestId> <walletKind>：资金腿状态（1 PENDING 2 POSTED 3 VOIDED）
get_payment_leg_status() {
    do_mysql "SELECT status FROM gacha_money.payment_legs WHERE leg_no='$(payment_no "$1")_$2';"
}

# get_payment_hold_status <requestId> <walletKind>：user 侧冻结状态（1 PENDING 2 POSTED 3 VOIDED 4 EXPIRED）
get_payment_hold_status() {
    do_mysql "SELECT status FROM gacha_money.wallet_holds WHERE biz_id='$(payment_no "$1")_$2';"
}

# ---------- M9 基础域断言（gacha_base：决策日志 / 风控案例） ----------

# （M14d 起资格策略走 CEL 引擎：场景解析直查库，无 Redis 场景缓存）

# flush_box_entries_cache <boxId>：删除奖池条目列表缓存（3 分钟 TTL，售罄场景自复位后必调，
# 否则残留的「售罄视图」会毒化预热，导致预检 2002 而非走 SAGA 实检）
flush_box_entries_cache() {
    "${REDIS_CLI}" -h 127.0.0.1 -p 6379 -a "${REDIS_PASS}" --no-auth-warning DEL "cache:gachaDraw:drawPoolEntries:box:$1" >/dev/null
}

# decision_no <requestId>：决策单号确定性派生（"d_"+requestId）
decision_no() {
    echo "d_$1"
}

# get_decision_result <requestId>：决策日志结果（1 通过 0 拒绝；无记录为空）
get_decision_result() {
    do_mysql "SELECT result FROM gacha_base.eligibility_decision_logs WHERE decision_no='$(decision_no "$1")';"
}

# get_decision_policy <requestId>：决策所用策略单号
get_decision_policy() {
    do_mysql "SELECT policy_no FROM gacha_base.eligibility_decision_logs WHERE decision_no='$(decision_no "$1")';"
}

# wait_decision_result <requestId> <expect> <maxWaitSec>：轮询决策日志至期望结果，成功返回 0
wait_decision_result() {
    local rid=$1 expect=$2 max=${3:-20} i=0 r
    while [ "$i" -lt "$max" ]; do
        r=$(get_decision_result "$rid")
        [ "$r" == "$expect" ] && return 0
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# get_risk_case_status <requestId>：风控案例状态（M14d 起数值化：1 OPEN/2 IN_REVIEW/3 RESOLVED/4 DISMISSED，
# SQL 侧映射回串值供场景断言；无案例为空）
get_risk_case_status() {
    do_mysql "SELECT CASE status WHEN 1 THEN 'OPEN' WHEN 2 THEN 'IN_REVIEW'
              WHEN 3 THEN 'RESOLVED' WHEN 4 THEN 'DISMISSED' ELSE status END
              FROM gacha_base.risk_cases WHERE case_no='rk_$1';"
}

# get_risk_case_rule <requestId>：风控案例命中规则单号
get_risk_case_rule() {
    do_mysql "SELECT rule_no FROM gacha_base.risk_cases WHERE case_no='rk_$1';"
}

# wait_risk_case_status <requestId> <expect> <maxWaitSec>：轮询案例至期望状态，成功返回 0
wait_risk_case_status() {
    local rid=$1 expect=$2 max=${3:-20} i=0 st
    while [ "$i" -lt "$max" ]; do
        st=$(get_risk_case_status "$rid")
        [ "$st" == "$expect" ] && return 0
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# ---------- M10 玩法域操作（gacha_marketing：领券/报价/用户券/报价锁） ----------

# claim_coupon <token> <templateNo>：领取券，输出 couponNo
claim_coupon() {
    local resp
    resp=$(do_post /coupons/claim "{\"templateNo\":\"$2\"}" "$1")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "领券失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" couponNo)"
}

# get_quote <token> <boxId> [userCouponId] [useLoyalty]：GET /box/quote 报价试算，输出完整响应
get_quote() {
    local token=$1 box_id=$2 coupon=${3:-0} loyalty=${4:-0}
    local url="/box/quote?boxId=${box_id}"
    [ "$coupon" -gt 0 ] 2>/dev/null && url="${url}&userCouponId=${coupon}"
    [ "$loyalty" == "1" ] && url="${url}&useLoyaltyOffset=true"
    do_get "$url" "$token"
}

# get_user_coupon_id <userId> <templateNo>：该用户最新领取的指定模板券 id
get_user_coupon_id() {
    do_mysql "SELECT id FROM gacha_marketing.user_coupons WHERE user_id=$1 AND template_no='$2' ORDER BY id DESC LIMIT 1;"
}

# get_coupon_status <userCouponId>：用户券状态（1 AVAILABLE 2 LOCKED 3 USED 4 EXPIRED）
get_coupon_status() {
    do_mysql "SELECT status FROM gacha_marketing.user_coupons WHERE id=$1;"
}

# get_coupon_locked_by <userCouponId>：锁定方 quote_token（LOCKED 时非空）
get_coupon_locked_by() {
    do_mysql "SELECT locked_by FROM gacha_marketing.user_coupons WHERE id=$1;"
}

# wait_coupon_status <userCouponId> <expect> [maxWaitSec]：轮询券至期望状态，成功返回 0
wait_coupon_status() {
    local cid=$1 expect=$2 max=${3:-20} i=0 st
    while [ "$i" -lt "$max" ]; do
        st=$(get_coupon_status "$cid")
        [ "$st" == "$expect" ] && return 0
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# get_quote_status <requestId>：报价锁状态（1 OPEN 2 LOCKED 3 USED 4 RELEASED 5 EXPIRED）
get_quote_status() {
    do_mysql "SELECT status FROM gacha_marketing.quote_locks WHERE request_id='$1';"
}

# wait_quote_status <requestId> <expect> [maxWaitSec]：轮询报价锁至期望状态，成功返回 0
wait_quote_status() {
    local rid=$1 expect=$2 max=${3:-20} i=0 st
    while [ "$i" -lt "$max" ]; do
        st=$(get_quote_status "$rid")
        [ "$st" == "$expect" ] && return 0
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# ---------- M11b 挖宝箱（gacha_play：dig_box_*） ----------

# dig_quote <token> <boxId> [cellsCsv] [takeAll]：挖宝报价预览，输出完整响应
dig_quote() {
    local token=$1 box_id=$2 cells=${3:-} take_all=${4:-}
    local url="/dig-boxes/${box_id}/quote?"
    [ -n "$cells" ] && url="${url}cells=${cells}&"
    [ "$take_all" == "1" ] && url="${url}takeAll=true&"
    do_get "${url%&}" "$token"
}

# dig_buy <token> <boxId> <cellsCsv|empty> [takeAll=1] [idemKey]：挖宝购买（SAGA 提交），输出 requestId
# 用法：dig_buy "$token" 1 "" 1 "my-key"  → 全开；dig_buy "$token" 1 "0,1" "" "my-key" → 指定格
dig_buy() {
    local token=$1 box_id=$2 cells=${3:-} take_all=${4:-} key=${5:-}
    local json resp
    json="{\"boxId\":${box_id}"
    if [ "$take_all" == "1" ]; then
        json="${json},\"takeAll\":true"
    else
        json="${json},\"cellIndexes\":[${cells}]"
    fi
    json="${json}}"
    resp=$(do_post "/dig-boxes/${box_id}/buy" "$json" "$token" "$key")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "挖宝购买失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" requestId)"
}

# poll_dig_record <token> <requestId> [timeoutSec=20]：轮询挖宝记录直到出现，输出记录完整 JSON
poll_dig_record() {
    local token=$1 rid=$2 timeout=${3:-20}
    local i=0 resp code
    while [ "$i" -lt $((timeout * 2)) ]; do
        resp=$(do_get "/dig-boxes/records?requestId=${rid}" "$token")
        code=$(extract_code "$resp")
        if [ "$code" == "0" ]; then
            echo "$resp"
            return 0
        fi
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# get_dig_session_field <userId> <boxId> <字段>：直查会话（occupied_bitmap/total_cells_paid/total_price_fen/status）
get_dig_session_field() {
    do_mysql "SELECT $3 FROM gacha_play.dig_box_sessions WHERE user_id=$1 AND config_id=$2;"
}

# count_dig_records <requestId>：按全局事务单号数挖宝记录
count_dig_records() {
    do_mysql "SELECT COUNT(*) FROM gacha_play.dig_box_records WHERE request_id='$1';"
}

# count_dig_prize_ledgers <gid>：该 gid 发奖钱包流水数（log_no="c_dgp_"+gid+"#"+idx）
count_dig_prize_ledgers() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_dgp_$1#%';"
}

# count_dig_prize_coupons <gid>：该 gid 发奖券数（coupon_no="uc_dgp_"+gid+"#"+idx）
count_dig_prize_coupons() {
    do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE coupon_no LIKE 'uc_dgp_$1#%';"
}

# set_dig_session <userId> <boxId> <bitmap> <paidCells> <priceFen> <status>：预置会话状态（已挖格测试/复原）
set_dig_session() {
    do_mysql "UPDATE gacha_play.dig_box_sessions SET occupied_bitmap=$3, total_cells_paid=$4, total_price_fen=$5, status=$6 WHERE user_id=$1 AND config_id=$2;"
}

# sum_dig_cash_ledger <gid>：该 gid 现金发奖流水金额合计（分）
sum_dig_cash_ledger() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_dgp_$1#%' AND wallet_kind=1;"
}

# count_barrier_comp <gid>：DTM 屏障补偿行数（失败 SAGA 收敛信号：补偿分支屏障落库）
count_barrier_comp() {
    do_mysql "SELECT COUNT(*) FROM dtm_barrier.barrier WHERE gid='$1' AND op='compensate';"
}

# wait_barrier_compensated <gid> [maxWaitSec=20]：轮询屏障补偿行出现（失败 SAGA 已走完补偿链）
wait_barrier_compensated() {
    local gid=$1 max=${2:-20} i=0 c
    while [ "$i" -lt $((max * 2)) ]; do
        c=$(count_barrier_comp "$gid")
        if [ -n "$c" ] && [ "$c" -ge 1 ]; then
            return 0
        fi
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# ---------- M12 滚动中奖（gacha_draw：rolling feed / 规则） ----------

# get_rolling_feed [marqueeOnly=1] [limit] [boxId]：滚动中奖 feed，输出完整响应
get_rolling_feed() {
    local marquee=${1:-1} limit=${2:-20} box_id=${3:-0}
    local url="/rolling-draw/recent-wins?marqueeOnly=$([ "$marquee" == "1" ] && echo true || echo false)&limit=${limit}"
    [ "$box_id" -gt 0 ] 2>/dev/null && url="${url}&boxId=${box_id}"
    do_get "$url"
}

# count_field <json> <grep模式>：数模式出现次数（数组计数代理，sed 无 JSON 数组解析能力）
# 模式整体透传（含引号），如：count_field "$resp" '"tier":3'
count_field() {
    echo "$1" | grep -o "$2" | wc -l
}

# set_rolling_rules <boxId> <json>：覆盖滚动规则（场景自复位用；-- 为清空）
set_rolling_rules() {
    if [ "$2" == "--" ]; then
        do_mysql "UPDATE gacha_draw.draw_boxes SET rolling_rules_json=NULL WHERE id=$1;"
    else
        do_mysql "UPDATE gacha_draw.draw_boxes SET rolling_rules_json='$2' WHERE id=$1;"
    fi
}

# set_entry_attr <boxId> <name> <字段> <值>：改条目权重/展示价/限量标记（自复位用）
set_entry_attr() {
    do_mysql "UPDATE gacha_draw.draw_pool_entries SET $3=$4 WHERE box_id=$1 AND \`name\`='$2';"
}

# ---------- M12b 满车（gacha_play：full_car_*） ----------

# get_full_car <token> <configId>：满车详情，输出完整响应
get_full_car() {
    do_get "/full-car/$2" "$1"
}

# full_car_buy <token> <configId> [seatIndex=-1] [idemKey]：购座（SAGA 提交），输出 requestId
full_car_buy() {
    local token=$1 config_id=$2 seat=${3:--1} key=${4:-}
    local resp
    resp=$(do_post "/full-car/${config_id}/buy" "{\"seatIndex\":${seat}}" "$token" "$key")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "满车购买失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" requestId)"
}

# poll_full_car_record <token> <requestId> [timeoutSec=20]：轮询满车记录直到出现，输出记录完整 JSON
poll_full_car_record() {
    local token=$1 rid=$2 timeout=${3:-20}
    local i=0 resp code
    while [ "$i" -lt $((timeout * 2)) ]; do
        resp=$(do_get "/full-car/records?requestId=${rid}" "$token")
        code=$(extract_code "$resp")
        if [ "$code" == "0" ]; then
            echo "$resp"
            return 0
        fi
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# get_full_car_state_field <configId> <字段>：直查状态（seats_json/awards_json/remaining_tickets/status）
get_full_car_state_field() {
    do_mysql "SELECT $2 FROM gacha_play.full_car_state WHERE config_id=$1;"
}

# reset_full_car_state <configId>：满车现场复位（空车重开：全空座/清奖/余票=座位数/OPEN）
reset_full_car_state() {
    local n zeros="" i
    n=$(do_mysql "SELECT seat_count FROM gacha_play.full_car_configs WHERE id=$1;")
    for ((i = 0; i < n; i++)); do zeros="${zeros}0,"; done
    zeros="[${zeros%,}]"
    do_mysql "UPDATE gacha_play.full_car_state SET seats_json='${zeros}', awards_json='[]', remaining_tickets=${n}, status=1 WHERE config_id=$1;"
}

# count_full_car_records <requestId>：按全局事务单号数满车记录
count_full_car_records() {
    do_mysql "SELECT COUNT(*) FROM gacha_play.full_car_records WHERE request_id='$1';"
}

# set_full_car_entry_stock <configId> <name> <remaining>：改满车条目库存（售罄注入/复原用）
set_full_car_entry_stock() {
    do_mysql "UPDATE gacha_play.full_car_entries SET remaining=$3 WHERE config_id=$1 AND \`name\`='$2';"
}

# count_full_car_prize_ledgers <gid>：该 gid 整车奖钱包流水数（log_no="c_fcp_"+gid+"#"+idx）
count_full_car_prize_ledgers() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_fcp_$1#%';"
}

# count_full_car_prize_coupons <gid>：该 gid 整车奖券数（coupon_no="uc_fcp_"+gid+"#"+idx）
count_full_car_prize_coupons() {
    do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE coupon_no LIKE 'uc_fcp_$1#%';"
}

# sum_full_car_cash_ledger <gid>：该 gid 整车奖现金流水金额合计（分）
sum_full_car_cash_ledger() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_fcp_$1#%' AND wallet_kind=1;"
}

# sum_json_field <json> <字段>：对 JSON 中 "字段":数字 全部出现求和（awards_json 与钱包对账用）
sum_json_field() {
    echo "$1" | grep -o "\"$2\":[0-9]*" | sed "s/\"$2\"://" | awk '{s+=$1} END {print s+0}'
}

# ---------- M12c 扭蛋（gacha_play：gashapon_*） ----------

# get_gashapon <token> <configId>：扭蛋详情，输出完整响应
get_gashapon() {
    do_get "/gashapon/$2" "$1"
}

# gashapon_buy <token> <configId> <count> [idemKey]：扭蛋抽取（SAGA 提交），输出 requestId
gashapon_buy() {
    local token=$1 config_id=$2 count=$3 key=${4:-}
    local resp
    resp=$(do_post "/gashapon/${config_id}/buy" "{\"count\":${count}}" "$token" "$key")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "扭蛋购买失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" requestId)"
}

# poll_gashapon_record <token> <requestId> [timeoutSec=20]：轮询扭蛋记录直到出现，输出记录完整 JSON
poll_gashapon_record() {
    local token=$1 rid=$2 timeout=${3:-20}
    local i=0 resp code
    while [ "$i" -lt $((timeout * 2)) ]; do
        resp=$(do_get "/gashapon/records?requestId=${rid}" "$token")
        code=$(extract_code "$resp")
        if [ "$code" == "0" ]; then
            echo "$resp"
            return 0
        fi
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# get_gashapon_state_field <configId> <字段>：直查状态（participants_json/records_json/sent_json/awards_json/remaining_tickets/status）
get_gashapon_state_field() {
    do_mysql "SELECT $2 FROM gacha_play.gashapon_state WHERE config_id=$1;"
}

# reset_gashapon_state <configId>：扭蛋现场复位（余票=Σquantity、participants {}、records []、
# sent 全 0、awards []、OPEN；清记录与幸运指派、条目库存=quantity）
reset_gashapon_state() {
    local n zeros="" i pcount
    n=$(do_mysql "SELECT COALESCE(SUM(quantity),0) FROM gacha_play.gashapon_entries WHERE config_id=$1;")
    pcount=$(do_mysql "SELECT JSON_LENGTH(special_prizes_json) FROM gacha_play.gashapon_configs WHERE id=$1;")
    for ((i = 0; i < pcount; i++)); do zeros="${zeros}0,"; done
    zeros="[${zeros%,}]"
    do_mysql "UPDATE gacha_play.gashapon_state SET participants_json='{}', records_json='[]', sent_json='${zeros}', awards_json='[]', remaining_tickets=${n}, status=1 WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.gashapon_records WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.gashapon_lucky_assignments WHERE config_id=$1;"
    do_mysql "UPDATE gacha_play.gashapon_entries SET remaining=quantity WHERE config_id=$1;"
}

# count_gashapon_records <requestId>：按全局事务单号数扭蛋记录
count_gashapon_records() {
    do_mysql "SELECT COUNT(*) FROM gacha_play.gashapon_records WHERE request_id='$1';"
}

# set_gashapon_entry_stock <configId> <name> <remaining>：改扭蛋条目库存（售罄注入/复原用；同名条目一并改）
set_gashapon_entry_stock() {
    do_mysql "UPDATE gacha_play.gashapon_entries SET remaining=$3 WHERE config_id=$1 AND \`name\`='$2';"
}

# set_lucky_assignment <configId> <prizeIndex> [userId]：幸运指派 upsert（缺省/0=清除）
# 场景用 SQL 直插（D4：指派无 HTTP 路由，仅 play-rpc RPC 供未来 ops 接入）
set_lucky_assignment() {
    local user_id=${3:-0}
    if [ "$user_id" == "0" ]; then
        do_mysql "DELETE FROM gacha_play.gashapon_lucky_assignments WHERE config_id=$1 AND prize_index=$2;"
    else
        do_mysql "INSERT INTO gacha_play.gashapon_lucky_assignments (config_id, prize_index, user_id) VALUES ($1, $2, ${user_id}) ON DUPLICATE KEY UPDATE user_id=${user_id};"
    fi
}

# count_lucky_assignments <configId>：数幸运指派行
count_lucky_assignments() {
    do_mysql "SELECT COUNT(*) FROM gacha_play.gashapon_lucky_assignments WHERE config_id=$1;"
}

# count_gashapon_prize_ledgers <gid>：该 gid 特别奖钱包流水数（log_no="c_gcp_"+gid+"#"+idx）
count_gashapon_prize_ledgers() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_gcp_$1#%';"
}

# count_gashapon_prize_coupons <gid>：该 gid 特别奖券数（coupon_no="uc_gcp_"+gid+"#"+idx）
count_gashapon_prize_coupons() {
    do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE coupon_no LIKE 'uc_gcp_$1#%';"
}

# sum_gashapon_cash_ledger <gid>：该 gid 特别奖现金流水金额合计（分，wallet_kind=1）
sum_gashapon_cash_ledger() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_gcp_$1#%' AND wallet_kind=1;"
}

# gasha_key_mod3 <gid> <position>：输出 SHA256(gid‖"stage"‖BE32(position)) 前 8 字节大端 % 3
# （与 core.gashaponRandom 同源；分阶段 LUCKY winningTicket=minTicket+该值%区间长，预计算中奖者用）
gasha_key_mod3() {
    local gid=$1 pos=$2 hex n=0 i d
    if [ "$pos" == "0" ]; then
        hex=$(printf '%sstage\x00\x00\x00\x00' "$gid" | sha256sum | cut -c1-16)
    else
        hex=$(printf '%sstage\x00\x00\x00\x01' "$gid" | sha256sum | cut -c1-16)
    fi
    for ((i = 0; i < 16; i++)); do
        d=${hex:i:1}
        case $d in
            a|A) d=10 ;;
            b|B) d=11 ;;
            c|C) d=12 ;;
            d|D) d=13 ;;
            e|E) d=14 ;;
            f|F) d=15 ;;
        esac
        n=$(( (n * 16 + d) % 3 ))
    done
    echo "$n"
}

# gasha_pick_key <prefix> <position> [want=2]：找一个 SHA256%3==want 的幂等键（预计算分阶段中奖者）
gasha_pick_key() {
    local prefix=$1 pos=$2 want=${3:-2} n
    for ((n = 0; n < 2000; n++)); do
        if [ "$(gasha_key_mod3 "${prefix}_${n}" "$pos")" == "$want" ]; then
            echo "${prefix}_${n}"
            return 0
        fi
    done
    return 1
}

# ---------- M13 连击链 combo_chain + 拼团抽 group_buy（gacha_play：combo_chain_* / group_buy_*） ----------

# gacha_hash_mod <gid> <domain> <pos> <mod>：确定性随机数 = SHA256(gid‖domain‖BE32(pos))
# 前 8 字节大端 % mod（与 core.FullCarRandom 同源；连击链/拼团抽结算预计算中奖者用，
# gasha_key_mod3 即本函数 domain=stage 的特例）
gacha_hash_mod() {
    local gid=$1 domain=$2 pos=$3 mod=$4 hex n=0 i d b0 b1 b2 b3
    b0=$(( (pos >> 24) & 255 ))
    b1=$(( (pos >> 16) & 255 ))
    b2=$(( (pos >> 8) & 255 ))
    b3=$(( pos & 255 ))
    hex=$( { printf '%s%s' "$gid" "$domain"; printf "\\$(printf '%03o' "$b0")\\$(printf '%03o' "$b1")\\$(printf '%03o' "$b2")\\$(printf '%03o' "$b3")"; } | sha256sum | cut -c1-16)
    for ((i = 0; i < 16; i++)); do
        d=${hex:i:1}
        case $d in
            a|A) d=10 ;;
            b|B) d=11 ;;
            c|C) d=12 ;;
            d|D) d=13 ;;
            e|E) d=14 ;;
            f|F) d=15 ;;
        esac
        n=$(( (n * 16 + d) % mod ))
    done
    echo "$n"
}

# get_combo_chain <token> <configId>：连击链详情，输出完整响应
get_combo_chain() {
    do_get "/combo-chain/$2" "$1"
}

# combo_chain_buy <token> <configId> [idemKey]：连击链购票（SAGA 提交），输出 requestId
combo_chain_buy() {
    local token=$1 config_id=$2 key=${3:-}
    local resp
    resp=$(do_post "/combo-chain/${config_id}/buy" '{}' "$token" "$key")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "连击链购买失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" requestId)"
}

# poll_combo_chain_record <token> <requestId> [timeoutSec=20]：轮询连击链记录直到出现，输出记录完整 JSON
poll_combo_chain_record() {
    local token=$1 rid=$2 timeout=${3:-20}
    local i=0 resp code
    while [ "$i" -lt $((timeout * 2)) ]; do
        resp=$(do_get "/combo-chain/records?requestId=${rid}" "$token")
        code=$(extract_code "$resp")
        if [ "$code" == "0" ]; then
            echo "$resp"
            return 0
        fi
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# get_combo_chain_state_field <configId> <字段>：直查状态（car_index/participants_json/
# awards_json/tier_remaining_json/streaks_before_json/remaining_tickets/status）
get_combo_chain_state_field() {
    do_mysql "SELECT $2 FROM gacha_play.combo_chain_state WHERE config_id=$1;"
}

# reset_combo_chain_state <configId>：连击链现场复位（车 1、participants []、awards []、
# COMBO_TIER 档位库存复位为 1/档、streaks_before {}、余票=tickets_per_car、OPEN；清记录与连击）
reset_combo_chain_state() {
    local n tiers zeros="" i
    n=$(do_mysql "SELECT tickets_per_car FROM gacha_play.combo_chain_configs WHERE id=$1;")
    tiers=$(do_mysql "SELECT (LENGTH(shared_prizes_json)-LENGTH(REPLACE(shared_prizes_json,'\"kind\":\"COMBO_TIER\"',''))) DIV LENGTH('\"kind\":\"COMBO_TIER\"') FROM gacha_play.combo_chain_configs WHERE id=$1;")
    for ((i = 0; i < tiers; i++)); do zeros="${zeros}1,"; done
    zeros="[${zeros%,}]"
    do_mysql "UPDATE gacha_play.combo_chain_state SET car_index=1, participants_json='[]', awards_json='[]', tier_remaining_json='${zeros}', streaks_before_json='{}', remaining_tickets=${n}, status=1 WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.combo_chain_records WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.combo_chain_streaks WHERE config_id=$1;"
}

# count_combo_chain_records <requestId>：按全局事务单号数连击链记录
count_combo_chain_records() {
    do_mysql "SELECT COUNT(*) FROM gacha_play.combo_chain_records WHERE request_id='$1';"
}

# count_combo_chain_prize_ledgers <gid>：该 gid 结算奖钱包流水数（log_no="c_ccp_"+gid+"#"+idx）
count_combo_chain_prize_ledgers() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_ccp_$1#%';"
}

# count_combo_chain_prize_coupons <gid>：该 gid 结算奖券数（coupon_no="uc_ccp_"+gid+"#"+idx）
count_combo_chain_prize_coupons() {
    do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE coupon_no LIKE 'uc_ccp_$1#%';"
}

# sum_combo_chain_cash_ledger <gid>：该 gid 结算奖现金流水金额合计（分，wallet_kind=1）
sum_combo_chain_cash_ledger() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_ccp_$1#%' AND wallet_kind=1;"
}

# sum_combo_chain_points_ledger <gid>：该 gid 结算奖积分流水金额合计（wallet_kind=4）
sum_combo_chain_points_ledger() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_ccp_$1#%' AND wallet_kind=4;"
}

# get_group_buy <token> <configId>：拼团详情，输出完整响应
get_group_buy() {
    do_get "/group-buy/$2" "$1"
}

# group_buy_buy <token> <configId> <count> [idemKey]：拼团下注（SAGA 提交），输出 requestId
group_buy_buy() {
    local token=$1 config_id=$2 count=$3 key=${4:-}
    local resp
    resp=$(do_post "/group-buy/${config_id}/buy" "{\"count\":${count}}" "$token" "$key")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "拼团下注失败: ${resp}" >&2
        return 1
    fi
    echo "$(extract_field "$resp" requestId)"
}

# poll_group_buy_record <token> <requestId> [timeoutSec=20]：轮询拼团记录直到出现，输出记录完整 JSON
poll_group_buy_record() {
    local token=$1 rid=$2 timeout=${3:-20}
    local i=0 resp code
    while [ "$i" -lt $((timeout * 2)) ]; do
        resp=$(do_get "/group-buy/records?requestId=${rid}" "$token")
        code=$(extract_code "$resp")
        if [ "$code" == "0" ]; then
            echo "$resp"
            return 0
        fi
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# get_group_buy_state_field <configId> <字段>：直查状态（bets_json/awards_json/lucky_sent_json/remaining_bets/status）
get_group_buy_state_field() {
    do_mysql "SELECT $2 FROM gacha_play.group_buy_state WHERE config_id=$1;"
}

# reset_group_buy_state <configId>：拼团现场复位（bets []、awards []、lucky_sent 全 0、
# 余位=target_bet_count、OPEN；清记录；条目库存=quantity）
reset_group_buy_state() {
    local n zeros="" i pcount
    n=$(do_mysql "SELECT target_bet_count FROM gacha_play.group_buy_configs WHERE id=$1;")
    pcount=$(do_mysql "SELECT JSON_LENGTH(lucky_prizes_json) FROM gacha_play.group_buy_configs WHERE id=$1;")
    for ((i = 0; i < pcount; i++)); do zeros="${zeros}0,"; done
    zeros="[${zeros%,}]"
    do_mysql "UPDATE gacha_play.group_buy_state SET bets_json='[]', awards_json='[]', lucky_sent_json='${zeros}', remaining_bets=${n}, status=1 WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.group_buy_records WHERE config_id=$1;"
    do_mysql "UPDATE gacha_play.group_buy_entries SET remaining=quantity WHERE config_id=$1;"
}

# count_group_buy_records <requestId>：按全局事务单号数拼团记录
count_group_buy_records() {
    do_mysql "SELECT COUNT(*) FROM gacha_play.group_buy_records WHERE request_id='$1';"
}

# count_group_buy_prize_ledgers <gid>：该 gid 结算奖钱包流水数（log_no="c_gbp_"+gid+"#"+idx）
count_group_buy_prize_ledgers() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_gbp_$1#%';"
}

# count_group_buy_prize_coupons <gid>：该 gid 结算奖券数（coupon_no="uc_gbp_"+gid+"#"+idx）
count_group_buy_prize_coupons() {
    do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE coupon_no LIKE 'uc_gbp_$1#%';"
}

# sum_group_buy_cash_ledger <gid>：该 gid 结算奖现金流水金额合计（分，wallet_kind=1）
sum_group_buy_cash_ledger() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_gbp_$1#%' AND wallet_kind=1;"
}

# sum_group_buy_points_ledger <gid>：该 gid 结算奖积分流水金额合计（wallet_kind=4）
sum_group_buy_points_ledger() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_gbp_$1#%' AND wallet_kind=4;"
}

# cc_pick_key <prefix>：找连击链车 1 预选幂等键——comboRandom("combo-winner",0)%3==0
# 且 comboRandom("global-target",1)%2==0（users 升序 → 连击大奖得主=最早注册用户、
# 槽 1 归 others[0]=第二早注册用户）
cc_pick_key() {
    local prefix=$1 n w t
    for ((n = 0; n < 2000; n++)); do
        w=$(gacha_hash_mod "${prefix}_${n}" "combo-winner" 0 3)
        t=$(gacha_hash_mod "${prefix}_${n}" "global-target" 1 2)
        if [ "$w" == "0" ] && [ "$t" == "0" ]; then
            echo "${prefix}_${n}"
            return 0
        fi
    done
    return 1
}

# gb_pick_key <prefix>：找拼团预选幂等键——groupBuyRandom("group-buy-lucky-pool",2)%3!=0
# 且 (...,1)%2==1（幸运池 Fisher-Yates 后 pool[0]=最早注册用户，幸运奖+幸运子奖归其所有）
gb_pick_key() {
    local prefix=$1 n h2 h1
    for ((n = 0; n < 2000; n++)); do
        h2=$(gacha_hash_mod "${prefix}_${n}" "group-buy-lucky-pool" 2 3)
        h1=$(gacha_hash_mod "${prefix}_${n}" "group-buy-lucky-pool" 1 2)
        if [ "$h2" != "0" ] && [ "$h1" == "1" ]; then
            echo "${prefix}_${n}"
            return 0
        fi
    done
    return 1
}

# ---------- M14 管理控制台（admin gateway 9321；kratos 形错误体 {code,message,reason}，成功体 protojson camelCase） ----------

ADMIN_URL="${ADMIN_URL:-http://127.0.0.1:9321/console/v2/api}"
ADMIN_RID=0

# admin_request <method> <path> <json|""> [token] [idempotencyKey]：管理网关请求（自动 X-Request-Id），
# 输出 "响应体\nHTTP状态"；JSON 走临时文件避免 Windows GBK 破坏 UTF-8；
# 第 5 参可携带 Idempotency-Key 头（M14b 草稿创建幂等）
admin_request() {
    local method=$1 path=$2 json=${3:-} token=${4:-} idem=${5:-} out="" tmp=""
    ADMIN_RID=$((ADMIN_RID + 1))
    local rid="e2e-rid-${ADMIN_RID}"
    local args=(-s -w '\n%{http_code}' -X "$method" -H "Content-Type: application/json" -H "X-Request-Id: ${rid}")
    [ -n "$token" ] && args+=(-H "Authorization: Bearer ${token}")
    [ -n "$idem" ] && args+=(-H "Idempotency-Key: ${idem}")
    if [ -n "$json" ]; then
        tmp=$(mktemp)
        printf '%s' "$json" > "$tmp"
        out=$(curl "${args[@]}" --data-binary "@${tmp}" "${ADMIN_URL}${path}")
        rm -f "$tmp"
    else
        out=$(curl "${args[@]}" "${ADMIN_URL}${path}")
    fi
    echo "$out"
}

# admin_status <out> / admin_body <out>：拆 HTTP 状态与响应体
admin_status() { echo "$1" | tail -1; }
admin_body() { echo "$1" | sed '$d'; }

# admin_request_rid <requestId> <method> <path> <json|""> [token] [idempotencyKey]：
# 同 admin_request 但指定 X-Request-Id（券模板发布同号重放/异内容冲突等场景 27 用）
admin_request_rid() {
    local rid=$1 method=$2 path=$3 json=${4:-} token=${5:-} idem=${6:-} out="" tmp=""
    local args=(-s -w '\n%{http_code}' -X "$method" -H "Content-Type: application/json" -H "X-Request-Id: ${rid}")
    [ -n "$token" ] && args+=(-H "Authorization: Bearer ${token}")
    [ -n "$idem" ] && args+=(-H "Idempotency-Key: ${idem}")
    if [ -n "$json" ]; then
        tmp=$(mktemp)
        printf '%s' "$json" > "$tmp"
        out=$(curl "${args[@]}" --data-binary "@${tmp}" "${ADMIN_URL}${path}")
        rm -f "$tmp"
    else
        out=$(curl "${args[@]}" "${ADMIN_URL}${path}")
    fi
    echo "$out"
}

# admin_totp <Base32Secret>：TOTP 6 位码（与 ops-rpc core/totp 同参）
admin_totp() {
    go run "${SCRIPT_DIR}/tools/totp/main.go" "$1"
}

# admin_login_begin <username> <password> <fingerprint>：登录第一步（无 ticket 时自动 TOTP setup），
# 输出 "loginTicket setupRequired totpSecret"（verify 流程 totpSecret 为空）
admin_login_begin() {
    local out
    out=$(admin_request POST /session/login "{\"tenantId\":\"1\",\"username\":\"$1\",\"password\":\"$2\",\"clientFingerprint\":\"$3\"}")
    [ "$(admin_status "$out")" == "200" ] || { echo "管理登录第一步失败: $(admin_body "$out")" >&2; return 1; }
    local body
    body=$(admin_body "$out")
    echo "$(extract_field "$body" loginTicket) $(extract_field "$body" setupRequired) $(extract_field "$body" totpSecret)"
}

# admin_login_complete <username> <password> <fingerprint> <ticket> <otpCode>：登录第二步，输出 accessToken
admin_login_complete() {
    local out
    out=$(admin_request POST /session/login "{\"tenantId\":\"1\",\"username\":\"$1\",\"password\":\"$2\",\"clientFingerprint\":\"$3\",\"loginTicket\":\"$4\",\"otpCode\":\"$5\"}")
    [ "$(admin_status "$out")" == "200" ] || { echo "管理登录第二步失败: $(admin_body "$out")" >&2; return 1; }
    extract_field "$(admin_body "$out")" accessToken
}

# admin_login <username> <password> [secret]：一键两步登录（verify 流程需传已绑定 secret），输出 accessToken
admin_login() {
    local user=$1 pass=$2 secret=${3:-} line ticket req code
    line=$(admin_login_begin "$user" "$pass" "e2e-fp") || return 1
    ticket=$(echo "$line" | cut -d' ' -f1)
    req=$(echo "$line" | cut -d' ' -f2)
    if [ "$req" == "true" ]; then
        code=$(admin_totp "$(echo "$line" | cut -d' ' -f3)")
    else
        code=$(admin_totp "$secret")
    fi
    admin_login_complete "$user" "$pass" "e2e-fp" "$ticket" "$code"
}

# admin_roles <token>：角色列表（完整响应体）
admin_roles() { admin_body "$(admin_request GET '/authorization/roles?tenantId=1' '' "$1")"; }

# admin_role_create <token> <code> <name>：创建角色，输出 roleId
admin_role_create() {
    local out
    out=$(admin_request POST /authorization/roles "{\"tenantId\":\"1\",\"code\":\"$2\",\"name\":\"$3\"}" "$1")
    [ "$(admin_status "$out")" == "200" ] || { echo "创建角色失败: $(admin_body "$out")" >&2; return 1; }
    extract_field "$(admin_body "$out")" 'id'
}

# admin_open_account <token> <username> <password>：开通控制台账号，输出 "principalId roleCode"
admin_open_account() {
    local out
    out=$(admin_request POST /session/accounts "{\"tenantId\":\"1\",\"username\":\"$2\",\"password\":\"$3\",\"displayName\":\"E2E Op\"}" "$1")
    [ "$(admin_status "$out")" == "200" ] || { echo "开通账号失败: $(admin_body "$out")" >&2; return 1; }
    echo "$(extract_field "$(admin_body "$out")" principalId) $(extract_field "$(admin_body "$out")" roleCode)"
}

# promo_role_id：种子角色 promotion-admin 的 id（不硬编码——从零重建库时 auto-increment 布局与旧残留库不同）
promo_role_id() {
    do_mysql "SELECT id FROM gacha_ops.roles WHERE tenant_id=1 AND code='promotion-admin';"
}

# admin_bind_roles <token> <principalId> <expectedVersion> <roleIds...>：绑定主体角色（完整响应体）
admin_bind_roles() {
    local token=$1 pid=$2 ver=$3 body="" i
    body="{\"tenantId\":\"1\",\"expectedVersion\":\"${ver}\",\"roleIds\":["
    shift 3
    for i in "$@"; do body="${body}\"${i}\","; done
    body="${body%,}]}"
    admin_request PUT "/authorization/principals/${pid}/roles" "$body" "$token"
}

# admin_reset_totp <token> <principalId>：重置账号 TOTP（完整响应体）
admin_reset_totp() {
    admin_request POST "/session/accounts/$2/totp-reset" '{}' "$1"
}

# admin_logout <token>：登出（完整响应体）
admin_logout() { admin_request POST /session/logout '{}' "$1"; }

# ---------- 输出 ----------

print_title() {
    echo -e "${BLUE}============================================================${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}============================================================${NC}"
}

print_step() {
    echo -e "${GREEN}>>> $1${NC}"
}

print_warn() {
    echo -e "${YELLOW}$1${NC}"
}

# 打印错误并置断言失败标志（场景脚本结尾 exit $FAILED 时退出码为1）
print_error() {
    echo -e "${RED}$1${NC}"
    FAILED=1
}

# ---------- M11 campaign 活动域 ----------

# participate_campaign <token> <campaignKey> [code] [idemKey]：统一参与（签到/兑换码/转盘）
# 输出：完整响应 JSON（断言 code/streak/spinsLeft/prizes 用）
participate_campaign() {
    local token=$1 key=$2 code=${3:-} idem=${4:-}
    local json
    json="{\"campaignKey\":\"${key}\""
    [ -n "$code" ] && json="${json},\"code\":\"${code}\""
    json="${json}}"
    do_post /campaigns/participate "$json" "$token" "$idem"
}

# campaign_preview <token> <campaignKey> [code]：参与预览
campaign_preview() {
    local token=$1 key=$2 code=${3:-}
    local url="/campaigns/${key}/preview"
    [ -n "$code" ] && url="${url}?code=${code}"
    do_get "$url" "$token"
}

# list_entitlements <token>：我的活动宝箱资格
list_entitlements() {
    do_get /campaign-boxes "$1"
}

# open_campaign_box <token> <entitlementNo> [idemKey]：活动宝箱开箱（entitlement 全抵零付）
# 资格单号含 '#'（grant_no 格式 "pg_<key>#<idx>"）——路径段内 '#' 会被 curl 当片段剥掉，
# 必须先百分号编码再拼 URL
open_campaign_box() {
    local no
    no=$(printf '%s' "$2" | sed 's/#/%23/g')
    do_post "/campaign-boxes/${no}/open" '{}' "$1" "$3"
}

# get_entitlement_no <userId>：该用户最新一条资格单号（开箱场景单用户单资格）
get_entitlement_no() {
    do_mysql "SELECT entitlement_no FROM gacha_marketing.campaign_box_entitlements WHERE user_id=$1 ORDER BY id DESC LIMIT 1;"
}

# get_entitlement_status <entitlementNo>：资格状态（1 AVAILABLE 2 LOCKED 3 USED）
get_entitlement_status() {
    do_mysql "SELECT status FROM gacha_marketing.campaign_box_entitlements WHERE entitlement_no='$1';"
}

# wait_entitlement_status <entitlementNo> <expect> [maxWaitSec]：轮询资格至期望状态
wait_entitlement_status() {
    local no=$1 expect=$2 max=${3:-20} i=0 st
    while [ "$i" -lt "$max" ]; do
        st=$(get_entitlement_status "$no")
        [ "$st" == "$expect" ] && return 0
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}

# get_entitlement_locked_by <entitlementNo>：锁定方（quote_token；AVAILABLE 时空）
get_entitlement_locked_by() {
    do_mysql "SELECT IFNULL(locked_by,'') FROM gacha_marketing.campaign_box_entitlements WHERE entitlement_no='$1';"
}

# count_entitlements <userId> [status]：资格计数（不传状态=全部）
count_entitlements() {
    local uid=$1 st=${2:-}
    if [ -n "$st" ]; then
        do_mysql "SELECT COUNT(*) FROM gacha_marketing.campaign_box_entitlements WHERE user_id=${uid} AND status=${st};"
    else
        do_mysql "SELECT COUNT(*) FROM gacha_marketing.campaign_box_entitlements WHERE user_id=${uid};"
    fi
}

# ---------- MySQL 直查（campaign 状态断言） ----------

# get_signin_state <userId> [campaignId=1]：输出 "streak total_days last_signed_on"
get_signin_state() {
    local uid=$1 cid=${2:-1}
    do_mysql "SELECT CONCAT(streak,' ',total_days,' ',IFNULL(last_signed_on,'NULL')) FROM gacha_marketing.campaign_sign_ins WHERE campaign_id=${cid} AND user_id=${uid};"
}

# set_signin_last_date <userId> <日期'YYYY-MM-DD'> [campaignId=1]：回拨/改写签到日期
set_signin_last_date() {
    local uid=$1 date=$2 cid=${3:-1}
    do_mysql "UPDATE gacha_marketing.campaign_sign_ins SET last_signed_on='${date}' WHERE campaign_id=${cid} AND user_id=${uid};"
}

# get_participation_count <idemKey>：参与幂等行计数
get_participation_count() {
    do_mysql "SELECT COUNT(*) FROM gacha_marketing.campaign_participations WHERE idem_key='$1';"
}

# count_prize_grants <idemKey>：某参与下的发放流水数（全部状态）
count_prize_grants() {
    do_mysql "SELECT COUNT(*) FROM gacha_marketing.campaign_prize_grants WHERE idem_key='$1';"
}

# count_grants_by_status <idemKey> <status>：按状态数发放流水
count_grants_by_status() {
    do_mysql "SELECT COUNT(*) FROM gacha_marketing.campaign_prize_grants WHERE idem_key='$1' AND status=$2;"
}

# get_loyalty_points <userId>：忠诚点余额（积分，wallet kind=4；available 单位=积分；无账户行→0）
get_loyalty_points() {
    do_mysql "SELECT IFNULL((SELECT available FROM gacha_money.wallet_accounts WHERE user_id=$1 AND wallet_kind=4),0);"
}

# get_cash_fen <userId>：现金钱包余额（分）
get_cash_fen() {
    do_mysql "SELECT IFNULL(available,0) FROM gacha_money.wallet_accounts WHERE user_id=$1 AND wallet_kind=1;"
}

# count_user_coupons <userId> [templateNo]：用户持有券数（不传模板=全部）
count_user_coupons() {
    local uid=$1 tpl=${2:-}
    if [ -n "$tpl" ]; then
        do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE user_id=${uid} AND template_no='${tpl}';"
    else
        do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE user_id=${uid};"
    fi
}

# get_wheel_daily_count <userId> [campaignId=3]：今日转盘次数
get_wheel_daily_count() {
    local uid=$1 cid=${2:-3}
    do_mysql "SELECT IFNULL(spin_count,0) FROM gacha_marketing.campaign_wheel_daily WHERE campaign_id=${cid} AND user_id=${uid} AND spin_date=CURDATE();"
}

# get_wheel_slot_remaining <slotIndex> [campaignId=3]：槽位剩余（-1 不限）
get_wheel_slot_remaining() {
    local slot=$1 cid=${2:-3}
    do_mysql "SELECT remaining FROM gacha_marketing.campaign_wheel_slots WHERE campaign_id=${cid} AND slot_index=${slot};"
}

# set_wheel_slot_remaining <slotIndex> <值> [campaignId=3]：改写槽位剩余（-1 复原不限量）
set_wheel_slot_remaining() {
    local slot=$1 val=$2 cid=${3:-3}
    do_mysql "UPDATE gacha_marketing.campaign_wheel_slots SET remaining=${val} WHERE campaign_id=${cid} AND slot_index=${slot};"
}

# get_redeem_count <code>：兑换码已兑次数
get_redeem_count() {
    do_mysql "SELECT redeemed_count FROM gacha_marketing.campaign_redemption_codes WHERE code='$1';"
}

# set_redeem_count <code> <值>：改写兑换码已兑次数（超发测试/复原）
set_redeem_count() {
    do_mysql "UPDATE gacha_marketing.campaign_redemption_codes SET redeemed_count=$2 WHERE code='$1';"
}

# ---------- M15b 记忆翻牌 memory_match（gacha_play：memory_match_*） ----------

# mm_purchase <token> <configId> <idemKey> [paymentMethod] [facePresetId] [backPresetId]：购买牌局
# （SAGA 提交 + 网关轮询会话建立后同步返回），输出完整响应 JSON
mm_purchase() {
    local token=$1 config_id=$2 key=$3 method=${4:-WALLET} face=${5:-0} back=${6:-0}
    local json resp
    json="{\"configId\":${config_id},\"facePresetId\":${face},\"backPresetId\":${back},\"paymentMethod\":\"${method}\"}"
    resp=$(do_post /memory-match/purchase "$json" "$token" "$key")
    if [ "$(extract_code "$resp")" != "0" ]; then
        echo "记忆翻牌购买失败: ${resp}" >&2
        return 1
    fi
    echo "$resp"
}

# mm_buy_session <token> <configId> <idemKey>：购买并仅输出 sessionId
mm_buy_session() {
    local resp
    resp=$(mm_purchase "$1" "$2" "$3") || return 1
    echo "$(extract_field "$resp" sessionId)"
}

# get_mm_session <token> <sessionId>：会话进度，输出完整响应
get_mm_session() {
    do_get "/memory-match/sessions/$2" "$1"
}

# get_mm_round <token> <sessionId> <roundNo>：回合视图，输出完整响应
get_mm_round() {
    do_get "/memory-match/sessions/$2/rounds/$3" "$1"
}

# mm_select_destiny <token> <sessionId> <patternId> [second]：选命运/第二命运，输出完整响应
mm_select_destiny() {
    local token=$1 sid=$2 pattern=$3 second=${4:-0}
    if [ "$second" == "1" ]; then
        do_post "/memory-match/sessions/${sid}/second-destiny" "{\"patternId\":${pattern}}" "$token"
    else
        do_post "/memory-match/sessions/${sid}/destiny" "{\"patternId\":${pattern}}" "$token"
    fi
}

# mm_pair <token> <sessionId> <roundNo> <first> <second>：配对，输出完整响应
mm_pair() {
    do_post "/memory-match/sessions/$2/rounds/$3/pair" "{\"firstCard\":$4,\"secondCard\":$5}" "$1"
}

# mm_destiny_draw <token> <sessionId> <roundNo>：命运抽牌，输出完整响应
mm_destiny_draw() {
    do_post "/memory-match/sessions/$2/rounds/$3/destiny-draw" '{}' "$1"
}

# mm_complete <token> <sessionId> <roundNo>：完成回合，输出完整响应
mm_complete() {
    do_post "/memory-match/sessions/$2/rounds/$3/complete" '{}' "$1"
}

# mm_skip <token> <sessionId>：跳过（AutoResolve 剩余回合 + 终局结算），输出完整响应
mm_skip() {
    do_post "/memory-match/sessions/$2/skip" '{}' "$1"
}

# mm_config <token> <configId>：玩法详情，输出完整响应
mm_config() {
    do_get "/memory-match/configs/$2" "$1"
}

# mm_appearance_get <token> <configId>：外观读取，输出完整响应
mm_appearance_get() {
    do_get "/memory-match/appearance?configId=$2" "$1"
}

# mm_appearance_put <token> <configId> <facePresetId> <backPresetId>：外观保存，输出完整响应
mm_appearance_put() {
    do_put "/memory-match/appearance" "{\"configId\":$2,\"facePresetId\":$3,\"backPresetId\":$4}" "$1"
}

# mm_session_field <sessionId> <字段>：直查会话行
mm_session_field() {
    do_mysql "SELECT $2 FROM gacha_play.memory_match_sessions WHERE id=$1;"
}

# mm_round_field <sessionId> <roundNo> <字段>：直查回合行
mm_round_field() {
    do_mysql "SELECT $3 FROM gacha_play.memory_match_rounds WHERE session_id=$1 AND round_no=$2;"
}

# mm_settlement_field <sessionId> <字段>：直查结算行
mm_settlement_field() {
    do_mysql "SELECT $2 FROM gacha_play.memory_match_settlements WHERE session_id=$1;"
}

# mm_champion_field <configId> <字段>：直查擂主行（无行输出空）
mm_champion_field() {
    do_mysql "SELECT $2 FROM gacha_play.memory_match_champion WHERE config_id=$1;"
}

# mm_count_champion_events <sessionId>：数该会话擂主事件
mm_count_champion_events() {
    do_mysql "SELECT COUNT(*) FROM gacha_play.memory_match_champion_events WHERE session_id=$1;"
}

# mm_count_champion_payouts <configId>：数该配置 PENDING/PAID 打款行
mm_count_champion_payouts() {
    do_mysql "SELECT COUNT(*) FROM gacha_play.memory_match_champion_payouts WHERE config_id=$1;"
}

# mm_payout_status <sessionId>：该会话（换擂会话）产生打款的 status（1 PENDING 2 PAID）
mm_payout_status() {
    do_mysql "SELECT status FROM gacha_play.memory_match_champion_payouts WHERE grant_no='mm-champion:$1';"
}

# mm_payout_user <sessionId>：该换擂会话打款的收款用户
mm_payout_user() {
    do_mysql "SELECT user_id FROM gacha_play.memory_match_champion_payouts WHERE grant_no='mm-champion:$1';"
}

# mm_entry_remaining <configId> <totalPairCount>：奖档剩余
mm_entry_remaining() {
    do_mysql "SELECT remaining FROM gacha_play.memory_match_entries WHERE config_id=$1 AND total_pair_count=$2;"
}

# mm_entry_prize <configId> <totalPairCount>：奖档 prize_kind|cash_fen|points_amount（竖线分隔）
mm_entry_prize() {
    do_mysql "SELECT CONCAT(prize_kind,'|',cash_fen,'|',points_amount) FROM gacha_play.memory_match_entries WHERE config_id=$1 AND total_pair_count=$2;"
}

# mm_count_settlement_ledgers <sessionId>：结算奖钱包流水数（log_no="c_mmr:<sessionId>"）
mm_count_settlement_ledgers() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_ledgers WHERE log_no='c_mmr:$1';"
}

# mm_sum_settlement_cash <sessionId>：结算奖现金流水合计（分，wallet_kind=1）
mm_sum_settlement_cash() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no='c_mmr:$1' AND wallet_kind=1;"
}

# mm_sum_settlement_points <sessionId>：结算奖积分流水合计（wallet_kind=4）
mm_sum_settlement_points() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no='c_mmr:$1' AND wallet_kind=4;"
}

# mm_count_settlement_coupons <sessionId>：结算奖券数（coupon_no="uc_mmr:<sessionId>"）
mm_count_settlement_coupons() {
    do_mysql "SELECT COUNT(*) FROM gacha_marketing.user_coupons WHERE coupon_no='uc_mmr:$1';"
}

# mm_count_champion_ledgers <sessionId>：换擂打款钱包流水数（log_no="c_mm-champion:<sessionId>"）
mm_count_champion_ledgers() {
    do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_ledgers WHERE log_no='c_mm-champion:$1';"
}

# mm_sum_champion_cash <sessionId>：换擂打款现金流水合计（分）
mm_sum_champion_cash() {
    do_mysql "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE log_no='c_mm-champion:$1' AND wallet_kind=1;"
}

# mm_insert_entitlement <configId> <userId> <presetId>：插入外观权益（场景 SQL 直插，D8）
mm_insert_entitlement() {
    do_mysql "INSERT INTO gacha_play.memory_match_appearance_entitlements (config_id, user_id, preset_id) VALUES ($1, $2, $3);"
}

# mm_count_entitlements <configId> <userId>：数该用户外观权益
mm_count_entitlements() {
    do_mysql "SELECT COUNT(*) FROM gacha_play.memory_match_appearance_entitlements WHERE config_id=$1 AND user_id=$2;"
}

# reset_memory_match <configId>：记忆翻牌现场复位（清会话/回合/结算/擂主事件/打款/外观选择+权益、
# 删除擂主行、奖档库存=quantity）
reset_memory_match() {
    do_mysql "DELETE FROM gacha_play.memory_match_sessions WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.memory_match_rounds WHERE session_id NOT IN (SELECT id FROM gacha_play.memory_match_sessions);"
    do_mysql "DELETE FROM gacha_play.memory_match_settlements WHERE session_id NOT IN (SELECT id FROM gacha_play.memory_match_sessions);"
    do_mysql "DELETE FROM gacha_play.memory_match_champion_events WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.memory_match_champion_payouts WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.memory_match_champion WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.memory_match_appearance_selections WHERE config_id=$1;"
    do_mysql "DELETE FROM gacha_play.memory_match_appearance_entitlements WHERE config_id=$1;"
    do_mysql "UPDATE gacha_play.memory_match_entries SET remaining=quantity WHERE config_id=$1;"
}

# mm_set_champion <configId> <userId> <totalPairCount>：SQL 锻造现任擂主（场景用；pending 清零）
mm_set_champion() {
    do_mysql "INSERT INTO gacha_play.memory_match_champion (config_id, user_id, total_pair_count, pending_income_minor, version) VALUES ($1, $2, $3, 0, 1) ON DUPLICATE KEY UPDATE user_id=$2, total_pair_count=$3, pending_income_minor=0;"
}

# mm_hand_pattern_counts <handCardIdsJson>：从回合视图 handCardIds JSON 输出 "pattern:count" 行（按数量降序）
# （卡 1..54 → 图案 = (卡-1)/6+1；命运=手牌最频图案可最大化命运资源）
mm_hand_pattern_counts() {
    echo "$1" | tr ',' '\n' | tr -d '[]' | sed 's/^ *//' | while read -r card; do
        [ -z "$card" ] && continue
        echo $(( (card - 1) / 6 + 1 ))
    done | sort -n | uniq -c | sort -rn | awk '{print $2":"$1}'
}

# mm_most_common_pattern <handCardIdsJson>：手牌最频图案（命运推荐；并列取小图案号）
mm_most_common_pattern() {
    mm_hand_pattern_counts "$1" | head -1 | cut -d: -f1
}

# mm_pair_of_hand <handCardIdsJson>：手牌首个同图案对子（引擎 FirstPair 同序），输出 "first second"；无对输出空
mm_pair_of_hand() {
    local json=$1 p cards first second
    for p in 1 2 3 4 5 6 7 8 9; do
        first=""
        cards=$(echo "$json" | tr ',' '\n' | tr -d '[]' | sed 's/^ *//' | while read -r card; do
            [ -z "$card" ] && continue
            if [ $(( (card - 1) / 6 + 1 )) == "$p" ]; then echo "$card"; fi
        done)
        first=$(echo "$cards" | head -1)
        second=$(echo "$cards" | sed -n '2p')
        if [ -n "$first" ] && [ -n "$second" ]; then
            echo "$first $second"
            return 0
        fi
    done
    return 0
}

# ---------- M15c task 玩家任务体系（gacha_task：task_* 8 表 + gacha_ops 计划快照） ----------

# task_tables_reset：清空 8 张玩家执行面表（场景 31 复位；计划快照另由 task_plan_seed_v2/reset 管）
task_tables_reset() {
    do_mysql "DELETE FROM gacha_task.task_progress; DELETE FROM gacha_task.task_event; DELETE FROM gacha_task.task_claim;"
    do_mysql "DELETE FROM gacha_task.task_sign_in; DELETE FROM gacha_task.task_pass_progress; DELETE FROM gacha_task.task_pass_claim;"
    do_mysql "DELETE FROM gacha_task.task_member; DELETE FROM gacha_task.task_member_claim;"
}

# task_plan_seed_v2：gacha_ops 计划快照切到 M15c v2 测试计划（head=2；config_hash/blob
# 由 .smoke_risk/hashgen 生成，与 task-rpc 快照校验 ConfigHashOf 同源）
task_plan_seed_v2() {
    do_mysql "DELETE FROM gacha_ops.task_plan_publication_approval; DELETE FROM gacha_ops.task_plan_version;"
    do_mysql "INSERT INTO gacha_ops.task_plan_version (version, status, config_hash, config_blob, created_at, published_at)
              VALUES (2, 2, UNHEX('f1c77a7681b03254d72f358497e1de87ee6ab60799b1950b82b9f8d1de0288fe'),
                      UNHEX('1a22080110021a0ce7b4afe8aea1e6b688e8b4b92002280230644206542d4341534848011a23080210021a0ce6af8fe697a5e7adbee588b02002280530024207542d504f494e5448011a22080310021a0ce68abde58da1e6aca1e695b02002280130014206542d434f494e48011a21080410021a09e9a696e58585e7a4bc20012804306438014206542d434f494e48012246080110021a14e9809ae8a18ce8af81c2b7e8b59be5ada3e4b88020642801320f080110011a07542d504f494e5420643211080110021a09542d4649582d3530302001380140012a0f080110f4031a06542d5649503120012a0f080210e8071a06542d564950322001'), NOW(3), NOW(3));"
    do_mysql "UPDATE gacha_ops.task_plan_head SET current_version=2, latest_version=2, updated_at=NOW(3) WHERE singleton_id=1;"
}

# task_plan_reset：gacha_ops 计划快照复位回 M14e bootstrap v1（head=1）
task_plan_reset() {
    do_mysql "DELETE FROM gacha_ops.task_plan_publication_approval; DELETE FROM gacha_ops.task_plan_version;"
    do_mysql "INSERT INTO gacha_ops.task_plan_version (version, status, config_hash, config_blob, created_at, published_at)
              VALUES (1, 2, UNHEX('c4ea9a722bcccc403081206cf2e0ad6c9525f8452f2891da3d6bbecaa4d7fe06'),
                      UNHEX('1a22080110011a0ce6af8fe697a5e799bbe5bd952002280330014206542d424f4f544801'), NOW(3), NOW(3));"
    do_mysql "UPDATE gacha_ops.task_plan_head SET current_version=1, latest_version=1, updated_at=NOW(3) WHERE singleton_id=1;"
}

# build_taskcli：编译 grpc 直连工具到 /tmp/taskcli_m15c.exe（已存在直接复用）
build_taskcli() {
    local exe=/tmp/taskcli_m15c.exe
    [ -x "$exe" ] && return 0
    (cd "${SCRIPT_DIR}/.." && go build -o "$exe" test/tools/taskcli/main.go) || return 1
}

# apply_task_progress <userId> <eventId> <metric> <amount>：进度事件（ApplyTaskProgress，
# 无 HTTP 路由的内部 RPC，D4；eventId 幂等），成功输出 OK；业务错误输出 code=XXXX 行返回 1
# metric: 1 DRAW_COUNT / 2 DRAW_SPEND_MINOR / 3 LOGIN / 4 RECHARGE_MINOR
apply_task_progress() {
    build_taskcli || return 2
    /tmp/taskcli_m15c.exe apply "$1" "$2" "$3" "$4"
}

# unlock_pass <userId> <passId>：付费线通行证解锁 premium（内部 RPC），成功输出 OK
unlock_pass() {
    build_taskcli || return 2
    /tmp/taskcli_m15c.exe unlock-pass "$1" "$2"
}

# task_progress_field <userId> <taskId> <字段>：直查任务进度行
task_progress_field() {
    do_mysql "SELECT $3 FROM gacha_task.task_progress WHERE user_id=$1 AND task_id=$2;"
}

# task_signin_field <userId> <taskId> <字段>：直查签到行（last_date/streak）
task_signin_field() {
    do_mysql "SELECT $3 FROM gacha_task.task_sign_in WHERE user_id=$1 AND task_id=$2;"
}

# task_pass_field <userId> <passId> <字段>：直查通行证进度行（xp/premium）
task_pass_field() {
    do_mysql "SELECT $3 FROM gacha_task.task_pass_progress WHERE user_id=$1 AND pass_id=$2;"
}

# task_member_field <userId> <字段>：直查会员行（spend_minor/level）
task_member_field() {
    do_mysql "SELECT $2 FROM gacha_task.task_member WHERE user_id=$1;"
}
