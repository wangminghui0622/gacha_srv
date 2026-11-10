#!/bin/bash
# ============================================================
# 场景16：活动宝箱（campaign_box，M11c entitlement 零付开箱）
#
# 流程：A 注册 → 兑 BOX888（kind=4 活动宝箱资格，box1）→ 开箱
#       → 记录 price=0.00、支付 PAID、余额不变 0.00、资格/报价 USED
#       → 同 key 重放 → success、计数全不变
#       → B 兑 BOX888 → 预热 box1 缓存后数据库清零库存 → 开箱
#       → SettleDraw 实检售罄 Aborted(2002) → 补偿链：资格回 AVAILABLE
#         （locked_by 清空）、报价 RELEASED(4)、支付 CANCELED(6)、无记录
#       → 复原库存重开（新 key）→ 成功：资格/报价 USED、余额 0.00
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景16：活动宝箱零付开箱（campaign_box）"

# ---------- 0. 自复位（场景可独立重复执行） ----------
print_step "0. 自复位：奖池库存 + BOX888 已兑次数"
"${MYSQL_BIN}" --connect-timeout=5 -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" -p"${DB_PASS}" \
    --default-character-set=utf8mb4 gacha_draw < "${SCRIPT_DIR}/../script/sql/reset_test_data.sql" \
    || { print_error "复位 SQL 执行失败"; exit 1; }
set_redeem_count "BOX888" 0 || { print_error "复位 BOX888 失败"; exit 1; }
flush_box_entries_cache 1
SUF="${RANDOM}${RANDOM}${RANDOM}" # 幂等键按次运行唯一，避免跨运行撞 uk(idem_key)

# ---------- 1. 注册 A + 兑 BOX888 资格 ----------
print_step "1. 注册 A、兑换 BOX888（box1 活动宝箱资格）"
reg=$(register_user "e2e_cbox_") || exit 1
uidA=$(echo "$reg" | cut -d' ' -f1)
nameA=$(echo "$reg" | cut -d' ' -f2)
tokenA=$(login_by_name "$nameA") || exit 1
[ "$(fen_of "$(get_balance "$tokenA")")" == "0" ] || print_error "新用户 A 余额应为 0.00"
KEYA="k16a_${SUF}"
resp=$(participate_campaign "$tokenA" "C-REDEEM-001" "BOX888" "$KEYA")
[ "$(extract_code "$resp")" == "0" ] || { print_error "A 兑 BOX888 失败: ${resp}"; exit $FAILED; }
echo "$resp" | grep -q '"kind":4' || print_error "BOX888 奖品 kind 应为 4: ${resp}"
entA=$(get_entitlement_no "$uidA")
[ -n "$entA" ] || { print_error "未查到 A 的资格单"; exit $FAILED; }
[ "$(get_entitlement_status "$entA")" == "1" ] || print_error "新资格应为 AVAILABLE(1)"
print_step "A 资格单: ${entA}"

# ---------- 2. A 开箱（零付）→ 记录/支付/资格/报价断言 ----------
print_step "2. A 开箱（entitlement 全抵，实付 0.00）"
KEYA1="k16a1_${SUF}"
resp=$(open_campaign_box "$tokenA" "$entA" "$KEYA1")
[ "$(extract_code "$resp")" == "0" ] || { print_error "A 开箱失败: ${resp}"; exit $FAILED; }
ridA=$(extract_field "$resp" requestId)
[ -n "$ridA" ] || { print_error "requestId 不应为空"; exit $FAILED; }
print_step "requestId: ${ridA}"
record=$(poll_record "$tokenA" "$ridA" 20)
[ -n "$record" ] || { print_error "轮询超时：A 开箱未完成"; exit $FAILED; }
price=$(extract_field "$record" price)
[ "$price" == "0.00" ] || print_error "记录 price 应为实付 0.00，实际: ${price}"
wait_payment_paid "$ridA" 20 || { print_error "支付单未在 20s 内到达 PAID"; exit $FAILED; }
[ "$(fen_of "$(get_balance "$tokenA")")" == "0" ] || print_error "零付开箱后 A 余额应仍为 0.00，实际: $(get_balance "$tokenA")"
wait_entitlement_status "$entA" 3 20 || print_error "资格未在 20s 内到达 USED(3)，实际: $(get_entitlement_status "$entA")"
qstat=$(get_quote_status "$ridA")
[ "$qstat" == "3" ] || print_error "开箱报价锁应为 USED(3)，实际: ${qstat}"
[ "$(count_records "$ridA")" == "1" ] || print_error "开箱应恰 1 条抽赏记录，实际: $(count_records "$ridA")"
print_step "A 开箱断言通过：price=0.00、支付 PAID、余额 0.00、资格/报价 USED"

# ---------- 3. A 同 key 重放 → success、计数全不变 ----------
print_step "3. A 同 key 重放（幂等快路径）"
resp=$(open_campaign_box "$tokenA" "$entA" "$KEYA1")
[ "$(extract_code "$resp")" == "0" ] || { print_error "重放失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" status)" == "success" ] || print_error "重放 status 应为 success，实际: ${resp}"
[ "$(count_records "$ridA")" == "1" ] || print_error "重放不应新增记录，实际: $(count_records "$ridA")"
[ "$(get_entitlement_status "$entA")" == "3" ] || print_error "重放后资格应仍为 USED(3)"
print_step "重放断言通过：success、记录/资格不变"

