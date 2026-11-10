// 幂等键生成：mp-weixin 无 crypto.randomUUID，本地随机 v4 兜底（幂等只需碰撞概率可忽略）
export function randomUUID(): string {
  // #ifdef H5
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  // #endif
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}
