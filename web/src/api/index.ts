/** API 模块:按领域拆分的接口封装。 */
import { http, httpUpload } from './request'
import type {
  CartView,
  Category,
  HomePayload,
  Order,
  OrderPage,
  ProductDetail,
  ProductInput,
  ProductPage,
  User,
  UserPage,
} from '@/types'

// ---- 认证 ----
export interface LoginResp {
  token: string
  user: User
}

export const authApi = {
  register: (body: { username: string; password: string; nickname?: string }) =>
    http.post<LoginResp>('/auth/register', body),
  login: (body: { username: string; password: string }) =>
    http.post<LoginResp>('/auth/login', body),
  logout: () => http.post<{ logged_out: boolean }>('/auth/logout'),
  me: () => http.get<User>('/user/me'),
}

// ---- 商品 ----
export interface ProductQuery {
  page?: number
  page_size?: number
  category_id?: number
  age_group?: string
  keyword?: string
}

export const productApi = {
  home: () => http.get<HomePayload>('/home'),
  categories: () => http.get<Category[]>('/categories'),
  list: (q: ProductQuery) => {
    const p = new URLSearchParams()
    Object.entries(q).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') p.set(k, String(v))
    })
    return http.get<ProductPage>(`/products?${p.toString()}`)
  },
  detail: (id: number) => http.get<ProductDetail>(`/products/${id}`),
}

// ---- 购物车 ----
export const cartApi = {
  get: () => http.get<CartView>('/cart'),
  add: (body: { product_id: number; size: string; quantity: number }) =>
    http.post<{ added: boolean }>('/cart/items', body),
  update: (field: string, body: { quantity?: number; selected?: boolean }) =>
    http.put<{ updated: boolean }>(`/cart/items/${encodeURIComponent(field)}`, body),
  remove: (field: string) =>
    http.delete<{ removed: boolean }>(`/cart/items/${encodeURIComponent(field)}`),
  selectAll: (selected: boolean) => http.put<{ selected: boolean }>('/cart/select-all', { selected }),
}

// ---- 订单 ----
export const orderApi = {
  place: (body: { receiver: string; phone: string; address: string; remark?: string }) =>
    http.post<{ order_no: string; total_amount: number }>('/orders', body),
  list: (q: { page?: number; page_size?: number; status?: string }) => {
    const p = new URLSearchParams()
    Object.entries(q).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') p.set(k, String(v))
    })
    return http.get<OrderPage>(`/orders?${p.toString()}`)
  },
  detail: (orderNo: string) => http.get<Order>(`/orders/${orderNo}`),
  pay: (orderNo: string) => http.post<Order>(`/orders/${orderNo}/pay`),
  cancel: (orderNo: string) => http.post<Order>(`/orders/${orderNo}/cancel`),
  confirm: (orderNo: string) => http.post<Order>(`/orders/${orderNo}/confirm`),
}

// ---- 管理端 ----
export interface AdminProductQuery extends ProductQuery {
  status?: number | string
}

export const adminApi = {
  /** 上传商品图到七牛(服务端中转),返回 { url, key, content_type, size } */
  uploadImage: (file: File) =>
    httpUpload.post<{ url: string; key: string; content_type: string; size: number }>(
      '/admin/upload/image',
      file,
    ),
  listProducts: (q: AdminProductQuery) => {
    const p = new URLSearchParams()
    Object.entries(q).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') p.set(k, String(v))
    })
    return http.get<ProductPage>(`/admin/products?${p.toString()}`)
  },
  createProduct: (body: ProductInput) => http.post<ProductDetail>('/admin/products', body),
  updateProduct: (id: number, body: ProductInput) =>
    http.put<{ updated: boolean }>(`/admin/products/${id}`, body),
  deleteProduct: (id: number) => http.delete<{ deleted: boolean }>(`/admin/products/${id}`),
  updateSKUStock: (skuID: number, stock: number) =>
    http.put<{ updated: boolean }>(`/admin/skus/${skuID}`, { stock }),

  listCategories: () => http.get<Category[]>('/admin/categories'),
  createCategory: (body: { name: string; sort: number }) => http.post<Category>('/admin/categories', body),
  updateCategory: (id: number, body: { name: string; sort: number }) =>
    http.put<{ updated: boolean }>(`/admin/categories/${id}`, body),
  deleteCategory: (id: number) => http.delete<{ deleted: boolean }>(`/admin/categories/${id}`),

  listOrders: (q: { page?: number; page_size?: number; status?: string }) => {
    const p = new URLSearchParams()
    Object.entries(q).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') p.set(k, String(v))
    })
    return http.get<OrderPage>(`/admin/orders?${p.toString()}`)
  },
  ship: (orderNo: string) => http.put<{ shipped: boolean }>(`/admin/orders/${orderNo}/ship`),

  listUsers: (q: { page?: number; page_size?: number; keyword?: string; role?: string; status?: number | string }) => {
    const p = new URLSearchParams()
    Object.entries(q).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') p.set(k, String(v))
    })
    return http.get<UserPage>(`/admin/users?${p.toString()}`)
  },
  setUserStatus: (id: number, status: 0 | 1) =>
    http.put<{ updated: boolean }>(`/admin/users/${id}/status`, { status }),
  setUserRole: (id: number, role: 'user' | 'admin') =>
    http.put<{ updated: boolean }>(`/admin/users/${id}/role`, { role }),
  resetUserPassword: (id: number, password: string) =>
    http.put<{ reset: boolean }>(`/admin/users/${id}/password`, { password }),
}
