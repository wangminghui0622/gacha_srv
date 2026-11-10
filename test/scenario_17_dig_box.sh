#!/bin/bash
# ============================================================
# 场景17：挖宝箱（dig_box，M11b 全流程 + 幂等 + 会话回收 + 格冲突补偿）
#
# 种子：1 号箱 9 格 100 分/格；格 0 现金100分、格 1 积分50、格 2 券 T-FIX-500、
#       格 3 空、格 4 现金500分、格 5 积分100、格 6 空、格 7 券 T-PCT-5000、格 8 现金1000分
# 流程：注册 → 充值 20 元 → 报价全开（9 格 9.00）→ 全开购买（SAGA 7 分支）
#       → 记录 9 格 price=9.00、现金流水恰 3 笔共 1600 分、积分 +150、
#         券恰 2 张、支付 PAID、报价 USED、bitmap=511、会话 COMPLETED、余额 27.00
#       → 同 Idempotency-Key 重放：success 且一切计数不变
#       → 二次全开（新 key）：会话回收 → 余额 34.00、积分 300、券再 +2
#       → SQL 预置已挖格 0（PLAYING）→ 买格 0 Aborted 3016 → 空补偿收敛：
#         会话原样不动（预置位不误回滚）、报价回 OPEN（自然过期）、无支付单、
#         无记录、余额不变
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景17：挖宝箱全流程与补偿（dig_box）"

