import { http } from '@/utils/request'

export interface Banner {
  id?: string | number
  position?: number
  type?: string
  name?: string
  imageUrl?: string
  visible?: boolean
  targetPoolId?: string | number
  popupType?: string
  popupTargetType?: string
  popupTargetId?: string
  popupContent?: string
  createdAtUnixMillis?: string | number
  updatedAtUnixMillis?: string | number
}

export interface ListBannersReply {
  items?: Banner[]
  total?: string
}

export function bannerClient(requestId?: string) {
  const headers = requestId ? { 'X-Request-Id': requestId } : undefined
  return {
    List(params: { page?: number; pageSize?: number }): Promise<ListBannersReply> {
      return http.get('/console/v2/api/banners', {
        params: { page: params.page ?? 1, pageSize: params.pageSize ?? 15 },
      })
    },
    Get(id: string | number): Promise<Banner> {
      return http.get(`/console/v2/api/banners/${encodeURIComponent(String(id))}`)
    },
    Create(banner: Banner): Promise<Banner> {
      return http.post('/console/v2/api/banners', banner, { headers })
    },
    Update(id: string | number, banner: Banner): Promise<Banner> {
      return http.put(`/console/v2/api/banners/${encodeURIComponent(String(id))}`, banner, { headers })
    },
  }
}
