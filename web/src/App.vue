<script setup lang="ts">
/** 全局布局:顶部导航(Logo / 购物车角标 / 用户区),移动端导航收进抽屉菜单。 */
import { ref, onMounted } from 'vue'
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useCartStore } from '@/stores/cart'

const userStore = useUserStore()
const cartStore = useCartStore()
const router = useRouter()
const menuOpen = ref(false) // 移动端抽屉菜单

onMounted(() => {
  if (userStore.isLoggedIn) {
    userStore.fetchMe()
    cartStore.fetch().catch(() => {})
  }
})

async function handleLogout() {
  menuOpen.value = false
  await userStore.logout()
  cartStore.reset()
  router.push('/login')
}
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <div class="topbar-inner">
        <!-- 移动端:汉堡菜单按钮(桌面隐藏) -->
        <button class="menu-btn" aria-label="菜单" @click="menuOpen = true">
          <el-icon :size="22"><Menu /></el-icon>
        </button>

        <RouterLink to="/" class="brand">
          <span class="brand-deer">🦌</span>
          <span class="brand-name">小鹿童装</span>
        </RouterLink>

        <nav class="top-nav">
          <RouterLink to="/" class="nav-link">首页</RouterLink>
          <RouterLink to="/categories" class="nav-link">商品分类</RouterLink>
          <RouterLink to="/orders" class="nav-link" v-if="userStore.isLoggedIn">我的订单</RouterLink>
          <RouterLink to="/addresses" class="nav-link" v-if="userStore.isLoggedIn">收货地址</RouterLink>
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
                  <el-dropdown-item @click="router.push('/addresses')">收货地址</el-dropdown-item>
                  <el-dropdown-item divided @click="handleLogout">退出登录</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
          <RouterLink v-else to="/login" class="login-btn">登录 / 注册</RouterLink>
        </div>
      </div>
    </header>

    <!-- 移动端抽屉菜单:竖排导航,点击跳转即关闭 -->
    <el-drawer v-model="menuOpen" direction="ttb" size="auto" :with-header="false" class="nav-drawer">
      <nav class="drawer-nav">
        <RouterLink to="/" class="drawer-link" @click="menuOpen = false">
          <el-icon><HomeFilled /></el-icon>首页
          <el-icon class="drawer-arrow"><ArrowRight /></el-icon>
        </RouterLink>
        <RouterLink to="/categories" class="drawer-link" @click="menuOpen = false">
          <el-icon><Goods /></el-icon>商品分类
          <el-icon class="drawer-arrow"><ArrowRight /></el-icon>
        </RouterLink>
        <RouterLink v-if="userStore.isLoggedIn" to="/orders" class="drawer-link" @click="menuOpen = false">
          <el-icon><Tickets /></el-icon>我的订单
          <el-icon class="drawer-arrow"><ArrowRight /></el-icon>
        </RouterLink>
        <RouterLink v-if="userStore.isLoggedIn" to="/addresses" class="drawer-link" @click="menuOpen = false">
          <el-icon><Location /></el-icon>收货地址
          <el-icon class="drawer-arrow"><ArrowRight /></el-icon>
        </RouterLink>
        <RouterLink v-if="userStore.isAdmin" to="/admin" class="drawer-link" @click="menuOpen = false">
          <el-icon><Setting /></el-icon>管理后台
          <el-icon class="drawer-arrow"><ArrowRight /></el-icon>
        </RouterLink>
        <a v-if="userStore.isLoggedIn" class="drawer-link drawer-logout" @click="handleLogout">
          <el-icon><SwitchButton /></el-icon>退出登录
        </a>
        <RouterLink v-else to="/login" class="drawer-link drawer-login" @click="menuOpen = false">
          <el-icon><User /></el-icon>登录 / 注册
        </RouterLink>
      </nav>
    </el-drawer>

    <main class="page-main">
      <RouterView />
    </main>

    <footer class="page-footer">
      <p>小鹿童装 Fawn Mall · 0-12 岁儿童服装 · 演示项目,支付为模拟流程</p>
    </footer>
  </div>
</template>
