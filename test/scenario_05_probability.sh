#!/bin/bash
# ============================================================
# 场景5：概率公示核对（M6 验收）
#
# 抽 box1 共 1000 次，统计各条目实际频率，与公示概率偏差须逐类 < max(2%, 3σ)
# （σ=sqrt(p(1-p)/N)：1000 抽下 3σ ≈ 2.07%~4.74%，逐类联合误报率 ~1%；
#   固定 2% 阈值对 50%/30% 类目仅 ~1.3σ，联合误报率 ~40%，属统计误报源）
# 公示概率（weight/Σweight=100，见 script/split/03_gacha_draw.sql）：
#   谢谢参与 50% / 金币×100 30% / 限定头像框 15% / 限定皮肤 5%
#
# 前置：先把低库存条目调足（限定皮肤 10→1000、限定头像框 100→500），
# 否则条目在抽取中耗尽后退出奖池（remaining=0 不参与抽取），概率分布会漂移。
# 脚本结束时提示用 reset_test_data.sql 复原库存。
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景5：概率公示核对（1000 抽，逐类偏差 < max(2%, 3σ)）"

DRAWS=1000

# 公示概率（%）
declare -A EXPECTED=(
    ["谢谢参与"]=50
    ["金币×100"]=30
    ["限定头像框"]=15
    ["限定皮肤"]=5
)

# ---------- 1. 调足低库存条目 ----------
print_step "1. 调足低库存条目（避免抽取中耗尽退出奖池）"
do_mysql "
UPDATE gacha_draw.draw_pool_entries SET remaining=1000 WHERE box_id=1 AND name='限定皮肤';
UPDATE gacha_draw.draw_pool_entries SET remaining=500  WHERE box_id=1 AND name='限定头像框';
" || print_error "库存调整失败"
print_step "限定皮肤 remaining=$(get_entry_remaining 1 '限定皮肤')，限定头像框 remaining=$(get_entry_remaining 1 '限定头像框')"

# ---------- 2. 注册 + 充值 ----------
print_step "2. 注册用户并充值 ${DRAWS} 元（${DRAWS} 抽 × 1.00 元）"
reg=$(register_user "prob_") || exit 1
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge "{\"amount\":\"${DRAWS}\"}" "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
print_step "充值后余额: $(extract_field "$resp" balance)"

# ---------- 3. 连抽 1000 次并统计 ----------
print_step "3. 连抽 ${DRAWS} 次（每次购买 → 轮询记录 → 按条目名计数）"
declare -A COUNTS=()
i=0
while [ "$i" -lt "$DRAWS" ]; do
    buy_resp=$(do_post /box/buy '{"boxId":1}' "$token")
    if [ "$(extract_code "$buy_resp")" != "0" ]; then
        print_error "第 $((i + 1)) 抽购买失败: ${buy_resp}"
        exit $FAILED
    fi
    rid=$(extract_field "$buy_resp" requestId)
    record=$(poll_record "$token" "$rid" 20)
    if [ -z "$record" ]; then
        print_error "第 $((i + 1)) 抽轮询超时: requestId=${rid}"
        exit $FAILED
    fi
    entry=$(extract_field "$record" entryName)
    COUNTS["$entry"]=$(( ${COUNTS["$entry"]:-0} + 1 ))
    i=$((i + 1))
    if [ $((i % 100)) -eq 0 ]; then
        print_step "已抽 ${i}/${DRAWS}"
    fi
done

# ---------- 4. 偏差核对 ----------
print_step "4. 实际频率 vs 公示概率（逐类阈值 = max(2%, 3σ)）"
ok=1
for name in "${!EXPECTED[@]}"; do
    cnt=${COUNTS["$name"]:-0}
    exp=${EXPECTED["$name"]}
    dev=$(awk -v c="$cnt" -v n="$DRAWS" -v e="$exp" 'BEGIN { d = c / n * 100 - e; if (d < 0) d = -d; printf "%.2f", d }')
    limit=$(awk -v e="$exp" -v n="$DRAWS" 'BEGIN { s = 3 * sqrt(e * (100 - e) / n); if (s < 2.0) s = 2.0; printf "%.2f", s }')
    over=$(awk -v d="$dev" -v l="$limit" 'BEGIN { print (d > l) ? "1" : "0" }')
    pct=$(awk -v c="$cnt" -v n="$DRAWS" 'BEGIN { printf "%.2f", c / n * 100 }')
    if [ "$over" == "1" ]; then
        print_error "${name}: 实际 ${pct}%（${cnt} 次），公示 ${exp}%，偏差 ${dev}% ≥ 阈值 ${limit}%"
        ok=0
    else
        print_step "${name}: 实际 ${pct}%（${cnt} 次），公示 ${exp}%，偏差 ${dev}% < ${limit}% ✓"
    fi
done

# ---------- 5. 余额守恒（只扣 1000 元） ----------
print_step "5. 校验余额恰好扣 ${DRAWS} 元"
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "0" ] || print_error "1000 抽后余额应为 0.00，实际: ${balance}"

print_step "6. 库存复原提示：bash test/run_all.sh 的开场 reset 会自动复原；或手动执行 reset_test_data.sql"

if [ "$ok" == "1" ] && [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景5 probability 通过${NC}"
else
    echo -e "${RED}× 场景5 probability 失败${NC}"
fi
exit $FAILED
