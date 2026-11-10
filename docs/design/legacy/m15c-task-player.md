# M15c 设计：task 玩家任务体系（任务/签到/通行证/会员 → game-rpc 9315 / gacha_game）

> 老项目参考：F:\workspace\src\server_v2 app/service/task（biz 1215 行 + data 状态机/事件管线 + 玩家网关 5 HTTP 端点）。
> 新仓落地：game-rpc 9315 / gacha_game；玩家执行面 8 表；计划快照跨库只读 gacha_ops（ops-rpc M14e 已克隆计划控制台）。
> 控制台任务计划（草稿→提请→审批→发布）已在 ops-rpc M14e 交付，本里程碑只做玩家面。

## 核心语义（照抄，常量不可改）

- **周期**：ONCE=1 / DAILY=2 / WEEKLY=3（enum 值照抄老项目）；时区固定 Asia/Shanghai；period_key = "once" / "2006-01-02" / ISOWeek "%04d-W%02d"。
- **任务指标**：DRAW_COUNT=1 / DRAW_SPEND_MINOR=2 / LOGIN=3 / RECHARGE_MINOR=4 / SIGN_IN=5。**SIGN_IN 不是外部事件**（Apply 拒绝）；老项目唯一实际生产者是支付事件（DRAW_SPEND_MINOR），LOGIN/DRAW_COUNT/RECHARGE 预留无生产者——照抄。
- **List**：userID=0 拒绝；page_size≤0 或 >50 → 默认 20；page_token=上一页末条 task_id 十进制字符串。
- **View 状态机**：CycleLimit>0 且 claimed≥limit → FINISHED；SIGN_IN 任务：今天已签且 streak≥Target → CLAIMABLE；否则 amount≥Target×(claimed+1) → CLAIMABLE；其余 IN_PROGRESS。进度展示：SIGN_IN 显示 streak，其他显示 amount。
- **签到**：today=上海日；lastDate==today → 已签返回 streak；==昨天且 streak>0 → +1；否则 1。乐观锁 `WHERE last_date=? AND streak=?`。签到不发奖，奖励走 ClaimTask（metric=SIGN_IN 领取条件=今天已签+streak≥Target+CycleLimit 未满）。
- **领取**：request_id 幂等原样回；门槛 amount≥Target×(claimed+1)（同周期可循环领多次）；claimed++；task_id+version 必须精确匹配当前发布快照。
- **通行证**：MaxLevel 1..200；XPPerLevel>0、XPPerSpendMinor>0；(level,track) 不重复；PremiumLine 三约束（UNSPECIFIED 无 premium 奖励；PAYMENT 有且 MemberLevel=0；MEMBER 有且 MemberLevel>0）。**等级 = xp / XPPerLevel 整除封顶 MaxLevel**（线性曲线）；XP 累加 `LEAST(xp+delta, XPPerLevel*MaxLevel)`。领取：passLevel(xp)≥level 否则 NotReady；槽位重复 → Finished；PREMIUM track：PAYMENT 线需 premium==true、MEMBER 线需会员等级≥PremiumMemberLevel。**UnlockPass 仅 PAYMENT 线合法，老项目玩家面无 HTTP 端点（照抄：内部 RPC 无路由）**。
- **会员**：档位 1..50、Level>0、SpendMinor 严格递增、Level 不重复；等级=满足 spend≥SpendMinor 的最高档；领取门槛 spend≥tier.SpendMinor、已领 → Finished；spend 只增不减、溢出饱和 1<<62。
- **进度应用**：`INSERT IGNORE task_event`（事件幂等）；一条事件对所有匹配 metric 的任务 `ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`；同时所有 Pass 加 XP（封顶）、member 加 spend。付费过滤照抄：business_type∈{gacha_draw, memory_match}、user>0、order>0、payable>0、currency=CNY；event_id="pay:"+订单号。
- **计划校验**（Board）：task ID>0、Version>0、Target>0、Metric/Period 非 UNSPECIFIED、奖励合法、ID 不重复、按 ID 升序。
- **奖励**：legacy 字符串（reward_template_id+count）→ 领取只回字段**不发实物**；结构化奖励（coupon/inventory）在**发布侧被 7019 拒绝**（M14e 已克隆 HasStructuredRewards 守卫）——老项目线上同样只有 legacy，照抄。

## 决策 D1-D9

