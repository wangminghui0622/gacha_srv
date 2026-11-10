#!/bin/bash
# ============================================================
# 场景21：扭蛋幸运指派（gashapon LUCKY assignee，M12c）
#
# 种子：1 号扭蛋 6 票；LUCKY 现金1000×2（stage1，prize_index=2）
# 流程：注册 U/A 各充值 20 元 → SQL 复位现场 + 插指派（LUCKY idx2 → U）
#       → U 抽 1（票 1）、A 抽 2（票 2-3）→ A 的 settle drawn=3 ≥ T0=3 触发
#         LUCKY 单元0：指派者 U 触及票据区间 [1..3] → 无哈希判定直接 U 中
#       → 断言 sent=[0,0,1]、awards 恰 1 条（unit0 → U，现金1000，挂 A 的 gid）、
#         钱包流水 1 笔（c_gcp_<gid_A>#% 归 U 1000）、U 余额 29.00、A 18.00、
#         余票 3、状态 OPEN（未清空不终局）
#       → SQL 清指派 → 现场复位
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景21：扭蛋幸运指派（gashapon LUCKY assignee）"

# ---------- 1. 注册 2 用户 + 各充值 20 元 ----------
print_step "1. 注册 2 用户（U/A）、各充值 20 元"
reg_u=$(register_user "e2e_gu_u_") || exit 1
uid_u=$(echo "$reg_u" | cut -d' ' -f1)
name_u=$(echo "$reg_u" | cut -d' ' -f2)
reg_a=$(register_user "e2e_gu_a_") || exit 1
uid_a=$(echo "$reg_a" | cut -d' ' -f1)
name_a=$(echo "$reg_a" | cut -d' ' -f2)
token_u=$(login_by_name "$name_u") || exit 1
token_a=$(login_by_name "$name_a") || exit 1
for t in "$token_u" "$token_a"; do
    resp=$(do_post /user/recharge '{"amount":"20"}' "$t")
    [ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
done
[ "$(fen_of "$(get_balance "$token_u")")" == "2000" ] || print_error "U 充值后余额应为 20.00"

# ---------- 2. SQL 复位现场 + 插幸运指派（LUCKY idx2 → U） ----------
print_step "2. SQL 复位扭蛋现场 + 指派 LUCKY（prize_index=2 → U）"
reset_gashapon_state 1
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "6" ] || print_error "复位后余票应为 6，实际: $(get_gashapon_state_field 1 remaining_tickets)"
[ "$(get_gashapon_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
set_lucky_assignment 1 2 "$uid_u"
[ "$(count_lucky_assignments 1)" == "1" ] || print_error "指派后应有恰 1 行幸运指派，实际: $(count_lucky_assignments 1)"

# ---------- 3. U 抽 1（票 1）、A 抽 2（票 2-3）触发 LUCKY 单元0 判定 ----------
print_step "3. U 抽 1（票 1）、A 抽 2（票 2-3）：drawn=3 触发 LUCKY 单元0，指派者 U 触及 [1..3]"
rid_u=$(gashapon_buy "$token_u" 1 1 "$name_u") || exit 1
rec_u=$(poll_gashapon_record "$token_u" "$rid_u" 20)
[ -n "$rec_u" ] || { print_error "U 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_u" count)" == "1" ] || print_error "U 应抽 1 次，实际: ${rec_u}"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,0]" ] || print_error "U 抽后 sent 应仍 [0,0,0]（drawn=1 无判定），实际: $(get_gashapon_state_field 1 sent_json)"

rid_a=$(gashapon_buy "$token_a" 1 2 "$name_a") || exit 1
rec_a=$(poll_gashapon_record "$token_a" "$rid_a" 20)
[ -n "$rec_a" ] || { print_error "A 记录轮询超时"; exit $FAILED; }
[ "$(extract_field "$rec_a" count)" == "2" ] || print_error "A 应抽 2 次，实际: ${rec_a}"
print_step "requestId(A): ${rid_a}（其 settle 触发指派判定）"

# ---------- 4. 等待 2 笔 SAGA 收敛 ----------
print_step "4. 2 笔支付 PAID、报价 USED"
for r in "$rid_u" "$rid_a"; do
    wait_payment_paid "$r" 20 || { print_error "支付单 ${r} 未在 20s 内到达 PAID"; exit $FAILED; }
    wait_quote_status "$r" 3 20 || print_error "报价锁 ${r} 未到达 USED(3)，实际: $(get_quote_status "$r")"
done

