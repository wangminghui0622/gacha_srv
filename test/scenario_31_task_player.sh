#!/bin/bash
# ============================================================
# 场景31：M15c task 玩家任务体系（任务/签到/通行证/会员，task-rpc 7 RPC + 玩家网关 5 端点）
#
# 种子：v2 测试计划（hashgen 生成，head=2）：4 任务（1 累计消费 DAILY DRAW_SPEND target100 T-CASH /
#       2 每日签到 DAILY SIGN_IN target2 T-POINT / 3 抽卡次数 DAILY DRAW_COUNT target1 T-COIN /
#       4 首充礼 ONCE RECHARGE_MINOR target100 cycleLimit1 T-COIN）、
#       通行证·赛季一 xp/级 100、1 级封顶、付费线 PAYMENT（FREE L1=T-POINT×100 / PREMIUM L1=T-FIX-500×1）、
#       会员 2 档（500/T-VIP1、1000/T-VIP2）
# 流程：8 表清空 + v2 种子 → 匿名 GET /tasks → 7003 → 注册 A → 视图断言（4 任务/通行证/会员档位）
#       → 负例（claim 未达标 7001、taskId 999 → 404、非签到任务 sign-in → 404）
#       → taskcli apply pay:tool 100（进度+通行证 XP+会员消费三联动；eventId 重放幂等）→ task1 可领取
#         → claim + 同 requestId 重放同回 → state 回 IN_PROGRESS（cycleLimit=0 不终态，老项目同款）
#       → 签到（首签 streak1、同日重签 alreadySigned、SQL 改昨天 → streak2 → task2 可领取 → claim
#         → state 保持 CLAIMABLE（lastDate=今天 且 streak≥2，老项目同款））
#       → D9 购买挂钩（充值 5 元 + buy_box 1 号 100 分 → 进度 200 / XP 封顶 100 / 消费 200）
#       → task3（apply DRAW_COUNT → claim → state 回 1）→ task4（apply RECHARGE_MINOR → claim
#         → FINISHED(3)（cycleLimit=1）+ 新 key 再 claim → 7002）
#       → 通行证（FREE claim、同槽新 key 7005、PREMIUM 未解锁 7004、unlock-pass → PREMIUM claim、
#         claim 表 2 行）→ 会员（SQL spend 600 → 档1 claim、同档新 key 7007、档2 未达 7006、
#         SQL 1200 → 档2 claim、level 保持 0 老项目同款）→ 8 表落库断言 → 复位（v1 计划 + 8 表清空）
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景31：M15c task 玩家任务体系（task_player）"

# task_field <listJson> <taskId> <字段>：提取任务对象内字段（任务对象无嵌套，[^}]* 截断安全）
task_field() {
    echo "$1" | grep -o "\"taskId\":$2,[^}]*" | grep -o "\"$3\":[^,]*" | head -1 | cut -d: -f2 | tr -d '"'
}

# ---------- 1. SQL 复位：8 表清空 + v2 计划种子 ----------
print_step "1. SQL 复位（task 8 表清空 + gacha_ops 计划快照 v2 head=2）"
task_tables_reset
task_plan_seed_v2
[ "$(do_mysql "SELECT current_version FROM gacha_ops.task_plan_head WHERE singleton_id=1;")" == "2" ] || print_error "head.current_version 应 2"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_claim;")" == "0" ] || print_error "复位后 task_claim 应为 0"

# ---------- 2. 匿名 GET /tasks → 7003 ----------
print_step "2. 匿名 GET /tasks → 7003（无绑定用户，老项目 ListContext 同款拒绝）"
resp=$(do_get /tasks)
[ "$(extract_code "$resp")" == "7003" ] || print_error "匿名应 7003，实际: ${resp}"

# ---------- 3. 注册 A + 视图断言 ----------
print_step "3. 注册 A → GET /tasks 视图断言（4 任务 / 通行证 / 会员 2 档）"
reg_a=$(register_user "e2e_task_a_") || exit 1
uid_a=$(echo "$reg_a" | cut -d' ' -f1)
name_a=$(echo "$reg_a" | cut -d' ' -f2)
token_a=$(login_by_name "$name_a") || exit 1

