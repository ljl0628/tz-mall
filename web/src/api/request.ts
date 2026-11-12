/**
 * 统一请求层:基于原生 fetch 封装(不使用 axios)。
 * - 自动携带 Authorization: Bearer <token>
 * - 统一解析 { code, msg, data } 响应结构
 * - code !== 0 抛出业务错误并全局提示
 * - 401 清除登录态并跳转登录页(携带 redirect)
 */

export interface ApiBody<T = unknown> {
  code: number
  msg: string
  data: T
}

const BASE_URL = '/api/v1'
const TOKEN_KEY = 'fawn_token'

export class ApiError extends Error {
  code: number
  constructor(code: number, msg: string) {
    super(msg)
    this.code = code
  }
}

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  opts: { quiet?: boolean } = {},
): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`

  let resp: Response
  try {
    resp = await fetch(BASE_URL + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    if (!opts.quiet) import('../utils/notify').then(m => m.toast('网络异常,请检查网络后重试', 'error'))
    throw new ApiError(-1, '网络异常')
  }

  // 401:登录态失效 → 清理并跳转登录页
  if (resp.status === 401 || resp.status === 403) {
    clearToken()
    redirectToLogin()
    throw new ApiError(40101, '请先登录')
  }

  let parsed: ApiBody<T>
  try {
    parsed = (await resp.json()) as ApiBody<T>
  } catch {
    if (!opts.quiet) import('../utils/notify').then(m => m.toast('服务响应异常', 'error'))
    throw new ApiError(-1, '服务响应异常')
  }

  if (parsed.code !== 0) {
    // 业务层错误码 40101:token 无效/在黑名单中
    if (parsed.code === 40101) {
      clearToken()
      redirectToLogin()
    }
    if (!opts.quiet) {
      import('../utils/notify').then(m => m.toast(parsed.msg || '操作失败', 'error'))
    }
    throw new ApiError(parsed.code, parsed.msg)
  }
  return parsed.data
}

function redirectToLogin() {
  const { pathname, search } = window.location
  if (pathname !== '/login') {
    const redirect = encodeURIComponent(pathname + search)
    window.location.href = `/login?redirect=${redirect}`
  }
}

export const http = {
  get: <T>(path: string, opts?: { quiet?: boolean }) => request<T>('GET', path, undefined, opts),
  post: <T>(path: string, body?: unknown, opts?: { quiet?: boolean }) =>
    request<T>('POST', path, body, opts),
  put: <T>(path: string, body?: unknown, opts?: { quiet?: boolean }) =>
    request<T>('PUT', path, body, opts),
  delete: <T>(path: string, body?: unknown, opts?: { quiet?: boolean }) =>
    request<T>('DELETE', path, body, opts),
}
