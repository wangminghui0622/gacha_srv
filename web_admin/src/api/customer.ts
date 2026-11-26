import { http } from '@/utils/request'

export interface ConsoleUser {
  userId?: string | number
  publicUserId?: string
  username?: string
  nickname?: string
  phone?: string
  avatarUrl?: string
  status?: number
  luckyDisplay?: string
  registerIp?: string
  registerDevice?: string
  rebateRate?: number
  identity?: string
  parentUserId?: string | number
  reissueLuckPoints?: boolean
  createdAtUnixMillis?: string | number
  updatedAtUnixMillis?: string | number
}

export interface ListConsoleUsersReply {
  items?: ConsoleUser[]
  total?: string
}

export interface WalletSetting {
  kind?: number
  enabled?: boolean
  balance?: string
}

export interface UserFinance {
  userId?: string | number
  wallets?: WalletSetting[]
  refundLimitFen?: string | number
  refundLimitCount?: number
  effectiveLimitFen?: string | number
  effectiveLimitCount?: number
}

export interface UpdateUserFinancePayload {
  wallets: Array<{ kind: number; enabled: boolean }>
  refundLimitFen: string | number
  refundLimitCount: number
}

export interface PaymentBinding {
  id?: string | number
  createdAtUnixMillis?: string | number
  userId?: string | number
  name?: string
  paymentAccount?: string
}

export interface RealnameBinding {
  id?: string | number
  createdAtUnixMillis?: string | number
  userId?: string | number
  name?: string
  idCard?: string
  phone?: string
}

export interface UserBindings {
  payments?: PaymentBinding[]
  realnames?: RealnameBinding[]
}

export interface CreateConsoleUserPayload {
  nickname: string
  phone?: string
  avatarUrl: string
  rebateRate: number
  identity: string
}

export interface UpdateConsoleUserPayload {
  nickname: string
  status: number
  luckyDisplay: string
  parentUserId: number
  reissueLuckPoints: boolean
  avatarUrl: string
  rebateRate: number
  identity: string
}

export function customerClient(requestId?: string) {
  const headers = requestId ? { 'X-Request-Id': requestId } : undefined
  return {
    List(params: {
      keyword?: string
      phone?: string
      identity?: string
      status?: number
      page?: number
      pageSize?: number
    }): Promise<ListConsoleUsersReply> {
      return http.get('/console/v2/api/users', {
        params: {
          keyword: params.keyword ?? '',
          phone: params.phone ?? '',
          identity: params.identity ?? '',
          status: params.status ?? 0,
          page: params.page ?? 1,
          pageSize: params.pageSize ?? 15,
        },
      })
    },
    Create(payload: CreateConsoleUserPayload): Promise<ConsoleUser> {
      return http.post('/console/v2/api/users', payload, { headers })
    },
    Get(id: string | number): Promise<ConsoleUser> {
      return http.get(`/console/v2/api/users/${encodeURIComponent(String(id))}`)
    },
    Update(id: string | number, payload: UpdateConsoleUserPayload): Promise<ConsoleUser> {
      return http.put(`/console/v2/api/users/${encodeURIComponent(String(id))}`, payload, { headers })
    },
    GetFinance(id: string | number): Promise<UserFinance> {
      return http.get(`/console/v2/api/users/${encodeURIComponent(String(id))}/finance`)
    },
    UpdateFinance(id: string | number, payload: UpdateUserFinancePayload): Promise<UserFinance> {
      return http.put(`/console/v2/api/users/${encodeURIComponent(String(id))}/finance`, payload, { headers })
    },
    Recharge(id: string | number, payload: { amount: string; kind: number; remark?: string }): Promise<{ balance?: string }> {
      return http.post(`/console/v2/api/users/${encodeURIComponent(String(id))}/finance/recharge`, payload, { headers })
    },
    RefundBeans(id: string | number, payload: { amount: string }): Promise<{ ok?: boolean }> {
      return http.post(`/console/v2/api/users/${encodeURIComponent(String(id))}/finance/refund-beans`, payload, { headers })
    },
    GetBindings(id: string | number): Promise<UserBindings> {
      return http.get(`/console/v2/api/users/${encodeURIComponent(String(id))}/bindings`)
    },
    ResetBindingLimit(id: string | number, kind: 'sms' | 'realname'): Promise<void> {
      return http.post(`/console/v2/api/users/${encodeURIComponent(String(id))}/bindings/reset-${kind}`, {}, { headers })
    },
  }
}
