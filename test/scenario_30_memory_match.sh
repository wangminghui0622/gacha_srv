#!/bin/bash
# ============================================================
# 场景30：记忆翻牌（memory_match，M15b 全流程 + 负例 + 终局 + 擂主）
#
# 种子：1 号「记忆翻牌·命运牌局」3 轮×3 对、100 分、DYNAMIC、第二命运开、
#       擂主门槛 5 对 / 分成 5%（100 分×5%=5 分）；奖档 9对→现金5000 / 6对→积分500 /
#       3对→现金100 / 0对→谢谢参与（不限量）
# 流程：注册 A/B 各充值 20 元 → SQL 复位现场（无擂主/奖档满库存）→
#       A WECHAT 购买 → 6021（仅 WALLET，偏离 2）→ A WALLET 购买（state 2 / 3 轮 / 实付 100 分）
#       → 同 key 重放 → 同 session（幂等）→ 命运（同值幂等、异值 6022）→ 第二命运 →
#       手打回合（错对 → 400、手牌满时命运抽牌 → 6025、未终局 complete → 6023、
#         贪心配对接命运抽牌至终局、已终局 pair → 6024、complete → 终局结算）
#       → 终局断言（total=各回合手打和、final_hash 64hex、settlement status=2、
#         奖档按 total 精确匹配：钱包/积分/券 mysql 直查、remaining 扣减、complete 重放幂等）
#       → SQL 锻造擂主 A/4 对 → B 购买+skip（AutoResolve；secondDestiny=手牌最频图案
#         保证 total≥5——引擎确定性统计 5000/5000 种子达标，仍保留 3 次重试兜底）
#       → 换擂断言：B 成为擂主、事件恰 1、A 获 pending 分成打款（5 分×购买次数）
#         恰一次 PAID、A 余额+打款、skip 重放幂等
#       → 外观（GET 默认 1/1、PUT 非默认无权益 → 6027、SQL 插权益 → OK）
#       → 越权读会话 → 404 → 现场复位
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景30：记忆翻牌全流程（memory_match）"

# ---------- 1. 注册 A + 充值 20 元 ----------
print_step "1. 注册 A、充值 20 元"
reg_a=$(register_user "e2e_mm_a_") || exit 1
uid_a=$(echo "$reg_a" | cut -d' ' -f1)
name_a=$(echo "$reg_a" | cut -d' ' -f2)
token_a=$(login_by_name "$name_a") || exit 1
resp=$(do_post /user/recharge '{"amount":"20"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "A 充值失败: ${resp}"
[ "$(fen_of "$(get_balance "$token_a")")" == "2000" ] || print_error "A 充值后余额应为 20.00"

# ---------- 2. SQL 复位记忆翻牌现场 ----------
print_step "2. SQL 复位现场（无会话/无擂主/奖档满库存）"
reset_memory_match 1
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_play.memory_match_sessions WHERE config_id=1;")" == "0" ] || print_error "复位后会话应为 0"
[ "$(mm_champion_field 1 user_id)" == "" ] || print_error "复位后应无擂主"
[ "$(mm_entry_remaining 1 9)" == "10" ] || print_error "复位后 9 对档库存应 10"
[ "$(mm_entry_remaining 1 6)" == "50" ] || print_error "复位后 6 对档库存应 50"
[ "$(mm_entry_remaining 1 3)" == "100" ] || print_error "复位后 3 对档库存应 100"

