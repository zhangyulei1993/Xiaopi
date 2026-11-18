export type DetailSectionTab = 'overview' | 'location' | 'facilities' | 'description' | 'nearby'
export type BasicConfigTab = 'brand' | 'tags' | 'facilities'
export const basicConfigSections: { key: BasicConfigTab; label: string }[] = [
  { key: 'brand', label: '品牌与联系' },
  { key: 'tags', label: '房源标签' },
  { key: 'facilities', label: '房屋设施' },
]

type CanvasNode = { label: string; kind: '标题文本' | '数据标签' | '数据规则' | '页面结构'; audience: 'customer' | 'operator' | 'both'; impacts: string; editable: string; locked: string; tab: 'basic' | 'page'; field: string; basicTab?: BasicConfigTab; page?: string; detail?: DetailSectionTab }
export const canvasNodes: Record<string, CanvasNode> = {
  'home.title': { label: '首页主标题', kind: '标题文本', audience: 'both', impacts: '首页首屏', editable: '标题文字', locked: '页面结构与找房逻辑', tab: 'page', page: 'home', field: 'home-title' },
  'home.channelTitle': { label: '频道标题', kind: '标题文本', audience: 'both', impacts: '首页', editable: '显示文案', locked: '入口行为', tab: 'page', page: 'home', field: 'home-channelTitle' },
  'home.recommendationTitle': { label: '推荐房源标题', kind: '标题文本', audience: 'both', impacts: '首页', editable: '显示文案', locked: '入口行为', tab: 'page', page: 'home', field: 'home-recommendationTitle' },
  'page.heading': { label: '页面标题与辅助文本', kind: '标题文本', audience: 'both', impacts: '当前所选小程序页面', editable: '页面标题、辅助文本与背景图', locked: '页面功能与业务流程', tab: 'page', field: 'page-copy' },
  'brand': { label: '品牌名称', kind: '标题文本', audience: 'both', impacts: '导航、登录、详情服务与分享落款', editable: '对外品牌名称', locked: '客服权限与业务数据', tab: 'basic', basicTab: 'brand', field: 'brand-name' },
  'status': { label: '房源状态标签', kind: '数据标签', audience: 'both', impacts: '管理员房源管理；客户仅显示可租', editable: '各状态显示名称', locked: '状态编码、流转和客户可见规则', tab: 'page', page: 'properties', field: 'status-labels' },
  'coverage': { label: '地区与关联范围', kind: '数据规则', audience: 'both', impacts: '首页地区、频道筛选与附近推荐', editable: '地区名称、频道、地图中心和半径', locked: '自动生成的内部 ID', tab: 'page', page: 'home', field: 'coverage' },
  'facilities': { label: '房屋设施库', kind: '数据标签', audience: 'both', impacts: '房源录入与详情设施展示', editable: '设施名称、分类与启用状态', locked: '已录入房源的历史数据', tab: 'basic', basicTab: 'facilities', field: 'facilities' },
  'tags': { label: '房源标签规则', kind: '页面结构', audience: 'both', impacts: '首页、列表、详情与微信分享', editable: '客户可见性、顺序与排版', locked: '房源真实字段内容', tab: 'basic', basicTab: 'tags', field: 'tags' },
  'detail.overview': { label: '固定首屏摘要', kind: '页面结构', audience: 'both', impacts: '房源详情打开后的首屏成交信息', editable: '详情标签的可见性与顺序', locked: '月租、租赁身份、核心参数与客户端视觉结构', tab: 'page', page: 'propertyDetail', detail: 'overview', field: 'detail-overview' },
  'detail.location': { label: '位置与周边', kind: '页面结构', audience: 'both', impacts: '房源详情位置区', editable: '模块标题与启用状态', locked: '房源保存的坐标', tab: 'page', page: 'propertyDetail', detail: 'location', field: 'detail-location' },
  'detail.facilities': { label: '房屋设施', kind: '数据标签', audience: 'both', impacts: '房源详情设施区与录入选项', editable: '模块标题和设施库内容', locked: '历史房源已保存的设施数据', tab: 'page', page: 'propertyDetail', detail: 'facilities', field: 'detail-facilities' },
  'detail.description': { label: '房源简介', kind: '页面结构', audience: 'both', impacts: '房源详情补充说明区', editable: '模块标题与启用状态', locked: '每套房源的真实描述', tab: 'page', page: 'propertyDetail', detail: 'description', field: 'detail-description' },
  'detail.nearby': { label: '附近房源', kind: '数据规则', audience: 'customer', impacts: '房源详情推荐区', editable: '标题、范围、关联逻辑、排序与数量', locked: '房源真实位置', tab: 'page', page: 'propertyDetail', detail: 'nearby', field: 'detail-nearby' },
}
