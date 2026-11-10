#!/bin/bash
# ============================================================
# 场景23：拼团抽（group_buy，M13 全流程 + 幂等重放 + 配置变更补偿）
#
# 种子：1 号拼团 100 分/注、3 注成团；条目 5 槽（谢谢×2、金币×2、皮肤×1）
#       + 皮肤子奖 积分100（随皮肤父条目，entry_id=4）+ 幸运奖 现金1000×1 +
#       幸运子奖 积分50×1（lucky_index=0）
# 流程：注册 A/B/C 各充值 20 元 → A/B/C 各下 1 注（最后一注用 gb_pick_key 预选
#       幂等键：lucky-pool 洗牌后 pool[0]=A → 幸运奖+幸运子奖归 A）→ 成团结算：
#       5 普通槽按确定性洗牌分给 3 注（每注 ≥1）、皮肤子奖随皮肤槽发放（得主动态）、
#       lucky A 现金1000 + lucky child A 积分50 → SETTLED、lucky_sent [1]、
#       条目库存清零 → 钱包/积分对账（A 29.00、B/C 19.00；皮肤得主另 +100 积分）
#       → 同 key 重放一切计数不变 → SQL 复位后注入 lucky_sent 长度错配
#       → C 新 key → 05 Settle Aborted(3034) → 补偿链：quote RELEASED、支付
#         CANCELED、无记录、下注弹出余位复原 → 现场复位
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景23：拼团抽全流程与补偿（group_buy）"

# ---------- 1. 注册 3 用户 + 各充值 20 元 ----------
print_step "1. 注册 3 用户（A/B/C）、各充值 20 元"
reg_a=$(register_user "e2e_gb_a_") || exit 1
uid_a=$(echo "$reg_a" | cut -d' ' -f1)
name_a=$(echo "$reg_a" | cut -d' ' -f2)
reg_b=$(register_user "e2e_gb_b_") || exit 1
uid_b=$(echo "$reg_b" | cut -d' ' -f1)
name_b=$(echo "$reg_b" | cut -d' ' -f2)
reg_c=$(register_user "e2e_gb_c_") || exit 1
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

