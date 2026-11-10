#!/bin/bash
# ============================================================
# 场景20：扭蛋（gashapon，M12c 全流程 + 幂等重放 + 售罄补偿）
#
# 种子：1 号扭蛋 6 票 100 分/票；特别奖 LAST 现金5000×1、MVP 积分2000×1、
#       LUCKY 现金1000×2（stage1）；entries：谢谢参与×2 w40、金币×50 ×2 w30、
#       限定皮肤×1 w20、限定头像框×1 w10（总量 6，全有限量）
# 流程：注册 A/B/C 各充值 20 元 → A 抽 2（drawn=2 无判定）→ B 抽 3
#       （drawn=5 ≥ T0=3 触发 LUCKY 单元0：票据区间 [1..3]，用 gasha_pick_key
#        预选 SHA256%3==2 的幂等键 → 中奖票 3 归 B）→ C 抽 1（清空：drawn=6 ≥ T1=6
#        触发 LUCKY 单元1（区间 [4..6]，预选键 → 票 6 归 C）+ 终局 LAST→C、MVP→B）
#       → 断言 sent=[1,1,2]、awards 4 条（1 条挂 B 的 gid、3 条挂 C 的 gid）、
#         发奖恰一次、钱包/积分对账、余额 A18/B27/C79
#       → 同 key 重放（C）一切计数不变 → SQL 复位后清空 entries 库存
#       → A 新 key 抽 2 → Settle Aborted → 补偿链：quote RELEASED、支付 CANCELED、
#         无记录、participants {}、余票复原 6、余额不变
#       → 复原库存 → A 再新 key 成功（余票4、余额 16.00）→ 现场复位
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景20：扭蛋全流程与补偿（gashapon）"

# ---------- 1. 注册 3 用户 + 各充值 20 元 ----------
print_step "1. 注册 3 用户（A/B/C）、各充值 20 元"
reg_a=$(register_user "e2e_ga_a_") || exit 1
uid_a=$(echo "$reg_a" | cut -d' ' -f1)
name_a=$(echo "$reg_a" | cut -d' ' -f2)
reg_b=$(register_user "e2e_ga_b_") || exit 1
uid_b=$(echo "$reg_b" | cut -d' ' -f1)
name_b=$(echo "$reg_b" | cut -d' ' -f2)
reg_c=$(register_user "e2e_ga_c_") || exit 1
uid_c=$(echo "$reg_c" | cut -d' ' -f1)
name_c=$(echo "$reg_c" | cut -d' ' -f2)
token_a=$(login_by_name "$name_a") || exit 1
token_b=$(login_by_name "$name_b") || exit 1
token_c=$(login_by_name "$name_c") || exit 1
for t in "$token_a" "$token_b" "$token_c"; do
    resp=$(do_post /user/recharge '{"amount":"20"}' "$t")
    [ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
done
[ "$(fen_of "$(get_balance "$token_a")")" == "2000" ] || print_error "A 充值后余额应为 20.00"

# ---------- 2. SQL 复位扭蛋现场 ----------
print_step "2. SQL 复位扭蛋现场（余票 6 / OPEN / sent 全 0）"
reset_gashapon_state 1
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "6" ] || print_error "复位后余票应为 6，实际: $(get_gashapon_state_field 1 remaining_tickets)"
[ "$(get_gashapon_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
[ "$(get_gashapon_state_field 1 participants_json)" == "{}" ] || print_error "复位后 participants 应为空"
[ "$(get_gashapon_state_field 1 records_json)" == "[]" ] || print_error "复位后 records 应为空"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,0]" ] || print_error "复位后 sent 应为 [0,0,0]，实际: $(get_gashapon_state_field 1 sent_json)"
[ "$(get_gashapon_state_field 1 awards_json)" == "[]" ] || print_error "复位后 awards 应为空"

# ---------- 3. A 抽 2（drawn=2 < T0=3，无分阶段判定） ----------
print_step "3. A 抽 2（drawn=2 无判定）"
rid_a=$(gashapon_buy "$token_a" 1 2 "$name_a") || exit 1
rec_a=$(poll_gashapon_record "$token_a" "$rid_a" 20)
[ -n "$rec_a" ] || { print_error "A 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_a" count)" == "2" ] || print_error "A 应抽 2 次，实际: ${rec_a}"
[ "$(extract_field "$rec_a" price)" == "2.00" ] || print_error "A 记录 price 应为 2.00，实际: ${rec_a}"
[ "$(extract_field "$rec_a" status)" == "1" ] || print_error "A 记录状态应为 1"
[ "$(count_field "$rec_a" '"name":')" == "2" ] || print_error "A 记录应含 2 个开奖条目，实际: ${rec_a}"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,0]" ] || print_error "A 抽后 sent 应仍为 [0,0,0]，实际: $(get_gashapon_state_field 1 sent_json)"
print_step "A 抽 2 通过，sent 无变化"

