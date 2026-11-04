export type AdminApiOptions = { baseUrl: string; onUnauthorized: () => void }

export function createAdminApi({ baseUrl, onUnauthorized }: AdminApiOptions) {
  return async function api<T>(path: string, options: RequestInit = {}) {
    const token = localStorage.getItem('xiaopi_admin_token')
    const response = await fetch(`${baseUrl}${path}`, {
      ...options,
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers },
    })
    const body = await response.json().catch(() => ({ message: '服务返回异常' }))
    if (response.status === 401 || response.status === 403) {
      localStorage.removeItem('xiaopi_admin_token')
      onUnauthorized()
      throw new Error('登录已过期，请重新登录')
    }
    if (!response.ok || body.code !== 0) throw new Error(body.message || '请求失败')
    return body.data as T
  }
}