# ---------- 2. SQL 复位拼团现场 ----------
print_step "2. SQL 复位拼团现场（余位 3 / OPEN / lucky_sent [0]）"
reset_group_buy_state 1
[ "$(get_group_buy_state_field 1 bets_json)" == "[]" ] || print_error "复位后 bets 应为空"
[ "$(get_group_buy_state_field 1 awards_json)" == "[]" ] || print_error "复位后 awards 应为空"
[ "$(get_group_buy_state_field 1 lucky_sent_json)" == "[0]" ] || print_error "复位后 lucky_sent 应为 [0]，实际: $(get_group_buy_state_field 1 lucky_sent_json)"
[ "$(get_group_buy_state_field 1 remaining_bets)" == "3" ] || print_error "复位后余位应为 3"
[ "$(get_group_buy_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"

# ---------- 3. A、B 各下 1 注（未成团不结算） ----------
print_step "3. A、B 各下 1 注（余位 3→2→1，不触发结算）"
rid_a=$(group_buy_buy "$token_a" 1 1 "${name_a}_a1") || exit 1
rec_a=$(poll_group_buy_record "$token_a" "$rid_a" 20)
[ -n "$rec_a" ] || { print_error "A 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_a" count)" == "1" ] || print_error "A 应下 1 注，实际: ${rec_a}"
[ "$(extract_field "$rec_a" price)" == "1.00" ] || print_error "A 记录 price 应为 1.00，实际: ${rec_a}"
[ "$(extract_field "$rec_a" status)" == "1" ] || print_error "A 记录状态应为 1"
[ "$(count_field "$rec_a" '"kind":')" == "0" ] || print_error "A 未触发结算，记录应无奖，实际: ${rec_a}"
[ "$(get_group_buy_state_field 1 remaining_bets)" == "2" ] || print_error "A 下注后余位应为 2"
rid_b=$(group_buy_buy "$token_b" 1 1 "${name_b}_b1") || exit 1
rec_b=$(poll_group_buy_record "$token_b" "$rid_b" 20)
[ -n "$rec_b" ] || { print_error "B 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_b" count)" == "1" ] || print_error "B 应下 1 注，实际: ${rec_b}"
[ "$(get_group_buy_state_field 1 remaining_bets)" == "1" ] || print_error "B 下注后余位应为 1"
print_step "A/B 下注通过，余位 1，等待最后一注"

# ---------- 4. 最后一注（C，预选键）：触发成团结算 ----------
print_step "4. 最后一注（C，gb_pick_key 预选幂等键）"
key_c=$(gb_pick_key "${name_c}_c1") || { print_error "C 的预选键搜索失败"; exit $FAILED; }
print_step "key_c: ${key_c}（lucky-pool(2)%3!=0 且 (1)%2==1 → 洗牌后 pool[0]=A，幸运奖归 A）"
rid_c=$(group_buy_buy "$token_c" 1 1 "$key_c") || exit 1
rec_c=$(poll_group_buy_record "$token_c" "$rid_c" 20)
[ -n "$rec_c" ] || { print_error "C 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_c" count)" == "1" ] || print_error "C 应下 1 注，实际: ${rec_c}"
award_items=$(count_field "$rec_c" '"kind":')
[ "$award_items" -ge 7 ] && [ "$award_items" -le 8 ] || print_error "C 记录应含 7-8 条结算奖（5 普通+幸运+幸运子奖+皮肤子奖0/1），实际: ${award_items}"

# ---------- 5. 等待 3 笔 SAGA 收敛（支付 PAID + 报价 USED） ----------
print_step "5. 3 笔支付 PAID、报价 USED"
for r in "$rid_a" "$rid_b" "$rid_c"; do
    wait_payment_paid "$r" 20 || { print_error "支付单 ${r} 未在 20s 内到达 PAID"; exit $FAILED; }
    wait_quote_status "$r" 3 20 || print_error "报价锁 ${r} 未到达 USED(3)，实际: $(get_quote_status "$r")"
done

# ---------- 6. 状态断言 ----------
print_step "6. 成团状态：SETTLED / 余位 0 / bets 3 注（保留）/ lucky_sent [1]"
[ "$(get_group_buy_state_field 1 status)" == "2" ] || print_error "终局状态应 SETTLED(2)，实际: $(get_group_buy_state_field 1 status)"
[ "$(get_group_buy_state_field 1 remaining_bets)" == "0" ] || print_error "终局余位应为 0，实际: $(get_group_buy_state_field 1 remaining_bets)"
[ "$(get_group_buy_state_field 1 lucky_sent_json)" == "[1]" ] || print_error "终局 lucky_sent 应为 [1]，实际: $(get_group_buy_state_field 1 lucky_sent_json)"
bets=$(get_group_buy_state_field 1 bets_json)
[ "$(count_field "$bets" '"user_id":')" == "3" ] || print_error "bets 应保留 3 注，实际: ${bets}"
[ "$(count_field "$bets" "\"user_id\":${uid_a},\"draw_id\":\"${rid_a}\",\"count\":1")" == "1" ] || print_error "bets 应含 A 的注，实际: ${bets}"
[ "$(count_field "$bets" "\"user_id\":${uid_b},\"draw_id\":\"${rid_b}\",\"count\":1")" == "1" ] || print_error "bets 应含 B 的注，实际: ${bets}"
[ "$(count_field "$bets" "\"user_id\":${uid_c},\"draw_id\":\"${rid_c}\",\"count\":1")" == "1" ] || print_error "bets 应含 C 的注，实际: ${bets}"

print_step "6.1 awards：5 普通槽（每注 ≥1）+ 幸运/幸运子奖归 A + 皮肤子奖随皮肤槽"
awards=$(get_group_buy_state_field 1 awards_json)
skins=$(count_field "$awards" '"kind":"normal","entry_id":3')
award_total=$((7 + skins))
[ "$(count_field "$awards" "\"request_id\":\"${rid_c}\"")" == "$award_total" ] || print_error "结算奖应恰 ${award_total} 条挂 C 的 gid，实际: ${awards}"
[ "$(count_field "$awards" '"kind":"normal"')" == "5" ] || print_error "普通槽应恰 5 条，实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_a},\"kind\":\"normal\"")" -ge 1 ] || print_error "A 应至少分到 1 槽，实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_b},\"kind\":\"normal\"")" -ge 1 ] || print_error "B 应至少分到 1 槽，实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_c},\"kind\":\"normal\"")" -ge 1 ] || print_error "C 应至少分到 1 槽，实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_a},\"kind\":\"lucky\",\"request_id\":\"${rid_c}\",\"prize_kind\":1,\"cash_fen\":1000")" == "1" ] || print_error "幸运奖应归 A（现金1000），实际: ${awards}"
[ "$(count_field "$awards" "\"user_id\":${uid_a},\"kind\":\"child\",\"request_id\":\"${rid_c}\",\"prize_kind\":2,\"cash_fen\":0,\"points_amount\":50")" == "1" ] || print_error "幸运子奖应归 A（积分50），实际: ${awards}"
[ "$(count_field "$awards" '"kind":"child","entry_id":4')" == "$skins" ] || print_error "皮肤子奖应恰 ${skins} 条（随皮肤槽发放），实际: ${awards}"
[ "$(count_field "$awards" '"kind":"child"')" == "$((1 + skins))" ] || print_error "子奖总数应 $((1 + skins))（幸运子奖 1 + 皮肤子奖 ${skins}），实际: ${awards}"
print_step "皮肤槽被抽中 ${skins} 次，皮肤子奖 ${skins} 条"

