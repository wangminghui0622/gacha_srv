<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as tasksApi from '@/services/tasks'
import { randomUUID } from '@/utils/id'
import type { ListTasksResponse, MemberView, PassViewItem, TaskItem } from '@/services/types'

// 任务中心：每日签到 + 任务列表 + 通行证 + 会员（web Tasks 页同流程）
const { t } = useI18n()

const loading = ref(true)
const error = ref('')
const tasks = ref<TaskItem[]>([])
const passes = ref<PassViewItem[]>([])
const member = ref<MemberView | null>(null)
// 已领槽位（页面内会话记忆：服务端视图不含槽位领取态，领取成功后本地标记）
const claimedPassSlots = ref<Set<string>>(new Set())
const claimedMemberLevels = ref<Set<number>>(new Set())
const busy = ref('') // 正在处理的操作键

// 签到任务卡（metric 5 SIGN_IN）
const signInTask = computed(() => tasks.value.find((x) => x.metric === 5))

// 任务状态 → 标签（1 IN_PROGRESS 2 CLAIMABLE 3 FINISHED）
const stateTag: Record<number, { text: string; color: string }> = {
  1: { text: 'tasks.stateInProgress', color: 'blue' },
  2: { text: 'tasks.stateClaimable', color: 'green' },
  3: { text: 'tasks.stateFinished', color: 'default' },
}

// 进度文案（按 metric）
function progressText(item: TaskItem): string {
  switch (item.metric) {
    case 2: // DRAW_SPEND_MINOR
      return t('tasks.progressSpend', { cur: item.progress, target: item.target })
    case 1: // DRAW_COUNT
      return t('tasks.progressDraws', { cur: item.progress, target: item.target })
    case 5: // SIGN_IN
      return t('tasks.progressStreak', { cur: item.progress, target: item.target })
    case 3: // LOGIN
      return t('tasks.progressLogins', { cur: item.progress, target: item.target })
    case 4: // RECHARGE_MINOR
      return t('tasks.progressRecharge', { cur: item.progress, target: item.target })
    default:
      return `${item.progress}/${item.target}`
  }
}