# ---------- 3. 玩法详情（第 12 端点） ----------
print_step "3. GET /memory-match/configs/1（配置+奖档+擂主+外观）"
cfg=$(mm_config "$token_a" 1)
[ "$(extract_code "$cfg")" == "0" ] || print_error "玩法详情查询失败: ${cfg}"
cfg_name=$(echo "$cfg" | sed -n 's/.*"configId":1,"name":"\([^"]*\)".*/\1/p')
[ "$cfg_name" == "记忆翻牌·命运牌局" ] || print_error "玩法名称不符: ${cfg}"
[ "$(extract_field "$cfg" roundsPerPurchase)" == "3" ] || print_error "roundsPerPurchase 应为 3，实际: ${cfg}"
[ "$(extract_field "$cfg" maxPairCount)" == "3" ] || print_error "maxPairCount 应为 3，实际: ${cfg}"
[ "$(count_field "$cfg" '"entryId":')" == "4" ] || print_error "应含 4 个奖档，实际: ${cfg}"
[ "$(count_field "$cfg" '"presetId":')" == "4" ] || print_error "应含 4 个外观预设（2 正面+2 背面），实际: ${cfg}"
[ "$(extract_field "$cfg" exists)" == "false" ] || print_error "复位后应无擂主"
print_step "玩法详情通过（4 奖档 / 4 预设 / 无擂主）"

# ---------- 4. WECHAT 购买 → 6021；WALLET 购买成功 ----------
print_step "4. WECHAT 购买 → 6021（仅 WALLET）"
key_wx="mm_a_wx_$(date +%s)"
resp=$(do_post /memory-match/purchase '{"configId":1,"facePresetId":0,"backPresetId":0,"paymentMethod":"WECHAT"}' "$token_a" "$key_wx")
[ "$(extract_code "$resp")" == "6021" ] || print_error "WECHAT 应返回 6021，实际: ${resp}"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_play.memory_match_sessions WHERE config_id=1;")" == "0" ] || print_error "WECHAT 拒绝不应产生会话"

print_step "4.1 WALLET 购买（3 轮 100 分）"
key_a="mm_a_buy_$(date +%s)"
resp=$(mm_purchase "$token_a" 1 "$key_a" WALLET 0 0) || exit 1
sid_a=$(extract_field "$resp" sessionId)
[ -n "$sid_a" ] || print_error "购买应返回 sessionId: ${resp}"
[ "$(extract_field "$resp" purchasedRounds)" == "3" ] || print_error "purchasedRounds 应为 3，实际: ${resp}"
[ "$(extract_field "$resp" paidMinor)" == "100" ] || print_error "paidMinor 应为 100，实际: ${resp}"
[ "$(extract_field "$resp" state)" == "2" ] || print_error "购买后 state 应为 2（待选命运），实际: ${resp}"
[ "$(extract_field "$resp" facePresetId)" == "1" ] || print_error "facePresetId 应默认 1，实际: ${resp}"
[ "$(extract_field "$resp" backPresetId)" == "1" ] || print_error "backPresetId 应默认 1，实际: ${resp}"
[ "$(mm_session_field "$sid_a" status)" == "2" ] || print_error "会话行 status 应 2"
[ "$(mm_session_field "$sid_a" engine_seed)" == "$(mm_session_field "$sid_a" engine_seed)" ] && true
seed_a=$(mm_session_field "$sid_a" engine_seed)
echo "$seed_a" | grep -qE '^[0-9a-f]{64}$' || print_error "engine_seed 应 64 位 hex，实际: ${seed_a}"
[ "$(mm_session_field "$sid_a" request_id)" == "$key_a" ] || print_error "会话 request_id 应 = 幂等键"
balance=$(wait_balance "$token_a" "19.00" 20)
[ -n "$balance" ] || print_error "A 购买后余额应 19.00，实际: $(get_balance "$token_a")"
print_step "购买成功 sessionId=${sid_a}（state 2 / seed 64hex / 余额 19.00）"

# ---------- 5. 同 key 重放 → 同 session ----------
print_step "5. 同 Idempotency-Key 重放 → 同 session"
resp=$(do_post /memory-match/purchase '{"configId":1,"facePresetId":0,"backPresetId":0}' "$token_a" "$key_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "重放应返回成功: ${resp}"
[ "$(extract_field "$resp" sessionId)" == "$sid_a" ] || print_error "重放应返回同 sessionId，实际: ${resp}"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_play.memory_match_sessions WHERE config_id=1;")" == "1" ] || print_error "重放后会话应仍 1 条"
print_step "重放幂等通过（同 session、无新会话）"

