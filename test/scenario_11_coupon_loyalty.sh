#!/bin/bash
# ============================================================
# 场景11：券+积分抵扣（coupon_loyalty，M10 玩法域 分支03）
#
# 流程：注册 → 充值10元 → SQL 授信积分钱包 1000 分（含 credit 流水）
#       → 领 T-FIX-500 券 → 报价断言（券：减5.00/实付5.00）
#       → 购买1（带券）→ 记录 price=实付 5.00、余额 5.00、券 USED、报价 USED
#       → 报价断言（积分：抵7.00/扣70分/现金3.00）
#       → 购买2（积分抵扣）→ 记录 price=10.00、余额 2.00、积分余额 300、
#         双腿 POSTED（CASH 300 + LOYALTY 700）、流水恰 5 行
#       → 购买3（免单边界）→ 5 元券全抵 1 元 box1 → 实付 0.00、
#         零腿零冻结、券 USED、报价 USED、余额/积分/流水均不变
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景11：券+积分抵扣（coupon_loyalty）"

# ---------- 1. 注册 + 充值 10 元 ----------
print_step "1. 注册用户并充值 10 元"
reg=$(register_user "e2e_coupon_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge '{"amount":"10"}' "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"

# ---------- 2. SQL 授信积分钱包（分单位：1000 分 = 100 积分） + credit 流水 ----------
print_step "2. SQL 授信积分钱包 1000 分 + credit 流水"
do_mysql "INSERT INTO gacha_money.wallet_accounts (user_id, wallet_kind, currency, available, held)
          VALUES (${user_id}, 4, 'POINT', 1000, 0)
          ON DUPLICATE KEY UPDATE available = available + 1000;"
do_mysql "INSERT INTO gacha_money.wallet_ledgers (log_no, user_id, wallet_kind, amount, biz_type, biz_id, status)
          VALUES ('e2e_credit_${user_id}', ${user_id}, 4, 1000, 'credit', 'e2e_s11', 'success');"
loyal=$(get_wallet_available "$user_id" 4)
[ "$loyal" == "1000" ] || print_error "积分钱包 available 应为 1000，实际: ${loyal}"
print_step "积分钱包就绪：available=1000"

# ---------- 3. 领取 T-FIX-500（无门槛 5 元直减） ----------
print_step "3. 领取 T-FIX-500 券"
coupon_no=$(claim_coupon "$token" "T-FIX-500") || exit 1
coupon_id=$(get_user_coupon_id "$user_id" "T-FIX-500")
[ -n "$coupon_id" ] || { print_error "未查到领取的券"; exit $FAILED; }
print_step "couponNo: ${coupon_no}, id: ${coupon_id}"

# ---------- 4. 报价断言（券）：box2 原价 10.00 → 券减 5.00 → 实付 5.00 ----------
print_step "4. 报价断言：box2(10.00) + 5 元券"
q1=$(get_quote "$token" 2 "$coupon_id")
[ "$(extract_code "$q1")" == "0" ] || { print_error "报价失败: ${q1}"; exit $FAILED; }
[ "$(extract_field "$q1" gross)" == "10.00" ] || print_error "原价应为 10.00，实际: $(extract_field "$q1" gross)"
[ "$(extract_field "$q1" couponDiscount)" == "5.00" ] || print_error "券抵扣应为 5.00，实际: $(extract_field "$q1" couponDiscount)"
[ "$(extract_field "$q1" payable)" == "5.00" ] || print_error "实付应为 5.00，实际: $(extract_field "$q1" payable)"
[ "$(extract_field "$q1" couponNo)" == "$coupon_no" ] || print_error "券号应一致: $(extract_field "$q1" couponNo) ≠ ${coupon_no}"
print_step "券报价断言通过：gross=10.00, couponDiscount=5.00, payable=5.00"

# ---------- 5. 购买1（带券） ----------
print_step "5. 购买1：box2 + 券（实付 5.00）"
rid1=$(buy_box "$token" 2 "" "$coupon_id") || exit 1
[ -n "$rid1" ] || { print_error "requestId 不应为空"; exit $FAILED; }
print_step "requestId: ${rid1}"
record=$(poll_record "$token" "$rid1" 20)
[ -n "$record" ] || { print_error "轮询超时：购买1 开奖未完成"; exit $FAILED; }
price=$(extract_field "$record" price)
[ "$price" == "5.00" ] || print_error "记录 price 应为实付 5.00，实际: ${price}"
wait_payment_paid "$rid1" 20 || { print_error "支付单未在 20s 内到达 PAID"; exit $FAILED; }
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "500" ] || print_error "购买1 后余额应为 5.00，实际: ${balance}"
wait_coupon_status "$coupon_id" 3 20 || print_error "券未在 20s 内到达 USED(3)，实际: $(get_coupon_status "$coupon_id")"
qstat=$(get_quote_status "$rid1")
[ "$qstat" == "3" ] || print_error "购买1 报价锁应为 USED(3)，实际: ${qstat}"
leg1=$(get_payment_leg_status "$rid1" 1)
[ "$leg1" == "2" ] || print_error "购买1 CASH 腿应为 2(POSTED)，实际: ${leg1}"
leg4=$(get_payment_leg_status "$rid1" 4)
[ -z "$leg4" ] || print_error "券单不应有 LOYALTY 腿，实际: ${leg4}"
print_step "购买1 断言通过：price=5.00, balance=5.00, 券 USED, 报价 USED, 单腿 POSTED"

