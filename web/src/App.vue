<script setup lang="ts">
/** 全局布局:顶部导航(Logo / 搜索入口 / 购物车角标 / 用户区),移动端自适应。 */
import { onMounted } from 'vue'
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useCartStore } from '@/stores/cart'

const userStore = useUserStore()
const cartStore = useCartStore()
const router = useRouter()

onMounted(() => {
  if (userStore.isLoggedIn) {
    userStore.fetchMe()
    cartStore.fetch().catch(() => {})
  }
})

async function handleLogout() {
  await userStore.logout()
  cartStore.reset()
  router.push('/login')
}
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <div class="topbar-inner">
        <RouterLink to="/" class="brand">
          <span class="brand-deer">🦌</span>
          <span class="brand-name">小鹿童装</span>
        </RouterLink>

        <nav class="top-nav">
          <RouterLink to="/" class="nav-link">首页</RouterLink>
          <RouterLink to="/orders" class="nav-link" v-if="userStore.isLoggedIn">我的订单</RouterLink>
          <RouterLink to="/admin" class="nav-link" v-if="userStore.isAdmin">管理后台</RouterLink>
        </nav>

        <div class="top-actions">
          <RouterLink to="/cart" class="cart-entry" aria-label="购物车">
            <el-badge :value="cartStore.badgeCount" :hidden="cartStore.badgeCount === 0" :max="99">
              <el-icon :size="22"><ShoppingCart /></el-icon>
            </el-badge>
          </RouterLink>
          <template v-if="userStore.isLoggedIn">
            <el-dropdown>
              <span class="user-chip">
                {{ userStore.user?.nickname || userStore.user?.username || '我' }}
                <el-icon><ArrowDown /></el-icon>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="router.push('/orders')">我的订单</el-dropdown-item>
                  <el-dropdown-item divided @click="handleLogout">退出登录</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
          <RouterLink v-else to="/login" class="login-btn">登录 / 注册</RouterLink>
        </div>
      </div>
    </header>

    <main class="page-main">
      <RouterView />
    </main>

    <footer class="page-footer">
      <p>小鹿童装 Fawn Mall · 0-12 岁儿童服装 · 演示项目,支付为模拟流程</p>
    </footer>
  </div>
</template>
