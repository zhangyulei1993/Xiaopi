<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, ArrowUp, Delete, House, Plus, RefreshLeft } from '@element-plus/icons-vue'

import AdminLayout from '../layouts/AdminLayout.vue'
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
type TabKey = '运营总览' | '房源列表' | '线索管理' | '租客需求' | '用户意向' | '微信用户' | '小程序基础配置' | '小程序页面配置' | '管理员列表' | '临时访问码'

const apiBase = import.meta.env.VITE_API_BASE || 'https://api.xiaopizufang.cn/api'
const loggedIn = ref(Boolean(localStorage.getItem('xiaopi_admin_token')))
const route = useRoute()
const router = useRouter()
const tab = ref<TabKey>(tabFromRoute(route.params.tab) as TabKey)
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
const previewOverlay = ref<'page' | 'share' | 'poster'>('page')
const tagMagnifierOpen = ref(false)
type ConfigAudience = 'customer' | 'operator'
type DetailConfigTab = 'detail' | 'share' | 'poster'
type DetailSectionTab = 'overview' | 'location' | 'facilities' | 'description' | 'nearby'
const detailModuleOptions: { key: DetailSectionTab; label: string; helper: string }[] = [
  { key: 'overview', label: '顶部概况', helper: '面积、朝向、入住信息' },
  { key: 'location', label: '位置与周边', helper: '位置、地图与通勤' },
  { key: 'facilities', label: '设施与环境', helper: '设施勾选与环境条件' },
  { key: 'description', label: '房源描述', helper: '描述与入住说明' },
  { key: 'nearby', label: '附近推荐', helper: '同区域可租房源' },
]
const configAudience = ref<ConfigAudience>('customer')
const detailConfigTab = ref<DetailConfigTab>('detail')
const detailSectionTab = ref<DetailSectionTab>('overview')
type BasicConfigTab = 'brand' | 'coverage' | 'tags' | 'facilities'
type BasicTagPreviewTab = 'list' | 'detail' | 'share' | 'poster'
const basicConfigTab = ref<BasicConfigTab>('brand')
const basicTagPreviewTab = ref<BasicTagPreviewTab>('list')
const basicPhonePreviewOpen = ref(false)
const editingFacilityKey = ref('')
const configPageMeta = computed(() => pagePresentationOptions.find(page => page.key === configPage.value))
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
  detailSectionTab.value = area
  detailConfigTab.value = 'detail'
  previewOverlay.value = 'page'
  window.requestAnimationFrame(() => document.querySelector(`[data-canvas-area="${area}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'center' }))
}
const selectConfigPage = (key: string) => {
  const target = pagePresentationOptions.find(page => page.key === key)
  if (!target) return
  if (target.audience !== 'both' && target.audience !== configAudience.value) configAudience.value = target.audience as ConfigAudience
  configPage.value = target.key
  detailConfigTab.value = 'detail'
  detailSectionTab.value = 'overview'
  previewOverlay.value = 'page'
  tagMagnifierOpen.value = false
}
function switchConfigAudience(value: ConfigAudience) {
  configAudience.value = value
  configPage.value = value === 'operator' ? 'adminWorkbench' : 'home'
  previewSelectedPropertyID.value = ''
  detailConfigTab.value = 'detail'
  detailSectionTab.value = 'overview'
  previewOverlay.value = 'page'
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
const requestUpdatingID = ref('')
const propertyFilter = ref('all')
const channelLocationResults = ref<Record<number, LocationSearchItem[]>>({})
const channelLocationLoading = ref<Record<number, boolean>>({})
const channelMapPreview = ref<Record<number, string>>({})
const channelMapLoading = ref<Record<number, boolean>>({})
const channelMapCache = ref<Record<string, string>>({})
const channelMapRequest = ref<Record<number, number>>({})
const channelMapActiveMarker = ref<Record<number, string>>({})
const channelSelectedLocation = ref<Partial<Record<number, LocationSearchItem>>>({})
const activeChannelIndex = ref(0)
const activeChannel = computed(() => publicConfig.featuredChannels[activeChannelIndex.value])
function channelSetupStatus(item: ChannelConfig) {
  if (!item.title.trim()) return '待填写'
  if (!item.latitude || !item.longitude) return '待选点'
  return '已定位'
}
const defaultShareCard = () => ({ titlePrefix: '', titleFields: ['community', 'layout'], posterFields: ['rentMonthly', 'depositPayment', 'community', 'areaSqm', 'floorInfo', 'availableDate'] })
const defaultListingTitle = () => ({ customerAllowedFields: ['community', 'floor', 'layout', 'rentType', 'listingSource', 'fee', 'area'], enabledFields: ['community', 'layout', 'rentType', 'fee'], customerDetailFields: ['community', 'layout', 'rentType', 'fee'], customerShareFields: ['community', 'layout', 'rentType', 'fee'], customerPosterFields: ['community', 'layout', 'rentType', 'fee'], adminEnabledFields: ['unit'], listTagLayout: 'grouped' as const, detailTagLayout: 'wrap' as const, listTagOrder: ['rentType', 'listingSource', 'layout', 'fee', 'floor', 'area'], listTagTopFields: ['rentType', 'listingSource'], detailTagOrder: ['listingSource', 'rentType', 'fee', 'layout', 'area', 'floor'], detailTagTopFields: ['listingSource', 'rentType', 'fee'] })
const defaultPropertyDetail = () => ({
  modules: [
    { key: 'overview', title: '房屋概况', enabled: true }, { key: 'location', title: '位置与周边', enabled: true },
    { key: 'facilities', title: '房屋设施与环境', enabled: true }, { key: 'description', title: '房源描述', enabled: true }, { key: 'nearby', title: '附近房源', enabled: true },
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
const previewProperties = computed(() => previewPropertyOptions.value.slice(0, 2))
const previewSelectedPropertyID = ref('')
const previewPrimaryProperty = computed(() => previewPropertyOptions.value.find((item) => item.id === previewSelectedPropertyID.value) || previewPropertyOptions.value[0])
function openPreviewProperty(item?: Property) {
  if (!item) return
  previewSelectedPropertyID.value = item.id
  selectConfigPage('propertyDetail')
}
const previewIdentity = computed(() => {
  if (configAudience.value === 'operator') {
    const account = admins.value.find((item) => item.status === 'active') || admins.value[0]
    return { name: account?.nickname || '管理员', caption: account?.phone ? `管理员已登录 · ${account.phone}` : '管理员已登录', mark: '管' }
  }
  const adminIDs = new Set(admins.value.map((item) => item.userId))
  const account = users.value.find((item) => !adminIDs.has(item.id) && item.status !== 'disabled') || users.value[0]
  return { name: account?.nickname || '微信用户', caption: account?.phone ? `已登录 · ${account.phone}` : '普通用户已登录', mark: account?.nickname?.slice(0, 1) || '小' }
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
  const ordered = arrangedFields(publicConfig.listingTitle.detailTagOrder, publicConfig.listingTitle.customerDetailFields)
  const tagByField = (field: string) => previewPropertyTags(item, [field])[0]
  const top = ordered.filter(key => publicConfig.listingTitle.detailTagTopFields.includes(key)).map(tagByField).filter(Boolean)
  const bottom = ordered.filter(key => !publicConfig.listingTitle.detailTagTopFields.includes(key)).map(tagByField).filter(Boolean)
  return { top, bottom }
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
  return String(item?.community || item?.title || '房源').trim()
}
function previewListingAddress(item?: Property) {
  const raw = String(item?.location || '').trim()
  if (!raw) return '位置待补充'
  const district = raw.split(/[·・]/).map(part => part.trim()).find(part => /区$/.test(part))
  const landmark = raw.match(/[^省市区\s·・]*(?:大道|一路|二路|三路|路|街道|街|商圈)/)?.[0]
  return [district, landmark].filter(Boolean).join(' · ') || raw.split(/[·・]/)[0].trim()
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
function detailModuleEnabled(key: string) { return detailModule(key).enabled }
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
const previewEnvironmentHighlights = computed(() => {
  const item = previewPrimaryProperty.value
  if (!item) return []
  return [
    item.orientation && { label: '朝向', value: item.orientation },
    item.utilitiesText && { label: '水电杂费', value: item.utilitiesText },
    { label: '电梯', value: item.elevator || '步梯' },
    Number(item.metroMinutes) > 0 && { label: '步行至地铁', value: `${item.metroMinutes} 分钟` },
  ].filter((item): item is { label: string; value: string } => Boolean(item))
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
function previewCoverStyle(item?: Property) {
  const cover = item?.media?.find((media) => media.type === 'image')?.url
  return cover ? { backgroundImage: `url(${cover})`, backgroundSize: 'cover', backgroundPosition: 'center' } : {}
}
const shareFieldOptions = [
  { key: 'community', label: '小区名称' }, { key: 'layout', label: '户型 / 房间' }, { key: 'roomNo', label: '门牌信息' },
  { key: 'rentMonthly', label: '月租' }, { key: 'listingSource', label: '直租 / 转租' }, { key: 'rentType', label: '整租 / 合租' },
  { key: 'depositPayment', label: '押付方式' }, { key: 'areaSqm', label: '面积' }, { key: 'floorInfo', label: '楼层' }, { key: 'availableDate', label: '入住时间' },
]
const shareFieldListingMap: Record<string, string> = { layout: 'layout', roomNo: 'unit', listingSource: 'listingSource', rentType: 'rentType', areaSqm: 'area' }
const shareFieldEnabledForCustomer = (key: string) => !shareFieldListingMap[key] || publicConfig.listingTitle.customerShareFields.includes(shareFieldListingMap[key])
const publicConfig = reactive<PublicConfig>({ heroTitle: '', contactPhone: '', relatedPropertyRadiusKm: 3, locations: [], featuredChannels: [], pagePresentations: defaultPagePresentations(), shareCard: defaultShareCard(), listingTitle: defaultListingTitle(), propertyDetail: defaultPropertyDetail() })

const api = createAdminApi({ baseUrl: apiBase, onUnauthorized: () => { loggedIn.value = false; password.value = '' } })

const cloneConfig = (value: PublicConfig): PublicConfig => JSON.parse(JSON.stringify(value))
const applyConfig = (value: PublicConfig, markSaved = false) => {
  const next = cloneConfig(value)
  next.relatedPropertyRadiusKm = next.relatedPropertyRadiusKm || 3
  next.featuredChannels = (next.featuredChannels || []).map((item, index) => ({ ...item, latitude: item.latitude || 0, longitude: item.longitude || 0, icon: normalizeChannelStyle(item.icon, index), radiusKm: item.radiusKm || 3 }))
  next.pagePresentations = { ...defaultPagePresentations(), ...(next.pagePresentations || {}) }
  next.shareCard = { ...defaultShareCard(), ...(next.shareCard || {}) }
  next.listingTitle = { ...defaultListingTitle(), ...(next.listingTitle || {}) }
  next.propertyDetail = { ...defaultPropertyDetail(), ...(next.propertyDetail || {}) }
  next.propertyDetail.modules = next.propertyDetail.modules?.length ? next.propertyDetail.modules : defaultPropertyDetail().modules
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
  } catch (error) {
    pageError.value = error instanceof Error ? error.message : '数据加载失败'
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
    const auth = await api<{ accessToken: string }>('/v1/admin/login', { method: 'POST', body: JSON.stringify({ username: username.value, password: password.value }) })
    localStorage.setItem('xiaopi_admin_token', auth.accessToken)
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
  localStorage.removeItem('xiaopi_admin_token')
  loggedIn.value = false
  password.value = ''
}

async function updateRentalRequest(item: RentalRequest, status: string) {
  requestUpdatingID.value = item.id
  try {
    await api('/v1/admin/rental-requests/follow-up', { method: 'PATCH', body: JSON.stringify({ id: item.id, status, followUpNote: item.followUpNote || '' }) })
    item.status = status
  } catch (error) {
    pageError.value = error instanceof Error ? error.message : '更新需求失败'
  } finally {
    requestUpdatingID.value = ''
  }
}

function selectTab(value: string) {
  const next = value as TabKey
  tab.value = next
  void router.push({ name: 'admin', params: { tab: pathForTab(next) } })
}

router.afterEach((to) => {
  tab.value = tabFromRoute(to.params.tab) as TabKey
})

const formatTime = (value: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const propertyStatuses = [
  { key: 'published', label: '已发布', icon: '✓' }, { key: 'available', label: '可租', icon: '钥' },
  { key: 'rented', label: '已租', icon: '住' }, { key: 'offline', label: '已下架', icon: '下' },
  { key: 'draft', label: '未发布（草稿）', icon: '稿' },
]
const normalizedPropertyStatus = (status: string) => status === 'pending_review' ? 'draft' : status
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
  if (publicConfig.locations.length >= 20) return
  const channel = publicConfig.featuredChannels[publicConfig.locations.length]
  publicConfig.locations.push({ label: channel?.title || '', keyword: channel?.keyword || '' })
}

function removeLocation(index: number) {
  publicConfig.locations.splice(index, 1)
}

function addChannel() {
  if (publicConfig.featuredChannels.length >= 12) return
  const location = publicConfig.locations[publicConfig.featuredChannels.length]
  publicConfig.featuredChannels.push({ icon: 'commute', title: location?.label || '', subtitle: '', keyword: location?.keyword || '', latitude: 0, longitude: 0, radiusKm: 3 })
  activeChannelIndex.value = publicConfig.featuredChannels.length - 1
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

onMounted(() => { if (loggedIn.value) load().catch(() => undefined) })
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
        <el-alert v-if="pageError" class="page-error" :title="pageError" type="error" show-icon :closable="false" />

        <DashboardPage v-if="tab === '运营总览'" :loading="loading" :property-count="properties.length" :lead-count="leads.length" :user-count="users.length" :location-count="publicConfig.locations.length" @navigate="selectTab" />

        <WechatUsersPage v-else-if="tab === '微信用户'" :users="users" :loading="loading" @refresh="load" />

        <RentalRequestsPage v-else-if="tab === '租客需求'" :requests="rentalRequests" :loading="loading" :updating-id="requestUpdatingID" @refresh="load" @update="updateRentalRequest" />

        <IntentionsPage v-else-if="tab === '用户意向'" :contacts="favoriteContacts" :loading="loading" @refresh="load" />

        <template v-else-if="tab === '管理员列表'">
          <section class="page-header">
            <div><span>平台设置</span><h1>管理员列表</h1></div>
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
              <el-table-column label="操作" width="170" align="center"><template #default="scope"><div class="admin-actions"><el-button v-if="scope.row.status !== 'active'" link type="primary" @click="setAdminStatus(scope.row, 'active')">启用</el-button><el-popconfirm v-else title="停用后，该用户重新登录将失去管理员权限，确认停用？" width="260" @confirm="setAdminStatus(scope.row, 'disabled')"><template #reference><el-button link type="warning">停用</el-button></template></el-popconfirm><el-popconfirm title="删除后该用户将移出管理员列表，但微信用户账号保留。确认删除？" width="280" @confirm="deleteAdmin(scope.row)"><template #reference><el-button link type="danger">删除</el-button></template></el-popconfirm></div></template></el-table-column>
            </el-table>
          </section>
        </template>

        <template v-else-if="tab === '临时访问码'">
          <section class="page-header">
            <div><span>平台设置</span><h1>临时访问码</h1></div>
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
            <el-form label-position="top" class="basic-config-form"><div class="form-row two-columns"><el-form-item label="品牌名称" required><el-input v-model="publicConfig.heroTitle" maxlength="20" show-word-limit placeholder="例如：小皮租房" /><div class="form-tip">统一用于小程序顶部导航、微信登录、房源详情服务入口与分享落款。</div></el-form-item><el-form-item label="服务联系电话"><el-input v-model.trim="publicConfig.contactPhone" maxlength="20" placeholder="用于房源详情联系" /><div class="form-tip">统一用于房源详情“打电话”和找房需求页的电话入口；不影响在线客服。</div></el-form-item></div></el-form>
            <div class="basic-config-note"><b>字段边界</b><span>品牌名称是对外品牌文本；在线客服人员由微信小程序客服配置管理，不在此设置。普通用户与管理员的房源标签规则在“房源标签规则”中单独管理。</span></div>
          </section>
          <section v-else-if="basicConfigTab === 'coverage'" class="surface form-section basic-config-panel">
            <div class="section-heading"><span>02</span><div><h2>地区与关联范围</h2><p>用于频道归属、房源附近推荐和默认筛选；页面文案与背景仍在“页面配置”里管理。</p></div></div>
            <el-form label-position="top" class="basic-config-form"><el-form-item label="附近房源默认关联距离"><el-select v-model="publicConfig.relatedPropertyRadiusKm" style="width:100%"><el-option v-for="distance in [1, 2, 3, 5, 10]" :key="distance" :label="`${distance} 公里`" :value="distance" /></el-select><div class="form-tip">按地图坐标关联；详情页可在页面配置中单独调整展示范围。</div></el-form-item></el-form>
            <div class="basic-config-note"><b>地区入口与精选频道</b><span>它们是首页、房源筛选与附近推荐共用的数据。频道名称、地图位置和关联半径在此维护；首页卡片样式、文案和排序在“页面配置 → 首页”维护。</span></div>
            <div class="basic-coverage-editor">
              <div class="section-heading config-section-heading"><span>03</span><div><h2>地区入口数据</h2><p>首页和房源筛选共用的地区名称与检索关键词。</p></div><el-button type="primary" plain :icon="Plus" :disabled="publicConfig.locations.length >= 20" @click="addLocation">新增地区</el-button></div>
              <div v-if="publicConfig.locations.length" class="structured-list">
                <article v-for="(item, index) in publicConfig.locations" :key="`basic-location-${index}`" class="config-list-item"><div class="item-index">{{ String(index + 1).padStart(2, '0') }}</div><div class="item-fields location-fields"><label><span>显示名称</span><el-input v-model="item.label" maxlength="12" placeholder="例如：光谷" @input="syncChannelFromLocation(index)" /></label><label><span>搜索关键词</span><el-input v-model="item.keyword" maxlength="20" placeholder="例如：光谷" @input="syncChannelFromLocation(index)" /></label></div><div class="item-actions"><el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="上移" @click="moveItem(publicConfig.locations, index, -1)" /><el-button text :icon="ArrowDown" :disabled="index === publicConfig.locations.length - 1" aria-label="下移" @click="moveItem(publicConfig.locations, index, 1)" /><el-popconfirm title="确认删除这个地区入口？" @confirm="removeLocation(index)"><template #reference><el-button text type="danger" :icon="Delete" aria-label="删除" /></template></el-popconfirm></div></article>
              </div>
              <div v-else class="config-empty"><strong>还没有地理位置</strong><span>至少添加一个地区，用户才能从首页快速筛选房源。</span><el-button type="primary" plain :icon="Plus" @click="addLocation">添加第一个地区</el-button></div>
            </div>
            <div class="basic-coverage-editor">
              <div class="section-heading config-section-heading"><span>03</span><div><h2>精选频道数据</h2><p>在这里搜索地点、在地图上选点，并设置频道覆盖半径。首页只负责频道卡片的文案、样式和排序。</p></div><el-button type="primary" plain :icon="Plus" :disabled="publicConfig.featuredChannels.length >= 12" @click="addChannel">新增频道</el-button></div>
              <div v-if="publicConfig.featuredChannels.length" class="channel-editor-tabs"><button v-for="(item, index) in publicConfig.featuredChannels" :key="`basic-channel-tab-${index}`" type="button" :class="{ active: activeChannelIndex === index, configured: item.latitude && item.longitude }" @click="activeChannelIndex = index"><span>{{ index + 1 }}</span><strong>{{ item.title || `频道 ${index + 1}` }}</strong><small>{{ channelSetupStatus(item) }}</small></button></div>
              <div v-if="publicConfig.featuredChannels.length" class="channel-editor-grid">
                <article v-for="(item, index) in publicConfig.featuredChannels" v-show="activeChannelIndex === index" :key="`basic-channel-${index}`" class="channel-editor-card active">
                  <header><div class="channel-order"><span>频道数据</span><div><strong>频道 {{ index + 1 }}</strong><small>按“基础信息 → 地点 → 范围”完成配置</small></div></div><div class="item-actions"><el-popconfirm title="确认删除这个精选频道？" @confirm="removeChannel(index)"><template #reference><el-button text type="danger" :icon="Delete" aria-label="删除" /></template></el-popconfirm></div></header>
                  <div class="channel-flow"><span class="active">1 基础信息</span><i>→</i><span :class="{ active: item.latitude && item.longitude }">2 地点定位</span><i>→</i><span :class="{ active: item.latitude && item.longitude && item.radiusKm > 0 }">3 关联范围</span></div>
                  <div class="channel-fields"><label><span>频道名称</span><el-input v-model="item.title" maxlength="12" placeholder="例如：金融港" @input="syncLocationFromChannel(index); invalidateChannelLocation(item, index)" /></label><label><span>搜索关键词（未选地点时使用）</span><el-input v-model="item.keyword" maxlength="20" placeholder="点击后筛选的关键词" @input="syncLocationFromChannel(index); invalidateChannelLocation(item, index)" /></label></div>
                </article>
              </div>
              <section v-if="activeChannel" class="channel-map-workbench"><header><div><strong>{{ activeChannel.title || `频道 ${activeChannelIndex + 1}` }} · 地图定位</strong><span>搜索结果、地图选点与中心坐标集中在这里处理。</span></div><div class="channel-location-action"><el-button :loading="channelLocationLoading[activeChannelIndex]" @click="searchChannelLocation(activeChannel, activeChannelIndex)">{{ activeChannel.latitude && activeChannel.longitude ? '重新搜索地点' : '搜索地点' }}</el-button><el-button v-if="activeChannel.latitude && activeChannel.longitude" text type="danger" @click="clearChannelLocation(activeChannel, activeChannelIndex)">清除位置</el-button></div></header><div v-if="channelMapLoading[activeChannelIndex]" class="channel-map-loading">正在加载已选地点…</div><div v-else-if="channelMapPreview[activeChannelIndex]" class="channel-map-picker" title="点击地图设置频道中心" @click="selectChannelMapPoint(activeChannel, activeChannelIndex, $event)"><img class="channel-map-preview" :src="channelMapPreview[activeChannelIndex]" alt="频道范围地图预览，点击可选点" /><div v-if="activeChannel.latitude && activeChannel.longitude && !channelLocationResults[activeChannelIndex]?.length" class="channel-radius-ring" :style="selectedRadiusStyle(activeChannel)"></div><button v-if="activeChannel.latitude && activeChannel.longitude && !channelLocationResults[activeChannelIndex]?.length" class="channel-selected-map-pin" type="button" :aria-label="`${channelSelectedLocation[activeChannelIndex]?.title || '频道中心'}，已选`" @click.stop><span>●</span></button><template v-for="marker in mapMarkerPositions(activeChannel, activeChannelIndex)" :key="marker.location.id"><button class="channel-map-marker" :class="{ active: channelMapActiveMarker[activeChannelIndex] === marker.location.id }" type="button" :style="{ left: `${marker.left}%`, top: `${marker.top}%` }" :aria-label="marker.location.title" @click.stop="toggleChannelMapMarker(activeChannelIndex, marker.location.id)"><span>{{ marker.label }}</span></button><div v-if="channelMapActiveMarker[activeChannelIndex] === marker.location.id" class="channel-map-callout" :style="{ left: `${marker.left}%`, top: `${marker.top}%` }" @click.stop><strong>{{ marker.location.title }}</strong><span>{{ marker.location.district }} · {{ marker.location.address }}</span><button type="button" @click="selectChannelLocation(activeChannel, activeChannelIndex, marker.location)">设为频道中心</button></div></template></div><small v-if="activeChannel.latitude && activeChannel.longitude">已选中心：{{ channelSelectedLocation[activeChannelIndex]?.title || '地图选点' }} · {{ activeChannel.latitude.toFixed(6) }}, {{ activeChannel.longitude.toFixed(6) }} · 半径 {{ activeChannel.radiusKm }} 公里</small><small v-else>点击地图可直接选点；搜索结果会以大头钉显示在图上。</small><div v-if="channelLocationResults[activeChannelIndex]?.length" class="admin-search-results channel-location-results"><button v-for="(location, locationIndex) in channelLocationResults[activeChannelIndex]" :key="location.id" type="button" @click="selectChannelLocation(activeChannel, activeChannelIndex, location)"><span><strong>{{ String.fromCharCode(65 + locationIndex) }} · {{ location.title }}</strong><small>{{ location.district }} · {{ location.address }}</small></span><em>{{ location.distance ? `${(location.distance / 1000).toFixed(1)}km` : '选用' }}</em></button></div><label class="channel-radius-field"><span>地图关联半径</span><el-input-number v-model="activeChannel.radiusKm" :min="0.1" :max="100" :precision="1" :step="0.5" controls-position="right" /><small>保存后，中心点半径内的房源会自动关联到此频道。</small></label></section>
              <div v-else class="config-empty"><strong>还没有精选频道</strong><span>添加频道后，用户可以从首页直接进入对应房源列表。</span><el-button type="primary" plain :icon="Plus" @click="addChannel">添加第一个频道</el-button></div>
            </div>
          </section>
          <section v-else-if="basicConfigTab === 'tags'" class="surface form-section basic-config-panel">
            <div class="section-heading"><span>03</span><div><h2>房源标签规则</h2><p>这里是标签配置唯一入口：首页推荐、房源列表、房源详情、微信分享卡和朋友圈海报均读取本规则。</p></div></div>
            <div class="share-field-editor listing-title-editor">
              <header><strong>第一步：开放给客户视角的可选标签</strong><span>管理员在此定义客户可选择哪些公开字段；开启门牌会出现住址暴露提醒。</span></header>
              <div class="listing-title-preview"><span>固定主标题</span><strong>小区名称 + 广告语（不作为可勾选标签）</strong></div>
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
              <header class="detail-subheading"><strong>朋友圈海报</strong><span>控制朋友圈海报可使用的标签；海报字段仍可在“房源详情 → 海报”单独排版。</span></header>
              <div class="share-field-options"><button v-for="field in listingTitleFieldOptions.filter(item => item.key !== 'community' && publicConfig.listingTitle.customerAllowedFields.includes(item.key))" :key="`basic-customer-poster-${field.key}`" type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.customerPosterFields.includes(field.key) }" :aria-pressed="publicConfig.listingTitle.customerPosterFields.includes(field.key)" @click="toggleCustomerPosterField(field.key, !publicConfig.listingTitle.customerPosterFields.includes(field.key))"><i>{{ publicConfig.listingTitle.customerPosterFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>
            </div>
            <div class="share-field-editor listing-title-editor">
              <header><strong>管理员视角的唯一差异</strong><span>管理员与客户复用公开标签；只可额外显示精确门牌，列表和详情同步生效。</span></header>
              <div class="share-field-options"><button type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.adminEnabledFields.includes('unit') }" :aria-pressed="publicConfig.listingTitle.adminEnabledFields.includes('unit')" @click="toggleAdminDoorplate(!publicConfig.listingTitle.adminEnabledFields.includes('unit'))"><i>{{ publicConfig.listingTitle.adminEnabledFields.includes('unit') ? '✓' : '' }}</i><span>额外显示门牌</span></button></div>
              <div class="listing-title-preview"><span>管理员最终效果</span><strong>{{ publicConfig.listingTitle.adminEnabledFields.includes('unit') ? '客户公开标签 + 5栋3单元101室' : '与客户公开标签完全一致' }}</strong></div>
            </div>
          </section>
          <section v-else class="surface form-section basic-config-panel facility-library-panel">
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
                  <div class="basic-preview-usage"><span>① 顶部导航 · 品牌名称</span><div class="basic-preview-nav">{{ publicConfig.heroTitle || '小皮租房' }}</div></div>
                  <div class="basic-preview-usage"><span>② 微信登录 · 品牌名称</span><div class="basic-preview-login"><i>皮</i><b>{{ publicConfig.heroTitle || '小皮租房' }}</b><small>登录后开始找房</small></div></div>
                  <div class="basic-preview-usage"><span>③ 房源详情 · 品牌名称</span><div class="basic-preview-service"><i>皮</i><div><b>{{ publicConfig.heroTitle || '小皮租房' }}</b><small>咨询费用、入住时间和看房安排</small></div><em>在线咨询 ›</em></div></div>
                  <div class="basic-preview-usage"><span>④ 房源详情 / 找房需求 · 服务联系电话</span><div class="basic-preview-phone"><div><b>{{ publicConfig.heroTitle || '小皮租房' }}客服</b><small>点击后调用系统拨号</small></div><button @click="basicPhonePreviewOpen = !basicPhonePreviewOpen">打电话</button><div v-if="basicPhonePreviewOpen" class="basic-phone-popover"><small>拨打服务电话</small><strong>{{ publicConfig.contactPhone || '联系电话待配置' }}</strong><span>真实小程序会交由系统拨号确认</span></div></div></div>
                  <div class="basic-preview-usage"><span>⑤ 分享卡与海报落款 · 品牌名称</span><div class="basic-preview-share-footer">⌁ {{ publicConfig.heroTitle || '小皮租房' }} · 小程序</div></div>
                </template>
                <template v-else-if="basicConfigTab === 'coverage'">
                  <div class="basic-preview-caption">首页地区入口</div>
                  <div class="mini-locations"><span class="active">附近</span><span v-for="(item, index) in publicConfig.locations.slice(0, 3)" :key="`basic-location-${index}`">{{ item.label || '未命名' }}</span></div>
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
                  <div class="basic-preview-radius">附近房源默认关联 <b>{{ publicConfig.relatedPropertyRadiusKm || 3 }} 公里</b></div>
                </template>
                <template v-else-if="basicConfigTab === 'tags'">
                  <div class="basic-tag-preview-tabs" role="tablist" aria-label="标签效果预览"><button :class="{ active: basicTagPreviewTab === 'list' }" @click="basicTagPreviewTab = 'list'">列表</button><button :class="{ active: basicTagPreviewTab === 'detail' }" @click="basicTagPreviewTab = 'detail'">详情</button><button :class="{ active: basicTagPreviewTab === 'share' }" @click="basicTagPreviewTab = 'share'">分享卡</button><button :class="{ active: basicTagPreviewTab === 'poster' }" @click="basicTagPreviewTab = 'poster'">朋友圈</button></div>
                  <template v-if="previewPrimaryProperty && basicTagPreviewTab === 'list'"><div class="basic-preview-caption">客户视角 · 首页推荐 / 房源列表</div><article class="preview-property-card basic-preview-property"><div :style="previewCoverStyle(previewPrimaryProperty)"><em>可租</em></div><section><b>{{ previewListingTitle(previewPrimaryProperty) }}</b><span>{{ previewListingAddress(previewPrimaryProperty) }}</span><small v-if="previewPropertyTags(previewPrimaryProperty).length" class="preview-tag-row secondary"><i v-for="tag in previewPropertyTags(previewPrimaryProperty)" :key="tag">{{ tag }}</i></small><strong>¥ {{ previewPrimaryProperty.rentMonthly }}<em>/月</em></strong></section></article></template>
                  <template v-else-if="previewPrimaryProperty && basicTagPreviewTab === 'detail'"><div class="basic-preview-caption">客户视角 · 房源详情</div><article class="basic-detail-preview"><div class="basic-detail-cover" :style="previewCoverStyle(previewPrimaryProperty)"></div><section><b>¥ {{ previewPrimaryProperty.rentMonthly }}<small>/月</small></b><strong>{{ previewListingTitle(previewPrimaryProperty) }}</strong><span>{{ previewListingAddress(previewPrimaryProperty) }}</span><div class="basic-detail-tags"><i v-for="tag in previewDetailTagGroups(previewPrimaryProperty).top" :key="`top-${tag}`">{{ tag }}</i><i v-for="tag in previewDetailTagGroups(previewPrimaryProperty).bottom" :key="`bottom-${tag}`">{{ tag }}</i></div></section></article></template>
                  <template v-else-if="previewPrimaryProperty && basicTagPreviewTab === 'share'"><div class="basic-preview-caption">微信好友 · 小程序分享卡</div><article class="basic-share-preview"><header><i>皮</i><span>{{ publicConfig.heroTitle || '小皮租房' }}</span></header><strong>{{ previewListingTitle(previewPrimaryProperty) }}</strong><div class="basic-share-card"><div :style="previewCoverStyle(previewPrimaryProperty)"></div><section><b>¥ {{ previewPrimaryProperty.rentMonthly }}<small>/月</small></b><strong>{{ previewPrimaryProperty.community }}</strong><span><i v-for="tag in previewShareTags(previewPrimaryProperty)" :key="tag">{{ tag }}</i></span></section></div><footer>⌁ 小程序</footer></article></template>
                  <template v-else-if="previewPrimaryProperty && basicTagPreviewTab === 'poster'"><div class="basic-preview-caption">朋友圈 · 房源海报</div><article class="basic-poster-preview"><div :style="previewCoverStyle(previewPrimaryProperty)"></div><b>¥ {{ previewPrimaryProperty.rentMonthly }}<small>/月</small></b><strong>{{ previewListingTitle(previewPrimaryProperty) }}</strong><span><i v-for="tag in previewPosterTags(previewPrimaryProperty)" :key="tag">{{ tag }}</i></span><em>{{ publicConfig.heroTitle || '小皮租房' }} · 扫码查看房源</em></article></template>
                  <div v-else class="preview-empty">暂无可预览房源</div>
                </template>
                <template v-else>
                  <div class="basic-preview-caption">房源详情 · 设施与环境</div>
                  <div class="basic-preview-facilities"><i v-for="facility in publicConfig.propertyDetail.facilities.filter(item => item.enabled).slice(0, 8)" :key="facility.key"><b>{{ detailFacilityIcon(facility.key) }}</b><span>{{ facility.label }}</span></i></div>
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
                  <el-form-item label="首页主标题" required><el-input v-model="publicConfig.pagePresentations.home.title" maxlength="30" show-word-limit placeholder="例如：今天想住哪里？" /></el-form-item>
                  <el-form-item label="搜索提示"><el-input v-model="publicConfig.pagePresentations.home.subtitle" maxlength="60" show-word-limit placeholder="例如：搜索小区、地铁或商圈" /></el-form-item>
                  <el-form-item label="首页背景图"><el-input v-model="publicConfig.pagePresentations.home.backgroundImage" placeholder="上传后自动填入，也可填写图片地址" /><div class="presentation-actions"><label class="upload-control"><input type="file" accept="image/jpeg,image/png,image/webp" @change="uploadPresentationBackground('home', $event)" />上传背景图</label><button v-if="publicConfig.pagePresentations.home.backgroundImage" @click="publicConfig.pagePresentations.home.backgroundImage = ''">移除背景图</button></div></el-form-item>
                </el-form>
              </section>

              <section v-if="configPage === 'home'" class="surface form-section config-section page-channel-presentation">
                <div class="section-heading config-section-heading"><span>02</span><div><h2>精选频道展示</h2><p>只影响首页卡片的顺序、样式与副标题；频道名称、地图定位和房源关联规则由“小程序基础配置 → 地区与关联范围”维护。</p></div></div>
                <div class="channel-editor-tabs"><button v-for="(item, index) in publicConfig.featuredChannels" :key="`home-channel-tab-${index}`" type="button" :class="{ active: activeChannelIndex === index }" @click="activeChannelIndex = index"><span>{{ index + 1 }}</span>{{ item.title || `频道 ${index + 1}` }}</button></div>
                <article v-for="(item, index) in publicConfig.featuredChannels" v-show="activeChannelIndex === index" :key="`home-channel-${index}`" class="channel-editor-card active"><header><div class="channel-order"><span>首页展示</span><strong>{{ item.title || `频道 ${index + 1}` }}</strong></div><div class="item-actions"><el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="首页上移" @click="moveItem(publicConfig.featuredChannels, index, -1)" /><el-button text :icon="ArrowDown" :disabled="index === publicConfig.featuredChannels.length - 1" aria-label="首页下移" @click="moveItem(publicConfig.featuredChannels, index, 1)" /></div></header><div class="channel-fields"><label class="icon-field"><span>卡片样式</span><el-select v-model="item.icon"><el-option v-for="style in channelStyleOptions" :key="style.value" :label="style.label" :value="style.value" /></el-select></label><label><span>卡片副标题</span><el-input v-model="item.subtitle" maxlength="20" placeholder="例如：通勤优选" /></label></div></article>
              </section>

              <section v-if="configPage !== 'home'" class="surface form-section config-section selected-page-config">
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
                <div class="section-heading config-section-heading"><span>02</span><div><h2>客户公开标签</h2><p>配置一次即同步列表、首页推荐、详情、好友分享与朋友圈海报；发布者无需、也不能为单套房源选择标签。</p></div></div>
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
                  <button type="button" :class="{ active: detailConfigTab === 'poster' }" role="tab" :aria-selected="detailConfigTab === 'poster'" @click="selectDetailConfigTab('poster')">朋友圈海报</button>
                </div>
                <el-select v-model="previewSelectedPropertyID" class="detail-preview-property" placeholder="选择真实房源">
                  <el-option v-for="property in previewPropertyOptions" :key="property.id" :label="`${property.community || property.title} · ¥${property.rentMonthly || 0}/月`" :value="property.id" />
                </el-select>
                <small>模块开关会同步影响管理员录入项、客户详情页与右侧画布；关闭只隐藏录入与展示，不删除已有房源数据。</small>
                <div class="detail-module-master-grid" aria-label="房源详情模块总控">
                  <article v-for="module in detailModuleOptions" :key="`module-master-${module.key}`" :class="{ active: detailSectionTab === module.key, disabled: !detailModule(module.key).enabled }" @click="selectDetailSectionTab(module.key)">
                    <span><b>{{ module.label }}</b><small>{{ module.helper }}</small></span>
                    <aside><em>{{ detailModule(module.key).enabled ? '已启用' : '已停用' }}</em><el-switch :model-value="detailModule(module.key).enabled" @click.stop @update:model-value="toggleDetailModule(module.key, $event)" /></aside>
                  </article>
                </div>
              </section>
              <section v-if="configPage === 'propertyDetail' && detailConfigTab === 'detail'" class="surface form-section config-section share-card-config">
                <div class="section-heading config-section-heading"><span>02</span><div><h2>房源详情模块</h2><p>这里配置录入和详情展示模块；标签规则统一在“小程序基础配置 → 房源标签规则”维护。</p></div><el-button plain @click="selectTab('小程序基础配置'); basicConfigTab = 'tags'">配置标签规则</el-button></div>
                  <div v-if="detailSectionTab === 'overview'" class="share-field-editor listing-title-editor">
                    <header><strong>顶部概况</strong><span>控制面积、朝向、楼内配置等附加字段的录入与展示。小区、户型、租金仍是发布房源的基础信息，会始终保留。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-module-form"><el-form-item label="模块标题"><el-input v-model="detailModule('overview').title" maxlength="12" /></el-form-item></el-form>
                    <div class="layout-choice-row detail-layout-choice"><div><strong>详情标签排版</strong><small>仅改变标题下标签的视觉排列；管理员门牌仍单独显示，不混入公开标签。</small></div><el-radio-group v-model="publicConfig.listingTitle.detailTagLayout"><el-radio-button value="wrap">自动折行</el-radio-button><el-radio-button value="grouped">分组底板</el-radio-button></el-radio-group></div>
                    <div class="tag-arranger detail-tag-arranger"><header><strong>拖动详情标签</strong><span>拖到上/下行调整位置；同一行拖到标签前调整左右顺序。</span><button type="button" class="tag-arranger-preset" @click="applyRecommendedDetailTagArrangement">推荐排序</button></header><div class="tag-arranger-lanes"><section class="tag-arranger-lane" @dragover.prevent @drop.prevent="moveDetailTag('top')"><div class="tag-arranger-lane-title"><b>第一行</b><small>租赁身份</small></div><div class="tag-arranger-items"><button v-for="key in detailTopTagFields" :key="`detail-top-${key}`" type="button" draggable="true" class="tag-drag-item" :class="{ dragging: draggingTag === key }" @dragstart="draggingTag = key" @dragend="draggingTag = ''" @dragover.prevent @drop.prevent.stop="moveDetailTag('top', key)"><i>⋮⋮</i>{{ listingTitleFieldOptions.find(field => field.key === key)?.label }}</button><span v-if="!detailTopTagFields.length" class="tag-drop-empty">拖到这里</span></div></section><section class="tag-arranger-lane" @dragover.prevent @drop.prevent="moveDetailTag('bottom')"><div class="tag-arranger-lane-title"><b>第二行</b><small>房屋参数</small></div><div class="tag-arranger-items"><button v-for="key in detailBottomTagFields" :key="`detail-bottom-${key}`" type="button" draggable="true" class="tag-drag-item" :class="{ dragging: draggingTag === key }" @dragstart="draggingTag = key" @dragend="draggingTag = ''" @dragover.prevent @drop.prevent.stop="moveDetailTag('bottom', key)"><i>⋮⋮</i>{{ listingTitleFieldOptions.find(field => field.key === key)?.label }}</button><span v-if="!detailBottomTagFields.length" class="tag-drop-empty">拖到这里</span></div></section></div></div>
                    <div class="basic-config-note"><b>详情标签已由基础配置统一控制</b><span>客户详情可展示字段：{{ publicConfig.listingTitle.customerDetailFields.filter(key => key !== 'community').map(key => listingTitleFieldOptions.find(field => field.key === key)?.label).filter(Boolean).join('、') || '仅显示小区主标题' }}</span><el-button plain @click="selectTab('小程序基础配置'); basicConfigTab = 'tags'">调整标签</el-button></div>
                  </div>
                  <div v-else-if="detailSectionTab === 'location'" class="share-field-editor listing-title-editor">
                    <header><strong>位置与周边</strong><span>小区与地图坐标用于归属地区和附近房源关联，始终保留；本模块控制通勤等周边补充字段及详情展示。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-module-form"><el-form-item label="模块标题"><el-input v-model="detailModule('location').title" maxlength="12" /></el-form-item></el-form>
                  </div>
                  <div v-else-if="detailSectionTab === 'facilities'" class="share-field-editor listing-title-editor">
                    <header><strong>设施与环境</strong><span>本页只控制该模块是否出现在录入页和详情页；设施的名称、分类、可勾选项统一由“小程序基础配置”维护。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-module-form"><el-form-item label="模块标题"><el-input v-model="detailModule('facilities').title" maxlength="12" /></el-form-item></el-form>
                    <div class="basic-config-note"><b>当前设施库：{{ publicConfig.propertyDetail.facilities.filter(item => item.enabled).length }} 项已启用</b><span>进入“小程序基础配置 → 房屋设施库”后，点击整张设施卡即可调整录入和详情可用项。</span><el-button plain @click="selectTab('小程序基础配置'); basicConfigTab = 'facilities'">配置设施库</el-button></div>
                  </div>
                  <div v-else-if="detailSectionTab === 'description'" class="share-field-editor listing-title-editor">
                    <header><strong>房源描述</strong><span>关闭后，录入页不再显示房源说明和入住规则输入；已有历史内容会保留，但不会展示。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-module-form"><el-form-item label="模块标题"><el-input v-model="detailModule('description').title" maxlength="12" /></el-form-item></el-form>
                  </div>
                  <div v-else class="share-field-editor listing-title-editor">
                    <header><strong>附近推荐房源</strong><span>这是纯展示模块，不增加录入项；仅推荐可租房源，并自动排除当前房源。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-nearby-form"><el-form-item label="模块标题"><el-input v-model="detailModule('nearby').title" maxlength="12" /></el-form-item><el-form-item label="关联逻辑"><el-radio-group v-model="publicConfig.propertyDetail.nearby.relation"><el-radio value="distance">按地图距离</el-radio><el-radio value="channel">按精选频道</el-radio></el-radio-group></el-form-item><el-form-item label="关联范围"><el-select v-model="publicConfig.propertyDetail.nearby.radiusKm"><el-option v-for="value in [1, 3, 5, 10]" :key="value" :label="`${value} 公里`" :value="value" /></el-select></el-form-item><el-form-item label="排序方式"><el-radio-group v-model="publicConfig.propertyDetail.nearby.sort"><el-radio value="distance">距离优先</el-radio><el-radio value="newest">最新发布优先</el-radio></el-radio-group></el-form-item><el-form-item label="展示数量"><el-radio-group v-model="publicConfig.propertyDetail.nearby.limit"><el-radio v-for="value in [2, 4, 6]" :key="value" :value="value">{{ value }} 套</el-radio></el-radio-group></el-form-item></el-form>
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
              <section v-if="configAudience === 'customer' && configPage === 'propertyDetail' && detailConfigTab === 'poster'" class="surface form-section config-section share-card-config">
                <div class="section-heading config-section-heading"><span>03</span><div><h2>朋友圈海报</h2><p>海报不区分角色，统一采用客户公开标签；右侧画布显示海报效果。</p></div></div>
                <div class="share-field-editor">
                  <header><strong>客户标签来源</strong><span>以下标签直接沿用“客户房源列表标签”配置；管理员门牌不会进入海报。</span></header>
                  <div class="listing-title-preview"><span>当前用于公开海报的标签</span><strong>{{ publicConfig.listingTitle.enabledFields.filter(key => key !== 'community').map(key => listingTitleFieldOptions.find(field => field.key === key)?.label).filter(Boolean).join('、') || '仅显示小区主标题' }}</strong></div>
                </div>
                <div class="share-field-editor">
                  <header><strong>海报详情字段</strong><span>选择 3–8 项，月租会自动作为重点信息展示。</span></header>
                  <div class="share-field-options"><button v-for="field in shareFieldOptions" :key="`poster-${field.key}`" type="button" class="field-check" :class="{ checked: publicConfig.shareCard.posterFields.includes(field.key) }" :disabled="!publicConfig.shareCard.posterFields.includes(field.key) && (publicConfig.shareCard.posterFields.length >= 8 || !shareFieldEnabledForCustomer(field.key))" :aria-pressed="publicConfig.shareCard.posterFields.includes(field.key)" @click="toggleShareField('posterFields', field.key, !publicConfig.shareCard.posterFields.includes(field.key))"><i>{{ publicConfig.shareCard.posterFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>
                  <div class="share-field-order"><article v-for="(key, index) in publicConfig.shareCard.posterFields" :key="`poster-order-${key}`"><i>{{ index + 1 }}</i><strong>{{ shareFieldLabel(key) }}</strong><span><button :disabled="index === 0" @click="moveShareField('posterFields', index, -1)">上移</button><button :disabled="index === publicConfig.shareCard.posterFields.length - 1" @click="moveShareField('posterFields', index, 1)">下移</button></span></article></div>
                </div>
              </section>
            </div>
          </div>
          </div>

            <aside v-if="tab === '小程序页面配置'" class="config-preview-column">
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
                  <div v-else class="tag-magnifier-detail" :class="`detail-tag-layout-${publicConfig.listingTitle.detailTagLayout}`"><span v-for="tag in previewDetailTagGroups(previewPrimaryProperty).top" :key="`top-${tag}`">{{ tag }}</span><span v-for="tag in previewDetailTagGroups(previewPrimaryProperty).bottom" :key="`bottom-${tag}`">{{ tag }}</span></div>
                </article>
              </section>
              <div v-if="configPage === 'home'" class="phone-preview">
                <div class="phone-status"><span>9:41</span><span>● ● ●</span></div>
                <div class="mini-navigation">{{ publicConfig.heroTitle || '小皮租房' }}</div>
                <div class="mini-content">
                  <section class="mini-hero"><span class="mini-city">武汉⌄</span><strong>{{ publicConfig.pagePresentations.home.title || '今天想住哪里？' }}</strong><i></i><div class="mini-search">{{ publicConfig.pagePresentations.home.subtitle || '搜索小区、地铁或商圈' }} <b>搜索</b></div></section>
                  <div class="mini-locations"><span class="active">附近</span><span v-for="(item, index) in publicConfig.locations.slice(0, 3)" :key="`preview-location-${index}`">{{ item.label || '未命名' }}</span><em v-if="!publicConfig.locations.length">尚未配置</em></div>
                  <div class="mini-budget"><b>租金预算</b><span>¥1,500 以下</span><span>¥1,500–2,500</span></div>
                  <div class="mini-section-title">精选频道</div>
                  <div class="mini-channels"><article v-for="(item, index) in publicConfig.featuredChannels.slice(0, 3)" :key="`preview-channel-${index}`" :class="`preview-tone-${item.icon || index}`"><b><i :class="`preview-mark-${item.icon || index}`"></i></b><strong>{{ item.title || '未命名' }}</strong><span>{{ item.subtitle || '频道说明' }}</span></article><em v-if="!publicConfig.featuredChannels.length">尚未配置</em></div>
                  <div class="mini-recommend"><strong>推荐</strong><span>整租</span><span>合租</span><b @click="selectConfigPage('properties')">查看房源</b></div>
                  <article v-if="previewPrimaryProperty" class="preview-property-card preview-home-property-card" @click="openPreviewProperty(previewPrimaryProperty)"><div class="preview-card-cover" :style="previewCoverStyle(previewPrimaryProperty)"><em>{{ configAudience === 'operator' ? ({ draft: '草稿', pending_review: '待审核', published: '已发布', available: '可租', rented: '已租', offline: '已下架' }[previewPrimaryProperty.status] || previewPrimaryProperty.status) : '可租' }}</em></div><section><b>{{ previewListingTitle(previewPrimaryProperty) }}</b><span>{{ previewListingAddress(previewPrimaryProperty) }}</span><small v-if="previewOperatorDoorplate" class="preview-operator-doorplate">门牌 · {{ previewOperatorDoorplate }}</small><div class="preview-tag-stack" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`"><template v-if="publicConfig.listingTitle.listTagLayout === 'grouped'"><small v-if="previewTagGroups(previewPrimaryProperty).primary.length" class="preview-tag-row primary"><i v-for="tag in previewTagGroups(previewPrimaryProperty).primary" :key="tag">{{ tag }}</i></small><small v-if="previewTagGroups(previewPrimaryProperty).secondary.length" class="preview-tag-row secondary"><i v-for="tag in previewTagGroups(previewPrimaryProperty).secondary" :key="tag">{{ tag }}</i></small></template><small v-else-if="previewTagGroups(previewPrimaryProperty).all.length" class="preview-tag-row all"><i v-for="tag in previewTagGroups(previewPrimaryProperty).all" :key="tag">{{ tag }}</i></small></div><strong>¥ {{ previewPrimaryProperty.rentMonthly }}<em>/月</em></strong></section></article>
                  <div v-else class="preview-empty">暂无可展示房源</div>
                </div>
                <div class="mini-tabbar"><button class="active" @click="selectConfigPage('home')">⌂<small>首页</small></button><button @click="selectConfigPage('properties')">▤<small>房源</small></button><button @click="selectConfigPage('me')">○<small>我的</small></button></div>
              </div>
              <div v-else class="phone-preview page-phone-preview" :class="`preview-${configPage}`">
                <div class="phone-status"><span>9:41</span><span>● ● ●</span></div>
                <div class="mini-navigation">{{ publicConfig.pagePresentations[configPage].title || pagePresentationOptions.find(page => page.key === configPage)?.label }}</div>
                <div class="page-mini-content" :style="publicConfig.pagePresentations[configPage].backgroundImage ? { backgroundImage: `linear-gradient(rgba(247,246,241,.9),rgba(247,246,241,.96)), url(${publicConfig.pagePresentations[configPage].backgroundImage})` } : {}">
                  <header><strong>{{ publicConfig.pagePresentations[configPage].title }}</strong><span>{{ publicConfig.pagePresentations[configPage].subtitle }}</span></header>
                  <template v-if="configPage === 'properties'"><div class="preview-search">⌕ 搜索小区、位置或户型</div><div class="preview-filter"><i>地区</i><i>预算</i><i>整租</i><i>合租</i></div><article v-for="item in previewProperties" :key="item.id" class="preview-property-card" @click="openPreviewProperty(item)"><div class="preview-card-cover" :style="previewCoverStyle(item)"><em>{{ configAudience === 'operator' ? ({ draft: '草稿', pending_review: '待审核', published: '已发布', available: '可租', rented: '已租', offline: '已下架' }[item.status] || item.status) : '可租' }}</em></div><section><b>{{ previewListingTitle(item) }}</b><span>{{ previewListingAddress(item) }}</span><small v-if="configAudience === 'operator' && publicConfig.listingTitle.adminEnabledFields.includes('unit')" class="preview-operator-doorplate">门牌 · {{ [item.buildingNo, item.unitNo && `${String(item.unitNo).replace(/单元$/, '')}单元`, item.roomNo && `${String(item.roomNo).replace(/室$/, '')}室`].filter(Boolean).join('') || '待补充' }}</small><div class="preview-tag-stack" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`"><template v-if="publicConfig.listingTitle.listTagLayout === 'grouped'"><small v-if="previewTagGroups(item).primary.length" class="preview-tag-row primary"><i v-for="tag in previewTagGroups(item).primary" :key="tag">{{ tag }}</i></small><small v-if="previewTagGroups(item).secondary.length" class="preview-tag-row secondary"><i v-for="tag in previewTagGroups(item).secondary" :key="tag">{{ tag }}</i></small></template><small v-else-if="previewTagGroups(item).all.length" class="preview-tag-row all"><i v-for="tag in previewTagGroups(item).all" :key="tag">{{ tag }}</i></small></div><strong>¥ {{ item.rentMonthly }}<em>/月</em></strong></section></article><div v-if="!previewProperties.length" class="preview-empty">暂无可展示房源</div></template>
                  <template v-if="configPage === 'propertyDetail'">
                    <div class="detail-canvas" :class="{ operator: configAudience === 'operator' }">
                      <div class="detail-canvas-head"><span>‹</span><strong>房源详情</strong><i>•••</i></div>
                      <div class="detail-canvas-scroll">
                        <div class="detail-canvas-cover" :style="previewCoverStyle(previewPrimaryProperty)"><small>实拍 {{ previewPrimaryProperty?.media?.length || 0 }} 张</small></div>
                        <section v-if="detailModuleEnabled('overview')" data-canvas-area="overview" class="detail-canvas-card overview-card" @click="focusCanvasArea('overview')">
                          <div class="detail-canvas-price">¥{{ previewPrimaryProperty?.rentMonthly || 0 }}<small>/月</small><i v-if="configAudience === 'customer'">♡</i></div>
                          <strong class="detail-canvas-title">{{ previewPrimaryProperty?.title || previewPrimaryProperty?.community || '暂无可展示房源' }}</strong>
                          <em v-if="previewPrimaryProperty?.listingSource === '转租'">转租 · 剩余 {{ previewPrimaryProperty?.remainingLeaseMonths || 0 }} 个月</em>
                          <span class="detail-canvas-address">{{ previewPrimaryProperty?.location || '位置待补充' }}</span>
                          <small v-if="previewOperatorDoorplate" class="preview-operator-doorplate">门牌定位 · {{ previewOperatorDoorplate }}</small>
                          <div v-if="previewDetailTags(previewPrimaryProperty).length" class="detail-canvas-tags" :class="`detail-tag-layout-${publicConfig.listingTitle.detailTagLayout}`"><div v-if="previewDetailTagGroups(previewPrimaryProperty).top.length" class="detail-canvas-tag-row"><i v-for="tag in previewDetailTagGroups(previewPrimaryProperty).top" :key="`top-${tag}`">{{ tag }}</i></div><div v-if="previewDetailTagGroups(previewPrimaryProperty).bottom.length" class="detail-canvas-tag-row"><i v-for="tag in previewDetailTagGroups(previewPrimaryProperty).bottom" :key="`bottom-${tag}`">{{ tag }}</i></div></div>
                          <div v-if="!publicConfig.listingTitle.customerDetailFields.includes('area') || !publicConfig.listingTitle.customerDetailFields.includes('layout') || !publicConfig.listingTitle.customerDetailFields.includes('floor')" class="detail-canvas-facts"><i v-if="!publicConfig.listingTitle.customerDetailFields.includes('area')"><small>面积</small>{{ previewPrimaryProperty?.areaSqm ? `${previewPrimaryProperty.areaSqm}㎡` : '待补充' }}</i><i v-if="!publicConfig.listingTitle.customerDetailFields.includes('layout')"><small>户型</small>{{ previewPrimaryProperty?.layout || '待补充' }}</i><i v-if="!publicConfig.listingTitle.customerDetailFields.includes('floor')"><small>楼层</small>{{ previewPrimaryProperty?.floorInfo || '待补充' }}</i></div>
                        </section>
                        <section v-if="detailModuleEnabled('overview') && (previewPrimaryProperty?.depositPayment || previewPrimaryProperty?.availableDate)" class="detail-canvas-card detail-canvas-info"><b>入住信息</b><div><i v-if="previewPrimaryProperty?.depositPayment"><small>押付方式</small>{{ previewPrimaryProperty.depositPayment }}</i><i v-if="previewPrimaryProperty?.availableDate"><small>可入住</small>{{ previewPrimaryProperty.availableDate }}</i></div></section>
                        <section v-if="detailModuleEnabled('location') && previewHasLocation" data-canvas-area="location" class="detail-canvas-card detail-canvas-location" @click="focusCanvasArea('location')"><b>{{ detailModuleTitle('location', '位置与周边') }}</b><small>{{ previewPrimaryProperty?.location || '已保存房源定位' }}</small><div><span><strong>查看地图位置</strong><small>已保存房源定位</small></span><i>›</i></div></section>
                        <section v-if="detailModuleEnabled('facilities')" data-canvas-area="facilities" class="detail-canvas-card detail-canvas-facilities" @click="focusCanvasArea('facilities')"><header><span><b>{{ detailModuleTitle('facilities', '房屋设施与环境') }}</b><small>家具家电、楼内条件与入住信息</small></span><i>{{ previewDetailAmenities.length + previewEnvironmentHighlights.length }} 项</i></header><template v-if="previewDetailFacilityGroups.length"><div v-for="group in previewDetailFacilityGroups" :key="group.title" class="detail-canvas-facility-group"><small>{{ group.title }}</small><div><i v-for="facility in group.items" :key="facility.key"><b>{{ detailFacilityIcon(facility.key) }}</b>{{ facility.label }}</i></div></div></template><p v-else>家具家电信息待补充，可联系小皮确认</p><div v-if="previewEnvironmentHighlights.length" class="detail-canvas-environment"><i v-for="environment in previewEnvironmentHighlights" :key="environment.label"><small>{{ environment.label }}</small>{{ environment.value }}</i></div></section>
                        <section v-if="detailModuleEnabled('description')" data-canvas-area="description" class="detail-canvas-card detail-canvas-description" @click="focusCanvasArea('description')"><b>{{ detailModuleTitle('description', '房源描述') }}</b><p>{{ previewDescription(previewPrimaryProperty) || '暂无描述' }}</p></section>
                        <section v-if="detailModuleEnabled('nearby')" data-canvas-area="nearby" class="detail-canvas-card detail-canvas-nearby" @click="focusCanvasArea('nearby')"><header><span><b>{{ detailModuleTitle('nearby', '附近房源') }}</b><small>距此房源 {{ publicConfig.propertyDetail.nearby.radiusKm }} 公里内</small></span><i>{{ previewNearbyProperties.length ? `${previewNearbyProperties.length} 套` : '继续找房' }}</i></header><article v-for="property in previewNearbyProperties" :key="property.id"><div :style="previewCoverStyle(property)"></div><span><b>{{ property.title || property.community }}</b><small>{{ property.layout || property.community }}</small><strong>¥{{ property.rentMonthly }}<i>/月</i></strong></span></article><p v-if="!previewNearbyProperties.length">附近暂时没有其他可租房源</p></section>
                      </div>
                      <div v-if="configAudience === 'customer'" class="detail-canvas-actions"><button>分享</button><button>在线咨询</button><button>打电话</button></div>
                      <div v-else class="detail-canvas-actions operator-actions"><button>分享房源</button><button>编辑房源</button></div>
                    </div>
                  </template>
                  <template v-else-if="configPage === 'requests' || configPage === 'publish'"><div class="preview-form-card"><label>意向地区</label><span>选择地区与商圈　›</span><label>预算范围</label><span>¥1500 – ¥2500</span><label>入住需求</label><span>填写补充说明</span><button>提交{{ configPage === 'requests' ? '找房需求' : '房源线索' }}</button></div></template>
                  <template v-else-if="configPage === 'me'"><div class="preview-profile" @click="selectConfigPage('authLogin')"><i>{{ previewIdentity.mark }}</i><div><b>{{ previewIdentity.name }}</b><span>{{ previewIdentity.caption }}</span></div></div><div class="preview-tool-grid"><article @click="selectConfigPage('properties')">全部房源<small>继续看房</small></article><article @click="selectConfigPage('footprints')">浏览足迹<small>最近查看</small></article><article @click="selectConfigPage('requests')">找房需求<small>查看进度</small></article><article @click="selectConfigPage('publish')">推荐房源<small>提交房源线索</small></article></div></template>
                  <template v-else-if="configPage === 'footprints'"><article v-for="name in ['拿铁公寓', '凤凰花园三期', '中建大公馆']" :key="name" class="preview-footprint"><div></div><section><b>{{ name }}</b><span>今天浏览 · ¥2300/月</span></section></article></template>
                  <template v-else-if="configPage === 'adminWorkbench'"><div class="preview-admin-card"><b>{{ previewIdentity.name }}</b><span>{{ previewIdentity.caption }}</span></div><div class="preview-stats"><i>{{ properties.filter(item => item.status === 'draft').length }}<small>房源草稿</small></i><i>{{ rentalRequests.filter(item => item.status !== 'done').length }}<small>待联系</small></i><i>{{ favoriteContacts.length }}<small>用户意向</small></i></div><div class="preview-tool-grid"><article @click="selectConfigPage('adminPropertyEdit')">＋ 录入房源<small>发布可租房源</small></article><article @click="selectConfigPage('properties')">▦ 房源管理<small>编辑与上下架</small></article><article>○ 客户需求<small>预约与找房</small></article><article>♡ 用户意向<small>关注记录</small></article></div></template>
                  <template v-else-if="configPage === 'adminPropertyEdit'"><div class="preview-steps"><i class="active">位置门牌</i><i>户型出租</i><i>环境配套</i><i>租金入住</i></div><div class="preview-edit-card"><b>发布前 · 标签展示规则</b><span>客户与管理员的标签均由后台统一配置；录入房源时无需选择。</span><div class="preview-checks"><i v-for="key in publicConfig.listingTitle.enabledFields" :key="key">✓ {{ listingTitleFieldOptions.find(field => field.key === key)?.label }}</i></div><strong>拿铁公寓 · 精装朝南</strong></div><button class="preview-primary-button">保存并查看</button></template>
                  <template v-else-if="configPage === 'authLogin'"><div class="preview-login-mark">皮</div><div class="preview-login-card"><b>登录后开始找房</b><span>同步关注、足迹和找房需求</span><button>微信一键登录</button><small>稍后完善头像与昵称</small></div></template>
                </div>
                <div v-if="configPage !== 'propertyDetail'" class="mini-tabbar"><button :class="{ active: configPage === (configAudience === 'customer' ? 'home' : 'adminWorkbench') }" @click="selectConfigPage(configAudience === 'customer' ? 'home' : 'adminWorkbench')">⌂<small>{{ configAudience === 'customer' ? '首页' : '工作台' }}</small></button><button :class="{ active: configPage === 'properties' }" @click="selectConfigPage('properties')">▤<small>房源</small></button><button :class="{ active: configPage === 'me' }" @click="selectConfigPage('me')">○<small>我的</small></button></div>
              </div>
              <div v-if="configPage === 'propertyDetail' && previewOverlay !== 'page'" class="preview-overlay" @click="previewOverlay = 'page'"><div class="preview-overlay-card" @click.stop><button class="overlay-close" @click="previewOverlay = 'page'">×</button><template v-if="previewOverlay === 'share'"><span>微信聊天中的小程序分享卡</span><strong>{{ shareTitlePreview || previewPrimaryProperty?.title || previewPrimaryProperty?.community || '暂无可展示房源' }}</strong><div class="share-cover" :style="previewCoverStyle(previewPrimaryProperty)"></div><b>¥{{ previewPrimaryProperty?.rentMonthly || 0 }}/月</b><p>{{ previewPrimaryProperty?.community || '房源小区' }} · {{ previewShareTags(previewPrimaryProperty).join(' · ') || '真实好房' }}</p><small>{{ publicConfig.heroTitle || '小皮租房' }}</small></template><template v-else><span>朋友圈海报预览</span><div class="poster-cover" :style="previewCoverStyle(previewPrimaryProperty)"></div><b>¥{{ previewPrimaryProperty?.rentMonthly || 0 }}<span>/月</span></b><strong>{{ previewPrimaryProperty?.title || previewPrimaryProperty?.community || '暂无可展示房源' }}</strong><p>{{ previewPosterTags(previewPrimaryProperty).join(' · ') || '真实好房 · 拎包入住' }}</p><i>{{ publicConfig.heroTitle || '小皮租房' }} · 扫码查看房源</i></template></div></div>
            </aside>
          </div>
        </template>

        <LeadsPage v-else-if="tab === '线索管理'" :leads="leads" :loading="loading" @refresh="load" />
  </AdminLayout>
</template>
