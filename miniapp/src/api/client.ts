import { permissionAudit } from "../utils/permission-audit"
import { clearSession, getSession } from '../auth/session'
import { safeRequestUrl } from './request-url.mjs'

const API_BASE = import.meta.env.VITE_API_BASE || 'https://api.xiaopizufang.cn/api'
export const API_ORIGIN = API_BASE.endsWith('/api') ? API_BASE.slice(0, -4) : API_BASE

import { validateUploadUrl } from './upload-url.mjs'
import { validateDownloadUrl } from './download-url.mjs'
export const safeUploadUrl = (url: string) => validateUploadUrl(url, import.meta.env.MODE === 'test', API_ORIGIN)

// Property media may be downloaded only from this API or the app's COS domain.
// This keeps admin video downloads from following arbitrary URLs in media data.
export const safeDownloadUrl = (path: string) => validateDownloadUrl(resolveAssetUrl(path), API_ORIGIN)

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

let reauthOpening = false
export const resetReauthentication = () => { reauthOpening = false }
const beginReauthentication = () => {
  if (reauthOpening) return
  const pages = getCurrentPages()
  const current = pages[pages.length - 1] as { route?: string } | undefined
  if (current?.route === 'pages/auth/login') return
  reauthOpening = true
  // navigateTo keeps the page that triggered the request beneath the login
  // page. After a successful login, the existing navigateBack flow resumes it.
  uni.navigateTo({
    url: '/pages/auth/login?reason=expired',
    fail: () => { reauthOpening = false },
  })
}

export function request<T>(url: string, options: RequestOptions = {}) {
  return new Promise<T>((resolve, reject) => {
    const session = getSession()
    const header = { ...options.header } as Record<string, string>
    if (session?.accessToken) header.Authorization = `Bearer ${session.accessToken}`

    const endpoint = url.split("?")[0]
    const audited = import.meta.env.MODE === "test" && /^\/v1\/(?:admin\/|property-leads(?:\/|$)|properties$|footprints(?:\/|$)|rental-requests(?:\/|$)|auth\/profile$)/.test(endpoint)
    if (audited) permissionAudit("api-call", endpoint, options.method || "GET")
    uni.request({
      ...options,
      header,
      timeout: options.timeout || 15000,
      url: safeRequestUrl(url, API_BASE, import.meta.env.MODE === 'test'),
      success: (res) => {
        const body = res.data as { code?: number; data?: T; message?: string }
        if (audited) {
          permissionAudit("api-response", endpoint, res.statusCode)
          permissionAudit("api-data-present", endpoint, body?.data != null)
        }
        if (res.statusCode >= 200 && res.statusCode < 300 && body?.code === 0) {
          resolve(body.data as T)
          return
        }

		if (res.statusCode === 401) {
          const hadSession = Boolean(session?.accessToken)
          clearSession()
          if (hadSession) beginReauthentication()
        } else if (res.statusCode === 404 && url.startsWith('/v1/auth/')) clearSession()
        reject(new ApiError(body?.message || `请求失败（${res.statusCode}）`, res.statusCode, body?.code))
      },
      fail: (error) => reject(new ApiError(error.errMsg || '网络连接失败')),
    } as UniApp.RequestOptions)
  })
}
