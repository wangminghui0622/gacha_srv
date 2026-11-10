#!/bin/bash
# ============================================================
# 场景2：余额不足 → 扣款分支 Aborted → 无记录、余额分毫不动
#
# 流程：注册 → 充值0.01元 → 买 boxId=2(10.00元)
# 选盒说明：box 3（传说宝箱）库存会被场景3清零且恢复动作在其后，场景2 若依赖
#           box 3 会在二次连跑时被售罄预检 2002 拦截；box 2 无任何场景清库存，状态稳定。
# 断言：
#   1. 购买响应 code=0（Submit 成功、SAGA 异步执行）
#   2. 轮询抽赏记录超时（未产生记录）
#   3. 余额仍为 0.01（扣款分支因余额不足 Aborted，未扣钱）
#   4. draw_records 0 条
#   5. M8 支付域：正向 barrier 事务随 Aborted 整体回滚 → payment_orders 0 条、
#      wallet_holds 0 条（冻结失败无泄漏）
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景2：余额不足（insufficient_balance）"

# ---------- 1. 注册 + 充值 ----------
print_step "1. 注册用户并充值 0.01 元"
reg=$(register_user "e2e_poor_") || exit 1
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge "{\"amount\":\"0.01\"}" "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
balance=$(extract_field "$resp" balance)
[ "$(fen_of "$balance")" == "1" ] || print_error "充值后余额应为 0.01，实际: ${balance}"

# ---------- 2. 购买（余额 0.01 远不够 10.00） ----------
print_step "2. 购买 boxId=2（价格 10.00 元）"
buy_resp=$(do_post /box/buy '{"boxId":2}' "$token")
[ "$(extract_code "$buy_resp")" == "0" ] || print_error "购买提交失败: ${buy_resp}"
rid=$(extract_field "$buy_resp" requestId)
print_step "requestId: ${rid}（SAGA 扣款分支将因余额不足 Aborted）"

# ---------- 3. 轮询：必然无记录 ----------
print_step "3. 轮询抽赏记录（5s 内不应出现）"
record=$(poll_record "$token" "$rid" 5)
[ -z "$record" ] || print_error "余额不足却产生了抽赏记录: ${record}"

# ---------- 4. 余额分毫不动 ----------
print_step "4. 校验余额仍为 0.01（未扣款）"
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "1" ] || print_error "余额应为 0.01 分毫不动，实际: ${balance}"

# ---------- 5. 数据库断言 ----------
print_step "5. 数据库断言：draw_records 0 条"
rec_count=$(count_records "$rid")
[ "$rec_count" == "0" ] || print_error "draw_records 应为 0 条，实际: ${rec_count}"

# ---------- 6. 支付域断言（M8） ----------
print_step "6. 支付域断言：正向事务回滚 → 无支付单、无冻结泄漏"
sleep 2  # 等 DTM 补偿收尾（CancelPayment 空补偿/清扫）
pcount=$(count_payment_orders "$rid")
[ "$pcount" == "0" ] || print_error "payment_orders 应为 0 条（Aborted 回滚），实际: ${pcount}"
hcount=$(count_wallet_holds "biz_type='payment_leg' AND biz_id LIKE 'P${rid}%'")
[ "$hcount" == "0" ] || print_error "wallet_holds 应为 0 条（冻结失败无泄漏），实际: ${hcount}"
print_step "支付域断言通过"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景2 insufficient_balance 通过${NC}"
else
    echo -e "${RED}× 场景2 insufficient_balance 失败${NC}"
fi
exit $FAILED
