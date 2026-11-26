import { http } from '@/utils/request'

export type RechargeOrderStatus =
  | 'RECHARGE_ORDER_STATUS_INIT'
  | 'RECHARGE_ORDER_STATUS_EXCEPTION'
  | 'RECHARGE_ORDER_STATUS_CANCELED'
  | 'RECHARGE_ORDER_STATUS_COMPLETED'
  | 'RECHARGE_ORDER_STATUS_CLOSED'
  | 'RECHARGE_ORDER_STATUS_FAILED'
  | 'RECHARGE_ORDER_STATUS_REFUNDING'
  | 'RECHARGE_ORDER_STATUS_REFUND_EXCEPTION'
  | 'RECHARGE_ORDER_STATUS_REFUNDED'
  | 'RECHARGE_ORDER_STATUS_REFUND_FAILED'
  | 'RECHARGE_ORDER_STATUS_FORCE_REFUNDING'
  | 'RECHARGE_ORDER_STATUS_FORCE_REFUND_EXCEPTION'
  | 'RECHARGE_ORDER_STATUS_FORCE_REFUNDED'
  | 'RECHARGE_ORDER_STATUS_FORCE_REFUND_FAILED'

export interface RechargeOrder {
  dbId?: string
  orderNo?: string
  userId?: string
  userNickname?: string
  amountFen?: string
  creditedFen?: string
  discountFen?: string
  platform?: string
  status?: RechargeOrderStatus
  rechargeType?: string
  createdAtUnixMillis?: string
  completedAtUnixMillis?: string
  refundedAtUnixMillis?: string
  refundAmountFen?: string
  refundOrderNo?: string
  payPlatformOrderId?: string
  payPlatformOpenid?: string
  errorMessage?: string
}

export interface ListRechargeOrdersReply {
  items?: RechargeOrder[]
  total?: string
}

export interface ListRechargeOrdersParams {
  keyword?: string
  status?: number
  rechargeType?: string
  platform?: string
  payKeyword?: string
  minAmountFen?: number
  maxAmountFen?: number
  timeField?: string
  startAtUnixMillis?: number
  endAtUnixMillis?: number
  page?: number
  pageSize?: number
}

export function rechargeOrderClient() {
  return {
    List(params: ListRechargeOrdersParams): Promise<ListRechargeOrdersReply> {
      return http.get('/console/v2/api/recharge-orders', {
        params: {
          keyword: params.keyword ?? '',
          status: params.status ?? 0,
          rechargeType: params.rechargeType ?? '',
          platform: params.platform ?? '',
          payKeyword: params.payKeyword ?? '',
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
    Refund(ids: (string | number)[]): Promise<{ refunded?: number; failedOrderNos?: string[] }> {
      return http.post('/console/v2/api/recharge-orders/refund', { ids })
    },
  }
}