# ---------- 7. 库存推进断言 ----------
print_step "7. 条目库存推进：5 槽全分完 → 谢谢/金币/皮肤清零；皮肤子奖余量 1-皮肤次数"
[ "$(do_mysql "SELECT remaining FROM gacha_play.group_buy_entries WHERE id=1;")" == "0" ] || print_error "谢谢参与剩余应 0"
[ "$(do_mysql "SELECT remaining FROM gacha_play.group_buy_entries WHERE id=2;")" == "0" ] || print_error "金币剩余应 0"
[ "$(do_mysql "SELECT remaining FROM gacha_play.group_buy_entries WHERE id=3;")" == "0" ] || print_error "限定皮肤剩余应 0"
[ "$(do_mysql "SELECT remaining FROM gacha_play.group_buy_entries WHERE id=4;")" == "$((1 - skins))" ] || print_error "皮肤碎片剩余应 $((1 - skins))，实际: $(do_mysql "SELECT remaining FROM gacha_play.group_buy_entries WHERE id=4;")"

# ---------- 8. 发放断言 ----------
print_step "8. 发放断言：幸运/幸运子奖/皮肤子奖恰发一次（现金 1000 + 积分 50+100×皮肤次数）"
[ "$(count_group_buy_prize_ledgers "$rid_c")" == "$((2 + skins))" ] || print_error "C 的 gid 应恰 $((2 + skins)) 笔流水，实际: $(count_group_buy_prize_ledgers "$rid_c")"
[ "$(sum_group_buy_cash_ledger "$rid_c")" == "1000" ] || print_error "现金流水合计应 1000（幸运奖），实际: $(sum_group_buy_cash_ledger "$rid_c")"
[ "$(sum_group_buy_points_ledger "$rid_c")" == "$((50 + 100 * skins))" ] || print_error "积分流水合计应 $((50 + 100 * skins))，实际: $(sum_group_buy_points_ledger "$rid_c")"
[ "$(count_group_buy_prize_coupons "$rid_c")" == "0" ] || print_error "不应发券，实际: $(count_group_buy_prize_coupons "$rid_c")"

