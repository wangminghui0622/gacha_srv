#!/bin/bash
# ============================================================
# 并发压测（VM 本机 127.0.0.1 直打网关 9310）
#
# 用例：
#   C1 幂等并发：同 Idempotency-Key × 20 并发买盒 → 恰 1 单/1 支付/1 记录
#   C2 钱包并发：余额 5 元 + 20 个不同 key 并发买 1 元盒 → 恰 5 成功、无负余额、无挂起冻结
#   C3 限量超发：传说坐骑 remaining=3 + 权重 1000，30 并发买传说宝箱 → 恰 3 中、不超发
#   C4 满车占座：4 座 × 20 并发（轮转抢同座）→ 恰 4 成功、不重座、输家全额退款
#   C5 扭蛋超发：6 票 × 20 并发 → 恰 6 成功、余票=0 不为负、奖项现金对账
#   C6 挖宝同用户格冲突：5 并发买格 0 → 恰 1 成功、bitmap 不破坏
#   G  全局不变量：DTM 无新 stuck、钱包无负/无挂起、台账=余额对账
#
# 运行：cd ~/gacha/test && env DB_PASS=we7root MYSQL_BIN=/usr/bin/mysql \
#      REDIS_CLI=/usr/bin/redis-cli bash conc_stress.sh
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

CASE_TS=$(date +%s)
WORK="/tmp/conc_${CASE_TS}"
mkdir -p "$WORK"
PASS=0
case_pass() { echo -e "${GREEN}RESULT $1: PASS${NC}"; PASS=$((PASS+1)); }
case_fail() { echo -e "${RED}RESULT $1: FAIL${NC}"; }
case_begin() { CF0=$FAILED; }
case_done() { if [ "$FAILED" == "$CF0" ]; then case_pass "$1"; else case_fail "$1"; fi; }

print_title "并发压测（CASE=${CASE_TS}，工作目录 ${WORK}）"

# ---------- 公共工具 ----------

# mk_users <n> <前缀> <充值元> <输出文件>：并行(≤8)注册+登录+充值，行格式 "uid name token"
mk_users() {
    local n=$1 pfx=$2 yuan=$3 out=$4
    : > "$out"
    local i
    for i in $(seq 1 "$n"); do
        (
            local reg uid name tok resp
            reg=$(register_user "$pfx") || exit 1
            uid=$(echo "$reg" | cut -d' ' -f1)
            name=$(echo "$reg" | cut -d' ' -f2)
            tok=$(login_by_name "$name") || exit 1
            resp=$(do_post /user/recharge "{\"amount\":\"${yuan}\"}" "$tok")
            [ "$(extract_code "$resp")" == "0" ] || exit 1
            echo "$uid $name $tok" >> "$out"
        ) &
        while [ "$(jobs -r | wc -l)" -ge 8 ]; do wait -n; done
    done
    wait
    [ "$(wc -l < "$out")" == "$n" ] || { print_error "用户创建不足: $(wc -l < "$out")/$n"; return 1; }
}

load_users() { # <文件> → U_IDS[1..n] U_TOK[1..n]，输出 n
    local f=$1 i=0 uid name tok
    U_IDS=(); U_TOK=()
    while read -r uid name tok; do
        i=$((i+1)); U_IDS[$i]=$uid; U_TOK[$i]=$tok
    done < "$f"
    echo "$i"
}

uid_list() { # [n] → 'a','b'…（U_IDS[1..n]）
    local n=$1 out="" i
    for i in $(seq 1 "$n"); do out="${out}'${U_IDS[$i]}',"; done
    echo "${out%,}"
}

# fire <tag> <n> <workerFn> [固定参数...]：n 个 worker 自旋等屏障后同时发起
fire() {
    local tag=$1 n=$2 fn=$3; shift 3
    local GO="$WORK/go_${tag}"; rm -f "$GO"
    local i
    for i in $(seq 1 "$n"); do
        (
            while [ ! -e "$GO" ]; do sleep 0.005; done
            "$fn" "$@" "$i" > "$WORK/${tag}_$i.out" 2>&1
        ) &
    done
    sleep 0.6
    : > "$GO"
    wait
}

# collect <tag> <n> → OK_N（code==0 数）、RID_ARR（非空 requestId 列表）、RID_IN（SQL IN 串）
collect() {
    local tag=$1 n=$2 i resp code rid
    OK_N=0; RID_ARR=(); RID_IN=""
    for i in $(seq 1 "$n"); do
        resp=$(cat "$WORK/${tag}_$i.out" 2>/dev/null)
        code=$(extract_code "$resp")
        [ "$code" == "0" ] && OK_N=$((OK_N+1))
        rid=$(extract_field "$resp" requestId)
        if [ -n "$rid" ]; then RID_ARR+=("$rid"); RID_IN="${RID_IN}'${rid}',"; fi
    done
    RID_IN="${RID_IN%,}"
}

