/**
 * 清洗后端错误文案：去掉 `code=10008 msg=` / `code=400 msg=...` 这类机器前缀，
 * 只保留人类可读的消息。嵌套前缀会逐层剥离。
 */
export function cleanErrorMessage(raw?: string, fallback = ''): string {
  if (raw === undefined || raw === null) return fallback
  let msg = String(raw).trim()
  const prefixed = /^code=\d+\s+msg=([\s\S]*)$/
  const grpcPrefixed = /^rpc error:\s*code\s*=\s*\w+\s*desc\s*=\s*([\s\S]*)$/i
  for (let i = 0; i < 5; i++) {
    const match = prefixed.exec(msg) ?? grpcPrefixed.exec(msg)
    if (!match) break
    msg = (match[1] ?? '').trim()
  }
  return msg || fallback
}
