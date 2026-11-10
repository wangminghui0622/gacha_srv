#!/bin/bash
# ============================================================
# 场景22：连击链（combo_chain，M13 全流程 + 幂等重放 + 配置变更补偿）
#
# 种子：1 号连击链 100 分/票、3 票/车 × 2 车；全局奖 4 槽按 index%2+1 轮转分车
#       （槽0/2→车1、槽1/3→车2）：槽0 现金500、槽1 积分100、槽2 现金500、槽3 券 T-FIX-500；
#       共享奖：COMBO_TIER 现金5000×1（bps=10000 必触发、concentrate_threshold=1）、
#       EMPTY_STREAK 积分100 阈值2、WIN_STREAK 现金300 阈值1
# 流程：注册 A/B/C 各充值 20 元 → 车 1 三票（最后一票用 cc_pick_key 预选幂等键：
#       combo-winner%3==0 → 连击大奖得主=A、global-target(1)%2==0 → 槽2 归 B）→
#       整车结算 awards 4 条（combo A 现金5000 + 槽0 A 500 + 槽2 B 现金500 +
#       win_streak B 300）、车号推进 2、tier_remaining [0] → 车 2 三票（档位耗尽
#       必不触发 → 安全上限分布确定性：槽1→A（积分100）、槽3→B（券）；win A 300、win B 300、
#       empty C 积分100）→ SETTLED、9 条结算奖、现金流水 6900 分、1 券、积分 200、
#       连击全清零 → 同 key 重放（车1 C 的键）一切计数不变 → SQL 复位后注入
#       参与者错配（幽灵占位 → 结算时 4≠3）→ C 新 key → 05 Settle Aborted(3031)
#       → 补偿链：quote RELEASED、支付 CANCELED、无记录、参与者/余票复原 → 现场复位
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景22：连击链全流程与补偿（combo_chain）"

# ---------- 1. 注册 3 用户 + 各充值 20 元 ----------
print_step "1. 注册 3 用户（A/B/C）、各充值 20 元"
reg_a=$(register_user "e2e_cc_a_") || exit 1
uid_a=$(echo "$reg_a" | cut -d' ' -f1)
name_a=$(echo "$reg_a" | cut -d' ' -f2)
reg_b=$(register_user "e2e_cc_b_") || exit 1
uid_b=$(echo "$reg_b" | cut -d' ' -f1)
name_b=$(echo "$reg_b" | cut -d' ' -f2)
reg_c=$(register_user "e2e_cc_c_") || exit 1
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

