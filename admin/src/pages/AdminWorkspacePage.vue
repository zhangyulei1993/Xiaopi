<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, ArrowUp, Delete, House, Plus, RefreshLeft } from '@element-plus/icons-vue'

import homeHeroFallback from '../../../miniapp/src/static/home-lifestyle-hero.jpg'
import propertyPlaceholderIcon from '../../../miniapp/src/static/icons/property-placeholder.svg'
import homeTabIcon from '../../../miniapp/src/static/tabbar/home.png'
import homeTabActiveIcon from '../../../miniapp/src/static/tabbar/home-active.png'
import propertyTabIcon from '../../../miniapp/src/static/tabbar/house.png'
import propertyTabActiveIcon from '../../../miniapp/src/static/tabbar/house-active.png'
import meTabIcon from '../../../miniapp/src/static/tabbar/me.png'
import meTabActiveIcon from '../../../miniapp/src/static/tabbar/me-active.png'
import AdminLayout from '../layouts/AdminLayout.vue'
import { isPendingLead, isNewRentalRequest } from '../config/operationalMetrics'
import DashboardPage from './DashboardPage.vue'
import WechatUsersPage from './WechatUsersPage.vue'
import IntentionsPage from './IntentionsPage.vue'
import PropertiesPage from './PropertiesPage.vue'
import RentalRequestsPage from './RentalRequestsPage.vue'
import LeadsPage from './LeadsPage.vue'
import { pathForTab, tabFromRoute } from '../router'
import type { AdminUser, ChannelConfig, FavoriteContact, Lead, LocationSearchItem, PagePresentation, Property, PublicConfig, RentalRequest, TemporaryAccess, WechatUser } from '../types/domain'
import { channelStyleOptions, defaultPagePresentations, normalizeChannelStyle, pagePresentationOptions } from '../config/navigation'
import { createAdminApi } from '../services/api'
type TabKey = '运营总览' | '房源列表' | '线索管理' | '租客需求' | '用户意向' | '微信用户' | '小程序基础配置' | '小程序页面配置' | '后台管理员' | '小程序管理员' | '临时访问码'

const apiBase = import.meta.env.VITE_API_BASE || 'https://api.xiaopizufang.cn/api'
const loggedIn = ref(Boolean(localStorage.getItem('xiaopi_admin_token')))
const route = useRoute()
const router = useRouter()
const tab = ref<TabKey>(tabFromRoute(route.params.tab) as TabKey)
type WebAccount = { username: string; enabled: boolean; super: boolean; mustChangePassword: boolean }
const webAccounts = ref<WebAccount[]>([])
const webAdminSuper = ref(sessionStorage.getItem('xiaopi_web_admin_super') === 'true')
const mustChangePassword = ref(false)
const newWebPassword = ref('')
const webAccountName = ref('')
const webAccountPassword = ref('')
const webAccountMessage = ref('')
const webAccountDialog = ref<'create' | 'reset' | 'self' | ''>('')
const webAccountTarget = ref<WebAccount | null>(null)

function openWebAccountDialog(mode: 'create' | 'reset' | 'self', account?: WebAccount) {
  webAccountName.value = ''
  webAccountPassword.value = ''
  newWebPassword.value = ''
  webAccountMessage.value = ''
  webAccountTarget.value = account ?? null
  webAccountDialog.value = mode
}

