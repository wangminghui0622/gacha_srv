// 任务计划编辑模型 + 校验（校验与变更摘要文案走 i18n，双语是刻意差异）。
import { i18n } from '@/i18n'
import type { MemberTier, PassSeason, PassReward, TaskDefinition, TaskPlan } from '@/api/task'

export type PlanContent = { tasks: TaskDefinition[]; passes: PassSeason[]; memberTiers: MemberTier[] }

const t = (key: string, params?: Record<string, string | number>) => i18n.global.t(key, params ?? {}) as string

export const contentOf = (plan?: TaskPlan | null): PlanContent => ({
  tasks: (plan?.tasks ?? []).map(row => ({ ...row })),
  passes: (plan?.passes ?? []).map(row => ({ ...row, rewards: (row.rewards ?? []).map(reward => ({ ...reward })) })),
  memberTiers: (plan?.memberTiers ?? []).map(row => ({ ...row })),
})

export const newTask = (): TaskDefinition => ({ taskId: '', version: '1', name: '', period: 'TASK_PERIOD_DAILY', metric: 'TASK_METRIC_DRAW_COUNT', target: '1', cycleLimit: 1, rewardTemplateId: '', rewardCount: 1 })
export const newPass = (): PassSeason => ({ passId: '', version: '1', name: '', xpPerLevel: '100', maxLevel: 1, xpPerSpendMinor: '1', premiumLine: 'PASS_PREMIUM_LINE_UNSPECIFIED', premiumMemberLevel: 0, rewards: [] })
export const newTier = (): MemberTier => ({ level: 1, spendMinor: '', rewardTemplateId: '', rewardCount: 1 })
export const newReward = (): PassReward => ({ level: 1, track: 'PASS_TRACK_FREE', rewardTemplateId: '', rewardCount: 1 })

const positive64 = (value: string | number | undefined, signed = false) => value !== undefined && value !== '' && /^[1-9]\d*$/.test(String(value)) && BigInt(value) <= (signed ? 9223372036854775807n : 18446744073709551615n)
const integer = (value: number | undefined, min: number, max: number) => value !== undefined && Number.isInteger(value) && value >= min && value <= max

