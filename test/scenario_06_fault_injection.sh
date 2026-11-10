#!/bin/bash
# ============================================================
# 场景6：故障注入 —— SettleDraw 人为阻塞 → DTM 退避重试 → 恢复后完成（M6 验收）
#
# 注入方式：后台 MySQL 会话对 draw_boxes(id=1) 加行锁并持有 72 秒，
# SettleDraw 的 FOR UPDATE 读被阻塞，超过 innodb_lock_wait_timeout(50s)
# 后返回锁等待超时（裸错误 → 拦截器折叠为 codes.Internal → DTM 退避重试）。
# 锁释放后 DTM 的下一次重试成功，事务最终完成。
#
# 断言：
#   1. 故障窗口内购买提交成功（Submit 返回 pending，DTM 接管重试）
#   2. 锁释放后（重试窗口内）事务完成：轮询出抽赏记录
#   3. 余额恰好扣一次（10.00 → 9.00），无退款（有退款则余额回到 10.00）
#   4. draw_records 恰 1 条（屏障 + request_id 唯一键防重）
#
# 与场景3 的区别：场景3 是业务失败（Aborted → 补偿退款）；
# 本场景是系统故障（Internal → 重试直到成功，不退款）。
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景6：故障注入（SettleDraw 阻塞 72s → DTM 重试 → 最终完成）"

LOCK_SECONDS=72

# ---------- 1. 注册 + 充值 ----------
print_step "1. 注册用户并充值 10 元"
reg=$(register_user "fault_") || exit 1
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge "{\"amount\":\"10\"}" "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
print_step "充值后余额: $(extract_field "$resp" balance)"

# ---------- 2. 后台会话持有 box1 行锁 ----------
print_step "2. 后台 MySQL 会话锁定 draw_boxes(id=1) ${LOCK_SECONDS} 秒"
"$MYSQL_BIN" -h127.0.0.1 -uroot -p"${DB_PASS}" --default-character-set=utf8mb4 -N -B -e \
    "BEGIN; SELECT id FROM gacha_draw.draw_boxes WHERE id=1 FOR UPDATE; DO SLEEP(${LOCK_SECONDS}); COMMIT;" \
    >/dev/null 2>&1 &
LOCK_PID=$!
sleep 3   # 等后台会话连接并拿到锁
print_step "锁会话已就绪（PID ${LOCK_PID}）"

# ---------- 3. 故障窗口内购买 ----------
print_step "3. 购买 box1：扣款成功、SettleDraw 阻塞（预计 50s 锁等待超时 → Internal → DTM 重试）"
buy_resp=$(do_post /box/buy '{"boxId":1}' "$token")
[ "$(extract_code "$buy_resp")" == "0" ] || print_error "购买提交失败: ${buy_resp}"
rid=$(extract_field "$buy_resp" requestId)
[ -n "$rid" ] || print_error "requestId 不应为空"
print_step "SAGA 已提交: requestId=${rid}"

# ---------- 4. 轮询（覆盖锁窗口 + DTM 重试 + 最终成功） ----------
# 实测时间线：退避间隔 20→40→80→160s，放锁后要等下一次 cron（约提交后 170s）才完成
print_step "4. 轮询抽赏记录（放锁后 DTM 重试应成功，最长等 240s）"
record=$(poll_record "$token" "$rid" 240)
if [ -z "$record" ]; then
    print_error "轮询超时：DTM 未在锁释放后完成事务（查 DTM/网关日志）"
    wait "$LOCK_PID" 2>/dev/null
    exit $FAILED
fi
entry_name=$(extract_field "$record" entryName)
print_step "开奖结果: ${entry_name}"

# ---------- 5. 断言：扣一次、不退款 ----------
print_step "5. 校验余额 10.00 → 9.00（扣款一次且无退款）"
wait_balance "$token" 900 30 || print_error "余额应为 9.00（一次扣款、无退款），实际: $(get_balance "$token")"

print_step "6. 数据库断言：draw_records 恰 1 条"
rec_count=$(count_records "$rid")
[ "$rec_count" == "1" ] || print_error "draw_records 应有 1 条，实际: ${rec_count}"

# 收尾：确认锁会话已退出
wait "$LOCK_PID" 2>/dev/null

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景6 fault_injection 通过（DTM 重试后完成，扣款一次不退款）${NC}"
else
    echo -e "${RED}× 场景6 fault_injection 失败${NC}"
fi
exit $FAILED