list=$(do_get /tasks "$token_a")
[ "$(extract_code "$list")" == "0" ] || print_error "列表查询失败: ${list}"
[ "$(count_field "$list" '"taskId":')" == "4" ] || print_error "应含 4 任务，实际: ${list}"
[ "$(task_field "$list" 1 name)" == "累计消费" ] || print_error "task1 名称不符: $(task_field "$list" 1 name)"
[ "$(task_field "$list" 1 metric)" == "2" ] || print_error "task1 metric 应 2(DRAW_SPEND_MINOR)，实际: $(task_field "$list" 1 metric)"
[ "$(task_field "$list" 1 target)" == "100" ] || print_error "task1 target 应 100"
[ "$(task_field "$list" 1 state)" == "1" ] || print_error "task1 初始 state 应 1(IN_PROGRESS)，实际: $(task_field "$list" 1 state)"
[ "$(task_field "$list" 1 progress)" == "0" ] || print_error "task1 初始 progress 应 0"
[ "$(task_field "$list" 2 name)" == "每日签到" ] || print_error "task2 名称不符: $(task_field "$list" 2 name)"
[ "$(task_field "$list" 2 metric)" == "5" ] || print_error "task2 metric 应 5(SIGN_IN)，实际: $(task_field "$list" 2 metric)"
[ "$(task_field "$list" 2 target)" == "2" ] || print_error "task2 target 应 2"
[ "$(task_field "$list" 3 name)" == "抽卡次数" ] || print_error "task3 名称不符: $(task_field "$list" 3 name)"
[ "$(task_field "$list" 3 metric)" == "1" ] || print_error "task3 metric 应 1(DRAW_COUNT)，实际: $(task_field "$list" 3 metric)"
[ "$(task_field "$list" 4 name)" == "首充礼" ] || print_error "task4 名称不符: $(task_field "$list" 4 name)"
[ "$(task_field "$list" 4 metric)" == "4" ] || print_error "task4 metric 应 4(RECHARGE_MINOR)，实际: $(task_field "$list" 4 metric)"
[ "$(task_field "$list" 4 period)" == "1" ] || print_error "task4 period 应 1(ONCE)，实际: $(task_field "$list" 4 period)"
[ "$(task_field "$list" 4 target)" == "100" ] || print_error "task4 target 应 100"
[ "$(task_field "$list" 4 cycleLimit)" == "1" ] || print_error "task4 cycleLimit 应 1，实际: $(task_field "$list" 4 cycleLimit)"
echo "$list" | grep -q '"passId":1' || print_error "通行证视图应含 passId 1，实际: ${list}"
echo "$list" | grep -q '"xpPerLevel":100' || print_error "pass xpPerLevel 应 100"
echo "$list" | grep -q '"maxLevel":1' || print_error "pass maxLevel 应 1"
echo "$list" | grep -q '"xpPerSpendMinor":1' || print_error "pass xpPerSpendMinor 应 1"
echo "$list" | grep -q '"premiumLine":1' || print_error "pass premiumLine 应 1(PAYMENT)"
echo "$list" | grep -q '"premium":false' || print_error "初始 premium 应 false"
echo "$list" | grep -q '"xp":0' || print_error "初始 pass xp 应 0"
[ "$(count_field "$list" '"track":')" == "2" ] || print_error "通行证应含 2 槽位奖励（FREE+PREMIUM），实际: ${list}"
echo "$list" | grep -q '"rewardTemplateId":"T-POINT"' || print_error "FREE 槽应 T-POINT"
echo "$list" | grep -q '"rewardTemplateId":"T-FIX-500"' || print_error "PREMIUM 槽应 T-FIX-500"
[ "$(count_field "$list" '"spendMinor":')" == "3" ] || print_error "会员 spendMinor 字段应 3 处（0/500/1000），实际: ${list}"
echo "$list" | grep -q '"rewardTemplateId":"T-VIP1"' || print_error "会员档 1 应 T-VIP1"
echo "$list" | grep -q '"rewardTemplateId":"T-VIP2"' || print_error "会员档 2 应 T-VIP2"
print_step "视图断言通过：4 任务 IN_PROGRESS / 通行证 2 槽 / 会员 2 档"