# ---------- 9. 钱包对账 ----------
print_step "9. 钱包对账：A 29.00、B/C 19.00；积分合计 50+100×皮肤次数"
balance=$(wait_balance "$token_a" "29.00" 20)
[ -n "$balance" ] || print_error "A 余额应 29.00（20-1+10），实际: $(get_balance "$token_a")"
balance=$(wait_balance "$token_b" "19.00" 20)
[ -n "$balance" ] || print_error "B 余额应 19.00（20-1），实际: $(get_balance "$token_b")"
balance=$(wait_balance "$token_c" "19.00" 20)
[ -n "$balance" ] || print_error "C 余额应 19.00（20-1），实际: $(get_balance "$token_c")"
points_total=$(( $(get_loyalty_points "$uid_a") + $(get_loyalty_points "$uid_b") + $(get_loyalty_points "$uid_c") ))
[ "$points_total" == "$((50 + 100 * skins))" ] || print_error "三人积分合计应 $((50 + 100 * skins))，实际: ${points_total}（A=$(get_loyalty_points "$uid_a") B=$(get_loyalty_points "$uid_b") C=$(get_loyalty_points "$uid_c")）"
[ "$(get_loyalty_points "$uid_a")" -ge 50 ] || print_error "A 积分应 ≥50（幸运子奖），实际: $(get_loyalty_points "$uid_a")"

# ---------- 10. HTTP 详情 ----------
print_step "10. HTTP 详情：GET /group-buy/1 显示 SETTLED/余位 0/幸运奖已发 1"
detail=$(get_group_buy "$token_a" 1)
[ "$(extract_code "$detail")" == "0" ] || print_error "拼团详情查询失败: ${detail}"
[ "$(extract_field "$detail" status)" == "2" ] || print_error "详情 status 应为 2，实际: ${detail}"
[ "$(extract_field "$detail" remainingBets)" == "0" ] || print_error "详情 remainingBets 应为 0，实际: ${detail}"
[ "$(extract_field "$detail" targetBetCount)" == "3" ] || print_error "详情 targetBetCount 应为 3，实际: ${detail}"
[ "$(extract_field "$detail" participants)" == "3" ] || print_error "详情 participants 应为 3，实际: ${detail}"
[ "$(count_field "$detail" '"prizeIndex":0')" == "1" ] || print_error "详情应含 1 条幸运奖，实际: ${detail}"
[ "$(count_field "$detail" '"sent":1')" == "1" ] || print_error "详情幸运奖 sent 应为 1，实际: ${detail}"
print_step "拼团主流程断言全部通过"

# ---------- 11. 同 key 重放（C 的键）：一切计数不变 ----------
print_step "11. 同 Idempotency-Key 重放（C 的键）"
ledgers_before=$(count_group_buy_prize_ledgers "$rid_c")
resp=$(do_post "/group-buy/1/buy" '{"count":1}' "$token_c" "$key_c")
[ "$(extract_code "$resp")" == "0" ] || print_error "重放应返回成功: ${resp}"
[ "$(extract_field "$resp" status)" == "success" ] || print_error "重放 status 应为 success，实际: ${resp}"
[ "$(count_group_buy_records "$rid_c")" == "1" ] || print_error "重放后 C 记录应仍为 1 条"
[ "$(count_group_buy_prize_ledgers "$rid_c")" == "$ledgers_before" ] || print_error "重放后流水应仍为 ${ledgers_before} 笔，实际: $(count_group_buy_prize_ledgers "$rid_c")"
[ "$(get_group_buy_state_field 1 status)" == "2" ] || print_error "重放后状态应仍 SETTLED(2)"
[ "$(count_field "$(get_group_buy_state_field 1 awards_json)" "\"request_id\":\"${rid_c}\"")" == "$award_total" ] || print_error "重放后 awards 应仍 ${award_total} 条挂 C 的 gid"
print_step "重放幂等通过：记录/流水/状态/awards 均不变"

