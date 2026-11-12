/** 路由:含 C 端页面与管理端子页面;登录守卫保护交易链路页面。 */
import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { getToken } from '@/api/request'

const routes: RouteRecordRaw[] = [
  { path: '/', name: 'home', component: () => import('@/views/HomeView.vue'), meta: { title: '小鹿童装' } },
  { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { title: '登录' } },
  {
    path: '/product/:id',
    name: 'product-detail',
    component: () => import('@/views/ProductDetailView.vue'),
    meta: { title: '商品详情' },
  },
  { path: '/cart', name: 'cart', component: () => import('@/views/CartView.vue'), meta: { title: '购物车', auth: true } },
  { path: '/checkout', name: 'checkout', component: () => import('@/views/CheckoutView.vue'), meta: { title: '确认订单', auth: true } },
  { path: '/orders', name: 'orders', component: () => import('@/views/OrderListView.vue'), meta: { title: '我的订单', auth: true } },
  {
    path: '/orders/:orderNo',
    name: 'order-detail',
    component: () => import('@/views/OrderDetailView.vue'),
    meta: { title: '订单详情', auth: true },
  },
  {
    path: '/admin',
    component: () => import('@/views/admin/AdminLayout.vue'),
    meta: { auth: true, admin: true, title: '管理后台' },
    redirect: '/admin/products',
    children: [
      { path: 'products', name: 'admin-products', component: () => import('@/views/admin/AdminProducts.vue'), meta: { title: '商品管理' } },
      { path: 'categories', name: 'admin-categories', component: () => import('@/views/admin/AdminCategories.vue'), meta: { title: '分类管理' } },
      { path: 'orders', name: 'admin-orders', component: () => import('@/views/admin/AdminOrders.vue'), meta: { title: '订单管理' } },
      { path: 'users', name: 'admin-users', component: () => import('@/views/admin/AdminUsers.vue'), meta: { title: '用户管理' } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · 小鹿童装` : '小鹿童装'
  if (to.meta.auth && !getToken()) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
})

export default router