# ---------- 4. B 抽 3：drawn=5 触发 LUCKY 单元0，预选键保证票 3 归 B ----------
print_step "4. B 抽 3（drawn=5 ≥ T0=3：LUCKY 单元0 判定，预选幂等键）"
key_b=$(gasha_pick_key "${name_b}_b" 0) || { print_error "B 的预选键搜索失败"; exit $FAILED; }
print_step "key_b: ${key_b}（SHA256(gid‖stage‖BE32(0))%3==2 → 中奖票 3 归 B）"
rid_b=$(gashapon_buy "$token_b" 1 3 "$key_b") || exit 1
rec_b=$(poll_gashapon_record "$token_b" "$rid_b" 20)
[ -n "$rec_b" ] || { print_error "B 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_b" count)" == "3" ] || print_error "B 应抽 3 次，实际: ${rec_b}"
[ "$(extract_field "$rec_b" price)" == "3.00" ] || print_error "B 记录 price 应为 3.00，实际: ${rec_b}"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,1]" ] || print_error "B 抽后 sent 应为 [0,0,1]，实际: $(get_gashapon_state_field 1 sent_json)"
print_step "B 抽 3 通过，LUCKY 单元0 已发（requestId: ${rid_b}）"

# ---------- 5. C 抽 1（清空）：单元1 判定 + 终局 LAST→C、MVP→B ----------
print_step "5. C 抽 1 清空（drawn=6：LUCKY 单元1 + LAST→C + MVP→B）"
key_c=$(gasha_pick_key "${name_c}_c" 1) || { print_error "C 的预选键搜索失败"; exit $FAILED; }
print_step "key_c: ${key_c}（SHA256(gid‖stage‖BE32(1))%3==2 → 中奖票 6 归 C）"
rid_c=$(gashapon_buy "$token_c" 1 1 "$key_c") || exit 1
rec_c=$(poll_gashapon_record "$token_c" "$rid_c" 20)
[ -n "$rec_c" ] || { print_error "C 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_c" count)" == "1" ] || print_error "C 应抽 1 次，实际: ${rec_c}"
[ "$(extract_field "$rec_c" price)" == "1.00" ] || print_error "C 记录 price 应为 1.00，实际: ${rec_c}"
print_step "requestId(C): ${rid_c}"

# ---------- 6. 等待 3 笔 SAGA 全部收敛（支付 PAID + 报价 USED） ----------
print_step "6. 3 笔支付 PAID、报价 USED"
for r in "$rid_a" "$rid_b" "$rid_c"; do
    wait_payment_paid "$r" 20 || { print_error "支付单 ${r} 未在 20s 内到达 PAID"; exit $FAILED; }
    wait_quote_status "$r" 3 20 || print_error "报价锁 ${r} 未到达 USED(3)，实际: $(get_quote_status "$r")"
done

# ---------- 7. 状态断言 ----------
print_step "7. 状态断言：余票0 / SETTLED / participants / records / sent=[1,1,2]"
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "0" ] || print_error "余票应为 0，实际: $(get_gashapon_state_field 1 remaining_tickets)"
[ "$(get_gashapon_state_field 1 status)" == "2" ] || print_error "状态应为 SETTLED(2)，实际: $(get_gashapon_state_field 1 status)"
parts=$(get_gashapon_state_field 1 participants_json)
[ "$(count_field "$parts" "\"${uid_a}\":2")" == "1" ] || print_error "participants 应含 A:2，实际: ${parts}"
[ "$(count_field "$parts" "\"${uid_b}\":3")" == "1" ] || print_error "participants 应含 B:3，实际: ${parts}"
[ "$(count_field "$parts" "\"${uid_c}\":1")" == "1" ] || print_error "participants 应含 C:1，实际: ${parts}"
recs=$(get_gashapon_state_field 1 records_json)
[ "$(count_field "$recs" "\"user_id\":${uid_a},\"request_id\":\"${rid_a}\",\"count\":2")" == "1" ] || print_error "records 应含 A 的票据记录，实际: ${recs}"
[ "$(count_field "$recs" "\"user_id\":${uid_b},\"request_id\":\"${rid_b}\",\"count\":3")" == "1" ] || print_error "records 应含 B 的票据记录，实际: ${recs}"
[ "$(count_field "$recs" "\"user_id\":${uid_c},\"request_id\":\"${rid_c}\",\"count\":1")" == "1" ] || print_error "records 应含 C 的票据记录，实际: ${recs}"
[ "$(get_gashapon_state_field 1 sent_json)" == "[1,1,2]" ] || print_error "终局 sent 应为 [1,1,2]，实际: $(get_gashapon_state_field 1 sent_json)"