# ---------- 2. SQL 复位连击链现场 ----------
print_step "2. SQL 复位连击链现场（车 1 / 3 票 / tier [1] / OPEN）"
reset_combo_chain_state 1
[ "$(get_combo_chain_state_field 1 car_index)" == "1" ] || print_error "复位后车号应为 1"
[ "$(get_combo_chain_state_field 1 participants_json)" == "[]" ] || print_error "复位后 participants 应为空"
[ "$(get_combo_chain_state_field 1 awards_json)" == "[]" ] || print_error "复位后 awards 应为空"
[ "$(get_combo_chain_state_field 1 tier_remaining_json)" == "[1]" ] || print_error "复位后档位库存应为 [1]，实际: $(get_combo_chain_state_field 1 tier_remaining_json)"
[ "$(get_combo_chain_state_field 1 remaining_tickets)" == "3" ] || print_error "复位后余票应为 3"
[ "$(get_combo_chain_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"

# ---------- 3. 车 1：A、B 购票（未满不结算） ----------
print_step "3. 车 1：A、B 各购 1 票（余票 3→2→1，不触发结算）"
rid_a1=$(combo_chain_buy "$token_a" 1 "${name_a}_a1") || exit 1
rec_a=$(poll_combo_chain_record "$token_a" "$rid_a1" 20)
[ -n "$rec_a" ] || { print_error "A 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_a" carIndex)" == "1" ] || print_error "A 记录车号应为 1，实际: ${rec_a}"
[ "$(extract_field "$rec_a" price)" == "1.00" ] || print_error "A 记录 price 应为 1.00，实际: ${rec_a}"
[ "$(extract_field "$rec_a" status)" == "1" ] || print_error "A 记录状态应为 1"
[ "$(count_field "$rec_a" '"kind":')" == "0" ] || print_error "A 未触发结算，记录应无奖，实际: ${rec_a}"
[ "$(get_combo_chain_state_field 1 remaining_tickets)" == "2" ] || print_error "A 购票后余票应为 2"
rid_b1=$(combo_chain_buy "$token_b" 1 "${name_b}_b1") || exit 1
rec_b=$(poll_combo_chain_record "$token_b" "$rid_b1" 20)
[ -n "$rec_b" ] || { print_error "B 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_b" carIndex)" == "1" ] || print_error "B 记录车号应为 1，实际: ${rec_b}"
[ "$(get_combo_chain_state_field 1 remaining_tickets)" == "1" ] || print_error "B 购票后余票应为 1"
print_step "A/B 购票通过，余票 1，等待最后一票"

# ---------- 4. 车 1 最后一票（C，预选键）：触发整车结算 ----------
print_step "4. 车 1 最后一票（C，cc_pick_key 预选幂等键）"
key_c1=$(cc_pick_key "${name_c}_c1") || { print_error "C 的预选键搜索失败"; exit $FAILED; }
print_step "key_c1: ${key_c1}（combo-winner%3==0 → 大奖得主=A；global-target(1)%2==0 → 槽2 归 B）"
rid_c1=$(combo_chain_buy "$token_c" 1 "$key_c1") || exit 1
rec_c=$(poll_combo_chain_record "$token_c" "$rid_c1" 20)
[ -n "$rec_c" ] || { print_error "C 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_c" carIndex)" == "1" ] || print_error "C 记录车号应为 1，实际: ${rec_c}"
[ "$(count_field "$rec_c" '"kind":')" == "4" ] || print_error "C 记录应含 4 条结算奖，实际: ${rec_c}"

# ---------- 5. 等待 3 笔 SAGA 收敛（支付 PAID + 报价 USED） ----------
print_step "5. 3 笔支付 PAID、报价 USED"
for r in "$rid_a1" "$rid_b1" "$rid_c1"; do
    wait_payment_paid "$r" 20 || { print_error "支付单 ${r} 未在 20s 内到达 PAID"; exit $FAILED; }
    wait_quote_status "$r" 3 20 || print_error "报价锁 ${r} 未到达 USED(3)，实际: $(get_quote_status "$r")"
done

# ---------- 6. 车 1 状态断言 ----------
print_step "6. 车 1 结算状态：车号 2 / 余票 3 / tier [0] / participants 清空"
[ "$(get_combo_chain_state_field 1 car_index)" == "2" ] || print_error "结算后车号应推进为 2，实际: $(get_combo_chain_state_field 1 car_index)"
[ "$(get_combo_chain_state_field 1 remaining_tickets)" == "3" ] || print_error "结算后余票应重置为 3，实际: $(get_combo_chain_state_field 1 remaining_tickets)"
[ "$(get_combo_chain_state_field 1 participants_json)" == "[]" ] || print_error "结算后 participants 应清空，实际: $(get_combo_chain_state_field 1 participants_json)"
[ "$(get_combo_chain_state_field 1 tier_remaining_json)" == "[0]" ] || print_error "COMBO_TIER 剩余应为 [0]，实际: $(get_combo_chain_state_field 1 tier_remaining_json)"
[ "$(get_combo_chain_state_field 1 status)" == "1" ] || print_error "车 2 未结算前状态应仍 OPEN(1)"

print_step "6.1 awards 4 条：combo→A、槽0→A、槽2→B、win→B，全挂 C 的 gid"
awards=$(get_combo_chain_state_field 1 awards_json)
[ "$(count_field "$awards" "\"request_id\":\"${rid_c1}\"")" == "4" ] || print_error "车 1 结算奖应恰 4 条挂 C 的 gid，实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_a},\"kind\":\"combo\",\"car_index\":1,\"request_id\":\"${rid_c1}\",\"prize_kind\":1,\"cash_fen\":5000")" == "1" ] || print_error "连击大奖应归 A（现金5000），实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_a},\"kind\":\"global\",\"car_index\":1,\"request_id\":\"${rid_c1}\",\"prize_kind\":1,\"cash_fen\":500")" == "1" ] || print_error "槽 0 应归 A（现金500），实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_b},\"kind\":\"global\",\"car_index\":1,\"request_id\":\"${rid_c1}\",\"prize_kind\":1,\"cash_fen\":500")" == "1" ] || print_error "槽 2 应归 B（现金500），实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_b},\"kind\":\"win_streak\",\"car_index\":1,\"request_id\":\"${rid_c1}\",\"prize_kind\":1,\"cash_fen\":300")" == "1" ] || print_error "win_streak 应归 B（现金300），实际: ${awards}"