# ---------- 6. 命运：同值幂等、异值 6022 ----------
print_step "6. 选命运（同值幂等、异值 6022）"
resp=$(mm_select_destiny "$token_a" "$sid_a" 5)
[ "$(extract_code "$resp")" == "0" ] || print_error "选命运应成功: ${resp}"
[ "$(extract_field "$resp" status)" == "SELECTED" ] || print_error "命运响应 status 应 SELECTED，实际: ${resp}"
[ "$(mm_session_field "$sid_a" status)" == "3" ] || print_error "选命运后 state 应 3（待选第二命运）"
resp=$(mm_select_destiny "$token_a" "$sid_a" 5)
[ "$(extract_code "$resp")" == "0" ] || print_error "同值命运重放应幂等成功: ${resp}"
resp=$(mm_select_destiny "$token_a" "$sid_a" 4)
[ "$(extract_code "$resp")" == "6022" ] || print_error "异值命运应 6022，实际: ${resp}"
print_step "命运选择通过（同值幂等 / 异值 6022）"

# ---------- 7. 第二命运 = 手牌最频图案 ----------
print_step "7. 第二命运（手牌最频图案，最大化命运资源）"
v1=$(get_mm_round "$token_a" "$sid_a" 1)
[ "$(extract_code "$v1")" == "0" ] || print_error "回合 1 视图查询失败: ${v1}"
hand1=$(echo "$v1" | sed -n 's/.*"handCardIds":\(\[[^]]*\]\).*/\1/p')
[ -n "$hand1" ] || print_error "回合 1 应含手牌: ${v1}"
p2=$(mm_most_common_pattern "$hand1")
print_step "回合 1 手牌: ${hand1}（最频图案 ${p2}）"
resp=$(mm_select_destiny "$token_a" "$sid_a" "$p2" 1)
[ "$(extract_code "$resp")" == "0" ] || print_error "选第二命运应成功: ${resp}"
[ "$(mm_session_field "$sid_a" status)" == "4" ] || print_error "选第二命运后 state 应 4（游玩中）"
resp=$(mm_select_destiny "$token_a" "$sid_a" "$p2" 1)
[ "$(extract_code "$resp")" == "0" ] || print_error "第二命运同值重放应幂等成功: ${resp}"
print_step "第二命运通过（state 4）"

