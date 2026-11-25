import { refreshAuth } from "../api/properties";
import { getSession, saveSession, type SessionRole } from "./session";

let refreshInFlight: Promise<{ accessToken: string; role: "admin" | "user" }> | null = null;

/** Refresh the current bearer token before a potentially long-running write flow. */
export const refreshCurrentSession = async (requiredRole?: SessionRole) => {
  const current = getSession();
  if (!current) throw new Error("登录状态已失效，请重新登录后重试");

  if (!refreshInFlight) {
    refreshInFlight = refreshAuth().finally(() => { refreshInFlight = null; });
  }
  const refreshed = await refreshInFlight;
  const role: SessionRole = refreshed.role === "admin" ? "admin" : "user";
  saveSession(refreshed.accessToken, role);
  if (requiredRole && role !== requiredRole) {
    throw new Error("当前账号权限已变化，请重新进入此页面");
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
