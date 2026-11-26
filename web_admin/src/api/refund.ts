import { http } from '@/utils/request'

export type RefundOrderStatus =
  | 'REFUND_ORDER_STATUS_PENDING'
  | 'REFUND_ORDER_STATUS_APPROVED'
  | 'REFUND_ORDER_STATUS_SUCCESS'
  | 'REFUND_ORDER_STATUS_REJECTED'
  | 'REFUND_ORDER_STATUS_EXCEPTION'
  | 'REFUND_ORDER_STATUS_MANUAL'

export interface RefundOrder {
  dbId?: string
  orderNo?: string
  userId?: string
  userNickname?: string
  realName?: string
  alipayAccount?: string
  amountFen?: string
  status?: RefundOrderStatus
  createdAtUnixMillis?: string
  completedAtUnixMillis?: string
  thirdPartyOrderId?: string
  refundMerchantId?: string
  auditor?: string
  financeAuditor?: string
  auditRemark?: string
  errorMessage?: string
}

export interface ListRefundOrdersReply {
  items?: RefundOrder[]
  total?: string
}

export interface ListRefundOrdersParams {
  keyword?: string
  auditRemark?: string
  status?: number
  minAmountFen?: number
  maxAmountFen?: number
  timeField?: string
  startAtUnixMillis?: number
  endAtUnixMillis?: number
  page?: number
  pageSize?: number
}

export interface RefundAutoConfig {
  remainingAmountFen?: string
  auditLimitFen?: string
  delaySeconds?: string
  alternateCount?: number
  alternateAmountFen?: string
}

export const REFUND_ORDER_ACTIONS = {
  APPROVE: 1,
  FORCE_REFUND: 2,
  MANUAL: 3,
  RETRY: 4,
} as const

export function refundOrderClient() {
  return {
    List(params: ListRefundOrdersParams): Promise<ListRefundOrdersReply> {
      return http.get('/console/v2/api/refund-orders', {
        params: {
          keyword: params.keyword ?? '',
          auditRemark: params.auditRemark ?? '',
          status: params.status ?? 0,
          minAmountFen: params.minAmountFen ?? 0,
          maxAmountFen: params.maxAmountFen ?? 0,
          timeField: params.timeField ?? 'created',
          startAtUnixMillis: params.startAtUnixMillis ?? 0,
          endAtUnixMillis: params.endAtUnixMillis ?? 0,
          page: params.page ?? 1,
          pageSize: params.pageSize ?? 15,
        },
      })
    },
    Operate(id: string | number, action: number, remark = ''): Promise<RefundOrder> {
      return http.post(`/console/v2/api/refund-orders/${encodeURIComponent(String(id))}/operate`, {
        action,
        remark,
      })
    },
    GetAutoConfig(): Promise<{ config?: RefundAutoConfig }> {
      return http.get('/console/v2/api/refund-auto-config')
    },
    UpdateAutoConfig(config: RefundAutoConfig): Promise<{ config?: RefundAutoConfig }> {
      return http.put('/console/v2/api/refund-auto-config', { config })
    },
  }
}
