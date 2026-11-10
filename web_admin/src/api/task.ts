// 任务计划 typed service：方法名与参考 views 调用点一致（ListPlans/GetPlan/
// CreatePlanDraft/RequestPlanPublication/ApprovePlanPublication/PublishPlan/
// GetPlanPublicationApproval），axios 直连本仓 /task-plans 路由。
// GET approval 无审批行 404 是正常空态 → silentNotFound。
import { http } from '@/utils/request'

export interface TaskReward {
  count?: number
  coupon?: { templateId?: string | number; templateVersion?: string | number }
  inventory?: { productId?: string | number; skuId?: string | number; productVersion?: string | number }
}

export interface TaskDefinition {
  taskId?: string | number
  version?: string | number
  name?: string
  period?: string | number
  metric?: string | number
  target?: string | number
  cycleLimit?: number
  rewardTemplateId?: string
  rewardCount?: number
  reward?: TaskReward
}

export interface PassReward {
  level?: number
  track?: string | number
  rewardTemplateId?: string
  rewardCount?: number
  reward?: TaskReward
}

export interface PassSeason {
  passId?: string | number
  version?: string | number
  name?: string
  xpPerLevel?: string | number
  maxLevel?: number
  rewards?: PassReward[]
  xpPerSpendMinor?: string | number
  premiumLine?: string | number
  premiumMemberLevel?: number
}

export interface MemberTier {
  level?: number
  spendMinor?: string | number
  rewardTemplateId?: string
  rewardCount?: number
  reward?: TaskReward
}

export interface TaskPlan {
  version?: string
  status?: string
  tasks?: TaskDefinition[]
  passes?: PassSeason[]
  memberTiers?: MemberTier[]
  configHash?: string
  createdAtUnixMillis?: string
  publishedAtUnixMillis?: string
}

export interface TaskPlanPublicationApproval {
  version?: string
  configHash?: string
  requesterId?: string
  approverId?: string
  approved?: boolean
  requestedAtUnixMillis?: string
  approvedAtUnixMillis?: string
}

export interface ListTaskPlansReply {
  plans?: TaskPlan[]
  nextAfterVersion?: string
}

export const taskClient = {
  ListPlans({ afterVersion, pageSize }: { afterVersion?: string; pageSize?: number }): Promise<ListTaskPlansReply> {
    return http.get('/console/v2/api/task-plans', { params: { ...(afterVersion ? { afterVersion } : {}), pageSize: pageSize ?? 20 } })
  },
  GetPlan({ version }: { version: string }): Promise<TaskPlan> {
    return http.get(`/console/v2/api/task-plans/${encodeURIComponent(version)}`)
  },
  CreatePlanDraft(payload: {
    expectedLatestVersion: string | number
    tasks?: TaskDefinition[]
    passes?: PassSeason[]
    memberTiers?: MemberTier[]
  }): Promise<TaskPlan> {
    return http.post('/console/v2/api/task-plans/drafts', payload)
  },
  RequestPlanPublication({ version, expectedConfigHash }: { version: string; expectedConfigHash: string }): Promise<TaskPlanPublicationApproval> {
    return http.post(`/console/v2/api/task-plans/${encodeURIComponent(version)}/publication-request`, { expectedConfigHash })
  },
  ApprovePlanPublication({ version, expectedConfigHash }: { version: string; expectedConfigHash: string }): Promise<TaskPlanPublicationApproval> {
    return http.post(`/console/v2/api/task-plans/${encodeURIComponent(version)}/approval`, { expectedConfigHash })
  },
  PublishPlan({ version, expectedConfigHash }: { version: string; expectedConfigHash: string }): Promise<TaskPlan> {
    return http.post(`/console/v2/api/task-plans/${encodeURIComponent(version)}/publish`, { expectedConfigHash })
  },
  GetPlanPublicationApproval({ version }: { version: string }): Promise<TaskPlanPublicationApproval> {
    return http.get(`/console/v2/api/task-plans/${encodeURIComponent(version)}/approval`, { silentNotFound: true })
  },
}
