#!/bin/bash
# ============================================================
# 场景18：滚动中奖（rolling_draw，M12a：feed 等级 + marquee 过滤 + 限量剔除 + 幂等）
#
# 种子：box1 已配滚动规则（阈值 3000、单抽剔除关、比例 0）；
#       条目：谢谢参与 0/0、金币×100 display=100、限定头像框 display=500、
#             限定皮肤 display=5000 rolling_limited=1（唯一限量）
# 流程：注册 → 充值 20 元 → SQL 抬高皮肤权重（90，自复位）→ 连抽至中皮肤（上限 10 次防卡）
#       → feed 断言：marqueeOnly 含 tier=3 限定皮肤且无 tier=1；全量含 tier=1（NORMAL 过滤）
#       → 复原权重 → 同 Idempotency-Key 重放：同 requestId、记录仍 1 条
#       → SQL 置 single_draw_excluded_percent=100 → 连抽 5 次必无皮肤（确定性剔除）→ 复原
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景18：滚动中奖 feed 与限量剔除（rolling_draw）"

# ---------- 1. 注册 + 充值 20 元 ----------
print_step "1. 注册用户、充值 20 元"
reg=$(register_user "e2e_roll_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge '{"amount":"20"}' "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
[ "$(fen_of "$(get_balance "$token")")" == "2000" ] || print_error "充值后余额应为 20.00"

# ---------- 2. 抬高皮肤权重自抽（场景自复位模式：SQL 改种子，结尾复原） ----------
print_step "2. 抬高皮肤权重至 90 并补库存（结尾复原），连抽至中皮肤"
set_entry_attr 1 "限定皮肤" weight 90 || print_error "改皮肤权重失败"
set_entry_attr 1 "限定皮肤" remaining 50 || print_error "补皮肤库存失败"
win_rid=""
for i in 1 2 3 4 5 6 7 8 9 10; do
    key="${username}_d${i}"
    rid=$(buy_box "$token" 1 "$key") || { print_error "第 ${i} 次购买失败"; exit 1; }
    rec=$(poll_record "$token" "$rid" 20)
    [ -n "$rec" ] || { print_error "第 ${i} 次轮询超时"; exit $FAILED; }
    entry=$(echo "$rec" | sed -n 's/.*"entryName":"\([^"]*\)".*/\1/p')
    print_step "第 ${i} 抽: ${entry}"
    if [ "$entry" == "限定皮肤" ]; then
        win_rid="$rid"
        break
    fi
done
[ -n "$win_rid" ] || { print_error "10 次内未抽中限定皮肤（权重已抬至 90）"; exit 1; }
print_step "中皮肤 requestId: ${win_rid}"

# ---------- 3. feed 断言：等级判定 + marquee 过滤 ----------
print_step "3. feed 等级与 marquee 过滤断言"
feed=$(get_rolling_feed 1 50 1)
[ "$(extract_code "$feed")" == "0" ] || { print_error "marquee feed 失败: ${feed}"; exit 1; }
[ "$(extract_field "$feed" highValueThresholdFen)" == "3000" ] || print_error "boxId=1 应回阈值 3000，实际: ${feed}"
[ "$(count_field "$feed" '"tier":3')" -ge 1 ] || print_error "marquee feed 应含 tier=3 限量中奖，实际: ${feed}"
echo "$feed" | grep -q "限定皮肤" || print_error "marquee feed 应含「限定皮肤」条目，实际: ${feed}"
[ "$(count_field "$feed" '"tier":1')" == "0" ] || print_error "marquee feed 不应含 tier=1 普通中奖（NORMAL 过滤），实际: ${feed}"
full=$(get_rolling_feed 0 50 1)
[ "$(extract_code "$full")" == "0" ] || { print_error "全量 feed 失败: ${full}"; exit 1; }
[ "$(count_field "$full" '"tier":1')" -ge 1 ] || print_error "全量 feed 应含 tier=1 普通中奖（金币/头像框），实际: ${full}"
print_step "feed 断言通过：marquee 只滚高档/限量，全量含普通"

# ---------- 4. 复原皮肤权重 ----------
print_step "4. 复原皮肤权重 90→5"
set_entry_attr 1 "限定皮肤" weight 5 || print_error "复原皮肤权重失败"

# ---------- 5. 同 key 重放：同 requestId、记录不增 ----------
print_step "5. 同 Idempotency-Key 重放（中皮肤那抽）"
resp=$(do_post /box/buy "{\"boxId\":1}" "$token" "${username}_d$((i))")
[ "$(extract_code "$resp")" == "0" ] || print_error "重放应返回成功: ${resp}"
[ "$(extract_field "$resp" requestId)" == "$win_rid" ] || print_error "重放应返回同 requestId，实际: ${resp}"
[ "$(count_records "$win_rid")" == "1" ] || print_error "重放后记录应仍为 1 条，实际: $(count_records "$win_rid")"
print_step "重放幂等通过"

# ---------- 6. 限量剔除：single_draw_excluded_percent=100 → 连抽 5 次必无皮肤 ----------
print_step "6. 置单抽剔除比例 100%（enabled）→ 连抽 5 次确定性无皮肤"
set_rolling_rules 1 '{"high_value_threshold_fen":3000,"single_draw_exclusion_enabled":true,"single_draw_excluded_percent":100,"user_excluded_percent":0}' \
    || { print_error "置剔除规则失败"; exit 1; }
for i in 1 2 3 4 5; do
    rid=$(buy_box "$token" 1 "${username}_x${i}") || { print_error "剔除期第 ${i} 次购买失败"; exit 1; }
    rec=$(poll_record "$token" "$rid" 20)
    [ -n "$rec" ] || { print_error "剔除期第 ${i} 次轮询超时"; exit $FAILED; }
    entry=$(echo "$rec" | sed -n 's/.*"entryName":"\([^"]*\)".*/\1/p')
    [ "$entry" != "限定皮肤" ] || print_error "剔除 100% 下第 ${i} 抽不应中皮肤，实际: ${entry}"
    print_step "第 ${i} 抽: ${entry}（无皮肤 ✓）"
done
print_step "剔除断言通过：100% 剔除下皮肤确定性不中"

# ---------- 7. 收尾：复原规则与库存（保持测试环境整洁） ----------
print_step "7. 收尾：复原滚动规则与皮肤库存"
set_rolling_rules 1 '{"high_value_threshold_fen":3000,"single_draw_exclusion_enabled":false,"single_draw_excluded_percent":0,"user_excluded_percent":0}' \
    || print_error "复原规则失败"
set_entry_attr 1 "限定皮肤" remaining 10 || print_error "复原皮肤库存失败"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景18 rolling_draw 通过${NC}"
else
    echo -e "${RED}× 场景18 rolling_draw 失败${NC}"
fi
exit $FAILED