# ---------- 8. 手打 3 回合（负例 + 贪心至终局） ----------
print_step "8. 手打回合（错对 400 / 命运抽牌 6025 / 未终局 complete 6023 / 已终局 pair 6024）"
neg_tested=0
total_a=0
for round_no in 1 2 3; do
    view=$(get_mm_round "$token_a" "$sid_a" "$round_no")
    [ "$(extract_code "$view")" == "0" ] || print_error "回合 ${round_no} 视图查询失败: ${view}"
    hand=$(echo "$view" | sed -n 's/.*"handCardIds":\(\[[^]]*\]\).*/\1/p')
    term=$(extract_field "$view" terminal)
    pc0=$(extract_field "$view" pairCount)
    if [ "$term" == "true" ] && [ "$pc0" == "0" ]; then
        # 开局即终局（无对且无命运资源，罕见）：直接完成 0 对
        print_warn "回合 ${round_no} 开局即终局（0 对），直接完成"
        resp=$(mm_complete "$token_a" "$sid_a" "$round_no")
        [ "$(extract_code "$resp")" == "0" ] || print_error "终局回合完成应成功: ${resp}"
        pc_r=$(extract_field "$resp" pairCount)
        total_a=$((total_a + pc_r))
        continue
    fi

    if [ "$neg_tested" == "0" ]; then
        neg_tested=1
        # 错对（异图案两卡）→ 400
        c1=$(echo "$hand" | tr ',' '\n' | tr -d '[]' | sed 's/^ *//' | head -1)
        p1=$(( (c1 - 1) / 6 + 1 ))
        c2=$(echo "$hand" | tr ',' '\n' | tr -d '[]' | sed 's/^ *//' | while read -r card; do
            [ -z "$card" ] && continue
            if [ $(( (card - 1) / 6 + 1 )) != "$p1" ]; then echo "$card"; break; fi
        done)
        [ -n "$c2" ] || print_error "手牌应存在异图案两卡: ${hand}"
        resp=$(mm_pair "$token_a" "$sid_a" "$round_no" "$c1" "$c2")
        [ "$(extract_code "$resp")" == "400" ] || print_error "错对（${c1}/${c2}）应 400，实际: ${resp}"
        # 手牌满（开局 9 张）命运抽牌 → 6025
        resp=$(mm_destiny_draw "$token_a" "$sid_a" "$round_no")
        [ "$(extract_code "$resp")" == "6025" ] || print_error "手牌满时命运抽牌应 6025，实际: ${resp}"
        # 未终局 complete → 6023
        resp=$(mm_complete "$token_a" "$sid_a" "$round_no")
        [ "$(extract_code "$resp")" == "6023" ] || print_error "未终局 complete 应 6023，实际: ${resp}"
        print_step "负例通过（400 / 6025 / 6023）"
    fi

    # 贪心推进至终局：有对配对，无对命运抽牌（与引擎 AutoResolve 同策略）
    i=0
    while [ "$term" != "true" ]; do
        i=$((i + 1))
        [ "$i" -gt 20 ] && { print_error "回合 ${round_no} 推进超 20 步，疑似异常"; exit $FAILED; }
        pair=$(mm_pair_of_hand "$hand")
        if [ -n "$pair" ]; then
            resp=$(mm_pair "$token_a" "$sid_a" "$round_no" $pair)
            [ "$(extract_code "$resp")" == "0" ] || { print_error "配对 ${pair} 应成功: ${resp}"; exit $FAILED; }
        else
            resp=$(mm_destiny_draw "$token_a" "$sid_a" "$round_no")
            [ "$(extract_code "$resp")" == "0" ] || { print_error "命运抽牌应成功: ${resp}"; exit $FAILED; }
        fi
        hand=$(echo "$resp" | sed -n 's/.*"handCardIds":\(\[[^]]*\]\).*/\1/p')
        term=$(extract_field "$resp" terminal)
    done

    # 已终局 pair → 6024（取手牌前两卡）
    d1=$(echo "$hand" | tr ',' '\n' | tr -d '[]' | sed 's/^ *//' | head -1)
    d2=$(echo "$hand" | tr ',' '\n' | tr -d '[]' | sed 's/^ *//' | sed -n '2p')
    resp=$(mm_pair "$token_a" "$sid_a" "$round_no" "$d1" "$d2")
    [ "$(extract_code "$resp")" == "6024" ] || print_error "已终局 pair 应 6024，实际: ${resp}"

    # complete
    resp=$(mm_complete "$token_a" "$sid_a" "$round_no")
    [ "$(extract_code "$resp")" == "0" ] || print_error "回合 ${round_no} complete 应成功: ${resp}"
    pc_r=$(extract_field "$resp" pairCount)
    total_a=$((total_a + pc_r))
    print_step "回合 ${round_no} 完成（${pc_r} 对）"
    if [ "$round_no" == "3" ]; then
        [ "$(extract_field "$resp" sessionState)" == "6" ] || print_error "末回合 complete 后 sessionState 应 6，实际: ${resp}"
        [ "$(extract_field "$resp" status)" == "2" ] || print_error "终局结算 status 应 2（已发奖），实际: ${resp}"
        fhash=$(extract_field "$resp" finalHash)
        echo "$fhash" | grep -qE '^[0-9a-f]{64}$' || print_error "finalHash 应 64 位 hex，实际: ${fhash}"
        [ "$(extract_field "$resp" totalPairCount)" == "$total_a" ] || print_error "终局 total 应 ${total_a}，实际: ${resp}"
    else
        [ "$(extract_field "$resp" sessionState)" == "4" ] || print_error "回合 ${round_no} 完成后 sessionState 应 4，实际: ${resp}"
    fi