# ---------- 7. 车 1 发放断言 ----------
print_step "7. 发放断言：4 条结算奖恰发一次（现金 6300 + 积分 0）"
[ "$(count_combo_chain_prize_ledgers "$rid_c1")" == "4" ] || print_error "C 的 gid 应恰 4 笔流水，实际: $(count_combo_chain_prize_ledgers "$rid_c1")"
[ "$(sum_combo_chain_cash_ledger "$rid_c1")" == "6300" ] || print_error "车 1 现金流水合计应 6300（5000+500+500+300），实际: $(sum_combo_chain_cash_ledger "$rid_c1")"
[ "$(sum_combo_chain_points_ledger "$rid_c1")" == "0" ] || print_error "车 1 不应有积分流水，实际: $(sum_combo_chain_points_ledger "$rid_c1")"
[ "$(count_combo_chain_prize_coupons "$rid_c1")" == "0" ] || print_error "车 1 不应发券，实际: $(count_combo_chain_prize_coupons "$rid_c1")"

# ---------- 8. 车 2：三票（档位耗尽 → 必不触发 → 确定性分布） ----------
print_step "8. 车 2：A、B、C 再各购 1 票（tier 已耗尽 → 必不触发，确定性安全上限分布）"
rid_a2=$(combo_chain_buy "$token_a" 1 "${name_a}_a2") || exit 1
poll_combo_chain_record "$token_a" "$rid_a2" 20 >/dev/null || { print_error "A 车 2 记录轮询超时"; exit $FAILED; }
rid_b2=$(combo_chain_buy "$token_b" 1 "${name_b}_b2") || exit 1
poll_combo_chain_record "$token_b" "$rid_b2" 20 >/dev/null || { print_error "B 车 2 记录轮询超时"; exit $FAILED; }
rid_c2=$(combo_chain_buy "$token_c" 1 "${name_c}_c2") || exit 1
rec_c2=$(poll_combo_chain_record "$token_c" "$rid_c2" 20)
[ -n "$rec_c2" ] || { print_error "C 车 2 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_c2" carIndex)" == "2" ] || print_error "C 记录车号应为 2，实际: ${rec_c2}"
[ "$(count_field "$rec_c2" '"kind":')" == "5" ] || print_error "C 车 2 记录应含 5 条结算奖，实际: ${rec_c2}"

print_step "8.1 终局状态：SETTLED / 余票 0 / 车号停 2"
[ "$(get_combo_chain_state_field 1 status)" == "2" ] || print_error "终局状态应 SETTLED(2)，实际: $(get_combo_chain_state_field 1 status)"
[ "$(get_combo_chain_state_field 1 remaining_tickets)" == "0" ] || print_error "终局余票应为 0，实际: $(get_combo_chain_state_field 1 remaining_tickets)"
[ "$(get_combo_chain_state_field 1 car_index)" == "2" ] || print_error "终局车号应停 2，实际: $(get_combo_chain_state_field 1 car_index)"
wait_payment_paid "$rid_c2" 20 || { print_error "C2 支付单未在 20s 内到达 PAID"; exit $FAILED; }
wait_quote_status "$rid_c2" 3 20 || print_error "C2 报价锁未到达 USED(3)，实际: $(get_quote_status "$rid_c2")"

