/** 用户状态:token + 用户信息,持久化到 localStorage。 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api'
import { getToken, setToken, clearToken } from '@/api/request'
import type { User } from '@/types'

export const useUserStore = defineStore('user', () => {
  const token = ref<string>(getToken())
  const user = ref<User | null>(null)

  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function fetchMe() {
    if (!token.value) return
    try {
      user.value = await authApi.me()
    } catch {
      // token 失效时 request 层已负责清理并跳转
    }
  }

  function applyLogin(newToken: string, u: User) {
    token.value = newToken
    user.value = u
    setToken(newToken)
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch {
      // 后端登出失败不阻断本地清理
    }
    token.value = ''
    user.value = null
    clearToken()
  }

  /** 本地清理(401 场景 request 层已调用 clearToken)。 */
  function reset() {
    token.value = ''
    user.value = null
    clearToken()
  }

  return { token, user, isLoggedIn, isAdmin, fetchMe, applyLogin, logout, reset }
})
