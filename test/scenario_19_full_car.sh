#!/bin/bash
# ============================================================
# 场景19：满车（full_car，M12b 全流程 + 幂等重放 + 满员拒绝 + 售罄补偿）
#
# 种子：1 号满车 4 座 100 分/座；整车结算 LUCKY 层=现金1000分+积分500、
#       NORMAL 层=积分100+券T-FIX-500；entries：谢谢参与 w30 q-1、
#       金币×50 w30 q5000、限定皮肤 w25 q100、限定头像框 w15 q100
# 流程：注册 4 用户各充值 20 元 → A 座0、B 座1、C 座2、D 座3 → 最后一票触发
#       DEPARTED：整车奖恰发一次（2 LUCKY+2 NORMAL 按 floor(4/2)；层内加权选奖
#       带放回，具体奖品随机——断言发放产物与落库 awards_json 逐项对账而非固定数）、
#       每人 regular 记录恰 1 条、余票 0、seats=[A,B,C,D]
#       → 同 key 重放（D）一切计数不变 → 已出发车再购被拒（无副作用）
#       → SQL 复位后清空 entries 库存 → E 购座 → Settle Aborted（3020 售罄）
#         → 补偿链：座位释放、余票复原、quote RELEASED、支付 CANCELED、无记录
#       → 复原库存 → E 新 key 成功（座0、余票3、余额 19.00）→ 现场复位
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景19：满车全流程与补偿（full_car）"