done
print_step "手打 3 回合完成，total=${total_a}"

# ---------- 9. 终局断言（会话/回合/结算/发奖/幂等重放） ----------
print_step "9. 终局断言（total=${total_a}）"
[ "$(mm_session_field "$sid_a" status)" == "6" ] || print_error "会话 status 应 6（COMPLETED）"
[ "$(mm_session_field "$sid_a" completed_rounds)" == "3" ] || print_error "completed_rounds 应 3"
for r in 1 2 3; do
    [ "$(mm_round_field "$sid_a" "$r" state)" == "3" ] || print_error "回合 ${r} 应 RESOLVED(3)"
done
sess=$(get_mm_session "$token_a" "$sid_a")
[ "$(extract_code "$sess")" == "0" ] || print_error "会话查询失败: ${sess}"
# 会话视图含会话+3 回合共 4 个 "state" 字段，用出现次数断言（extract_field 贪心取末位不可靠）
[ "$(count_field "$sess" '"state":6')" == "1" ] || print_error "会话视图 state 应 6（恰 1 处），实际: ${sess}"
[ "$(extract_field "$sess" completedRounds)" == "3" ] || print_error "会话视图 completedRounds 应 3"
[ "$(count_field "$sess" '"state":3')" == "3" ] || print_error "会话视图应 3 个 RESOLVED 回合，实际: ${sess}"

prize=$(mm_entry_prize 1 "$total_a")
kind=${prize%%|*}
rest=${prize#*|}
cash=${rest%%|*}
points=${rest#*|}
print_step "匹配奖档 kind=${kind} cash=${cash} points=${points}"
case "$kind" in
    1)
        [ "$(mm_count_settlement_ledgers "$sid_a")" == "1" ] || print_error "结算现金流水应恰 1 笔"
        [ "$(mm_sum_settlement_cash "$sid_a")" == "$cash" ] || print_error "结算现金应 ${cash} 分，实际: $(mm_sum_settlement_cash "$sid_a")"
        ;;
    2)
        [ "$(mm_count_settlement_ledgers "$sid_a")" == "1" ] || print_error "结算积分流水应恰 1 笔"
        [ "$(mm_sum_settlement_points "$sid_a")" == "$points" ] || print_error "结算积分应 ${points}，实际: $(mm_sum_settlement_points "$sid_a")"
        [ "$(get_loyalty_points "$uid_a")" == "$points" ] || print_error "A 积分应 ${points}，实际: $(get_loyalty_points "$uid_a")"
        ;;
    *)
        [ "$(mm_count_settlement_ledgers "$sid_a")" == "0" ] || print_error "无奖/谢谢参与不应有流水"
        ;;
esac
case "$total_a" in
    9) [ "$(mm_entry_remaining 1 9)" == "9" ] || print_error "9 对档库存应 10-1=9，实际: $(mm_entry_remaining 1 9)" ;;
    6) [ "$(mm_entry_remaining 1 6)" == "49" ] || print_error "6 对档库存应 50-1=49，实际: $(mm_entry_remaining 1 6)" ;;
    3) [ "$(mm_entry_remaining 1 3)" == "99" ] || print_error "3 对档库存应 100-1=99，实际: $(mm_entry_remaining 1 3)" ;;
esac
if [ "$kind" == "1" ]; then
    fen_a=$((1900 + cash))
    balance=$(wait_balance "$token_a" "$(printf '%d.%02d' $((fen_a / 100)) $((fen_a % 100)))" 20)
    [ -n "$balance" ] || print_error "A 余额应为 $(printf '%d.%02d' $((fen_a / 100)) $((fen_a % 100)))，实际: $(get_balance "$token_a")"
else
    balance=$(wait_balance "$token_a" "19.00" 20)
    [ -n "$balance" ] || print_error "A 余额应 19.00，实际: $(get_balance "$token_a")"
