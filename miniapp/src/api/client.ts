const API_BASE = 'http://150.158.157.213/api'

export function request<T>(url: string, options: UniApp.RequestOptions = {}) {
  return new Promise<T>((resolve, reject) => {
    uni.request({ ...options, url: `${API_BASE}${url}`, success: (res) => {
      const body = res.data as { code: number; data: T; message?: string }
      if (body.code === 0) resolve(body.data); else reject(new Error(body.message || '请求失败'))
    }, fail: reject })
  })
}