# ---------- 1. 注册 5 用户 + 各充值 20 元 ----------
print_step "1. 注册 5 用户（A/B/C/D/E）、各充值 20 元"
reg_a=$(register_user "e2e_fc_a_") || exit 1
uid_a=$(echo "$reg_a" | cut -d' ' -f1)
name_a=$(echo "$reg_a" | cut -d' ' -f2)
reg_b=$(register_user "e2e_fc_b_") || exit 1
uid_b=$(echo "$reg_b" | cut -d' ' -f1)
name_b=$(echo "$reg_b" | cut -d' ' -f2)
reg_c=$(register_user "e2e_fc_c_") || exit 1
uid_c=$(echo "$reg_c" | cut -d' ' -f1)
name_c=$(echo "$reg_c" | cut -d' ' -f2)
reg_d=$(register_user "e2e_fc_d_") || exit 1
uid_d=$(echo "$reg_d" | cut -d' ' -f1)
name_d=$(echo "$reg_d" | cut -d' ' -f2)
reg_e=$(register_user "e2e_fc_e_") || exit 1
uid_e=$(echo "$reg_e" | cut -d' ' -f1)
name_e=$(echo "$reg_e" | cut -d' ' -f2)
token_a=$(login_by_name "$name_a") || exit 1
token_b=$(login_by_name "$name_b") || exit 1
token_c=$(login_by_name "$name_c") || exit 1
token_d=$(login_by_name "$name_d") || exit 1
token_e=$(login_by_name "$name_e") || exit 1
for t in "$token_a" "$token_b" "$token_c" "$token_d" "$token_e"; do
    resp=$(do_post /user/recharge '{"amount":"20"}' "$t")
    [ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
done
[ "$(fen_of "$(get_balance "$token_a")")" == "2000" ] || print_error "A 充值后余额应为 20.00"

# ---------- 2. A 座0 / B 座1 / C 座2 依次购座 ----------
print_step "2. A→座0、B→座1、C→座2 依次购座（各 1.00）"
rid_a=$(full_car_buy "$token_a" 1 0 "$name_a") || exit 1
rec_a=$(poll_full_car_record "$token_a" "$rid_a" 20)
[ -n "$rec_a" ] || { print_error "A 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_a" seatIndex)" == "0" ] || print_error "A 应得座0，实际: ${rec_a}"
[ "$(extract_field "$rec_a" price)" == "1.00" ] || print_error "A 记录 price 应为 1.00，实际: ${rec_a}"
[ "$(extract_field "$rec_a" status)" == "1" ] || print_error "A 记录状态应为 1"

rid_b=$(full_car_buy "$token_b" 1 1 "$name_b") || exit 1
rec_b=$(poll_full_car_record "$token_b" "$rid_b" 20)
[ -n "$rec_b" ] || { print_error "B 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_b" seatIndex)" == "1" ] || print_error "B 应得座1，实际: ${rec_b}"

rid_c=$(full_car_buy "$token_c" 1 2 "$name_c") || exit 1
rec_c=$(poll_full_car_record "$token_c" "$rid_c" 20)
[ -n "$rec_c" ] || { print_error "C 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_c" seatIndex)" == "2" ] || print_error "C 应得座2，实际: ${rec_c}"
print_step "A/B/C 占座通过"

# ---------- 3. D 座3：最后一票触发整车结算 DEPARTED ----------
print_step "3. D→座3 最后一票：触发整车结算 DEPARTED"
rid_d=$(full_car_buy "$token_d" 1 3 "$name_d") || exit 1
rec_d=$(poll_full_car_record "$token_d" "$rid_d" 20)
[ -n "$rec_d" ] || { print_error "D 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_d" seatIndex)" == "3" ] || print_error "D 应得座3，实际: ${rec_d}"
print_step "requestId(D): ${rid_d}"

# ---------- 4. 等待 4 笔 SAGA 全部收敛（支付 PAID + 报价 USED） ----------
print_step "4. 4 笔支付 PAID、报价 USED"
for r in "$rid_a" "$rid_b" "$rid_c" "$rid_d"; do
    wait_payment_paid "$r" 20 || { print_error "支付单 ${r} 未在 20s 内到达 PAID"; exit $FAILED; }
    wait_quote_status "$r" 3 20 || print_error "报价锁 ${r} 未到达 USED(3)，实际: $(get_quote_status "$r")"
done

# ---------- 5. 状态与整车奖断言 ----------
print_step "5. 状态断言：余票0 / DEPARTED / seats=[A,B,C,D]"
[ "$(get_full_car_state_field 1 remaining_tickets)" == "0" ] || print_error "余票应为 0，实际: $(get_full_car_state_field 1 remaining_tickets)"
[ "$(get_full_car_state_field 1 status)" == "2" ] || print_error "状态应为 DEPARTED(2)，实际: $(get_full_car_state_field 1 status)"
[ "$(get_full_car_state_field 1 seats_json)" == "[${uid_a},${uid_b},${uid_c},${uid_d}]" ] || print_error "seats 应为 [A,B,C,D]，实际: $(get_full_car_state_field 1 seats_json)"

print_step "5.2 整车奖恰发一次：awards 4 条（2 LUCKY+2 NORMAL）挂本 gid、每座一条"
awards_json=$(get_full_car_state_field 1 awards_json)
[ "$(count_field "$awards_json" "\"request_id\":\"${rid_d}\"")" == "4" ] || print_error "awards 应恰 4 条且全挂 D 的 gid，实际: ${awards_json}"
[ "$(count_field "$awards_json" '"tier":1')" == "2" ] || print_error "LUCKY 奖应恰 2 条，实际: ${awards_json}"
[ "$(count_field "$awards_json" '"tier":2')" == "2" ] || print_error "NORMAL 奖应恰 2 条，实际: ${awards_json}"
for s in 0 1 2 3; do
    [ "$(count_field "$awards_json" "\"seat_index\":${s}")" == "1" ] || print_error "座${s} 应恰 1 个整车奖，实际: ${awards_json}"
done

print_step "5.3 发放断言：4 个整车奖恰发一次，且与 awards_json 逐项对账"
# 层内加权选奖带放回（server_v2 语义），具体谁中现金/积分/券随机——
# 断言改为「发放产物与落库 awards 完全一致」而非固定数值：
cash_kind=$(count_field "$awards_json" '"prize_kind":1')
points_kind=$(count_field "$awards_json" '"prize_kind":2')
coupon_kind=$(count_field "$awards_json" '"prize_kind":3')
ledgers_cnt=$(count_full_car_prize_ledgers "$rid_d")
coupons_cnt=$(count_full_car_prize_coupons "$rid_d")
[ "$((ledgers_cnt + coupons_cnt))" == "4" ] || print_error "发放产物合计应恰 4（流水${ledgers_cnt}+券${coupons_cnt}）"
[ "$ledgers_cnt" == "$((cash_kind + points_kind))" ] || print_error "钱包流水数应=现金奖+积分奖，实际: ${ledgers_cnt} vs $((cash_kind + points_kind))"
[ "$coupons_cnt" == "$coupon_kind" ] || print_error "发奖券数应=券奖数，实际: ${coupons_cnt} vs ${coupon_kind}"
cash_awarded=$(sum_json_field "$awards_json" cash_fen)
[ "$(sum_full_car_cash_ledger "$rid_d")" == "$cash_awarded" ] || print_error "现金流水合计应=awards cash_fen 之和，实际: $(sum_full_car_cash_ledger "$rid_d") vs ${cash_awarded}"
points_awarded=$(sum_json_field "$awards_json" points_amount)
points_total=$(( $(get_loyalty_points "$uid_a") + $(get_loyalty_points "$uid_b") + $(get_loyalty_points "$uid_c") + $(get_loyalty_points "$uid_d") ))
[ "$points_total" == "$points_awarded" ] || print_error "4 用户积分合计应=awards points_amount 之和，实际: ${points_total} vs ${points_awarded}"
coupons_total=$(( $(count_user_coupons "$uid_a" "T-FIX-500") + $(count_user_coupons "$uid_b" "T-FIX-500") + $(count_user_coupons "$uid_c" "T-FIX-500") + $(count_user_coupons "$uid_d" "T-FIX-500") ))
[ "$coupons_total" == "$coupons_cnt" ] || print_error "4 用户 T-FIX-500 券合计应=发奖券数，实际: ${coupons_total} vs ${coupons_cnt}"
balance_total=$(( $(fen_of "$(get_balance "$token_a")") + $(fen_of "$(get_balance "$token_b")") + $(fen_of "$(get_balance "$token_c")") + $(fen_of "$(get_balance "$token_d")") ))
expect_balance=$((7600 + cash_awarded))
[ "$balance_total" == "$expect_balance" ] || print_error "4 用户余额合计应 ${expect_balance}（4×19.00+现金奖${cash_awarded}），实际: ${balance_total}"

print_step "5.4 记录断言：每人 regular 记录恰 1 条"
for r in "$rid_a" "$rid_b" "$rid_c" "$rid_d"; do
    [ "$(count_full_car_records "$r")" == "1" ] || print_error "gid ${r} 记录应恰 1 条，实际: $(count_full_car_records "$r")"
done

print_step "5.5 HTTP 详情：GET /full-car/1 显示 DEPARTED/余票0/4 整车奖"
detail=$(get_full_car "$token_a" 1)
[ "$(extract_code "$detail")" == "0" ] || print_error "满车详情查询失败: ${detail}"
[ "$(extract_field "$detail" status)" == "2" ] || print_error "详情 status 应为 2，实际: ${detail}"
[ "$(extract_field "$detail" remainingTickets)" == "0" ] || print_error "详情 remainingTickets 应为 0，实际: ${detail}"
[ "$(count_field "$detail" '"tier":')" == "4" ] || print_error "详情应含 4 个整车奖，实际: ${detail}"
print_step "满车主流程断言全部通过"

# ---------- 6. 同 key 重放（D）：一切计数不变 ----------
print_step "6. 同 Idempotency-Key 重放（D）"
ledgers_before=$(count_full_car_prize_ledgers "$rid_d")
coupons_before=$(count_full_car_prize_coupons "$rid_d")
resp=$(do_post "/full-car/1/buy" '{"seatIndex":3}' "$token_d" "$name_d")
[ "$(extract_code "$resp")" == "0" ] || print_error "重放应返回成功: ${resp}"
[ "$(extract_field "$resp" status)" == "success" ] || print_error "重放 status 应为 success，实际: ${resp}"
[ "$(count_full_car_records "$rid_d")" == "1" ] || print_error "重放后 D 记录应仍为 1 条"
[ "$(count_full_car_prize_ledgers "$rid_d")" == "$ledgers_before" ] || print_error "重放后发奖流水应仍为 ${ledgers_before} 笔，实际: $(count_full_car_prize_ledgers "$rid_d")"
[ "$(count_full_car_prize_coupons "$rid_d")" == "$coupons_before" ] || print_error "重放后发奖券应仍为 ${coupons_before} 张，实际: $(count_full_car_prize_coupons "$rid_d")"
[ "$(get_full_car_state_field 1 remaining_tickets)" == "0" ] || print_error "重放后余票应仍为 0"
[ "$(count_field "$(get_full_car_state_field 1 awards_json)" "\"request_id\":\"${rid_d}\"")" == "4" ] || print_error "重放后 awards 应仍为 4 条"
print_step "重放幂等通过：记录/流水/券/余票/awards 均不变"

# ---------- 7. 已出发车再购：03 Aborted 3024 → 无副作用 ----------
print_step "7. 已出发车 E 再购：LockFullCar Aborted（满车状态不满足锁定）"
rid_e1=$(full_car_buy "$token_e" 1 0 "$name_e") || exit 1
wait_barrier_compensated "$rid_e1" 20 || print_error "已出发拒绝的补偿链未在 20s 内收敛"
print_step "7.1 全回滚断言：无记录 / 报价 OPEN / 无支付单 / 车状态不变 / E 余额不变"
[ "$(count_full_car_records "$rid_e1")" == "0" ] || print_error "已出发车不应产生记录"
wait_quote_status "$rid_e1" 1 20 || print_error "正向自身回滚后报价应回到 OPEN(1)，实际: $(get_quote_status "$rid_e1")"
[ "$(count_payment_orders "$rid_e1")" == "0" ] || print_error "03 中止后不应建支付单，实际: $(count_payment_orders "$rid_e1")"
[ "$(get_full_car_state_field 1 remaining_tickets)" == "0" ] || print_error "已出发后余票应仍为 0"
[ "$(get_full_car_state_field 1 status)" == "2" ] || print_error "已出发后状态应仍为 DEPARTED(2)"
[ "$(get_full_car_state_field 1 seats_json)" == "[${uid_a},${uid_b},${uid_c},${uid_d}]" ] || print_error "已出发后 seats 不应变动"
balance=$(wait_balance "$token_e" "20.00" 5)
[ -n "$balance" ] || print_error "E 余额应仍为 20.00，实际: $(get_balance "$token_e")"
print_step "已出发拒绝全回滚通过"

# ---------- 8. 现场复位 + 清空 entries 库存 → E 购座 Settle Aborted ----------
print_step "8. SQL 复位满车现场 + 清空 entries 库存（注入售罄）"
reset_full_car_state 1
[ "$(get_full_car_state_field 1 seats_json)" == "[0,0,0,0]" ] || print_error "复位后 seats 应为全空，实际: $(get_full_car_state_field 1 seats_json)"
[ "$(get_full_car_state_field 1 remaining_tickets)" == "4" ] || print_error "复位后余票应为 4"
[ "$(get_full_car_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
set_full_car_entry_stock 1 '谢谢参与' 0
set_full_car_entry_stock 1 '金币×50' 0
set_full_car_entry_stock 1 '限定皮肤' 0
set_full_car_entry_stock 1 '限定头像框' 0

print_step "8.1 E 购座：03 占座成功 → 05 Settle Aborted（3020 奖池售罄）"
rid_e2=$(full_car_buy "$token_e" 1 0 "${name_e}_2") || exit 1
print_step "requestId: ${rid_e2}（SettleFullCar 将 Aborted，补偿链收敛）"
wait_barrier_compensated "$rid_e2" 20 || print_error "售罄 SAGA 补偿链未在 20s 内收敛"

print_step "8.2 补偿链断言：座位释放 / 余票复原 / 报价 RELEASED / 支付 CANCELED / 无记录"
wait_quote_status "$rid_e2" 4 20 || print_error "补偿后报价应 RELEASED(4)，实际: $(get_quote_status "$rid_e2")"
wait_payment_status "$rid_e2" 6 20 || print_error "补偿后支付单应 CANCELED(6)，实际: $(get_payment_status "$rid_e2")"
[ "$(count_full_car_records "$rid_e2")" == "0" ] || print_error "售罄不应产生记录，实际: $(count_full_car_records "$rid_e2")"
[ "$(get_full_car_state_field 1 seats_json)" == "[0,0,0,0]" ] || print_error "补偿后座位应全空，实际: $(get_full_car_state_field 1 seats_json)"
[ "$(get_full_car_state_field 1 remaining_tickets)" == "4" ] || print_error "补偿后余票应复原为 4，实际: $(get_full_car_state_field 1 remaining_tickets)"
[ "$(get_full_car_state_field 1 status)" == "1" ] || print_error "补偿后状态应 OPEN(1)"
balance=$(wait_balance "$token_e" "20.00" 5)
[ -n "$balance" ] || print_error "售罄补偿后 E 余额应仍为 20.00，实际: $(get_balance "$token_e")"
print_step "售罄补偿链通过：座/票/报价/支付/记录/余额 六项全回滚"

# ---------- 9. 复原库存 → E 新 key 重试成功 ----------
print_step "9. 复原 entries 库存 → E 新 key 重试"
set_full_car_entry_stock 1 '谢谢参与' -1
set_full_car_entry_stock 1 '金币×50' 5000
set_full_car_entry_stock 1 '限定皮肤' 100
set_full_car_entry_stock 1 '限定头像框' 100
rid_e3=$(full_car_buy "$token_e" 1 0 "${name_e}_3") || exit 1
rec_e=$(poll_full_car_record "$token_e" "$rid_e3" 20)
[ -n "$rec_e" ] || { print_error "E 重试记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_e" seatIndex)" == "0" ] || print_error "E 应得座0，实际: ${rec_e}"
[ "$(extract_field "$rec_e" status)" == "1" ] || print_error "E 记录状态应为 1"
wait_payment_paid "$rid_e3" 20 || { print_error "E 重试支付未 PAID"; exit $FAILED; }
wait_quote_status "$rid_e3" 3 20 || print_error "E 重试报价未 USED(3)"
balance=$(wait_balance "$token_e" "19.00" 20)
[ -n "$balance" ] || print_error "E 重试后余额应为 19.00，实际: $(get_balance "$token_e")"
[ "$(get_full_car_state_field 1 seats_json)" == "[${uid_e},0,0,0]" ] || print_error "E 重试后 seats 应为 [E,0,0,0]，实际: $(get_full_car_state_field 1 seats_json)"
[ "$(get_full_car_state_field 1 remaining_tickets)" == "3" ] || print_error "E 重试后余票应为 3，实际: $(get_full_car_state_field 1 remaining_tickets)"
print_step "售罄复原后重试通过"

# ---------- 10. 收尾：现场复位（保持测试环境整洁，run_all 二轮可重复执行） ----------
print_step "10. 收尾：满车现场复位为空车 OPEN"
reset_full_car_state 1
[ "$(get_full_car_state_field 1 seats_json)" == "[0,0,0,0]" ] || print_error "复位后 seats 应为全空"
[ "$(get_full_car_state_field 1 remaining_tickets)" == "4" ] || print_error "复位后余票应为 4"
[ "$(get_full_car_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
[ "$(get_full_car_state_field 1 awards_json)" == "[]" ] || print_error "复位后 awards 应为空"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景19 full_car 通过${NC}"
else
    echo -e "${RED}× 场景19 full_car 失败${NC}"
fi
exit $FAILED
