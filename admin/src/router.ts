import { createRouter, createWebHashHistory } from 'vue-router'

export const tabRoutes = {
  '运营总览': 'dashboard',
  '房源列表': 'properties',
  '线索管理': 'leads',
  '租客需求': 'rental-requests',
  '用户意向': 'intentions',
  '微信用户': 'wechat-users',
  '小程序配置': 'miniapp-config',
  '管理员列表': 'admins',
  '临时访问码': 'temporary-access',
} as const

export type AdminTab = keyof typeof tabRoutes
const pathToTab = Object.fromEntries(Object.entries(tabRoutes).map(([tab, path]) => [path, tab])) as Record<string, AdminTab>

export function tabFromRoute(value: unknown): AdminTab {
  return pathToTab[String(value)] || '运营总览'
}

export function pathForTab(tab: string) {
  return tabRoutes[tab as AdminTab] || tabRoutes['运营总览']
}

export const router = createRouter({
  history: createWebHashHistory('/admin/'),
  routes: [{ path: '/:tab?', name: 'admin', component: { template: '<span />' } }],
})
