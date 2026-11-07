import { createRouter, createWebHashHistory } from 'vue-router'

export const tabRoutes = {
  '运营总览': 'dashboard',
  '房源列表': 'properties',
  '线索管理': 'leads',
  '租客需求': 'rental-requests',
  '用户意向': 'intentions',
  '微信用户': 'wechat-users',
  '小程序基础配置': 'miniapp-basic-config',
  '小程序页面配置': 'miniapp-page-config',
  '管理员列表': 'admins',
  '临时访问码': 'temporary-access',
} as const

export type AdminTab = keyof typeof tabRoutes
const pathToTab = Object.fromEntries(Object.entries(tabRoutes).map(([tab, path]) => [path, tab])) as Record<string, AdminTab>

export function tabFromRoute(value: unknown): AdminTab {
  // `miniapp-config` was the old public hash. Keep it valid without keeping
  // an invisible, third sidebar state in the application.
  if (String(value) === 'miniapp-config') return '小程序页面配置'
  return pathToTab[String(value)] || '运营总览'
}

export function pathForTab(tab: string) {
  return tabRoutes[tab as AdminTab] || tabRoutes['运营总览']
}

export const router = createRouter({
  history: createWebHashHistory('/admin/'),
  routes: [{ path: '/:tab?', name: 'admin', component: { template: '<span />' } }],
})