# ---------- 5. 状态与指派断言 ----------
print_step "5. 状态断言：sent=[0,0,1] / 余票 3 / OPEN（未清空不终局）"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,1]" ] || print_error "sent 应为 [0,0,1]，实际: $(get_gashapon_state_field 1 sent_json)"
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "3" ] || print_error "余票应为 3，实际: $(get_gashapon_state_field 1 remaining_tickets)"
[ "$(get_gashapon_state_field 1 status)" == "1" ] || print_error "状态应仍 OPEN(1)，实际: $(get_gashapon_state_field 1 status)"
parts=$(get_gashapon_state_field 1 participants_json)
[ "$(count_field "$parts" "\"${uid_u}\":1")" == "1" ] || print_error "participants 应含 U:1，实际: ${parts}"
[ "$(count_field "$parts" "\"${uid_a}\":2")" == "1" ] || print_error "participants 应含 A:2，实际: ${parts}"

print_step "5.1 awards 恰 1 条：LUCKY 单元0 → U（现金1000），挂 A 的 gid"
awards_json=$(get_gashapon_state_field 1 awards_json)
[ "$(count_field "$awards_json" '"special_kind":"LUCKY"')" == "1" ] || print_error "awards 应恰 1 条 LUCKY，实际: ${awards_json}"
[ "$(count_field "$awards_json" "\"unit\":0,\"user_id\":${uid_u},\"special_kind\":\"LUCKY\",\"prize_kind\":1,\"cash_fen\":1000")" == "1" ] || print_error "LUCKY 单元0 应归 U（现金1000），实际: ${awards_json}"
[ "$(count_field "$awards_json" "\"request_id\":\"${rid_a}\"")" == "1" ] || print_error "指派奖应挂 A 的 gid（A 的 settle 创建），实际: ${awards_json}"
[ "$(count_field "$awards_json" "\"user_id\":${uid_a}")" == "0" ] || print_error "awards 不应有归 A 的奖，实际: ${awards_json}"

# ---------- 6. 发放断言：流水归 U 恰 1 笔 ----------
print_step "6. 发放断言：c_gcp_<gid_A>#% 恰 1 笔现金 1000 归 U、无券"
[ "$(count_gashapon_prize_ledgers "$rid_a")" == "1" ] || print_error "A 的 gid 应恰 1 笔流水，实际: $(count_gashapon_prize_ledgers "$rid_a")"
[ "$(sum_gashapon_cash_ledger "$rid_a")" == "1000" ] || print_error "A 的 gid 现金流水合计应 1000，实际: $(sum_gashapon_cash_ledger "$rid_a")"
[ "$(count_gashapon_prize_coupons "$rid_a")" == "0" ] || print_error "指派奖为现金，不应有发奖券"
balance=$(wait_balance "$token_u" "29.00" 20)
[ -n "$balance" ] || print_error "U 余额应 29.00（20-1+10），实际: $(get_balance "$token_u")"
balance=$(wait_balance "$token_a" "18.00" 20)
[ -n "$balance" ] || print_error "A 余额应 18.00（20-2），实际: $(get_balance "$token_a")"
print_step "幸运指派全流程断言通过"

# ---------- 7. SQL 清指派（无副作用校验） ----------
print_step "7. SQL 清指派（user_id=0 删除）"
set_lucky_assignment 1 2 0
[ "$(count_lucky_assignments 1)" == "0" ] || print_error "清指派后应无指派行，实际: $(count_lucky_assignments 1)"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,1]" ] || print_error "清指派不应影响已发 sent"

# ---------- 8. 收尾：现场复位 ----------
print_step "8. 收尾：扭蛋现场复位为 OPEN 全票"
reset_gashapon_state 1
[ "$(get_gashapon_state_field 1 remaining_tickets)" == "6" ] || print_error "复位后余票应为 6"
[ "$(get_gashapon_state_field 1 status)" == "1" ] || print_error "复位后状态应为 OPEN(1)"
[ "$(get_gashapon_state_field 1 participants_json)" == "{}" ] || print_error "复位后 participants 应为空"
[ "$(get_gashapon_state_field 1 records_json)" == "[]" ] || print_error "复位后 records 应为空"
[ "$(get_gashapon_state_field 1 sent_json)" == "[0,0,0]" ] || print_error "复位后 sent 应为 [0,0,0]"
[ "$(get_gashapon_state_field 1 awards_json)" == "[]" ] || print_error "复位后 awards 应为空"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景21 lucky_assign 通过${NC}"
else
    echo -e "${RED}× 场景21 lucky_assign 失败${NC}"
fi
exit $FAILED