# ---------- 1. 注册 + 充值 20 元 ----------
print_step "1. 注册用户、充值 20 元"
reg=$(register_user "e2e_dig_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge '{"amount":"20"}' "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
[ "$(fen_of "$(get_balance "$token")")" == "2000" ] || print_error "充值后余额应为 20.00"

# ---------- 2. 报价预览（全开 9 格） ----------
print_step "2. 报价预览：takeAll 全开 9 格"
quote=$(dig_quote "$token" 1 "" 1)
[ "$(extract_code "$quote")" == "0" ] || { print_error "报价失败: ${quote}"; exit 1; }
[ "$(extract_field "$quote" count)" == "9" ] || print_error "全开 count 应为 9，实际: ${quote}"
[ "$(extract_field "$quote" gross)" == "9.00" ] || print_error "全开 gross 应为 9.00，实际: ${quote}"
[ "$(extract_field "$quote" payable)" == "9.00" ] || print_error "全开 payable 应为 9.00，实际: ${quote}"
print_step "报价通过：9 格 × 1.00 = 9.00"

# ---------- 3. 全开购买（SAGA 7 分支异步执行） ----------
print_step "3. 全开购买（Idempotency-Key=${username}）"
key1="${username}"
rid1=$(dig_buy "$token" 1 "" 1 "$key1") || exit 1
[ -n "$rid1" ] || { print_error "requestId 不应为空"; exit $FAILED; }
print_step "requestId: ${rid1}"
record=$(poll_dig_record "$token" "$rid1" 20)
[ -n "$record" ] || { print_error "轮询超时：挖宝记录未产生"; exit $FAILED; }
price=$(extract_field "$record" price)
[ "$price" == "9.00" ] || print_error "记录 price 应为 9.00，实际: ${price}"
# cellsChosen 是逗号分隔 CSV，extract_field 会在首个逗号截断，需按引号串取
cells=$(echo "$record" | sed -n 's/.*"cellsChosen":"\([^"]*\)".*/\1/p')
commas=$(echo "$cells" | tr -cd ',' | wc -c)
[ "$commas" == "8" ] || print_error "记录应含 9 格，实际 cellsChosen: ${cells}"
status=$(extract_field "$record" status)
[ "$status" == "1" ] || print_error "记录状态应为 1(正常)，实际: ${status}"
print_step "记录：price=9.00, 9 格, status=1"

# ---------- 4. 支付/报价/会话/钱包/奖品断言 ----------
print_step "4. 支付 PAID、报价 USED、会话满格 COMPLETED"
wait_payment_paid "$rid1" 20 || { print_error "支付单未在 20s 内到达 PAID"; exit $FAILED; }
wait_quote_status "$rid1" 3 20 || print_error "报价锁未到达 USED(3)，实际: $(get_quote_status "$rid1")"
[ "$(get_dig_session_field "$user_id" 1 occupied_bitmap)" == "511" ] || print_error "bitmap 应为 511(9 格全挖)，实际: $(get_dig_session_field "$user_id" 1 occupied_bitmap)"
[ "$(get_dig_session_field "$user_id" 1 total_cells_paid)" == "9" ] || print_error "累计已购应为 9 格"
[ "$(get_dig_session_field "$user_id" 1 total_price_fen)" == "900" ] || print_error "累计实付应为 900 分"
[ "$(get_dig_session_field "$user_id" 1 status)" == "2" ] || print_error "会话应为 COMPLETED(2)，实际: $(get_dig_session_field "$user_id" 1 status)"

print_step "4.2 奖品恰发一次（现金 3 笔共 1600 分 / 积分 +150 / 券 2 张）"
balance=$(wait_balance "$token" "27.00" 20)
[ -n "$balance" ] || print_error "余额应为 27.00（20-9+16），实际: $(get_balance "$token")"
points=$(get_loyalty_points "$user_id")
[ "$points" == "150" ] || print_error "积分应为 150（50+100），实际: ${points}"
[ "$(count_user_coupons "$user_id" "T-FIX-500")" == "1" ] || print_error "T-FIX-500 券应恰 1 张，实际: $(count_user_coupons "$user_id" "T-FIX-500")"
[ "$(count_user_coupons "$user_id" "T-PCT-5000")" == "1" ] || print_error "T-PCT-5000 券应恰 1 张，实际: $(count_user_coupons "$user_id" "T-PCT-5000")"
[ "$(count_dig_prize_ledgers "$rid1")" == "5" ] || print_error "发奖流水应恰 5 笔（现金3+积分2），实际: $(count_dig_prize_ledgers "$rid1")"
[ "$(sum_dig_cash_ledger "$rid1")" == "1600" ] || print_error "现金发奖合计应 1600 分（100+500+1000），实际: $(sum_dig_cash_ledger "$rid1")"
[ "$(count_dig_prize_coupons "$rid1")" == "2" ] || print_error "发奖券应恰 2 张，实际: $(count_dig_prize_coupons "$rid1")"
prize_kinds=$(do_mysql "SELECT CAST((LENGTH(prizes_json)-LENGTH(REPLACE(prizes_json,'\"kind\"','')))/6 AS UNSIGNED) FROM gacha_play.dig_box_records WHERE request_id='${rid1}';")
[ "$prize_kinds" == "7" ] || print_error "记录 prizes_json 应含 7 项奖品（跳过 2 空格），实际: ${prize_kinds}"
print_step "奖品断言通过"

# ---------- 5. 同 key 重放：一切计数不变 ----------
print_step "5. 同 Idempotency-Key 重放"
resp=$(do_post "/dig-boxes/1/buy" '{"boxId":1,"takeAll":true}' "$token" "$key1")
[ "$(extract_code "$resp")" == "0" ] || print_error "重放应返回成功: ${resp}"
[ "$(extract_field "$resp" status)" == "success" ] || print_error "重放 status 应为 success，实际: ${resp}"
balance=$(wait_balance "$token" "27.00" 5)
[ -n "$balance" ] || print_error "重放后余额应仍为 27.00"
[ "$(count_dig_records "$rid1")" == "1" ] || print_error "重放后记录应仍为 1 条，实际: $(count_dig_records "$rid1")"
[ "$(count_dig_prize_ledgers "$rid1")" == "5" ] || print_error "重放后发奖流水应仍为 5 笔，实际: $(count_dig_prize_ledgers "$rid1")"
[ "$(count_dig_prize_coupons "$rid1")" == "2" ] || print_error "重放后发奖券应仍为 2 张，实际: $(count_dig_prize_coupons "$rid1")"
[ "$(get_loyalty_points "$user_id")" == "150" ] || print_error "重放后积分应仍为 150"
print_step "重放幂等通过：记录/流水/券/积分均不变"

# ---------- 6. 二次全开：COMPLETED 会话回收 ----------
print_step "6. 二次全开（新 key）：满格会话回收后重开"
key2="${username}_2"
rid2=$(dig_buy "$token" 1 "" 1 "$key2") || exit 1
record2=$(poll_dig_record "$token" "$rid2" 20)
[ -n "$record2" ] || { print_error "轮询超时：二次全开记录未产生"; exit $FAILED; }
balance=$(wait_balance "$token" "34.00" 20)
[ -n "$balance" ] || print_error "二次全开后余额应为 34.00，实际: $(get_balance "$token")"
[ "$(get_loyalty_points "$user_id")" == "300" ] || print_error "二次全开后积分应为 300，实际: $(get_loyalty_points "$user_id")"
[ "$(count_user_coupons "$user_id" "T-FIX-500")" == "2" ] || print_error "二次全开后 T-FIX-500 应 2 张"
[ "$(count_user_coupons "$user_id" "T-PCT-5000")" == "2" ] || print_error "二次全开后 T-PCT-5000 应 2 张"
[ "$(get_dig_session_field "$user_id" 1 occupied_bitmap)" == "511" ] || print_error "回收重开后 bitmap 应 511"
[ "$(get_dig_session_field "$user_id" 1 total_price_fen)" == "900" ] || print_error "回收后累计实付应为 900 分（重新计数）"
[ "$(get_dig_session_field "$user_id" 1 status)" == "2" ] || print_error "二次全开后会话应 COMPLETED(2)"
print_step "会话回收断言通过"

# ---------- 7. 预置已挖格 → 买该格 Aborted 3016 → 全回滚 ----------
# 屏障语义：03 正向自身 Aborted → 其 barrier 行随事务回滚，补偿是「空补偿」
# （originAffected>0 跳过业务）——正向自己的回滚已撤销报价锁与会话改动，
# 报价回到 OPEN（自然过期回收），会话保持预置原样。
print_step "7. SQL 预置格 0 已挖（PLAYING）后购买格 0"
set_dig_session "$user_id" 1 1 1 100 1 || { print_error "预置会话失败"; exit 1; }
[ "$(get_dig_session_field "$user_id" 1 occupied_bitmap)" == "1" ] || print_error "预置后 bitmap 应为 1"
key3="${username}_3"
rid3=$(dig_buy "$token" 1 "0" "" "$key3") || exit 1
print_step "requestId: ${rid3}（LockDigBox 将 Aborted 3016，补偿链收敛）"
wait_barrier_compensated "$rid3" 20 || print_error "失败 SAGA 补偿链未在 20s 内收敛"
print_step "7.1 全回滚断言：会话原样、无支付单、无记录、余额不变"
[ "$(get_dig_session_field "$user_id" 1 occupied_bitmap)" == "1" ] || print_error "预置位不应被改动，bitmap 应为 1，实际: $(get_dig_session_field "$user_id" 1 occupied_bitmap)"
[ "$(get_dig_session_field "$user_id" 1 total_cells_paid)" == "1" ] || print_error "预置已购格数不应被改动，应为 1"
[ "$(get_dig_session_field "$user_id" 1 total_price_fen)" == "100" ] || print_error "预置实付不应被改动，应为 100"
[ "$(get_dig_session_field "$user_id" 1 status)" == "1" ] || print_error "会话应仍为 PLAYING(1)，实际: $(get_dig_session_field "$user_id" 1 status)"
[ "$(get_quote_status "$rid3")" == "1" ] || print_error "正向自身回滚后报价应回到 OPEN(1)（自然过期），实际: $(get_quote_status "$rid3")"
[ "$(count_dig_records "$rid3")" == "0" ] || print_error "格冲突不应产生记录，实际: $(count_dig_records "$rid3")"
[ "$(count_payment_orders "$rid3")" == "0" ] || print_error "03 中止后不应建支付单，实际: $(count_payment_orders "$rid3")"
balance=$(wait_balance "$token" "34.00" 5)
[ -n "$balance" ] || print_error "冲突补偿后余额应仍为 34.00，实际: $(get_balance "$token")"
print_step "格冲突全回滚通过：会话原样 / 报价 OPEN / 无支付单 / 无记录 / 余额不变"

# ---------- 8. 收尾：会话复位（保持测试环境整洁） ----------
print_step "8. 收尾：会话复位为满格 COMPLETED"
set_dig_session "$user_id" 1 511 9 900 2

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景17 dig_box 通过${NC}"
else
    echo -e "${RED}× 场景17 dig_box 失败${NC}"
fi
exit $FAILED
