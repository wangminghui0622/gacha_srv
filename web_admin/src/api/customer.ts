import { http } from '@/utils/request'

export interface ConsoleUser {
  userId?: string | number
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
  createdAtUnixMillis?: string | number
  updatedAtUnixMillis?: string | number
}

export interface ListConsoleUsersReply {
  items?: ConsoleUser[]
  total?: string
}

export interface CreateConsoleUserPayload {
  nickname: string
  phone?: string
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
  }
}
