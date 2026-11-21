import { http } from '@/utils/request'

export interface UploadFileReply {
  objectKey?: string
  url?: string
  size?: number | string
}

export function uploadFileToOss(file: File, dir = 'gacha'): Promise<UploadFileReply> {
  const form = new FormData()
  form.append('file', file)
  form.append('dir', dir)
  return http.post('/console/v2/api/upload', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}