# ---------- 6. 报价断言（积分）：1000 分 → 抵 7.00（70% 上限）/ 扣 70 分 ----------
print_step "6. 报价断言：box2 + 积分抵扣"
q2=$(get_quote "$token" 2 0 1)
[ "$(extract_code "$q2")" == "0" ] || { print_error "积分报价失败: ${q2}"; exit $FAILED; }
[ "$(extract_field "$q2" loyalty)" == "7.00" ] || print_error "积分抵扣应为 7.00（70% 上限），实际: $(extract_field "$q2" loyalty)"
[ "$(extract_field "$q2" loyaltyPointsDebit)" == "70" ] || print_error "应扣 70 积分，实际: $(extract_field "$q2" loyaltyPointsDebit)"
[ "$(extract_field "$q2" cash)" == "3.00" ] || print_error "现金部分应为 3.00，实际: $(extract_field "$q2" cash)"
[ "$(extract_field "$q2" payable)" == "10.00" ] || print_error "实付应为 10.00（payable=cash+loyalty），实际: $(extract_field "$q2" payable)"
[ "$(extract_field "$q2" loyaltyMax)" == "7.00" ] || print_error "积分最高可抵应为 7.00，实际: $(extract_field "$q2" loyaltyMax)"
print_step "积分报价断言通过：loyalty=7.00, pointsDebit=70, cash=3.00, payable=10.00"

# ---------- 7. 购买2（积分抵扣） ----------
print_step "7. 购买2：box2 + 积分抵扣（现金 3.00 + 积分 7.00）"
rid2=$(buy_box "$token" 2 "" 0 1) || exit 1
[ -n "$rid2" ] || { print_error "requestId 不应为空"; exit $FAILED; }
print_step "requestId: ${rid2}"
record=$(poll_record "$token" "$rid2" 20)
[ -n "$record" ] || { print_error "轮询超时：购买2 开奖未完成"; exit $FAILED; }
price=$(extract_field "$record" price)
[ "$price" == "10.00" ] || print_error "记录 price 应为实付 10.00，实际: ${price}"
wait_payment_paid "$rid2" 20 || { print_error "支付单未在 20s 内到达 PAID"; exit $FAILED; }
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "200" ] || print_error "购买2 后余额应为 2.00，实际: ${balance}"
loyal=$(get_wallet_available "$user_id" 4)
[ "$loyal" == "300" ] || print_error "积分钱包 available 应为 300（1000-700），实际: ${loyal}"
leg1=$(get_payment_leg_status "$rid2" 1)
[ "$leg1" == "2" ] || print_error "购买2 CASH 腿应为 2(POSTED)，实际: ${leg1}"
leg4=$(get_payment_leg_status "$rid2" 4)
[ "$leg4" == "2" ] || print_error "购买2 LOYALTY 腿应为 2(POSTED)，实际: ${leg4}"
leg4_amt=$(do_mysql "SELECT amount FROM gacha_money.payment_legs WHERE leg_no='$(payment_no "$rid2")_4';")
[ "$leg4_amt" == "700" ] || print_error "LOYALTY 腿金额应为 700（分），实际: ${leg4_amt}"
hold1=$(get_payment_hold_status "$rid2" 1)
[ "$hold1" == "2" ] || print_error "CASH hold 应为 2(POSTED)，实际: ${hold1}"
hold4=$(get_payment_hold_status "$rid2" 4)
[ "$hold4" == "2" ] || print_error "LOYALTY hold 应为 2(POSTED)，实际: ${hold4}"
freeze4=$(get_ledger_field "log_no='h_$(payment_no "$rid2")_4'" amount)
[ "$freeze4" == "-700" ] || print_error "LOYALTY freeze 流水应为 -700，实际: ${freeze4}"
lc=$(count_wallet_ledgers "user_id=${user_id}")
[ "$lc" == "5" ] || print_error "流水应恰 5 行（充值1+授信1+购买1冻结1+购买2冻结2），实际: ${lc}"
print_step "购买2 断言通过：price=10.00, balance=2.00, 积分 300, 双腿 POSTED, 流水 5 行"

