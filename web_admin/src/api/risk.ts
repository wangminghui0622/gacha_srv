// 风控 typed service：方法名与参考 RiskCaseView 调用点一致（List/Claim/Resolve），
// axios 直连本仓 /risk 路由。
// 相对参考的 wire 差异：List 的 status 本仓为数字（QueryUint32）；
// Claim 无 body（caseId 走路径、operationId 走 Idempotency-Key 头）；
// Resolve body 仅 {status(枚举名), resolutionCode}。
import { http } from '@/utils/request'

// 本仓 risk.case:list 的 status 查询参数为数字。
export const riskStatusNumbers = {
  RISK_CASE_STATUS_OPEN: 1,
  RISK_CASE_STATUS_IN_REVIEW: 2,
  RISK_CASE_STATUS_RESOLVED: 3,
  RISK_CASE_STATUS_DISMISSED: 4,
} as const

export type RiskStatusName = keyof typeof riskStatusNumbers

export interface RiskCaseSummary {
  caseId?: string
  decisionId?: string
  sceneCode?: string
  userId?: string
  deviceId?: string
  action?: string
  riskScore?: number
  status?: string
  assignedActorId?: string
  createdAtUnixMillis?: string
}

export interface RiskCaseTransition {
  caseId?: string
  status?: string
  actorId?: string
  occurredAtUnixMillis?: string
}

export interface ListRiskCasesReply {
  cases?: RiskCaseSummary[]
  nextBeforeCreatedAtUnixMillis?: string
  nextBeforeCaseId?: string
}

export function riskClient(idempotencyKey?: string) {
  const headers = idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined
  return {
    List({ status, pageSize, beforeCreatedAtUnixMillis, beforeCaseId }: {
      status?: number
      pageSize?: number
      beforeCreatedAtUnixMillis?: string
      beforeCaseId?: string
    }): Promise<ListRiskCasesReply> {
      return http.get('/console/v2/api/risk/cases', {
        params: {
          ...(status ? { status } : {}),
          pageSize: pageSize ?? 20,
          ...(beforeCreatedAtUnixMillis ? { beforeCreatedAtUnixMillis } : {}),
          ...(beforeCaseId ? { beforeCaseId } : {}),
        },
      })
    },
    Claim({ caseId }: { caseId: string }): Promise<RiskCaseTransition> {
      return http.post(`/console/v2/api/risk/cases/${encodeURIComponent(caseId)}/claim`, {}, { headers })
    },
    Resolve(payload: { caseId: string; status: RiskStatusName | 'RISK_CASE_STATUS_RESOLVED' | 'RISK_CASE_STATUS_DISMISSED'; resolutionCode: string }): Promise<RiskCaseTransition> {
      const { caseId, ...body } = payload
      return http.post(`/console/v2/api/risk/cases/${encodeURIComponent(caseId)}/resolve`, body, { headers })
    },
  }
}
