<script setup lang="ts">
/** 登录/注册:同一页双 Tab,登录成功跳回 redirect 或首页。 */
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { authApi } from '@/api'
import { useUserStore } from '@/stores/user'
import { useCartStore } from '@/stores/cart'
import { toast } from '@/utils/notify'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const cartStore = useCartStore()

const tab = ref<'login' | 'register'>('login')
const loading = ref(false)

const loginForm = ref({ username: '', password: '' })
const regForm = ref({ username: '', password: '', confirm: '', nickname: '' })

onMounted(() => {
  if (route.query.tab === 'register') tab.value = 'register'
})

async function handleLogin() {
  if (!loginForm.value.username || !loginForm.value.password) {
    toast('请输入用户名和密码', 'warning')
    return
  }
  loading.value = true
  try {
    const { token, user } = await authApi.login(loginForm.value)
    userStore.applyLogin(token, user)
    cartStore.fetch().catch(() => {})
    toast(`欢迎回来,${user.nickname || user.username}`)
    const redirect = route.query.redirect
    router.push(typeof redirect === 'string' && redirect ? redirect : '/')
  } finally {
    loading.value = false
  }
}

async function handleRegister() {
  const f = regForm.value
  // 与后端 binding:"min=3,max=32,alphanum" 保持一致,前置拦截给出友好提示
  if (!/^[a-zA-Z0-9]{3,32}$/.test(f.username)) {
    toast('用户名仅支持 3-32 位字母或数字', 'warning')
    return
  }
  if (f.password.length < 6) {
    toast('密码至少 6 位', 'warning')
    return
  }
  if (f.password !== f.confirm) {
    toast('两次输入的密码不一致', 'warning')
    return
  }
  loading.value = true
  try {
    const { token, user } = await authApi.register({
      username: f.username,
      password: f.password,
      nickname: f.nickname || undefined,
    })
    userStore.applyLogin(token, user)
    cartStore.fetch().catch(() => {})
    toast('注册成功,欢迎加入小鹿童装')
    router.push('/')
  } finally {
    loading.value = false
  }
}

/** 快速填入演示账号。 */
function fillDemo() {
  tab.value = 'login'
  loginForm.value = { username: 'demo', password: 'demo123' }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card card">
      <div class="login-brand">
        <span class="deer">🦌</span>
        <h2>小鹿童装</h2>
        <p>宝爸宝妈的放心童装店</p>
      </div>

      <el-tabs v-model="tab" stretch>
        <el-tab-pane label="登录" name="login">
          <el-form @submit.prevent>
            <el-form-item>
              <el-input v-model="loginForm.username" placeholder="用户名" size="large" @keyup.enter="handleLogin">
                <template #prefix><el-icon><User /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item>
              <el-input
                v-model="loginForm.password"
                type="password"
                placeholder="密码"
                size="large"
                show-password
                @keyup.enter="handleLogin"
              >
                <template #prefix><el-icon><Lock /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-button
              type="primary"
              size="large"
              class="submit-btn"
              :loading="loading"
              @click="handleLogin"
            >
              登 录
            </el-button>
            <div class="demo-tip">
              演示账号:<el-link type="primary" :underline="false" @click="fillDemo">demo / demo123</el-link>
              (管理员 admin / admin123)
            </div>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="注册" name="register">
          <el-form @submit.prevent>
            <el-form-item>
              <el-input v-model="regForm.username" placeholder="用户名(3-32 位字母数字)" size="large">
                <template #prefix><el-icon><User /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item>
              <el-input v-model="regForm.nickname" placeholder="昵称(选填,如:果果妈妈)" size="large">
                <template #prefix><el-icon><Avatar /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item>
              <el-input v-model="regForm.password" type="password" placeholder="密码(至少 6 位)" size="large" show-password>
                <template #prefix><el-icon><Lock /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item>
              <el-input v-model="regForm.confirm" type="password" placeholder="确认密码" size="large" show-password @keyup.enter="handleRegister">
                <template #prefix><el-icon><Lock /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-button
              type="primary"
              size="large"
              class="submit-btn"
              :loading="loading"
              @click="handleRegister"
            >
              注 册
            </el-button>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  justify-content: center;
  padding-top: 6vh;
}

.login-card {
  width: 100%;
  max-width: 400px;
  padding: 26px 26px 18px;
}

.login-brand {
  text-align: center;
  margin-bottom: 8px;
}

.login-brand .deer {
  font-size: 44px;
}

.login-brand h2 {
  margin: 6px 0 4px;
  letter-spacing: 2px;
}

.login-brand p {
  margin: 0 0 10px;
  color: var(--fawn-text-sub);
  font-size: 13px;
}

.submit-btn {
  width: 100%;
  border-radius: 999px;
  font-weight: 600;
  letter-spacing: 4px;
}

.demo-tip {
  margin-top: 14px;
  text-align: center;
  font-size: 12.5px;
  color: var(--fawn-text-sub);
}
</style>