// 奖励文案（legacy 字符串奖励，记录不发放）
function rewardText(rewardTemplateId: string, rewardCount: number): string {
  return `${rewardTemplateId} ×${rewardCount}`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const resp: ListTasksResponse = await tasksApi.listTasks()
    tasks.value = resp.tasks
    passes.value = resp.passes ?? []
    member.value = resp.member ?? null
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

// 每日签到
async function signIn() {
  const task = signInTask.value
  if (!task) return
  busy.value = `sign-${task.taskId}`
  try {
    const r = await tasksApi.signInTask(task.taskId, task.version)
    if (r.alreadySigned) {
      uni.showToast({ title: t('tasks.signedToday'), icon: 'none' })
    } else {
      uni.showToast({ title: t('tasks.signSuccess', { streak: r.streak }), icon: 'success' })
    }
    await load()
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    busy.value = ''
  }
}

// 任务奖励领取（requestId 幂等重放）
async function claim(task: TaskItem) {
  busy.value = `task-${task.taskId}`
  try {
    const r = await tasksApi.claimTask(task.taskId, task.version, randomUUID())
    uni.showToast({
      title: t('tasks.claimSuccess', { reward: rewardText(r.rewardTemplateId, r.rewardCount) }),
      icon: 'success',
    })
    await load()
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    busy.value = ''
  }
}

// 通行证槽位：是否可领（等级达标 && FREE 或已解锁 premium；已领本地记忆）
function passSlotClaimable(pass: PassViewItem, level: number, track: number): boolean {
  if (claimedPassSlots.value.has(`${pass.passId}:${level}:${track}`)) return false
  if (pass.level < level) return false
  return track === 1 || pass.premium
}

// 通行证奖励领取
async function claimPass(pass: PassViewItem, level: number, track: number) {
  const key = `${pass.passId}:${level}:${track}`
  busy.value = key
  try {
    const r = await tasksApi.claimPassReward(pass.passId, level, track, randomUUID())
    claimedPassSlots.value.add(key)
    uni.showToast({
      title: t('tasks.claimSuccess', { reward: rewardText(r.rewardTemplateId, r.rewardCount) }),
      icon: 'success',
    })
    await load()
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    busy.value = ''
  }
}

// 会员档位领取
async function claimMember(level: number) {
  busy.value = `member-${level}`
  try {
    const r = await tasksApi.claimMemberReward(level, randomUUID())
    claimedMemberLevels.value.add(level)
    uni.showToast({
      title: t('tasks.claimSuccess', { reward: rewardText(r.rewardTemplateId, r.rewardCount) }),
      icon: 'success',
    })
    await load()
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    busy.value = ''
  }
}

onLoad(() => {
  void load()
})
</script>

<template>
  <view>
    <view class="page-title">{{ t('tasks.title') }}</view>
    <view v-if="loading" class="list-empty">{{ t('common.loading') }}</view>
    <view v-else-if="error" class="error-text">{{ error }}</view>
    <template v-else>
      <!-- 签到卡（SIGN_IN 任务） -->
      <template v-if="signInTask">
        <view class="section-title section-title-page">{{ t('tasks.signInTitle') }}</view>
        <view class="gacha-card task-card">
          <view class="task-main">
            <view class="task-name">{{ signInTask.name }}</view>
            <view class="task-sub accent">{{ t('tasks.streak', { n: signInTask.progress }) }}</view>
            <view class="task-sub">{{ progressText(signInTask) }}</view>
          </view>
          <view
            class="btn-primary task-btn"
            :class="{ disabled: busy === `sign-${signInTask.taskId}` }"
            @click="signIn"
          >
            {{ t('tasks.signIn') }}
          </view>
        </view>
      </template>

      <!-- 任务列表 -->
      <view class="section-title section-title-page">{{ t('tasks.taskList') }}</view>
      <view v-if="tasks.length === 0" class="list-empty">{{ t('tasks.empty') }}</view>
      <view v-for="task in tasks" :key="task.taskId" class="gacha-card task-card">
        <view class="task-main">
          <view class="task-name">
            <text>{{ task.name }}</text>
            <text
              class="tag-chip"
              :class="{
                'tag-green': stateTag[task.state]?.color === 'green',
                'tag-blue': stateTag[task.state]?.color === 'blue',
              }"
            >
              {{ t(stateTag[task.state]?.text ?? 'tasks.stateInProgress') }}
            </text>
          </view>
          <view class="task-sub">{{ progressText(task) }}</view>
          <view class="task-sub">{{ t('tasks.reward', { reward: rewardText(task.rewardTemplateId, task.rewardCount) }) }}</view>
          <view v-if="task.cycleLimit > 0" class="task-sub">
            {{ t('tasks.claimedCount', { n: task.claimed, limit: task.cycleLimit }) }}
          </view>
        </view>
        <view
          class="btn-plain task-btn"
          :class="{ disabled: task.state !== 2 || busy === `task-${task.taskId}` }"
          @click="claim(task)"
        >
          {{ task.state === 3 ? t('tasks.finished') : t('tasks.claim') }}
        </view>
      </view>

      <!-- 通行证赛季卡 -->
      <template v-if="passes.length > 0">
        <view class="section-title section-title-page">{{ t('tasks.passTitle') }}</view>
        <view v-for="pass in passes" :key="pass.passId" class="gacha-card pass-card">
          <view class="task-name">
            <text>{{ pass.name }}</text>
            <text v-if="pass.premium" class="tag-chip tag-gold">{{ t('tasks.premium') }}</text>
          </view>
          <view
            class="progress-track pass-progress"
            :class="{ done: pass.xp >= pass.xpPerLevel * pass.maxLevel }"
          >
            <view
              class="progress-fill"
              :style="{ width: Math.min(100, Math.round((pass.xp / (pass.xpPerLevel * pass.maxLevel)) * 100)) + '%' }"
            ></view>
          </view>
          <view class="task-sub">{{ t('tasks.passLevel', { level: pass.level, xp: pass.xp }) }}</view>
          <view class="pass-rewards">
            <view
              v-for="reward in pass.rewards"
              :key="`${reward.level}-${reward.track}`"
              class="pass-reward-row"
            >
              <text class="task-sub">
                {{ t('tasks.passSlot', { level: reward.level, track: reward.track === 1 ? t('tasks.trackFree') : t('tasks.trackPremium') }) }}：
                {{ rewardText(reward.rewardTemplateId, reward.rewardCount) }}
              </text>
              <view
                class="btn-plain task-btn"
                :class="{ disabled: !passSlotClaimable(pass, reward.level, reward.track) || busy === `${pass.passId}:${reward.level}:${reward.track}` }"
                @click="claimPass(pass, reward.level, reward.track)"
              >
                {{ t('tasks.claim') }}
              </view>
            </view>
          </view>
          <view class="task-sub">{{ t('tasks.passXpPerSpend', { n: pass.xpPerSpendMinor }) }}</view>
        </view>
      </template>

      <!-- 会员卡 -->
      <template v-if="member">
        <view class="section-title section-title-page">{{ t('tasks.memberTitle') }}</view>
        <view class="gacha-card member-card">
          <view class="task-name">{{ t('tasks.memberLevel', { level: member.level }) }}</view>
          <view class="task-sub accent">{{ t('tasks.memberSpend', { n: member.spendMinor }) }}</view>
          <view v-for="tier in member.tiers" :key="tier.level" class="member-tier-row">
            <text class="task-sub">
              Lv.{{ tier.level }}（≥{{ tier.spendMinor }}）：{{ rewardText(tier.rewardTemplateId, tier.rewardCount) }}
            </text>
            <view
              class="btn-plain task-btn"
              :class="{ disabled: member.spendMinor < tier.spendMinor || claimedMemberLevels.has(tier.level) || busy === `member-${tier.level}` }"
              @click="claimMember(tier.level)"
            >
              {{ t('tasks.claim') }}
            </view>
          </view>
        </view>
      </template>
    </template>
  </view>
