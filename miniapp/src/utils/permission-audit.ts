export function permissionAudit(event: string, source: string, state?: boolean | string | number) {
  if (import.meta.env.MODE !== "test") return;
  const trace = uni.getStorageSync("xiaopi_qa_permission_audit");
  if (!Array.isArray(trace) || trace.length >= 250) return;
  const pages = getCurrentPages();
  trace.push({ at: Date.now(), route: pages[pages.length - 1]?.route, event, source, state });
  uni.setStorageSync("xiaopi_qa_permission_audit", trace);
}