print_step "7.1 awards 4 条：1 条挂 B 的 gid（LUCKY 单元0 现金1000→B）、3 条挂 C 的 gid"
awards_json=$(get_gashapon_state_field 1 awards_json)
[ "$(count_field "$awards_json" "\"request_id\":\"${rid_b}\"")" == "1" ] || print_error "awards 应恰 1 条挂 B 的 gid，实际: ${awards_json}"
[ "$(count_field "$awards_json" "\"request_id\":\"${rid_c}\"")" == "3" ] || print_error "awards 应恰 3 条挂 C 的 gid，实际: ${awards_json}"
[ "$(count_field "$awards_json" '"special_kind":"LUCKY"')" == "2" ] || print_error "LUCKY 奖应恰 2 条，实际: ${awards_json}"
[ "$(count_field "$awards_json" '"special_kind":"LAST"')" == "1" ] || print_error "LAST 奖应恰 1 条，实际: ${awards_json}"
[ "$(count_field "$awards_json" '"special_kind":"MVP"')" == "1" ] || print_error "MVP 奖应恰 1 条，实际: ${awards_json}"
[ "$(count_field "$awards_json" "\"unit\":0,\"user_id\":${uid_b},\"special_kind\":\"LUCKY\",\"prize_kind\":1,\"cash_fen\":1000")" == "1" ] || print_error "LUCKY 单元0 应归 B（现金1000），实际: ${awards_json}"
[ "$(count_field "$awards_json" "\"unit\":1,\"user_id\":${uid_c},\"special_kind\":\"LUCKY\",\"prize_kind\":1,\"cash_fen\":1000")" == "1" ] || print_error "LUCKY 单元1 应归 C（现金1000），实际: ${awards_json}"
[ "$(count_field "$awards_json" "\"unit\":0,\"user_id\":${uid_c},\"special_kind\":\"LAST\",\"prize_kind\":1,\"cash_fen\":5000")" == "1" ] || print_error "LAST 应归 C（现金5000），实际: ${awards_json}"
[ "$(count_field "$awards_json" "\"unit\":0,\"user_id\":${uid_b},\"special_kind\":\"MVP\",\"prize_kind\":2,\"cash_fen\":0,\"points_amount\":2000")" == "1" ] || print_error "MVP 应归 B（积分2000），实际: ${awards_json}"

# ---------- 8. 发放断言：恰发一次 + 钱包/积分对账 ----------
print_step "8. 发放断言：4 个特别奖恰发一次，与 awards_json 逐项对账"
ledgers_b=$(count_gashapon_prize_ledgers "$rid_b")
ledgers_c=$(count_gashapon_prize_ledgers "$rid_c")
coupons_b=$(count_gashapon_prize_coupons "$rid_b")
coupons_c=$(count_gashapon_prize_coupons "$rid_c")
[ "$ledgers_b" == "1" ] || print_error "B 的 gid 应恰 1 笔流水，实际: ${ledgers_b}"
[ "$ledgers_c" == "3" ] || print_error "C 的 gid 应恰 3 笔流水（2 现金+1 积分），实际: ${ledgers_c}"
[ "$coupons_b" == "0" ] && [ "$coupons_c" == "0" ] || print_error "特别奖无券，不应有发奖券"
[ "$(sum_gashapon_cash_ledger "$rid_b")" == "1000" ] || print_error "B 的 gid 现金流水合计应 1000，实际: $(sum_gashapon_cash_ledger "$rid_b")"
[ "$(sum_gashapon_cash_ledger "$rid_c")" == "6000" ] || print_error "C 的 gid 现金流水合计应 6000（1000+5000），实际: $(sum_gashapon_cash_ledger "$rid_c")"
[ "$(get_loyalty_points "$uid_b")" == "2000" ] || print_error "B 积分应恰 2000（MVP），实际: $(get_loyalty_points "$uid_b")"
balance=$(wait_balance "$token_b" "27.00" 20)
[ -n "$balance" ] || print_error "B 余额应 27.00（20-3+10），实际: $(get_balance "$token_b")"
balance=$(wait_balance "$token_c" "79.00" 20)
[ -n "$balance" ] || print_error "C 余额应 79.00（20-1+10+50），实际: $(get_balance "$token_c")"
balance=$(wait_balance "$token_a" "18.00" 20)
[ -n "$balance" ] || print_error "A 余额应 18.00（20-2），实际: $(get_balance "$token_a")"