print_step "8.2 终局 awards 9 条：车 2 槽1→A（积分）、槽3→B（券）、win A、win B、empty C"
awards=$(get_combo_chain_state_field 1 awards_json)
[ "$(count_field "$awards" "\"request_id\":\"${rid_c2}\"")" == "5" ] || print_error "车 2 结算奖应恰 5 条挂 C2 的 gid，实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_a},\"kind\":\"global\",\"car_index\":2,\"request_id\":\"${rid_c2}\",\"prize_kind\":2,\"cash_fen\":0,\"points_amount\":100")" == "1" ] || print_error "车 2 槽 1 应归 A（积分100），实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_b},\"kind\":\"global\",\"car_index\":2,\"request_id\":\"${rid_c2}\",\"prize_kind\":3,\"cash_fen\":0,\"points_amount\":0,\"coupon_template_no\":\"T-FIX-500\"")" == "1" ] || print_error "车 2 槽 3 应归 B（券 T-FIX-500），实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_a},\"kind\":\"win_streak\",\"car_index\":2,\"request_id\":\"${rid_c2}\",\"prize_kind\":1,\"cash_fen\":300")" == "1" ] || print_error "车 2 win 应归 A（现金300），实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_b},\"kind\":\"win_streak\",\"car_index\":2,\"request_id\":\"${rid_c2}\",\"prize_kind\":1,\"cash_fen\":300")" == "1" ] || print_error "车 2 win 应归 B（现金300），实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_c},\"kind\":\"empty_streak\",\"car_index\":2,\"request_id\":\"${rid_c2}\",\"prize_kind\":2,\"cash_fen\":0,\"points_amount\":100")" == "1" ] || print_error "车 2 empty 应归 C（积分100），实际: ${awards}"

# ---------- 9. 终局发放/钱包对账 ----------
print_step "9. 终局发放与钱包对账：现金 6900 / 1 券 / 积分 200 / 连击清零"
[ "$(count_combo_chain_prize_ledgers "$rid_c2")" == "4" ] || print_error "C2 的 gid 应恰 4 笔流水，实际: $(count_combo_chain_prize_ledgers "$rid_c2")"
[ "$(sum_combo_chain_cash_ledger "$rid_c2")" == "600" ] || print_error "车 2 现金流水合计应 600（300+300），实际: $(sum_combo_chain_cash_ledger "$rid_c2")"
[ "$(sum_combo_chain_points_ledger "$rid_c2")" == "200" ] || print_error "车 2 积分流水合计应 200（100+100），实际: $(sum_combo_chain_points_ledger "$rid_c2")"
[ "$(count_combo_chain_prize_coupons "$rid_c2")" == "1" ] || print_error "车 2 应恰发 1 张券，实际: $(count_combo_chain_prize_coupons "$rid_c2")"
balance=$(wait_balance "$token_a" "76.00" 20)
[ -n "$balance" ] || print_error "A 余额应 76.00（20-2+58），实际: $(get_balance "$token_a")"
balance=$(wait_balance "$token_b" "29.00" 20)
[ -n "$balance" ] || print_error "B 余额应 29.00（20-2+11），实际: $(get_balance "$token_b")"
balance=$(wait_balance "$token_c" "18.00" 20)
[ -n "$balance" ] || print_error "C 余额应 18.00（20-2），实际: $(get_balance "$token_c")"
[ "$(get_loyalty_points "$uid_b")" == "0" ] || print_error "B 积分应为 0，实际: $(get_loyalty_points "$uid_b")"
[ "$(get_loyalty_points "$uid_c")" == "100" ] || print_error "C 积分应恰 100，实际: $(get_loyalty_points "$uid_c")"
[ "$(get_loyalty_points "$uid_a")" == "100" ] || print_error "A 积分应恰 100，实际: $(get_loyalty_points "$uid_a")"
[ "$(count_user_coupons "$uid_b" "T-FIX-500")" == "1" ] || print_error "B 应持有 1 张 T-FIX-500，实际: $(count_user_coupons "$uid_b" "T-FIX-500")"
streaks_nonzero=$(do_mysql "SELECT COUNT(*) FROM gacha_play.combo_chain_streaks WHERE config_id=1 AND (empty_streak!=0 OR win_streak!=0);")
[ "$streaks_nonzero" == "0" ] || print_error "终局连击应全部清零，非零行: ${streaks_nonzero}"