# wait_sql <期望值> <超时秒> <SQL>：轮询 COUNT 类 SQL 至等于期望
wait_sql() {
    local expect=$1 max=$2 sql=$3 i=0 c
    while [ "$i" -lt "$max" ]; do
        c=$(do_mysql "$sql")
        [ "$c" == "$expect" ] && return 0
        sleep 1
        i=$((i+1))
    done
    print_error "  [超时] 实际=$(do_mysql "$sql") 期望=${expect}（${i}s）"
    return 1
}

# ---------- 工作函数 ----------

wk_box_samekey() { do_post /box/buy "{\"boxId\":${WK_BOX}}" "$1" "$2"; }            # 固定 token/key
wk_box_w2()      { do_post /box/buy '{"boxId":1}' "$W2_TOK" "w2_${CASE_TS}_$1"; }   # 固定用户、独立 key
wk_box_u()       { do_post /box/buy "{\"boxId\":${WK_BOX}}" "${U_TOK[$1]}" "u${WK_BOX}_${CASE_TS}_$1"; } # 每 worker 一用户
wk_fc()          { do_post "/full-car/1/buy" "{\"seatIndex\":$(( $1 % 4 ))}" "${U_TOK[$1]}" "fc_${CASE_TS}_$1"; }
wk_ga()          { do_post "/gashapon/1/buy" '{"count":1}' "${U_TOK[$1]}" "ga_${CASE_TS}_$1"; }
wk_dg()          { do_post "/dig-boxes/1/buy" '{"boxId":1,"cellIndexes":[0]}' "$DG_TOK" "dg_${CASE_TS}_$1"; }

sum_rids() { # <SQL 模板（RIDD 占位）> → 对 RID_ARR 求和
    local tmpl=$1 total=0 r v
    for r in "${RID_ARR[@]}"; do
        v=$(do_mysql "${tmpl//RIDD/$r}")
        total=$((total + ${v:-0}))
    done
    echo "$total"
}

# ============================================================
print_step "C1：同 Idempotency-Key × 20 并发买盒（幂等竞态）"
case_begin
mk_users 1 "cc_idem_" 5 "$WORK/u_c1.txt" || exit 1
read -r C1_UID C1_NAME C1_TOK < "$WORK/u_c1.txt"
KEY1="idem_${CASE_TS}"
WK_BOX=1
fire idem 20 wk_box_samekey "$C1_TOK" "$KEY1"
collect idem 20
[ "$OK_N" == "20" ] || print_error "C1 并发响应应全 code=0，实际 ${OK_N}/20"
DISTINCT_RID=$(printf '%s\n' "${RID_ARR[@]}" | sort -u | wc -l)
[ "$DISTINCT_RID" == "1" ] || print_error "C1 requestId 应唯一，实际 ${DISTINCT_RID} 个: ${RID_ARR[*]:0:5}…"
RID1=${RID_ARR[0]}
[ "$(count_payment_orders "$RID1")" == "1" ] || print_error "C1 支付单应恰 1，实际 $(count_payment_orders "$RID1")"
wait_payment_paid "$RID1" 30 || print_error "C1 支付未 PAID"
[ "$(count_records "$RID1")" == "1" ] || print_error "C1 抽赏记录应恰 1，实际 $(count_records "$RID1")"
case_done "C1 idem-concurrent"

