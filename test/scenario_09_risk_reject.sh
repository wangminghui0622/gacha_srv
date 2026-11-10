#!/bin/bash
# ============================================================
# 场景9：风控拒绝（risk_reject，M9 基础域 分支02）
#
# 流程：注册 → 充值10元 → 插入风控规则（amount_fen gte 1000，即 ≥10 元拒绝）
#       → 购买 boxId=2(10.00元) → 轮询风控案例=OPEN
# 断言：
#   1. 风控案例 case_no="rk_"+requestId、状态 OPEN、命中规则单号正确
#   2. 余额不变（10.00）、无支付单、无抽赏记录（02 分支 Aborted，04 不执行）
#   3. 恢复现场：删除风控规则（案例保留为审计数据，case_no 唯一不影响复跑）
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景9：风控拒绝（risk_reject）"

# ---------- 1. 注册 + 充值 ----------
print_step "1. 注册用户并充值 10 元（足以购买 boxId=2）"
reg=$(register_user "e2e_risk_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge '{"amount":"10"}' "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"

# ---------- 2. 植入风控规则（金额 ≥10 元拒绝） ----------
print_step "2. 植入风控规则：DRAW 场景 amount_fen gte 1000"
# uk_rule_no 撞键时全字段覆盖，保证残留规则（历史期望值 5000）被重置
do_mysql "INSERT INTO gacha_base.risk_rules (rule_no, scene, fact_key, op, expected_value, severity, enabled)
          VALUES ('R-DRAW-AMT-99', 'DRAW', 'amount_fen', 'gte', '1000', 'HIGH', 1)
          ON DUPLICATE KEY UPDATE scene=VALUES(scene), fact_key=VALUES(fact_key),
                                  op=VALUES(op), expected_value=VALUES(expected_value),
                                  severity=VALUES(severity), enabled=1;"
print_step "风控规则已就位"

# ---------- 3. 购买（SAGA 提交成功，02 分支异步拒绝） ----------
print_step "3. 购买 boxId=2（10.00 元）"
buy_resp=$(do_post /box/buy '{"boxId":2}' "$token")
[ "$(extract_code "$buy_resp")" == "0" ] || print_error "购买提交失败: ${buy_resp}"
rid=$(extract_field "$buy_resp" requestId)
[ -n "$rid" ] || print_error "requestId 不应为空"
print_step "requestId: ${rid}"

# ---------- 4. 风控案例断言（命中先落库） ----------
print_step "4. 轮询风控案例 → OPEN"
wait_risk_case_status "$rid" OPEN 20 || { print_error "风控案例未在 20s 内出现"; exit $FAILED; }
rule_no=$(get_risk_case_rule "$rid")
[ "$rule_no" == "R-DRAW-AMT-99" ] || print_error "案例命中规则应为 R-DRAW-AMT-99，实际: ${rule_no}"
print_step "风控案例已落库：status=OPEN，rule_no=${rule_no}"

# ---------- 5. 无副作用断言（等 DTM 补偿收尾后查） ----------
print_step "5. 无副作用断言：余额不变 / 无支付单 / 无记录"
sleep 2
balance=$(get_balance "$token") || exit $FAILED
[ "$(fen_of "$balance")" == "1000" ] || print_error "拒绝后余额应仍为 10.00，实际: ${balance}"
pcount=$(count_payment_orders "$rid")
[ "$pcount" == "0" ] || print_error "不应有支付单，实际: ${pcount}"
rcount=$(count_records "$rid")
[ "$rcount" == "0" ] || print_error "不应有抽赏记录，实际: ${rcount}"
held=$(get_wallet_held "$user_id" 1)
[ "$held" == "0" ] || print_error "拒绝后 CASH held 应为 0，实际: ${held}"
print_step "无副作用断言通过：余额 10.00、支付单 0、记录 0、held 0"

# ---------- 6. 恢复现场 ----------
print_step "6. 恢复现场：删除风控规则（案例保留为审计数据）"
do_mysql "DELETE FROM gacha_base.risk_rules WHERE rule_no='R-DRAW-AMT-99';"
print_step "现场已恢复"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景9 risk_reject 通过${NC}"
else
    echo -e "${RED}× 场景9 risk_reject 失败${NC}"
fi
exit $FAILED