# ---------- 12. 现场复位 + 注入 lucky_sent 长度错配 → 05 Settle Aborted(3034) ----------
print_step "12. SQL 复位 + 注入 lucky_sent 长度错配（[] vs 配置 1 条）"
reset_group_buy_state 1
[ "$(get_group_buy_state_field 1 remaining_bets)" == "3" ] || print_error "复位后余位应为 3"
[ "$(get_group_buy_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
rid_a2=$(group_buy_buy "$token_a" 1 1 "${name_a}_a2") || exit 1
poll_group_buy_record "$token_a" "$rid_a2" 20 >/dev/null || { print_error "A 预热下注记录轮询超时"; exit $FAILED; }
rid_b2=$(group_buy_buy "$token_b" 1 1 "${name_b}_b2") || exit 1
poll_group_buy_record "$token_b" "$rid_b2" 20 >/dev/null || { print_error "B 预热下注记录轮询超时"; exit $FAILED; }
do_mysql "UPDATE gacha_play.group_buy_state SET lucky_sent_json='[]' WHERE config_id=1;"
[ "$(get_group_buy_state_field 1 lucky_sent_json)" == "[]" ] || print_error "注入后 lucky_sent 应为 []"

print_step "12.1 C 新 key 下注：03 锁定成功 → 05 Settle Aborted(3034 配置变更)"
rid_c2=$(group_buy_buy "$token_c" 1 1 "${name_c}_c2") || exit 1
print_step "requestId: ${rid_c2}（SettleGroupBuy 将 Aborted，补偿链收敛）"
wait_barrier_compensated "$rid_c2" 20 || print_error "错配 SAGA 补偿链未在 20s 内收敛"

print_step "12.2 补偿链断言：quote RELEASED / 支付 CANCELED / 无记录 / 下注弹出余位复原"
wait_quote_status "$rid_c2" 4 20 || print_error "补偿后报价应 RELEASED(4)，实际: $(get_quote_status "$rid_c2")"
wait_payment_status "$rid_c2" 6 20 || print_error "补偿后支付单应 CANCELED(6)，实际: $(get_payment_status "$rid_c2")"
[ "$(count_group_buy_records "$rid_c2")" == "0" ] || print_error "错配结算不应产生记录（事务回滚），实际: $(count_group_buy_records "$rid_c2")"
bets2=$(get_group_buy_state_field 1 bets_json)
[ "$(count_field "$bets2" '"user_id":')" == "2" ] || print_error "补偿后 bets 应弹回 2 注，实际: ${bets2}"
[ "$(count_field "$bets2" "\"draw_id\":\"${rid_c2}\"")" == "0" ] || print_error "补偿后 C 的注应已弹出，实际: ${bets2}"
[ "$(get_group_buy_state_field 1 remaining_bets)" == "1" ] || print_error "补偿后余位应复原为 1，实际: $(get_group_buy_state_field 1 remaining_bets)"
[ "$(get_group_buy_state_field 1 status)" == "1" ] || print_error "补偿后状态应 OPEN(1)"
balance=$(wait_balance "$token_c" "19.00" 5)
[ -n "$balance" ] || print_error "补偿后 C 余额应仍为 19.00，实际: $(get_balance "$token_c")"
print_step "错配补偿链通过：下注/余位/报价/支付/余额 全回滚"

# ---------- 13. 收尾：现场复位（保持测试环境整洁，run_all 二轮可重复执行） ----------
print_step "13. 收尾：拼团/连击链现场复位"
reset_group_buy_state 1
reset_combo_chain_state 1
[ "$(get_group_buy_state_field 1 bets_json)" == "[]" ] || print_error "复位后 bets 应为空"
[ "$(get_group_buy_state_field 1 awards_json)" == "[]" ] || print_error "复位后 awards 应为空"
[ "$(get_group_buy_state_field 1 lucky_sent_json)" == "[0]" ] || print_error "复位后 lucky_sent 应为 [0]"
[ "$(get_group_buy_state_field 1 remaining_bets)" == "3" ] || print_error "复位后余位应为 3"
[ "$(get_group_buy_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
[ "$(do_mysql "SELECT remaining FROM gacha_play.group_buy_entries WHERE id=4;")" == "1" ] || print_error "复位后皮肤碎片剩余应 1"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景23 group_buy 通过${NC}"
else
    echo -e "${RED}× 场景23 group_buy 失败${NC}"
fi
exit $FAILED
