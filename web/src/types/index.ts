/** 与后端模型对齐的领域类型。 */

export interface User {
  id: number
  username: string
  nickname: string
  role: 'user' | 'admin'
  created_at: string
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

export interface HomePayload {
  categories: Category[]
  age_groups: { key: AgeGroup; name: string }[]
  recommended: Product[]
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
  created_at: string
  items?: OrderItem[]
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
