#!/bin/bash
# ============================================================
# 场景12：券释放（coupon_release，M10 分支03 补偿 ReleaseQuote）
#
# 流程：注册 → 充值100元 → 领 T-FIX-500 券 → 预热 box3 缓存后数据库清零库存
#       → 带券买 box3（预检走缓存放行）→ SettleDraw 实检售罄 Aborted(2002)
#       → 补偿链：券回 AVAILABLE（locked_by 清空）、报价 RELEASED(4)、
#         支付单 CANCELED(6)、余额回 100.00、无抽赏记录
#       → 同券再买 box2 → 成功：记录 price=5.00、券 USED(3)、报价 USED(3)、余额 95.00
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景12：售罄补偿释放券（coupon_release）"

# ---------- 0. 自复位库存（场景可独立重复执行） ----------
print_step "0. 自复位奖池库存（reset_test_data.sql）"
"${MYSQL_BIN}" --connect-timeout=5 -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" -p"${DB_PASS}" \
    --default-character-set=utf8mb4 gacha_draw < "${SCRIPT_DIR}/../script/sql/reset_test_data.sql" \
    || { print_error "复位 SQL 执行失败"; exit 1; }

# ---------- 1. 注册 + 充值 100 元 + 领券 ----------
print_step "1. 注册用户、充值 100 元、领取 T-FIX-500 券"
reg=$(register_user "e2e_release_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge '{"amount":"100"}' "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
coupon_no=$(claim_coupon "$token" "T-FIX-500") || exit 1
coupon_id=$(get_user_coupon_id "$user_id" "T-FIX-500")
[ -n "$coupon_id" ] || { print_error "未查到领取的券"; exit $FAILED; }
[ "$(get_coupon_status "$coupon_id")" == "1" ] || print_error "新领券应为 AVAILABLE(1)"
print_step "couponNo: ${coupon_no}, id: ${coupon_id}"

# ---------- 2. 预热 box3 缓存后清零库存（预检放行，SAGA 实检拦截） ----------
print_step "2. 预热 box3 缓存视图后数据库清零中奖条目"
flush_box_entries_cache 3
detail=$(do_get /boxes/3)
[ "$(extract_code "$detail")" == "0" ] || { print_error "box3 详情查询失败: ${detail}"; exit 1; }
do_mysql "UPDATE gacha_draw.draw_pool_entries SET remaining = 0 WHERE box_id = 3 AND is_prize = 1;" \
    || { print_error "清零库存失败"; exit 1; }
remaining=$(get_entry_remaining 3 '传说坐骑')
[ "$remaining" == "0" ] || print_error "清零后库存应为 0，实际: ${remaining}"

# ---------- 3. 带券购买 box3（预检走缓存放行 → 05 实检 Aborted） ----------
print_step "3. 带券购买 box3（实付 95.00，预检放行）"
rid1=$(buy_box "$token" 3 "" "$coupon_id") || exit 1
[ -n "$rid1" ] || { print_error "requestId 不应为空"; exit $FAILED; }
print_step "requestId: ${rid1}（SettleDraw 实检将 Aborted，触发补偿链）"

# ---------- 4. 补偿链断言：券回 AVAILABLE / 报价 RELEASED / 支付 CANCELED / 余额退回 ----------
print_step "4. 等待补偿：券释放、报价释放、撤单解冻、余额退回"
balance=$(wait_balance "$token" "100.00" 20)
[ -n "$balance" ] || print_error "退款超时：余额应为 100.00，实际: $(get_balance "$token")"
wait_coupon_status "$coupon_id" 1 20 || print_error "券未在 20s 内回到 AVAILABLE(1)，实际: $(get_coupon_status "$coupon_id")"
locked_by=$(get_coupon_locked_by "$coupon_id")
[ -z "$locked_by" ] || print_error "释放后 locked_by 应为空，实际: ${locked_by}"
wait_quote_status "$rid1" 4 20 || print_error "报价锁未在 20s 内到达 RELEASED(4)，实际: $(get_quote_status "$rid1")"
wait_payment_status "$rid1" 6 20 || print_error "支付单未在 20s 内到达 CANCELED(6)"
pstatus=$(get_payment_status "$rid1")
[ "$pstatus" == "6" ] || print_error "payment_orders.status 应为 6(CANCELED)，实际: ${pstatus}"
leg_status=$(get_payment_leg_status "$rid1" 1)
[ "$leg_status" == "3" ] || print_error "payment_legs CASH 腿应为 3(VOIDED)，实际: ${leg_status}"
hold_status=$(get_payment_hold_status "$rid1" 1)
[ "$hold_status" == "3" ] || print_error "wallet_holds 应为 3(VOIDED)，实际: ${hold_status}"
rcount=$(count_records "$rid1")
[ "$rcount" == "0" ] || print_error "售罄不应产生抽赏记录，实际: ${rcount}"
print_step "补偿链断言通过：券 AVAILABLE / 报价 RELEASED / 支付 CANCELED / 余额 100.00 / 无记录"

# ---------- 5. 同券再买 box2（券可复用） ----------
print_step "5. 同券再买 box2（实付 5.00）"
rid2=$(buy_box "$token" 2 "" "$coupon_id") || exit 1
[ -n "$rid2" ] || { print_error "requestId 不应为空"; exit $FAILED; }
print_step "requestId: ${rid2}"
record=$(poll_record "$token" "$rid2" 20)
[ -n "$record" ] || { print_error "轮询超时：再买开奖未完成"; exit $FAILED; }
price=$(extract_field "$record" price)
[ "$price" == "5.00" ] || print_error "记录 price 应为实付 5.00，实际: ${price}"
wait_payment_paid "$rid2" 20 || { print_error "支付单未在 20s 内到达 PAID"; exit $FAILED; }
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "9500" ] || print_error "再买后余额应为 95.00，实际: ${balance}"
wait_coupon_status "$coupon_id" 3 20 || print_error "复用后券应为 USED(3)，实际: $(get_coupon_status "$coupon_id")"
qstat=$(get_quote_status "$rid2")
[ "$qstat" == "3" ] || print_error "再买报价锁应为 USED(3)，实际: ${qstat}"
print_step "券复用断言通过：price=5.00, 余额 95.00, 券 USED, 报价 USED"

# ---------- 6. 恢复现场（box3 库存复位 + 缓存刷新） ----------
print_step "6. 恢复现场：box3 库存复位 + 缓存刷新"
"${MYSQL_BIN}" --connect-timeout=5 -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" -p"${DB_PASS}" \
    --default-character-set=utf8mb4 gacha_draw < "${SCRIPT_DIR}/../script/sql/reset_test_data.sql" \
    || { print_error "复位 SQL 执行失败"; exit 1; }
flush_box_entries_cache 3
print_step "现场已恢复"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景12 coupon_release 通过${NC}"
else
    echo -e "${RED}× 场景12 coupon_release 失败${NC}"
fi
exit $FAILED
