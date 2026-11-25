import { http } from '@/utils/request'

export interface Popup {
  id?: string | number
  activeDisplay?: boolean
  status?: string
  sortOrder?: number
  name?: string
  type?: string
  condition?: string
  jumpType?: string
  jumpTarget?: string
  imageUrl?: string
  content?: string
  createdAtUnixMillis?: string | number
  updatedAtUnixMillis?: string | number
}

export interface ListPopupsReply {
  items?: Popup[]
  total?: string
}

export function popupClient(requestId?: string) {
  const headers = requestId ? { 'X-Request-Id': requestId } : undefined
  return {
    List(params: { page?: number; pageSize?: number }): Promise<ListPopupsReply> {
      return http.get('/console/v2/api/popups', {
        params: { page: params.page ?? 1, pageSize: params.pageSize ?? 15 },
      })
    },
    Get(id: string | number): Promise<Popup> {
      return http.get(`/console/v2/api/popups/${encodeURIComponent(String(id))}`)
    },
    Create(popup: Popup): Promise<Popup> {
      return http.post('/console/v2/api/popups', popup, { headers })
    },
    Update(id: string | number, popup: Popup): Promise<Popup> {
      return http.put(`/console/v2/api/popups/${encodeURIComponent(String(id))}`, popup, { headers })
    },
  }
}
