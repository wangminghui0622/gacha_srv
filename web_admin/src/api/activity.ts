import { http } from '@/utils/request'

export interface Activity {
  id?: string | number
  activityKey?: string
  name?: string
  description?: string
  protocol?: string
  enabled?: boolean
  startAtUnixMillis?: string | number
  endAtUnixMillis?: string | number
  dailyStart?: string
  dailyEnd?: string
  images?: string[]
  imageMap?: Record<string, string>
  params?: Record<string, string>
  createdAtUnixMillis?: string | number
  updatedAtUnixMillis?: string | number
}

export interface ListActivitiesReply {
  items?: Activity[]
  total?: string
}

export function activityClient(requestId?: string) {
  const headers = requestId ? { 'X-Request-Id': requestId } : undefined
  return {
    List(params: { keyword?: string; page?: number; pageSize?: number }): Promise<ListActivitiesReply> {
      return http.get('/console/v2/api/activities', {
        params: {
          keyword: params.keyword ?? '',
          page: params.page ?? 1,
          pageSize: params.pageSize ?? 20,
        },
      })
    },
    Get(id: string | number): Promise<Activity> {
      return http.get(`/console/v2/api/activities/${encodeURIComponent(String(id))}`)
    },
    Create(activity: Activity): Promise<Activity> {
      return http.post('/console/v2/api/activities', activity, { headers })
    },
    Update(id: string | number, activity: Activity): Promise<Activity> {
      return http.put(`/console/v2/api/activities/${encodeURIComponent(String(id))}`, activity, { headers })
    },
  }
}
