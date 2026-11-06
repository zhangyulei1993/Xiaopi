import type { PagePresentation } from '../types/domain'

export const pagePresentationOptions = [
  { key: 'home', label: '首页', audience: 'customer' }, { key: 'properties', label: '房源列表', audience: 'customer' }, { key: 'propertyDetail', label: '房源详情', audience: 'customer' }, { key: 'requests', label: '找房需求', audience: 'customer' }, { key: 'publish', label: '房源线索', audience: 'customer' }, { key: 'me', label: '我的', audience: 'customer' }, { key: 'footprints', label: '浏览足迹', audience: 'customer' }, { key: 'authLogin', label: '微信登录', audience: 'customer' }, { key: 'adminWorkbench', label: '运营工作台', audience: 'operator' }, { key: 'adminPropertyEdit', label: '录入与编辑房源', audience: 'operator' },
]

export const channelStyleOptions = [
  { value: 'commute', label: '通勤' }, { value: 'campus', label: '校园' }, { value: 'work', label: '职场' }, { value: 'life', label: '生活' },
]

export const defaultPagePresentations = (): Record<string, PagePresentation> => ({
  home: { title: '今天想住哪里？', subtitle: '搜索小区、地铁或商圈', backgroundImage: '' }, properties: { title: '找一处合适的家', subtitle: '按地区和预算筛选', backgroundImage: '' }, propertyDetail: { title: '房源详情', subtitle: '真实房源，放心查看', backgroundImage: '' }, me: { title: '我的', subtitle: '管理关注与找房需求', backgroundImage: '' }, footprints: { title: '浏览足迹', subtitle: '最近看过的房源', backgroundImage: '' }, requests: { title: '小皮帮你找房', subtitle: '告诉我们你的找房需求', backgroundImage: '' }, publish: { title: '发布房源', subtitle: '提交真实房源线索', backgroundImage: '' }, adminWorkbench: { title: '小皮工作台', subtitle: '房源运营与数据处理', backgroundImage: '' }, adminPropertyEdit: { title: '编辑房源', subtitle: '完善房源信息后发布', backgroundImage: '' }, authLogin: { title: '微信登录', subtitle: '登录后管理你的找房服务', backgroundImage: '' },
})

export const normalizeChannelStyle = (value: string, index: number) => {
  const legacy: Record<string, string> = { '🏢': 'work', '🌳': 'campus', '🌆': 'life', '🏠': 'life' }
  return legacy[value] || (channelStyleOptions.some((item) => item.value === value) ? value : channelStyleOptions[index % channelStyleOptions.length].value)
}