- **D1 服务归属与表**：game-rpc 9315 / gacha_game；玩家执行面 8 表（task_progress、task_event、task_claim、task_sign_in、task_pass_progress、task_pass_claim、task_member、task_member_claim）；**无 reward_grant/outbox 表**（D3 legacy-only 不需要发奖管线）。
- **D2 计划快照来源（偏离 1：跨库直读替代同库）**：game-rpc 开 gacha_ops 只读连接（M14e reporting 跨库只读先例），每次玩家操作读 `task_plan_head.current_version` + `task_plan_version.config_blob`（status=2）→ decode `ops.TaskPlan`（复用 ops-rpc 生成的 payload 消息类型，字段见 M14e）→ 校验 config_hash → 重建 Board 内存快照。控制台发布新计划玩家立即生效（老项目同款）。计划 payload 类型复用 ops 包（跨服务消息依赖，只读不建 RPC）。
- **D3 无发奖管线（关键简化，照抄老项目线上现状）**：结构化奖励发布被 7019 拦截 → 玩家面只有 legacy 字符串奖励 → 领取事务只落 claim 行 + 返回字段，**无 Kafka/outbox/异步发奖**。若未来开放结构化奖励，届时在 game-rpc 加同步发奖（M11/M12 grant 先例）。
- **D4 进度事件去 Kafka（偏离 2：同步挂钩）**：game-rpc 暴露内部 RPC **ApplyTaskProgress**（无 HTTP 路由）；网关在**抽盒购买 SAGA 成功返回前**（business_type=gacha_draw）与**记忆翻牌购买成功返回前**（business_type=memory_match，M15b 交付时同钩）调用：{userId, eventId="pay:"+订单号, metric=DRAW_SPEND_MINOR, amount=实付 minor}。INSERT IGNORE 事件幂等 → SAGA 重放不重复计。老项目为 Kafka 事件驱动近实时，业务终点一致（付费即入账）。LOGIN/RECHARGE/DRAW_COUNT 维持预留不接（老项目现状）。
- **D5 5 端点全量**（接口照抄，路径按本仓风格）：

| 方法/路径 | 组 | 说明 |
|---|---|---|
| GET /tasks?pageSize=&pageToken= | 公开（无会话放行，有会话绑定） | 任务列表（period 过滤可选） |
| POST /tasks/:taskId/sign-in | JWT | 签到 |
| POST /tasks/:taskId/claim | JWT | 领任务（body{version, requestId}） |
| POST /tasks/passes/:passId/claim | JWT | 领通行证（body{level, track, requestId}） |
| POST /membership/claim | JWT | 领会员（body{level, requestId}） |

  game-rpc 7 RPC：ListTasks / SignInTask / ClaimTask / ClaimPassReward / ClaimMemberReward / ApplyTaskProgress / UnlockPass（后两个内部 RPC 无 HTTP 路由）。
- **D6 errorx**（game 域，05-errors.md 登记）：`3046 任务未达标（NotReady）` `3047 任务已领完（Finished）` `3048 任务参数无效` `3049 通行证领取条件未满足（等级未达/未解锁 premium/会员等级不足）` `3050 通行证槽位已领` `3051 会员消费未达档位` `3052 会员档已领` `3053 任务状态冲突（乐观锁失败）` `3054 进度事件非法（SIGN_IN/metric 非法/amount≤0）`；任务/通行证/会员档不存在 → 404；入参 → 400。
- **D7 场景 31**（test/scenario_31_task_player.sh + run_all/menu 注册，27→28 场景）：复位（8 表清 + v2 计划种子 head=2）→ 匿名 List 3048 → 注册 A → List 断言（4 任务 IN_PROGRESS/通行证 2 槽/会员 2 档）→ 负例（claim 未达标 3046、taskId 999 → 404、非签到任务 sign-in → 404）→ taskcli apply pay 事件 100 分（进度+XP+会员消费三联动，eventId 重放幂等）→ task1 CLAIMABLE → claim（幂等重放同回）→ **state 回 1**（cycleLimit=0 不终态，老项目 view 同款）→ 签到（首签 streak1 / 同日 alreadySigned / SQL 改昨天 → streak2）→ claim task2 → **state 保持 CLAIMABLE**（今天已签+streak≥2，老项目同款）→ 抽盒购买后断言 DRAW_SPEND 自动入账（网关挂钩验证：进度 200/XP 封顶 100/会员消费 200）→ task3（DRAW_COUNT claim → state 回 1）→ task4（ONCE RECHARGE_MINOR cycleLimit=1：claim → **FINISHED(3)** → 新 key 再领 3047）→ 通行证（FREE claim、同槽新 key 3050、PREMIUM 未解锁 3049、UnlockPass 工具直连、PREMIUM claim、claim 表 2 行）→ 会员（SQL spend 600 → 档1 claim、同档新 key 3052、档2 未达 3051、SQL 1200 → 档2 claim、**level 保持 0**——老项目 claim 不更新 level 忠实）→ 8 表落库断言 → 复位（v1 计划 head=1 + 8 表清）。
  v2 测试计划（4 任务：1 累计消费 DAILY DRAW_SPEND target100 T-CASH / 2 每日签到 DAILY SIGN_IN target2 T-POINT / 3 抽卡次数 DAILY DRAW_COUNT target1 T-COIN / 4 首充礼 ONCE RECHARGE_MINOR target100 cycleLimit1 T-COIN；pass 1 赛季：xp_per_level=100、max_level=1、xp_per_spend=1、PAYMENT 线，FREE L1=T-POINT×100 / PREMIUM L1=T-FIX-500×1；member 2 档 500/T-VIP1、1000/T-VIP2）：config_hash `f1c77a7681b03254d72f358497e1de87ee6ab60799b1950b82b9f8d1de0288fe`，blob UNHEX 见 test/common.sh `task_plan_seed_v2`（`.smoke_risk/hashgen` 生成，与 game-rpc 快照校验 ConfigHashOf 同源）。内部 RPC 经 **test/tools/taskcli**（grpc 直连 9315，apply/unlock-pass）调用。