fi

print_step "9.1 complete 重放幂等"
ledgers_before=$(mm_count_settlement_ledgers "$sid_a")
resp=$(mm_complete "$token_a" "$sid_a" 3)
[ "$(extract_code "$resp")" == "0" ] || print_error "终局 complete 重放应成功: ${resp}"
[ "$(extract_field "$resp" sessionState)" == "6" ] || print_error "重放 sessionState 应 6"
[ "$(extract_field "$resp" totalPairCount)" == "$total_a" ] || print_error "重放 total 应 ${total_a}"
[ "$(mm_count_settlement_ledgers "$sid_a")" == "$ledgers_before" ] || print_error "重放后流水数应不变"
print_step "终局断言全部通过（发奖恰一次 + 重放幂等）"

# ---------- 10. 擂主：SQL 锻造 A/4 对 → B 购买 + skip 换擂 ----------
print_step "10. SQL 锻造擂主 A/4 对（B total≥5 即换擂）"
mm_set_champion 1 "$uid_a" 4
[ "$(mm_champion_field 1 user_id)" == "$uid_a" ] || print_error "擂主应为 A"
[ "$(mm_champion_field 1 total_pair_count)" == "4" ] || print_error "擂主纪录应 4 对"

print_step "10.1 B 购买 + skip（AutoResolve；secondDestiny=手牌最频图案，失败自动重试 ≤3 次）"
sid_b=""
total_b=0
attempts=0
token_b=""
uid_b=""
while [ "$attempts" -lt 3 ]; do
    attempts=$((attempts + 1))
    reg_b=$(register_user "e2e_mm_b${attempts}_") || exit 1
    uid_b=$(echo "$reg_b" | cut -d' ' -f1)
    name_b=$(echo "$reg_b" | cut -d' ' -f2)
    token_b=$(login_by_name "$name_b") || exit 1
    resp=$(do_post /user/recharge '{"amount":"20"}' "$token_b")
    [ "$(extract_code "$resp")" == "0" ] || print_error "B 充值失败: ${resp}"
    key_b="mm_b${attempts}_$(date +%s)"
    resp=$(mm_purchase "$token_b" 1 "$key_b" WALLET 0 0) || exit 1
    sid_b=$(extract_field "$resp" sessionId)
    # 购买即计入现任擂主分成：100 分×500bps=5 分/次
    pend=$(mm_champion_field 1 pending_income_minor)
    [ "$pend" == "$((5 * attempts))" ] || print_error "第 ${attempts} 次购买后擂主 pending 应 $((5 * attempts))，实际: ${pend}"
    resp=$(mm_select_destiny "$token_b" "$sid_b" 5)
    [ "$(extract_code "$resp")" == "0" ] || print_error "B 选命运应成功: ${resp}"
    vb1=$(get_mm_round "$token_b" "$sid_b" 1)
    handb=$(echo "$vb1" | sed -n 's/.*"handCardIds":\(\[[^]]*\]\).*/\1/p')
    p2b=$(mm_most_common_pattern "$handb")
    resp=$(mm_select_destiny "$token_b" "$sid_b" "$p2b" 1)
    [ "$(extract_code "$resp")" == "0" ] || print_error "B 选第二命运应成功: ${resp}"
    resp=$(mm_skip "$token_b" "$sid_b")
    [ "$(extract_code "$resp")" == "0" ] || print_error "B skip 应成功: ${resp}"
    [ "$(extract_field "$resp" sessionState)" == "6" ] || print_error "B skip 后 sessionState 应 6，实际: ${resp}"
    [ "$(extract_field "$resp" status)" == "2" ] || print_error "B 结算 status 应 2，实际: ${resp}"
    total_b=$(extract_field "$resp" totalPairCount)
    fhash_b=$(extract_field "$resp" finalHash)
    echo "$fhash_b" | grep -qE '^[0-9a-f]{64}$' || print_error "B finalHash 应 64 位 hex，实际: ${fhash_b}"
    print_step "第 ${attempts} 次尝试：B skip 后 total=${total_b}"
    if [ "$total_b" -ge 5 ]; then
        break
    fi
    print_warn "total<5（罕见），换新用户重试"
    sid_b=""