# ---------- 4. 负例 ----------
print_step "4. 负例：claim 未达标 7001 / taskId 999 → 404 / 非签到任务 sign-in → 404"
resp=$(do_post /tasks/1/claim '{"version":2,"requestId":"e2e31-neg1"}' "$token_a")
[ "$(extract_code "$resp")" == "7001" ] || print_error "未达标 claim 应 7001，实际: ${resp}"
resp=$(do_post /tasks/999/claim '{"version":2,"requestId":"e2e31-neg2"}' "$token_a")
[ "$(extract_code "$resp")" == "404" ] || print_error "claim 不存在任务应 404，实际: ${resp}"
resp=$(do_post /tasks/1/sign-in '{"version":2}' "$token_a")
[ "$(extract_code "$resp")" == "404" ] || print_error "非 SIGN_IN 任务签到应 404，实际: ${resp}"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_claim;")" == "0" ] || print_error "负例不应落 task_claim"
print_step "负例通过：7001/404/404"

# ---------- 5. taskcli apply：进度+XP+会员三联动（幂等） ----------
print_step "5. taskcli apply pay:tool 100（DRAW_SPEND_MINOR）→ 进度/XP/会员消费三联动 + 重放幂等"
out=$(apply_task_progress "$uid_a" "pay:tool" 2 100)
[ "$out" == "OK" ] || print_error "apply 应 OK，实际: ${out}"
out=$(apply_task_progress "$uid_a" "pay:tool" 2 100)
[ "$out" == "OK" ] || print_error "同 eventId 重放应 OK（幂等），实际: ${out}"
[ "$(task_progress_field "$uid_a" 1 amount)" == "100" ] || print_error "task1 进度应 100，实际: $(task_progress_field "$uid_a" 1 amount)"
[ "$(task_pass_field "$uid_a" 1 xp)" == "100" ] || print_error "pass xp 应 100（封顶 100*1），实际: $(task_pass_field "$uid_a" 1 xp)"
[ "$(task_member_field "$uid_a" spend_minor)" == "100" ] || print_error "会员 spend 应 100，实际: $(task_member_field "$uid_a" spend_minor)"
[ "$(task_member_field "$uid_a" level)" == "0" ] || print_error "会员 level 应 0（未达 500 档）"