- **D8 前端**：pages/Tasks/index.vue（任务卡列表：进度/状态/领取按钮 + 签到卡 streak + 通行证卡等级条/双轨奖励槽 + 会员卡等级权益）+ services/tasks.ts（5 API）+ 路由 + MainLayout 菜单 + i18n 双语。
- **D9 购买挂钩**：抽盒（gateway drawBoxBuy 成功路径）+ 记忆翻牌（M15b memoryMatchBuy 成功路径）调用 ApplyTaskProgress；失败仅记日志不阻断购买（老项目事件消费失败同样重试不阻断）。

## DDL — 23_gacha_game_m15c.sql（幂等，init_all.sql 聚合）

8 表形状（uk/PK 照抄老项目 schema.sql，DATETIME(3) → DATETIME 微调为本仓惯例）：
- `task_progress` PK(user_id,task_id,period_key) + amount + claimed_count
- `task_event` PK(event_id)——事件幂等去重
- `task_claim` PK(request_id)——领取幂等
- `task_sign_in` PK(user_id,task_id) + last_date CHAR(10) + streak
- `task_pass_progress` PK(user_id,pass_id) + xp + premium
- `task_pass_claim` PK(request_id) + UK(user_id,pass_id,level,track)
- `task_member` PK(user_id) + spend_minor + level
- `task_member_claim` PK(request_id) + UK(user_id,level)
全部 nocache 模型（行锁+幂等键写入）；计划面表在 gacha_ops（M14e 已有，只读）。

## proto 扩展（`bash script/goctl.sh game`，禁跑 gateway）

game.proto 新增枚举 TaskPeriod/TaskMetric/TaskProgressState/PassTrack + 消息 + 7 RPC（见 D5）。计划 payload 复用 ops.TaskPlan（D2）。

## 测试

- 单测：core/task 五个测试文件（task/pass/member/plan/reward 全用例——纯内存 Board 直译）。
- 场景 31（D7，含 taskcli 工具 + v2 计划种子/复位 helper）+ run_all/menu 注册（28 场景）。
- 边界门：run_all 28 场景 ×2 全绿、go build ./...、go test 核心包、npm run build ×2。

## 偏离记录汇总（m15c）

1. **D2**：计划快照跨库只读（老项目同服务同库；新仓计划控制台在 ops-rpc/gacha_ops，玩家面 game-rpc 只读，M14e reporting 先例）。
2. **D4**：Kafka 支付事件 → 网关购买成功后同步 ApplyTaskProgress（业务终点一致；无事件总线）。
3. **D6（会员 fact）**：会员等级变化不写 eligibility fact——本仓政策引擎无事实存储（老项目 fact 供 eligibility 政策引用 membership.level）；若未来政策需要会员条件，届时加事实链路。

## 实施顺序

