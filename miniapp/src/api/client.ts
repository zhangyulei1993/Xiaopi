const API_BASE = import.meta.env.VITE_API_BASE || 'http://127.0.0.1:18080/api'
export const API_ORIGIN = API_BASE.endsWith('/api') ? API_BASE.slice(0, -4) : API_BASE

export function request<T>(url: string, options: UniApp.RequestOptions = {}) {
  return new Promise<T>((resolve, reject) => {
    uni.request({ ...options, url: `${API_BASE}${url}`, success: (res) => {
      const body = res.data as { code: number; data: T; message?: string }
      if (body.code === 0) resolve(body.data); else reject(new Error(body.message || '请求失败'))
    }, fail: reject })
  })
}
