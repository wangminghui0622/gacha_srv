#!/bin/bash
# ============================================================
# 场景15：幸运转盘（prize_wheel，M11 campaign kind=3）
#
# 流程：注册 → 预览（spinsLeft=2、日限 2、3 槽）
#       → 抽 2 次（每次奖品恰发一次：按 DB 中奖 kind 核对钱包/券）
#       → 同 key 重放（replayed、计数/余额不变）
#       → 第 3 次 5011（日限）→ SQL 置全部槽 remaining=0
#       → 新用户抽 → 5012（奖池空）→ 复原 remaining=-1
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景15：幸运转盘（prize_wheel）"

# ---------- 1. 注册用户 ----------
print_step "1. 注册用户"
reg=$(register_user "e2e_wheel_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
SUF="${RANDOM}${RANDOM}${RANDOM}" # 幂等键按次运行唯一，避免跨运行撞 uk(idem_key)

# ---------- 2. 预览 ----------
print_step "2. 预览（spinsLeft=2、日限 2、3 槽）"
resp=$(campaign_preview "$token" "C-WHEEL-001")
[ "$(extract_code "$resp")" == "0" ] || { print_error "预览失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" spinsLeft)" == "2" ] || print_error "spinsLeft 应为 2，实际: $(extract_field "$resp" spinsLeft)"
[ "$(extract_field "$resp" spinDailyLimit)" == "2" ] || print_error "spinDailyLimit 应为 2，实际: $(extract_field "$resp" spinDailyLimit)"
slots=$(do_mysql "SELECT COUNT(*) FROM gacha_marketing.campaign_wheel_slots WHERE campaign_id=3;")
[ "$slots" == "3" ] || print_error "槽位应恰 3 个，实际: ${slots}"
print_step "预览断言通过"

# ---------- 3. 抽 2 次：每次奖品恰发一次 ----------
print_step "3. 抽 2 次（每次奖品恰发一次）"
for i in 1 2; do
    key="k15_${i}_${SUF}"
    cash0=$(get_cash_fen "$user_id"); points0=$(get_loyalty_points "$user_id")
    coup0=$(count_user_coupons "$user_id" "T-FIX-500")
    resp=$(participate_campaign "$token" "C-WHEEL-001" "" "$key")
    [ "$(extract_code "$resp")" == "0" ] || { print_error "第${i}抽失败: ${resp}"; exit $FAILED; }
    [ "$(extract_field "$resp" spinsLeft)" == "$((2 - i))" ] || print_error "第${i}抽后 spinsLeft 应为 $((2 - i))，实际: $(extract_field "$resp" spinsLeft)"
    rc=$(count_prize_grants "$key")
    [ "$rc" == "1" ] || print_error "第${i}抽流水应恰 1 条，实际: ${rc}"
    rc=$(count_grants_by_status "$key" 1)
    [ "$rc" == "1" ] || print_error "第${i}抽流水应已发放，实际: ${rc}"
    # 按中奖 kind 核对发放去向（现金→CASH 钱包；积分→忠诚点；券→用户券）
    gkind=$(do_mysql "SELECT prize_kind FROM gacha_marketing.campaign_prize_grants WHERE idem_key='${key}';")
    case "$gkind" in
    1)
        amt=$(do_mysql "SELECT amount_fen FROM gacha_marketing.campaign_prize_grants WHERE idem_key='${key}';")
        cash1=$(get_cash_fen "$user_id")
        [ "$cash1" == "$((cash0 + amt))" ] || print_error "第${i}抽现金应 +${amt}，实际: ${cash0}→${cash1}"
        ;;
    2)
        pts=$(do_mysql "SELECT points_amount FROM gacha_marketing.campaign_prize_grants WHERE idem_key='${key}';")
        points1=$(get_loyalty_points "$user_id")
        [ "$points1" == "$((points0 + pts))" ] || print_error "第${i}抽积分应 +${pts}，实际: ${points0}→${points1}"
        ;;
    3)
        coup1=$(count_user_coupons "$user_id" "T-FIX-500")
        [ "$coup1" == "$((coup0 + 1))" ] || print_error "第${i}抽券应 +1，实际: ${coup0}→${coup1}"
        ;;
    *)
        print_error "第${i}抽奖品类型未知: ${gkind}"
        ;;
    esac
    print_step "第${i}抽：kind=${gkind} 奖品恰发一次（流水 GRANTED）"
done
daily=$(get_wheel_daily_count "$user_id")
[ "$daily" == "2" ] || print_error "日计数应恰 2 次，实际: ${daily}"

# ---------- 4. 同 key 重放：计数/余额不变 ----------
print_step "4. 同 key 重放（幂等）"
cash0=$(get_cash_fen "$user_id"); points0=$(get_loyalty_points "$user_id")
coup0=$(count_user_coupons "$user_id" "T-FIX-500")
resp=$(participate_campaign "$token" "C-WHEEL-001" "" "k15_1_${SUF}")
[ "$(extract_code "$resp")" == "0" ] || { print_error "重放失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" replayed)" == "true" ] || print_error "重放 replayed 应为 true"
rc=$(get_participation_count "k15_1_${SUF}")
[ "$rc" == "1" ] || print_error "重放不应新增参与行，实际: ${rc}"
cash1=$(get_cash_fen "$user_id"); points1=$(get_loyalty_points "$user_id")
coup1=$(count_user_coupons "$user_id" "T-FIX-500")
[ "$cash1" == "$cash0" ] || print_error "重放后现金不应变，实际: ${cash0}→${cash1}"
[ "$points1" == "$points0" ] || print_error "重放后积分不应变，实际: ${points0}→${points1}"
[ "$coup1" == "$coup0" ] || print_error "重放后券数不应变，实际: ${coup0}→${coup1}"
print_step "重放断言通过：余额/券数不变"

# ---------- 5. 第 3 次：5011 日限 ----------
print_step "5. 第 3 次抽 → 5011（日限）"
resp=$(participate_campaign "$token" "C-WHEEL-001" "" "k15_3_${SUF}")
[ "$(extract_code "$resp")" == "5011" ] || print_error "第 3 次应 5011，实际: $(extract_code "$resp") ${resp}"
print_step "日限断言通过"

# ---------- 6. 奖池耗尽：5012 → 复原 ----------
print_step "6. 奖池耗尽（5012）→ 复原"
reg2=$(register_user "e2e_wheel2_") || exit 1
name2=$(echo "$reg2" | cut -d' ' -f2)
token2=$(login_by_name "$name2") || exit 1
set_wheel_slot_remaining 0 0
set_wheel_slot_remaining 1 0
set_wheel_slot_remaining 2 0
resp=$(participate_campaign "$token2" "C-WHEEL-001" "" "k15_4_${SUF}")
[ "$(extract_code "$resp")" == "5012" ] || print_error "奖池空应 5012，实际: $(extract_code "$resp") ${resp}"
set_wheel_slot_remaining 0 -1
set_wheel_slot_remaining 1 -1
set_wheel_slot_remaining 2 -1
print_step "奖池空断言通过：5012、已复原"

# ---------- 7. 复原后新用户可正常抽 ----------
print_step "7. 复原后正常可抽"
resp=$(participate_campaign "$token2" "C-WHEEL-001" "" "k15_5_${SUF}")
[ "$(extract_code "$resp")" == "0" ] || { print_error "复原后应可抽，实际: ${resp}"; exit $FAILED; }
print_step "复原断言通过"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景15 prize_wheel 通过${NC}"
else
    echo -e "${RED}× 场景15 prize_wheel 失败${NC}"
fi
exit $FAILED
