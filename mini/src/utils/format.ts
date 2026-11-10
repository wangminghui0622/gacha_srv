import dayjs from 'dayjs'

// 时间格式化：后端时间字段为 unix 秒（createdAt），转 "2006-01-02 15:04:05"
export function formatUnix(sec: number): string {
  if (!sec) return '-'
  return dayjs.unix(sec).format('YYYY-MM-DD HH:mm:ss')
}

// 金额直接展示：后端已为两位小数字符串（如 "1.00"）
export function formatMoney(yuan: string): string {
  return yuan ? `¥${yuan}` : '-'
}
