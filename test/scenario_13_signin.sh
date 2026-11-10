#!/bin/bash
# ============================================================
# 场景13：签到活动（sign_in，M11 campaign kind=1）
#
# 流程：注册 → 首签（streak=1，day1 积分+10）
#       → 同 key 重放（replayed=true、流水不变）
#       → 同日新 key（日期幂等：无新奖品）
#       → SQL 回拨昨日 → 连签 streak=2（day2 积分+20）
#       → SQL 回拨 3 天前 → 断签重置 streak=1（day1 积分+10）
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景13：签到活动（sign_in）"

# ---------- 1. 注册用户 ----------
print_step "1. 注册用户"
reg=$(register_user "e2e_signin_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
SUF="${RANDOM}${RANDOM}${RANDOM}" # 幂等键按次运行唯一，避免跨运行撞 uk(idem_key)
KEYA="k13a_${SUF}"; KEYB="k13b_${SUF}"; KEYC="k13c_${SUF}"; KEYD="k13d_${SUF}"

# ---------- 2. 首签：streak=1，day1 积分+10 ----------
print_step "2. 首签（streak=1，day1 积分+10）"
resp=$(participate_campaign "$token" "C-SIGN-IN-001" "" "$KEYA")
[ "$(extract_code "$resp")" == "0" ] || { print_error "首签失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" replayed)" == "false" ] || print_error "首签 replayed 应为 false"
[ "$(extract_field "$resp" streak)" == "1" ] || print_error "首签 streak 应为 1，实际: $(extract_field "$resp" streak)"
echo "$resp" | grep -q '"kind":2' || print_error "首签奖品应含积分: ${resp}"
echo "$resp" | grep -q '"points":10' || print_error "day1 应奖 10 积分: ${resp}"
points=$(get_loyalty_points "$user_id")
[ "$points" == "10" ] || print_error "首签后积分应为 10，实际: ${points}"
rc=$(count_prize_grants "$KEYA")
[ "$rc" == "1" ] || print_error "首签流水应恰 1 条，实际: ${rc}"
rc=$(count_grants_by_status "$KEYA" 1)
[ "$rc" == "1" ] || print_error "首签流水应已发放(GRANTED)，实际: ${rc}"
print_step "首签断言通过：streak=1, 积分 10, 流水 1 条 GRANTED"

# ---------- 3. 同 key 重放：replayed=true、流水不变 ----------
print_step "3. 同 key 重放（幂等）"
resp=$(participate_campaign "$token" "C-SIGN-IN-001" "" "$KEYA")
[ "$(extract_code "$resp")" == "0" ] || { print_error "重放失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" replayed)" == "true" ] || print_error "重放 replayed 应为 true"
[ "$(extract_field "$resp" streak)" == "1" ] || print_error "重放 streak 应为 1，实际: $(extract_field "$resp" streak)"
rc=$(get_participation_count "$KEYA")
[ "$rc" == "1" ] || print_error "重放不应新增参与行，实际: ${rc}"
points=$(get_loyalty_points "$user_id")
[ "$points" == "10" ] || print_error "重放后积分应仍为 10，实际: ${points}"
print_step "重放断言通过：replayed=true, 参与行 1, 积分不变"

# ---------- 4. 同日新 key：日期幂等，无新奖品 ----------
print_step "4. 同日新 key（日期幂等：无新奖品）"
resp=$(participate_campaign "$token" "C-SIGN-IN-001" "" "$KEYB")
[ "$(extract_code "$resp")" == "0" ] || { print_error "同日再签失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" replayed)" == "false" ] || print_error "新 key replayed 应为 false"
[ "$(extract_field "$resp" streak)" == "1" ] || print_error "同日 streak 应为 1，实际: $(extract_field "$resp" streak)"
rc=$(count_prize_grants "$KEYB")
[ "$rc" == "0" ] || print_error "同日再签不应有流水，实际: ${rc}"
points=$(get_loyalty_points "$user_id")
[ "$points" == "10" ] || print_error "同日再签后积分应仍为 10，实际: ${points}"
print_step "日期幂等断言通过：无流水、积分不变"

# ---------- 5. SQL 回拨昨日：连签 streak=2（day2 积分+20） ----------
print_step "5. 回拨昨日 → 连签 streak=2（day2 积分+20）"
yesterday=$(date -d "yesterday" +%F 2>/dev/null || date -v-1d +%F)
set_signin_last_date "$user_id" "$yesterday"
resp=$(participate_campaign "$token" "C-SIGN-IN-001" "" "$KEYC")
[ "$(extract_code "$resp")" == "0" ] || { print_error "连签失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" streak)" == "2" ] || print_error "连签 streak 应为 2，实际: $(extract_field "$resp" streak)"
echo "$resp" | grep -q '"points":20' || print_error "day2 应奖 20 积分: ${resp}"
points=$(get_loyalty_points "$user_id")
[ "$points" == "30" ] || print_error "连签后积分应为 30（10+20），实际: ${points}"
print_step "连签断言通过：streak=2, 积分 30"

# ---------- 6. SQL 回拨 3 天前：断签重置 streak=1（day1 积分+10） ----------
print_step "6. 回拨 3 天前 → 断签重置 streak=1（day1 积分+10）"
d3ago=$(date -d "3 days ago" +%F 2>/dev/null || date -v-3d +%F)
set_signin_last_date "$user_id" "$d3ago"
resp=$(participate_campaign "$token" "C-SIGN-IN-001" "" "$KEYD")
[ "$(extract_code "$resp")" == "0" ] || { print_error "断签重签失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" streak)" == "1" ] || print_error "断签后 streak 应重置为 1，实际: $(extract_field "$resp" streak)"
points=$(get_loyalty_points "$user_id")
[ "$points" == "40" ] || print_error "断签重签后积分应为 40（30+10），实际: ${points}"
state=$(get_signin_state "$user_id")
[ "$(echo "$state" | cut -d' ' -f1)" == "1" ] || print_error "签到表 streak 应为 1，实际: ${state}"
[ "$(echo "$state" | cut -d' ' -f2)" == "3" ] || print_error "签到表 total_days 应为 3，实际: ${state}"
print_step "断签断言通过：streak=1, total_days=3, 积分 40"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景13 sign_in 通过${NC}"
else
    echo -e "${RED}× 场景13 sign_in 失败${NC}"
fi
exit $FAILED
