<script setup lang="ts">
/** 管理端布局:侧边导航 + 内容区;进入时校验管理员身份,非管理员跳回首页。 */
import { onMounted, ref } from 'vue'
import { RouterView, RouterLink, useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { toast } from '@/utils/notify'

const userStore = useUserStore()
const route = useRoute()
const router = useRouter()
const checking = ref(true)

onMounted(async () => {
  try {
    await userStore.fetchMe()
    if (!userStore.isAdmin) {
      toast('需要管理员账号登录后访问', 'warning')
      router.replace('/')
    }
  } finally {
    checking.value = false
  }
})

const menuGroups: { title: string; items: { path: string; label: string }[] }[] = [
  {
    title: '',
    items: [
      { path: '/admin/products', label: '商品管理' },
      { path: '/admin/categories', label: '分类管理' },
      { path: '/admin/orders', label: '订单管理' },
      { path: '/admin/users', label: '用户管理' },
    ],
  },
  {
    title: '财务中心',
    items: [
      { path: '/admin/finance/dashboard', label: '经营看板' },
      { path: '/admin/finance/flows', label: '资金流水' },
      { path: '/admin/finance/refunds', label: '退款售后' },
      { path: '/admin/finance/reconcile', label: '对账管理' },
    ],
  },
]
</script>

<template>
  <div v-loading="checking" class="admin">
    <aside class="admin-side">
      <div class="admin-brand">🦌 管理后台</div>
      <nav class="admin-nav">
        <template v-for="g in menuGroups">
          <div v-if="g.title" :key="g.title" class="nav-group">{{ g.title }}</div>
          <RouterLink
            v-for="m in g.items"
            :key="m.path"
            :to="m.path"
            class="admin-link"
            :class="{ active: route.path === m.path }"
          >
            {{ m.label }}
          </RouterLink>
        </template>
      </nav>
      <div class="admin-user">
        {{ userStore.user?.nickname || userStore.user?.username }}(管理员)
      </div>
    </aside>
    <section class="admin-main">
      <RouterView />
    </section>
  </div>
</template>

<style scoped>
.admin {
  display: grid;
  grid-template-columns: 190px 1fr;
  gap: 16px;
  min-height: 60vh;
}

.admin-side {
  background: #fff;
  border: 1px solid var(--fawn-border);
  border-radius: var(--fawn-radius);
  padding: 16px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: fit-content;
}

.admin-brand {
  font-weight: 700;
  padding: 4px 10px 10px;
  border-bottom: 1px solid var(--fawn-border);
}

.admin-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.admin-link {
  text-decoration: none;
  color: var(--fawn-text-sub);
  padding: 9px 12px;
  border-radius: 10px;
  font-size: 14px;
  transition: all 0.15s;
}

.admin-link:hover,
.admin-link.active {
  background: var(--fawn-primary-bg);
  color: var(--fawn-primary);
  font-weight: 600;
}

.nav-group {
  margin-top: 8px;
  padding: 6px 12px 2px;
  font-size: 12px;
  color: var(--fawn-text-sub);
  border-top: 1px dashed var(--fawn-border);
}

.admin-user {
  margin-top: auto;
  padding: 10px 10px 2px;
  border-top: 1px solid var(--fawn-border);
  font-size: 12.5px;
  color: var(--fawn-text-sub);
}

.admin-main {
  min-width: 0;
}

@media (max-width: 720px) {
  .admin {
    grid-template-columns: 1fr;
  }
  .admin-nav {
    flex-direction: row;
    flex-wrap: wrap;
  }
}
</style>