# ============================================================
print_step "C2：余额 5 元 + 20 并发买 1 元盒（钱包扣减竞态）"
case_begin
mk_users 1 "cc_wal_" 5 "$WORK/u_c2.txt" || exit 1
read -r W2_UID W2_NAME W2_TOK < "$WORK/u_c2.txt"
fire w2 20 wk_box_w2
collect w2 20
W2_RECSQL="SELECT COUNT(*) FROM gacha_draw.draw_records WHERE request_id IN (${RID_IN})"
wait_sql 5 90 "$W2_RECSQL" || print_error "C2 成功记录应恰 5（20 并发抢 5 元）"
PAY_IN="SELECT COUNT(*) FROM gacha_money.payment_orders WHERE payment_no IN (${RID_IN})"
[ "$(do_mysql "$PAY_IN")" == "5" ] || print_error "C2 支付单应恰 5，实际 $(do_mysql "$PAY_IN")"
[ "$(do_mysql "${PAY_IN} AND status=3")" == "5" ] || print_error "C2 PAID 支付单应恰 5"
BAL2=$(get_cash_fen "$W2_UID")
[ "$BAL2" -ge 0 ] || print_error "C2 余额为负: ${BAL2}"
HELD2=$(do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_accounts WHERE user_id=${W2_UID} AND held<>0")
[ "$HELD2" == "0" ] || print_error "C2 结束后仍有 ${HELD2} 笔冻结未释放"
echo "  成功=$(do_mysql "$W2_RECSQL") 余额=${BAL2}分（含奖品回流，应≥0）"
case_done "C2 wallet-concurrent"

# ============================================================
print_step "C3：传说坐骑 remaining=3 + 权重 1000，30 并发买传说宝箱（限量超发）"
case_begin
set_entry_attr 3 传说坐骑 weight 1000
set_entry_attr 3 传说坐骑 remaining 3
flush_box_entries_cache 3
mk_users 30 "cc_mt_" 100 "$WORK/u_c3.txt" || exit 1
N3=$(load_users "$WORK/u_c3.txt")
WK_BOX=3
fire mt 30 wk_box_u
collect mt 30
MOUNT_SQL="SELECT COUNT(*) FROM gacha_draw.draw_records WHERE request_id IN (${RID_IN}) AND entry_name='传说坐骑'"
wait_sql 3 120 "$MOUNT_SQL" || print_error "C3 传说坐骑应恰发 3 次"
REMAIN3=$(get_entry_remaining 3 传说坐骑)
[ "$REMAIN3" == "0" ] || print_error "C3 剩余库存应为 0（不为负），实际 ${REMAIN3}"
WIN_SQL="SELECT COUNT(*) FROM gacha_draw.draw_records WHERE request_id IN (${RID_IN})"
W3_RECS=$(do_mysql "$WIN_SQL")
UID3_IN=$(uid_list 30)
WINNERS3=$(do_mysql "SELECT GROUP_CONCAT(DISTINCT user_id) FROM gacha_draw.draw_records WHERE request_id IN (${RID_IN})")
LOSE_SQL="SELECT COUNT(*) FROM gacha_money.wallet_accounts WHERE wallet_kind=1 AND user_id IN (${UID3_IN}) AND user_id NOT IN (${WINNERS3:-0}) AND available<>10000"
wait_sql 0 120 "$LOSE_SQL" || print_error "C3 未中奖用户应全额退款（余额 100 元不变），违规 $(do_mysql "$LOSE_SQL") 人"
echo "  坐骑发放=$(do_mysql "$MOUNT_SQL") 成功记录=${W3_RECS} 库存余=${REMAIN3}"
set_entry_attr 3 传说坐骑 weight 1
set_entry_attr 3 传说坐骑 remaining 5
flush_box_entries_cache 3
case_done "C3 oversell-limited-entry"

# ============================================================
print_step "C4：满车 4 座 × 20 并发轮转抢座（座位竞态 + 终局结算）"
case_begin
set_full_car_entry_stock 1 金币×50 5000
reset_full_car_state 1
mk_users 20 "cc_fc_" 1 "$WORK/u_c4.txt" || exit 1
N4=$(load_users "$WORK/u_c4.txt")
fire fc 20 wk_fc
collect fc 20
FC_RECSQL="SELECT COUNT(*) FROM gacha_play.full_car_records WHERE request_id IN (${RID_IN})"
wait_sql 4 120 "$FC_RECSQL" || print_error "C4 满车应恰 4 成功，实际 $(do_mysql "$FC_RECSQL")"
wait_sql 1 30 "SELECT COUNT(*) FROM gacha_play.full_car_state WHERE config_id=1 AND remaining_tickets=0 AND status=2" \
    || print_error "C4 满车状态异常: 余票=$(get_full_car_state_field 1 remaining_tickets) 状态=$(get_full_car_state_field 1 status)"
SEATS=$(get_full_car_state_field 1 seats_json)
SEAT_ZEROS=$(echo "$SEATS" | tr -d '[] ' | tr ',' '\n' | grep -c '^0$')
SEAT_TOTAL=$(echo "$SEATS" | tr -d '[] ' | tr ',' '\n' | grep -c '.')
[ "$SEAT_ZEROS" == "0" ] && [ "$SEAT_TOTAL" == "4" ] || print_error "C4 座位分配异常: ${SEATS}（空座${SEAT_ZEROS}/总${SEAT_TOTAL}）"
# 每个中奖 uid 恰占一座
DUP=0
for i in $(seq 1 20); do
    c=$(echo "$SEATS" | grep -o "${U_IDS[$i]}" | wc -l)
    [ "$c" -gt 1 ] && DUP=1
done
[ "$DUP" == "0" ] || print_error "C4 存在一人多座: ${SEATS}"
FC_WIN=$(do_mysql "SELECT GROUP_CONCAT(DISTINCT user_id) FROM gacha_play.full_car_records WHERE request_id IN (${RID_IN})")
UID4_IN=$(uid_list 20)
LOSE4_SQL="SELECT COUNT(*) FROM gacha_money.wallet_accounts WHERE wallet_kind=1 AND user_id IN (${UID4_IN}) AND user_id NOT IN (${FC_WIN:-0}) AND available<>100"
wait_sql 0 120 "$LOSE4_SQL" || print_error "C4 落座失败者应全额退款，违规 $(do_mysql "$LOSE4_SQL") 人"
AW4=$(get_full_car_state_field 1 awards_json)
[ "$(count_field "$AW4" '"seat_index":')" == "4" ] || print_error "C4 整车奖应恰 4 条"
CASH4_AW=$(sum_json_field "$AW4" cash_fen)
CASH4_LED=$(sum_rids "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE wallet_kind=1 AND log_no LIKE 'c_fcp_RIDD#%'")
[ "$CASH4_AW" == "$CASH4_LED" ] || print_error "C4 整车现金奖对账不平: awards=${CASH4_AW} 流水=${CASH4_LED}"
echo "  成功=$(do_mysql "$FC_RECSQL") seats=${SEATS} 现金奖=${CASH4_AW}分"
reset_full_car_state 1
case_done "C4 fullcar-seat-race"

# ============================================================
print_step "C5：扭蛋 6 票 × 20 并发（票据超发 + 终局奖项对账）"
case_begin
reset_gashapon_state 1
mk_users 20 "cc_ga_" 1 "$WORK/u_c5.txt" || exit 1
N5=$(load_users "$WORK/u_c5.txt")
fire ga 20 wk_ga
collect ga 20
GA_RECSQL="SELECT COUNT(*) FROM gacha_play.gashapon_records WHERE request_id IN (${RID_IN})"
wait_sql 6 120 "$GA_RECSQL" || print_error "C5 扭蛋应恰 6 成功，实际 $(do_mysql "$GA_RECSQL")"
REMT=$(get_gashapon_state_field 1 remaining_tickets)
[ "$REMT" == "0" ] || print_error "C5 余票应为 0（不为负），实际 ${REMT}"
[ "$(get_gashapon_state_field 1 status)" == "2" ] || print_error "C5 状态应 SETTLED(2)，实际 $(get_gashapon_state_field 1 status)"
PARTS=$(get_gashapon_state_field 1 participants_json)
PARTS_SUM=$(echo "$PARTS" | grep -o ':[0-9]*' | sed 's/://' | awk '{s+=$1} END{print s+0}')
[ "$PARTS_SUM" == "6" ] || print_error "C5 participants 票数和应为 6，实际 ${PARTS_SUM}: ${PARTS}"
RECS=$(get_gashapon_state_field 1 records_json)
[ "$(count_field "$RECS" '"count":1')" == "6" ] || print_error "C5 records_json 应恰 6 条票据"
GA_WIN=$(do_mysql "SELECT GROUP_CONCAT(DISTINCT user_id) FROM gacha_play.gashapon_records WHERE request_id IN (${RID_IN})")
UID5_IN=$(uid_list 20)
LOSE5_SQL="SELECT COUNT(*) FROM gacha_money.wallet_accounts WHERE wallet_kind=1 AND user_id IN (${UID5_IN}) AND user_id NOT IN (${GA_WIN:-0}) AND available<>100"
wait_sql 0 120 "$LOSE5_SQL" || print_error "C5 落票失败者应全额退款，违规 $(do_mysql "$LOSE5_SQL") 人"
AW5=$(get_gashapon_state_field 1 awards_json)
CASH5_AW=$(sum_json_field "$AW5" cash_fen)
PTS5_AW=$(sum_json_field "$AW5" points_amount)
CASH5_LED=$(sum_rids "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE wallet_kind=1 AND log_no LIKE 'c_gcp_RIDD#%'")
PTS5_LED=$(sum_rids "SELECT IFNULL(SUM(amount),0) FROM gacha_money.wallet_ledgers WHERE wallet_kind=4 AND log_no LIKE 'c_gcp_RIDD#%'")
[ "$CASH5_AW" == "$CASH5_LED" ] || print_error "C5 特别奖现金对账不平: awards=${CASH5_AW} 流水=${CASH5_LED}"
[ "$PTS5_AW" == "$PTS5_LED" ] || print_error "C5 特别奖积分对账不平: awards=${PTS5_AW} 流水=${PTS5_LED}"
echo "  成功=$(do_mysql "$GA_RECSQL") 余票=${REMT} 现金奖=${CASH5_AW}分 积分奖=${PTS5_AW}"
reset_gashapon_state 1
case_done "C5 gashapon-oversell"

# ============================================================
print_step "C6：同一用户 5 并发买挖宝格 0（会话 bitmap 竞态）"
case_begin
mk_users 1 "cc_dg_" 2 "$WORK/u_c6.txt" || exit 1
read -r DG_UID DG_NAME DG_TOK < "$WORK/u_c6.txt"
fire dg 5 wk_dg
collect dg 5
DG_RECSQL="SELECT COUNT(*) FROM gacha_play.dig_box_records WHERE request_id IN (${RID_IN})"
wait_sql 1 90 "$DG_RECSQL" || print_error "C6 挖宝应恰 1 成功，实际 $(do_mysql "$DG_RECSQL")"
BM=$(get_dig_session_field "$DG_UID" 1 occupied_bitmap)
[ "$BM" == "1" ] || print_error "C6 bitmap 应为 1（恰格 0），实际 ${BM}"
[ "$(get_dig_session_field "$DG_UID" 1 total_cells_paid)" == "1" ] || print_error "C6 已购格数应为 1"
balance=$(wait_balance "$DG_TOK" "2.00" 30)
[ -n "$balance" ] || print_error "C6 余额应回到 2.00（输家退款+赢家付1得1），实际 $(get_balance "$DG_TOK")"
PRZ=$(sum_rids "SELECT COUNT(*) FROM gacha_money.wallet_ledgers WHERE log_no LIKE 'c_dgp_RIDD#%'")
[ "$PRZ" == "1" ] || print_error "C6 格 0 现金奖应恰发 1 次，实际 ${PRZ}"
echo "  成功=$(do_mysql "$DG_RECSQL") bitmap=${BM} 奖流水=${PRZ}"
case_done "C6 digbox-cell-race"

# ============================================================
print_title "G：全局不变量（DTM 无新 stuck / 钱包无负无冻结 / 台账对账）"
case_begin
print_step "G1 DTM 无新增未终结事务"
DTM_STUCK=$(do_mysql "SELECT COUNT(*) FROM gacha_dtm.trans_global WHERE status IN ('prepared','submitted')")
[ "$DTM_STUCK" == "0" ] || print_error "G1 DTM 存在 ${DTM_STUCK} 笔未终结事务"

print_step "G2 钱包无负余额、测试用户无残留冻结"
NEG=$(do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_accounts WHERE available<0 OR held<0")
[ "$NEG" == "0" ] || print_error "G2 存在 ${NEG} 行负余额/负冻结"
ALL_UIDS=$(cat "$WORK"/u_c*.txt | cut -d' ' -f1 | sed "s/.*/'&'/" | paste -sd, -)
HELDG=$(do_mysql "SELECT COUNT(*) FROM gacha_money.wallet_accounts WHERE user_id IN (${ALL_UIDS}) AND held<>0")
[ "$HELDG" == "0" ] || print_error "G2 测试用户残留 ${HELDG} 行未释放冻结"

print_step "G3 台账对账：available == Σ流水（kind 1/2/4）"
BADG=$(do_mysql "SELECT COUNT(*) FROM (
        SELECT a.user_id, a.wallet_kind, a.available - COALESCE(SUM(l.amount),0) d
        FROM gacha_money.wallet_accounts a
        LEFT JOIN gacha_money.wallet_ledgers l ON l.user_id=a.user_id AND l.wallet_kind=a.wallet_kind
        WHERE a.user_id IN (${ALL_UIDS}) AND a.wallet_kind IN (1,2,4)
        GROUP BY a.user_id, a.wallet_kind, a.available
      ) t WHERE d<>0")
[ "$BADG" == "0" ] || print_error "G3 台账与余额不对账 ${BADG} 行"
case_done "G global-invariants"

# ---------- 汇总 ----------
echo ""
echo "============================================================"
if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}并发压测全部通过：${PASS}/7（C1-C6 + G）${NC}"
else
    echo -e "${RED}并发压测有失败项：通过 ${PASS}/7${NC}"
fi
echo "============================================================"
rm -rf "$WORK"
exit $FAILED
