#!/bin/bash
# ============================================================
# 场景14：兑换码活动（redeem_code，M11 campaign kind=2）
#
# 流程：注册 → 预览 GACHA666（per_user 2、券 T-FIX-500）
#       → 兑 2 次（券数 1→2）→ 第 3 次 5010（用户限）
#       → 坏码 5008 → 同 key 重放（replayed、券数不变）
#       → SQL 置 redeemed_count=上限 → 新用户兑 → 5009 → 复原
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景14：兑换码活动（redeem_code）"

# ---------- 1. 注册用户 ----------
print_step "1. 注册用户"
reg=$(register_user "e2e_redeem_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
SUF="${RANDOM}${RANDOM}${RANDOM}" # 幂等键按次运行唯一，避免跨运行撞 uk(idem_key)
KEYA="k14a_${SUF}"; KEYB="k14b_${SUF}"; KEYC="k14c_${SUF}"; KEYD="k14d_${SUF}"; KEYE="k14e_${SUF}"

# ---------- 2. 预览 GACHA666 ----------
print_step "2. 预览 GACHA666（per_user 2、券 T-FIX-500）"
resp=$(campaign_preview "$token" "C-REDEEM-001" "GACHA666")
[ "$(extract_code "$resp")" == "0" ] || { print_error "预览失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" redemptionUserLimit)" == "2" ] || print_error "每用户上限应为 2，实际: $(extract_field "$resp" redemptionUserLimit)"
echo "$resp" | grep -q "T-FIX-500" || print_error "预览奖品应含券 T-FIX-500: ${resp}"
echo "$resp" | grep -q '"kind":3' || print_error "预览奖品 kind 应为 3(券): ${resp}"
print_step "预览断言通过"

# ---------- 3. 兑换 2 次 ----------
print_step "3. 兑换 2 次（券数 1→2）"
resp=$(participate_campaign "$token" "C-REDEEM-001" "GACHA666" "$KEYA")
[ "$(extract_code "$resp")" == "0" ] || { print_error "兑换1失败: ${resp}"; exit $FAILED; }
c1=$(count_user_coupons "$user_id" "T-FIX-500")
[ "$c1" == "1" ] || print_error "兑换1后券数应为 1，实际: ${c1}"
resp=$(participate_campaign "$token" "C-REDEEM-001" "GACHA666" "$KEYB")
[ "$(extract_code "$resp")" == "0" ] || { print_error "兑换2失败: ${resp}"; exit $FAILED; }
c2=$(count_user_coupons "$user_id" "T-FIX-500")
[ "$c2" == "2" ] || print_error "兑换2后券数应为 2，实际: ${c2}"
print_step "兑换断言通过：券数 2"

# ---------- 4. 第 3 次：5010 用户限 ----------
print_step "4. 第 3 次兑换 → 5010（用户限）"
resp=$(participate_campaign "$token" "C-REDEEM-001" "GACHA666" "$KEYC")
[ "$(extract_code "$resp")" == "5010" ] || print_error "第 3 次应 5010，实际: $(extract_code "$resp") ${resp}"
c3=$(count_user_coupons "$user_id" "T-FIX-500")
[ "$c3" == "2" ] || print_error "超限后券数应仍为 2，实际: ${c3}"
print_step "用户限断言通过"

# ---------- 5. 坏码：5008 ----------
print_step "5. 坏码 → 5008"
resp=$(participate_campaign "$token" "C-REDEEM-001" "NO-SUCH-CODE" "$KEYD")
[ "$(extract_code "$resp")" == "5008" ] || print_error "坏码应 5008，实际: $(extract_code "$resp") ${resp}"
print_step "坏码断言通过"

# ---------- 6. 同 key 重放：券数不变 ----------
print_step "6. 同 key 重放（幂等）"
resp=$(participate_campaign "$token" "C-REDEEM-001" "GACHA666" "$KEYA")
[ "$(extract_code "$resp")" == "0" ] || { print_error "重放失败: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" replayed)" == "true" ] || print_error "重放 replayed 应为 true"
rc=$(get_participation_count "$KEYA")
[ "$rc" == "1" ] || print_error "重放不应新增参与行，实际: ${rc}"
c4=$(count_user_coupons "$user_id" "T-FIX-500")
[ "$c4" == "2" ] || print_error "重放后券数应仍为 2，实际: ${c4}"
print_step "重放断言通过：券数不变"

# ---------- 7. 总量超发保护：置 redeemed_count=上限 → 新用户兑 → 5009 → 复原 ----------
print_step "7. 总量超发保护（5009）→ 复原"
orig=$(get_redeem_count "GACHA666")
maxn=$(do_mysql "SELECT max_redemptions FROM gacha_marketing.campaign_redemption_codes WHERE code='GACHA666';")
set_redeem_count "GACHA666" "$maxn"
reg2=$(register_user "e2e_redeem2_") || exit 1
user2=$(echo "$reg2" | cut -d' ' -f1)
name2=$(echo "$reg2" | cut -d' ' -f2)
token2=$(login_by_name "$name2") || exit 1
resp=$(participate_campaign "$token2" "C-REDEEM-001" "GACHA666" "$KEYE")
[ "$(extract_code "$resp")" == "5009" ] || print_error "总量耗尽应 5009，实际: $(extract_code "$resp") ${resp}"
c5=$(count_user_coupons "$user2" "T-FIX-500")
[ "$c5" == "0" ] || print_error "超发拦截后用户2不应得券，实际: ${c5}"
set_redeem_count "GACHA666" "$orig"
print_step "总量超发断言通过：5009、无券、已复原（${orig}）"

# ---------- 8. 预览 BOX888（活动宝箱资格奖品，kind=4） ----------
print_step "8. 预览 BOX888（kind=4 活动宝箱资格）"
resp=$(campaign_preview "$token" "C-REDEEM-001" "BOX888")
[ "$(extract_code "$resp")" == "0" ] || { print_error "BOX888 预览失败: ${resp}"; exit $FAILED; }
echo "$resp" | grep -q '"kind":4' || print_error "BOX888 奖品 kind 应为 4(活动宝箱): ${resp}"
echo "$resp" | grep -q '"boxId":1' || print_error "BOX888 奖品 boxId 应为 1: ${resp}"
print_step "BOX888 预览断言通过"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景14 redeem_code 通过${NC}"
else
    echo -e "${RED}× 场景14 redeem_code 失败${NC}"
fi
exit $FAILED
