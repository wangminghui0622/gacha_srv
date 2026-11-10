<template>
  <a-tabs>
    <a-tab-pane key="tasks" :tab="$t('taskEditor.tasks')">
      <a-button :disabled="disabled" @click="model.tasks.push(newTask())">{{ $t('taskEditor.addTask') }}</a-button>
      <a-empty v-if="!model.tasks.length" :description="$t('taskEditor.notConfigured')" />
      <a-card v-for="(row, index) in model.tasks" :key="index" size="small" style="margin-top: 12px" :title="$t('taskEditor.taskN', { n: index + 1 })">
        <template #extra><a-button danger :disabled="disabled" @click="model.tasks.splice(index, 1)">{{ $t('common.delete') }}</a-button></template>
        <a-form layout="vertical">
          <a-row :gutter="12">
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.taskId')" :validate-status="errors[`tasks.${index}.taskId`] ? 'error' : undefined" :help="errors[`tasks.${index}.taskId`]"><a-input v-model:value="row.taskId" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.configVersion')" :validate-status="errors[`tasks.${index}.version`] ? 'error' : undefined" :help="errors[`tasks.${index}.version`]"><a-input v-model:value="row.version" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('common.name')" :validate-status="errors[`tasks.${index}.name`] ? 'error' : undefined" :help="errors[`tasks.${index}.name`]"><a-input v-model:value="row.name" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.period')" :validate-status="errors[`tasks.${index}.period`] ? 'error' : undefined" :help="errors[`tasks.${index}.period`]"><a-select v-model:value="row.period" :disabled="disabled" :options="periods" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.metric')" :validate-status="errors[`tasks.${index}.metric`] ? 'error' : undefined" :help="errors[`tasks.${index}.metric`]"><a-select v-model:value="row.metric" :disabled="disabled" :options="metrics" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.target')" :validate-status="errors[`tasks.${index}.target`] ? 'error' : undefined" :help="errors[`tasks.${index}.target`]"><a-input v-model:value="row.target" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.cycleLimit')" :validate-status="errors[`tasks.${index}.cycleLimit`] ? 'error' : undefined" :help="errors[`tasks.${index}.cycleLimit`]"><a-input-number v-model:value="row.cycleLimit" :disabled="disabled" :precision="0" :min="0" style="width: 100%" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.rewardTemplate')" :validate-status="errors[`tasks.${index}.rewardTemplateId`] ? 'error' : undefined" :help="errors[`tasks.${index}.rewardTemplateId`]"><a-input v-model:value="row.rewardTemplateId" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.rewardCount')" :validate-status="errors[`tasks.${index}.rewardCount`] ? 'error' : undefined" :help="errors[`tasks.${index}.rewardCount`]"><a-input-number v-model:value="row.rewardCount" :disabled="disabled" :precision="0" :min="0" style="width: 100%" /></a-form-item></a-col>
          </a-row>
        </a-form>
      </a-card>
    </a-tab-pane>
    <a-tab-pane key="passes" :tab="$t('taskEditor.passes')">
      <a-button :disabled="disabled" @click="model.passes.push(newPass())">{{ $t('taskEditor.addPass') }}</a-button>
      <a-empty v-if="!model.passes.length" :description="$t('taskEditor.notConfigured')" />
      <a-card v-for="(row, index) in model.passes" :key="index" size="small" style="margin-top: 12px" :title="$t('taskEditor.passN', { n: index + 1 })">
        <template #extra><a-button danger :disabled="disabled" @click="model.passes.splice(index, 1)">{{ $t('common.delete') }}</a-button></template>
        <a-form layout="vertical">
          <a-row :gutter="12">
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.passId')" :validate-status="errors[`passes.${index}.passId`] ? 'error' : undefined" :help="errors[`passes.${index}.passId`]"><a-input v-model:value="row.passId" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.configVersion')" :validate-status="errors[`passes.${index}.version`] ? 'error' : undefined" :help="errors[`passes.${index}.version`]"><a-input v-model:value="row.version" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('common.name')" :validate-status="errors[`passes.${index}.name`] ? 'error' : undefined" :help="errors[`passes.${index}.name`]"><a-input v-model:value="row.name" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.xpPerLevel')" :validate-status="errors[`passes.${index}.xpPerLevel`] ? 'error' : undefined" :help="errors[`passes.${index}.xpPerLevel`]"><a-input v-model:value="row.xpPerLevel" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.maxLevel')" :validate-status="errors[`passes.${index}.maxLevel`] ? 'error' : undefined" :help="errors[`passes.${index}.maxLevel`]"><a-input-number v-model:value="row.maxLevel" :disabled="disabled" :precision="0" :min="0" style="width: 100%" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.xpPerSpendMinor')" :validate-status="errors[`passes.${index}.xpPerSpendMinor`] ? 'error' : undefined" :help="errors[`passes.${index}.xpPerSpendMinor`]"><a-input v-model:value="row.xpPerSpendMinor" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.premiumLine')" :validate-status="errors[`passes.${index}.premiumLine`] ? 'error' : undefined" :help="errors[`passes.${index}.premiumLine`]"><a-select v-model:value="row.premiumLine" :disabled="disabled" :options="premiumLines" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.premiumMemberLevel')" :validate-status="errors[`passes.${index}.premiumMemberLevel`] ? 'error' : undefined" :help="errors[`passes.${index}.premiumMemberLevel`]"><a-input-number v-model:value="row.premiumMemberLevel" :disabled="disabled" :precision="0" :min="0" style="width: 100%" /></a-form-item></a-col>
          </a-row>
        </a-form>
        <a-button :disabled="disabled" @click="(row.rewards ??= []).push(newReward())">{{ $t('taskEditor.addReward') }}</a-button>
        <a-alert v-if="errors[`passes.${index}.rewards`]" type="error" :message="errors[`passes.${index}.rewards`]" />
        <a-card v-for="(reward, rewardIndex) in row.rewards" :key="rewardIndex" size="small" style="margin-top: 8px">
          <a-space wrap>
            <span>{{ $t('taskEditor.level') }}</span>
            <a-input-number v-model:value="reward.level" :disabled="disabled" :precision="0" :min="1" />
            <a-select v-model:value="reward.track" :disabled="disabled" :options="tracks" style="width: 110px" />
            <a-input v-model:value="reward.rewardTemplateId" :disabled="disabled" :placeholder="$t('taskEditor.rewardTemplate')" />
            <a-input-number v-model:value="reward.rewardCount" :disabled="disabled" :precision="0" :min="1" :max="1000" />
            <a-button danger :disabled="disabled" @click="row.rewards?.splice(rewardIndex, 1)">{{ $t('common.delete') }}</a-button>
          </a-space>
          <a-alert v-if="errors[`passes.${index}.rewards.${rewardIndex}`]" type="error" :message="errors[`passes.${index}.rewards.${rewardIndex}`]" />
        </a-card>
      </a-card>
    </a-tab-pane>
    <a-tab-pane key="memberTiers" :tab="$t('taskEditor.memberTiers')">
      <a-button :disabled="disabled" @click="model.memberTiers.push(newTier())">{{ $t('taskEditor.addTier') }}</a-button>
      <a-empty v-if="!model.memberTiers.length" :description="$t('taskEditor.notConfigured')" />
      <a-card v-for="(row, index) in model.memberTiers" :key="index" size="small" style="margin-top: 12px" :title="$t('taskEditor.tierN', { n: index + 1 })">
        <template #extra><a-button danger :disabled="disabled" @click="model.memberTiers.splice(index, 1)">{{ $t('common.delete') }}</a-button></template>
        <a-form layout="vertical">
          <a-row :gutter="12">
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.tierLevel')" :validate-status="errors[`memberTiers.${index}.level`] ? 'error' : undefined" :help="errors[`memberTiers.${index}.level`]"><a-input-number v-model:value="row.level" :disabled="disabled" :precision="0" :min="0" style="width: 100%" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.spendMinor')" :validate-status="errors[`memberTiers.${index}.spendMinor`] ? 'error' : undefined" :help="errors[`memberTiers.${index}.spendMinor`]"><a-input v-model:value="row.spendMinor" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.rewardTemplateId')" :validate-status="errors[`memberTiers.${index}.rewardTemplateId`] ? 'error' : undefined" :help="errors[`memberTiers.${index}.rewardTemplateId`]"><a-input v-model:value="row.rewardTemplateId" :disabled="disabled" /></a-form-item></a-col>
            <a-col :xs="24" :md="12"><a-form-item :label="$t('taskEditor.rewardCount')" :validate-status="errors[`memberTiers.${index}.rewardCount`] ? 'error' : undefined" :help="errors[`memberTiers.${index}.rewardCount`]"><a-input-number v-model:value="row.rewardCount" :disabled="disabled" :precision="0" :min="0" style="width: 100%" /></a-form-item></a-col>
          </a-row>
        </a-form>
      </a-card>
    </a-tab-pane>
  </a-tabs>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PlanContent } from '@/views/operations/taskPlanForm'