# ---------- 4. B 兑资格 + 预热 box1 缓存后清零库存 ----------
print_step "4. B 兑 BOX888、预热 box1 缓存后数据库清零中奖条目"
reg=$(register_user "e2e_cbox2_") || exit 1
uidB=$(echo "$reg" | cut -d' ' -f1)
nameB=$(echo "$reg" | cut -d' ' -f2)
tokenB=$(login_by_name "$nameB") || exit 1
KEYB="k16b_${SUF}"
resp=$(participate_campaign "$tokenB" "C-REDEEM-001" "BOX888" "$KEYB")
[ "$(extract_code "$resp")" == "0" ] || { print_error "B 兑 BOX888 失败: ${resp}"; exit $FAILED; }
entB=$(get_entitlement_no "$uidB")
[ -n "$entB" ] || { print_error "未查到 B 的资格单"; exit $FAILED; }
flush_box_entries_cache 1
detail=$(do_get /boxes/1)
[ "$(extract_code "$detail")" == "0" ] || { print_error "box1 详情查询失败: ${detail}"; exit 1; }
do_mysql "UPDATE gacha_draw.draw_pool_entries SET remaining = 0 WHERE box_id = 1 AND is_prize = 1;" \
    || { print_error "清零库存失败"; exit 1; }
remaining=$(get_entry_remaining 1 '金币×100')
[ "$remaining" == "0" ] || print_error "清零后库存应为 0，实际: ${remaining}"
print_step "B 资格单: ${entB}，库存已清零"

# ---------- 5. B 开箱（预检走缓存放行 → 05 实检 Aborted） ----------
print_step "5. B 开箱（售罄：SettleDraw 实检 Aborted，触发补偿链）"
KEYB1="k16b1_${SUF}"
resp=$(open_campaign_box "$tokenB" "$entB" "$KEYB1")
[ "$(extract_code "$resp")" == "0" ] || { print_error "B 开箱提交失败: ${resp}"; exit $FAILED; }
ridB=$(extract_field "$resp" requestId)
print_step "requestId: ${ridB}"

# ---------- 6. 补偿链断言：资格释放 / 报价释放 / 撤单 / 无记录 ----------
print_step "6. 等待补偿：资格回 AVAILABLE、报价 RELEASED、支付 CANCELED"
wait_entitlement_status "$entB" 1 20 || print_error "资格未在 20s 内回到 AVAILABLE(1)，实际: $(get_entitlement_status "$entB")"
locked_by=$(get_entitlement_locked_by "$entB")
[ -z "$locked_by" ] || print_error "释放后资格 locked_by 应为空，实际: ${locked_by}"
wait_quote_status "$ridB" 4 20 || print_error "报价锁未在 20s 内到达 RELEASED(4)，实际: $(get_quote_status "$ridB")"
wait_payment_status "$ridB" 6 20 || print_error "支付单未在 20s 内到达 CANCELED(6)"
[ "$(fen_of "$(get_balance "$tokenB")")" == "0" ] || print_error "失败后 B 余额应仍为 0.00，实际: $(get_balance "$tokenB")"
[ "$(count_records "$ridB")" == "0" ] || print_error "售罄不应产生抽赏记录，实际: $(count_records "$ridB")"
print_step "补偿链断言通过：资格 AVAILABLE / 报价 RELEASED / 支付 CANCELED / 余额 0.00 / 无记录"

# ---------- 7. 复原库存 → B 重开成功 ----------
print_step "7. 复原库存，B 重开（新 key）"
"${MYSQL_BIN}" --connect-timeout=5 -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" -p"${DB_PASS}" \
    --default-character-set=utf8mb4 gacha_draw < "${SCRIPT_DIR}/../script/sql/reset_test_data.sql" \
    || { print_error "复位 SQL 执行失败"; exit 1; }
flush_box_entries_cache 1
KEYB2="k16b2_${SUF}"
resp=$(open_campaign_box "$tokenB" "$entB" "$KEYB2")
[ "$(extract_code "$resp")" == "0" ] || { print_error "B 重开失败: ${resp}"; exit $FAILED; }
ridB2=$(extract_field "$resp" requestId)
record=$(poll_record "$tokenB" "$ridB2" 20)
[ -n "$record" ] || { print_error "轮询超时：B 重开未完成"; exit $FAILED; }
price=$(extract_field "$record" price)
[ "$price" == "0.00" ] || print_error "重开记录 price 应为 0.00，实际: ${price}"
wait_payment_paid "$ridB2" 20 || { print_error "重开支付单未在 20s 内到达 PAID"; exit $FAILED; }
[ "$(fen_of "$(get_balance "$tokenB")")" == "0" ] || print_error "重开后 B 余额应仍为 0.00，实际: $(get_balance "$tokenB")"
wait_entitlement_status "$entB" 3 20 || print_error "重开后资格应为 USED(3)，实际: $(get_entitlement_status "$entB")"
qstat=$(get_quote_status "$ridB2")
[ "$qstat" == "3" ] || print_error "重开报价锁应为 USED(3)，实际: ${qstat}"
print_step "B 重开断言通过：price=0.00、资格/报价 USED、余额 0.00"

# ---------- 8. 恢复现场 ----------
print_step "8. 恢复现场：box1 库存复位 + 缓存刷新"
"${MYSQL_BIN}" --connect-timeout=5 -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" -p"${DB_PASS}" \
    --default-character-set=utf8mb4 gacha_draw < "${SCRIPT_DIR}/../script/sql/reset_test_data.sql" \
    || { print_error "复位 SQL 执行失败"; exit 1; }
flush_box_entries_cache 1
print_step "现场已恢复"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景16 campaign_box 通过${NC}"
else
    echo -e "${RED}× 场景16 campaign_box 失败${NC}"
fi
exit $FAILED