# ---------- 6. task1 可领取 → claim + 重放 → 状态回退 ----------
print_step "6. task1 达 100/100 → CLAIMABLE → claim T-CASH + 重放同回 → state 回 1（cycleLimit=0 不终态）"
list=$(do_get /tasks "$token_a")
[ "$(task_field "$list" 1 state)" == "2" ] || print_error "task1 应 CLAIMABLE(2)，实际: $(task_field "$list" 1 state)"
resp=$(do_post /tasks/1/claim '{"version":2,"requestId":"e2e31-c1"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "claim task1 应 0，实际: ${resp}"
[ "$(extract_field "$resp" rewardTemplateId)" == "T-CASH" ] || print_error "task1 奖励应 T-CASH，实际: ${resp}"
[ "$(extract_field "$resp" claimed)" == "1" ] || print_error "claimed 应 1，实际: ${resp}"
resp2=$(do_post /tasks/1/claim '{"version":2,"requestId":"e2e31-c1"}' "$token_a")
[ "$(extract_code "$resp2")" == "0" ] || print_error "claim 重放应 0，实际: ${resp2}"
[ "$(extract_field "$resp2" rewardTemplateId)" == "$(extract_field "$resp" rewardTemplateId)" ] || print_error "重放响应应与首领一致: ${resp2}"
list=$(do_get /tasks "$token_a")
[ "$(task_field "$list" 1 claimed)" == "1" ] || print_error "task1 claimed 应 1，实际: $(task_field "$list" 1 claimed)"
[ "$(task_field "$list" 1 state)" == "1" ] || print_error "claim 后 state 应回 1（100<100*(1+1)=200，下一档阈值），实际: $(task_field "$list" 1 state)"

# ---------- 7. 每日签到：首签 → 同日重签 → 昨天改签 → streak2 → claim task2 ----------
print_step "7. 每日签到：首签 streak1 → 同日 alreadySigned → SQL 改昨天 → streak2 → claim task2"
resp=$(do_post /tasks/2/sign-in '{"version":2}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "首签应 0，实际: ${resp}"
[ "$(extract_field "$resp" streak)" == "1" ] || print_error "首签 streak 应 1，实际: ${resp}"
[ "$(extract_field "$resp" alreadySigned)" == "false" ] || print_error "首签 alreadySigned 应 false，实际: ${resp}"
last_date=$(extract_field "$resp" calendarDate)
[ -n "$last_date" ] || print_error "签到应回 calendarDate: ${resp}"
[ "$(task_signin_field "$uid_a" 2 streak)" == "1" ] || print_error "task_sign_in streak 应 1"

resp=$(do_post /tasks/2/sign-in '{"version":2}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "同日重签应 0，实际: ${resp}"
[ "$(extract_field "$resp" alreadySigned)" == "true" ] || print_error "同日重签 alreadySigned 应 true，实际: ${resp}"
[ "$(extract_field "$resp" streak)" == "1" ] || print_error "同日重签 streak 应仍 1，实际: ${resp}"

# 改 last_date 为昨天（按签到返回的上海日历日推算，避免时区歧义）
yesterday=$(date -d "$last_date -1 day" +%F 2>/dev/null)
[ -n "$yesterday" ] || print_error "无法推算昨日日期: ${last_date}"
do_mysql "UPDATE gacha_task.task_sign_in SET last_date='${yesterday}' WHERE user_id=${uid_a} AND task_id=2;"
resp=$(do_post /tasks/2/sign-in '{"version":2}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "次日首签应 0，实际: ${resp}"
[ "$(extract_field "$resp" streak)" == "2" ] || print_error "连续签到 streak 应 2，实际: ${resp}"
[ "$(extract_field "$resp" alreadySigned)" == "false" ] || print_error "次日首签 alreadySigned 应 false，实际: ${resp}"
[ "$(task_signin_field "$uid_a" 2 streak)" == "2" ] || print_error "task_sign_in streak 应 2"
list=$(do_get /tasks "$token_a")
[ "$(task_field "$list" 2 progress)" == "2" ] || print_error "task2 progress 应 2(streak)，实际: $(task_field "$list" 2 progress)"
[ "$(task_field "$list" 2 state)" == "2" ] || print_error "task2 应 CLAIMABLE(2)，实际: $(task_field "$list" 2 state)"
resp=$(do_post /tasks/2/claim '{"version":2,"requestId":"e2e31-c2"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "claim task2 应 0，实际: ${resp}"
[ "$(extract_field "$resp" rewardTemplateId)" == "T-POINT" ] || print_error "task2 奖励应 T-POINT，实际: ${resp}"
resp2=$(do_post /tasks/2/claim '{"version":2,"requestId":"e2e31-c2"}' "$token_a")
[ "$(extract_code "$resp2")" == "0" ] || print_error "claim task2 重放应 0，实际: ${resp2}"
list=$(do_get /tasks "$token_a")
[ "$(task_field "$list" 2 claimed)" == "1" ] || print_error "task2 claimed 应 1，实际: $(task_field "$list" 2 claimed)"
[ "$(task_field "$list" 2 state)" == "2" ] || print_error "claim 后 state 应保持 CLAIMABLE(2)（lastDate=今天 且 streak≥2，老项目同款），实际: $(task_field "$list" 2 state)"
print_step "签到断言通过：streak 1→同日重签→2→claim→state 保持 CLAIMABLE"

# ---------- 8. D9 购买挂钩：充值 5 元 + buy_box 100 分 ----------
print_step "8. D9 购买挂钩：充值 5 元 + buy_box 1 号（100 分）→ 进度/XP/会员消费联动"
resp=$(do_post /user/recharge '{"amount":"5"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "充值失败: ${resp}"
rid=$(buy_box "$token_a" 1 "e2e31-buy-$(date +%s)") || exit 1
rec=$(poll_record "$token_a" "$rid") || print_error "抽赏记录未产生: ${rid}"
[ "$(fen_of "$(wait_balance "$token_a" "4.00")")" == "400" ] || print_error "购买后余额应 4.00"
[ "$(task_progress_field "$uid_a" 1 amount)" == "200" ] || print_error "购买后 task1 进度应 200，实际: $(task_progress_field "$uid_a" 1 amount)"
[ "$(task_pass_field "$uid_a" 1 xp)" == "100" ] || print_error "购买后 pass xp 应仍 100（封顶），实际: $(task_pass_field "$uid_a" 1 xp)"
[ "$(task_member_field "$uid_a" spend_minor)" == "200" ] || print_error "购买后会员 spend 应 200，实际: $(task_member_field "$uid_a" spend_minor)"
print_step "购买挂钩通过：task_progress 200 / pass xp 封顶 100 / member spend 200"

# ---------- 9. task3：DRAW_COUNT → claim ----------
print_step "9. task3：apply DRAW_COUNT 1 → CLAIMABLE → claim T-COIN + 重放 → state 回 1"
out=$(apply_task_progress "$uid_a" "draw:tool" 1 1)
[ "$out" == "OK" ] || print_error "apply draw 应 OK，实际: ${out}"
list=$(do_get /tasks "$token_a")
[ "$(task_field "$list" 3 progress)" == "1" ] || print_error "task3 progress 应 1，实际: $(task_field "$list" 3 progress)"
[ "$(task_field "$list" 3 state)" == "2" ] || print_error "task3 应 CLAIMABLE(2)，实际: $(task_field "$list" 3 state)"
resp=$(do_post /tasks/3/claim '{"version":2,"requestId":"e2e31-c3"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "claim task3 应 0，实际: ${resp}"
[ "$(extract_field "$resp" rewardTemplateId)" == "T-COIN" ] || print_error "task3 奖励应 T-COIN，实际: ${resp}"
resp2=$(do_post /tasks/3/claim '{"version":2,"requestId":"e2e31-c3"}' "$token_a")
[ "$(extract_code "$resp2")" == "0" ] || print_error "claim task3 重放应 0，实际: ${resp2}"
[ "$(task_field "$(do_get /tasks "$token_a")" 3 state)" == "1" ] || print_error "claim 后 task3 state 应回 1（1<1*(1+1)=2）"

# ---------- 10. task4：ONCE + cycleLimit=1 → FINISHED + 再领 7002 ----------
print_step "10. task4：apply RECHARGE_MINOR 100 → claim → FINISHED(3)（cycleLimit=1）→ 新 key 再领 7002"
out=$(apply_task_progress "$uid_a" "recharge:tool" 4 100)
[ "$out" == "OK" ] || print_error "apply recharge 应 OK，实际: ${out}"
list=$(do_get /tasks "$token_a")
[ "$(task_field "$list" 4 progress)" == "100" ] || print_error "task4 progress 应 100，实际: $(task_field "$list" 4 progress)"
[ "$(task_field "$list" 4 state)" == "2" ] || print_error "task4 应 CLAIMABLE(2)，实际: $(task_field "$list" 4 state)"
resp=$(do_post /tasks/4/claim '{"version":2,"requestId":"e2e31-c4"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "claim task4 应 0，实际: ${resp}"
[ "$(extract_field "$resp" rewardTemplateId)" == "T-COIN" ] || print_error "task4 奖励应 T-COIN，实际: ${resp}"
[ "$(extract_field "$resp" claimed)" == "1" ] || print_error "task4 claimed 应 1，实际: ${resp}"
resp2=$(do_post /tasks/4/claim '{"version":2,"requestId":"e2e31-c4"}' "$token_a")
[ "$(extract_code "$resp2")" == "0" ] || print_error "claim task4 重放应 0，实际: ${resp2}"
list=$(do_get /tasks "$token_a")
[ "$(task_field "$list" 4 state)" == "3" ] || print_error "task4 应 FINISHED(3)（claimed 1≥cycleLimit 1），实际: $(task_field "$list" 4 state)"
resp=$(do_post /tasks/4/claim '{"version":2,"requestId":"e2e31-c4b"}' "$token_a")
[ "$(extract_code "$resp")" == "7002" ] || print_error "超 cycleLimit 再领应 7002，实际: ${resp}"
print_step "task4 断言通过：ONCE/cycleLimit=1 → FINISHED(3) / 7002"

# ---------- 11. 通行证：FREE → 槽位重放 → PREMIUM 未解锁 → unlock → PREMIUM ----------
print_step "11. 通行证：FREE claim（xp100 达 1 级）→ 同 requestId 重放 → 同槽新 key 7005 → PREMIUM 7004 → unlock → PREMIUM claim"
resp=$(do_post /tasks/passes/1/claim '{"level":1,"track":1,"requestId":"e2e31-pf1"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "FREE claim 应 0，实际: ${resp}"
[ "$(extract_field "$resp" rewardTemplateId)" == "T-POINT" ] || print_error "FREE 奖励应 T-POINT，实际: ${resp}"
[ "$(extract_field "$resp" rewardCount)" == "100" ] || print_error "FREE 数量应 100，实际: ${resp}"
[ "$(extract_field "$resp" track)" == "1" ] || print_error "FREE track 应 1，实际: ${resp}"
resp2=$(do_post /tasks/passes/1/claim '{"level":1,"track":1,"requestId":"e2e31-pf1"}' "$token_a")
[ "$(extract_code "$resp2")" == "0" ] || print_error "FREE 同 requestId 重放应 0，实际: ${resp2}"
[ "$(extract_field "$resp2" rewardTemplateId)" == "T-POINT" ] || print_error "重放奖励应一致，实际: ${resp2}"
resp=$(do_post /tasks/passes/1/claim '{"level":1,"track":1,"requestId":"e2e31-pf1b"}' "$token_a")
[ "$(extract_code "$resp")" == "7005" ] || print_error "同槽新 key 应 7005（槽位已领），实际: ${resp}"
resp=$(do_post /tasks/passes/1/claim '{"level":1,"track":2,"requestId":"e2e31-pp1"}' "$token_a")
[ "$(extract_code "$resp")" == "7004" ] || print_error "PREMIUM 未解锁应 7004，实际: ${resp}"
[ "$(task_pass_field "$uid_a" 1 premium)" == "0" ] || print_error "unlock 前 premium 应 0"
out=$(unlock_pass "$uid_a" 1)
[ "$out" == "OK" ] || print_error "unlock-pass 应 OK，实际: ${out}"
[ "$(task_pass_field "$uid_a" 1 premium)" == "1" ] || print_error "unlock 后 premium 应 1"
resp=$(do_post /tasks/passes/1/claim '{"level":1,"track":2,"requestId":"e2e31-pp1"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "PREMIUM claim 应 0，实际: ${resp}"
[ "$(extract_field "$resp" rewardTemplateId)" == "T-FIX-500" ] || print_error "PREMIUM 奖励应 T-FIX-500，实际: ${resp}"
[ "$(extract_field "$resp" track)" == "2" ] || print_error "PREMIUM track 应 2，实际: ${resp}"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_pass_claim WHERE user_id=${uid_a};")" == "2" ] || print_error "task_pass_claim 应 2 行（FREE+PREMIUM）"
list=$(do_get /tasks "$token_a")
echo "$list" | grep -q '"premium":true' || print_error "pass 视图 premium 应 true，实际: ${list}"
echo "$list" | grep -q '"level":1' || print_error "pass 视图 level 应 1，实际: ${list}"
print_step "通行证断言通过：2 槽各恰一次 / 7005 / 7004 / unlock 解锁 premium"

# ---------- 12. 会员：SQL 600 → 档1 → 重放/同档新 key → 档2 未达 → SQL 1200 → 档2 ----------
print_step "12. 会员：spend 600 → 档1 claim → 同档新 key 7007 → 档2 未达 7006 → spend 1200 → 档2 claim"
do_mysql "UPDATE gacha_task.task_member SET spend_minor=600 WHERE user_id=${uid_a};"
resp=$(do_post /membership/claim '{"level":1,"requestId":"e2e31-m1"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "档1 claim 应 0，实际: ${resp}"
[ "$(extract_field "$resp" rewardTemplateId)" == "T-VIP1" ] || print_error "档1 奖励应 T-VIP1，实际: ${resp}"
[ "$(extract_field "$resp" claimed)" == "1" ] || print_error "档1 claimed 应 1，实际: ${resp}"
resp2=$(do_post /membership/claim '{"level":1,"requestId":"e2e31-m1"}' "$token_a")
[ "$(extract_code "$resp2")" == "0" ] || print_error "档1 同 requestId 重放应 0，实际: ${resp2}"
resp=$(do_post /membership/claim '{"level":1,"requestId":"e2e31-m1b"}' "$token_a")
[ "$(extract_code "$resp")" == "7007" ] || print_error "同档新 key 应 7007（档位已领），实际: ${resp}"
resp=$(do_post /membership/claim '{"level":2,"requestId":"e2e31-m2"}' "$token_a")
[ "$(extract_code "$resp")" == "7006" ] || print_error "档2 未达（600<1000）应 7006，实际: ${resp}"
do_mysql "UPDATE gacha_task.task_member SET spend_minor=1200 WHERE user_id=${uid_a};"
resp=$(do_post /membership/claim '{"level":2,"requestId":"e2e31-m2"}' "$token_a")
[ "$(extract_code "$resp")" == "0" ] || print_error "档2 claim 应 0，实际: ${resp}"
[ "$(extract_field "$resp" rewardTemplateId)" == "T-VIP2" ] || print_error "档2 奖励应 T-VIP2，实际: ${resp}"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_member_claim WHERE user_id=${uid_a};")" == "2" ] || print_error "task_member_claim 应 2 行"
[ "$(task_member_field "$uid_a" level)" == "0" ] || print_error "会员 level 应保持 0（老项目 claim 不更新 level，忠实）"
list=$(do_get /tasks "$token_a")
echo "$list" | grep -q '"spendMinor":1200' || print_error "会员视图 spendMinor 应 1200，实际: ${list}"
print_step "会员断言通过：2 档各恰一次 / 7007 / 7006 / level 保持 0"

# ---------- 13. 8 表落库断言 ----------
print_step "13. 8 表落库断言"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_progress WHERE user_id=${uid_a};")" == "4" ] || print_error "task_progress 应 4 行（task1/3/4 apply + task2 claim 经 lockProgress 占位）"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_event WHERE event_id IN ('pay:tool','draw:tool','recharge:tool');")" == "3" ] || print_error "task_event 应含 3 个工具事件"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_event WHERE event_id LIKE 'pay:%' AND event_id NOT IN ('pay:tool');")" == "1" ] || print_error "task_event 应含 1 个购买事件（pay:<gid>）"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_claim WHERE user_id=${uid_a};")" == "4" ] || print_error "task_claim 应 4 行"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_sign_in WHERE user_id=${uid_a};")" == "1" ] || print_error "task_sign_in 应 1 行（task2）"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_pass_progress WHERE user_id=${uid_a};")" == "1" ] || print_error "task_pass_progress 应 1 行"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_pass_claim WHERE user_id=${uid_a};")" == "2" ] || print_error "task_pass_claim 应 2 行"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_member WHERE user_id=${uid_a};")" == "1" ] || print_error "task_member 应 1 行"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_member_claim WHERE user_id=${uid_a};")" == "2" ] || print_error "task_member_claim 应 2 行"
print_step "8 表落库断言通过"

# ---------- 14. 复位：v1 计划 + 8 表清空 ----------
print_step "14. 复位（gacha_ops 计划回 bootstrap v1 + task 8 表清空）"
task_plan_reset
task_tables_reset
[ "$(do_mysql "SELECT current_version FROM gacha_ops.task_plan_head WHERE singleton_id=1;")" == "1" ] || print_error "复位后 head.current_version 应 1"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_task.task_member;")" == "0" ] || print_error "复位后 task_member 应为 0"

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}√ 场景31 M15c task 玩家任务体系 通过${NC}"
    exit 0
else
    echo -e "${RED}× 场景31 失败${NC}"
    exit 1
fi
