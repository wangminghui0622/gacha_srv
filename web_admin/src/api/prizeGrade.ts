import { http } from '@/utils/request'

export interface PrizeGrade {
  sort: number
  code: string
  category: string
}

export interface ListPrizeGradesReply {
  items?: PrizeGrade[]
}

export function listPrizeGrades(): Promise<ListPrizeGradesReply> {
  return http.get('/console/v2/api/gacha/prize-grades')
}
