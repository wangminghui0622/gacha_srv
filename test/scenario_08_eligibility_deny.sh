#!/bin/bash
# ============================================================
# 场景8：资格策略拒绝（eligibility_deny，M9 基础域 分支01；M14d 起 CEL 引擎）
#
# 流程：注册 → 充值10元 → 插入 DRAW 场景拒绝政策（新三表 head+version+rule，
#       规则 CEL 表达式 facts.user_id != <uid>，published_at=NOW() 晚于种子）
#       → 购买 boxId=2(10.00元) → 轮询决策日志=拒绝
# 断言：
#   1. 决策日志 result=0、policy_no=P-DRAW-DENY、reason=E2E_USER_DENY（failure_code 透出）
#   2. 余额不变（10.00）、无支付单、无抽赏记录（01 分支 Aborted，04 不执行）
#   3. 恢复现场：删除拒绝政策（rules/versions/heads 三表）
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景8：资格策略拒绝（eligibility_deny）"

# ---------- 1. 注册 + 充值 ----------
print_step "1. 注册用户并充值 10 元（足以购买 boxId=2）"
reg=$(register_user "e2e_policy_") || exit 1
user_id=$(echo "$reg" | cut -d' ' -f1)
username=$(echo "$reg" | cut -d' ' -f2)
token=$(login_by_name "$username") || exit 1
resp=$(do_post /user/recharge '{"amount":"10"}' "$token")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"

# ---------- 2. 植入拒绝策略（DRAW 场景，M14d 新三表：head + version + CEL rule） ----------
print_step "2. 植入拒绝策略：P-DRAW-DENY v1 PUBLISHED，规则 facts.user_id != ${user_id}"
# head（policy_code uk 幂等）
do_mysql "INSERT INTO gacha_base.eligibility_policies (policy_code, name, current_version, latest_version)
          VALUES ('P-DRAW-DENY', 'e2e拒绝策略', 1, 1)
          ON DUPLICATE KEY UPDATE name=VALUES(name), current_version=1, latest_version=1;"
# version：scene=3 DRAW、status=2 PUBLISHED、published_at=NOW() 晚于种子 → 场景解析命中。
# config_hash 为占位 32 字节（console 发布才做 CAS；场景只经评估引擎，不比对哈希）
do_mysql "INSERT INTO gacha_base.eligibility_policy_versions
              (policy_id, version, scene, status, combination, required_facts, config_hash, published_at)
          SELECT id, 1, 3, 2, 1, JSON_ARRAY(), UNHEX(SHA2('scenario8-P-DRAW-DENY-v1', 256)), NOW()
          FROM gacha_base.eligibility_policies WHERE policy_code='P-DRAW-DENY'
          ON DUPLICATE KEY UPDATE status=2, published_at=NOW();"
# rule（先删后插保证可复跑；policy_id+policy_version+rule_id uk）
do_mysql "DELETE FROM gacha_base.eligibility_policy_rules WHERE policy_id =
          (SELECT id FROM gacha_base.eligibility_policies WHERE policy_code='P-DRAW-DENY');"
# 资格语义=规则通过即放行（ALL=全部通过）；拒绝用户 <uid> ⇒ 规则 facts.user_id != <uid>
# （目标用户不满足即拒，failure_code 进 reason）
do_mysql "INSERT INTO gacha_base.eligibility_policy_rules
              (policy_id, policy_version, rule_id, expression, failure_code, sort_order)
          SELECT id, 1, 'r1', 'facts.user_id != ${user_id}', 'E2E_USER_DENY', 0
          FROM gacha_base.eligibility_policies WHERE policy_code='P-DRAW-DENY';"
print_step "拒绝策略已就位（DRAW 场景，CEL 规则拒绝用户 ${user_id}）"

# ---------- 3. 购买（SAGA 提交成功，01 分支异步拒绝） ----------
print_step "3. 购买 boxId=2（10.00 元）"
buy_resp=$(do_post /box/buy '{"boxId":2}' "$token")
[ "$(extract_code "$buy_resp")" == "0" ] || print_error "购买提交失败: ${buy_resp}"
rid=$(extract_field "$buy_resp" requestId)
[ -n "$rid" ] || print_error "requestId 不应为空"
print_step "requestId: ${rid}"

# ---------- 4. 决策日志断言（拒绝先落库） ----------
print_step "4. 轮询决策日志 → result=0（拒绝）"
wait_decision_result "$rid" 0 20 || { print_error "决策日志未在 20s 内出现拒绝结果"; exit $FAILED; }
dpolicy=$(get_decision_policy "$rid")
[ "$dpolicy" == "P-DRAW-DENY" ] || print_error "决策日志 policy_no 应为 P-DRAW-DENY，实际: ${dpolicy}"
dreason=$(do_mysql "SELECT reason FROM gacha_base.eligibility_decision_logs WHERE decision_no='d_${rid}' LIMIT 1")
[ "$dreason" == "E2E_USER_DENY" ] || print_error "决策日志 reason 应为 E2E_USER_DENY（failure_code 透出），实际: ${dreason}"
print_step "决策日志拒绝已落库：policy_no=${dpolicy}，reason=${dreason}"

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
print_step "6. 恢复现场：删除拒绝政策（rules → versions → heads）"
do_mysql "DELETE FROM gacha_base.eligibility_policy_rules WHERE policy_id =
          (SELECT id FROM gacha_base.eligibility_policies WHERE policy_code='P-DRAW-DENY');"
do_mysql "DELETE FROM gacha_base.eligibility_policy_versions WHERE policy_id =
          (SELECT id FROM gacha_base.eligibility_policies WHERE policy_code='P-DRAW-DENY');"
do_mysql "DELETE FROM gacha_base.eligibility_policies WHERE policy_code='P-DRAW-DENY';"
print_step "现场已恢复"

if [ "$FAILED" == "0" ]; then
    echo -e "${GREEN}√ 场景8 eligibility_deny 通过${NC}"
else
    echo -e "${RED}× 场景8 eligibility_deny 失败${NC}"
fi
exit $FAILED