import { newTask, newPass, newTier, newReward } from '@/views/operations/taskPlanForm'

const model = defineModel<PlanContent>({ required: true })
defineProps<{ errors: Record<string, string>; disabled: boolean }>()
const { t } = useI18n()

// 选项文案走 i18n → computed 保证语言切换后下拉即时刷新。
const periods = computed(() => [
  { value: 'TASK_PERIOD_ONCE', label: t('taskEditor.periods.once') },
  { value: 'TASK_PERIOD_DAILY', label: t('taskEditor.periods.daily') },
  { value: 'TASK_PERIOD_WEEKLY', label: t('taskEditor.periods.weekly') },
])
const metrics = computed(() => [
  { value: 'TASK_METRIC_DRAW_COUNT', label: t('taskEditor.metrics.drawCount') },
  { value: 'TASK_METRIC_DRAW_SPEND_MINOR', label: t('taskEditor.metrics.drawSpendMinor') },
  { value: 'TASK_METRIC_LOGIN', label: t('taskEditor.metrics.login') },
  { value: 'TASK_METRIC_RECHARGE_MINOR', label: t('taskEditor.metrics.rechargeMinor') },
  { value: 'TASK_METRIC_SIGN_IN', label: t('taskEditor.metrics.signIn') },
])
const premiumLines = computed(() => [
  { value: 'PASS_PREMIUM_LINE_UNSPECIFIED', label: t('taskEditor.premiumLines.unspecified') },
  { value: 'PASS_PREMIUM_LINE_PAYMENT', label: t('taskEditor.premiumLines.payment') },
  { value: 'PASS_PREMIUM_LINE_MEMBER', label: t('taskEditor.premiumLines.member') },
])
const tracks = computed(() => [
  { value: 'PASS_TRACK_FREE', label: t('taskEditor.tracks.free') },
  { value: 'PASS_TRACK_PREMIUM', label: t('taskEditor.tracks.premium') },
])
</script>
