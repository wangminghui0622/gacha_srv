import { http } from '@/utils/request'

export interface Announcement {
  id?: string | number
  title?: string
  type?: number
  publishAtUnixMillis?: string | number
  visible?: boolean
  jumpType?: string
  jumpTarget?: string
  content?: string
  createdAtUnixMillis?: string | number
  updatedAtUnixMillis?: string | number
}

export interface ListAnnouncementsReply {
  items?: Announcement[]
  total?: string
}

export function announcementClient(requestId?: string) {
  const headers = requestId ? { 'X-Request-Id': requestId } : undefined
  return {
    List(params: { page?: number; pageSize?: number }): Promise<ListAnnouncementsReply> {
      return http.get('/console/v2/api/announcements', {
        params: { page: params.page ?? 1, pageSize: params.pageSize ?? 15 },
      })
    },
    Get(id: string | number): Promise<Announcement> {
      return http.get(`/console/v2/api/announcements/${encodeURIComponent(String(id))}`)
    },
    Create(announcement: Announcement): Promise<Announcement> {
      return http.post('/console/v2/api/announcements', announcement, { headers })
    },
    Update(id: string | number, announcement: Announcement): Promise<Announcement> {
      return http.put(`/console/v2/api/announcements/${encodeURIComponent(String(id))}`, announcement, { headers })
    },
  }
}
