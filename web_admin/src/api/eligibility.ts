// 条件政策 typed service：方法名与参考 EligibilityPolicyView 调用点一致
// （List/Get/Create/CreateRevision/Publish），axios 直连本仓 /eligibility 路由。
import { http } from '@/utils/request'

export interface EligibilityRule {
  ruleId?: string
  expression?: string
  failureCode?: string
}

export interface PolicyVersion {
  policyId?: string
  version?: string
  status?: string
  scene?: string | number
  combination?: string | number
  rules?: EligibilityRule[]
  requiredFacts?: string[]
  configHash?: string
}

export interface EligibilityPolicy {
  policyId?: string
  tenantId?: string
  code?: string
  name?: string
  currentVersion?: string
  latestVersion?: string
}

export interface EligibilityPolicyRecord {
  policy?: EligibilityPolicy
  policyVersion?: PolicyVersion
}

export interface ListPoliciesReply {
  policies?: EligibilityPolicyRecord[]
  nextAfterPolicyId?: string
}

export const eligibilityClient = {
  List({ afterPolicyId, pageSize }: { tenantId?: number; afterPolicyId?: string; pageSize?: number }): Promise<ListPoliciesReply> {
    return http.get('/console/v2/api/eligibility/policies', {
      params: { ...(afterPolicyId ? { afterPolicyId } : {}), pageSize: pageSize ?? 20 },
    })
  },
  Get({ policyId, version }: { tenantId?: number; policyId: string; version: string }): Promise<EligibilityPolicyRecord> {
    return http.get(`/console/v2/api/eligibility/policies/${encodeURIComponent(policyId)}/versions/${encodeURIComponent(version)}`)
  },
  Create(payload: {
    tenantId?: number
    code: string
    name: string
    scene: number
    combination: number
    rules: EligibilityRule[]
    requiredFacts: string[]
  }): Promise<EligibilityPolicyRecord> {
    return http.post('/console/v2/api/eligibility/policies', payload)
  },
  CreateRevision(payload: {
    tenantId?: number
    policyId: string
    name: string
    scene: number
    combination: number
    rules: EligibilityRule[]
    requiredFacts: string[]
    expectedLatestVersion: string | number
  }): Promise<EligibilityPolicyRecord> {
    const { policyId, ...body } = payload
    return http.post(`/console/v2/api/eligibility/policies/${encodeURIComponent(policyId)}/revisions`, body)
  },
  Publish({ policyId, version, expectedConfigHash }: { tenantId?: number; policyId: string; version: string; expectedConfigHash: string }): Promise<EligibilityPolicyRecord> {
    return http.post(
      `/console/v2/api/eligibility/policies/${encodeURIComponent(policyId)}/versions/${encodeURIComponent(version)}/publish`,
      { expectedConfigHash },
    )
  },
}
