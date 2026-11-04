import { clearSession, getSession } from '../auth/session'

const API_BASE = import.meta.env.VITE_API_BASE || 'https://api.xiaopizufang.cn/api'
export const API_ORIGIN = API_BASE.endsWith('/api') ? API_BASE.slice(0, -4) : API_BASE

export const resolveAssetUrl = (path = '') => {
  const value = String(path).trim()
  if (!value) return ''

  if (value.startsWith('/')) return `${API_ORIGIN}${value}`

  if (/^https?:\/\//i.test(value)) {
    const secure = value.replace(/^http:\/\//i, 'https://')
    return secure.replace(/^https:\/\/150\.158\.157\.213(?=\/|$)/i, API_ORIGIN)
  }

  return `${API_ORIGIN}/${value.replace(/^\/+/, '')}`
}

export type RequestOptions = Omit<UniApp.RequestOptions, 'url' | 'method'> & {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
}

export class ApiError extends Error {
  constructor(message: string, public statusCode = 0, public code = 0) {
    super(message)
  }
}

export function request<T>(url: string, options: RequestOptions = {}) {
  return new Promise<T>((resolve, reject) => {
    const session = getSession()
    const header = { ...options.header } as Record<string, string>
    if (session?.accessToken) header.Authorization = `Bearer ${session.accessToken}`

    uni.request({
      ...options,
      header,
      timeout: options.timeout || 15000,
      url: `${API_BASE}${url}`,
      success: (res) => {
        const body = res.data as { code?: number; data?: T; message?: string }
        if (res.statusCode >= 200 && res.statusCode < 300 && body?.code === 0) {
          resolve(body.data as T)
          return
        }

        if (res.statusCode === 401) clearSession()
        reject(new ApiError(body?.message || `请求失败（${res.statusCode}）`, res.statusCode, body?.code))
      },
      fail: (error) => reject(new ApiError(error.errMsg || '网络连接失败')),
    } as UniApp.RequestOptions)
  })
}
