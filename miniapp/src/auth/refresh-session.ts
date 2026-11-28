import { refreshAuth } from "../api/properties";
import { getSession, saveSession, type SessionRole } from "./session";

export class SessionPermissionChanged extends Error {}

type RefreshResult = Awaited<ReturnType<typeof refreshAuth>>;
const refreshInFlight = new Map<string, Promise<RefreshResult>>();
const changedSession = () => new Error("登录账号已变化，请重新进入此页面");

/** Refresh the current bearer token before a potentially long-running write flow. */
export const refreshCurrentSession = async (requiredRole?: SessionRole) => {
  const current = getSession();
  if (!current) throw new Error("登录状态已失效，请重新登录后重试");

  const token = current.accessToken;
  let pending = refreshInFlight.get(token);
  if (!pending) {
    pending = refreshAuth().then(refreshed => {
      if (getSession()?.accessToken !== token) throw changedSession();
      const role: SessionRole = refreshed.role === "admin" ? "admin" : "user";
      saveSession(refreshed.accessToken, role);
      return { ...refreshed, role };
    }).finally(() => { refreshInFlight.delete(token); });
    refreshInFlight.set(token, pending);
  }
  const refreshed = await pending;
  if (getSession()?.accessToken !== refreshed.accessToken) throw changedSession();
  const role = refreshed.role;
  if (requiredRole && role !== requiredRole) {
    throw new SessionPermissionChanged("当前账号权限已变化，请重新进入此页面");
  }
  return refreshed;
};

/** Keep a long-running admin editor authenticated while it remains visible. */
export const startSessionKeepAlive = (requiredRole: SessionRole, intervalMs = 10 * 60 * 1000) => {
  const refresh = () => { void refreshCurrentSession(requiredRole).catch(() => undefined); };
  refresh();
  const timer = setInterval(refresh, intervalMs);
  return () => clearInterval(timer);
};