export function validateContent(content: PlanContent): Record<string,string> {
  const errors: Record<string,string> = {}
  const require = (path: string, valid: boolean, hint: string) => { if (!valid) errors[path] = hint }
  const ids = new Set<string>()
  content.tasks.forEach((task, index) => {
    const p = `tasks.${index}`
    require(`${p}.taskId`, positive64(task.taskId) && !ids.has(String(task.taskId)), t('taskEditor.validation.uniquePositiveId')); ids.add(String(task.taskId ?? ''))
    require(`${p}.version`, positive64(task.version), t('taskEditor.validation.positiveVersion'))
    require(`${p}.period`, ['TASK_PERIOD_ONCE','TASK_PERIOD_DAILY','TASK_PERIOD_WEEKLY'].includes(String(task.period ?? '')), t('taskEditor.validation.validPeriod'))
    require(`${p}.metric`, ['TASK_METRIC_DRAW_COUNT','TASK_METRIC_DRAW_SPEND_MINOR','TASK_METRIC_LOGIN','TASK_METRIC_RECHARGE_MINOR','TASK_METRIC_SIGN_IN'].includes(String(task.metric ?? '')), t('taskEditor.validation.validMetric'))
    require(`${p}.target`, positive64(task.target,true), t('taskEditor.validation.positiveTarget'))
    require(`${p}.cycleLimit`, integer(task.cycleLimit,0,4294967295), t('taskEditor.validation.cycleLimitRange'))
    require(`${p}.rewardTemplateId`, !!task.rewardTemplateId?.trim(), t('taskEditor.validation.rewardTemplateRequired'))
    require(`${p}.rewardCount`, integer(task.rewardCount,1,1000), t('taskEditor.validation.rewardCountRange'))
  })
  ids.clear()
  content.passes.forEach((pass,index) => {
    const p = `passes.${index}`
    require(`${p}.passId`, positive64(pass.passId) && !ids.has(String(pass.passId)), t('taskEditor.validation.uniquePositiveId')); ids.add(String(pass.passId ?? ''))
    for (const key of ['version','xpPerLevel','xpPerSpendMinor'] as const) require(`${p}.${key}`,positive64(pass[key],key !== 'version'),t('taskEditor.validation.positiveXp'))
    require(`${p}.maxLevel`,integer(pass.maxLevel,1,200),t('taskEditor.validation.maxLevelRange'))
    const rewards = pass.rewards ?? [], keys = new Set<string>()
    require(`${p}.rewards`,rewards.length > 0,t('taskEditor.validation.atLeastOneReward'))
    rewards.forEach((reward,r) => {
      const key = `${reward.level}:${reward.track}`
      require(`${p}.rewards.${r}`,integer(reward.level,1,pass.maxLevel ?? 0) && ['PASS_TRACK_FREE','PASS_TRACK_PREMIUM'].includes(String(reward.track ?? '')) && !!reward.rewardTemplateId?.trim() && integer(reward.rewardCount,1,1000) && !keys.has(key),t('taskEditor.validation.rewardInvalid'))
      keys.add(key)
    })
    const premium = rewards.some(r => r.track === 'PASS_TRACK_PREMIUM')
    require(`${p}.premiumLine`, pass.premiumLine === 'PASS_PREMIUM_LINE_UNSPECIFIED' ? !premium : ['PASS_PREMIUM_LINE_PAYMENT','PASS_PREMIUM_LINE_MEMBER'].includes(String(pass.premiumLine ?? '')) && premium,t('taskEditor.validation.premiumLineMismatch'))
    require(`${p}.premiumMemberLevel`, pass.premiumLine === 'PASS_PREMIUM_LINE_MEMBER' ? integer(pass.premiumMemberLevel,1,4294967295) : pass.premiumMemberLevel === 0,t('taskEditor.validation.premiumMemberLevelInvalid'))
  })
  require('memberTiers',content.memberTiers.length <= 50,t('taskEditor.validation.tierCountLimit'))
  let previous = 0n; const levels = new Set<number>()
  content.memberTiers.forEach((tier,index) => {
    const p = `memberTiers.${index}`
    require(`${p}.level`,integer(tier.level,1,4294967295) && !levels.has(tier.level!), t('taskEditor.validation.uniqueLevel')); levels.add(tier.level ?? 0)
    const valid = positive64(tier.spendMinor,true)
    require(`${p}.spendMinor`,valid && BigInt(tier.spendMinor!) > previous,t('taskEditor.validation.spendIncreasing'))
    if (valid) previous = BigInt(tier.spendMinor!)
    require(`${p}.rewardTemplateId`,!!tier.rewardTemplateId?.trim(),t('taskEditor.validation.rewardTemplateRequired'))
    require(`${p}.rewardCount`,integer(tier.rewardCount,1,1000),t('taskEditor.validation.rewardCountRange'))
  })
  require('plan',content.tasks.length + content.passes.length + content.memberTiers.length > 0,t('taskEditor.validation.planEmpty'))
  return errors
}

export function summarizeChanges(before: PlanContent, after: PlanContent): string[] {
  const result: string[] = []
  function compare<T extends object>(label: string, oldRows: T[], newRows: T[], identity: (row:T) => string) {
    const old = new Map(oldRows.map(row => [identity(row),row])), next = new Map(newRows.map(row => [identity(row),row]))
    for (const [id,row] of next) {
      const previous = old.get(id)
      if (!previous) result.push(`${label} ${id || t('taskEditor.change.unfilledId')}：${t('taskEditor.change.added')}`)
      else { const fields = (Object.keys(row) as (keyof T)[]).filter(key => JSON.stringify(row[key]) !== JSON.stringify(previous[key])); if (fields.length) result.push(`${label} ${id}：${t('taskEditor.change.modified', { fields: fields.join('、') })}`) }
    }
    for (const id of old.keys()) if (!next.has(id)) result.push(`${label} ${id}：${t('taskEditor.change.removed')}`)
  }
  compare(t('taskEditor.change.task'),before.tasks,after.tasks,row => String(row.taskId ?? ''))
  compare(t('taskEditor.change.pass'),before.passes,after.passes,row => String(row.passId ?? ''))
  compare(t('taskEditor.change.tier'),before.memberTiers,after.memberTiers,row => String(row.level ?? ''))
  if (before.memberTiers.map(r=>r.level).join(',') !== after.memberTiers.map(r=>r.level).join(',')) result.push(t('taskEditor.change.orderChanged'))
  return result
}