const webAccountBusy = ref(false)
const username = ref('admin')
const password = ref('')
const loginError = ref('')
const loading = ref(false)
const pageError = ref('')
const savingConfig = ref(false)
const configMessage = ref('')
const configFailed = ref(false)
const savedConfigJSON = ref('')
const configLastSavedAt = ref('')
const configPage = ref('home')
const previewOverlay = ref<'page' | 'share'>('page')
const tagMagnifierOpen = ref(false)
type ConfigAudience = 'customer' | 'operator'
type DetailConfigTab = 'detail' | 'share'
type DetailSectionTab = 'overview' | 'location' | 'facilities' | 'description' | 'nearby'
const detailModuleOptions: { key: DetailSectionTab; label: string; helper: string; fixed?: boolean }[] = [
  { key: 'overview', label: '固定首屏摘要', helper: '租赁信息、列表标签与房屋核心参数', fixed: true },
  { key: 'location', label: '位置与周边', helper: '位置、地图与通勤' },
  { key: 'facilities', label: '房屋设施', helper: '家具家电等可勾选配置' },
  { key: 'description', label: '房源简介', helper: '补充说明与居住规则' },
  { key: 'nearby', label: '附近推荐', helper: '同区域可租房源' },
]
const configAudience = ref<ConfigAudience>('customer')
type CanvasDataState = 'normal' | 'loading' | 'error' | 'empty'
const canvasIdentityState = ref<'guest' | 'user' | 'admin'>('user')
const canvasEditStep = ref(0)
const canvasPageHistory = ref<string[]>([])
const canvasSearchQuery = ref('')
const canvasAreaFilter = ref('全部')
const canvasChannelFilter = ref('')
const canvasRentTypeFilter = ref('全部')
const canvasBudgetFilter = ref('不限')
const canvasStatusFilter = ref('all')
const canvasFavoriteOnly = ref(false)
const canvasPropertyFiltersOpen = ref(false)
const canvasPreviewFavorites = ref<string[]>([])
const canvasMediaIndex = ref(0)
const canvasPreviewStage = ref<HTMLElement | null>(null)
const canvasPreviewPhone = ref<HTMLElement | null>(null)
const canvasFitScale = ref(1)
let canvasFitObserver: ResizeObserver | undefined
const canvasAdminWorkbenchTab = ref<'properties' | 'leads' | 'requests' | 'interests'>('properties')
const canvasNotice = ref('')
const canvasForm = reactive({ contactName: '', phone: '', location: '', layout: '', budgetMin: '', budgetMax: '', moveInDate: '', preferredTime: '', note: '', sourceName: '', sourcePhone: '', community: '', expectedRent: '' })
const canvasDetailModal = ref<'appointment' | 'map' | ''>('')
const canvasPropertyDraft = reactive<Record<string, string>>({})
const canvasLoginMode = ref<'wechat' | 'accessCode'>('wechat')
const canvasLoginPhone = ref('')
const canvasLoginAccessCode = ref('')
const canvasPropertyFacilities = ref<string[]>([])
const canvasEditingExistingProperty = ref(false)
const detailConfigTab = ref<DetailConfigTab>('detail')
const detailSectionTab = ref<DetailSectionTab>('overview')
type BasicConfigTab = 'brand' | 'coverage' | 'tags' | 'facilities'
type CanvasNode = { label: string; kind: '标题文本' | '数据标签' | '数据规则' | '页面结构'; audience: 'customer' | 'operator' | 'both'; impacts: string; editable: string; locked: string; tab: 'basic' | 'page'; field: string; basicTab?: BasicConfigTab; page?: string; detail?: DetailSectionTab }
const canvasNodes: Record<string, CanvasNode> = {
  'home.title': { label: '首页主标题', kind: '标题文本', audience: 'customer', impacts: '首页首屏', editable: '标题文字', locked: '页面结构与找房逻辑', tab: 'page', page: 'home', field: 'home-title' },
  'home.search': { label: '搜索提示', kind: '标题文本', audience: 'customer', impacts: '首页搜索入口', editable: '提示文字', locked: '搜索功能与筛选逻辑', tab: 'page', page: 'home', field: 'home-search' },
  'home.sections': { label: '首页分区文案', kind: '标题文本', audience: 'customer', impacts: '频道与推荐房源区', editable: '分区标题和查看按钮', locked: '频道与房源数据', tab: 'page', page: 'home', field: 'home-sections' },
  'page.heading': { label: '页面标题与辅助文本', kind: '标题文本', audience: 'both', impacts: '当前所选小程序页面', editable: '页面标题、辅助文本与背景图', locked: '页面功能与业务流程', tab: 'page', field: 'page-copy' },
  'brand': { label: '品牌名称', kind: '标题文本', audience: 'both', impacts: '导航、登录、详情服务与分享落款', editable: '对外品牌名称', locked: '客服权限与业务数据', tab: 'basic', basicTab: 'brand', field: 'brand-name' },
  'status': { label: '房源状态标签', kind: '数据标签', audience: 'both', impacts: '管理员房源管理；客户仅显示可租', editable: '各状态显示名称', locked: '状态编码、流转和客户可见规则', tab: 'basic', basicTab: 'brand', field: 'status-labels' },
  'coverage': { label: '地区与关联范围', kind: '数据规则', audience: 'both', impacts: '首页地区、频道筛选与附近推荐', editable: '地区名称、频道、地图中心和半径', locked: '自动生成的内部 ID', tab: 'basic', basicTab: 'coverage', field: 'coverage' },
  'facilities': { label: '房屋设施库', kind: '数据标签', audience: 'both', impacts: '房源录入与详情设施展示', editable: '设施名称、分类与启用状态', locked: '已录入房源的历史数据', tab: 'basic', basicTab: 'facilities', field: 'facilities' },
  'tags': { label: '房源标签规则', kind: '页面结构', audience: 'both', impacts: '首页、列表、详情与微信分享', editable: '客户可见性、顺序与排版', locked: '房源真实字段内容', tab: 'basic', basicTab: 'tags', field: 'tags' },
  'detail.overview': { label: '固定首屏摘要', kind: '页面结构', audience: 'both', impacts: '房源详情打开后的首屏成交信息', editable: '详情标签的可见性与顺序', locked: '月租、租赁身份、核心参数与客户端视觉结构', tab: 'page', page: 'propertyDetail', detail: 'overview', field: 'detail-overview' },
  'detail.location': { label: '位置与周边', kind: '页面结构', audience: 'both', impacts: '房源详情位置区', editable: '模块标题与启用状态', locked: '房源保存的坐标', tab: 'page', page: 'propertyDetail', detail: 'location', field: 'detail-location' },
  'detail.facilities': { label: '房屋设施', kind: '数据标签', audience: 'both', impacts: '房源详情设施区与录入选项', editable: '模块标题和设施库内容', locked: '历史房源已保存的设施数据', tab: 'page', page: 'propertyDetail', detail: 'facilities', field: 'detail-facilities' },
  'detail.description': { label: '房源简介', kind: '页面结构', audience: 'both', impacts: '房源详情补充说明区', editable: '模块标题与启用状态', locked: '每套房源的真实描述', tab: 'page', page: 'propertyDetail', detail: 'description', field: 'detail-description' },
  'detail.nearby': { label: '附近房源', kind: '数据规则', audience: 'customer', impacts: '房源详情推荐区', editable: '标题、范围、关联逻辑、排序与数量', locked: '房源真实位置', tab: 'page', page: 'propertyDetail', detail: 'nearby', field: 'detail-nearby' },
}
const selectedCanvasNodeID = ref('home.title')
const selectedCanvasNode = computed(() => canvasNodes[selectedCanvasNodeID.value])
type BasicTagPreviewTab = 'list' | 'detail' | 'share'
const basicConfigTab = ref<BasicConfigTab>('brand')
const basicTagPreviewTab = ref<BasicTagPreviewTab>('list')
const basicPhonePreviewOpen = ref(false)
const editingFacilityKey = ref('')
const configPageMeta = computed(() => pagePresentationOptions.find(page => page.key === configPage.value))
const miniappNativeTitles: Record<string, string> = {
  home: '小皮租房', properties: '房源', requests: '找房需求', publish: '发布房源', me: '我的',
  footprints: '浏览足迹', requestHistory: '我的找房记录', authLogin: '微信登录',
  adminWorkbench: '小皮工作台', adminPropertyEdit: '编辑房源',
}
const canvasNativeTitle = computed(() => miniappNativeTitles[configPage.value] || '房源详情')
function selectDetailConfigTab(value: DetailConfigTab) {
  detailConfigTab.value = value
  previewOverlay.value = value === 'detail' ? 'page' : value
}
function selectDetailSectionTab(value: DetailSectionTab) {
  detailSectionTab.value = value
  detailConfigTab.value = 'detail'
  previewOverlay.value = 'page'
  window.requestAnimationFrame(() => document.querySelector(`[data-canvas-area="${value}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'center' }))
}
function toggleDetailModule(key: DetailSectionTab, enabled: boolean) {
  if (key === 'overview') return
  const module = detailModule(key)
  module.enabled = enabled
  selectDetailSectionTab(key)
}
function selectBasicConfigTab(value: BasicConfigTab) {
  basicConfigTab.value = value
  basicPhonePreviewOpen.value = false
  if (value === 'tags') basicTagPreviewTab.value = 'list'
  if (value === 'coverage') configPage.value = 'home'
}
function toggleFacility(facility: { enabled: boolean }) { facility.enabled = !facility.enabled }
function editFacility(facility: { key: string }) { editingFacilityKey.value = editingFacilityKey.value === facility.key ? '' : facility.key }
function removeDetailFacility(key: string) {
  if (publicConfig.propertyDetail.facilities.length <= 1) {
    configMessage.value = '至少保留一个设施类型'
    configFailed.value = true
    return
  }
  publicConfig.propertyDetail.facilities = publicConfig.propertyDetail.facilities.filter((facility) => facility.key !== key)
  if (editingFacilityKey.value === key) editingFacilityKey.value = ''
}
function focusCanvasArea(area: DetailSectionTab) {
  selectedCanvasNodeID.value = `detail.${area}`
  detailSectionTab.value = area
  detailConfigTab.value = 'detail'
  previewOverlay.value = 'page'
  window.requestAnimationFrame(() => document.querySelector(`[data-canvas-area="${area}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'center' }))
}
function selectCanvasNodeForPage(page: string) {
  selectedCanvasNodeID.value = page === 'home'
    ? 'home.title'
    : page === 'propertyDetail'
      ? 'detail.overview'
      : 'page.heading'
}
const selectConfigPage = (key: string) => {
  const target = pagePresentationOptions.find(page => page.key === key)
  if (!target) return
  canvasPageHistory.value = []
  if (target.audience !== 'both' && target.audience !== configAudience.value) configAudience.value = target.audience as ConfigAudience
  configPage.value = target.key
  if (target.key === 'adminPropertyEdit') canvasEditStep.value = 0
  selectCanvasNodeForPage(target.key)
  detailConfigTab.value = 'detail'
  detailSectionTab.value = 'overview'
  previewOverlay.value = 'page'
  canvasDetailModal.value = ''
  tagMagnifierOpen.value = false
}
function switchConfigAudience(value: ConfigAudience) {
  configAudience.value = value
  canvasIdentityState.value = value === 'operator' ? 'admin' : 'user'
  canvasPageHistory.value = []
  canvasPageFilterReset()
  configPage.value = value === 'operator' ? 'adminWorkbench' : 'home'
  selectCanvasNodeForPage(configPage.value)
  previewSelectedPropertyID.value = ''
  detailConfigTab.value = 'detail'
  detailSectionTab.value = 'overview'
  previewOverlay.value = 'page'
}
function focusConfigField(field: string, basicTab?: BasicConfigTab) {
  if (basicTab) {
    selectTab('小程序基础配置')
    selectBasicConfigTab(basicTab)
  }
  window.setTimeout(() => {
    const target = document.querySelector(`[data-config-field="${field}"]`) as HTMLElement | null
      || document.querySelector('.basic-config-panel, .config-section') as HTMLElement | null
    target?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    target?.querySelector('input, textarea, button')?.focus()
  }, 80)
}
function openCanvasNode(id: string) {
  const node = canvasNodes[id]
  if (!node) return
  selectedCanvasNodeID.value = id
  if (node.tab === 'basic') {
    selectTab('小程序基础配置')
    selectBasicConfigTab(node.basicTab || 'brand')
    focusConfigField(node.field)
    return
  }
  selectTab('小程序页面配置')
  if (node.audience !== 'both') configAudience.value = node.audience
  selectConfigPage(node.page || (id === 'page.heading' ? configPage.value : 'home'))
  if (node.detail) {
    detailConfigTab.value = 'detail'
    detailSectionTab.value = node.detail
  }
  focusConfigField(node.field)
}
function previewStatusLabel(status: string) {
  if (configAudience.value === 'customer') return publicConfig.statusLabels.available || '可租'
  const labels: Record<string, string> = {
    draft: publicConfig.statusLabels.draft,
    pending_review: publicConfig.statusLabels.pendingReview,
    published: publicConfig.statusLabels.published,
    available: publicConfig.statusLabels.available,
    rented: publicConfig.statusLabels.rented,
    offline: publicConfig.statusLabels.offline,
  }
  return labels[status] || status
}
const assigningAdmin = ref(false)
const adminPhone = ref('')
const adminCandidate = ref<WechatUser | null>(null)
const adminSearchOpen = ref(false)
const adminMessage = ref('')
const adminFailed = ref(false)
const properties = ref<Property[]>([])
const leads = ref<Lead[]>([])
const rentalRequests = ref<RentalRequest[]>([])
const favoriteContacts = ref<FavoriteContact[]>([])
const users = ref<WechatUser[]>([])
const admins = ref<AdminUser[]>([])
const temporaryAccess = ref<TemporaryAccess[]>([])
const temporaryPhone = ref('')
const temporaryHours = ref(168)
const generatingTemporaryCode = ref(false)
const generatedTemporaryCode = ref('')
const temporaryMessage = ref('')
const temporaryFailed = ref(false)
const propertyFilter = ref('all')
const leadFilter = computed(() => route.query.filter === 'pending' && tab.value === '线索管理' ? 'pending' : 'all')
const requestFilter = computed(() => route.query.filter === 'new' && tab.value === '租客需求' ? 'new' : 'all')
const overviewState = ref<'loading' | 'ready' | 'error'>('loading')
const overviewError = ref('')
const channelLocationResults = ref<Record<number, LocationSearchItem[]>>({})
const channelLocationLoading = ref<Record<number, boolean>>({})
const channelMapPreview = ref<Record<number, string>>({})
const channelMapLoading = ref<Record<number, boolean>>({})
const channelMapCache = ref<Record<string, string>>({})
const channelMapRequest = ref<Record<number, number>>({})
const channelMapActiveMarker = ref<Record<number, string>>({})
const channelSelectedLocation = ref<Partial<Record<number, LocationSearchItem>>>({})
const activeChannelIndex = ref(0)
const coverageDataTab = ref<'locations' | 'channels'>('locations')
const activeChannel = computed(() => publicConfig.featuredChannels[activeChannelIndex.value])
function channelSetupStatus(item: ChannelConfig) {
  if (!item.title.trim()) return '待填写'
  if (!item.latitude || !item.longitude) return '待选点'
  return '已定位'
}
const defaultShareCard = () => ({ titlePrefix: '', titleFields: ['community', 'layout'], posterFields: ['rentMonthly', 'depositPayment', 'community', 'areaSqm', 'floorInfo', 'availableDate'] })
const defaultListingTitle = () => ({ marketingPhrases: ['好房快抢', '捡漏不等人'], customerAllowedFields: ['community', 'floor', 'layout', 'rentType', 'listingSource', 'fee', 'area'], enabledFields: ['community', 'layout', 'rentType', 'fee'], customerDetailFields: ['community', 'layout', 'rentType', 'fee'], customerShareFields: ['community', 'layout', 'rentType', 'fee'], customerPosterFields: ['community', 'layout', 'rentType', 'fee'], adminEnabledFields: ['unit'], listTagLayout: 'grouped' as const, detailTagLayout: 'wrap' as const, listTagOrder: ['rentType', 'listingSource', 'layout', 'fee', 'floor', 'area'], listTagTopFields: ['rentType', 'listingSource'], detailTagOrder: ['listingSource', 'rentType', 'fee', 'layout', 'area', 'floor'], detailTagTopFields: ['listingSource', 'rentType', 'fee'] })
const defaultPropertyDetail = () => ({
  modules: [
    { key: 'overview', title: '房屋概况', enabled: true }, { key: 'location', title: '位置与周边', enabled: true },
    { key: 'facilities', title: '房屋设施', enabled: true }, { key: 'description', title: '房源简介', enabled: true }, { key: 'nearby', title: '附近房源', enabled: true },
  ],
  nearby: { radiusKm: 3, relation: 'distance' as const, sort: 'distance' as const, limit: 4 },
  facilities: [
    { key: 'refrigerator', label: '冰箱', group: '家具家电', enabled: true }, { key: 'washer', label: '洗衣机', group: '家具家电', enabled: true },
    { key: 'air_conditioner', label: '空调', group: '家具家电', enabled: true }, { key: 'water_heater', label: '热水器', group: '家具家电', enabled: true },
    { key: 'television', label: '电视', group: '家具家电', enabled: true }, { key: 'broadband', label: '宽带', group: '家具家电', enabled: true },
    { key: 'gas', label: '燃气', group: '厨房', enabled: true }, { key: 'hood', label: '油烟机', group: '厨房', enabled: true },
    { key: 'sofa', label: '沙发', group: '家具家电', enabled: true }, { key: 'wardrobe', label: '衣柜', group: '家具家电', enabled: true },
    { key: 'desk', label: '书桌', group: '家具家电', enabled: true }, { key: 'bed', label: '床', group: '家具家电', enabled: true },
  ],
})
const facilityCatalog = [
  { key: 'refrigerator', label: '冰箱', group: '家具家电', icon: '▥' }, { key: 'washer', label: '洗衣机', group: '家具家电', icon: '◉' },
  { key: 'air_conditioner', label: '空调', group: '家具家电', icon: '❄' }, { key: 'water_heater', label: '热水器', group: '家具家电', icon: '♨' },
  { key: 'television', label: '电视', group: '家具家电', icon: '▣' }, { key: 'broadband', label: '宽带', group: '家具家电', icon: '⌁' },
  { key: 'gas', label: '燃气', group: '厨房', icon: '⌁' }, { key: 'hood', label: '油烟机', group: '厨房', icon: '⌂' },
  { key: 'sofa', label: '沙发', group: '家具家电', icon: '▰' }, { key: 'wardrobe', label: '衣柜', group: '家具家电', icon: '▤' },
  { key: 'desk', label: '书桌', group: '家具家电', icon: '▱' }, { key: 'bed', label: '床', group: '家具家电', icon: '▰' },
  { key: 'elevator', label: '电梯', group: '居住条件', icon: '↕' }, { key: 'private_bathroom', label: '独立卫生间', group: '居住条件', icon: '◌' },
  { key: 'balcony', label: '阳台', group: '居住条件', icon: '▱' }, { key: 'smart_lock', label: '智能门锁', group: '安全', icon: '◈' }, { key: 'parking', label: '车位', group: '配套服务', icon: 'P' },
]
const facilityToAdd = ref('')
const availableFacilityCatalog = computed(() => facilityCatalog.filter(item => !publicConfig.propertyDetail.facilities.some(facility => facility.key === item.key)))
const detailFacilityIcon = (key: string) => facilityCatalog.find(item => item.key === key)?.icon || '•'
const defaultFacilityKeys = new Set(defaultPropertyDetail().facilities.map(item => item.key))
const normalizeDetailFacilities = (facilities: typeof defaultPropertyDetail extends () => infer Result ? Result['facilities'] : never) => {
  const stored = new Map((facilities || []).filter(item => !['elevator', 'private_bathroom', 'balcony'].includes(item.key)).map(item => [item.key, item]))
  return facilityCatalog.filter(item => defaultFacilityKeys.has(item.key) || stored.has(item.key)).map(item => {
    const current = stored.get(item.key)
    return {
      key: item.key,
      label: current?.label?.trim() || item.label,
      group: current?.group?.trim() || item.group,
      enabled: current?.enabled ?? true,
    }
  })
}
const listingTitleFieldOptions = [
  { key: 'community', label: '小区' }, { key: 'unit', label: '门牌标签' }, { key: 'floor', label: '楼层' },
  { key: 'layout', label: '户型' }, { key: 'rentType', label: '整租 / 合租' }, { key: 'listingSource', label: '直租 / 转租' }, { key: 'fee', label: '有无中介费' }, { key: 'area', label: '面积' },
]
const draggingTag = ref('')
function arrangedFields(source: string[], visible: string[]) {
  const known = new Set<string>()
  return [...source, ...visible].filter(key => key !== 'community' && key !== 'unit' && visible.includes(key)).filter(key => known.has(key) ? false : (known.add(key), true))
}
const listOrderedTagFields = computed(() => arrangedFields(publicConfig.listingTitle.listTagOrder, publicConfig.listingTitle.enabledFields))
const listTopTagFields = computed(() => listOrderedTagFields.value.filter(key => publicConfig.listingTitle.listTagTopFields.includes(key)))
const listBottomTagFields = computed(() => listOrderedTagFields.value.filter(key => !publicConfig.listingTitle.listTagTopFields.includes(key)))
const detailOrderedTagFields = computed(() => arrangedFields(publicConfig.listingTitle.detailTagOrder, publicConfig.listingTitle.customerDetailFields))
const detailTopTagFields = computed(() => detailOrderedTagFields.value.filter(key => publicConfig.listingTitle.detailTagTopFields.includes(key)))
const detailBottomTagFields = computed(() => detailOrderedTagFields.value.filter(key => !publicConfig.listingTitle.detailTagTopFields.includes(key)))
function moveTagToListRow(row: 'top' | 'bottom', targetKey?: string) {
  const key = draggingTag.value
  if (!key || !publicConfig.listingTitle.enabledFields.includes(key)) return
  const order = publicConfig.listingTitle.listTagOrder.filter(item => item !== key)
  const targetIndex = targetKey ? order.indexOf(targetKey) : order.length
  order.splice(targetIndex < 0 ? order.length : targetIndex, 0, key)
  publicConfig.listingTitle.listTagOrder = order
  publicConfig.listingTitle.listTagTopFields = row === 'top'
    ? [...new Set([...publicConfig.listingTitle.listTagTopFields, key])]
    : publicConfig.listingTitle.listTagTopFields.filter(item => item !== key)
  draggingTag.value = ''
}
function moveDetailTag(row: 'top' | 'bottom', targetKey?: string) {
  const key = draggingTag.value
  if (!key || !publicConfig.listingTitle.customerDetailFields.includes(key)) return
  const order = publicConfig.listingTitle.detailTagOrder.filter(item => item !== key)
  const targetIndex = targetKey ? order.indexOf(targetKey) : order.length
  order.splice(targetIndex < 0 ? order.length : targetIndex, 0, key)
  publicConfig.listingTitle.detailTagOrder = order
  publicConfig.listingTitle.detailTagTopFields = row === 'top'
    ? [...new Set([...publicConfig.listingTitle.detailTagTopFields, key])]
    : publicConfig.listingTitle.detailTagTopFields.filter(item => item !== key)
  draggingTag.value = ''
}
function applyRecommendedListTagArrangement() {
  const preferred = ['rentType', 'listingSource', 'layout', 'fee', 'area', 'floor']
  publicConfig.listingTitle.listTagOrder = arrangedFields(preferred, publicConfig.listingTitle.enabledFields)
  publicConfig.listingTitle.listTagTopFields = ['rentType', 'listingSource'].filter(key => publicConfig.listingTitle.enabledFields.includes(key))
}
function applyRecommendedDetailTagArrangement() {
  const preferred = ['listingSource', 'rentType', 'fee', 'layout', 'area', 'floor']
  publicConfig.listingTitle.detailTagOrder = arrangedFields(preferred, publicConfig.listingTitle.customerDetailFields)
  publicConfig.listingTitle.detailTagTopFields = ['listingSource', 'rentType', 'fee'].filter(key => publicConfig.listingTitle.customerDetailFields.includes(key))
}
const previewTagExamples: Record<string, string> = { unit: '5栋3单元101室', floor: '18楼', layout: '两室一厅', rentType: '整租', listingSource: '直租', fee: '无中介费', area: '68㎡' }
const previewListFields = computed(() => [
  ...publicConfig.listingTitle.enabledFields.filter(key => key !== 'community' && key !== 'unit'),
])
const previewOperatorDoorplate = computed(() => {
  if (configAudience.value !== 'operator' || !publicConfig.listingTitle.adminEnabledFields.includes('unit')) return ''
  const item = previewPrimaryProperty.value
  return [item?.buildingNo, item?.unitNo && `${String(item.unitNo).replace(/单元$/, '')}单元`, item?.roomNo && `${String(item.roomNo).replace(/室$/, '')}室`].filter(Boolean).join('')
})
const previewListingTags = computed(() => previewListFields.value
  .filter(key => key !== 'community')
  .map(key => previewTagExamples[key])
  .filter(Boolean))
const previewPropertyOptions = computed(() => {
  return configAudience.value === 'customer'
    ? properties.value.filter((item) => item.status === 'available')
    : properties.value
})
function previewMatchesArea(item: Property, label: string) {
  if (label === '全部') return true
  const areaLabel = String((item as Property & { areaLabel?: string }).areaLabel || '')
  if (areaLabel) return areaLabel === label
  const candidates = publicConfig.featuredChannels.filter((channel) => channel.latitude && channel.longitude)
  if (!item.latitude || !item.longitude || !candidates.length) return false
  const nearest = candidates.reduce((current, candidate) => previewDistanceKm(Number(candidate.latitude), Number(candidate.longitude), Number(item.latitude), Number(item.longitude)) < previewDistanceKm(Number(current.latitude), Number(current.longitude), Number(item.latitude), Number(item.longitude)) ? candidate : current)
  return nearest.title === label
}
function previewMatchesKind(item: Property, kind: string) {
  if (!kind || kind === '全部' || kind === '不限' || kind === '推荐') return true
  const text = `${item.title || ''} ${item.layout || ''} ${item.rentType || ''}`
  if (kind === '整租' && !item.rentType) return true
  const roomMatch = `${item.layout || ''} ${item.title || ''}`.match(/(\d+)\s*室/) || `${item.layout || ''} ${item.title || ''}`.match(/([一二两三四五六七八九])\s*居/)
  const roomCount = roomMatch ? (/^\d+$/.test(roomMatch[1]) ? Number(roomMatch[1]) : ({ 一: 1, 二: 2, 两: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9 } as Record<string, number>)[roomMatch[1]] || 0) : 0
  if (kind === '一居') return roomCount === 1
  if (kind === '两居') return roomCount === 2
  if (kind === '三居+') return roomCount >= 3
  return text.includes(kind)
}
function previewPropertyNeedsDetails(item: Property) {
  return !item.community || !item.location || !item.latitude || !item.longitude || !item.media?.length
}
const previewProperties = computed(() => {
  const query = canvasSearchQuery.value.trim().toLowerCase()
  const budget = canvasBudgetFilter.value
  return previewPropertyOptions.value.filter((item) => {
    if (canvasStatusFilter.value !== 'all') {
      if (configAudience.value === 'customer' && item.status !== 'available') return false
      if (configAudience.value === 'operator') {
        const status = canvasStatusFilter.value
        const matched = status === 'draft' ? item.status === 'draft'
          : status === 'todo' ? ['draft', 'pending_review'].includes(item.status)
            : status === 'online' ? ['published', 'available'].includes(item.status)
              : status === 'incomplete' ? previewPropertyNeedsDetails(item)
                : item.status === status
        if (!matched) return false
      } else if (normalizedPropertyStatus(item.status) !== canvasStatusFilter.value) return false
    }
    if (!previewMatchesArea(item, canvasAreaFilter.value)) return false
    if (canvasChannelFilter.value && !previewMatchesArea(item, canvasChannelFilter.value)) return false
    if (!previewMatchesKind(item, canvasRentTypeFilter.value)) return false
    if (canvasFavoriteOnly.value && configAudience.value === 'customer' && !canvasPreviewFavorites.value.includes(item.id)) return false
    if (query && !`${item.title} ${item.community} ${item.location} ${item.layout}`.toLowerCase().includes(query)) return false
    if (budget === '≤1500' && item.rentMonthly > 1500) return false
    if (budget === '1500–2500' && (item.rentMonthly <= 1500 || item.rentMonthly > 2500)) return false
    if (budget === '2500–3500' && (item.rentMonthly <= 2500 || item.rentMonthly >= 3500)) return false
    if (budget === '≥3500' && item.rentMonthly < 3500) return false
    return true
  })
})
const canvasHasPropertyFilters = computed(() => Boolean(
  canvasSearchQuery.value.trim()
  || canvasAreaFilter.value !== '全部'
  || canvasChannelFilter.value
  || canvasRentTypeFilter.value !== '全部'
  || canvasBudgetFilter.value !== '不限'
))
const canvasPropertyEmptyTitle = computed(() => canvasFavoriteOnly.value
  ? '还没有找到关注的房源'
  : canvasAreaFilter.value !== '全部'
    ? `${canvasAreaFilter.value} 暂时没有合适的房源`
    : canvasSearchQuery.value.trim()
      ? `没有找到“${canvasSearchQuery.value.trim()}”相关房源`
      : '暂时没有符合条件的房源')
const canvasPropertyEmptyAction = computed(() => canvasFavoriteOnly.value
  ? '浏览全部房源'
  : canvasAreaFilter.value !== '全部'
    ? '关注该片区新房'
    : '提交找房需求')
function handleCanvasPropertyEmptyAction() {
  if (canvasFavoriteOnly.value) {
    canvasPageFilterReset()
  } else if (canvasAreaFilter.value !== '全部' && canvasIdentityState.value === 'guest') {
    navigateCanvasPage('authLogin')
  } else {
    navigateCanvasPage('requests')
  }
}
const previewCustomerMasonryColumns = computed(() => {
  const columns: Property[][] = [[], []]
  const heights = [0, 0]
  previewProperties.value.forEach((item) => {
    const copyLength = `${previewListingTitle(item)} ${previewTagGroups(item).all.join(' ')}`.length
    const estimate = 360 + Math.ceil(copyLength / 12) * 38
    const index = heights[0] <= heights[1] ? 0 : 1
    columns[index].push(item)
    heights[index] += estimate
  })
  return columns
})
const previewSelectedPropertyID = ref('')
const previewPrimaryProperty = computed(() => previewPropertyOptions.value.find((item) => item.id === previewSelectedPropertyID.value) || previewProperties.value[0] || previewPropertyOptions.value[0])
const canvasDataState = computed<CanvasDataState>(() => {
  if (loading.value || overviewState.value === 'loading') return 'loading'
  if (overviewState.value === 'error') return 'error'
  if (['home', 'properties'].includes(configPage.value)) return previewPropertyOptions.value.length ? 'normal' : 'empty'
  if (configPage.value === 'propertyDetail') return previewPrimaryProperty.value ? 'normal' : 'empty'
  return 'normal'
})
function setCanvasNotice(message = '此画布只读，未提交任何业务数据。') {
  canvasNotice.value = message
  window.setTimeout(() => { if (canvasNotice.value === message) canvasNotice.value = '' }, 2800)
}
function updateCanvasFit() {
  const stage = canvasPreviewStage.value
  const phone = canvasPreviewPhone.value
  if (!stage || !phone) return
  const width = phone.offsetWidth || 203
  const height = phone.offsetHeight || 441
  // The 203×441 design frame is scaled to the measured iPhone 17 simulator
  // viewport: 604×1312 physical pixels at deviceScaleFactor 2 (302×656 CSS px).
  // Preserve the 0.460 aspect ratio and center the device in the available rail.
  const simulatorScale = 302 / width
  canvasFitScale.value = Math.max(0.35, Math.min(simulatorScale, stage.clientWidth / width, stage.clientHeight / height))
}
function navigateCanvasPage(key: string) {
  if (key === configPage.value) return
  const current = configPage.value
  selectConfigPage(key)
  canvasPageHistory.value.push(current)
}
function switchCanvasTab(key: string) {
  canvasPageHistory.value = []
  selectConfigPage(key)
}
function backCanvasPage() {
  const previous = canvasPageHistory.value.pop()
  if (previous) { selectConfigPage(previous); return }
  selectConfigPage(configAudience.value === 'customer' ? 'properties' : 'adminWorkbench')
}
function switchCanvasIdentity(value: 'guest' | 'user' | 'admin') {
  canvasIdentityState.value = value
  const audience: ConfigAudience = value === 'admin' ? 'operator' : 'customer'
  configAudience.value = audience
  canvasPageHistory.value = []
  canvasPageFilterReset()
  selectConfigPage(audience === 'customer' ? 'home' : 'adminWorkbench')
}
function canvasPageFilterReset() {
  canvasSearchQuery.value = ''
  canvasAreaFilter.value = '全部'
  canvasChannelFilter.value = ''
  canvasRentTypeFilter.value = '全部'
  canvasBudgetFilter.value = '不限'
  canvasStatusFilter.value = 'all'
  canvasFavoriteOnly.value = false
  canvasPropertyFiltersOpen.value = false
}
function toggleCanvasFavorite(propertyId: string) {
  if (canvasIdentityState.value === 'guest') { navigateCanvasPage('authLogin'); return }
  canvasPreviewFavorites.value = canvasPreviewFavorites.value.includes(propertyId)
    ? canvasPreviewFavorites.value.filter((id) => id !== propertyId)
    : [...canvasPreviewFavorites.value, propertyId]
  setCanvasNotice('关注状态仅在本次画布预览中显示，不会写入业务数据。')
}
function submitCanvasPreview() {
  if (configAudience.value === 'customer' && canvasIdentityState.value === 'guest' && ['requests', 'requestHistory', 'footprints'].includes(configPage.value)) {
    navigateCanvasPage('authLogin')
    return
  }
  canvasDetailModal.value = ''
  setCanvasNotice('预览模式不会提交、修改或创建业务数据。')
}
function startCanvasPropertyEdit(item?: Property) {
  previewSelectedPropertyID.value = item?.id || ''
  canvasEditingExistingProperty.value = Boolean(item)
  Object.assign(canvasPropertyDraft, {
    community: item?.community || '', location: item?.location || '', buildingNo: item?.buildingNo || '', unitNo: item?.unitNo || '', roomNo: item?.roomNo || '', rentType: item?.rentType || '整租', listingSource: item?.listingSource || '直租', layout: item?.layout || '', areaSqm: item?.areaSqm ? String(item.areaSqm) : '', floorInfo: item?.floorInfo || '', orientation: item?.orientation || '', elevator: item?.elevator || '', rentMonthly: item?.rentMonthly ? String(item.rentMonthly) : '', feeText: item?.feeText || '', depositPayment: item?.depositPayment || '', availableDate: item?.availableDate || '', utilitiesText: item?.utilitiesText || '', description: item?.description || '', rulesText: item?.rulesText || '', marketingPhrase: item?.marketingPhrase || '',
  })
  canvasPropertyFacilities.value = []
  canvasEditStep.value = 0
  navigateCanvasPage('adminPropertyEdit')
}
function toggleCanvasPropertyFacility(label: string) {
  canvasPropertyFacilities.value = canvasPropertyFacilities.value.includes(label)
    ? canvasPropertyFacilities.value.filter(item => item !== label)
    : [...canvasPropertyFacilities.value, label]
}
const canvasDraftTitle = computed(() => [canvasPropertyDraft.community || '房源小区', canvasPropertyDraft.marketingPhrase].filter(Boolean).join(' · '))
function openPreviewProperty(item?: Property) {
  if (!item) return
  previewSelectedPropertyID.value = item.id
  canvasMediaIndex.value = 0
  navigateCanvasPage('propertyDetail')
}
const previewIdentity = computed(() => {
  if (configAudience.value === 'customer' && canvasIdentityState.value === 'guest') return { name: '登录后开始找房', caption: '同步收藏、足迹和找房需求', mark: '小' }
  if (configAudience.value === 'operator') {
    const account = admins.value.find((item) => item.status === 'active') || admins.value[0]
    return { name: account?.nickname || '管理员', caption: account?.phone ? `管理员已登录 · ${account.phone}` : '管理员已登录', mark: '管' }
  }
  if (canvasIdentityState.value === 'admin') {
    const account = admins.value.find((item) => item.status === 'active') || admins.value[0]
    return { name: account?.nickname || '管理员', caption: account?.phone ? `管理员 · ${account.phone}` : '管理员身份已启用', mark: '管' }
  }
  return { name: '微信用户', caption: '客户页面预览 · 未绑定具体微信账号', mark: '小' }
})
function previewPropertyTags(item?: Property, configuredFields?: string[]) {
  if (!item) return previewListingTags.value
  const fields = configuredFields || previewListFields.value
  const unit = [item.buildingNo, item.unitNo && (String(item.unitNo).includes('单元') ? item.unitNo : `${item.unitNo}单元`), item.roomNo && (String(item.roomNo).includes('室') ? item.roomNo : `${item.roomNo}室`)].filter(Boolean).join('')
  const layout = String(item.layout || '').split(/[·・|｜]/).map((value) => value.trim()).filter((value) => value && !/^(整租|合租|直租|转租|有中介费|无中介费)$/.test(value)).join(' · ')
  const fee = String(item.feeText || '').includes('无中介费') ? '无中介费' : String(item.feeText || '').includes('有中介费') ? '有中介费' : ''
  const values: Record<string, string> = {
    unit,
    floor: item.floorInfo || '',
    layout,
    rentType: item.rentType === '合租' ? '合租' : '整租',
    listingSource: item.listingSource === '转租' ? '转租' : '直租',
    fee,
    area: item.areaSqm ? `${item.areaSqm}㎡` : '',
  }
  const known = new Set<string>()
  return fields
    .filter((key) => key !== 'community')
    .map((key) => values[key])
    .filter((value) => Boolean(value))
    .filter((value) => known.has(value) ? false : (known.add(value), true))
}
function previewDetailTags(item?: Property) {
  // Keep the right canvas and the local magnifier on the exact same ordering
  // source as the draggable editor.  The customer field list decides what is
  // allowed; detailTagOrder decides where each allowed tag is rendered.
  const fields = arrangedFields(
    publicConfig.listingTitle.detailTagOrder,
    publicConfig.listingTitle.customerDetailFields,
  )
  return previewPropertyTags(item, fields)
}
function previewDetailTagGroups(item?: Property) {
  // The mini-program owns fixed wrapping and card geometry. Config may order
  // public data labels, but it cannot select a visual layout variant.
  return { top: previewDetailTags(item), bottom: [] as string[] }
}
function previewShareTags(item?: Property) {
  const fields = publicConfig.listingTitle.customerShareFields
    .filter(key => key !== 'community' && key !== 'unit')
  return previewPropertyTags(item, fields)
}
function previewPosterTags(item?: Property) {
  const fields = publicConfig.listingTitle.customerPosterFields
    .filter(key => key !== 'community' && key !== 'unit')
  return previewPropertyTags(item, fields)
}
// Keep the configuration canvas honest: these are the same presentation rules
// as the miniapp cards (stable community heading, short address, then two tag rows).
function previewListingTitle(item?: Property) {
  const base = String(item?.community || item?.title || '房源').trim()
  const phrase = configAudience.value === 'customer' ? String(item?.marketingPhrase || '').trim() : ''
  return phrase ? `${base} · ${phrase}` : base
}
function previewListingAddress(item?: Property) {
  const raw = String(item?.location || '').trim()
  if (!raw) return '位置待补充'
  const district = raw.split(/[·・]/).map(part => part.trim()).find(part => /区$/.test(part))
  const landmark = raw.match(/[^省市区\s·・]*(?:大道|一路|二路|三路|路|街道|街|商圈)/)?.[0]
  return [district, landmark].filter(Boolean).join(' · ') || raw.split(/[·・]/)[0].trim()
}
function previewChannelLabel(item?: Property) {
  const label = String(item?.areaLabel || '').trim()
  return publicConfig.featuredChannels.some(channel => channel.title === label) ? label : ''
}
function previewTagGroups(item?: Property) {
  const enabled = previewListFields.value
  const all = previewPropertyTags(item)
  const labelFor = (key: string) => {
    const value = previewPropertyTags(item)
    const raw = key === 'rentType' ? (item?.rentType || '整租')
      : key === 'listingSource' ? (item?.listingSource || '直租')
        : key === 'fee' ? (String(item?.feeText || '').includes('无中介费') ? '无中介费' : String(item?.feeText || '').includes('有中介费') ? '有中介费' : '')
          : key === 'unit' ? [item?.buildingNo, item?.unitNo && `${String(item.unitNo).replace(/单元$/, '')}单元`, item?.roomNo && `${String(item.roomNo).replace(/室$/, '')}室`].filter(Boolean).join('')
            : key === 'layout' ? String(item?.layout || '').split(/[·・|｜]/).map(part => part.trim()).filter(part => part && !/^(整租|合租|直租|转租|有中介费|无中介费)$/.test(part)).join(' · ')
              : key === 'floor' ? String(item?.floorInfo || '') : key === 'area' ? (item?.areaSqm ? `${item.areaSqm}㎡` : '') : ''
    return raw && value.includes(raw) ? raw : ''
  }
  const values = (keys: string[]) => keys.filter(key => enabled.includes(key)).map(labelFor).filter(Boolean)
  const ordered = arrangedFields(publicConfig.listingTitle.listTagOrder, enabled)
  const primary = values(ordered.filter(key => publicConfig.listingTitle.listTagTopFields.includes(key)))
  const secondary = values(ordered.filter(key => !publicConfig.listingTitle.listTagTopFields.includes(key)))
  return { primary, secondary, all: [...primary, ...secondary] }
}
function previewDescription(item?: Property) {
  return String(item?.description || '').replace(/配套设施[ \t]*[：:][ \t]*[^\n]*\n?/, '').trim()
}
function detailModule(key: string) {
  return publicConfig.propertyDetail.modules.find(item => item.key === key) || { key, title: '', enabled: false }
}
function detailModuleEnabled(key: string) { return key === 'overview' || detailModule(key).enabled }
function detailModuleTitle(key: string, fallback: string) { return detailModule(key).title || fallback }
const previewDetailAmenities = computed(() => {
  const match = String(previewPrimaryProperty.value?.description || '').match(/配套设施[ \t]*[：:][ \t]*([^\n]*)/)
  const source = match?.[1]?.split(/[、,，]/).map(item => item.trim()).filter(Boolean) || []
  return source.map(label => publicConfig.propertyDetail.facilities.find(facility => facility.enabled && facility.label === label)).filter((facility): facility is { key: string; label: string; group: string; enabled: boolean } => Boolean(facility))
})
const previewDetailFacilityGroups = computed(() => {
  const groups = new Map<string, typeof previewDetailAmenities.value>()
  previewDetailAmenities.value.forEach((facility) => groups.set(facility.group, [...(groups.get(facility.group) || []), facility]))
  return Array.from(groups, ([title, items]) => ({ title, items }))
})
const previewHasLocation = computed(() => Boolean(Number(previewPrimaryProperty.value?.latitude) && Number(previewPrimaryProperty.value?.longitude)))
function previewDistanceKm(lat1: number, lon1: number, lat2: number, lon2: number) {
  const radians = Math.PI / 180
  const latitudeDelta = (lat2 - lat1) * radians
  const longitudeDelta = (lon2 - lon1) * radians
  const value = Math.sin(latitudeDelta / 2) ** 2 + Math.cos(lat1 * radians) * Math.cos(lat2 * radians) * Math.sin(longitudeDelta / 2) ** 2
  return 6371 * 2 * Math.atan2(Math.sqrt(value), Math.sqrt(1 - value))
}
const previewNearbyProperties = computed(() => {
  const current = previewPrimaryProperty.value
  if (!current || !previewHasLocation.value) return []
  const candidates = previewPropertyOptions.value
    .filter(item => item.id !== current.id && ['published', 'available'].includes(item.status) && Number(item.latitude) && Number(item.longitude))
    .map(item => ({ ...item, distance: previewDistanceKm(Number(current.latitude), Number(current.longitude), Number(item.latitude), Number(item.longitude)) }))
    .filter(item => item.distance <= Number(publicConfig.propertyDetail.nearby.radiusKm || 3))
  const inSameChannel = (item: Property) => publicConfig.featuredChannels.some(channel => Number(channel.latitude) && Number(channel.longitude)
    && previewDistanceKm(Number(current.latitude), Number(current.longitude), Number(channel.latitude), Number(channel.longitude)) <= Number(channel.radiusKm || 3)
    && previewDistanceKm(Number(item.latitude), Number(item.longitude), Number(channel.latitude), Number(channel.longitude)) <= Number(channel.radiusKm || 3))
  return candidates
    .filter(item => publicConfig.propertyDetail.nearby.relation !== 'channel' || inSameChannel(item))
    .sort((left, right) => publicConfig.propertyDetail.nearby.sort === 'newest' ? String(right.id).localeCompare(String(left.id)) : left.distance - right.distance)
    .slice(0, Number(publicConfig.propertyDetail.nearby.limit) || 4)
})
function previewCoverStyle(item?: Property, mediaIndex = 0) {
  // Use actual media where available. For records without a photo, mirror the
  // Mini Program's explicit community-name placeholder instead of a blank block.
  const media = item?.media?.[mediaIndex]
  const cover = media?.type === 'image' ? media.url : item?.media?.find((entry) => entry.type === 'image')?.url
  if (cover) return { backgroundImage: `url(${cover})`, backgroundSize: 'cover', backgroundPosition: 'center' }
  const label = (item?.community || '小皮').slice(0, 2).replace(/[&<>]/g, value => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[value] || value))
  const placeholder = `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="240" viewBox="0 0 320 240"><defs><linearGradient id="g" x2="1" y2="1"><stop stop-color="#e4cdaa"/><stop offset="1" stop-color="#b9a58c"/></linearGradient></defs><rect width="320" height="240" fill="url(#g)"/><rect x="103" y="96" width="114" height="48" rx="8" fill="none" stroke="#ffffffaa"/><text x="160" y="127" fill="white" font-size="22" text-anchor="middle" font-family="sans-serif">${label}</text></svg>`
  return { backgroundImage: `url("data:image/svg+xml,${encodeURIComponent(placeholder)}")`, backgroundSize: 'cover', backgroundPosition: 'center' }
}
const shareFieldOptions = [
  { key: 'community', label: '小区名称' }, { key: 'layout', label: '户型 / 房间' }, { key: 'roomNo', label: '门牌信息' },
  { key: 'rentMonthly', label: '月租' }, { key: 'listingSource', label: '直租 / 转租' }, { key: 'rentType', label: '整租 / 合租' },
  { key: 'depositPayment', label: '押付方式' }, { key: 'areaSqm', label: '面积' }, { key: 'floorInfo', label: '楼层' }, { key: 'availableDate', label: '入住时间' },
]
const shareFieldListingMap: Record<string, string> = { layout: 'layout', roomNo: 'unit', listingSource: 'listingSource', rentType: 'rentType', areaSqm: 'area' }
const shareFieldEnabledForCustomer = (key: string) => !shareFieldListingMap[key] || publicConfig.listingTitle.customerShareFields.includes(shareFieldListingMap[key])
const publicConfig = reactive<PublicConfig>({ heroTitle: '', contactPhone: '', relatedPropertyRadiusKm: 3, locations: [], featuredChannels: [], pagePresentations: defaultPagePresentations(), shareCard: defaultShareCard(), listingTitle: defaultListingTitle(), propertyDetail: defaultPropertyDetail(), statusLabels: { draft: '草稿', pendingReview: '待审核', published: '已发布', available: '可租', rented: '已租', offline: '已下架' } })

const createConfigId = (prefix: string) => `${prefix}_${globalThis.crypto?.randomUUID?.() || `${Date.now()}_${Math.random().toString(36).slice(2)}`}`

const api = createAdminApi({ baseUrl: apiBase, onUnauthorized: () => { loggedIn.value = false; password.value = '' } })

const cloneConfig = (value: PublicConfig): PublicConfig => JSON.parse(JSON.stringify(value))
const applyConfig = (value: PublicConfig, markSaved = false) => {
  const next = cloneConfig(value)
  next.relatedPropertyRadiusKm = next.relatedPropertyRadiusKm || 3
  next.listingTitle = { ...defaultListingTitle(), ...(next.listingTitle || {}) }
  next.listingTitle.marketingPhrases = (next.listingTitle.marketingPhrases || []).map(item => String(item || '').trim()).filter(Boolean)
  next.locations = (next.locations || []).map(item => ({ ...item, id: item.id || createConfigId('loc') }))
  next.featuredChannels = (next.featuredChannels || []).map((item, index) => ({ ...item, id: item.id || createConfigId('channel'), latitude: item.latitude || 0, longitude: item.longitude || 0, icon: normalizeChannelStyle(item.icon, index), radiusKm: item.radiusKm || 3 }))
  next.pagePresentations = { ...defaultPagePresentations(), ...(next.pagePresentations || {}) }
  next.shareCard = { ...defaultShareCard(), ...(next.shareCard || {}) }
  next.listingTitle = { ...defaultListingTitle(), ...(next.listingTitle || {}) }
  next.propertyDetail = { ...defaultPropertyDetail(), ...(next.propertyDetail || {}) }
  next.statusLabels = { draft: '草稿', pendingReview: '待审核', published: '已发布', available: '可租', rented: '已租', offline: '已下架', ...(next.statusLabels || {}) }
  next.propertyDetail.modules = next.propertyDetail.modules?.length ? next.propertyDetail.modules : defaultPropertyDetail().modules
  // Upgrade historical display names in memory. Module keys and saved property
  // data stay unchanged; an explicit future save writes back the clearer terms.
  next.propertyDetail.modules = next.propertyDetail.modules.map(module => ({
    ...module,
    title: module.key === 'facilities' && (!module.title || module.title === '房屋设施与环境')
      ? '房屋设施'
      : module.key === 'description' && (!module.title || module.title === '房源描述')
        ? '房源简介'
        : module.title,
  }))
  next.propertyDetail.facilities = normalizeDetailFacilities(next.propertyDetail.facilities?.length ? next.propertyDetail.facilities : defaultPropertyDetail().facilities)
  next.propertyDetail.nearby = { ...defaultPropertyDetail().nearby, ...(next.propertyDetail.nearby || {}) }
  Object.assign(publicConfig, next)
  // Keep a map visible as soon as the configuration view is opened. Existing
  // channel coordinates are used when present; otherwise this is the Wuhan
  // overview until the operator searches for a POI.
  preloadChannelMaps()
  if (markSaved) savedConfigJSON.value = JSON.stringify(publicConfig)
}

async function load() {
  overviewState.value = 'loading'
  overviewError.value = ''
  loading.value = true
  pageError.value = ''
  try {
    const [propertyData, leadData, rentalData, favoriteData, configData, userData, adminData, temporaryData] = await Promise.all([
      api<Property[]>('/v1/properties'),
      api<Lead[]>('/v1/property-leads'),
      api<RentalRequest[]>('/v1/admin/rental-requests'),
      api<FavoriteContact[]>('/v1/admin/favorite-contacts'),
      api<PublicConfig>('/v1/config/public'),
      api<WechatUser[]>('/v1/admin/users'),
      api<AdminUser[]>('/v1/admin/admin-users'),
      api<TemporaryAccess[]>('/v1/admin/temporary-access'),
    ])
    properties.value = propertyData || []
    leads.value = leadData || []
    rentalRequests.value = rentalData || []
    favoriteContacts.value = favoriteData || []
    users.value = userData || []
    admins.value = adminData || []
    temporaryAccess.value = temporaryData || []
    applyConfig(configData, true)
    overviewState.value = 'ready'
  } catch (error) {
    overviewState.value = 'error'
    overviewError.value = error instanceof Error ? error.message : '数据加载失败'
    pageError.value = overviewError.value
    throw error
  } finally {
    loading.value = false
  }
}

async function login() {
  if (!username.value || !password.value || loading.value) return
  loginError.value = ''
  loading.value = true
  try {
    const auth = await api<{ accessToken: string; username: string; super?: boolean; mustChangePassword?: boolean }>('/v1/admin/login', { method: 'POST', body: JSON.stringify({ username: username.value, password: password.value }) })
    localStorage.setItem('xiaopi_admin_token', auth.accessToken)
    webAdminSuper.value = auth.super === true
    sessionStorage.setItem('xiaopi_web_admin_super', String(webAdminSuper.value))
    mustChangePassword.value = auth.mustChangePassword === true
    loggedIn.value = true
    selectTab('运营总览')
    await load()
  } catch (error) {
    localStorage.removeItem('xiaopi_admin_token')
    loggedIn.value = false
    loginError.value = error instanceof Error ? error.message : '登录失败'
  } finally {
    loading.value = false
  }
}

function logout() {
  sessionStorage.removeItem('xiaopi_web_admin_super')
  webAdminSuper.value = false
  mustChangePassword.value = false
  webAccounts.value = []
  localStorage.removeItem('xiaopi_admin_token')
  loggedIn.value = false
  password.value = ''
}

async function webAccountAction(action: () => Promise<void>) {
  if (webAccountBusy.value) return
  webAccountBusy.value = true
  webAccountMessage.value = ''
  try {
    await action()
  } catch (error) {
    webAccountMessage.value = error instanceof Error ? error.message : '操作失败'
  } finally {
    webAccountBusy.value = false
  }
}

async function readWebAccounts() {
  webAccounts.value = await api<WebAccount[]>('/v1/admin/web-accounts')
}

function refreshWebAccounts() {
  return webAccountAction(readWebAccounts)
}

function changeWebPassword() {
  return webAccountAction(async () => {
    await api('/v1/admin/web-password', { method: 'POST', body: JSON.stringify({ password: newWebPassword.value }) })
    newWebPassword.value = ''
    webAccountDialog.value = ''
    logout()
    loginError.value = '密码已更新，请使用新密码登录'
  })
}

function createWebAccount() {
  return webAccountAction(async () => {
    await api('/v1/admin/web-accounts', { method: 'POST', body: JSON.stringify({ username: webAccountName.value, password: webAccountPassword.value }) })
    webAccountName.value = ''
    webAccountPassword.value = ''
    webAccountDialog.value = ''
    await readWebAccounts()
  })
}

function updateWebAccount(account: WebAccount, method: 'PATCH' | 'DELETE', fields?: { enabled?: boolean; password?: string }) {
  return webAccountAction(async () => {
    await api('/v1/admin/web-accounts/' + encodeURIComponent(account.username), { method, body: fields ? JSON.stringify(fields) : undefined })
    webAccountPassword.value = ''
    webAccountDialog.value = ''
    await readWebAccounts()
  })
}

function openOverviewMetric(target: string) {
  const filter = target === '线索管理' ? 'pending' : target === '租客需求' ? 'new' : undefined
  selectTab(target, filter)
}

function refreshOverview() {
  if (!loading.value) void load().catch(() => undefined)
}

function selectTab(value: string, filter?: string) {
  if (value === '后台管理员' && webAdminSuper.value) void refreshWebAccounts()
  const next = value as TabKey
  tab.value = next
  void router.push({ name: 'admin', params: { tab: pathForTab(next) }, query: filter ? { filter } : {} })
}

watch(() => route.params.tab, value => { tab.value = tabFromRoute(value) as TabKey })

router.afterEach((to) => {
  tab.value = tabFromRoute(to.params.tab) as TabKey
})

const formatTime = (value: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const propertyStatuses = [
  { key: 'published', label: '已发布', icon: '✓' }, { key: 'available', label: '可租', icon: '钥' },
  { key: 'rented', label: '已租', icon: '住' }, { key: 'offline', label: '已下架', icon: '下' },
  { key: 'draft', label: '未发布（草稿）', icon: '稿' },
]
const canvasAdminPropertyFilters = [
  { key: 'all', label: '全部' }, { key: 'draft', label: '草稿' }, { key: 'todo', label: '待处理' },
  { key: 'online', label: '在租' }, { key: 'rented', label: '已租' }, { key: 'offline', label: '已下架' }, { key: 'incomplete', label: '待补资料' },
]
const previewIncompletePropertyCount = computed(() => properties.value.filter(previewPropertyNeedsDetails).length)
const previewAdminPendingCount = computed(() => leads.value.filter(item => !['converted', 'closed'].includes(item.status)).length + rentalRequests.value.filter(item => !item.status || item.status === 'new').length)
const normalizedPropertyStatus = (status: string) => status === 'pending_review' ? 'draft' : status
const previewAdminStatusLabel = computed(() => {
  const status = previewPrimaryProperty.value?.status
  if (!status || status === 'published' || status === 'available') return ''
  if (status === 'offline') return '已下架预览'
  if (status === 'pending_review') return '待发布预览'
  return '草稿预览'
})
const previewAdminStatusActions = computed(() => {
  switch (previewPrimaryProperty.value?.status) {
    case 'draft':
    case 'pending_review': return ['发布房源']
    case 'published': return ['设为可租', '标记已租', '下架']
    case 'available': return ['标记已租', '下架']
    case 'rented': return ['设为可租']
    case 'offline': return ['上架可租', '设为已租']
    default: return []
  }
})
const filteredProperties = computed(() => propertyFilter.value === 'all' ? properties.value : properties.value.filter(item => normalizedPropertyStatus(item.status) === propertyFilter.value))
const statusLabel = (status: string) => propertyStatuses.find(item => item.key === normalizedPropertyStatus(status))?.label || status
async function setPropertyStatus(item: Property, status: string) {
  try { await api('/v1/properties/status', { method: 'PATCH', body: JSON.stringify({ propertyId: item.id, status, note: `后台调整为${statusLabel(status)}` }) }); item.status = status }
  catch (error) { pageError.value = error instanceof Error ? error.message : '房源状态更新失败' }
}
const propertyCover = (item: Property) => {
  const url = item.media?.find(media => media.type === 'image')?.url || ''
  if (!url || /^https?:\/\//.test(url)) return url
  return `${apiBase.replace(/\/api\/?$/, '')}${url.startsWith('/') ? url : `/${url}`}`
}

const configDirty = computed(() => Boolean(savedConfigJSON.value) && JSON.stringify(publicConfig) !== savedConfigJSON.value)
const configErrors = computed(() => {
  const errors: string[] = []
  if (!publicConfig.heroTitle.trim()) errors.push('品牌标题不能为空')
  if (publicConfig.contactPhone && !/^\+?\d{6,20}$/.test(publicConfig.contactPhone.trim())) errors.push('联系电话格式不正确')
  if (!publicConfig.relatedPropertyRadiusKm || publicConfig.relatedPropertyRadiusKm < 0.5 || publicConfig.relatedPropertyRadiusKm > 20) errors.push('附近房源关联距离需在 0.5 到 20 公里之间')
  if (!publicConfig.locations.length) errors.push('至少配置一个地理位置')
  if (publicConfig.locations.length > 20) errors.push('地理位置不能超过 20 个')
  publicConfig.locations.forEach((item, index) => {
    if (!item.label.trim() || !item.keyword.trim()) errors.push(`第 ${index + 1} 个地理位置缺少名称或搜索关键词`)
  })
  const locationKeywords = publicConfig.locations.map((item) => item.keyword.trim()).filter(Boolean)
  if (new Set(locationKeywords).size !== locationKeywords.length) errors.push('地理位置搜索关键词不能重复')
  if (!publicConfig.featuredChannels.length) errors.push('至少配置一个精选频道')
  if (publicConfig.featuredChannels.length > 12) errors.push('精选频道不能超过 12 个')
  publicConfig.featuredChannels.forEach((item, index) => {
    if (!item.title.trim() || !item.subtitle.trim() || !item.keyword.trim()) errors.push(`第 ${index + 1} 个精选频道信息不完整`)
    if (!item.radiusKm || item.radiusKm <= 0 || item.radiusKm > 100) errors.push(`第 ${index + 1} 个精选频道关联半径需在 0 到 100 公里之间`)
    if ((item.latitude === 0) !== (item.longitude === 0) || item.latitude < -90 || item.latitude > 90 || item.longitude < -180 || item.longitude > 180) errors.push(`第 ${index + 1} 个精选频道中心坐标不正确`)
  })
  const channelKeywords = publicConfig.featuredChannels.map((item) => item.keyword.trim()).filter(Boolean)
  if (new Set(channelKeywords).size !== channelKeywords.length) errors.push('精选频道搜索关键词不能重复')
  const marketingPhrases = publicConfig.listingTitle.marketingPhrases.map(item => item.trim())
  if (marketingPhrases.length > 20) errors.push('广告短语不能超过 20 条')
  if (marketingPhrases.some(item => !item || [...item].length > 16)) errors.push('每条广告短语需为 1 到 16 个字')
  if (new Set(marketingPhrases).size !== marketingPhrases.length) errors.push('广告短语不能重复')
  if (!publicConfig.shareCard.titleFields.length || publicConfig.shareCard.titleFields.length > 4) errors.push('分享标题需选择 1 到 4 个字段')
  if (publicConfig.shareCard.titleFields.some(key => !shareFieldEnabledForCustomer(key))) errors.push('分享标题含有未在客户标签中启用的字段')
  if (publicConfig.shareCard.posterFields.length < 3 || publicConfig.shareCard.posterFields.length > 8) errors.push('分享海报需选择 3 到 8 个字段')
  if (!publicConfig.listingTitle.customerAllowedFields.length) errors.push('客户视角至少开放一个标签字段')
  if (!publicConfig.listingTitle.enabledFields.length) errors.push('客户列表至少启用一个字段')
  if (!publicConfig.listingTitle.enabledFields.every(field => publicConfig.listingTitle.customerAllowedFields.includes(field))) errors.push('客户展示标签必须先在“客户可选标签”中开启')
  if (!publicConfig.listingTitle.customerDetailFields.length) errors.push('客户详情至少启用一个字段')
  if (!publicConfig.listingTitle.customerDetailFields.every(field => publicConfig.listingTitle.customerAllowedFields.includes(field))) errors.push('客户详情标签必须先在“客户可选标签”中开启')
  if (!publicConfig.listingTitle.customerShareFields.length) errors.push('客户分享至少启用一个字段')
  if (!publicConfig.listingTitle.customerShareFields.every(field => publicConfig.listingTitle.customerAllowedFields.includes(field))) errors.push('客户分享标签必须先在“客户可选标签”中开启')
  if (!publicConfig.listingTitle.customerPosterFields.length) errors.push('客户朋友圈至少启用一个字段')
  if (!publicConfig.listingTitle.customerPosterFields.every(field => publicConfig.listingTitle.customerAllowedFields.includes(field))) errors.push('客户朋友圈标签必须先在“客户可选标签”中开启')
  if (publicConfig.propertyDetail.modules.some(item => !item.title.trim())) errors.push('房源详情模块标题不能为空')
  if (!publicConfig.propertyDetail.facilities.length) errors.push('至少保留一个设施类型')
  if (publicConfig.propertyDetail.facilities.some(item => !item.label.trim() || !item.group.trim())) errors.push('设施名称和分类不能为空')
  return errors
})

const shareFieldLabel = (key: string) => shareFieldOptions.find(item => item.key === key)?.label || key
function toggleShareField(target: 'titleFields' | 'posterFields', key: string, enabled: boolean) {
  if (enabled && !shareFieldEnabledForCustomer(key)) return
  const fields = publicConfig.shareCard[target]
  if (enabled && !fields.includes(key)) fields.push(key)
  if (!enabled) publicConfig.shareCard[target] = fields.filter(item => item !== key)
}
function moveShareField(target: 'titleFields' | 'posterFields', index: number, offset: number) {
  const fields = publicConfig.shareCard[target]
  const next = index + offset
  if (next < 0 || next >= fields.length) return
  ;[fields[index], fields[next]] = [fields[next], fields[index]]
}
function addDetailFacility(key: string) {
  const item = facilityCatalog.find(candidate => candidate.key === key)
  if (!item || publicConfig.propertyDetail.facilities.some(facility => facility.key === key) || publicConfig.propertyDetail.facilities.length >= 24) return
  publicConfig.propertyDetail.facilities.push({ key: item.key, label: item.label, group: item.group, enabled: true })
  facilityToAdd.value = ''
}
const shareTitlePreview = computed(() => {
  const examples: Record<string, string> = { community: '拿铁公寓', layout: '两室一厅', roomNo: '5栋3单元101室', rentMonthly: '¥2300/月', listingSource: '直租', rentType: '整租', depositPayment: '押一付一', areaSqm: '68㎡', floorInfo: '中层', availableDate: '拎包入住' }
  return [publicConfig.shareCard.titlePrefix, ...publicConfig.shareCard.titleFields.filter(shareFieldEnabledForCustomer).map(key => examples[key])].filter(Boolean).join(' · ')
})
function toggleListingTitleField(key: string, enabled: boolean) {
  basicTagPreviewTab.value = 'list'
  if (key === 'community') return
  if (!publicConfig.listingTitle.customerAllowedFields.includes(key)) return
  const fields = publicConfig.listingTitle.enabledFields
  if (enabled && !fields.includes(key)) fields.push(key)
  if (!enabled) publicConfig.listingTitle.enabledFields = fields.filter(item => item !== key)
}
function toggleCustomerDetailField(key: string, enabled: boolean) {
  basicTagPreviewTab.value = 'detail'
  if (key === 'community') return
  if (!publicConfig.listingTitle.customerAllowedFields.includes(key)) return
  const fields = publicConfig.listingTitle.customerDetailFields
  if (enabled && !fields.includes(key)) fields.push(key)
  if (!enabled) publicConfig.listingTitle.customerDetailFields = fields.filter(item => item !== key)
}
function toggleCustomerShareField(key: string, enabled: boolean) {
  basicTagPreviewTab.value = 'share'
  if (key === 'community') return
  const fields = publicConfig.listingTitle.customerShareFields
  if (enabled && !fields.includes(key)) fields.push(key)
  if (!enabled) {
    publicConfig.listingTitle.customerShareFields = fields.filter(item => item !== key)
    const belongsToRemovedTag = (field: string) => shareFieldListingMap[field] === key
    publicConfig.shareCard.titleFields = publicConfig.shareCard.titleFields.filter(field => !belongsToRemovedTag(field))
    publicConfig.shareCard.posterFields = publicConfig.shareCard.posterFields.filter(field => !belongsToRemovedTag(field))
    if (!publicConfig.shareCard.titleFields.length) publicConfig.shareCard.titleFields.push('community')
    for (const fallback of ['rentMonthly', 'depositPayment', 'community', 'availableDate']) {
      if (publicConfig.shareCard.posterFields.length >= 3) break
      if (!publicConfig.shareCard.posterFields.includes(fallback)) publicConfig.shareCard.posterFields.push(fallback)
    }
  }
}
function toggleCustomerPosterField(key: string, enabled: boolean) {
  basicTagPreviewTab.value = 'poster'
  if (key === 'community') return
  const fields = publicConfig.listingTitle.customerPosterFields
  if (enabled && !fields.includes(key)) fields.push(key)
  if (!enabled) {
    publicConfig.listingTitle.customerPosterFields = fields.filter(item => item !== key)
    const belongsToRemovedTag = (field: string) => shareFieldListingMap[field] === key
    publicConfig.shareCard.posterFields = publicConfig.shareCard.posterFields.filter(field => !belongsToRemovedTag(field))
    for (const fallback of ['rentMonthly', 'depositPayment', 'community', 'availableDate']) {
      if (publicConfig.shareCard.posterFields.length >= 3) break
      if (!publicConfig.shareCard.posterFields.includes(fallback)) publicConfig.shareCard.posterFields.push(fallback)
    }
  }
}
function toggleCustomerAllowedListingField(key: string, enabled: boolean) {
  if (key === 'community') return
  if (key === 'unit' && enabled && !window.confirm('开启“门牌标签”后，客户视角可将精确门牌号作为展示标签，可能暴露住址信息。确认开启吗？')) return
  const fields = publicConfig.listingTitle.customerAllowedFields
  if (enabled && !fields.includes(key)) {
    fields.push(key)
  }
  if (!enabled) {
    publicConfig.listingTitle.customerAllowedFields = fields.filter(item => item !== key)
    publicConfig.listingTitle.enabledFields = publicConfig.listingTitle.enabledFields.filter(item => item !== key)
    publicConfig.listingTitle.customerDetailFields = publicConfig.listingTitle.customerDetailFields.filter(item => item !== key)
    publicConfig.listingTitle.customerShareFields = publicConfig.listingTitle.customerShareFields.filter(item => item !== key)
    publicConfig.listingTitle.customerPosterFields = publicConfig.listingTitle.customerPosterFields.filter(item => item !== key)
  }
}
function toggleAdminDoorplate(enabled: boolean) {
  publicConfig.listingTitle.adminEnabledFields = enabled ? ['unit'] : []
}

function addLocation() {
  if (publicConfig.locations.length >= 12 || publicConfig.featuredChannels.length >= 12) return
  publicConfig.locations.push({ id: createConfigId('loc'), label: '', keyword: '' })
  publicConfig.featuredChannels.push({ id: createConfigId('channel'), icon: 'commute', title: '', subtitle: '', keyword: '', latitude: 0, longitude: 0, radiusKm: 3 })
  activeChannelIndex.value = publicConfig.featuredChannels.length - 1
}

function removeLocation(index: number) {
  publicConfig.locations.splice(index, 1)
  if (publicConfig.featuredChannels[index]) publicConfig.featuredChannels.splice(index, 1)
  activeChannelIndex.value = Math.max(0, Math.min(activeChannelIndex.value, publicConfig.featuredChannels.length - 1))
}

function addChannel() {
  if (publicConfig.locations.length >= 12 || publicConfig.featuredChannels.length >= 12) return
  publicConfig.locations.push({ id: createConfigId('loc'), label: '', keyword: '' })
  publicConfig.featuredChannels.push({ id: createConfigId('channel'), icon: 'commute', title: '', subtitle: '', keyword: '', latitude: 0, longitude: 0, radiusKm: 3 })
  activeChannelIndex.value = publicConfig.featuredChannels.length - 1
}

function addMarketingPhrase() {
  if (publicConfig.listingTitle.marketingPhrases.length < 20) publicConfig.listingTitle.marketingPhrases.push('')
}

function removeMarketingPhrase(index: number) {
  publicConfig.listingTitle.marketingPhrases.splice(index, 1)
}

function moveCoverageItem(index: number, offset: -1 | 1) {
  moveItem(publicConfig.locations, index, offset)
  moveItem(publicConfig.featuredChannels, index, offset)
  activeChannelIndex.value = Math.max(0, Math.min(activeChannelIndex.value, publicConfig.featuredChannels.length - 1))
}

function syncChannelFromLocation(index: number) {
  const location = publicConfig.locations[index]
  const channel = publicConfig.featuredChannels[index]
  if (!location || !channel) return
  channel.title = location.label
  channel.keyword = location.keyword
}

function syncLocationFromChannel(index: number) {
  const channel = publicConfig.featuredChannels[index]
  const location = publicConfig.locations[index]
  if (!channel || !location) return
  location.label = channel.title
  location.keyword = channel.keyword
}

function clearChannelLocation(item: ChannelConfig, index: number) {
  item.latitude = 0
  item.longitude = 0
  channelLocationResults.value = { ...channelLocationResults.value, [index]: [] }
  channelSelectedLocation.value = { ...channelSelectedLocation.value, [index]: undefined }
  void loadChannelMapPreview(index, 30.45604, 114.425337)
}

function invalidateChannelLocation(item: ChannelConfig, index: number) {
  if (item.latitude || item.longitude) clearChannelLocation(item, index)
}

async function searchChannelLocation(item: ChannelConfig, index: number) {
  const keyword = (item.keyword || item.title).trim()
  if (keyword.length < 2) {
    pageError.value = '请先填写至少两个字的频道标题或搜索关键词'
    return
  }
  channelLocationLoading.value = { ...channelLocationLoading.value, [index]: true }
  try {
    const items = await api<LocationSearchItem[]>(`/v1/admin/location/search?keyword=${encodeURIComponent(keyword)}&latitude=30.45604&longitude=114.425337`)
    channelLocationResults.value = { ...channelLocationResults.value, [index]: items }
    void loadChannelMapPreview(index, item.latitude || 30.45604, item.longitude || 114.425337, items)
  } catch (error) {
    pageError.value = error instanceof Error ? error.message : '地点搜索失败'
  } finally {
    channelLocationLoading.value = { ...channelLocationLoading.value, [index]: false }
  }
}

function selectChannelLocation(item: ChannelConfig, index: number, location: LocationSearchItem) {
  item.latitude = Number(location.latitude.toFixed(7))
  item.longitude = Number(location.longitude.toFixed(7))
  channelLocationResults.value = { ...channelLocationResults.value, [index]: [] }
  channelMapActiveMarker.value = { ...channelMapActiveMarker.value, [index]: '' }
  channelSelectedLocation.value = { ...channelSelectedLocation.value, [index]: location }
  // Do not leave the multi-marker search image visible while the selected
  // location's single-pin map is being generated.
  channelMapPreview.value = { ...channelMapPreview.value, [index]: '' }
  void loadChannelMapPreview(index, item.latitude, item.longitude)
}

function mapPreviewCenter(latitude: number, longitude: number, points: LocationSearchItem[]) {
  if (!points.length) return { latitude, longitude }
  const total = points.reduce((result, point) => ({ latitude: result.latitude + point.latitude, longitude: result.longitude + point.longitude }), { latitude: 0, longitude: 0 })
  return { latitude: total.latitude / points.length, longitude: total.longitude / points.length }
}

function mapMarkerPositions(item: ChannelConfig, index: number) {
  const points = (channelLocationResults.value[index] || []).slice(0, 8)
  if (!points.length) return []
  const center = mapPreviewCenter(item.latitude || 30.45604, item.longitude || 114.425337, points)
  const zoom = points.length > 1 ? 12 : 14
  const worldSize = 256 * (2 ** zoom)
  const project = (latitude: number, longitude: number) => ({
    x: ((longitude + 180) / 360) * worldSize,
    y: (1 - Math.asinh(Math.tan(latitude * Math.PI / 180)) / Math.PI) / 2 * worldSize,
  })
  const centerPoint = project(center.latitude, center.longitude)
  return points.map((location, markerIndex) => {
    const point = project(location.latitude, location.longitude)
    return {
      location,
      label: String.fromCharCode(65 + markerIndex),
      left: 50 + ((point.x - centerPoint.x) / 620) * 100,
      top: 50 + ((point.y - centerPoint.y) / 260) * 100,
    }
  }).filter((marker) => marker.left > -3 && marker.left < 103 && marker.top > -8 && marker.top < 108)
}

function selectedRadiusStyle(item: ChannelConfig) {
  if (!item.latitude || !item.longitude) return {}
  const zoom = 14
  const metersPerPixel = (156543.03392 * Math.cos(item.latitude * Math.PI / 180)) / (2 ** zoom)
  const radiusPixels = Math.max(18, Math.min(310, (item.radiusKm * 1000) / metersPerPixel))
  return { width: `${radiusPixels * 2}px`, height: `${radiusPixels * 2}px` }
}

function toggleChannelMapMarker(index: number, locationID: string) {
  channelMapActiveMarker.value = { ...channelMapActiveMarker.value, [index]: channelMapActiveMarker.value[index] === locationID ? '' : locationID }
}

function selectChannelMapPoint(item: ChannelConfig, index: number, event: MouseEvent) {
  const target = event.currentTarget as HTMLButtonElement
  const bounds = target.getBoundingClientRect()
  if (!bounds.width || !bounds.height) return
  const points = channelLocationResults.value[index] || []
  const fallbackLatitude = item.latitude || 30.45604
  const fallbackLongitude = item.longitude || 114.425337
  const center = mapPreviewCenter(fallbackLatitude, fallbackLongitude, points)
  const zoom = points.length > 1 ? 12 : 14
  const worldSize = 256 * (2 ** zoom)
  const clickX = ((event.clientX - bounds.left) / bounds.width - 0.5) * 620
  const clickY = ((event.clientY - bounds.top) / bounds.height - 0.5) * 260
  const centerX = ((center.longitude + 180) / 360) * worldSize
  const centerLatitudeRadians = center.latitude * Math.PI / 180
  const centerY = (1 - Math.asinh(Math.tan(centerLatitudeRadians)) / Math.PI) / 2 * worldSize
  const longitude = ((centerX + clickX) / worldSize) * 360 - 180
  const mercatorY = centerY + clickY
  const latitude = Math.atan(Math.sinh(Math.PI * (1 - 2 * mercatorY / worldSize))) * 180 / Math.PI
  item.latitude = Number(latitude.toFixed(7))
  item.longitude = Number(longitude.toFixed(7))
  channelLocationResults.value = { ...channelLocationResults.value, [index]: [] }
  channelMapActiveMarker.value = { ...channelMapActiveMarker.value, [index]: '' }
  channelSelectedLocation.value = { ...channelSelectedLocation.value, [index]: { id: 'map-point', title: '地图选点', address: '', district: '', latitude: item.latitude, longitude: item.longitude } }
  channelMapPreview.value = { ...channelMapPreview.value, [index]: '' }
  void loadChannelMapPreview(index, item.latitude, item.longitude)
}

async function loadChannelMapPreview(index: number, latitude: number, longitude: number, points: LocationSearchItem[] = []) {
  const pointValue = points.slice(0, 8).map((point) => `${point.latitude.toFixed(6)},${point.longitude.toFixed(6)}`).join(';')
  const mapKey = `${latitude.toFixed(6)},${longitude.toFixed(6)}|${pointValue}`
  const requestID = (channelMapRequest.value[index] || 0) + 1
  channelMapRequest.value = { ...channelMapRequest.value, [index]: requestID }
  if (channelMapCache.value[mapKey]) {
    channelMapPreview.value = { ...channelMapPreview.value, [index]: channelMapCache.value[mapKey] }
    channelMapLoading.value = { ...channelMapLoading.value, [index]: false }
    return
  }
  const token = localStorage.getItem('xiaopi_admin_token')
  channelMapLoading.value = { ...channelMapLoading.value, [index]: true }
  try {
    const response = await fetch(`${apiBase}/v1/admin/location/static?latitude=${latitude}&longitude=${longitude}&points=${encodeURIComponent(pointValue)}`, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
    if (!response.ok) throw new Error('地图预览生成失败')
    const next = URL.createObjectURL(await response.blob())
    channelMapCache.value = { ...channelMapCache.value, [mapKey]: next }
    if (channelMapRequest.value[index] === requestID) channelMapPreview.value = { ...channelMapPreview.value, [index]: next }
  } catch (error) {
    if (channelMapRequest.value[index] === requestID) pageError.value = error instanceof Error ? error.message : '地图预览生成失败'
  } finally {
    if (channelMapRequest.value[index] === requestID) channelMapLoading.value = { ...channelMapLoading.value, [index]: false }
  }
}

function preloadChannelMaps() {
  publicConfig.featuredChannels.forEach((item, index) => {
    void loadChannelMapPreview(index, item.latitude || 30.45604, item.longitude || 114.425337)
  })
}

function removeChannel(index: number) {
  publicConfig.featuredChannels.splice(index, 1)
  if (publicConfig.locations[index]) publicConfig.locations.splice(index, 1)
  activeChannelIndex.value = Math.max(0, Math.min(activeChannelIndex.value, publicConfig.featuredChannels.length - 1))
}

function moveItem<T>(items: T[], index: number, offset: -1 | 1) {
  const target = index + offset
  if (target < 0 || target >= items.length) return
  const [item] = items.splice(index, 1)
  items.splice(target, 0, item)
}

function resetConfig() {
  if (!savedConfigJSON.value) return
  applyConfig(JSON.parse(savedConfigJSON.value))
  configMessage.value = '已恢复为最近一次发布的配置。'
  configFailed.value = false
}

async function saveConfig() {
  if (configErrors.value.length) {
    configFailed.value = true
    configMessage.value = configErrors.value[0]
    return
  }
  savingConfig.value = true
  configMessage.value = ''
  configFailed.value = false
  try {
    const value = await api<PublicConfig>('/v1/admin/config/public', { method: 'PUT', body: JSON.stringify(publicConfig) })
    applyConfig(value, true)
    configLastSavedAt.value = new Date().toLocaleString('zh-CN', { hour12: false })
    configMessage.value = '配置已发布，小程序重新进入后即可获取最新内容。'
  } catch (error) {
    configFailed.value = true
    configMessage.value = error instanceof Error ? error.message : '保存失败'
  } finally {
    savingConfig.value = false
  }
}

async function uploadPresentationBackground(key: string, event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  const form = new FormData()
  form.append('file', file)
  try {
    const token = localStorage.getItem('xiaopi_admin_token')
    const response = await fetch(`${apiBase}/v1/media/upload`, { method: 'POST', headers: token ? { Authorization: `Bearer ${token}` } : {}, body: form })
    const body = await response.json()
    if (!response.ok || body.code !== 0) throw new Error(body.message || '图片上传失败')
    publicConfig.pagePresentations[key].backgroundImage = body.data.url || body.data.path || ''
  } catch (error) {
    configFailed.value = true
    configMessage.value = error instanceof Error ? error.message : '图片上传失败'
  } finally {
    ;(event.target as HTMLInputElement).value = ''
  }
}

async function assignAdmin() {
  const phone = adminCandidate.value?.phone || ''
  if (!phone || assigningAdmin.value) return
  assigningAdmin.value = true
  adminMessage.value = ''
  adminFailed.value = false
  try {
    await api<AdminUser>('/v1/admin/admin-users', { method: 'POST', body: JSON.stringify({ phone }) })
    adminPhone.value = ''
    adminCandidate.value = null
    adminMessage.value = '已指定为管理员，该用户重新登录小程序后获得管理员权限。'
    admins.value = await api<AdminUser[]>('/v1/admin/admin-users')
  } catch (error) {
    adminFailed.value = true
    adminMessage.value = error instanceof Error ? error.message : '指定管理员失败'
  } finally {
    assigningAdmin.value = false
  }
}

function resetAdminSearch() {
  adminCandidate.value = null
  adminMessage.value = ''
  adminFailed.value = false
  adminSearchOpen.value = adminPhone.value.replace(/\D/g, '').length >= 2
}

const adminSearchResults = computed(() => {
  const query = adminPhone.value.replace(/\D/g, '')
  if (query.length < 2) return []
  return users.value.filter((item) => item.phone.replace(/\D/g, '').includes(query)).slice(0, 8)
})

function selectAdminCandidate(user: WechatUser) {
  adminCandidate.value = user
  adminPhone.value = user.phone
  adminSearchOpen.value = false
  adminFailed.value = false
  adminMessage.value = '已选择微信用户，请确认后指定管理员。'
}

const candidateIsAdmin = computed(() => Boolean(adminCandidate.value && admins.value.some((item) => item.userId === adminCandidate.value?.id)))

async function setAdminStatus(item: AdminUser, status: 'active' | 'disabled') {
  adminMessage.value = ''
  adminFailed.value = false
  try {
    await api('/v1/admin/admin-users/status', { method: 'PATCH', body: JSON.stringify({ userId: item.userId, status }) })
    item.status = status
    adminMessage.value = status === 'active' ? '管理员已启用。' : '管理员已停用。'
  } catch (error) {
    adminFailed.value = true
    adminMessage.value = error instanceof Error ? error.message : '管理员状态更新失败'
  }
}

async function deleteAdmin(item: AdminUser) {
  adminMessage.value = ''
  adminFailed.value = false
  try {
    await api(`/v1/admin/admin-users/${encodeURIComponent(item.userId)}`, { method: 'DELETE' })
    admins.value = admins.value.filter((admin) => admin.userId !== item.userId)
    adminMessage.value = '管理员身份已删除，微信用户账号仍然保留。'
  } catch (error) {
    adminFailed.value = true
    adminMessage.value = error instanceof Error ? error.message : '管理员删除失败'
  }
}

async function generateTemporaryAccess() {
  if (!temporaryPhone.value || generatingTemporaryCode.value) return
  generatingTemporaryCode.value = true
  temporaryMessage.value = ''
  temporaryFailed.value = false
  generatedTemporaryCode.value = ''
  try {
    const result = await api<{ access: TemporaryAccess; accessCode: string }>('/v1/admin/temporary-access', {
      method: 'POST',
      body: JSON.stringify({ phone: temporaryPhone.value, expiresInHours: temporaryHours.value }),
    })
    generatedTemporaryCode.value = result.accessCode
    temporaryMessage.value = '访问码已生成，可在下方访问记录中随时查看。'
    temporaryAccess.value = await api<TemporaryAccess[]>('/v1/admin/temporary-access')
    temporaryPhone.value = ''
  } catch (error) {
    temporaryFailed.value = true
    temporaryMessage.value = error instanceof Error ? error.message : '访问码生成失败'
  } finally {
    generatingTemporaryCode.value = false
  }
}

async function revokeTemporaryAccess(item: TemporaryAccess) {
  try {
    await api('/v1/admin/temporary-access/status', { method: 'PATCH', body: JSON.stringify({ phone: item.phone, status: 'revoked' }) })
    item.status = 'revoked'
  } catch (error) {
    temporaryFailed.value = true
    temporaryMessage.value = error instanceof Error ? error.message : '访问码撤销失败'
  }
}

onMounted(() => {
  if (loggedIn.value) load().catch(() => undefined)
  if (loggedIn.value && webAdminSuper.value && tab.value === '后台管理员') void refreshWebAccounts()
  void nextTick(() => {
    if (canvasPreviewStage.value) {
      canvasFitObserver = new ResizeObserver(updateCanvasFit)
      canvasFitObserver.observe(canvasPreviewStage.value)
    }
    window.addEventListener('resize', updateCanvasFit)
    updateCanvasFit()
  })
})
watch([tab, configPage], async () => {
  await nextTick()
  if (canvasFitObserver && canvasPreviewStage.value) canvasFitObserver.observe(canvasPreviewStage.value)
  updateCanvasFit()
}, { flush: 'post' })
onBeforeUnmount(() => {
  canvasFitObserver?.disconnect()
  window.removeEventListener('resize', updateCanvasFit)
})
</script>

<template>
  <main v-if="!loggedIn" class="login-page">
    <section class="login-panel surface">
      <div class="login-brand">
        <div class="login-mark">小</div>
        <span>小皮租房</span>
        <h1>运营管理后台</h1>
        <p>运营管理</p>
      </div>
      <el-form label-position="top" @submit.prevent="login">
        <el-form-item label="管理员账号"><el-input v-model.trim="username" autocomplete="username" :disabled="loading" /></el-form-item>
        <el-form-item label="管理员密码"><el-input v-model="password" type="password" autocomplete="current-password" show-password :disabled="loading" @keyup.enter="login" /></el-form-item>
        <el-alert v-if="loginError" :title="loginError" type="error" show-icon :closable="false" />
        <el-button type="primary" size="large" class="login-button" :loading="loading" :disabled="!username || !password" @click="login">登录后台</el-button>
      </el-form>
    </section>
  </main>

  <AdminLayout v-else :active="tab" :title="tab" @select="selectTab" @logout="logout">
        <el-alert v-if="mustChangePassword" class="page-error" title="当前账号仍在使用初始密码，建议在「后台管理员」中修改密码。" type="warning" show-icon @close="mustChangePassword = false" />
        <el-alert v-if="pageError" class="page-error" :title="pageError" type="error" show-icon :closable="false" />

        <DashboardPage v-if="tab === '运营总览'" :state="overviewState" :error-message="overviewError" :property-count="properties.length" :pending-lead-count="leads.filter(isPendingLead).length" :new-request-count="rentalRequests.filter(isNewRentalRequest).length" :favorite-count="favoriteContacts.length" :user-count="users.length" @navigate="openOverviewMetric" @refresh="refreshOverview" />

        <WechatUsersPage v-else-if="tab === '微信用户'" :users="users" :loading="loading" @refresh="load" />

        <RentalRequestsPage v-else-if="tab === '租客需求'" :requests="rentalRequests" :loading="loading" :filter="requestFilter" @refresh="load" @filter="value => router.replace({ query: value === 'new' ? { filter: value } : {} })" />

        <IntentionsPage v-else-if="tab === '用户意向'" :contacts="favoriteContacts" :loading="loading" @refresh="load" />

        <template v-else-if="tab === '后台管理员'">
          <section class="page-header">
            <div><span>后台账号</span><h1>后台管理员</h1><p>用于登录 Web 运营后台。</p></div>
          </section>
          <el-alert v-if="webAccountMessage && !webAccountDialog" class="config-alert" :title="webAccountMessage" type="error" show-icon />
          <section class="surface table-panel">
            <div class="table-heading">
              <div><h2>后台账号列表</h2><p>{{ webAdminSuper ? '管理可登录 Web 运营后台的账号' : '账号列表由超级管理员管理' }}</p></div>
              <div>
                <el-button @click="openWebAccountDialog('self')">修改我的密码</el-button>
                <el-button v-if="webAdminSuper" @click="refreshWebAccounts">刷新</el-button>
                <el-button v-if="webAdminSuper" type="primary" @click="openWebAccountDialog('create')">新增账号</el-button>
              </div>
            </div>
            <template v-if="webAdminSuper">
              <el-table :data="webAccounts" v-loading="webAccountBusy" :empty-text="webAccountMessage ? '账号加载失败，请刷新重试' : '暂无后台账号'">
                <el-table-column prop="username" label="账号" />
                <el-table-column label="角色"><template #default="{ row }">{{ row.super ? '超级管理员' : '后台管理员' }}</template></el-table-column>
                <el-table-column label="状态"><template #default="{ row }">{{ !row.enabled ? '已停用' : row.mustChangePassword ? '待修改密码' : '已启用' }}</template></el-table-column>
                <el-table-column label="操作" min-width="240"><template #default="{ row }">
                  <template v-if="!row.super">
                    <el-button link @click="updateWebAccount(row, 'PATCH', { enabled: !row.enabled })">{{ row.enabled ? '停用' : '启用' }}</el-button>
                    <el-button link type="primary" @click="openWebAccountDialog('reset', row)">重置密码</el-button>
                    <el-popconfirm title="确认删除该后台账号？" @confirm="updateWebAccount(row, 'DELETE')"><template #reference><el-button link type="danger">删除</el-button></template></el-popconfirm>
                  </template>
                </template></el-table-column>
              </el-table>
            </template>
          </section>
          <el-dialog :model-value="!!webAccountDialog" :title="webAccountDialog === 'create' ? '新增后台账号' : webAccountDialog === 'reset' ? '重置密码 · ' + webAccountTarget?.username : '修改我的密码'" width="min(440px, 92vw)" @update:model-value="webAccountDialog = ''">
            <el-form label-position="top" @submit.prevent="webAccountDialog === 'create' ? createWebAccount() : webAccountDialog === 'self' ? changeWebPassword() : webAccountTarget && updateWebAccount(webAccountTarget, 'PATCH', { password: webAccountPassword })">
              <el-form-item v-if="webAccountDialog === 'create'" label="账号"><el-input v-model.trim="webAccountName" /></el-form-item>
              <el-form-item v-if="webAccountDialog === 'self'" label="新密码"><el-input v-model="newWebPassword" type="password" autocomplete="new-password" /></el-form-item>
              <el-form-item v-else :label="webAccountDialog === 'create' ? '初始密码' : '新密码'"><el-input v-model="webAccountPassword" type="password" autocomplete="new-password" /></el-form-item>
              <el-alert v-if="webAccountMessage" class="config-alert" :title="webAccountMessage" type="error" show-icon />
              <el-button @click="webAccountDialog = ''">取消</el-button>
              <el-button native-type="submit" type="primary" :loading="webAccountBusy" :disabled="webAccountDialog === 'self' ? !newWebPassword : !webAccountPassword || (webAccountDialog === 'create' && !webAccountName)">确认</el-button>
            </el-form>
          </el-dialog>
        </template>

        <template v-else-if="tab === '小程序管理员'">
          <section class="page-header">
            <div><span>小程序权限</span><h1>小程序管理员</h1></div>
          </section>
          <section class="surface admin-search-panel">
            <div class="admin-search-heading"><h2>搜索微信用户</h2><p>只有已经出现在“微信用户”列表并绑定手机号的用户可以成为管理员。</p></div>
            <div class="admin-search-control">
              <el-input v-model.trim="adminPhone" placeholder="输入手机号任意连续数字，例如后四位" clearable @input="resetAdminSearch" @focus="adminSearchOpen = adminPhone.replace(/\D/g, '').length >= 2" />
              <div v-if="adminSearchOpen" class="admin-search-results">
                <button v-for="item in adminSearchResults" :key="item.id" type="button" @click="selectAdminCandidate(item)">
                  <span><strong>{{ item.nickname || '微信用户' }}</strong><small>{{ item.phone }}</small></span>
                  <el-tag v-if="admins.some((admin) => admin.userId === item.id)" size="small" type="success">已是管理员</el-tag>
                  <small v-else>选择</small>
                </button>
                <div v-if="adminSearchResults.length === 0" class="no-search-result">没有匹配的已绑定手机号用户</div>
              </div>
            </div>
            <div v-if="adminCandidate" class="admin-candidate">
              <div class="user-cell"><el-avatar :size="44" :src="adminCandidate.avatarUrl">微</el-avatar><div><strong>{{ adminCandidate.nickname || '微信用户' }}</strong><span>{{ adminCandidate.phone }} · {{ adminCandidate.id }}</span></div></div>
              <div class="candidate-actions"><el-tag v-if="candidateIsAdmin" type="success">已是管理员</el-tag><el-button v-else type="primary" :loading="assigningAdmin" @click="assignAdmin">指定为管理员</el-button></div>
            </div>
          </section>
          <el-alert v-if="adminMessage" class="config-alert" :title="adminMessage" :type="adminFailed ? 'error' : 'success'" show-icon :closable="false" />
          <section class="surface table-panel">
            <div class="table-heading"><div><h2>已指定管理员</h2><p>共 {{ admins.length }} 人</p></div></div>
            <el-table :data="admins" empty-text="暂无微信管理员">
              <el-table-column label="管理员" min-width="220"><template #default="scope"><div class="user-cell"><el-avatar :size="38" :src="scope.row.avatarUrl">管</el-avatar><div><strong>{{ scope.row.nickname || '微信用户' }}</strong><span>{{ scope.row.userId }}</span></div></div></template></el-table-column>
              <el-table-column prop="phone" label="手机号" min-width="150" />
              <el-table-column label="角色" width="120"><template #default><el-tag>运营管理员</el-tag></template></el-table-column>
              <el-table-column label="状态" width="120"><template #default="scope"><el-tag :type="scope.row.status === 'active' ? 'success' : 'info'">{{ scope.row.status === 'active' ? '已启用' : '已停用' }}</el-tag></template></el-table-column>
              <el-table-column label="指定时间" min-width="180"><template #default="scope">{{ formatTime(scope.row.createdAt) }}</template></el-table-column>
              <el-table-column label="操作" width="170" align="center"><template #default="scope"><div class="admin-actions"><el-button v-if="scope.row.status !== 'active'" link type="primary" @click="setAdminStatus(scope.row, 'active')">启用</el-button><el-popconfirm v-else title="停用后，该用户重新登录将失去管理员权限，确认停用？" width="260" @confirm="setAdminStatus(scope.row, 'disabled')"><template #reference><el-button link type="warning">停用</el-button></template></el-popconfirm><el-popconfirm title="删除后该用户将移出小程序管理员列表，但微信用户账号保留。确认删除？" width="280" @confirm="deleteAdmin(scope.row)"><template #reference><el-button link type="danger">删除</el-button></template></el-popconfirm></div></template></el-table-column>
            </el-table>
          </section>
        </template>

        <template v-else-if="tab === '临时访问码'">
          <section class="page-header">
            <div><span>小程序权限</span><h1>临时访问码</h1></div>
          </section>
          <section class="surface temporary-access-panel">
            <div class="temporary-form">
              <el-select v-model="temporaryPhone" filterable clearable placeholder="搜索已绑定手机号的微信用户"><el-option v-for="item in users.filter(user => user.phone && !user.id.startsWith('tmp-'))" :key="item.id" :label="`${item.nickname || '微信用户'} · ${item.phone}`" :value="item.phone" /></el-select>
              <el-input-number v-model="temporaryHours" :min="1" :max="720" controls-position="right" />
              <span class="hours-unit">小时有效</span>
              <el-button type="primary" :loading="generatingTemporaryCode" :disabled="!temporaryPhone" @click="generateTemporaryAccess">生成访问码</el-button>
            </div>
            <div v-if="generatedTemporaryCode" class="generated-code"><span>本次访问码</span><strong>{{ generatedTemporaryCode }}</strong><small>测试阶段会同时保留在下方访问记录中。</small></div>
          </section>
          <el-alert v-if="temporaryMessage" class="config-alert" :title="temporaryMessage" :type="temporaryFailed ? 'error' : 'success'" show-icon :closable="false" />
          <section class="surface table-panel">
            <div class="table-heading"><div><h2>访问码记录</h2><p>重新生成会立即替换同一手机号的旧访问码</p></div></div>
            <el-table :data="temporaryAccess" empty-text="暂无临时访问码">
              <el-table-column prop="phone" label="手机号" min-width="160" />
              <el-table-column label="访问码" min-width="170"><template #default="scope"><strong v-if="scope.row.accessCode" class="access-code-cell">{{ scope.row.accessCode }}</strong><span v-else class="unavailable-code">历史记录，需重新生成</span></template></el-table-column>
              <el-table-column label="状态" width="120"><template #default="scope"><el-tag :type="scope.row.status === 'active' && new Date(scope.row.expiresAt) > new Date() ? 'success' : 'info'">{{ scope.row.status === 'revoked' ? '已撤销' : new Date(scope.row.expiresAt) <= new Date() ? '已过期' : '有效' }}</el-tag></template></el-table-column>
              <el-table-column label="有效期至" min-width="180"><template #default="scope">{{ formatTime(scope.row.expiresAt) }}</template></el-table-column>
              <el-table-column prop="createdBy" label="创建人" min-width="140" />
              <el-table-column label="操作" width="100" align="center"><template #default="scope"><el-popconfirm v-if="scope.row.status === 'active'" title="确认撤销该手机号的访问码？" @confirm="revokeTemporaryAccess(scope.row)"><template #reference><el-button link type="danger">撤销</el-button></template></el-popconfirm><span v-else>-</span></template></el-table-column>
            </el-table>
          </section>
        </template>

        <PropertiesPage v-else-if="tab === '房源列表'" :properties="properties" :filtered-properties="filteredProperties" :filter="propertyFilter" :statuses="propertyStatuses" :loading="loading" :normalized-status="normalizedPropertyStatus" :status-label="statusLabel" :cover="propertyCover" @refresh="load" @filter="propertyFilter = $event" @status="setPropertyStatus" />

        <template v-else-if="tab === '小程序基础配置'">
          <div class="miniapp-config-shell basic-config-shell">
          <div class="miniapp-config-main">
          <section class="page-header">
            <div><span>内容与运营</span><h1>小程序基础配置</h1><p>配置所有页面共用的数据、可选项与角色展示边界。</p></div>
          </section>
          <section class="surface config-toolbar">
            <div class="publish-state"><i :class="{ dirty: configDirty }" /><div><strong>{{ configDirty ? '存在未发布修改' : '线上配置已同步' }}</strong><span>基础配置会同步到录入页、详情页和页面画布。</span></div></div>
            <div class="config-toolbar-actions"><el-button :icon="RefreshLeft" :disabled="!configDirty || savingConfig" @click="resetConfig">恢复</el-button><el-button type="primary" :loading="savingConfig" :disabled="!configDirty || configErrors.length > 0" @click="saveConfig">保存并发布</el-button></div>
          </section>
          <el-alert v-if="configMessage" class="config-alert" :title="configMessage" :type="configFailed ? 'error' : 'success'" show-icon :closable="false" />
          <section class="surface basic-config-tabs" aria-label="小程序基础配置分类">
            <button type="button" :class="{ active: basicConfigTab === 'brand' }" @click="selectBasicConfigTab('brand')">品牌与联系</button>
            <button type="button" :class="{ active: basicConfigTab === 'coverage' }" @click="selectBasicConfigTab('coverage')">地区与关联范围</button>
            <button type="button" :class="{ active: basicConfigTab === 'tags' }" @click="selectBasicConfigTab('tags')">房源标签规则</button>
            <button type="button" :class="{ active: basicConfigTab === 'facilities' }" @click="selectBasicConfigTab('facilities')">房屋设施库</button>
          </section>
          <section v-if="basicConfigTab === 'brand'" class="surface form-section basic-config-panel">
            <div class="section-heading"><span>01</span><div><h2>品牌与联系</h2><p>全局品牌名和联系方式，不属于某一个页面。</p></div></div>
            <el-form data-config-field="brand-name" label-position="top" class="basic-config-form"><div class="form-row two-columns"><el-form-item label="品牌名称" required><el-input v-model="publicConfig.heroTitle" maxlength="20" show-word-limit placeholder="例如：小皮租房" /><div class="form-tip">统一用于小程序顶部导航、微信登录、房源详情服务入口与分享落款。</div></el-form-item><el-form-item label="服务联系电话"><el-input v-model.trim="publicConfig.contactPhone" maxlength="20" placeholder="用于房源详情联系" /><div class="form-tip">统一用于房源详情“打电话”和找房需求页的电话入口；不影响在线客服。</div></el-form-item></div></el-form>
            <el-form data-config-field="status-labels" label-position="top" class="basic-config-form"><div class="form-section-title">房源状态显示标签</div><div class="form-row three-columns"><el-form-item label="草稿"><el-input v-model="publicConfig.statusLabels.draft" /></el-form-item><el-form-item label="待审核"><el-input v-model="publicConfig.statusLabels.pendingReview" /></el-form-item><el-form-item label="已发布"><el-input v-model="publicConfig.statusLabels.published" /></el-form-item><el-form-item label="可租"><el-input v-model="publicConfig.statusLabels.available" /></el-form-item><el-form-item label="已租"><el-input v-model="publicConfig.statusLabels.rented" /></el-form-item><el-form-item label="已下架"><el-input v-model="publicConfig.statusLabels.offline" /></el-form-item></div><div class="form-tip">这是数据标签级配置，只改显示名称；不会改变房源状态流转。直租/转租与整租/合租仍是两套独立业务字段。</div></el-form>
            <div class="basic-config-note"><b>字段边界</b><span>品牌名称是对外品牌文本；在线客服人员由微信小程序客服配置管理，不在此设置。普通用户与管理员的房源标签规则在“房源标签规则”中单独管理。</span></div>
          </section>
          <section v-else-if="basicConfigTab === 'coverage'" data-config-field="coverage" class="surface form-section basic-config-panel">
            <div class="section-heading"><span>02</span><div><h2>地区与关联范围</h2><p>用于频道归属、房源附近推荐和默认筛选；页面文案与背景仍在“页面配置”里管理。</p></div></div>
            <el-form label-position="top" class="basic-config-form"><el-form-item label="首页附近 · 全局默认距离"><el-select v-model="publicConfig.relatedPropertyRadiusKm" style="width:100%"><el-option v-for="distance in [1, 2, 3, 5, 10]" :key="distance" :label="`${distance} 公里`" :value="distance" /></el-select><div class="form-tip">只作为首页“附近”的默认范围；频道地图和详情页推荐各自有独立范围，不会被这里覆盖。</div></el-form-item></el-form>
            <div class="basic-config-note"><b>三类范围，互不覆盖</b><span>首页附近使用全局默认距离；每个精选频道使用自己的地图关联半径；房源详情的附近推荐在“页面配置 → 房源详情”单独设置。地区名称、频道地图和排序仍在这里维护。</span></div>
            <div class="coverage-data-tabs" role="tablist" aria-label="地区与频道数据切换"><button type="button" :class="{ active: coverageDataTab === 'locations' }" @click="coverageDataTab = 'locations'">地区入口数据 <small>{{ publicConfig.locations.length }} 项</small></button><button type="button" :class="{ active: coverageDataTab === 'channels' }" @click="coverageDataTab = 'channels'">精选频道数据 <small>{{ publicConfig.featuredChannels.length }} 项</small></button></div>
            <div v-if="coverageDataTab === 'locations'" class="basic-coverage-editor">
              <div class="section-heading config-section-heading"><span>03</span><div><h2>地区入口数据</h2><p>与下方精选频道共用名称、关键词和排序；新增、删除、上移、下移会同步处理。</p></div><el-button type="primary" plain :icon="Plus" :disabled="publicConfig.locations.length >= 12" @click="addLocation">新增地区</el-button></div>
              <div v-if="publicConfig.locations.length" class="structured-list">
                <article v-for="(item, index) in publicConfig.locations" :key="`basic-location-${index}`" class="config-list-item"><div class="item-index">{{ String(index + 1).padStart(2, '0') }}</div><div class="item-fields location-fields"><label><span>显示名称</span><el-input v-model="item.label" maxlength="12" placeholder="例如：光谷" @input="syncChannelFromLocation(index)" /></label><label><span>搜索关键词</span><el-input v-model="item.keyword" maxlength="20" placeholder="例如：光谷" @input="syncChannelFromLocation(index)" /></label></div><div class="item-actions"><el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="上移" @click="moveCoverageItem(index, -1)" /><el-button text :icon="ArrowDown" :disabled="index === publicConfig.locations.length - 1" aria-label="下移" @click="moveCoverageItem(index, 1)" /><el-popconfirm title="确认删除这个地区入口和对应频道？" @confirm="removeLocation(index)"><template #reference><el-button text type="danger" :icon="Delete" aria-label="删除" /></template></el-popconfirm></div></article>
              </div>
              <div v-else class="config-empty"><strong>还没有地理位置</strong><span>至少添加一个地区，用户才能从首页快速筛选房源。</span><el-button type="primary" plain :icon="Plus" @click="addLocation">添加第一个地区</el-button></div>
            </div>
            <div v-else class="basic-coverage-editor">
              <div class="section-heading config-section-heading"><span>03</span><div><h2>精选频道数据</h2><p>与地区入口共用名称、关键词和排序；这里维护当前频道的地图中心、专属半径、卡片样式和副标题。</p></div><el-button type="primary" plain :icon="Plus" :disabled="publicConfig.featuredChannels.length >= 12" @click="addChannel">新增频道</el-button></div>
              <div v-if="publicConfig.featuredChannels.length" class="channel-editor-tabs"><button v-for="(item, index) in publicConfig.featuredChannels" :key="`basic-channel-tab-${index}`" type="button" :class="{ active: activeChannelIndex === index, configured: item.latitude && item.longitude }" @click="activeChannelIndex = index"><span>{{ index + 1 }}</span><strong>{{ item.title || `频道 ${index + 1}` }}</strong><small>{{ channelSetupStatus(item) }}</small></button></div>
              <div v-if="publicConfig.featuredChannels.length" class="channel-editor-grid">
                <article v-for="(item, index) in publicConfig.featuredChannels" v-show="activeChannelIndex === index" :key="`basic-channel-${index}`" class="channel-editor-card active">
                  <header><div class="channel-order"><span>频道数据</span><div><strong>频道 {{ index + 1 }}</strong><small>按“基础信息 → 地点 → 范围”完成配置</small></div></div><div class="item-actions"><el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="频道上移" @click="moveCoverageItem(index, -1)" /><el-button text :icon="ArrowDown" :disabled="index === publicConfig.featuredChannels.length - 1" aria-label="频道下移" @click="moveCoverageItem(index, 1)" /><el-popconfirm title="确认删除这个精选频道和对应地区？" @confirm="removeChannel(index)"><template #reference><el-button text type="danger" :icon="Delete" aria-label="删除" /></template></el-popconfirm></div></header>
                  <div class="channel-flow"><span class="active">1 基础信息</span><i>→</i><span :class="{ active: item.latitude && item.longitude }">2 地点定位</span><i>→</i><span :class="{ active: item.latitude && item.longitude && item.radiusKm > 0 }">3 关联范围</span></div>
                  <div class="channel-fields"><label><span>频道名称</span><el-input v-model="item.title" maxlength="12" placeholder="例如：金融港" @input="syncLocationFromChannel(index); invalidateChannelLocation(item, index)" /></label><label><span>搜索关键词（未选地点时使用）</span><el-input v-model="item.keyword" maxlength="20" placeholder="点击后筛选的关键词" @input="syncLocationFromChannel(index); invalidateChannelLocation(item, index)" /></label></div>
                </article>
              </div>
              <section v-if="activeChannel" class="channel-map-workbench"><header><div><strong>{{ activeChannel.title || `频道 ${activeChannelIndex + 1}` }} · 地图定位</strong><span>搜索结果、地图选点与中心坐标集中在这里处理。</span></div><div class="channel-location-action"><el-button :loading="channelLocationLoading[activeChannelIndex]" @click="searchChannelLocation(activeChannel, activeChannelIndex)">{{ activeChannel.latitude && activeChannel.longitude ? '重新搜索地点' : '搜索地点' }}</el-button><el-button v-if="activeChannel.latitude && activeChannel.longitude" text type="danger" @click="clearChannelLocation(activeChannel, activeChannelIndex)">清除位置</el-button></div></header><div v-if="channelMapLoading[activeChannelIndex]" class="channel-map-loading">正在加载已选地点…</div><div v-else-if="channelMapPreview[activeChannelIndex]" class="channel-map-picker" title="点击地图设置频道中心" @click="selectChannelMapPoint(activeChannel, activeChannelIndex, $event)"><img class="channel-map-preview" :src="channelMapPreview[activeChannelIndex]" alt="频道范围地图预览，点击可选点" /><div v-if="activeChannel.latitude && activeChannel.longitude && !channelLocationResults[activeChannelIndex]?.length" class="channel-radius-ring" :style="selectedRadiusStyle(activeChannel)"></div><button v-if="activeChannel.latitude && activeChannel.longitude && !channelLocationResults[activeChannelIndex]?.length" class="channel-selected-map-pin" type="button" :aria-label="`${channelSelectedLocation[activeChannelIndex]?.title || '频道中心'}，已选`" @click.stop><span>●</span></button><template v-for="marker in mapMarkerPositions(activeChannel, activeChannelIndex)" :key="marker.location.id"><button class="channel-map-marker" :class="{ active: channelMapActiveMarker[activeChannelIndex] === marker.location.id }" type="button" :style="{ left: `${marker.left}%`, top: `${marker.top}%` }" :aria-label="marker.location.title" @click.stop="toggleChannelMapMarker(activeChannelIndex, marker.location.id)"><span>{{ marker.label }}</span></button><div v-if="channelMapActiveMarker[activeChannelIndex] === marker.location.id" class="channel-map-callout" :style="{ left: `${marker.left}%`, top: `${marker.top}%` }" @click.stop><strong>{{ marker.location.title }}</strong><span>{{ marker.location.district }} · {{ marker.location.address }}</span><button type="button" @click="selectChannelLocation(activeChannel, activeChannelIndex, marker.location)">设为频道中心</button></div></template></div><small v-if="activeChannel.latitude && activeChannel.longitude">已选中心：{{ channelSelectedLocation[activeChannelIndex]?.title || '地图选点' }} · {{ activeChannel.latitude.toFixed(6) }}, {{ activeChannel.longitude.toFixed(6) }} · 半径 {{ activeChannel.radiusKm }} 公里</small><small v-else>点击地图可直接选点；搜索结果会以大头钉显示在图上。</small><div v-if="channelLocationResults[activeChannelIndex]?.length" class="admin-search-results channel-location-results"><button v-for="(location, locationIndex) in channelLocationResults[activeChannelIndex]" :key="location.id" type="button" @click="selectChannelLocation(activeChannel, activeChannelIndex, location)"><span><strong>{{ String.fromCharCode(65 + locationIndex) }} · {{ location.title }}</strong><small>{{ location.district }} · {{ location.address }}</small></span><em>{{ location.distance ? `${(location.distance / 1000).toFixed(1)}km` : '选用' }}</em></button></div><label class="channel-radius-field"><span>地图关联半径</span><el-input-number v-model="activeChannel.radiusKm" :min="0.1" :max="100" :precision="1" :step="0.5" controls-position="right" /><small>保存后，中心点半径内的房源会自动关联到此频道。</small></label></section>
              <div v-else class="config-empty"><strong>还没有精选频道</strong><span>添加频道后，用户可以从首页直接进入对应房源列表。</span><el-button type="primary" plain :icon="Plus" @click="addChannel">添加第一个频道</el-button></div>
            </div>
          </section>
          <section v-else-if="basicConfigTab === 'tags'" data-config-field="tags" class="surface form-section basic-config-panel">
            <div class="section-heading"><span>03</span><div><h2>房源标签规则</h2><p>这里是标签配置唯一入口：首页推荐、房源列表、房源详情与微信分享卡均读取本规则。</p></div></div>
            <div class="share-field-editor listing-title-editor marketing-phrase-editor">
              <header><strong>客户广告短语库</strong><span>客户标题为“小区名 · 广告词”。录入房源时只从此处选择；广告词不是标签、筛选条件或业务数据，管理员视角不显示。</span></header>
              <div class="marketing-phrase-list">
                <label v-for="(phrase, index) in publicConfig.listingTitle.marketingPhrases" :key="`marketing-phrase-${index}`"><el-input v-model="publicConfig.listingTitle.marketingPhrases[index]" maxlength="16" placeholder="例如：好房快抢" /><el-button text type="danger" @click="removeMarketingPhrase(index)">删除</el-button></label>
              </div>
              <el-button text type="primary" :disabled="publicConfig.listingTitle.marketingPhrases.length >= 20" @click="addMarketingPhrase">＋ 添加广告短语</el-button>
            </div>
            <div class="share-field-editor listing-title-editor">
              <header><strong>第一步：开放给客户视角的可选标签</strong><span>管理员在此定义客户可选择哪些公开字段；开启门牌会出现住址暴露提醒。</span></header>
              <div class="listing-title-preview"><span>客户固定主标题</span><strong>小区名称 + 已选择的广告短语（不作为可勾选标签）</strong></div>
              <div class="share-field-options"><button v-for="field in listingTitleFieldOptions.filter(item => item.key !== 'community')" :key="`basic-customer-allowed-${field.key}`" type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.customerAllowedFields.includes(field.key) }" :aria-pressed="publicConfig.listingTitle.customerAllowedFields.includes(field.key)" @click="toggleCustomerAllowedListingField(field.key, !publicConfig.listingTitle.customerAllowedFields.includes(field.key))"><i>{{ publicConfig.listingTitle.customerAllowedFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>
              <div class="listing-title-preview"><span>客户可配置范围</span><strong>{{ publicConfig.listingTitle.customerAllowedFields.filter(key => key !== 'community').map(key => listingTitleFieldOptions.find(field => field.key === key)?.label).filter(Boolean).join('、') || '仅显示小区主标题' }}</strong></div>
            </div>
            <div class="share-field-editor listing-title-editor">
              <header><strong>第二步：客户公开展示标签</strong><span>小区名称固定作为主标题。以下每一处都只显示第一步已开放的标签，可独立做减法。</span></header>
              <div class="listing-title-preview"><span>固定标题</span><strong>小区名称 + 广告语</strong></div>
              <header class="detail-subheading"><strong>首页推荐与房源列表</strong><span>控制客户首页推荐卡和“房源”页列表中的标签。</span></header>
              <div class="share-field-options"><button v-for="field in listingTitleFieldOptions.filter(item => item.key !== 'community' && publicConfig.listingTitle.customerAllowedFields.includes(item.key))" :key="`basic-customer-list-${field.key}`" type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.enabledFields.includes(field.key) }" :aria-pressed="publicConfig.listingTitle.enabledFields.includes(field.key)" @click="toggleListingTitleField(field.key, !publicConfig.listingTitle.enabledFields.includes(field.key))"><i>{{ publicConfig.listingTitle.enabledFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>
              <header class="detail-subheading"><strong>房源详情</strong><span>控制客户在房源详情页标题下方看到的标签。</span></header>
              <div class="share-field-options"><button v-for="field in listingTitleFieldOptions.filter(item => item.key !== 'community' && publicConfig.listingTitle.customerAllowedFields.includes(item.key))" :key="`basic-customer-detail-${field.key}`" type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.customerDetailFields.includes(field.key) }" :aria-pressed="publicConfig.listingTitle.customerDetailFields.includes(field.key)" @click="toggleCustomerDetailField(field.key, !publicConfig.listingTitle.customerDetailFields.includes(field.key))"><i>{{ publicConfig.listingTitle.customerDetailFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>
              <header class="detail-subheading"><strong>微信好友分享卡</strong><span>控制微信好友转发时，小程序分享卡可使用的标签；标题字段仍可在“房源详情 → 分享卡”单独排版。</span></header>
              <div class="share-field-options"><button v-for="field in listingTitleFieldOptions.filter(item => item.key !== 'community' && publicConfig.listingTitle.customerAllowedFields.includes(item.key))" :key="`basic-customer-share-${field.key}`" type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.customerShareFields.includes(field.key) }" :aria-pressed="publicConfig.listingTitle.customerShareFields.includes(field.key)" @click="toggleCustomerShareField(field.key, !publicConfig.listingTitle.customerShareFields.includes(field.key))"><i>{{ publicConfig.listingTitle.customerShareFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>
            </div>
            <div class="share-field-editor listing-title-editor">
              <header><strong>管理员视角的唯一差异</strong><span>管理员与客户复用公开标签；只可额外显示精确门牌，列表和详情同步生效。</span></header>
              <div class="share-field-options"><button type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.adminEnabledFields.includes('unit') }" :aria-pressed="publicConfig.listingTitle.adminEnabledFields.includes('unit')" @click="toggleAdminDoorplate(!publicConfig.listingTitle.adminEnabledFields.includes('unit'))"><i>{{ publicConfig.listingTitle.adminEnabledFields.includes('unit') ? '✓' : '' }}</i><span>额外显示门牌</span></button></div>
              <div class="listing-title-preview"><span>管理员最终效果</span><strong>{{ publicConfig.listingTitle.adminEnabledFields.includes('unit') ? '客户公开标签 + 5栋3单元101室' : '与客户公开标签完全一致' }}</strong></div>
            </div>
          </section>
          <section v-else data-config-field="facilities" class="surface form-section basic-config-panel facility-library-panel">
            <div class="section-heading"><span>03</span><div><h2>房屋设施库</h2><p>点击整张卡片启用或停用。启用项会进入管理员录入页的设施勾选，并可在房源详情展示；停用不删除历史房源数据。</p></div></div>
            <div class="facility-library-summary"><b>已启用 {{ publicConfig.propertyDetail.facilities.filter(item => item.enabled).length }} / {{ publicConfig.propertyDetail.facilities.length }} 项</b><span>设施名称与分类仅在点击“编辑”后修改，避免把选择操作误当成输入框。</span></div>
            <div class="facility-config-list facility-library-list">
              <article v-for="facility in publicConfig.propertyDetail.facilities" :key="facility.key" class="facility-choice" :class="{ checked: facility.enabled, editing: editingFacilityKey === facility.key }" @click="toggleFacility(facility)">
                <button type="button" class="facility-check" :aria-pressed="facility.enabled" @click.stop="toggleFacility(facility)">{{ facility.enabled ? '✓' : '' }}</button>
                <i class="facility-option-icon">{{ detailFacilityIcon(facility.key) }}</i>
                <span><strong>{{ facility.label }}</strong><small>{{ facility.group }}</small></span>
                <button type="button" class="facility-edit" @click.stop="editFacility(facility)">{{ editingFacilityKey === facility.key ? '收起' : '编辑' }}</button>
                <el-popconfirm title="移除后可从预置列表重新添加，确认移除？" @confirm="removeDetailFacility(facility.key)"><template #reference><button type="button" class="facility-edit facility-remove" @click.stop>移除</button></template></el-popconfirm>
                <div v-if="editingFacilityKey === facility.key" class="facility-edit-fields" @click.stop><el-input v-model.trim="facility.label" maxlength="12" aria-label="设施名称" placeholder="设施名称" /><el-input v-model.trim="facility.group" maxlength="12" aria-label="设施分类" placeholder="设施分类" /></div>
              </article>
            </div>
            <div class="facility-add-row"><span>添加预置设施</span><el-select v-model="facilityToAdd" placeholder="从预置列表选择" :disabled="!availableFacilityCatalog.length || publicConfig.propertyDetail.facilities.length >= 24" @change="addDetailFacility"><el-option v-for="facility in availableFacilityCatalog" :key="facility.key" :label="`${facility.label} · ${facility.group}`" :value="facility.key" /></el-select></div>
          </section>
          </div>
          <aside class="config-preview-column basic-config-preview-column" aria-label="小程序基础配置效果预览">
            <div class="basic-preview-heading"><strong>实时效果</strong><span>{{ basicConfigTab === 'brand' ? '品牌与联系' : basicConfigTab === 'coverage' ? '地区与频道' : basicConfigTab === 'tags' ? '房源卡片标签' : '房屋设施' }}</span></div>
            <div class="phone-preview basic-phone-preview">
              <div class="phone-status"><span>9:41</span><span>● ● ●</span></div>
              <div class="mini-navigation">{{ publicConfig.heroTitle || '小皮租房' }}</div>
              <div class="basic-preview-content">
                <template v-if="basicConfigTab === 'brand'">
                  <div class="basic-preview-usage"><span>① 顶部导航 · 品牌名称</span><div class="basic-preview-nav canvas-editable-copy" title="点击编辑品牌名称" @click="focusConfigField('brand-name')">{{ publicConfig.heroTitle || '小皮租房' }}</div></div>
                  <div class="basic-preview-usage"><span>② 微信登录 · 品牌名称</span><div class="basic-preview-login canvas-editable-copy" title="点击编辑品牌名称" @click="focusConfigField('brand-name')"><i>皮</i><b>{{ publicConfig.heroTitle || '小皮租房' }}</b><small>登录后开始找房</small></div></div>
                  <div class="basic-preview-usage"><span>③ 房源详情 · 品牌名称</span><div class="basic-preview-service"><i>皮</i><div><b>{{ publicConfig.heroTitle || '小皮租房' }}</b><small>咨询费用、入住时间和看房安排</small></div><em>在线咨询 ›</em></div></div>
                  <div class="basic-preview-usage"><span>④ 房源详情 / 找房需求 · 服务联系电话</span><div class="basic-preview-phone"><div><b>{{ publicConfig.heroTitle || '小皮租房' }}客服</b><small>点击后调用系统拨号</small></div><button @click="basicPhonePreviewOpen = !basicPhonePreviewOpen">打电话</button><div v-if="basicPhonePreviewOpen" class="basic-phone-popover"><small>拨打服务电话</small><strong>{{ publicConfig.contactPhone || '联系电话待配置' }}</strong><span>真实小程序会交由系统拨号确认</span></div></div></div>
                  <div class="basic-preview-usage"><span>⑤ 分享卡与海报落款 · 品牌名称</span><div class="basic-preview-share-footer canvas-editable-copy" title="点击编辑品牌名称" @click="focusConfigField('brand-name')">⌁ {{ publicConfig.heroTitle || '小皮租房' }} · 小程序</div></div>
                  <div class="basic-preview-usage"><span>⑥ 房源状态标签 · 客户与管理员</span><div class="basic-status-preview"><button type="button" class="canvas-editable-copy" title="点击编辑状态标签" @click="focusConfigField('status-labels', 'brand')"><small>客户视角</small><b>{{ publicConfig.statusLabels.available || '可租' }}</b><em>仅展示可租房源</em></button><button type="button" class="canvas-editable-copy" title="点击编辑状态标签" @click="focusConfigField('status-labels', 'brand')"><small>管理员视角</small><span><i>{{ publicConfig.statusLabels.draft || '草稿' }}</i><i>{{ publicConfig.statusLabels.rented || '已租' }}</i><i>{{ publicConfig.statusLabels.offline || '已下架' }}</i></span></button></div></div>
                </template>
                <template v-else-if="basicConfigTab === 'coverage'">
                  <div class="basic-preview-caption">首页地区入口</div>
                  <div class="mini-locations"><span class="active">附近</span><span v-for="(item, index) in publicConfig.locations.slice(0, 3)" :key="`basic-location-${index}`" class="canvas-editable-copy" title="点击编辑地区入口" @click="coverageDataTab = 'locations'; focusConfigField('coverage')">{{ item.label || '未命名' }}</span></div>
                  <div class="basic-preview-caption">精选频道</div>
                  <div class="mini-channels"><article v-for="(item, index) in publicConfig.featuredChannels.slice(0, 3)" :key="`basic-channel-${index}`" :class="`preview-tone-${item.icon || index}`"><b><i :class="`preview-mark-${item.icon || index}`"></i></b><strong>{{ item.title || '未命名' }}</strong><span>{{ item.subtitle || '频道说明' }}</span></article></div>
                  <div class="basic-preview-caption basic-preview-map-caption">频道地图 · {{ activeChannel?.title || '未选择频道' }}</div>
                  <div v-if="activeChannel && channelMapPreview[activeChannelIndex]" class="basic-preview-map" title="点击地图设置频道中心" @click="selectChannelMapPoint(activeChannel, activeChannelIndex, $event)">
                    <img :src="channelMapPreview[activeChannelIndex]" alt="当前频道地图范围预览" />
                    <div v-if="activeChannel.latitude && activeChannel.longitude" class="basic-preview-map-ring" :style="selectedRadiusStyle(activeChannel)"></div>
                    <div v-if="activeChannel.latitude && activeChannel.longitude" class="basic-preview-map-pin">●</div>
                  </div>
                  <div v-else class="basic-preview-map-empty">先在左侧搜索地点或选取地图中心，右侧会同步显示频道范围。</div>
                  <div v-if="channelLocationResults[activeChannelIndex]?.length" class="basic-preview-search-results">
                    <button v-for="(location, locationIndex) in channelLocationResults[activeChannelIndex].slice(0, 5)" :key="`canvas-location-${location.id}`" type="button" @click="selectChannelLocation(activeChannel, activeChannelIndex, location)">
                      <b>{{ String.fromCharCode(65 + locationIndex) }}</b><span><strong>{{ location.title }}</strong><small>{{ location.district }} · {{ location.address }}</small></span><em>{{ location.distance ? `${(location.distance / 1000).toFixed(1)}km` : '选用' }}</em>
                    </button>
                  </div>
                  <div v-if="activeChannel" class="basic-preview-map-meta"><strong>{{ channelSelectedLocation[activeChannelIndex]?.title || (activeChannel.latitude ? '地图选点' : '尚未选点') }}</strong><span>{{ activeChannel.latitude ? `${activeChannel.latitude.toFixed(6)}, ${activeChannel.longitude.toFixed(6)} · 半径 ${activeChannel.radiusKm} 公里` : '频道地图中心待配置' }}</span></div>
                  <div class="basic-preview-radius canvas-editable-copy" title="点击编辑全局关联距离" @click="focusConfigField('coverage')">附近房源默认关联 <b>{{ publicConfig.relatedPropertyRadiusKm || 3 }} 公里</b></div>
                </template>
                <template v-else-if="basicConfigTab === 'tags'">
                  <div class="basic-tag-preview-tabs" role="tablist" aria-label="标签效果预览"><button :class="{ active: basicTagPreviewTab === 'list' }" @click="basicTagPreviewTab = 'list'">列表</button><button :class="{ active: basicTagPreviewTab === 'detail' }" @click="basicTagPreviewTab = 'detail'">详情</button><button :class="{ active: basicTagPreviewTab === 'share' }" @click="basicTagPreviewTab = 'share'">分享卡</button></div>
                  <template v-if="previewPrimaryProperty && basicTagPreviewTab === 'list'"><div class="basic-preview-caption">客户视角 · 首页推荐 / 房源列表</div><article class="preview-property-card basic-preview-property"><div :style="previewCoverStyle(previewPrimaryProperty)"><em>可租</em></div><section><b>{{ previewListingTitle(previewPrimaryProperty) }}</b><span>{{ previewListingAddress(previewPrimaryProperty) }}</span><small v-if="previewPropertyTags(previewPrimaryProperty).length" class="preview-tag-row secondary"><i v-for="tag in previewPropertyTags(previewPrimaryProperty)" :key="tag">{{ tag }}</i></small><strong>¥ {{ previewPrimaryProperty.rentMonthly }}<em>/月</em></strong></section></article></template>
                  <template v-else-if="previewPrimaryProperty && basicTagPreviewTab === 'detail'"><div class="basic-preview-caption">客户视角 · 房源详情</div><article class="basic-detail-preview"><div class="basic-detail-cover" :style="previewCoverStyle(previewPrimaryProperty)"></div><section><b>¥ {{ previewPrimaryProperty.rentMonthly }}<small>/月</small></b><strong>{{ previewListingTitle(previewPrimaryProperty) }}</strong><span>{{ previewListingAddress(previewPrimaryProperty) }}</span><div class="basic-detail-tags"><i v-for="tag in previewDetailTagGroups(previewPrimaryProperty).top" :key="`top-${tag}`">{{ tag }}</i><i v-for="tag in previewDetailTagGroups(previewPrimaryProperty).bottom" :key="`bottom-${tag}`">{{ tag }}</i></div></section></article></template>
                  <template v-else-if="previewPrimaryProperty && basicTagPreviewTab === 'share'"><div class="basic-preview-caption">微信好友 · 小程序分享卡</div><article class="basic-share-preview"><header><i>皮</i><span>{{ publicConfig.heroTitle || '小皮租房' }}</span></header><strong>{{ previewListingTitle(previewPrimaryProperty) }}</strong><div class="basic-share-card"><div :style="previewCoverStyle(previewPrimaryProperty)"></div><section><b>¥ {{ previewPrimaryProperty.rentMonthly }}<small>/月</small></b><strong>{{ previewPrimaryProperty.community }}</strong><span><i v-for="tag in previewShareTags(previewPrimaryProperty)" :key="tag">{{ tag }}</i></span></section></div><footer>⌁ 小程序</footer></article></template>
                  <div v-else class="preview-empty">暂无可预览房源</div>
                </template>
                <template v-else>
                  <div class="basic-preview-caption">房源详情 · 设施与环境</div>
                  <div class="basic-preview-facilities"><i v-for="facility in publicConfig.propertyDetail.facilities.filter(item => item.enabled)" :key="facility.key"><b>{{ detailFacilityIcon(facility.key) }}</b><span>{{ facility.label }}</span></i></div>
                  <div v-if="!publicConfig.propertyDetail.facilities.some(item => item.enabled)" class="preview-empty">暂未启用设施</div>
                  <div class="basic-preview-hint">录入房源时，仅展示这里已启用的设施选项。</div>
                </template>
              </div>
              <div class="mini-tabbar"><button class="active">⌂<small>首页</small></button><button>▤<small>房源</small></button><button>○<small>我的</small></button></div>
            </div>
            <p class="preview-note">左侧修改会立即反映到此处；保存发布后同步到小程序。</p>
          </aside>
          </div>
        </template>

        <template v-else-if="tab === '小程序页面配置'">
          <div class="miniapp-config-shell">
          <div class="miniapp-config-main">
          <section class="page-header">
            <div><span>内容与运营</span><h1>{{ tab === '小程序基础配置' ? '小程序基础配置 · 地区与频道' : '小程序页面配置' }}</h1><p>{{ tab === '小程序基础配置' ? '配置地区入口、精选频道、地图中心与关联半径；小程序端数据结构保持不变。' : '按真实页面编辑文案、模块和展示方式；右侧画布会跟随定位。' }}</p></div>
          </section>
          <section class="surface config-toolbar">
            <div class="publish-state">
              <i :class="{ dirty: configDirty }" />
              <div><strong>{{ configDirty ? '存在未发布修改' : '线上配置已同步' }}</strong><span>{{ configLastSavedAt ? `最近发布：${configLastSavedAt}` : '当前内容来自线上配置' }}</span></div>
            </div>
            <div class="config-toolbar-meta"><span>{{ publicConfig.locations.length }} 个地区</span><span>{{ publicConfig.featuredChannels.length }} 个频道</span></div>
            <div class="config-toolbar-actions"><el-button :icon="RefreshLeft" :disabled="!configDirty || savingConfig" @click="resetConfig">恢复</el-button><el-button type="primary" :loading="savingConfig" :disabled="!configDirty || configErrors.length > 0" @click="saveConfig">保存并发布</el-button></div>
          </section>
          <div class="canvas-editor-controls">
              <section class="canvas-control-deck" aria-label="小程序画布控制">
                <div class="canvas-control-heading"><span>实时画布</span><small>当前环境数据镜像</small></div>
                <div class="canvas-preview-toolbar"><label class="canvas-preview-control">预览角色<select v-model="canvasIdentityState" @change="switchCanvasIdentity(canvasIdentityState)"><option value="guest">游客</option><option value="user">微信用户</option><option value="admin">小程序管理员</option></select></label><div class="canvas-data-source"><i :class="{ live: overviewState === 'ready', failed: overviewState === 'error' }"></i><span>{{ overviewState === 'loading' ? '正在载入数据…' : overviewState === 'error' ? `加载失败：${overviewError}` : `${properties.length} 套房源 · 当前 Admin 环境` }}</span><button type="button" :disabled="loading" @click="load().catch(() => undefined)">刷新</button></div></div>
              </section>
              <details class="canvas-context-drawer">
                <summary>页面说明、数据边界与状态标签</summary><p class="canvas-data-note">使用当前 Admin API 环境的业务数据。表单提交、删除、状态变更均不写入；微信用户私有数据需由对应用户身份访问。</p>
                <section v-if="selectedCanvasNode" class="canvas-node-inspector" aria-live="polite"><header><span>{{ selectedCanvasNode.kind }}</span><strong>{{ selectedCanvasNode.label }}</strong><button type="button" @click="openCanvasNode(selectedCanvasNodeID)">编辑此处</button></header><p>{{ selectedCanvasNode.impacts }}</p><dl><div><dt>可改</dt><dd>{{ selectedCanvasNode.editable }}</dd></div><div><dt>不可改</dt><dd>{{ selectedCanvasNode.locked }}</dd></div></dl></section>
                <section v-if="configPage === 'properties'" class="canvas-status-legend canvas-editable-copy" title="点击编辑状态标签" @click="openCanvasNode('status')"><small>{{ configAudience === 'customer' ? '客户列表' : '管理员列表' }}</small><span v-if="configAudience === 'customer'"><i class="available">{{ previewStatusLabel('available') }}</i>客户只会看到可租房源</span><span v-else><i>{{ previewStatusLabel('draft') }}</i><i>{{ previewStatusLabel('available') }}</i><i>{{ previewStatusLabel('rented') }}</i><i>{{ previewStatusLabel('offline') }}</i></span></section>
                <section v-if="configPage === 'properties' || configPage === 'propertyDetail'" class="tag-magnifier" :class="{ open: tagMagnifierOpen }">
                <header><span>标签局部预览</span><button type="button" @click="tagMagnifierOpen = !tagMagnifierOpen">{{ tagMagnifierOpen ? '收起' : '放大标签' }}</button></header>
                <article v-if="tagMagnifierOpen && previewPrimaryProperty">
                  <b>{{ configPage === 'properties' ? '列表卡片标签' : '详情页标签' }}</b>
                  <strong>{{ previewListingTitle(previewPrimaryProperty) }}</strong>
                  <small v-if="configAudience === 'operator' && publicConfig.listingTitle.adminEnabledFields.includes('unit')">门牌定位 · {{ previewOperatorDoorplate || '待补充' }}</small>
                  <div v-if="configPage === 'properties'" class="tag-magnifier-list" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`">
                    <template v-if="publicConfig.listingTitle.listTagLayout === 'grouped'"><span v-for="tag in previewTagGroups(previewPrimaryProperty).primary" :key="`primary-${tag}`" class="primary">{{ tag }}</span><span v-for="tag in previewTagGroups(previewPrimaryProperty).secondary" :key="`secondary-${tag}`">{{ tag }}</span></template>
                    <span v-else v-for="tag in previewTagGroups(previewPrimaryProperty).all" :key="tag">{{ tag }}</span>
                  </div>
                  <div v-else class="tag-magnifier-detail"><span v-for="tag in previewDetailTagGroups(previewPrimaryProperty).top" :key="`top-${tag}`">{{ tag }}</span></div>
                </article>
                </section>
              </details>
              <nav class="canvas-page-dock" aria-label="预览页面切换"><span>页面</span><div class="canvas-page-rail"><button v-for="item in pagePresentationOptions.filter(page => page.audience === 'both' || page.audience === configAudience)" :key="`canvas-page-${item.key}`" type="button" :class="{ active: configPage === item.key }" @click="selectConfigPage(item.key)">{{ item.label }}</button></div></nav>
          </div>
          <section v-if="tab === '小程序页面配置'" class="surface page-config-nav">
            <div class="config-audience-heading"><strong>选择使用视角</strong><span>先按角色筛选，再以真实小程序页面进行配置</span></div>
            <div class="config-audience-switch"><button type="button" :class="{ active: configAudience === 'customer' }" @click="switchConfigAudience('customer')"><b>客户视角</b><small>找房、关注与联系</small></button><button type="button" :class="{ active: configAudience === 'operator' }" @click="switchConfigAudience('operator')"><b>非客户视角</b><small>运营、录入与管理</small></button></div>
          </section>
          <el-alert v-if="configMessage" class="config-alert" :title="configMessage" :type="configFailed ? 'error' : 'success'" show-icon :closable="false" />
          <section v-if="configErrors.length" class="config-validation">
            <strong>发布前需要处理 {{ configErrors.length }} 项</strong>
            <ul><li v-for="error in configErrors" :key="error">{{ error }}</li></ul>
          </section>
          <div v-loading="loading" class="config-workspace">
            <div class="config-editor">
              <section v-if="configPage === 'home' && tab === '小程序页面配置'" class="surface form-section config-section">
                <div class="section-heading"><span>01</span><div><h2>首页页面配置</h2><p>页面文案与背景只在这里编辑；品牌、地区和房源基础字段请前往“小程序基础配置”。</p></div></div>
                <el-form label-position="top" class="basic-config-form">
                  <el-form-item label="首页主标题" required><div data-config-field="home-title"><el-input v-model="publicConfig.pagePresentations.home.title" maxlength="30" show-word-limit placeholder="例如：今天想住哪里？" /></div></el-form-item>
                  <el-form-item label="搜索提示"><div data-config-field="home-search"><el-input v-model="publicConfig.pagePresentations.home.subtitle" maxlength="60" show-word-limit placeholder="例如：搜索小区、地铁或商圈" /></div></el-form-item>
                  <el-form-item label="首页分区文案"><div data-config-field="home-sections" class="form-row two-columns"><el-input v-model="publicConfig.pagePresentations.home.nearbyLabel" placeholder="附近入口" /><el-input v-model="publicConfig.pagePresentations.home.channelTitle" placeholder="精选频道" /><el-input v-model="publicConfig.pagePresentations.home.allChannelsText" placeholder="全部频道" /><el-input v-model="publicConfig.pagePresentations.home.recommendationTitle" placeholder="推荐房源" /><el-input v-model="publicConfig.pagePresentations.home.viewAllText" placeholder="查看房源" /></div><div class="form-tip">这些是标题文本级配置，只影响显示，不改变地区、频道或房源数据。</div></el-form-item>
                  <el-form-item label="首页背景图"><el-input v-model="publicConfig.pagePresentations.home.backgroundImage" placeholder="上传后自动填入，也可填写图片地址" /><div class="presentation-actions"><label class="upload-control"><input type="file" accept="image/jpeg,image/png,image/webp" @change="uploadPresentationBackground('home', $event)" />上传背景图</label><button v-if="publicConfig.pagePresentations.home.backgroundImage" @click="publicConfig.pagePresentations.home.backgroundImage = ''">移除背景图</button></div></el-form-item>
                </el-form>
              </section>

              <section v-if="configPage === 'home'" class="surface form-section config-section page-channel-presentation">
                <div class="section-heading config-section-heading"><span>02</span><div><h2>精选频道展示</h2><p>只影响首页卡片的顺序、样式与副标题；频道名称、地图定位和房源关联规则由“小程序基础配置 → 地区与关联范围”维护。</p></div></div>
                <div class="channel-editor-tabs"><button v-for="(item, index) in publicConfig.featuredChannels" :key="`home-channel-tab-${index}`" type="button" :class="{ active: activeChannelIndex === index }" @click="activeChannelIndex = index"><span>{{ index + 1 }}</span>{{ item.title || `频道 ${index + 1}` }}</button></div>
                <article v-for="(item, index) in publicConfig.featuredChannels" v-show="activeChannelIndex === index" :key="`home-channel-${index}`" class="channel-editor-card active"><header><div class="channel-order"><span>首页展示</span><strong>{{ item.title || `频道 ${index + 1}` }}</strong></div><div class="item-actions"><el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="首页上移" @click="moveItem(publicConfig.featuredChannels, index, -1)" /><el-button text :icon="ArrowDown" :disabled="index === publicConfig.featuredChannels.length - 1" aria-label="首页下移" @click="moveItem(publicConfig.featuredChannels, index, 1)" /></div></header><div class="channel-fields"><label class="icon-field"><span>卡片样式</span><el-select v-model="item.icon"><el-option v-for="style in channelStyleOptions" :key="style.value" :label="style.label" :value="style.value" /></el-select></label><label><span>卡片副标题</span><el-input v-model="item.subtitle" maxlength="20" placeholder="例如：通勤优选" /></label></div></article>
              </section>

              <section v-if="configPage !== 'home'" data-config-field="page-copy" class="surface form-section config-section selected-page-config">
                <div class="section-heading config-section-heading"><span>01</span><div><h2>{{ pagePresentationOptions.find(page => page.key === configPage)?.label }}配置</h2><p>仅调整该页面的展示文案和背景，不影响页面功能。</p></div></div>
                <el-form label-position="top" class="basic-config-form">
                  <el-form-item label="页面标题"><el-input v-model="publicConfig.pagePresentations[configPage].title" maxlength="30" show-word-limit /></el-form-item>
                  <el-form-item label="辅助文本"><el-input v-model="publicConfig.pagePresentations[configPage].subtitle" maxlength="60" show-word-limit /></el-form-item>
                  <el-form-item label="背景图"><el-input v-model="publicConfig.pagePresentations[configPage].backgroundImage" placeholder="上传后自动填入，也可填写图片地址" /></el-form-item>
                  <div class="presentation-actions"><label class="upload-control"><input type="file" accept="image/jpeg,image/png,image/webp" @change="uploadPresentationBackground(configPage, $event)" />上传背景图</label><button v-if="publicConfig.pagePresentations[configPage].backgroundImage" @click="publicConfig.pagePresentations[configPage].backgroundImage = ''">移除背景图</button></div>
                </el-form>
              </section>
              <section v-if="configPage === 'properties'" class="surface form-section config-section tag-layout-config">
                <div class="section-heading config-section-heading"><span>02</span><div><h2>列表标签排版</h2><p>首页推荐与“房源”页共用。这里只调整排列，不改变基础配置中的公开标签范围。</p></div></div>
                <div class="layout-choice-row">
                  <div><strong>标签布局</strong><small>分组两行优先突出租赁方式；紧凑换行适合标签较少的卡片。</small></div>
                  <el-radio-group v-model="publicConfig.listingTitle.listTagLayout">
                    <el-radio-button value="grouped">分组两行</el-radio-button>
                    <el-radio-button value="wrap">紧凑换行</el-radio-button>
                  </el-radio-group>
                </div>
                <div class="tag-arranger">
                  <header><strong>拖动编排标签</strong><span>拖到上/下行调整上下位置；在同一行拖到标签前调整左右顺序。</span><button type="button" class="tag-arranger-preset" @click="applyRecommendedListTagArrangement">推荐排序</button></header>
                  <div class="tag-arranger-lanes">
                    <section class="tag-arranger-lane" @dragover.prevent @drop.prevent="moveTagToListRow('top')">
                      <div class="tag-arranger-lane-title"><b>第一行</b><small>主标签行</small></div>
                      <div class="tag-arranger-items">
                        <button v-for="key in listTopTagFields" :key="`top-${key}`" type="button" draggable="true" class="tag-drag-item" :class="{ dragging: draggingTag === key }" :title="`拖到“${listingTitleFieldOptions.find(field => field.key === key)?.label}”前`" @dragstart="draggingTag = key" @dragend="draggingTag = ''" @dragover.prevent @drop.prevent.stop="moveTagToListRow('top', key)"><i>⋮⋮</i>{{ listingTitleFieldOptions.find(field => field.key === key)?.label }}</button>
                        <span v-if="!listTopTagFields.length" class="tag-drop-empty">拖到这里</span>
                      </div>
                    </section>
                    <section class="tag-arranger-lane" @dragover.prevent @drop.prevent="moveTagToListRow('bottom')">
                      <div class="tag-arranger-lane-title"><b>第二行</b><small>补充标签行</small></div>
                      <div class="tag-arranger-items">
                        <button v-for="key in listBottomTagFields" :key="`bottom-${key}`" type="button" draggable="true" class="tag-drag-item" :class="{ dragging: draggingTag === key }" :title="`拖到“${listingTitleFieldOptions.find(field => field.key === key)?.label}”前`" @dragstart="draggingTag = key" @dragend="draggingTag = ''" @dragover.prevent @drop.prevent.stop="moveTagToListRow('bottom', key)"><i>⋮⋮</i>{{ listingTitleFieldOptions.find(field => field.key === key)?.label }}</button>
                        <span v-if="!listBottomTagFields.length" class="tag-drop-empty">拖到这里</span>
                      </div>
                    </section>
                  </div>
                </div>
                <div class="basic-config-note"><b>右侧画布已同步</b><span>切换后可在画布内点击“放大标签”查看真实卡片的局部效果。</span></div>
              </section>
              <section v-if="false" class="surface form-section config-section share-card-config">
                <div class="section-heading config-section-heading"><span>02</span><div><h2>全局房源标签规则</h2><p>只需在这里配置一次；客户列表、客户详情、管理员房源与所有公开分享都会复用。</p></div></div>
                <div class="share-field-editor listing-title-editor">
                  <header><strong>客户视角：允许配置的标签</strong><span>在这里开启后会立即加入客户列表和详情，并同步右侧画布；客户视角可再分别取消。发布者不参与标签选择。</span></header>
                  <div class="share-field-options"><button v-for="field in listingTitleFieldOptions" :key="`customer-allowed-${field.key}`" type="button" class="field-check" :class="{ checked: field.key === 'community' || publicConfig.listingTitle.customerAllowedFields.includes(field.key), locked: field.key === 'community' }" :disabled="field.key === 'community'" :aria-pressed="field.key === 'community' || publicConfig.listingTitle.customerAllowedFields.includes(field.key)" @click="toggleCustomerAllowedListingField(field.key, !publicConfig.listingTitle.customerAllowedFields.includes(field.key))"><i>{{ field.key === 'community' || publicConfig.listingTitle.customerAllowedFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.key === 'community' ? '小区主标题' : field.label }}</span></button></div>
                  <div class="listing-title-preview"><span>客户可配置标签范围</span><strong>{{ publicConfig.listingTitle.customerAllowedFields.filter(key => key !== 'community').map(key => listingTitleFieldOptions.find(field => field.key === key)?.label).filter(Boolean).join('、') || '仅显示小区主标题' }}</strong></div>
                </div>
                <div class="share-field-editor listing-title-editor">
                  <header><strong>非客户视角：管理员附加信息</strong><span>管理员与客户复用同一套公开标签；这里只决定管理员是否额外看见精确门牌。</span></header>
                  <div class="share-field-options"><button type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.adminEnabledFields.includes('unit') }" :aria-pressed="publicConfig.listingTitle.adminEnabledFields.includes('unit')" @click="toggleAdminDoorplate(!publicConfig.listingTitle.adminEnabledFields.includes('unit'))"><i>{{ publicConfig.listingTitle.adminEnabledFields.includes('unit') ? '✓' : '' }}</i><span>额外显示门牌标签</span></button></div>
                  <div class="listing-title-preview"><span>管理员最终效果</span><strong>{{ publicConfig.listingTitle.adminEnabledFields.includes('unit') ? '客户公开标签 + 5栋3单元101室' : '与客户公开标签完全一致' }}</strong></div>
                </div>
              </section>
              <section v-if="false" class="surface form-section config-section share-card-config">
                <div class="section-heading config-section-heading"><span>02</span><div><h2>客户公开标签</h2><p>配置一次即同步列表、首页推荐、详情与好友分享；发布者无需、也不能为单套房源选择标签。</p></div></div>
                <div class="share-field-editor listing-title-editor">
                  <header><strong>客户实际展示标签</strong><span>小区名称始终作为主标题。灰色项尚未在非客户视角开放，需先回到非客户视角开启。</span></header>
                  <div class="share-field-options"><button v-for="field in listingTitleFieldOptions" :key="`customer-visible-${field.key}`" type="button" class="field-check" :class="{ checked: field.key === 'community' || publicConfig.listingTitle.enabledFields.includes(field.key), locked: field.key === 'community' }" :disabled="field.key === 'community' || !publicConfig.listingTitle.customerAllowedFields.includes(field.key)" :aria-pressed="field.key === 'community' || publicConfig.listingTitle.enabledFields.includes(field.key)" @click="toggleListingTitleField(field.key, !publicConfig.listingTitle.enabledFields.includes(field.key))"><i>{{ field.key === 'community' || publicConfig.listingTitle.enabledFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.key === 'community' ? '小区主标题' : field.label }}</span></button></div>
                  <div class="listing-title-preview"><span>当前客户展示标签</span><strong>{{ publicConfig.listingTitle.enabledFields.filter(key => key !== 'community').map(key => listingTitleFieldOptions.find(field => field.key === key)?.label).filter(Boolean).join('、') || '仅显示小区主标题' }}</strong></div>
                </div>
              </section>
              <section v-if="configPage === 'propertyDetail'" class="surface detail-config-workspace" aria-label="房源详情配置工作区">
                <span>配置对象</span>
                <div class="detail-config-tabs" role="tablist" aria-label="房源详情配置选项卡">
                  <button type="button" :class="{ active: detailConfigTab === 'detail' }" role="tab" :aria-selected="detailConfigTab === 'detail'" @click="selectDetailConfigTab('detail')">房源详情</button>
                  <button type="button" :class="{ active: detailConfigTab === 'share' }" role="tab" :aria-selected="detailConfigTab === 'share'" @click="selectDetailConfigTab('share')">微信分享卡</button>
                </div>
                <el-select v-model="previewSelectedPropertyID" class="detail-preview-property" placeholder="选择真实房源">
                  <el-option v-for="property in previewPropertyOptions" :key="property.id" :label="`${property.community || property.title} · ¥${property.rentMonthly || 0}/月`" :value="property.id" />
                </el-select>
                <small>固定首屏摘要始终展示；下方四个模块的开关会同步影响管理员录入项、客户详情页与右侧画布。关闭只隐藏录入与展示，不删除已有房源数据。</small>
                <div class="detail-module-master-grid" aria-label="房源详情模块总控">
                  <article v-for="module in detailModuleOptions" :key="`module-master-${module.key}`" :class="{ active: detailSectionTab === module.key, disabled: !module.fixed && !detailModule(module.key).enabled }" @click="selectDetailSectionTab(module.key)">
                    <span><b>{{ module.label }}</b><small>{{ module.helper }}</small></span>
                    <aside><em>{{ module.fixed ? '固定展示' : detailModule(module.key).enabled ? '已启用' : '已停用' }}</em><el-switch v-if="!module.fixed" :model-value="detailModule(module.key).enabled" @click.stop @update:model-value="toggleDetailModule(module.key, $event)" /></aside>
                  </article>
                </div>
              </section>
              <section v-if="configPage === 'propertyDetail' && detailConfigTab === 'detail'" class="surface form-section config-section share-card-config">
                <div class="section-heading config-section-heading"><span>02</span><div><h2>房源详情模块</h2><p>这里配置录入和详情展示模块；标签规则统一在“小程序基础配置 → 房源标签规则”维护。</p></div><el-button plain @click="selectTab('小程序基础配置'); basicConfigTab = 'tags'">配置标签规则</el-button></div>
                  <div v-if="detailSectionTab === 'overview'" data-config-field="detail-overview" class="share-field-editor listing-title-editor">
                    <header><strong>固定首屏摘要</strong><span>这不是可关闭模块。面积、户型、楼层等房屋概况始终在这里展示；合租录入整套参数后，这里显示整套户型、整套面积。当前出租房间的实际空间与面积则用于列表标签，并在下方“租赁信息”展示。</span></header>
                    <div class="basic-config-note"><b>列表标签与详情信息已分层</b><span>列表可使用标签吸引与筛选；详情首屏以对称参数为主，不继承普通列表标签。房源简介、位置与通勤、房屋设施、附近推荐在下方作为独立模块管理。</span></div>
                    <div class="basic-config-note"><b>详情页与列表标签分层</b><span>详情首屏仅在转租时突出“剩余租期”等特殊提醒；整租/合租、面积、户型、楼层等在参数区对称展示，中介费在租赁信息中说明。</span><el-button plain @click="selectTab('小程序基础配置'); basicConfigTab = 'tags'">调整列表标签</el-button></div>
                  </div>
                  <div v-else-if="detailSectionTab === 'location'" data-config-field="detail-location" class="share-field-editor listing-title-editor">
                    <header><strong>位置与周边</strong><span>小区与地图坐标用于归属地区和附近房源关联，始终保留；本模块控制通勤等周边补充字段及详情展示。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-module-form"><el-form-item label="模块标题"><el-input v-model="detailModule('location').title" maxlength="12" /></el-form-item></el-form>
                  </div>
                  <div v-else-if="detailSectionTab === 'facilities'" data-config-field="detail-facilities" class="share-field-editor listing-title-editor">
                    <header><strong>房屋设施</strong><span>本页只控制该模块是否出现在录入页和详情页；设施的名称、分类、可勾选项统一由“小程序基础配置”维护。水电杂费属于首屏租赁信息，不归入本模块。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-module-form"><el-form-item label="模块标题"><el-input v-model="detailModule('facilities').title" maxlength="12" /></el-form-item></el-form>
                    <div class="basic-config-note"><b>当前设施库：{{ publicConfig.propertyDetail.facilities.filter(item => item.enabled).length }} 项已启用</b><span>进入“小程序基础配置 → 房屋设施库”后，点击整张设施卡即可调整录入和详情可用项。</span><el-button plain @click="selectTab('小程序基础配置'); basicConfigTab = 'facilities'">配置设施库</el-button></div>
                  </div>
                  <div v-else-if="detailSectionTab === 'description'" data-config-field="detail-description" class="share-field-editor listing-title-editor">
                    <header><strong>房源简介</strong><span>用于房东补充说明、看房方式和居住规则；关闭后，录入页不再显示这两项输入，已有历史内容会保留但不会展示。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-module-form"><el-form-item label="模块标题"><el-input v-model="detailModule('description').title" maxlength="12" /></el-form-item></el-form>
                  </div>
                  <div v-else data-config-field="detail-nearby" class="share-field-editor listing-title-editor">
                    <header><strong>详情推荐 · 独立关联范围</strong><span>这是纯展示模块，不增加录入项；仅推荐可租房源，并自动排除当前房源。它独立于首页附近默认距离和频道地图半径。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-nearby-form"><el-form-item label="模块标题"><el-input v-model="detailModule('nearby').title" maxlength="12" /></el-form-item><el-form-item label="关联逻辑"><el-radio-group v-model="publicConfig.propertyDetail.nearby.relation"><el-radio value="distance">按地图距离</el-radio><el-radio value="channel">按精选频道</el-radio></el-radio-group></el-form-item><el-form-item label="详情推荐范围"><el-select v-model="publicConfig.propertyDetail.nearby.radiusKm"><el-option v-for="value in [1, 3, 5, 10]" :key="value" :label="`${value} 公里`" :value="value" /></el-select></el-form-item><el-form-item label="排序方式"><el-radio-group v-model="publicConfig.propertyDetail.nearby.sort"><el-radio value="distance">距离优先</el-radio><el-radio value="newest">最新发布优先</el-radio></el-radio-group></el-form-item><el-form-item label="展示数量"><el-radio-group v-model="publicConfig.propertyDetail.nearby.limit"><el-radio v-for="value in [2, 4, 6]" :key="value" :value="value">{{ value }} 套</el-radio></el-radio-group></el-form-item></el-form>
                  </div>
              </section>
              <section v-if="configAudience === 'customer' && configPage === 'propertyDetail' && detailConfigTab === 'share'" class="surface form-section config-section share-card-config">
                <div class="section-heading config-section-heading"><span>03</span><div><h2>微信分享卡</h2><p>好友分享不区分角色，统一采用客户公开标签；右侧画布显示聊天中的实际卡片效果。</p></div></div>
                <el-form label-position="top" class="basic-config-form"><el-form-item label="标题前缀（选填）"><el-input v-model="publicConfig.shareCard.titlePrefix" maxlength="12" show-word-limit placeholder="例如：精选好房" /></el-form-item></el-form>
                <div class="share-field-editor">
                  <header><strong>客户标签来源</strong><span>以下标签直接沿用“客户房源列表标签”配置；管理员门牌不会进入分享卡。</span></header>
                  <div class="listing-title-preview"><span>当前用于公开分享的标签</span><strong>{{ publicConfig.listingTitle.enabledFields.filter(key => key !== 'community').map(key => listingTitleFieldOptions.find(field => field.key === key)?.label).filter(Boolean).join('、') || '仅显示小区主标题' }}</strong></div>
                </div>
                <div class="share-field-editor">
                  <header><strong>分享标题</strong><span>默认推荐“小区名称 + 户型 / 房间”，不突出合租或分租。</span></header>
                  <div class="share-field-options"><button v-for="field in shareFieldOptions" :key="`title-${field.key}`" type="button" class="field-check" :class="{ checked: publicConfig.shareCard.titleFields.includes(field.key) }" :disabled="!publicConfig.shareCard.titleFields.includes(field.key) && (publicConfig.shareCard.titleFields.length >= 4 || !shareFieldEnabledForCustomer(field.key))" :aria-pressed="publicConfig.shareCard.titleFields.includes(field.key)" @click="toggleShareField('titleFields', field.key, !publicConfig.shareCard.titleFields.includes(field.key))"><i>{{ publicConfig.shareCard.titleFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>
                  <div class="share-field-order"><article v-for="(key, index) in publicConfig.shareCard.titleFields" :key="`title-order-${key}`"><i>{{ index + 1 }}</i><strong>{{ shareFieldLabel(key) }}</strong><span><button :disabled="index === 0" @click="moveShareField('titleFields', index, -1)">上移</button><button :disabled="index === publicConfig.shareCard.titleFields.length - 1" @click="moveShareField('titleFields', index, 1)">下移</button></span></article></div>
                </div>
              </section>
            </div>
          </div>
          </div>

            <aside v-if="tab === '小程序页面配置'" class="config-preview-column">
              <div ref="canvasPreviewStage" class="canvas-preview-stage">
              <div v-if="configPage === 'home'" ref="canvasPreviewPhone" class="phone-preview canvas-home-phone" :style="{ transform: `scale(${canvasFitScale})` }">
                <div class="phone-status"><span>22:18</span><span>4G ▰</span></div>
                <div class="mini-navigation">{{ publicConfig.heroTitle || '小皮租房' }}</div>
                <div class="mini-content" :style="publicConfig.pagePresentations.home.backgroundImage ? { backgroundImage: `linear-gradient(rgba(247,246,241,.88),rgba(247,246,241,.96)), url(${publicConfig.pagePresentations.home.backgroundImage})` } : {}">
                  <div v-if="canvasNotice" class="canvas-preview-notice" role="status">{{ canvasNotice }}</div>
                  <section class="mini-hero" :style="{ backgroundImage: `linear-gradient(90deg, #f9e9d7 0%, #f9e9d7a3 57%, #f9e9d729 100%), url(${publicConfig.pagePresentations.home.backgroundImage || homeHeroFallback}), url(${homeHeroFallback})` }"><span class="mini-city" @click="canvasAreaFilter = canvasAreaFilter === '全部' ? (publicConfig.locations[0]?.label || '全部') : '全部'"><i class="canvas-city-pin" aria-hidden="true">⌖</i><span>武汉</span><small>⌄</small></span><strong class="canvas-editable-copy" title="点击编辑首页主标题" @click="openCanvasNode('home.title')">{{ publicConfig.pagePresentations.home.title || '今天想住哪里？' }}</strong><i></i><div class="mini-search"><span class="canvas-search-icon" aria-hidden="true">⌕</span><input v-model="canvasSearchQuery" class="canvas-search-input" :placeholder="publicConfig.pagePresentations.home.subtitle || '搜索小区、地铁或商圈'" @keyup.enter="switchCanvasTab('properties')"><button type="button" @click="switchCanvasTab('properties')">搜索</button></div></section>
                  <div class="mini-locations"><button type="button" :class="{ active: canvasAreaFilter === '全部' }" @click="canvasAreaFilter = '全部'; switchCanvasTab('properties')">{{ publicConfig.pagePresentations.home.nearbyLabel || '附近' }}</button><button v-for="(item, index) in publicConfig.locations" :key="`preview-location-${index}`" type="button" :class="{ active: canvasAreaFilter === item.label }" @click="canvasAreaFilter = item.label; switchCanvasTab('properties')">{{ item.label || '未命名' }}</button><em v-if="!publicConfig.locations.length">尚未配置</em></div>
                  <div class="mini-budget"><b>租金预算</b><button v-for="budget in ['≤1500', '1500–2500', '2500–3500', '≥3500']" :key="budget" type="button" :class="{ selected: canvasBudgetFilter === budget }" @click="canvasBudgetFilter = canvasBudgetFilter === budget ? '不限' : budget">{{ budget === '不限' ? '不限价格' : budget === '≤1500' ? '≤1500' : budget }}</button></div>
                  <div class="mini-budget mini-kind"><b>住法</b><button v-for="kind in ['整租', '合租', '一居', '两居', '三居+']" :key="kind" type="button" :class="{ selected: canvasRentTypeFilter === kind || (kind === '推荐' && canvasRentTypeFilter === '全部') }" @click="canvasRentTypeFilter = canvasRentTypeFilter === kind ? '推荐' : kind">{{ kind }}</button></div>
                  <div class="mini-section-title"><strong class="canvas-editable-copy" title="点击编辑首页分区文案" @click="openCanvasNode('home.sections')">{{ publicConfig.pagePresentations.home.channelTitle || '精选频道' }}</strong><b role="button" tabindex="0" @click="canvasPageFilterReset(); switchCanvasTab('properties')">{{ publicConfig.pagePresentations.home.allChannelsText || '全部频道' }}</b></div>
                  <div class="mini-channels"><button v-for="(item, index) in publicConfig.featuredChannels" :key="`preview-channel-${index}`" type="button" :class="`preview-tone-${item.icon || index}`" @click="canvasChannelFilter = item.title; switchCanvasTab('properties')"><b><i :class="`preview-mark-${item.icon || index}`"></i></b><strong>{{ item.title || '未命名' }}</strong><span>{{ item.subtitle || '频道说明' }}</span></button><em v-if="!publicConfig.featuredChannels.length">尚未配置</em></div>
                <div class="mini-recommend"><strong class="canvas-editable-copy" title="点击编辑首页分区文案" @click="openCanvasNode('home.sections')">{{ publicConfig.pagePresentations.home.recommendationTitle || '推荐房源' }}</strong><b @click="canvasPageFilterReset(); switchCanvasTab('properties')">{{ publicConfig.pagePresentations.home.viewAllText || '查看房源' }}</b></div>
                  <div v-if="canvasDataState !== 'normal'" class="preview-empty">{{ canvasDataState === 'loading' ? '正在加载房源…' : canvasDataState === 'error' ? '房源暂时加载失败 · 检查网络后重新加载' : '暂时没有可租房源 · 新房源正在整理中' }}</div>
                  <div v-if="canvasDataState === 'normal' && configAudience === 'customer' && previewProperties.length" class="preview-home-masonry">
                    <article v-for="item in previewProperties.slice(0, 6)" :key="item.id" class="preview-property-card preview-masonry-card" @click="openPreviewProperty(item)"><div class="preview-card-cover" :style="previewCoverStyle(item)"><em>{{ previewStatusLabel(item.status) }}</em><i v-if="item.media?.some(media => media.type === 'video')" class="preview-video-play">▶</i><small v-if="previewChannelLabel(item)" class="preview-cover-location">⌖ {{ previewChannelLabel(item) }}</small><button type="button" class="canvas-favorite-toggle" :class="{ active: canvasPreviewFavorites.includes(item.id) }" @click.stop="toggleCanvasFavorite(item.id)">{{ canvasPreviewFavorites.includes(item.id) ? '♥' : '♡' }}</button></div><section><b>{{ previewListingTitle(item) }}</b><div class="preview-tag-stack" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`"><small v-if="previewTagGroups(item).primary.length" class="preview-tag-row primary"><i v-for="tag in previewTagGroups(item).primary" :key="tag">{{ tag }}</i></small></div><strong>¥ {{ item.rentMonthly }}<em>/月</em></strong></section></article>
                  </div>
                  <article v-else-if="canvasDataState === 'normal' && previewPrimaryProperty" class="preview-property-card preview-home-property-card" @click="openPreviewProperty(previewPrimaryProperty)"><div class="preview-card-cover" :style="previewCoverStyle(previewPrimaryProperty)"><em>{{ previewStatusLabel(previewPrimaryProperty.status) }}</em><i v-if="previewPrimaryProperty.media?.some(media => media.type === 'video')" class="preview-video-play">▶</i><small v-if="previewChannelLabel(previewPrimaryProperty)" class="preview-cover-location">⌖ {{ previewChannelLabel(previewPrimaryProperty) }}</small></div><section><b>{{ previewListingTitle(previewPrimaryProperty) }}</b><small v-if="previewOperatorDoorplate" class="preview-operator-doorplate">门牌 · {{ previewOperatorDoorplate }}</small><div class="preview-tag-stack" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`"><template v-if="publicConfig.listingTitle.listTagLayout === 'grouped'"><small v-if="previewTagGroups(previewPrimaryProperty).primary.length" class="preview-tag-row primary"><i v-for="tag in previewTagGroups(previewPrimaryProperty).primary" :key="tag">{{ tag }}</i></small><small v-if="previewTagGroups(previewPrimaryProperty).secondary.length" class="preview-tag-row secondary"><i v-for="tag in previewTagGroups(previewPrimaryProperty).secondary" :key="tag">{{ tag }}</i></small></template><small v-else-if="previewTagGroups(previewPrimaryProperty).all.length" class="preview-tag-row all"><i v-for="tag in previewTagGroups(previewPrimaryProperty).all" :key="tag">{{ tag }}</i></small></div><strong>¥ {{ previewPrimaryProperty.rentMonthly }}<em>/月</em></strong></section></article>
                  <div v-else-if="canvasDataState === 'normal'" class="preview-empty">暂无可展示房源</div>
                </div>
                <button v-if="configPage === 'home' && configAudience === 'customer'" type="button" class="canvas-request-fab" @click="navigateCanvasPage('requests')">✎ <span>说说找房需求</span></button>
                <div class="mini-tabbar"><button :class="{ active: configPage === 'home' }" @click="switchCanvasTab('home')"><img :src="configPage === 'home' ? homeTabActiveIcon : homeTabIcon" alt=""><small>首页</small></button><button :class="{ active: configPage === 'properties' }" @click="switchCanvasTab('properties')"><img :src="configPage === 'properties' ? propertyTabActiveIcon : propertyTabIcon" alt=""><small>房源</small></button><button :class="{ active: configPage === 'me' }" @click="switchCanvasTab('me')"><img :src="configPage === 'me' ? meTabActiveIcon : meTabIcon" alt=""><small>我的</small></button></div>
              </div>
              <div v-else ref="canvasPreviewPhone" class="phone-preview page-phone-preview" :class="`preview-${configPage}`" :style="{ transform: `scale(${canvasFitScale})` }">
                <div class="phone-status"><span>22:18</span><span>4G ▰</span></div>
                <div v-if="configPage !== 'propertyDetail'" class="mini-navigation"><button v-if="!['home', 'properties', 'me', 'adminWorkbench'].includes(configPage)" type="button" :aria-label="configAudience === 'customer' ? '返回首页' : '返回'" @click="configAudience === 'customer' ? switchCanvasTab('home') : backCanvasPage()">{{ configAudience === 'customer' ? '⌂' : '‹' }}</button><span>{{ canvasNativeTitle }}</span></div>
                <div class="page-mini-content" :style="publicConfig.pagePresentations[configPage].backgroundImage ? { backgroundImage: `linear-gradient(rgba(247,246,241,.9),rgba(247,246,241,.96)), url(${publicConfig.pagePresentations[configPage].backgroundImage})` } : {}">
                  <div v-if="canvasNotice" class="canvas-preview-notice" role="status">{{ canvasNotice }}</div>
                  <header v-if="!['requests', 'authLogin'].includes(configPage)" class="canvas-editable-copy" :class="{ 'canvas-me-heading': configPage === 'me', 'canvas-listing-heading': configPage === 'properties', 'canvas-history-heading': configPage === 'requestHistory' }" title="点击编辑本页标题与辅助文本" @click="openCanvasNode('page.heading')"><template v-if="configPage === 'properties'"><div class="canvas-listing-title-row"><strong>{{ publicConfig.pagePresentations[configPage].title }}</strong><span class="canvas-listing-count">{{ previewProperties.length }} 套可租</span></div><span>{{ publicConfig.pagePresentations[configPage].subtitle }}</span></template><template v-else-if="configPage === 'me'"><div class="canvas-me-heading-row"><div><small>个人中心</small><strong>{{ publicConfig.pagePresentations[configPage].title }}</strong></div><span>{{ publicConfig.pagePresentations[configPage].subtitle }}</span></div></template><template v-else-if="configPage === 'requestHistory'"><div><strong>{{ publicConfig.pagePresentations[configPage].title }}</strong><span>{{ publicConfig.pagePresentations[configPage].subtitle }}</span></div><div class="canvas-history-count"><strong>{{ canvasIdentityState === 'guest' ? 0 : '—' }}</strong><small>{{ canvasIdentityState === 'guest' ? '进行中' : '需授权' }}</small></div></template><template v-else><strong>{{ publicConfig.pagePresentations[configPage].title }}</strong><span>{{ publicConfig.pagePresentations[configPage].subtitle }}</span></template></header>
                  <div v-if="canvasDataState === 'loading'" class="preview-empty"><b>正在加载…</b><small>这是小程序列表页的加载状态。</small></div>
                  <div v-else-if="canvasDataState === 'error'" class="preview-empty"><b>数据加载失败</b><small>检查网络后重新加载。</small><button type="button" @click="load().catch(() => undefined)">重新加载</button></div>
                  <div v-else-if="canvasDataState === 'empty' && configPage === 'properties' && configAudience === 'customer'" class="canvas-properties-empty"><span class="canvas-properties-empty-icon">⌕</span><strong>{{ canvasPropertyEmptyTitle }}</strong><small>可以调整筛选条件，或告诉小皮你的预算和入住要求。</small><button type="button" @click="handleCanvasPropertyEmptyAction">{{ canvasPropertyEmptyAction }}</button><a v-if="canvasHasPropertyFilters && !canvasFavoriteOnly" href="#" @click.prevent="canvasPageFilterReset">清除筛选条件</a></div>
                  <div v-else-if="canvasDataState === 'empty'" class="preview-empty"><b>当前没有可展示记录</b><small>清除筛选条件或稍后再试。</small></div>
                  <template v-else-if="configPage === 'properties'">
                    <div class="preview-search canvas-list-search"><span class="canvas-search-icon" aria-hidden="true">⌕</span><input v-model="canvasSearchQuery" placeholder="搜索小区、位置或户型"><button v-if="canvasSearchQuery" type="button" @click="canvasSearchQuery = ''">×</button></div>
                    <template v-if="configAudience === 'operator'"><div class="preview-filter canvas-filter-row"><button v-for="status in canvasAdminPropertyFilters" :key="status.key" type="button" :class="{ active: canvasStatusFilter === status.key }" @click="canvasStatusFilter = status.key">{{ status.label }}</button></div><div class="canvas-filter-selects"><select v-model="canvasAreaFilter"><option>全部</option><option v-for="area in publicConfig.locations" :key="area.id || area.label">{{ area.label }}</option></select><select v-model="canvasRentTypeFilter"><option>全部</option><option>整租</option><option>合租</option></select><select v-model="canvasBudgetFilter"><option>不限</option><option>≤1500</option><option>1500–2500</option><option>2500–3500</option><option>≥3500</option></select></div></template>
                    <template v-else><button type="button" class="canvas-favorite-filter" :class="{ active: canvasFavoriteOnly }" @click="canvasFavoriteOnly = !canvasFavoriteOnly"><span aria-hidden="true">♡</span>{{ canvasFavoriteOnly ? '只看已关注' : '只看关注' }}</button><div class="canvas-area-filter"><strong>地区</strong><div class="preview-filter canvas-filter-row"><button type="button" :class="{ active: canvasAreaFilter === '全部' }" @click="canvasAreaFilter = '全部'">全部</button><button v-for="area in publicConfig.locations" :key="area.id || area.label" type="button" :class="{ active: canvasAreaFilter === area.label }" @click="canvasAreaFilter = area.label">{{ area.label }}</button></div></div><button type="button" class="canvas-property-filter-toggle" :aria-expanded="canvasPropertyFiltersOpen" @click="canvasPropertyFiltersOpen = !canvasPropertyFiltersOpen"><span><i aria-hidden="true">☷</i> 更多筛选</span><small>{{ canvasBudgetFilter === '不限' ? '预算不限' : canvasBudgetFilter }} · {{ canvasRentTypeFilter === '全部' ? '住法不限' : canvasRentTypeFilter }}</small><b :class="{ expanded: canvasPropertyFiltersOpen }">⌄</b></button><div v-if="canvasPropertyFiltersOpen" class="canvas-property-extra-filters"><div><strong>月租预算</strong><div class="preview-filter canvas-filter-row"><button v-for="budget in ['不限', '≤1500', '1500–2500', '2500–3500', '≥3500']" :key="budget" type="button" :class="{ active: canvasBudgetFilter === budget }" @click="canvasBudgetFilter = budget">{{ budget }}</button></div></div><div><strong>住法</strong><div class="preview-filter canvas-filter-row"><button v-for="kind in ['全部', '整租', '合租']" :key="kind" type="button" :class="{ active: canvasRentTypeFilter === kind }" @click="canvasRentTypeFilter = kind">{{ kind === '全部' ? '不限' : kind }}</button></div></div></div></template>
                    <div v-if="configAudience === 'customer'" class="preview-masonry-flow">
                      <div v-for="(column, columnIndex) in previewCustomerMasonryColumns" :key="`property-column-${columnIndex}`" class="preview-masonry-column">
                        <article v-for="item in column" :key="item.id" class="preview-property-card preview-masonry-card" @click="openPreviewProperty(item)"><div class="preview-card-cover" :class="{ 'preview-cover-placeholder': !item.media?.length }" :style="previewCoverStyle(item)"><template v-if="!item.media?.length"><img class="canvas-property-placeholder" :src="propertyPlaceholderIcon" alt=""><small class="preview-placeholder-label">暂无实拍图</small></template><em>{{ previewStatusLabel(item.status) }}</em><i v-if="item.media?.some(media => media.type === 'video')" class="preview-video-play">▶</i><small v-if="previewChannelLabel(item)" class="preview-cover-location">⌖ {{ previewChannelLabel(item) }}</small><button type="button" class="canvas-favorite-toggle" :class="{ active: canvasPreviewFavorites.includes(item.id) }" @click.stop="toggleCanvasFavorite(item.id)">{{ canvasPreviewFavorites.includes(item.id) ? '♥' : '♡' }}</button></div><section><b>{{ previewListingTitle(item) }}</b><div class="preview-tag-stack" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`"><small v-if="previewTagGroups(item).primary.length" class="preview-tag-row primary"><i v-for="tag in previewTagGroups(item).primary" :key="tag">{{ tag }}</i></small></div><strong>¥ {{ item.rentMonthly }}<em>/月</em></strong></section></article>
                      </div>
                    </div>
                    <div v-else class="preview-property-flow">
                      <article v-for="item in previewProperties" :key="item.id" class="preview-property-card" @click="openPreviewProperty(item)"><div class="preview-card-cover" :class="{ 'preview-cover-placeholder': !item.media?.length }" :style="previewCoverStyle(item)"><template v-if="!item.media?.length"><img class="canvas-property-placeholder" :src="propertyPlaceholderIcon" alt=""><small class="preview-placeholder-label">暂无实拍图</small></template><em>{{ previewStatusLabel(item.status) }}</em></div><section><b>{{ previewListingTitle(item) }}</b><small v-if="configAudience === 'operator' && publicConfig.listingTitle.adminEnabledFields.includes('unit')" class="preview-operator-doorplate">门牌 · {{ [item.buildingNo, item.unitNo && `${String(item.unitNo).replace(/单元$/, '')}单元`, item.roomNo && `${String(item.roomNo).replace(/室$/, '')}室`].filter(Boolean).join('') || '待补充' }}</small><div class="preview-tag-stack" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`"><template v-if="publicConfig.listingTitle.listTagLayout === 'grouped'"><small v-if="previewTagGroups(item).primary.length" class="preview-tag-row primary"><i v-for="tag in previewTagGroups(item).primary" :key="tag">{{ tag }}</i></small><small v-if="previewTagGroups(item).secondary.length" class="preview-tag-row secondary"><i v-for="tag in previewTagGroups(item).secondary" :key="tag">{{ tag }}</i></small></template><small v-else-if="previewTagGroups(item).all.length" class="preview-tag-row all"><i v-for="tag in previewTagGroups(item).all" :key="tag">{{ tag }}</i></small></div><strong>¥ {{ item.rentMonthly }}<em>/月</em></strong></section></article>
                    </div>
                    <div v-if="!previewProperties.length" class="canvas-properties-empty"><span class="canvas-properties-empty-icon">⌕</span><strong>{{ canvasPropertyEmptyTitle }}</strong><small>可以调整筛选条件，或告诉小皮你的预算和入住要求。</small><button type="button" @click="handleCanvasPropertyEmptyAction">{{ canvasPropertyEmptyAction }}</button><a v-if="canvasHasPropertyFilters && !canvasFavoriteOnly" href="#" @click.prevent="canvasPageFilterReset">清除筛选条件</a></div>
                  </template>
                  <template v-if="canvasDataState === 'normal' && configPage === 'propertyDetail'">
                    <div class="detail-canvas" :class="{ operator: configAudience === 'operator' }">
                      <div class="detail-canvas-scroll">
                        <div class="detail-canvas-cover" :class="{ 'is-placeholder': !previewPrimaryProperty?.media?.length }" :style="previewCoverStyle(previewPrimaryProperty, canvasMediaIndex)"><div class="detail-canvas-media-nav"><button type="button" @click="backCanvasPage">‹</button><i>•••　⊙</i></div><div v-if="!previewPrimaryProperty?.media?.length" class="detail-canvas-empty-media"><img :src="propertyPlaceholderIcon" alt=""><b>暂无实拍图</b><small>联系小皮了解房源详情</small></div><small v-else>实拍 {{ previewPrimaryProperty.media.length }} 张</small><div v-if="previewPrimaryProperty?.media?.length && previewPrimaryProperty.media.length > 1" class="detail-canvas-media-tabs"><button v-for="(media, index) in previewPrimaryProperty.media" :key="`${media.url}-${index}`" type="button" :class="{ video: media.type === 'video', active: index === canvasMediaIndex }" @click="canvasMediaIndex = index; media.type === 'video' && setCanvasNotice('预览显示视频封面，无法播放原视频。')">{{ media.type === 'video' ? '视频' : `${index + 1}` }}</button></div></div>
                        <section v-if="detailModuleEnabled('overview')" data-canvas-area="overview" class="detail-canvas-card overview-card" @click="focusCanvasArea('overview')">
                          <em v-if="configAudience === 'operator' && previewAdminStatusLabel" class="detail-canvas-status-preview">{{ previewAdminStatusLabel }}</em>
                          <div class="detail-canvas-price">¥{{ previewPrimaryProperty?.rentMonthly || 0 }}<small>/月</small><button v-if="configAudience === 'customer'" type="button" :class="{ active: canvasPreviewFavorites.includes(previewPrimaryProperty?.id || '') }" @click="previewPrimaryProperty && toggleCanvasFavorite(previewPrimaryProperty.id)">{{ canvasPreviewFavorites.includes(previewPrimaryProperty?.id || '') ? '♥' : '♡' }}</button></div>
                          <strong class="detail-canvas-title">{{ previewPrimaryProperty?.title || previewPrimaryProperty?.community || '房源详情' }}</strong>
                          <span v-if="configAudience === 'operator'" class="detail-canvas-address">{{ previewPrimaryProperty?.location || '位置待补充' }}</span>
                          <small v-if="previewOperatorDoorplate" class="preview-operator-doorplate">门牌定位 · {{ previewOperatorDoorplate }}</small>
                          <div v-if="previewPrimaryProperty?.listingSource === '转租'" class="detail-canvas-meta"><em>转租 · 剩余 {{ previewPrimaryProperty?.remainingLeaseMonths || 0 }} 个月</em></div>
                          <span v-if="configAudience !== 'operator'" class="detail-canvas-address">{{ previewPrimaryProperty?.location || '位置待补充' }}</span>
                          <div class="detail-canvas-facts"><i><small>{{ previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.wholeAreaSqm ? '整套面积' : '面积' }}</small>{{ previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.wholeAreaSqm ? `${previewPrimaryProperty.wholeAreaSqm}㎡` : previewPrimaryProperty?.areaSqm ? `${previewPrimaryProperty.areaSqm}㎡` : '待补充' }}</i><i><small>{{ previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.wholeLayout ? '整套户型' : '户型' }}</small>{{ previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.wholeLayout ? previewPrimaryProperty.wholeLayout : previewPrimaryProperty?.layout || '待补充' }}</i><i><small>楼层</small>{{ previewPrimaryProperty?.floorInfo || '待补充' }}</i><i v-if="previewPrimaryProperty?.orientation"><small>朝向</small>{{ previewPrimaryProperty.orientation }}</i><i v-if="previewPrimaryProperty?.elevator"><small>电梯</small>{{ previewPrimaryProperty.elevator }}</i><i><small>租赁方式</small>{{ previewPrimaryProperty?.rentType || '待补充' }}</i></div>
                        </section>
                        <section v-if="previewPrimaryProperty?.depositPayment || previewPrimaryProperty?.feeText || previewPrimaryProperty?.utilitiesText || previewPrimaryProperty?.listingSource === '转租' || (previewPrimaryProperty?.rentType === '合租' && (previewPrimaryProperty?.layout || previewPrimaryProperty?.areaSqm))" class="detail-canvas-card detail-canvas-info"><b>租赁信息</b><small class="detail-canvas-section-copy">费用与租期说明</small><div><i v-if="previewPrimaryProperty?.depositPayment"><small>押付方式</small>{{ previewPrimaryProperty.depositPayment }}</i><i v-if="previewPrimaryProperty?.feeText"><small>中介费</small>{{ previewPrimaryProperty.feeText }}</i><i v-if="previewPrimaryProperty?.utilitiesText" class="wide"><small>水电杂费</small>{{ previewPrimaryProperty.utilitiesText }}</i><i v-if="previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.layout"><small>实际使用空间</small>{{ previewPrimaryProperty.layout }}</i><i v-if="previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.areaSqm"><small>实际使用面积</small>{{ previewPrimaryProperty.areaSqm }}㎡</i><i v-if="previewPrimaryProperty?.listingSource === '转租'"><small>剩余租期</small>{{ previewPrimaryProperty.remainingLeaseMonths || 0 }} 个月</i></div></section>
                        <section v-if="detailModuleEnabled('location') && previewHasLocation" data-canvas-area="location" class="detail-canvas-card detail-canvas-location" @click="canvasDetailModal = 'map'"><b>{{ detailModuleTitle('location', '位置与周边') }}</b><small>{{ previewPrimaryProperty?.location || '已保存房源定位' }}</small><div><span><strong>查看地图位置</strong><small>已保存房源定位</small></span><i>›</i></div></section>
                        <section v-if="detailModuleEnabled('facilities')" data-canvas-area="facilities" class="detail-canvas-card detail-canvas-facilities" @click="focusCanvasArea('facilities')"><header><span><b>{{ detailModuleTitle('facilities', '房屋设施') }}</b><small>家具家电等可勾选配置</small></span><i>{{ previewDetailAmenities.length }} 项设施</i></header><template v-if="previewDetailFacilityGroups.length"><div v-for="group in previewDetailFacilityGroups" :key="group.title" class="detail-canvas-facility-group"><small>{{ group.title }}</small><div><i v-for="facility in group.items" :key="facility.key"><b>{{ detailFacilityIcon(facility.key) }}</b>{{ facility.label }}</i></div></div></template><p v-else>家具家电信息待补充，可联系小皮确认</p></section>
                        <section v-if="detailModuleEnabled('description')" data-canvas-area="description" class="detail-canvas-card detail-canvas-description" @click="focusCanvasArea('description')"><b>{{ detailModuleTitle('description', '房源描述') }}</b><p>{{ previewDescription(previewPrimaryProperty) || '暂无描述' }}</p></section>
                        <section v-if="detailModuleEnabled('nearby')" data-canvas-area="nearby" class="detail-canvas-card detail-canvas-nearby"><header><span><b>{{ detailModuleTitle('nearby', '附近房源') }}</b><small>距此房源 {{ publicConfig.propertyDetail.nearby.radiusKm }} 公里内</small></span><i>{{ previewNearbyProperties.length ? `${previewNearbyProperties.length} 套` : '继续找房' }}</i></header><article v-for="property in previewNearbyProperties" :key="property.id" @click="openPreviewProperty(property)"><div :style="previewCoverStyle(property)"></div><span><b>{{ previewListingTitle(property) }}</b><small>{{ property.layout || property.community }}</small><strong>¥{{ property.rentMonthly }}<i>/月</i></strong></span></article><button v-if="!previewNearbyProperties.length" class="nearby-canvas-empty" type="button" @click="navigateCanvasPage('properties')">附近暂时没有其他可租房源 · 查看全部</button></section>
                      </div>
                      <template v-if="configAudience === 'customer'"><button class="detail-canvas-floating-appointment" type="button" @click="canvasIdentityState === 'guest' ? navigateCanvasPage('authLogin') : (canvasDetailModal = 'appointment')"><b>预约</b><small>看房</small></button><button class="detail-canvas-floating-share" type="button" @click="previewOverlay = 'share'">分享</button><div class="detail-canvas-actions customer-actions"><span class="detail-canvas-signature"><i>皮</i><b>{{ publicConfig.heroTitle || '小皮租房' }}</b></span><button type="button" @click="setCanvasNotice('预览不会发起客服会话。')">在线咨询</button><button type="button" @click="setCanvasNotice(`客服电话：${publicConfig.contactPhone || '暂未配置'}`)">打电话</button></div></template>
                      <div v-else class="detail-canvas-actions operator-actions"><div class="detail-canvas-admin-lead"><button type="button" v-if="previewPrimaryProperty?.status !== 'offline'" @click="previewOverlay = 'share'">分享好友</button><button type="button" @click="startCanvasPropertyEdit(previewPrimaryProperty)">编辑房源</button></div><div class="detail-canvas-admin-status"><button v-for="(action, index) in previewAdminStatusActions" :key="action" type="button" :class="{ primary: index === 0 }" @click="submitCanvasPreview">{{ action }}</button></div><button class="detail-canvas-delete" type="button" @click="setCanvasNotice('预览不会删除房源。')">删除房源</button></div>
                    </div>
                  </template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'requests'"><section class="request-canvas-hero canvas-editable-copy" title="点击编辑页面文案" @click="openCanvasNode('page.heading')"><strong>{{ publicConfig.pagePresentations.requests.title || '小皮帮你找房' }}</strong><b>说清需求，比反复筛选更高效</b><small>{{ publicConfig.pagePresentations.requests.subtitle || '告诉我们你的找房需求' }}</small></section><form class="preview-form-card canvas-form request-canvas-form" @submit.prevent="submitCanvasPreview"><div class="request-canvas-field"><label>联系手机号 *</label><span class="request-canvas-phone">{{ canvasForm.phone ? `已绑定 ${canvasForm.phone.slice(0, 3)}****${canvasForm.phone.slice(-4)}` : '提交时需绑定微信手机号' }}</span></div><div class="request-canvas-field"><label>意向地区 *</label><input v-model="canvasForm.location" placeholder="例如：佛祖岭、金融港"></div><div class="request-canvas-field"><label>期望户型</label><input v-model="canvasForm.layout" placeholder="例如：一室一厅 / 合租主卧"></div><div class="request-canvas-field"><label>月租预算</label><div class="canvas-budget-inputs"><input v-model="canvasForm.budgetMin" inputmode="numeric" placeholder="最低"><span>至</span><input v-model="canvasForm.budgetMax" inputmode="numeric" placeholder="最高"><span>元</span></div></div><div class="request-canvas-field"><label>预计入住</label><button class="canvas-picker-field" type="button" @click="setCanvasNotice('日期选择仅作画布交互预览。')">{{ canvasForm.moveInDate || '选择日期' }} <i>›</i></button></div><div class="request-canvas-field request-canvas-textarea"><label>补充要求</label><textarea v-model="canvasForm.note" maxlength="300" placeholder="通勤地点、整租/合租、楼层、宠物、采光等"></textarea></div><small class="request-canvas-privacy">提交即表示同意小皮使用已绑定手机号联系你，仅用于本次找房服务。</small><button type="submit">提交找房需求</button><button class="request-canvas-contact" type="button" @click="setCanvasNotice('联系入口只用于预览，不会拨打电话。')">想马上确认房源？联系小皮 ›</button></form></template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'publish'"><div class="publish-canvas-tip"><strong>提供房源线索</strong><span>提交基础信息后，小皮会电话联系并实地核验。正式房源由运营人员审核发布。</span></div><form class="preview-form-card canvas-form publish-canvas-form" @submit.prevent="submitCanvasPreview"><label>联系人</label><input v-model="canvasForm.sourceName" placeholder="怎么称呼你"><label>联系电话</label><input v-model="canvasForm.sourcePhone" inputmode="tel" maxlength="11" placeholder="方便小皮联系"><label>小区</label><input v-model="canvasForm.community" placeholder="请输入小区名称"><label>位置</label><input v-model="canvasForm.location" placeholder="区域 / 地铁 / 商圈"><label>户型</label><input v-model="canvasForm.layout" placeholder="例如：一室一厅"><label>预期租金</label><input v-model="canvasForm.expectedRent" inputmode="numeric" placeholder="请输入租金"><label>补充说明</label><textarea v-model="canvasForm.note" maxlength="500" placeholder="入住时间、房屋情况等"></textarea></form><button type="button" class="publish-submit-button" @click="submitCanvasPreview">提交房源线索</button></template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'me'">
                    <div v-if="canvasNotice" class="canvas-preview-notice" role="status">{{ canvasNotice }}</div>
                    <div class="preview-profile" :class="{ guest: canvasIdentityState === 'guest' }" @click="canvasIdentityState === 'guest' && navigateCanvasPage('authLogin')"><div class="preview-profile-main"><i>{{ previewIdentity.mark }}</i><div><b>{{ previewIdentity.name }}</b><span>{{ previewIdentity.caption }}</span></div><small>{{ canvasIdentityState === 'guest' ? '›' : '预览' }}</small></div><div class="preview-profile-bottom"><span>{{ canvasIdentityState === 'guest' ? '点击直接微信登录' : '绑定手机号，方便联系与身份识别' }}</span><b>{{ canvasIdentityState === 'guest' ? '进入' : '已登录' }}</b></div></div>
                    <template v-if="configAudience === 'customer'">
                      <h4><strong>我的找房</strong><span>常用服务</span></h4>
                      <div class="preview-tool-grid">
                        <article @click="navigateCanvasPage('properties')"><div class="preview-tool-copy"><b>全部房源</b><small>无需登录</small></div></article>
                        <article @click="canvasFavoriteOnly = true; navigateCanvasPage('properties')"><div class="preview-tool-copy"><b>我的关注</b><small>{{ canvasIdentityState === 'guest' ? '登录后查看' : '收藏的房源' }}</small></div></article>
                        <article @click="navigateCanvasPage('footprints')"><div class="preview-tool-copy"><b>浏览足迹</b><small>{{ canvasIdentityState === 'guest' ? '登录后查看' : '最近看过' }}</small></div></article>
                        <article @click="navigateCanvasPage('requests')"><div class="preview-tool-copy"><b>找房需求</b><small>提交时登录</small></div></article>
                      </div>
                      <article class="preview-menu-row" @click="navigateCanvasPage('requestHistory')">我的预约与找房记录　›<small>{{ canvasIdentityState === 'guest' ? '登录后可查看' : '查看预约、订阅和跟进进度' }}</small></article>
                      <article class="preview-menu-row" @click="navigateCanvasPage('publish')">有房源想推荐？　›<small>无需登录，提交后电话核验</small></article>
                      <p v-if="canvasIdentityState === 'user'" class="canvas-private-data-note">此处未绑定具体微信账号。个人收藏、足迹和找房记录仍由小程序用户身份读取，画布不会代取或伪造。</p>
                    </template>
                    <template v-else>
                      <h4>运营工作台</h4>
                      <div class="preview-stats"><button type="button" @click="canvasStatusFilter = 'draft'; navigateCanvasPage('properties')">{{ properties.filter(item => item.status === 'draft').length }}<small>房源草稿</small></button><button type="button" @click="canvasAdminWorkbenchTab = 'leads'">{{ leads.filter(isPendingLead).length }}<small>待跟进线索</small></button><button type="button" @click="canvasAdminWorkbenchTab = 'requests'">{{ rentalRequests.filter(isNewRentalRequest).length }}<small>待联系需求</small></button></div>
                      <div class="preview-tool-grid"><article @click="startCanvasPropertyEdit()">＋ 录入房源<small>进入房源录入流程</small></article><article @click="navigateCanvasPage('properties')">▦ 房源管理<small>查看当前环境房源</small></article><article @click="canvasAdminWorkbenchTab = 'leads'; navigateCanvasPage('adminWorkbench')">房源线索<small>{{ leads.length }} 条记录</small></article><article @click="canvasAdminWorkbenchTab = 'interests'; navigateCanvasPage('adminWorkbench')">♡ 用户意向<small>{{ favoriteContacts.length }} 条关注记录</small></article></div>
                      <article class="preview-menu-row" @click="startCanvasPropertyEdit()">＋ 录入新房源<small>保存和发布在画布中为空操作</small></article>
                    </template>
                  </template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'footprints'"><section class="canvas-private-page"><template v-if="canvasIdentityState === 'guest'"><strong>登录后查看浏览足迹</strong><small>最近看过的房源会保存在微信账号中。</small><button type="button" @click="navigateCanvasPage('authLogin')">微信登录</button></template><template v-else><strong>浏览足迹属于微信用户个人数据</strong><small>当前 Admin 页面没有指定微信用户会话，不能代取或用房源列表伪造足迹。</small><button type="button" @click="navigateCanvasPage('properties')">继续找房</button></template></section></template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'requestHistory'"><section class="canvas-private-page"><template v-if="canvasIdentityState === 'guest'"><strong>登录后查看找房记录</strong><small>预约、片区订阅和找房需求会关联到微信账号。</small><button type="button" @click="navigateCanvasPage('authLogin')">微信登录</button></template><template v-else><strong>当前 Admin 未绑定微信用户身份</strong><small>个人预约和找房记录需要对应微信用户身份读取；画布不冒充用户，也不生成示例记录。</small><button type="button" @click="navigateCanvasPage('requests')">填写找房需求</button></template></section></template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'adminWorkbench'">
                    <div v-if="canvasNotice" class="canvas-preview-notice" role="status">{{ canvasNotice }}</div>
                    <div class="preview-admin-card"><b>{{ previewIdentity.name }}</b><span>{{ previewIdentity.caption }}</span></div>
                    <div class="preview-stats"><button type="button" @click="canvasAdminWorkbenchTab = 'properties'; canvasStatusFilter = 'draft'">{{ properties.filter(item => item.status === 'draft').length }}<small>房源草稿</small></button><button type="button" @click="canvasAdminWorkbenchTab = 'leads'">{{ previewAdminPendingCount }}<small>待联系</small></button><button type="button" @click="canvasAdminWorkbenchTab = 'properties'; canvasStatusFilter = 'incomplete'">{{ previewIncompletePropertyCount }}<small>待补资料</small></button></div>
                    <nav class="preview-admin-tabs"><button type="button" :class="{ active: canvasAdminWorkbenchTab === 'properties' }" @click="canvasAdminWorkbenchTab = 'properties'">房源管理 {{ properties.length }}</button><button type="button" :class="{ active: canvasAdminWorkbenchTab === 'leads' }" @click="canvasAdminWorkbenchTab = 'leads'">房源线索 {{ leads.length }}</button><button type="button" :class="{ active: canvasAdminWorkbenchTab === 'requests' }" @click="canvasAdminWorkbenchTab = 'requests'">客户需求 {{ rentalRequests.length }}</button><button type="button" :class="{ active: canvasAdminWorkbenchTab === 'interests' }" @click="canvasAdminWorkbenchTab = 'interests'">用户意向 {{ favoriteContacts.length }}</button></nav>
                    <section v-if="canvasAdminWorkbenchTab === 'properties'" class="canvas-workbench-list"><article v-for="item in properties.slice(0, 6)" :key="item.id" class="preview-menu-row canvas-workbench-row" @click="openPreviewProperty(item)"><span><b>{{ previewListingTitle(item) }}</b><small>{{ item.location || item.community }} · ¥{{ item.rentMonthly }}/月</small></span><em>{{ previewStatusLabel(item.status) }}</em><button type="button" @click.stop="startCanvasPropertyEdit(item)">编辑</button></article><button type="button" class="canvas-workbench-more" @click="navigateCanvasPage('properties')">打开房源列表</button><button type="button" class="canvas-workbench-more" @click="startCanvasPropertyEdit()">＋ 录入房源</button></section>
                    <section v-else-if="canvasAdminWorkbenchTab === 'leads'" class="canvas-workbench-list"><article v-for="item in leads.slice(0, 6)" :key="item.id" class="preview-menu-row canvas-workbench-row"><span><b>{{ item.community || item.sourceName || '房源线索' }}</b><small>{{ item.sourceName }} · {{ item.sourcePhone }} · {{ item.location || '位置待补充' }}</small></span><em>{{ item.status }}</em><button type="button" @click="setCanvasNotice('画布不会转换线索或写入房源数据。')">转房源</button></article><div v-if="!leads.length" class="preview-empty">暂无房源线索</div></section>
                    <section v-else-if="canvasAdminWorkbenchTab === 'requests'" class="canvas-workbench-list"><article v-for="item in rentalRequests.slice(0, 6)" :key="item.id" class="preview-menu-row canvas-workbench-row"><span><b>{{ item.location || '找房需求' }}</b><small>{{ item.phone }} · {{ item.layout || '户型待补充' }} · {{ item.budgetMin || 0 }}–{{ item.budgetMax || '不限' }} 元/月</small></span><em>{{ item.status || 'new' }}</em><button type="button" @click="setCanvasNotice('画布不会更新需求跟进状态。')">跟进</button></article><div v-if="!rentalRequests.length" class="preview-empty">暂无客户需求</div></section>
                    <section v-else class="canvas-workbench-list"><article v-for="item in favoriteContacts.slice(0, 6)" :key="`${item.userId}-${item.propertyId}`" class="preview-menu-row canvas-workbench-row"><span><b>{{ item.propertyName }}</b><small>{{ item.nickname || '微信用户' }} · {{ item.phone || '手机号待绑定' }} · {{ formatTime(item.createdAt) }}</small></span><button type="button" :disabled="!item.phone" @click="setCanvasNotice(`联系人号码：${item.phone || '手机号待绑定'}`)">{{ item.phone ? '查看联系信息' : '手机号待绑定' }}</button></article><div v-if="!favoriteContacts.length" class="preview-empty">暂无用户意向</div></section>
                    <article class="preview-menu-row" @click="startCanvasPropertyEdit()">＋ 录入新房源<small>页面操作可走通，保存与状态变更不会写入</small></article>
                  </template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'adminPropertyEdit'">
                    <div class="canvas-editor-hero"><div><b>{{ canvasEditingExistingProperty ? '完善房源信息' : '开始录入房源' }}</b><small>位置、户型、配套、租金和素材分步填写</small></div><strong>{{ canvasEditStep + 1 }}/5<small>当前步骤</small></strong></div>
                    <nav class="preview-steps" aria-label="房源编辑步骤">
                      <button v-for="(step, index) in ['位置门牌', '户型出租', '配套与描述', '租金与入住', '图片与视频']" :key="step" type="button" :class="{ active: canvasEditStep === index }" @click="canvasEditStep = index">{{ String(index + 1).padStart(2, '0') }} {{ step }}</button>
                    </nav>
                    <div class="preview-edit-card">
                      <template v-if="canvasEditStep === 0"><b>01 · 小区与门牌</b><label>小区名称 *</label><input v-model="canvasPropertyDraft.community" placeholder="搜索并选择小区"><label>地图位置</label><input v-model="canvasPropertyDraft.location" placeholder="选择小区后确认地图位置"><button type="button" @click="setCanvasNotice('地图定位只展示选择流程，不会保存坐标。')">地图选点</button><label>门牌信息</label><div class="canvas-budget-inputs"><input v-model="canvasPropertyDraft.buildingNo" placeholder="楼栋"><input v-model="canvasPropertyDraft.unitNo" placeholder="单元"><input v-model="canvasPropertyDraft.roomNo" placeholder="房间号"></div><label>客户广告词</label><input v-model="canvasPropertyDraft.marketingPhrase" placeholder="不使用广告词 · 仅用于客户标题"><label>归属地区</label><select v-model="canvasPropertyDraft.location"><option value="">选择地区</option><option v-for="area in publicConfig.locations" :key="area.id || area.label">{{ area.label }}</option></select><div class="preview-map-placeholder">地图定位与坐标确认 · {{ canvasPropertyDraft.location || '未设置' }}</div></template>
                      <template v-else-if="canvasEditStep === 1"><b>02 · 户型与出租方式</b><label>出租方式</label><select v-model="canvasPropertyDraft.rentType"><option>整租</option><option>合租</option></select><label>出租来源</label><select v-model="canvasPropertyDraft.listingSource"><option>直租</option><option>转租</option></select><label>房间 / 客厅 / 厨房 / 卫生间 / 阳台</label><input v-model="canvasPropertyDraft.layout" placeholder="例如：两室一厅"><label>面积、楼层、朝向与电梯</label><div class="canvas-budget-inputs"><input v-model="canvasPropertyDraft.areaSqm" inputmode="numeric" placeholder="面积（㎡）"><input v-model="canvasPropertyDraft.floorInfo" placeholder="楼层"><input v-model="canvasPropertyDraft.orientation" placeholder="朝向"><select v-model="canvasPropertyDraft.elevator"><option value="">电梯</option><option>有电梯</option><option>无电梯</option></select></div><div class="preview-form-card"><label>房源摘要</label><strong>{{ canvasPropertyDraft.layout || '户型待补充' }}　·　{{ canvasPropertyDraft.areaSqm || '面积待补充' }}㎡</strong></div></template>
                      <template v-else-if="canvasEditStep === 2"><b>03 · 配套与描述</b><label>房屋设施</label><div class="preview-checks"><button v-for="facility in publicConfig.propertyDetail.facilities.filter(item => item.enabled)" :key="facility.key" type="button" :class="{ selected: canvasPropertyFacilities.includes(facility.label) }" @click="toggleCanvasPropertyFacility(facility.label)">{{ canvasPropertyFacilities.includes(facility.label) ? '✓ ' : '+ ' }}{{ facility.label }}</button></div><label>房源描述</label><textarea v-model="canvasPropertyDraft.description" placeholder="填写采光、楼层、朝向、入住要求、周边交通等信息"></textarea><label>入住规则</label><textarea v-model="canvasPropertyDraft.rulesText" placeholder="如：可做饭，宠物需沟通，女生优先"></textarea></template>
                      <template v-else-if="canvasEditStep === 3"><b>04 · 租金与入住</b><label>月租 *</label><input v-model="canvasPropertyDraft.rentMonthly" inputmode="numeric" placeholder="输入月租（元）"><label>中介费</label><select v-model="canvasPropertyDraft.feeText"><option value="">请选择</option><option>无中介费</option><option>有中介费</option></select><label>押付方式 *</label><select v-model="canvasPropertyDraft.depositPayment"><option value="">请选择押付方式</option><option>押一付一</option><option>押一付三</option><option>押二付一</option></select><label>可入住日期</label><input v-model="canvasPropertyDraft.availableDate" type="date"><label>水电与杂费</label><input v-model="canvasPropertyDraft.utilitiesText" placeholder="填写水电、物业等费用说明"></template>
                      <template v-else><b>05 · 图片与视频</b><label>封面素材</label><span>图片 ≥640px / ≤10MB · 视频 ≤20MB / 60秒</span><button type="button" @click="setCanvasNotice('画布不上传或保存任何媒体文件。')">＋ 图片　　＋ 视频</button><label>客户标题预览</label><strong>{{ canvasDraftTitle }}</strong><label>列表效果核对</label><span>客户瀑布流与管理员列表使用不同布局，标签规则由后台统一配置。</span></template>
                    </div>
                    <div class="preview-step-actions"><button type="button" :disabled="canvasEditStep === 0" @click="canvasEditStep = Math.max(0, canvasEditStep - 1)">上一步</button><button type="button" :disabled="canvasEditStep === 4" @click="canvasEditStep = Math.min(4, canvasEditStep + 1)">下一步</button><button type="button" @click="submitCanvasPreview">{{ canvasEditStep === 4 ? '保存草稿' : '保存并查看' }}</button><button v-if="canvasEditingExistingProperty" type="button" @click="setCanvasNotice('画布不会更新已有房源。')">更新房源</button></div>
                  </template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'authLogin'"><div class="preview-login-hero"><div class="preview-login-mark">小</div><small>{{ publicConfig.heroTitle || '小皮租房' }}</small><strong>微信登录</strong><p>登录后管理你的找房服务</p></div><form class="preview-login-card canvas-form" @submit.prevent="submitCanvasPreview"><header><div><b>微信登录</b><span>登录后可同步关注、足迹与找房进度</span></div><i>可信房源</i></header><button class="preview-wechat-login" type="button" @click="submitCanvasPreview"><i>微</i><span><b>微信一键登录</b><small>推荐使用</small></span><strong>›</strong></button><div class="preview-login-divider"><span>测试访问</span></div><div class="preview-login-credentials"><label>手机号<input v-model="canvasLoginPhone" inputmode="tel" placeholder="请输入手机号"></label><label>访问码<input v-model="canvasLoginAccessCode" inputmode="numeric" maxlength="6" placeholder="6 位数字"></label></div><button class="preview-code-login" type="submit">使用访问码登录</button><div class="preview-test-login"><small>隔离测试身份</small><button type="button" @click="switchCanvasIdentity('user')">普通用户 A</button><button type="button" @click="switchCanvasIdentity('user')">普通用户 B</button><button type="button" @click="switchCanvasIdentity('admin')">测试管理员</button><button type="button" @click="switchCanvasIdentity('guest')">禁用用户（验证拒绝）</button></div></form><p class="preview-login-note">登录即表示同意小程序隐私保护说明</p></template>
                </div>
                <div v-if="['home', 'properties', 'me', 'adminWorkbench'].includes(configPage)" class="mini-tabbar"><button :class="{ active: configPage === (configAudience === 'customer' ? 'home' : 'adminWorkbench') }" @click="switchCanvasTab(configAudience === 'customer' ? 'home' : 'adminWorkbench')"><img v-if="configAudience === 'customer'" :src="configPage === 'home' ? homeTabActiveIcon : homeTabIcon" alt=""><span v-else>⌂</span><small>{{ configAudience === 'customer' ? '首页' : '工作台' }}</small></button><button :class="{ active: configPage === 'properties' }" @click="switchCanvasTab('properties')"><img :src="configPage === 'properties' ? propertyTabActiveIcon : propertyTabIcon" alt=""><small>房源</small></button><button v-if="configAudience === 'customer'" :class="{ active: configPage === 'me' }" @click="switchCanvasTab('me')"><img :src="configPage === 'me' ? meTabActiveIcon : meTabIcon" alt=""><small>我的</small></button><button v-else :class="{ active: configPage === 'adminPropertyEdit' }" @click="startCanvasPropertyEdit()"><span>＋</span><small>录入</small></button></div>
              </div>
              </div>
              <div v-if="configPage === 'propertyDetail' && previewOverlay === 'share'" class="preview-overlay" @click="previewOverlay = 'page'"><div class="preview-overlay-card" @click.stop><button class="overlay-close" @click="previewOverlay = 'page'">×</button><span>微信聊天中的小程序分享卡</span><strong>{{ shareTitlePreview || previewPrimaryProperty?.title || previewPrimaryProperty?.community || '暂无可展示房源' }}</strong><div class="share-cover" :style="previewCoverStyle(previewPrimaryProperty)"></div><b>¥{{ previewPrimaryProperty?.rentMonthly || 0 }}/月</b><p>{{ previewPrimaryProperty?.community || '房源小区' }} · {{ previewShareTags(previewPrimaryProperty).join(' · ') || '真实好房' }}</p><small>{{ publicConfig.heroTitle || '小皮租房' }}</small><button type="button" @click="setCanvasNotice('分享卡仅预览，不会发送消息。')">发给微信好友</button></div></div>
              <div v-if="configPage === 'propertyDetail' && canvasDetailModal === 'appointment'" class="preview-overlay" @click="canvasDetailModal = ''"><form class="canvas-bottom-sheet canvas-form" @submit.prevent="submitCanvasPreview" @click.stop><header><div><strong>预约看房</strong><small>{{ previewPrimaryProperty?.title || previewPrimaryProperty?.community }}</small></div><button type="button" aria-label="关闭" @click="canvasDetailModal = ''">×</button></header><label>怎么称呼您</label><input v-model="canvasForm.contactName" maxlength="20" placeholder="例如：张女士"><label>联系手机号</label><input v-model="canvasForm.phone" inputmode="tel" maxlength="11" placeholder="请填写手机号"><label>什么时候方便</label><div class="canvas-appointment-times"><button v-for="time in ['今天 14:00–16:00', '今天 16:00–18:00', '明天 10:00–12:00', '明天 14:00–16:00']" :key="time" type="button" :class="{ active: canvasForm.preferredTime === time }" @click="canvasForm.preferredTime = time">{{ time }}</button></div><button class="canvas-submit-button" type="submit">提交预约</button><small>画布不会创建真实预约。</small></form></div>
              <div v-if="configPage === 'propertyDetail' && canvasDetailModal === 'map'" class="preview-overlay" @click="canvasDetailModal = ''"><section class="canvas-bottom-sheet canvas-map-sheet" @click.stop><header><div><strong>房源位置</strong><small>{{ previewPrimaryProperty?.location }}</small></div><button type="button" aria-label="关闭" @click="canvasDetailModal = ''">×</button></header><div class="canvas-map-placeholder"><b>⌖</b><span>{{ previewPrimaryProperty?.community }}</span></div><p>坐标：{{ previewPrimaryProperty?.latitude || '未设置' }}，{{ previewPrimaryProperty?.longitude || '未设置' }}</p><button class="canvas-submit-button" type="button" @click="canvasDetailModal = ''">返回房源详情</button></section></div>
            </aside>
          </div>
        </template>

        <LeadsPage v-else-if="tab === '线索管理'" :leads="leads" :loading="loading" :filter="leadFilter" @refresh="load" @filter="value => router.replace({ query: value === 'pending' ? { filter: value } : {} })" />
  </AdminLayout>
</template>
