#!/bin/bash
# ============================================================
# 场景10：捕获故障注入 —— CaptureWalletPayment 人为阻塞 → DTM 退避重试 → 最终 PAID 且捕获恰一次（M8 验收）
#
# 注入方式（两段锁接力，制造 07 分支独占的故障窗口）：
#   锁A：后台 MySQL 会话对 draw_boxes(id=1) 加行锁持有 72s —— 拖住 05 SettleDraw
#        （与场景6 同款；05 阻塞 50s 锁等待超时 → Internal → DTM 重试）。
#        05 被拖住期间支付单稳定处于 AWAITING_PAYMENT（04 已建单冻结完成）。
#   锁B：测试在观察到 AWAITING 后，立刻对 payment_orders(payment_no) 加行锁持有 130s。
#        锁A 释放后 05 重试成功 → 07 CaptureWalletPayment 撞上锁B → 50s 超时
#        → Internal → DTM 退避重试 → 锁B 释放后重试成功 → 订单 PAID。
#
# 断言：
#   1. 故障窗口内购买提交成功（Submit 返回 pending，DTM 接管重试）
#   2. 轮询出抽赏记录（05 在锁A 释放后重试成功）
#   3. 支付单最终 PAID(3)、CASH 腿 POSTED(2)、hold POSTED(2)（捕获恰一次，uk 防重）
#   4. 余额恰好扣一次（10.00 → 9.00），无退款（有退款则余额回到 10.00）
#   5. freeze 流水恰 1 条、无 unfreeze 流水（未走补偿路径）
#
# 与场景6 的区别：场景6 故障打在开奖分支（05）；本场景故障打在捕获分支（07），
# 验证 07 的 Internal→重试→幂等收敛（PostHold 幂等 + 盲更新）。
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景10：捕获故障注入（CaptureWalletPayment 阻塞 → DTM 重试 → PAID 恰一次）"

LOCK_A_SECONDS=72
LOCK_B_SECONDS=130

