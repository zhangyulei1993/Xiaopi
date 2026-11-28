export type SessionRole = 'user' | 'admin'

const TOKEN_STORAGE_KEY = 'xiaopi_access_token'
const ROLE_STORAGE_KEY = 'xiaopi_session_role'

export interface Session {
  accessToken: string
  role: SessionRole
}

export const getSession = (): Session | null => {
  const accessToken = String(uni.getStorageSync(TOKEN_STORAGE_KEY) || '')
  if (!accessToken) return null

  const storedRole = uni.getStorageSync(ROLE_STORAGE_KEY)
  const role: SessionRole = storedRole === 'admin' ? 'admin' : 'user'
  return { accessToken, role }
}

export const saveSession = (accessToken: string, role: SessionRole) => {
  uni.setStorageSync(TOKEN_STORAGE_KEY, accessToken)
  uni.setStorageSync(ROLE_STORAGE_KEY, role)
}

export const clearSession = () => {
  uni.removeStorageSync(TOKEN_STORAGE_KEY)
  uni.removeStorageSync(ROLE_STORAGE_KEY)
}

export const hasRole = (role: SessionRole) => getSession()?.role === role

export function sessionAccount(): string {
  const session = getSession()
  if (!session) return ""
  try {
    const raw = session.accessToken.split(".")[0].replace(/-/g, "+").replace(/_/g, "/")
    const bytes = new Uint8Array(uni.base64ToArrayBuffer(raw.padEnd(Math.ceil(raw.length / 4) * 4, "=")))
    const text = decodeURIComponent(Array.from(bytes, byte => "%" + byte.toString(16).padStart(2, "0")).join(""))
    const value = JSON.parse(text).sub
    return typeof value === "string" ? value : ""
  } catch { return "" }
}
