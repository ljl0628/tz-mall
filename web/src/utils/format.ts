/** 展示格式化工具。 */
import type { AgeGroup, OrderStatus } from '@/types'

/** 分 → 元(保留两位小数)。 */
export function fen2yuan(fen: number): string {
  return (fen / 100).toFixed(2)
}

export const AGE_GROUP_NAME: Record<AgeGroup, string> = {
  baby: '婴幼儿 0-3岁',
  little: '小童 3-6岁',
  middle: '中童 6-9岁',
  big: '大童 9-12岁',
}

export const AGE_GROUP_SHORT: Record<AgeGroup, string> = {
  baby: '0-3岁',
  little: '3-6岁',
  middle: '6-9岁',
  big: '9-12岁',
}

export const ORDER_STATUS_NAME: Record<OrderStatus, string> = {
  pending_payment: '待支付',
  paid: '已支付',
  shipped: '已发货',
  completed: '已完成',
  canceled: '已取消',
}

export const ORDER_STATUS_TAG: Record<OrderStatus, 'warning' | 'primary' | 'success' | 'info' | 'danger'> = {
  pending_payment: 'warning',
  paid: 'primary',
  shipped: 'success',
  completed: 'success',
  canceled: 'danger',
}

export const SEASON_NAME: Record<string, string> = {
  spring_summer: '春夏',
  autumn_winter: '秋冬',
}

export function formatTime(t: string | null | undefined): string {
  if (!t) return '-'
  return t.replace('T', ' ').replace(/\+08:00$/, '').slice(0, 19)
}
