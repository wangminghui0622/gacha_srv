#!/bin/bash
# ============================================================
# 场景4：幂等（idempotent）——同一 Idempotency-Key 只生效一次
# M10 起带 5 折券（T-PCT-5000）复跑：券/报价状态机不破坏幂等。
#
# 流程：注册 → 充值100元 → 领 5 折券 → 带 Key 带券买 box2 → 轮询到记录
#       → 同一 Key 带同券再买
# 断言：
#   1. 报价：box2(10.00) + 5 折券 → 券抵扣 5.00、实付 5.00
#   2. 两次购买返回同一个 requestId（Key → gid 复用）
#   3. 第二次响应 status=success（Submit 失败回查已有记录，幂等返回）
#   4. 余额只扣一次：100.00 → 95.00（再买后仍 95.00）
#   5. draw_records 恰 1 条；记录列表 total=1；记录 price=实付 5.00
#   6. M8 支付域：支付单恰 1 条且 PAID、CASH 腿 POSTED、hold POSTED
#   7. M10 玩法域：券 USED、报价锁 USED（恰 1 条，未被重复锁/核销）
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景4：同 Key 幂等（idempotent，带 5 折券）"

# ---------- 1. 注册 + 充值 + 领券 ----------
print_step "1. 注册用户、充值 100 元、领取 T-PCT-5000 券"
reg=$(register_user "e2e_idem_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge "{\"amount\":\"100\"}" "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
coupon_no=$(claim_coupon "$token" "T-PCT-5000") || exit 1
coupon_id=$(get_user_coupon_id "$user_id" "T-PCT-5000")
[ -n "$coupon_id" ] || { print_error "未查到领取的券"; exit $FAILED; }

# ---------- 1.5 报价断言（5 折券：box2 10.00 → 减 5.00 → 实付 5.00） ----------
print_step "1.5 报价断言：box2(10.00) + 5 折券"
q=$(get_quote "$token" 2 "$coupon_id")
[ "$(extract_code "$q")" == "0" ] || { print_error "报价失败: ${q}"; exit $FAILED; }
[ "$(extract_field "$q" couponDiscount)" == "5.00" ] || print_error "券抵扣应为 5.00，实际: $(extract_field "$q" couponDiscount)"
[ "$(extract_field "$q" payable)" == "5.00" ] || print_error "实付应为 5.00，实际: $(extract_field "$q" payable)"

# ---------- 2. 第一次购买（带 Idempotency-Key + 券） ----------
key="e2e-idem-$(date +%s)"
print_step "2. 第一次购买 box2（带券），Idempotency-Key: ${key}"
buy1=$(do_post /box/buy "{\"boxId\":2,\"userCouponId\":${coupon_id}}" "$token" "$key")
[ "$(extract_code "$buy1")" == "0" ] || print_error "首次购买失败: ${buy1}"
rid1=$(extract_field "$buy1" requestId)
print_step "requestId: ${rid1}"

# ---------- 3. 轮询到记录（确认第一次已成功） ----------
print_step "3. 轮询第一次抽赏记录"
record=$(poll_record "$token" "$rid1" 20)
[ -n "$record" ] || { print_error "轮询超时：首次开奖未完成"; exit $FAILED; }
price=$(extract_field "$record" price)
[ "$price" == "5.00" ] || print_error "记录 price 应为实付 5.00，实际: ${price}"
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "9500" ] || print_error "首次扣款后余额应为 95.00，实际: ${balance}"
print_step "首次开奖完成，余额: ${balance}"

# ---------- 4. 同一 Key 再次购买（同券参数） ----------
print_step "4. 同一 Key 带同券再次购买（应幂等返回同一单号）"
buy2=$(do_post /box/buy "{\"boxId\":2,\"userCouponId\":${coupon_id}}" "$token" "$key")
[ "$(extract_code "$buy2")" == "0" ] || print_error "二次购买失败: ${buy2}"
rid2=$(extract_field "$buy2" requestId)
status2=$(extract_field "$buy2" status)
[ "$rid2" == "$rid1" ] || print_error "二次 requestId 应相同: ${rid1} ≠ ${rid2}"
[ "$status2" == "success" ] || print_error "二次 status 应为 success，实际: ${status2}"

# ---------- 5. 余额只扣一次 ----------
print_step "5. 校验余额仍为 95.00（只扣一次）"
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "9500" ] || print_error "余额应仍为 95.00，实际: ${balance}"

# ---------- 6. 数据库断言 ----------
print_step "6. 数据库断言：draw_records 恰 1 条、列表 total=1"
rec_count=$(count_records "$rid1")
[ "$rec_count" == "1" ] || print_error "draw_records 应有 1 条，实际: ${rec_count}"
list_resp=$(do_get "/draw/records?page=1&pageSize=10" "$token")
[ "$(extract_code "$list_resp")" == "0" ] || print_error "记录列表查询失败: ${list_resp}"
total=$(extract_field "$list_resp" total)
[ "$total" == "1" ] || print_error "记录列表 total 应为 1，实际: ${total}"

# ---------- 7. 支付域断言（M8） ----------
print_step "7. 支付域断言：支付单恰 1 条 PAID、腿/hold POSTED"
wait_payment_paid "$rid1" 20 || print_error "支付单未在 20s 内到达 PAID"
pstatus=$(get_payment_status "$rid1")
[ "$pstatus" == "3" ] || print_error "payment_orders.status 应为 3(PAID)，实际: ${pstatus}"
leg_status=$(get_payment_leg_status "$rid1" 1)
[ "$leg_status" == "2" ] || print_error "payment_legs CASH 腿应为 2(POSTED)，实际: ${leg_status}"
hold_status=$(get_payment_hold_status "$rid1" 1)
[ "$hold_status" == "2" ] || print_error "wallet_holds 应为 2(POSTED)，实际: ${hold_status}"
pcount=$(count_payment_orders "$rid1")
[ "$pcount" == "1" ] || print_error "payment_orders 应恰 1 条（幂等），实际: ${pcount}"
print_step "支付域断言通过"

# ---------- 8. 玩法域断言（M10） ----------
print_step "8. 玩法域断言：券 USED、报价锁恰 1 条 USED"
wait_coupon_status "$coupon_id" 3 20 || print_error "券应为 USED(3)，实际: $(get_coupon_status "$coupon_id")"
qstat=$(get_quote_status "$rid1")
[ "$qstat" == "3" ] || print_error "报价锁应为 USED(3)，实际: ${qstat}"
qcount=$(do_mysql "SELECT COUNT(*) FROM gacha_marketing.quote_locks WHERE request_id='${rid1}';")
[ "$qcount" == "1" ] || print_error "报价锁应恰 1 条（幂等），实际: ${qcount}"
print_step "玩法域断言通过"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景4 idempotent 通过${NC}"
else
    echo -e "${RED}× 场景4 idempotent 失败${NC}"
fi
exit $FAILED
