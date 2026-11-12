/** 与后端模型对齐的领域类型。 */

export interface User {
  id: number
  username: string
  nickname: string
  role: 'user' | 'admin'
  status: 0 | 1 // 1 正常 0 已禁用
  created_at: string
}

export interface UserPage {
  list: User[]
  total: number
  page: number
  page_size: number
}

export interface Category {
  id: number
  name: string
  sort: number
}

export interface SKU {
  id: number
  product_id: number
  size: string
  stock: number
}

export interface Product {
  id: number
  category_id: number
  category_name?: string
  name: string
  age_group: AgeGroup
  material: string
  season: 'spring_summer' | 'autumn_winter'
  description: string
  image: string
  price: number // 分
  status: 0 | 1
  recommended: 0 | 1
  created_at: string
}

export type AgeGroup = 'baby' | 'little' | 'middle' | 'big'

export interface ProductDetail extends Product {
  skus: SKU[]
}

export interface ProductPage {
  list: (Product & { category_name: string })[]
  total: number
  page: number
  page_size: number
}

/** 热销商品:含累计销量(有效订单件数,无销量兜底时为 0)。 */
export interface HotProduct extends Product {
  sold: number
}

export interface HomePayload {
  categories: Category[]
  age_groups: { key: AgeGroup; name: string }[]
  recommended: Product[]
  hot: HotProduct[]
}

export interface CartItem {
  product_id: number
  size: string
  quantity: number
  selected: boolean
  name: string
  image: string
  price: number
  age_group: AgeGroup
  stock: number
  off_shelf: boolean
}

export interface CartSummary {
  count: number
  total_quantity: number
  selected_amount: number
  selected_quantity: number
}

export interface CartView {
  items: CartItem[]
  summary: CartSummary
}

export type OrderStatus =
  | 'pending_payment'
  | 'paid'
  | 'shipped'
  | 'completed'
  | 'canceled'
  | 'refunded'

export interface OrderItem {
  product_id: number
  product_name: string
  image: string
  size: string
  price: number
  quantity: number
}

export interface Order {
  order_no: string
  user_id: number
  status: OrderStatus
  total_amount: number
  pay_amount: number
  receiver: string
  phone: string
  address: string
  remark: string
  pay_time: string | null
  ship_time: string | null
  finish_time: string | null
  cancel_time: string | null
  cancel_reason: string
  refund_time: string | null
  created_at: string
  items?: OrderItem[]
  /** 待支付订单剩余支付秒数(服务端计算;其余状态无此字段) */
  pay_remaining_seconds?: number
}

// ---- 财务中心 ----

/** 资金流水方向:1=进账(支付) 2=出账(退款)。 */
export type FlowDirection = 1 | 2

/** 资金流水(账本,只增不改)。 */
export interface PayFlow {
  flow_no: string
  order_no: string
  user_id: number
  direction: FlowDirection
  biz_type: 'pay' | 'refund'
  amount: number
  status: string
  remark: string
  created_at: string
}

export type RefundStatus = 'pending' | 'refunded' | 'rejected'

/** 退款售后单。 */
export interface RefundOrder {
  refund_no: string
  order_no: string
  user_id: number
  amount: number
  reason: string
  status: RefundStatus
  operator_id: number
  created_at: string
  processed_at: string | null
}

/** 日结账单。 */
export interface DailyStatement {
  stat_date: string
  pay_amount: number
  refund_amount: number
  net_amount: number
  pay_count: number
  refund_count: number
  updated_at: string
}

/** 经营看板指标(金额单位:分)。 */
export interface FinanceDashboard {
  today_pay: number
  today_refund: number
  month_net: number
  pending_refunds: number
}

/** 趋势图单日点位(金额单位:分)。 */
export interface TrendPoint {
  date: string
  pay_amount: number
  refund_amount: number
  net_amount: number
}

/** 对账单侧结果。 */
export interface ReconcileSide {
  order_total: number
  flow_total: number
  diff: number
  order_count: number
  flow_count: number
}

/** 对账差异明细。 */
export interface ReconcileMismatch {
  order_no: string
  order_amount: number
  flow_amount: number
  reason: string
}

/** 一键对账结果。 */
export interface ReconcileResult {
  balanced: boolean
  pay_side: ReconcileSide
  refund_side: ReconcileSide
  mismatches?: ReconcileMismatch[]
  generated_at: string
}

/** 分页包装(财务)。 */
export interface FlowPage {
  list: PayFlow[]
  total: number
  page: number
  page_size: number
}

export interface RefundPage {
  list: RefundOrder[]
  total: number
  page: number
  page_size: number
}

export interface StatementPage {
  list: DailyStatement[]
  total: number
  page: number
  page_size: number
}

/** 收货地址簿条目。 */
export interface Address {
  id: number
  user_id: number
  receiver: string
  phone: string
  province: string
  city: string
  district: string
  detail: string
  is_default: boolean
  created_at: string
  updated_at: string
}

/** 下单请求:省市区结构化(后端组合为地址快照)。 */
export interface PlaceOrderBody {
  receiver: string
  phone: string
  province: string
  city: string
  district: string
  detail: string
  remark?: string
}

export interface OrderPage {
  list: Order[]
  total: number
  page: number
  page_size: number
}

export interface SKUInput {
  size: string
  stock: number
}

export interface ProductInput {
  category_id: number
  name: string
  age_group: AgeGroup
  material: string
  season: 'spring_summer' | 'autumn_winter'
  description: string
  image: string
  price: number
  status: 0 | 1
  recommended: 0 | 1
  skus: SKUInput[]
}