# ---------- 10. HTTP 详情 ----------
print_step "10. HTTP 详情：GET /combo-chain/1 显示 SETTLED/车 2/余票 0/共享奖 3 类"
detail=$(get_combo_chain "$token_a" 1)
[ "$(extract_code "$detail")" == "0" ] || print_error "连击链详情查询失败: ${detail}"
[ "$(extract_field "$detail" status)" == "2" ] || print_error "详情 status 应为 2，实际: ${detail}"
[ "$(extract_field "$detail" carIndex)" == "2" ] || print_error "详情 carIndex 应为 2，实际: ${detail}"
[ "$(extract_field "$detail" remainingTickets)" == "0" ] || print_error "详情 remainingTickets 应为 0，实际: ${detail}"
[ "$(extract_field "$detail" totalCars)" == "2" ] || print_error "详情 totalCars 应为 2，实际: ${detail}"
[ "$(extract_field "$detail" emptyStreak)" == "0" ] || print_error "详情 emptyStreak 应为 0，实际: ${detail}"
[ "$(extract_field "$detail" winStreak)" == "0" ] || print_error "详情 winStreak 应为 0，实际: ${detail}"
[ "$(count_field "$detail" '"kind":"COMBO_TIER"')" == "1" ] || print_error "详情应含 1 条 COMBO_TIER 共享奖，实际: ${detail}"
[ "$(count_field "$detail" '"kind":"EMPTY_STREAK"')" == "1" ] || print_error "详情应含 1 条 EMPTY_STREAK 共享奖，实际: ${detail}"
[ "$(count_field "$detail" '"kind":"WIN_STREAK"')" == "1" ] || print_error "详情应含 1 条 WIN_STREAK 共享奖，实际: ${detail}"
print_step "连击链主流程断言全部通过"

# ---------- 11. 同 key 重放（车 1 C 的键）：一切计数不变 ----------
print_step "11. 同 Idempotency-Key 重放（车 1 C 的键）"
ledgers_before=$(count_combo_chain_prize_ledgers "$rid_c1")
resp=$(do_post "/combo-chain/1/buy" '{}' "$token_c" "$key_c1")
[ "$(extract_code "$resp")" == "0" ] || print_error "重放应返回成功: ${resp}"
[ "$(extract_field "$resp" status)" == "success" ] || print_error "重放 status 应为 success，实际: ${resp}"
[ "$(count_combo_chain_records "$rid_c1")" == "1" ] || print_error "重放后 C 车 1 记录应仍为 1 条"
[ "$(count_combo_chain_prize_ledgers "$rid_c1")" == "$ledgers_before" ] || print_error "重放后流水应仍为 ${ledgers_before} 笔，实际: $(count_combo_chain_prize_ledgers "$rid_c1")"
[ "$(get_combo_chain_state_field 1 status)" == "2" ] || print_error "重放后状态应仍 SETTLED(2)"
[ "$(get_combo_chain_state_field 1 car_index)" == "2" ] || print_error "重放后车号应仍为 2"
[ "$(count_field "$(get_combo_chain_state_field 1 awards_json)" "\"request_id\":\"${rid_c1}\"")" == "4" ] || print_error "重放后 awards 应仍 4 条挂 C 的 gid"
print_step "重放幂等通过：记录/流水/状态/awards 均不变"

