// 券类型工具（M10）：kind 1 FIXED_MINOR（value=分）/ 2 PERCENT_BPS（value=万分比）/ 3 FREE（免单）

// 面值文案：满减立减金额 / 折扣抵扣百分比 / 免单（文案经 i18n t 包裹）
export function couponValueText(
  kind: number,
  value: number,
  t: (key: string, options?: Record<string, unknown>) => string,
): string {
  if (kind === 1) return t('coupon.valueFixed', { value: `¥${(value / 100).toFixed(2)}` })
  if (kind === 2) return t('coupon.valuePercent', { value: (value / 100).toFixed(0) })
  return t('coupon.valueFree')
}