1. 23_gacha_game_m15c.sql + 模型脚本更新+跑 → serviceContext 接入（含 gacha_ops 只读连接）。
2. errorx 3046-3054 + 05-errors.md 登记。
3. game.proto +7 RPC → `bash script/goctl.sh game`。
4. core/task 纯函数（board/pass/member/period）+ 单测全绿。
5. 7 logic（list/signin/claim/pass/member/apply/unlock——计划快照加载 + 事务状态机）。
6. 网关 5 端点 + 抽盒/记忆翻牌购买挂钩 ApplyTaskProgress。
7. 前端 Tasks 页 + services + i18n。
8. 场景 31 → run_all 28 场景 ×2 全绿。
9. 边界门 + 提交（Co-Authored-By，排除清单）+ memory 更新。

## 实施记录（2026-10-25）

**步骤 1-8 全部完成**；步骤 9 边界门执行中。关键实现事实与修正：

1. **错误码以 D6 为准**：claim 未达标 3046 / 超 cycleLimit 3047 / 参数无效 3048 / pass 未达 3049 / pass 槽位已领 3050 / 会员未达档 3051 / 会员档已领 3052 / 状态冲突 3053 / apply 非法 3054；不存在 → 404。任务不存在或非 SIGN_IN 的 sign-in → 404（老项目同款）。
2. **View 状态机与老项目逐字一致**（biz/task.go:430-446 对照 core/task/task.go:399-409）：`CycleLimit>0 且 claimed≥limit` → FINISHED；否则 SIGN_IN：今天已签且 streak≥Target → CLAIMABLE；否则 `amount≥Target×(claimed+1)` → CLAIMABLE。**cycleLimit=0（不限）的任务永不 FINISHED**——场景 31 按此断言（task1/task3 claim 后回 IN_PROGRESS、task2 保持 CLAIMABLE）；为覆盖 FINISHED 分支，v2 计划加 task4（cycleLimit=1，claim 后 FINISHED，新 key 再领 3047）。
3. **claim 落库**：claim 行 PK=request_id 幂等重放原样回；`task_progress.claimed_count` 经 lockProgress（无行先 INSERT IGNORE 占位再锁）递增——SIGN_IN 任务 claim 也会占位一行（场景 31 断言 task_progress 共 4 行）。
4. **ApplyTaskProgress 三联动**：INSERT IGNORE task_event 去重 → 匹配 metric 任务 amount 累加 + 所有 Pass XP（LEAST 封顶 XPPerLevel×MaxLevel）+ member spend（溢饱和）；重复事件直接返回不重复计。
5. **会员 claim 不更新 task_member.level**（老项目 state_machine.go 同款：仅 LockMember 检查 + 插 claim 行）；level 仅由 spend 应用路径（applyMember 乐观锁 UPDATE）维护——场景 31 断言 claim 后 level 保持 0。
6. **网关 handler 修复**：claim/sign-in/pass 三个 POST 端点带路径参数，初始实现两次 `httpx.Parse` 读同一 body 导致 EOF（500）——改为 `httpx.ParsePath`（路径结构）+ `httpx.Parse`（body 结构），与 dig-boxes 路径参数先例一致。
7. **gameClient 别名不含枚举**：网关 logic 直接 import `gacha/app/game/rpc/game` 取 TaskMetric/PassTrack/TaskPeriod 常量；game.Definition 的 TaskId/Version 为 uint64，网关类型转 int64。
8. **e2e 工具**：test/tools/taskcli/main.go（grpc 直连 9315）：`apply <userId> <eventId> <metric> <amount>` / `unlock-pass <userId> <passId>`；业务码=grpc status code（ServerErrorInterceptor），业务错误输出 `code=XXXX` 行退出 1。
9. **计划生成**：`.smoke_risk/hashgen`（一次性冒烟目录，不提交）与 game-rpc `ConfigHashOf` 同源：clone → Version=0/Status=UNSPECIFIED/ConfigHash=nil/时间戳=0 → Deterministic marshal → SHA256；v1 bootstrap 与 M14e 种子逐字节一致，v2 见 D7。
10. **场景 31 复跑幂等**：首尾各做 8 表清 + 计划种子/复位，standalone 连跑两遍全绿（run_all 全量边界门另跑）。
11. **签到日期推算**：场景用签到响应返回的 calendarDate（上海日历日）推算"昨天"再 SQL 改写 last_date，规避 MySQL 服务器时区与 Asia/Shanghai 的午夜歧义。