# ---------- 1. 注册 + 充值 ----------
print_step "1. 注册用户并充值 10 元"
reg=$(register_user "capture_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge "{\"amount\":\"10\"}" "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
print_step "充值后余额: $(extract_field "$resp" balance)"

# ---------- 2. 锁A：后台会话持有 box1 行锁（拖住 05） ----------
print_step "2. 锁A：后台 MySQL 会话锁定 draw_boxes(id=1) ${LOCK_A_SECONDS} 秒"
"$MYSQL_BIN" -h127.0.0.1 -uroot -p"${DB_PASS}" --default-character-set=utf8mb4 -N -B -e \
    "BEGIN; SELECT id FROM gacha_draw.draw_boxes WHERE id=1 FOR UPDATE; DO SLEEP(${LOCK_A_SECONDS}); COMMIT;" \
    >/dev/null 2>&1 &
LOCK_A_PID=$!
sleep 3   # 等后台会话连接并拿到锁
print_step "锁A 已就绪（PID ${LOCK_A_PID}）"

# ---------- 3. 故障窗口内购买 ----------
print_step "3. 购买 box1：04 建单冻结完成、05 阻塞在锁A"
buy_resp=$(do_post /box/buy '{"boxId":1}' "$token")
[ "$(extract_code "$buy_resp")" == "0" ] || print_error "购买提交失败: ${buy_resp}"
rid=$(extract_field "$buy_resp" requestId)
[ -n "$rid" ] || print_error "requestId 不应为空"
print_step "SAGA 已提交: requestId=${rid}"

# ---------- 4. 等 04 完成（AWAITING）后立刻上锁B ----------
print_step "4. 等支付单到达 AWAITING(2)，随即锁B 持有 payment_orders 行 ${LOCK_B_SECONDS} 秒"
st=$(get_payment_status "$rid")
i=0
while [ "$st" != "2" ] && [ "$i" -lt 40 ]; do
    sleep 0.5
    i=$((i + 1))
    st=$(get_payment_status "$rid")
done
if [ "$st" != "2" ]; then
    print_error "支付单未在 20s 内到达 AWAITING，实际: ${st:-<不存在>}"
    wait "$LOCK_A_PID" 2>/dev/null
    exit $FAILED
fi
print_step "04 已完成，支付单 AWAITING；锁B 就位"
"$MYSQL_BIN" -h127.0.0.1 -uroot -p"${DB_PASS}" --default-character-set=utf8mb4 -N -B -e \
    "BEGIN; SELECT payment_no FROM gacha_money.payment_orders WHERE payment_no='$(payment_no "$rid")' FOR UPDATE; DO SLEEP(${LOCK_B_SECONDS}); COMMIT;" \
    >/dev/null 2>&1 &
LOCK_B_PID=$!
sleep 3   # 等锁B 会话连接并拿到行锁
print_step "锁B 已就绪（PID ${LOCK_B_PID}）"

# ---------- 5. 轮询抽赏记录（锁A 释放后 05 重试成功） ----------
print_step "5. 轮询抽赏记录（锁A 释放后 05 重试应成功，最长等 240s）"
record=$(poll_record "$token" "$rid" 240)
if [ -z "$record" ]; then
    print_error "轮询超时：05 未在锁A 释放后完成（查 DTM 日志）"
    wait "$LOCK_A_PID" 2>/dev/null
    wait "$LOCK_B_PID" 2>/dev/null
    exit $FAILED
fi
entry_name=$(extract_field "$record" entryName)
print_step "开奖结果: ${entry_name}（05 已完成；07 正在锁B 上重试）"

# ---------- 6. 等捕获完成（锁B 释放后 07 重试成功） ----------
print_step "6. 等支付单 PAID（锁B 释放后 07 重试应成功，最长等 240s）"
# 注：wait_payment_paid 的 max 参数是轮询次数（每次 0.5s），480 次 ≈ 240s
wait_payment_paid "$rid" 480 || {
    print_error "支付单未在 240s 内到达 PAID，实际: $(get_payment_status "$rid")"
    wait "$LOCK_A_PID" 2>/dev/null
    wait "$LOCK_B_PID" 2>/dev/null
    exit $FAILED
}
print_step "支付单已 PAID"

# ---------- 7. 断言：捕获恰一次、扣款一次、无退款 ----------
print_step "7. 断言：捕获恰一次（腿/hold POSTED）+ 余额 9.00 + 无补偿流水"
leg_status=$(get_payment_leg_status "$rid" 1)
[ "$leg_status" == "2" ] || print_error "payment_legs CASH 腿应为 2(POSTED)，实际: ${leg_status}"
hold_status=$(get_payment_hold_status "$rid" 1)
[ "$hold_status" == "2" ] || print_error "wallet_holds 应为 2(POSTED)，实际: ${hold_status}"
pcount=$(count_payment_orders "$rid")
[ "$pcount" == "1" ] || print_error "payment_orders 应恰 1 条，实际: ${pcount}"

wait_balance "$token" 900 30 || print_error "余额应为 9.00（一次扣款、无退款），实际: $(get_balance "$token")"

leg_no=$(payment_no "$rid")_1
fc=$(count_wallet_ledgers "user_id=${user_id} AND log_no='h_${leg_no}' AND biz_type='freeze'")
[ "$fc" == "1" ] || print_error "freeze 流水应恰 1 条（捕获恰一次），实际: ${fc}"
uc=$(count_wallet_ledgers "user_id=${user_id} AND biz_type='unfreeze'")
[ "$uc" == "0" ] || print_error "不应有 unfreeze 流水（未走补偿），实际: ${uc}"
rec_count=$(count_records "$rid")
[ "$rec_count" == "1" ] || print_error "draw_records 应有 1 条，实际: ${rec_count}"

# 收尾：确认锁会话已退出
wait "$LOCK_A_PID" 2>/dev/null
wait "$LOCK_B_PID" 2>/dev/null

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景10 capture_fault 通过（07 Internal 重试后 PAID，捕获恰一次）${NC}"
else
    echo -e "${RED}× 场景10 capture_fault 失败${NC}"
fi
exit $FAILED
