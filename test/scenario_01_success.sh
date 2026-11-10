#!/bin/bash
# ============================================================
# 场景1：正常购买开奖（success）
#
# 流程：注册 → 充值100元 → 买新手体验盒(1.00元) → 轮询抽赏记录
# 断言：
#   1. 购买响应 code=0 且 status=pending，requestId 非空
#   2. 轮询出记录：code=0、boxId=1、price=1.00
#   3. 余额 100.00 → 99.00（恰好扣一次）
#   4. draw_records 恰 1 条（request_id 唯一）且 status=1 正常
#   5. 中奖(isPrize=true)时吉仓必有该物品；谢谢参与只记录不入仓
#   6. M8 支付域：支付单 PAID(3)、CASH 腿 POSTED(2)、hold POSTED(2)、单恰 1 条
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景1：正常购买开奖（success）"

# ---------- 1. 注册 + 充值 ----------
print_step "1. 注册用户并充值 100 元"
reg=$(register_user "e2e_success_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge "{\"amount\":\"100\"}" "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
balance=$(extract_field "$resp" balance)
[ "$(fen_of "$balance")" == "10000" ] || print_error "充值后余额应为 100.00，实际: ${balance}"
print_step "充值后余额: ${balance}"

# ---------- 2. 购买开奖 ----------
print_step "2. 购买新手体验盒（boxId=1，价格 1.00 元）"
buy_resp=$(do_post /box/buy '{"boxId":1}' "$token")
[ "$(extract_code "$buy_resp")" == "0" ] || print_error "购买失败: ${buy_resp}"
rid=$(extract_field "$buy_resp" requestId)
status=$(extract_field "$buy_resp" status)
[ -n "$rid" ] || print_error "requestId 不应为空"
[ "$status" == "pending" ] || print_error "首次购买 status 应为 pending，实际: ${status}"
print_step "requestId: ${rid}，status: ${status}"

# ---------- 3. 轮询抽赏记录 ----------
print_step "3. 轮询抽赏记录（SAGA 异步开奖）"
record=$(poll_record "$token" "$rid" 20)
if [ -z "$record" ]; then
    print_error "轮询超时：20s 内未出现抽赏记录（SAGA 可能失败，查 DTM 日志）"
    exit $FAILED
fi
entry_name=$(extract_field "$record" entryName)
is_prize=$(extract_field "$record" isPrize)
box_id=$(extract_field "$record" boxId)
price=$(extract_field "$record" price)
rec_status=$(extract_field "$record" status)
print_step "开奖结果: ${entry_name}，isPrize: ${isPrize}"

[ "$box_id" == "1" ] || print_error "记录 boxId 应为 1，实际: ${box_id}"
[ "$price" == "1.00" ] || print_error "记录 price 应为 1.00，实际: ${price}"
[ "$rec_status" == "1" ] || print_error "记录 status 应为 1 正常，实际: ${rec_status}"

# ---------- 4. 余额校验：恰好扣一次 ----------
print_step "4. 校验余额 100.00 → 99.00"
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "9900" ] || print_error "扣款后余额应为 99.00，实际: ${balance}"

# ---------- 5. 数据库断言 ----------
print_step "5. 数据库断言：draw_records 恰 1 条"
rec_count=$(count_records "$rid")
[ "$rec_count" == "1" ] || print_error "draw_records 应有 1 条，实际: ${rec_count}"

# ---------- 5.5 支付域断言（M8） ----------
print_step "5.5 支付域断言：支付单 PAID / 腿 POSTED / hold POSTED"
wait_payment_paid "$rid" 20 || print_error "支付单未在 20s 内到达 PAID"
pstatus=$(get_payment_status "$rid")
[ "$pstatus" == "3" ] || print_error "payment_orders.status 应为 3(PAID)，实际: ${pstatus}"
leg_status=$(get_payment_leg_status "$rid" 1)
[ "$leg_status" == "2" ] || print_error "payment_legs CASH 腿应为 2(POSTED)，实际: ${leg_status}"
hold_status=$(get_payment_hold_status "$rid" 1)
[ "$hold_status" == "2" ] || print_error "wallet_holds 应为 2(POSTED)，实际: ${hold_status}"
pcount=$(count_payment_orders "$rid")
[ "$pcount" == "1" ] || print_error "payment_orders 应恰 1 条，实际: ${pcount}"
print_step "支付域断言通过"

# ---------- 6. 中奖则吉仓必有货 ----------
print_step "6. 吉仓校验"
if [ "$is_prize" == "true" ]; then
    wh_resp=$(do_get /warehouse "$token")
    [ "$(extract_code "$wh_resp")" == "0" ] || print_error "吉仓查询失败: ${wh_resp}"
    if echo "$wh_resp" | grep -q "\"itemName\":\"${entry_name}\""; then
        print_step "吉仓已入物品: ${entry_name}"
    else
        print_error "中奖但吉仓缺少物品: ${entry_name}"
    fi
else
    wh_resp=$(do_get /warehouse "$token")
    if echo "$wh_resp" | grep -q "\"itemName\":\"${entry_name}\""; then
        print_error "谢谢参与不应入吉仓，但吉仓出现了: ${entry_name}"
    else
        print_step "谢谢参与只记录、未入吉仓 ✓"
    fi
fi

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景1 success 通过${NC}"
else
    echo -e "${RED}× 场景1 success 失败${NC}"
fi
exit $FAILED
