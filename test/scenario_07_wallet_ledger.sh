#!/bin/bash
# ============================================================
# 场景7：钱包流水断言（wallet_ledger，M7 钱包 + M8 支付域）
#
# 流程：注册 → 充值100元 → 同 Idempotency-Key 充值5元×2（幂等重放）
#       → 购买(1.00元) → 轮询记录
# 断言：
#   1. 充值后 wallet_accounts(CASH) available=10000、held=0
#   2. wallet_ledgers 恰 1 条 recharge 流水
#   3. 幂等重放：同 key 充值两次，余额只加 5 元、流水仍 1 条（uk_log_no）
#   4. 购买后 available=10400、held=0（M8 支付 SAGA：冻结→核销，held 归 0）
#   5. freeze 流水 log_no="h_"+legNo（legNo="P"+requestId+"_1"）、amount=-100、status=success
#   6. 支付单 PAID(3)、资金腿 POSTED(2)、hold POSTED(2)（M8 支付域断言）
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景7：钱包流水断言（wallet_ledger）"

# ---------- 1. 注册 ----------
print_step "1. 注册用户"
reg=$(register_user "e2e_wallet_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1

# ---------- 2. 充值 100 元 + 钱包/流水断言 ----------
print_step "2. 充值 100 元 → 钱包/流水断言"
resp=$(do_post /user/recharge '{"amount":"100"}' "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
balance=$(extract_field "$resp" balance)
[ "$(fen_of "$balance")" == "10000" ] || print_error "充值后余额应为 100.00，实际: ${balance}"

avail=$(get_wallet_available "$user_id" 1)
[ "$avail" == "10000" ] || print_error "wallet_accounts CASH available 应为 10000，实际: ${avail}"
held=$(get_wallet_held "$user_id" 1)
[ "$held" == "0" ] || print_error "wallet_accounts CASH held 应为 0，实际: ${held}"
rc=$(count_wallet_ledgers "user_id=${user_id} AND biz_type='recharge'")
[ "$rc" == "1" ] || print_error "recharge 流水应为 1 条，实际: ${rc}"
print_step "充值断言通过：available=10000, held=0, recharge 流水 1 条"

# ---------- 3. 幂等重放 ----------
print_step "3. 幂等重放：同 Idempotency-Key 充值 5 元两次"
idem="e2e_wallet_${user_id}_topup"
resp1=$(do_post /user/recharge '{"amount":"5"}' "$token" "$idem")
[ "$(extract_code "$resp1")" == "0" ] || print_error "第一次充值失败: ${resp1}"
resp2=$(do_post /user/recharge '{"amount":"5"}' "$token" "$idem")
[ "$(extract_code "$resp2")" == "0" ] || print_error "第二次充值（重放）失败: ${resp2}"
b1=$(extract_field "$resp1" balance)
b2=$(extract_field "$resp2" balance)
[ "$(fen_of "$b1")" == "10500" ] || print_error "第一次充值后余额应为 105.00，实际: ${b1}"
[ "$(fen_of "$b2")" == "10500" ] || print_error "重放后余额应仍为 105.00，实际: ${b2}"
lc=$(count_wallet_ledgers "user_id=${user_id} AND log_no='c_${idem}'")
[ "$lc" == "1" ] || print_error "同 key 充值流水应恰 1 条（uk_log_no 幂等），实际: ${lc}"
print_step "幂等重放通过：余额只加一次，流水恰 1 条"

# ---------- 4. 购买开奖 ----------
print_step "4. 购买新手体验盒（boxId=1，价格 1.00 元）"
buy_resp=$(do_post /box/buy '{"boxId":1}' "$token")
[ "$(extract_code "$buy_resp")" == "0" ] || print_error "购买失败: ${buy_resp}"
rid=$(extract_field "$buy_resp" requestId)
[ -n "$rid" ] || print_error "requestId 不应为空"
print_step "requestId: ${rid}"

record=$(poll_record "$token" "$rid" 20)
if [ -z "$record" ]; then
    print_error "轮询超时：20s 内未出现抽赏记录（SAGA 可能失败，查 DTM 日志）"
    exit $FAILED
fi
# 记录出现在 05 分支，07 捕获可能仍在途：等支付单 PAID 再断言（否则 held 未归零）
wait_payment_paid "$rid" 20 || { print_error "支付单未在 20s 内到达 PAID"; exit $FAILED; }
print_step "支付单已 PAID"

# ---------- 5. 扣款后钱包/流水断言 ----------
print_step "5. 扣款后钱包/流水断言"
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "10400" ] || print_error "扣款后余额应为 104.00，实际: ${balance}"
avail=$(get_wallet_available "$user_id" 1)
[ "$avail" == "10400" ] || print_error "wallet_accounts CASH available 应为 10400，实际: ${avail}"
held=$(get_wallet_held "$user_id" 1)
[ "$held" == "0" ] || print_error "M8 冻结-核销后 held 应为 0，实际: ${held}"
leg_no=$(payment_no "$rid")_1
amount=$(get_ledger_field "log_no='h_${leg_no}'" amount)
lstatus=$(get_ledger_field "log_no='h_${leg_no}'" status)
[ "$amount" == "-100" ] || print_error "freeze 流水 amount 应为 -100，实际: ${amount}"
[ "$lstatus" == "success" ] || print_error "freeze 流水 status 应为 success，实际: ${lstatus}"
print_step "扣款断言通过：available=10400, freeze 流水 -100/success"

# ---------- 6. 支付域断言 ----------
print_step "6. 支付域断言：支付单 PAID / 腿 POSTED / hold POSTED"
pstatus=$(get_payment_status "$rid")
[ "$pstatus" == "3" ] || print_error "payment_orders.status 应为 3(PAID)，实际: ${pstatus}"
leg_status=$(get_payment_leg_status "$rid" 1)
[ "$leg_status" == "2" ] || print_error "payment_legs CASH 腿应为 2(POSTED)，实际: ${leg_status}"
hold_status=$(get_payment_hold_status "$rid" 1)
[ "$hold_status" == "2" ] || print_error "wallet_holds 应为 2(POSTED)，实际: ${hold_status}"
pcount=$(count_payment_orders "$rid")
[ "$pcount" == "1" ] || print_error "payment_orders 应恰 1 条，实际: ${pcount}"
print_step "支付域断言通过：PAID / POSTED / 单恰 1 条"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景7 wallet_ledger 通过${NC}"
else
    echo -e "${RED}× 场景7 wallet_ledger 失败${NC}"
fi
exit $FAILED