# ---------- 8. 购买3（免单边界）：5 元券全抵 1 元 box1 → 实付 0，零腿零冻结 ----------
print_step "8. 购买3：box1(1.00) + 5 元券（实付 0.00 免单）"
coupon_no3=$(claim_coupon "$token" "T-FIX-500") || exit 1
coupon_id3=$(get_user_coupon_id "$user_id" "T-FIX-500")
[ -n "$coupon_id3" ] || { print_error "未查到领取的券"; exit $FAILED; }
q3=$(get_quote "$token" 1 "$coupon_id3")
[ "$(extract_code "$q3")" == "0" ] || { print_error "免单报价失败: ${q3}"; exit $FAILED; }
[ "$(extract_field "$q3" couponDiscount)" == "1.00" ] || print_error "券抵扣应为 1.00（min(500,100)），实际: $(extract_field "$q3" couponDiscount)"
[ "$(extract_field "$q3" payable)" == "0.00" ] || print_error "免单实付应为 0.00，实际: $(extract_field "$q3" payable)"
rid3=$(buy_box "$token" 1 "" "$coupon_id3") || exit 1
print_step "requestId: ${rid3}"
record=$(poll_record "$token" "$rid3" 20)
[ -n "$record" ] || { print_error "轮询超时：购买3 开奖未完成"; exit $FAILED; }
price=$(extract_field "$record" price)
[ "$price" == "0.00" ] || print_error "记录 price 应为实付 0.00，实际: ${price}"
wait_payment_paid "$rid3" 20 || { print_error "支付单未在 20s 内到达 PAID"; exit $FAILED; }
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "200" ] || print_error "免单后余额应仍为 2.00，实际: ${balance}"
loyal=$(get_wallet_available "$user_id" 4)
[ "$loyal" == "300" ] || print_error "免单后积分应仍为 300，实际: ${loyal}"
leg1=$(get_payment_leg_status "$rid3" 1)
[ -z "$leg1" ] || print_error "免单单不应有 CASH 腿，实际: ${leg1}"
leg4=$(get_payment_leg_status "$rid3" 4)
[ -z "$leg4" ] || print_error "免单单不应有 LOYALTY 腿，实际: ${leg4}"
wait_coupon_status "$coupon_id3" 3 20 || print_error "券未在 20s 内到达 USED(3)，实际: $(get_coupon_status "$coupon_id3")"
qstat=$(get_quote_status "$rid3")
[ "$qstat" == "3" ] || print_error "免单报价锁应为 USED(3)，实际: ${qstat}"
lc=$(count_wallet_ledgers "user_id=${user_id}")
[ "$lc" == "5" ] || print_error "免单无冻结，流水应仍恰 5 行，实际: ${lc}"
print_step "购买3 断言通过：price=0.00、零腿零冻结、券 USED、报价 USED、流水不变"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景11 coupon_loyalty 通过${NC}"
else
    echo -e "${RED}× 场景11 coupon_loyalty 失败${NC}"
fi
exit $FAILED