done
[ -n "$sid_b" ] || { print_error "3 次尝试均未达 5 对，场景失败"; exit $FAILED; }

print_step "10.2 换擂断言：B 成为擂主、A 获 pending 打款恰一次"
[ "$(mm_champion_field 1 user_id)" == "$uid_b" ] || print_error "换擂后擂主应为 B，实际: $(mm_champion_field 1 user_id)"
[ "$(mm_champion_field 1 total_pair_count)" == "$total_b" ] || print_error "擂主纪录应为 ${total_b} 对"
[ "$(mm_champion_field 1 pending_income_minor)" == "0" ] || print_error "换擂后 pending 应清零"
[ "$(mm_count_champion_events "$sid_b")" == "1" ] || print_error "换擂事件应恰 1 条"
[ "$(mm_count_champion_payouts 1)" == "1" ] || print_error "换擂打款应恰 1 笔"
[ "$(mm_payout_user "$sid_b")" == "$uid_a" ] || print_error "打款收款人应为 A"
[ "$(mm_payout_status "$sid_b")" == "2" ] || print_error "打款应 PAID(2)"
pend_final=$((5 * attempts))
[ "$(mm_sum_champion_cash "$sid_b")" == "$pend_final" ] || print_error "打款应 ${pend_final} 分，实际: $(mm_sum_champion_cash "$sid_b")"
[ "$(mm_count_champion_ledgers "$sid_b")" == "1" ] || print_error "打款流水应恰 1 笔"

# B 的结算奖（自适应断言）
prize_b=$(mm_entry_prize 1 "$total_b")
kind_b=${prize_b%%|*}
rest_b=${prize_b#*|}
cash_b=${rest_b%%|*}
points_b=${rest_b#*|}
print_step "B 匹配奖档 kind=${kind_b} cash=${cash_b} points=${points_b}"
case "$kind_b" in
    1)
        [ "$(mm_count_settlement_ledgers "$sid_b")" == "1" ] || print_error "B 结算现金流水应恰 1 笔"
        [ "$(mm_sum_settlement_cash "$sid_b")" == "$cash_b" ] || print_error "B 结算现金应 ${cash_b} 分"
        fen_b=$((1900 + cash_b))
        balance=$(wait_balance "$token_b" "$(printf '%d.%02d' $((fen_b / 100)) $((fen_b % 100)))" 20)
        [ -n "$balance" ] || print_error "B 余额应为 $(printf '%d.%02d' $((fen_b / 100)) $((fen_b % 100)))，实际: $(get_balance "$token_b")"
        ;;
    2)
        [ "$(mm_count_settlement_ledgers "$sid_b")" == "1" ] || print_error "B 结算积分流水应恰 1 笔"
        [ "$(mm_sum_settlement_points "$sid_b")" == "$points_b" ] || print_error "B 结算积分应 ${points_b}"
        [ "$(get_loyalty_points "$uid_b")" == "$points_b" ] || print_error "B 积分应 ${points_b}，实际: $(get_loyalty_points "$uid_b")"
        balance=$(wait_balance "$token_b" "19.00" 20)
        [ -n "$balance" ] || print_error "B 余额应 19.00，实际: $(get_balance "$token_b")"
        ;;
    *)
        [ "$(mm_count_settlement_ledgers "$sid_b")" == "0" ] || print_error "B 无奖不应有流水"
        balance=$(wait_balance "$token_b" "19.00" 20)
        [ -n "$balance" ] || print_error "B 余额应 19.00，实际: $(get_balance "$token_b")"
        ;;
esac