# ---------- 12. 现场复位 + 注入参与者错配 → 05 Settle Aborted(3031) ----------
print_step "12. SQL 复位 + 注入参与者错配（幽灵占位 → 结算时 4≠3）"
reset_combo_chain_state 1
[ "$(get_combo_chain_state_field 1 remaining_tickets)" == "3" ] || print_error "复位后余票应为 3"
[ "$(get_combo_chain_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
rid_a3=$(combo_chain_buy "$token_a" 1 "${name_a}_a3") || exit 1
poll_combo_chain_record "$token_a" "$rid_a3" 20 >/dev/null || { print_error "A 预热购票记录轮询超时"; exit $FAILED; }
rid_b3=$(combo_chain_buy "$token_b" 1 "${name_b}_b3") || exit 1
poll_combo_chain_record "$token_b" "$rid_b3" 20 >/dev/null || { print_error "B 预热购票记录轮询超时"; exit $FAILED; }
do_mysql "UPDATE gacha_play.combo_chain_state SET participants_json='[${uid_a},${uid_b},999999]' WHERE config_id=1;"
[ "$(get_combo_chain_state_field 1 participants_json)" == "[${uid_a},${uid_b},999999]" ] || print_error "注入后 participants 应为 [${uid_a},${uid_b},999999]"

print_step "12.1 C 新 key 购票：03 锁定成功 → 05 Settle Aborted(3031 配置变更)"
rid_c3=$(combo_chain_buy "$token_c" 1 "${name_c}_c3") || exit 1
print_step "requestId: ${rid_c3}（SettleComboChain 将 Aborted，补偿链收敛）"
wait_barrier_compensated "$rid_c3" 20 || print_error "错配 SAGA 补偿链未在 20s 内收敛"

print_step "12.2 补偿链断言：quote RELEASED / 支付 CANCELED / 无记录 / 参与者余票复原"
wait_quote_status "$rid_c3" 4 20 || print_error "补偿后报价应 RELEASED(4)，实际: $(get_quote_status "$rid_c3")"
wait_payment_status "$rid_c3" 6 20 || print_error "补偿后支付单应 CANCELED(6)，实际: $(get_payment_status "$rid_c3")"
[ "$(count_combo_chain_records "$rid_c3")" == "0" ] || print_error "错配结算不应产生记录（事务回滚），实际: $(count_combo_chain_records "$rid_c3")"
[ "$(get_combo_chain_state_field 1 participants_json)" == "[${uid_a},${uid_b},999999]" ] || print_error "补偿后参与者应复原（C 弹出），实际: $(get_combo_chain_state_field 1 participants_json)"
[ "$(get_combo_chain_state_field 1 remaining_tickets)" == "1" ] || print_error "补偿后余票应复原为 1，实际: $(get_combo_chain_state_field 1 remaining_tickets)"
[ "$(get_combo_chain_state_field 1 status)" == "1" ] || print_error "补偿后状态应 OPEN(1)"
[ "$(get_combo_chain_state_field 1 tier_remaining_json)" == "[1]" ] || print_error "补偿后档位库存应仍 [1]，实际: $(get_combo_chain_state_field 1 tier_remaining_json)"
balance=$(wait_balance "$token_c" "18.00" 5)
[ -n "$balance" ] || print_error "补偿后 C 余额应仍为 18.00，实际: $(get_balance "$token_c")"
print_step "错配补偿链通过：参与者/余票/报价/支付/余额 全回滚"

# ---------- 13. 收尾：现场复位（保持测试环境整洁，run_all 二轮可重复执行） ----------
print_step "13. 收尾：连击链/拼团现场复位"
reset_combo_chain_state 1
reset_group_buy_state 1
[ "$(get_combo_chain_state_field 1 car_index)" == "1" ] || print_error "复位后车号应为 1"
[ "$(get_combo_chain_state_field 1 remaining_tickets)" == "3" ] || print_error "复位后余票应为 3"
[ "$(get_combo_chain_state_field 1 participants_json)" == "[]" ] || print_error "复位后 participants 应为空"
[ "$(get_combo_chain_state_field 1 tier_remaining_json)" == "[1]" ] || print_error "复位后档位库存应为 [1]"
[ "$(get_combo_chain_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
[ "$(get_group_buy_state_field 1 remaining_bets)" == "3" ] || print_error "复位后拼团余位应为 3"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景22 combo_chain 通过${NC}"
else
    echo -e "${RED}× 场景22 combo_chain 失败${NC}"
fi
exit $FAILED