# ---------- 9. HTTP 详情 ----------
print_step "9. HTTP 详情：GET /gashapon/1 显示 SETTLED/余票0/特别奖已发"
detail=$(get_gashapon "$token_a" 1)
[ "$(extract_code "$detail")" == "0" ] || print_error "扭蛋详情查询失败: ${detail}"
[ "$(extract_field "$detail" status)" == "2" ] || print_error "详情 status 应为 2，实际: ${detail}"
[ "$(extract_field "$detail" remainingTickets)" == "0" ] || print_error "详情 remainingTickets 应为 0，实际: ${detail}"
[ "$(extract_field "$detail" totalTickets)" == "6" ] || print_error "详情 totalTickets 应为 6，实际: ${detail}"
[ "$(extract_field "$detail" participants)" == "3" ] || print_error "详情 participants 应为 3，实际: ${detail}"
[ "$(count_field "$detail" '"specialKind":')" == "3" ] || print_error "详情应含 3 个特别奖，实际: ${detail}"
print_step "扭蛋主流程断言全部通过"

# ---------- 10. 同 key 重放（C）：一切计数不变 ----------
print_step "10. 同 Idempotency-Key 重放（C）"
ledgers_before=$(count_gashapon_prize_ledgers "$rid_c")
resp=$(do_post "/gashapon/1/buy" '{"count":1}' "$token_c" "$key_c")
[ "$(extract_code "$resp")" == "0" ] || print_error "重放应返回成功: ${resp}"
[ "$(extract_field "$resp" status)" == "success" ] || print_error "重放 status 应为 success，实际: ${resp}"
[ "$(count_gashapon_records "$rid_c")" == "1" ] || print_error "重放后 C 记录应仍为 1 条"
[ "$(count_gashapon_prize_ledgers "$rid_c")" == "$ledgers_before" ] || print_error "重放后发奖流水应仍为 ${ledgers_before} 笔，实际: $(count_gashapon_prize_ledgers "$rid_c")"
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "0" ] || print_error "重放后余票应仍为 0"
[ "$(get_gashapon_state_field 1 sent_json)" == "[1,1,2]" ] || print_error "重放后 sent 应仍为 [1,1,2]"
[ "$(count_field "$(get_gashapon_state_field 1 awards_json)" "\"request_id\":\"${rid_c}\"")" == "3" ] || print_error "重放后 awards 应仍为 3 条挂 C"
print_step "重放幂等通过：记录/流水/余票/sent/awards 均不变"