# A 余额 = 1900 + 现金奖 + 打款分成
cash_a=0
if [ "$kind" == "1" ]; then cash_a=$cash; fi
fen_a=$((1900 + cash_a + pend_final))
balance=$(wait_balance "$token_a" "$(printf '%d.%02d' $((fen_a / 100)) $((fen_a % 100)))" 20)
[ -n "$balance" ] || print_error "A 余额应为 $(printf '%d.%02d' $((fen_a / 100)) $((fen_a % 100)))（含分成 ${pend_final} 分），实际: $(get_balance "$token_a")"

print_step "10.3 skip 重放幂等"
ledgers_b_before=$(mm_count_settlement_ledgers "$sid_b")
resp=$(mm_skip "$token_b" "$sid_b")
[ "$(extract_code "$resp")" == "0" ] || print_error "B skip 重放应成功: ${resp}"
[ "$(extract_field "$resp" totalPairCount)" == "$total_b" ] || print_error "B 重放 total 应 ${total_b}"
[ "$(mm_count_settlement_ledgers "$sid_b")" == "$ledgers_b_before" ] || print_error "B 重放后流水数应不变"
[ "$(mm_count_champion_payouts 1)" == "1" ] || print_error "B 重放后打款应仍 1 笔"
print_step "擂主换擂全链路通过"

# ---------- 11. 越权读会话 → 404 ----------
print_step "11. 越权：B 读 A 的会话 → 404"
resp=$(get_mm_session "$token_b" "$sid_a")
[ "$(extract_code "$resp")" == "404" ] || print_error "越权读会话应 404，实际: ${resp}"

# ---------- 12. 外观：默认 OK / 非默认无权益 6027 / 插权益 OK ----------
print_step "12. 外观（默认 OK → 非默认无权益 6027 → SQL 插权益 → OK）"
resp=$(mm_appearance_get "$token_a" 1)
[ "$(extract_code "$resp")" == "0" ] || print_error "外观读取失败: ${resp}"
[ "$(extract_field "$resp" facePresetId)" == "1" ] || print_error "默认 facePresetId 应 1，实际: ${resp}"
[ "$(extract_field "$resp" backPresetId)" == "1" ] || print_error "默认 backPresetId 应 1，实际: ${resp}"
resp=$(mm_appearance_put "$token_a" 1 2 1)
[ "$(extract_code "$resp")" == "6027" ] || print_error "非默认无权益应 6027，实际: ${resp}"
mm_insert_entitlement 1 "$uid_a" 2
[ "$(mm_count_entitlements 1 "$uid_a")" == "1" ] || print_error "权益应插入 1 条"
resp=$(mm_appearance_put "$token_a" 1 2 1)
[ "$(extract_code "$resp")" == "0" ] || print_error "有权益后保存应成功: ${resp}"
[ "$(extract_field "$resp" facePresetId)" == "2" ] || print_error "保存后 facePresetId 应 2，实际: ${resp}"
resp=$(mm_appearance_get "$token_a" 1)
[ "$(extract_field "$resp" facePresetId)" == "2" ] || print_error "读取 facePresetId 应 2"
[ "$(count_field "$resp" '"ownedFacePresetIds":\[2')" == "1" ] || print_error "ownedFacePresetIds 应含 2，实际: ${resp}"
print_step "外观链路通过"

# ---------- 13. 收尾：现场复位 ----------
print_step "13. 收尾：记忆翻牌现场复位"
reset_memory_match 1
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_play.memory_match_sessions WHERE config_id=1;")" == "0" ] || print_error "复位后会话应为 0"
[ "$(mm_champion_field 1 user_id)" == "" ] || print_error "复位后应无擂主"
[ "$(mm_entry_remaining 1 9)" == "10" ] || print_error "复位后 9 对档库存应 10"
[ "$(mm_entry_remaining 1 6)" == "50" ] || print_error "复位后 6 对档库存应 50"
[ "$(mm_entry_remaining 1 3)" == "100" ] || print_error "复位后 3 对档库存应 100"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景30 memory_match 通过${NC}"
else
    echo -e "${RED}× 场景30 memory_match 失败${NC}"
fi
exit $FAILED