</template>

<style scoped lang="scss">
.section-title-page {
  margin-left: 24rpx;
}

.error-text {
  color: var(--color-danger);
  text-align: center;
}

.task-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24rpx;
  margin-bottom: 16rpx;

  .task-main {
    flex: 1;
    min-width: 0;

    .task-name {
      font-size: 30rpx;
      font-weight: 600;
      display: flex;
      align-items: center;

      .tag-chip {
        margin-left: 12rpx;
      }
    }

    .task-sub {
      font-size: 24rpx;
      color: var(--color-text-secondary);
      margin-top: 4rpx;
    }
  }

  .task-btn {
    line-height: 68rpx;
    padding: 0 32rpx;
    font-size: 26rpx;
    flex-shrink: 0;
  }
}

.pass-card,
.member-card {
  margin-bottom: 16rpx;
  padding: 24rpx;

  .task-name {
    font-size: 30rpx;
    font-weight: 600;
    display: flex;
    align-items: center;

    .tag-chip {
      margin-left: 12rpx;
    }
  }

  .task-sub {
    font-size: 24rpx;
    color: var(--color-text-secondary);
    margin-top: 4rpx;
  }

  .pass-progress {
    margin: 16rpx 0 8rpx;
  }
}

.pass-reward-row,
.member-tier-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24rpx;
  margin-top: 16rpx;

  .task-sub {
    flex: 1;
    min-width: 0;
    margin-top: 0;
  }

  .task-btn {
    line-height: 60rpx;
    padding: 0 28rpx;
    font-size: 24rpx;
    flex-shrink: 0;
  }
}
</style>
