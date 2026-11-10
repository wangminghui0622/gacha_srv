#!/bin/bash
# ============================================================
# 场景3：奖池售罄 → SAGA 全额退款（insufficient_stock）
#
# 预置：把传说宝箱(box3)全部中奖条目 remaining 清零，制造整箱售罄。
# 关键路径：购买预检走缓存（看到的是清零前的库存，放行）→
#          SettleDraw 事务内 FOR UPDATE 读真实库存 → 无中奖条目可抽
#          → Aborted(2002) → DTM 逆序补偿 CancelPayment 撤单解冻。
# 断言：
#   1. 购买响应 code=0（预检放行，靠 SAGA 实检拦截）
#   2. 轮询无抽赏记录
#   3. 余额 200.00 扣了又退回（SAGA 补偿生效）
#   4. draw_records 0 条；条目库存仍为 0（没被错误扣减）
#   5. M8 支付域：支付单 CANCELED(6)、CASH 腿 VOIDED(3)、hold VOIDED(3)
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景3：奖池售罄退款（insufficient_stock）"

# ---------- 0. 自复位库存（场景可独立重复执行） ----------
print_step "0. 自复位奖池库存（reset_test_data.sql）"
"${MYSQL_BIN}" --connect-timeout=5 -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" -p"${DB_PASS}" \
    --default-character-set=utf8mb4 gacha_draw < "${SCRIPT_DIR}/../script/sql/reset_test_data.sql" \
    || { print_error "复位 SQL 执行失败"; exit 1; }

# ---------- 1. 先看详情预热缓存（此时库存充足） ----------
print_step "1. 预检预热：刷新条目缓存后查看 box3 详情（缓存充足库存快照）"
# 防毒化：上次连跑残留的「售罄视图」（3 分钟 TTL）会让预热读到旧缓存，
#         买时预检 2002 提前拦截 → 清键保证预热必走库、回填充足视图
flush_box_entries_cache 3
detail=$(do_get /boxes/3)
[ "$(extract_code "$detail")" == "0" ] || { print_error "box3 详情查询失败: ${detail}"; exit 1; }

# ---------- 2. 直连数据库清零中奖条目（模拟被抢空） ----------
print_step "2. 数据库清零 box3 全部中奖条目（remaining=0 制造售罄）"
do_mysql "UPDATE gacha_draw.draw_pool_entries SET remaining = 0 WHERE box_id = 3 AND is_prize = 1;" \
    || { print_error "清零库存失败"; exit 1; }
remaining=$(get_entry_remaining 3 '传说坐骑')
[ "$remaining" == "0" ] || print_error "清零后库存应为 0，实际: ${remaining}"

# ---------- 3. 注册 + 充值 + 购买 ----------
print_step "3. 注册用户、充值 200 元、购买 box3（预检走缓存放行）"
reg=$(register_user "e2e_stock_") || exit 1
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge "{\"amount\":\"200\"}" "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
buy_resp=$(do_post /box/buy '{"boxId":3}' "$token")
[ "$(extract_code "$buy_resp")" == "0" ] || print_error "购买提交失败（预检应走缓存放行）: ${buy_resp}"
rid=$(extract_field "$buy_resp" requestId)
print_step "requestId: ${rid}（SettleDraw 实检将 Aborted，触发退款）"

# ---------- 4. 轮询：无记录 ----------
print_step "4. 轮询抽赏记录（5s 内不应出现）"
record=$(poll_record "$token" "$rid" 5)
[ -z "$record" ] || print_error "售罄却产生了抽赏记录: ${record}"

# ---------- 5. 余额退回：SAGA 补偿生效 ----------
print_step "5. 等待退款：余额应回到 200.00"
balance=$(wait_balance "$token" "200.00" 20)
[ -n "$balance" ] || print_error "退款超时：余额应为 200.00（扣款已退），实际: $(get_balance "$token")"
print_step "退款后余额: ${balance}"

# ---------- 6. 数据库断言 ----------
print_step "6. 数据库断言"
rec_count=$(count_records "$rid")
[ "$rec_count" == "0" ] || print_error "draw_records 应为 0 条，实际: ${rec_count}"
remaining=$(get_entry_remaining 3 '传说坐骑')
[ "$remaining" == "0" ] || print_error "库存应仍为 0（未被错误扣减），实际: ${remaining}"

# ---------- 7. 支付域断言（M8）：撤单解冻到位 ----------
print_step "7. 支付域断言：支付单 CANCELED / 腿 VOIDED / hold VOIDED"
wait_payment_status "$rid" 6 20 || print_error "支付单未在 20s 内到达 CANCELED(6)"
pstatus=$(get_payment_status "$rid")
[ "$pstatus" == "6" ] || print_error "payment_orders.status 应为 6(CANCELED)，实际: ${pstatus}"
leg_status=$(get_payment_leg_status "$rid" 1)
[ "$leg_status" == "3" ] || print_error "payment_legs CASH 腿应为 3(VOIDED)，实际: ${leg_status}"
hold_status=$(get_payment_hold_status "$rid" 1)
[ "$hold_status" == "3" ] || print_error "wallet_holds 应为 3(VOIDED)，实际: ${hold_status}"
print_step "支付域断言通过：CANCELED / VOIDED"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景3 insufficient_stock 通过${NC}"
else
    echo -e "${RED}× 场景3 insufficient_stock 失败${NC}"
fi
exit $FAILED