# ---------- 11. 现场复位 + 清空 entries 库存 → A 抽 2 Settle Aborted ----------
print_step "11. SQL 复位扭蛋现场 + 清空 entries 库存（注入售罄）"
reset_gashapon_state 1
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "6" ] || print_error "复位后余票应为 6"
[ "$(get_gashapon_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
set_gashapon_entry_stock 1 '谢谢参与' 0
set_gashapon_entry_stock 1 '金币×50' 0
set_gashapon_entry_stock 1 '限定皮肤' 0
set_gashapon_entry_stock 1 '限定头像框' 0

print_step "11.1 A 新 key 抽 2：03 锁定成功 → 05 Settle Aborted（奖池售罄）"
rid_a2=$(gashapon_buy "$token_a" 1 2 "${name_a}_2") || exit 1
print_step "requestId: ${rid_a2}（SettleGashapon 将 Aborted，补偿链收敛）"
wait_barrier_compensated "$rid_a2" 20 || print_error "售罄 SAGA 补偿链未在 20s 内收敛"

print_step "11.2 补偿链断言：报价 RELEASED / 支付 CANCELED / 无记录 / participants {} / 余票复原 6"
wait_quote_status "$rid_a2" 4 20 || print_error "补偿后报价应 RELEASED(4)，实际: $(get_quote_status "$rid_a2")"
wait_payment_status "$rid_a2" 6 20 || print_error "补偿后支付单应 CANCELED(6)，实际: $(get_payment_status "$rid_a2")"
[ "$(count_gashapon_records "$rid_a2")" == "0" ] || print_error "售罄不应产生记录，实际: $(count_gashapon_records "$rid_a2")"
[ "$(get_gashapon_state_field 1 participants_json)" == "{}" ] || print_error "补偿后 participants 应为空，实际: $(get_gashapon_state_field 1 participants_json)"
[ "$(get_gashapon_state_field 1 records_json)" == "[]" ] || print_error "补偿后 records 应为空，实际: $(get_gashapon_state_field 1 records_json)"
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "6" ] || print_error "补偿后余票应复原为 6，实际: $(get_gashapon_state_field 1 remaining_tickets)"
[ "$(get_gashapon_state_field 1 status)" == "1" ] || print_error "补偿后状态应 OPEN(1)"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,0]" ] || print_error "补偿后 sent 应 [0,0,0]"
balance=$(wait_balance "$token_a" "18.00" 5)
[ -n "$balance" ] || print_error "售罄补偿后 A 余额应仍为 18.00，实际: $(get_balance "$token_a")"
print_step "售罄补偿链通过：票/参与/记录/报价/支付/余额 六项全回滚"

# ---------- 12. 复原库存 → A 新 key 重试成功 ----------
print_step "12. 复原 entries 库存 → A 再新 key 重试"
set_gashapon_entry_stock 1 '谢谢参与' 2
set_gashapon_entry_stock 1 '金币×50' 2
set_gashapon_entry_stock 1 '限定皮肤' 1
set_gashapon_entry_stock 1 '限定头像框' 1
rid_a3=$(gashapon_buy "$token_a" 1 2 "${name_a}_3") || exit 1
rec_a3=$(poll_gashapon_record "$token_a" "$rid_a3" 20)
[ -n "$rec_a3" ] || { print_error "A 重试记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_a3" count)" == "2" ] || print_error "A 重试应抽 2 次，实际: ${rec_a3}"
[ "$(extract_field "$rec_a3" status)" == "1" ] || print_error "A 重试记录状态应为 1"
wait_payment_paid "$rid_a3" 20 || { print_error "A 重试支付未 PAID"; exit $FAILED; }
wait_quote_status "$rid_a3" 3 20 || print_error "A 重试报价未 USED(3)"
balance=$(wait_balance "$token_a" "16.00" 20)
[ -n "$balance" ] || print_error "A 重试后余额应为 16.00，实际: $(get_balance "$token_a")"
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "4" ] || print_error "A 重试后余票应为 4，实际: $(get_gashapon_state_field 1 remaining_tickets)"
[ "$(get_gashapon_state_field 1 status)" == "1" ] || print_error "A 重试后状态应 OPEN(1)"
[ "$(count_field "$(get_gashapon_state_field 1 participants_json)" "\"${uid_a}\":2")" == "1" ] || print_error "A 重试后 participants 应含 A:2，实际: $(get_gashapon_state_field 1 participants_json)"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,0]" ] || print_error "A 重试后 sent 应仍 [0,0,0]（drawn=2 无判定）"
print_step "售罄复原后重试通过"

# ---------- 13. 收尾：现场复位（保持测试环境整洁，run_all 二轮可重复执行） ----------
print_step "13. 收尾：扭蛋现场复位为 OPEN 全票"
reset_gashapon_state 1
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "6" ] || print_error "复位后余票应为 6"
[ "$(get_gashapon_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
[ "$(get_gashapon_state_field 1 participants_json)" == "{}" ] || print_error "复位后 participants 应为空"
[ "$(get_gashapon_state_field 1 records_json)" == "[]" ] || print_error "复位后 records 应为空"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,0]" ] || print_error "复位后 sent 应为 [0,0,0]"
[ "$(get_gashapon_state_field 1 awards_json)" == "[]" ] || print_error "复位后 awards 应为空"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景20 gashapon 通过${NC}"
else
    echo -e "${RED}× 场景20 gashapon 失败${NC}"
fi
exit $FAILED
