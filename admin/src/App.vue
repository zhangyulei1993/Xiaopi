<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, ArrowUp, Delete, House, Plus, RefreshLeft } from '@element-plus/icons-vue'

import AdminLayout from './layouts/AdminLayout.vue'
import DashboardPage from './pages/DashboardPage.vue'
import WechatUsersPage from './pages/WechatUsersPage.vue'
import IntentionsPage from './pages/IntentionsPage.vue'
import PropertiesPage from './pages/PropertiesPage.vue'
import RentalRequestsPage from './pages/RentalRequestsPage.vue'
import { pathForTab, tabFromRoute } from './router'
import type { AdminUser, ChannelConfig, FavoriteContact, Lead, LocationSearchItem, PagePresentation, Property, PublicConfig, RentalRequest, TemporaryAccess, WechatUser } from './types/domain'
import { channelStyleOptions, defaultPagePresentations, normalizeChannelStyle, pagePresentationOptions } from './config/navigation'
import { createAdminApi } from './services/api'
type TabKey = '运营总览' | '房源列表' | '线索管理' | '租客需求' | '用户意向' | '微信用户' | '小程序配置' | '管理员列表' | '临时访问码'

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
const publicConfig = reactive<PublicConfig>({ heroTitle: '', heroSubtitle: '', heroDescription: '', notice: '', contactPhone: '', relatedPropertyRadiusKm: 3, locations: [], featuredChannels: [], pagePresentations: defaultPagePresentations() })

const api = createAdminApi({ baseUrl: apiBase, onUnauthorized: () => { loggedIn.value = false; password.value = '' } })

const cloneConfig = (value: PublicConfig): PublicConfig => JSON.parse(JSON.stringify(value))
const applyConfig = (value: PublicConfig, markSaved = false) => {
  const next = cloneConfig(value)
  next.relatedPropertyRadiusKm = next.relatedPropertyRadiusKm || 3
  next.featuredChannels = (next.featuredChannels || []).map((item, index) => ({ latitude: 0, longitude: 0, ...item, icon: normalizeChannelStyle(item.icon, index), radiusKm: item.radiusKm || 3 }))
  next.pagePresentations = { ...defaultPagePresentations(), ...(next.pagePresentations || {}) }
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

const rows = computed(() => {
  if (tab.value === '线索管理') return leads.value.map((item) => ({ name: item.sourceName, info: `${item.community} · ${item.sourcePhone}`, status: item.status }))
  return properties.value.map((item) => ({ name: item.title, info: item.community, status: item.status }))
})
const currentMetric = computed(() => tab.value === '线索管理' ? '线索总数' : '房源总数')
const statusType = (status: string) => status === '正常' || status === 'PUBLISHED' ? 'success' : status === 'PENDING' ? 'warning' : 'info'
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
  return errors
})

function addLocation() {
  if (publicConfig.locations.length >= 20) return
  publicConfig.locations.push({ label: '', keyword: '' })
}

function removeLocation(index: number) {
  publicConfig.locations.splice(index, 1)
}

function addChannel() {
  if (publicConfig.featuredChannels.length >= 12) return
  publicConfig.featuredChannels.push({ icon: 'commute', title: '', subtitle: '', keyword: '', latitude: 0, longitude: 0, radiusKm: 3 })
  activeChannelIndex.value = publicConfig.featuredChannels.length - 1
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

        <template v-else-if="tab === '小程序配置'">
          <section class="page-header">
            <div><span>内容与运营</span><h1>小程序配置中心</h1></div>
          </section>
          <section class="surface config-toolbar">
            <div class="publish-state">
              <i :class="{ dirty: configDirty }" />
              <div><strong>{{ configDirty ? '存在未发布修改' : '线上配置已同步' }}</strong><span>{{ configLastSavedAt ? `最近发布：${configLastSavedAt}` : '当前内容来自线上配置' }}</span></div>
            </div>
            <div class="config-toolbar-meta"><span>{{ publicConfig.locations.length }} 个地区</span><span>{{ publicConfig.featuredChannels.length }} 个频道</span></div>
          </section>
          <section class="surface page-config-nav">
            <div><strong>选择小程序页面</strong><span>每页单独维护展示文案与背景图</span></div>
            <div class="page-config-tabs"><button v-for="page in pagePresentationOptions" :key="page.key" :class="{ active: configPage === page.key }" @click="configPage = page.key">{{ page.label }}</button></div>
          </section>
          <el-alert v-if="configMessage" class="config-alert" :title="configMessage" :type="configFailed ? 'error' : 'success'" show-icon :closable="false" />
          <section v-if="configErrors.length" class="config-validation">
            <strong>发布前需要处理 {{ configErrors.length }} 项</strong>
            <ul><li v-for="error in configErrors" :key="error">{{ error }}</li></ul>
          </section>
          <div v-loading="loading" class="config-workspace">
            <div class="config-editor">
              <section v-if="configPage === 'home'" class="surface form-section config-section">
                <div class="section-heading"><span>01</span><div><h2>首页页面配置</h2></div></div>
                <el-form label-position="top" class="basic-config-form">
                  <div class="form-row two-columns">
                    <el-form-item label="品牌名称" required><el-input v-model="publicConfig.heroTitle" maxlength="20" show-word-limit placeholder="例如：小皮租房" /></el-form-item>
                    <el-form-item label="联系电话"><el-input v-model.trim="publicConfig.contactPhone" maxlength="20" placeholder="用于房源详情联系" /></el-form-item>
                  </div>
                  <el-form-item label="首页主标题" required><el-input v-model="publicConfig.pagePresentations.home.title" maxlength="30" show-word-limit placeholder="例如：今天想住哪里？" /></el-form-item>
                  <el-form-item label="搜索提示"><el-input v-model="publicConfig.pagePresentations.home.subtitle" maxlength="60" show-word-limit placeholder="例如：搜索小区、地铁或商圈" /></el-form-item>
                  <el-form-item label="首页背景图"><el-input v-model="publicConfig.pagePresentations.home.backgroundImage" placeholder="上传后自动填入，也可填写图片地址" /><div class="presentation-actions"><label class="upload-control"><input type="file" accept="image/jpeg,image/png,image/webp" @change="uploadPresentationBackground('home', $event)" />上传背景图</label><button v-if="publicConfig.pagePresentations.home.backgroundImage" @click="publicConfig.pagePresentations.home.backgroundImage = ''">移除背景图</button></div></el-form-item>
                  <el-form-item label="附近房源关联距离">
                    <el-select v-model="publicConfig.relatedPropertyRadiusKm" placeholder="选择距离" style="width: 100%">
                      <el-option v-for="distance in [1, 2, 3, 5, 10]" :key="distance" :label="`${distance} 公里`" :value="distance" />
                    </el-select>
                    <div class="form-tip">按地图坐标自动关联。</div>
                  </el-form-item>
                </el-form>
              </section>

              <section v-if="configPage === 'home'" class="surface form-section config-section">
                <div class="section-heading config-section-heading"><span>02</span><div><h2>地区入口</h2></div><el-button type="primary" plain :icon="Plus" :disabled="publicConfig.locations.length >= 20" @click="addLocation">新增地区</el-button></div>
                <div v-if="publicConfig.locations.length" class="structured-list">
                  <article v-for="(item, index) in publicConfig.locations" :key="`location-${index}`" class="config-list-item">
                    <div class="item-index">{{ String(index + 1).padStart(2, '0') }}</div>
                    <div class="item-fields location-fields">
                      <label><span>显示名称</span><el-input v-model="item.label" maxlength="12" placeholder="例如：光谷" /></label>
                      <label><span>搜索关键词</span><el-input v-model="item.keyword" maxlength="20" placeholder="例如：光谷" /></label>
                    </div>
                    <div class="item-actions">
                      <el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="上移" @click="moveItem(publicConfig.locations, index, -1)" />
                      <el-button text :icon="ArrowDown" :disabled="index === publicConfig.locations.length - 1" aria-label="下移" @click="moveItem(publicConfig.locations, index, 1)" />
                      <el-popconfirm title="确认删除这个地区入口？" @confirm="removeLocation(index)"><template #reference><el-button text type="danger" :icon="Delete" aria-label="删除" /></template></el-popconfirm>
                    </div>
                  </article>
                </div>
                <div v-else class="config-empty"><strong>还没有地理位置</strong><span>至少添加一个地区，用户才能从首页快速筛选房源。</span><el-button type="primary" plain :icon="Plus" @click="addLocation">添加第一个地区</el-button></div>
              </section>

              <section v-if="configPage === 'home'" class="surface form-section config-section">
                <div class="section-heading config-section-heading"><span>03</span><div><h2>精选频道</h2></div><el-button type="primary" plain :icon="Plus" :disabled="publicConfig.featuredChannels.length >= 12" @click="addChannel">新增频道</el-button></div>
                <div v-if="publicConfig.featuredChannels.length" class="channel-editor-tabs"><button v-for="(item, index) in publicConfig.featuredChannels" :key="`channel-tab-${index}`" type="button" :class="{ active: activeChannelIndex === index }" @click="activeChannelIndex = index"><span>{{ index + 1 }}</span>{{ item.title || `频道 ${index + 1}` }}</button></div>
                <div v-if="publicConfig.featuredChannels.length" class="channel-editor-grid">
                  <article v-for="(item, index) in publicConfig.featuredChannels" v-show="activeChannelIndex === index" :key="`channel-${index}`" class="channel-editor-card active">
                    <header><div class="channel-order"><span>{{ channelStyleOptions.find((style) => style.value === item.icon)?.label || '频道' }}</span><strong>频道 {{ index + 1 }}</strong></div><div class="item-actions"><el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="上移" @click="moveItem(publicConfig.featuredChannels, index, -1)" /><el-button text :icon="ArrowDown" :disabled="index === publicConfig.featuredChannels.length - 1" aria-label="下移" @click="moveItem(publicConfig.featuredChannels, index, 1)" /><el-popconfirm title="确认删除这个精选频道？" @confirm="removeChannel(index)"><template #reference><el-button text type="danger" :icon="Delete" aria-label="删除" /></template></el-popconfirm></div></header>
                    <div class="channel-fields">
                      <label class="icon-field"><span>频道样式</span><el-select v-model="item.icon"><el-option v-for="style in channelStyleOptions" :key="style.value" :label="style.label" :value="style.value" /></el-select></label>
                      <label><span>标题</span><el-input v-model="item.title" maxlength="12" placeholder="例如：金融港" @input="invalidateChannelLocation(item, index)" /></label>
                      <label><span>副标题</span><el-input v-model="item.subtitle" maxlength="20" placeholder="例如：通勤优选" /></label>
                      <label><span>搜索关键词（未选地点时使用）</span><el-input v-model="item.keyword" maxlength="20" placeholder="点击后筛选的关键词" @input="invalidateChannelLocation(item, index)" /></label>
                    </div>
                  </article>
                </div>
                <section v-if="activeChannel" class="channel-map-workbench"><header><div><strong>{{ activeChannel.title || `频道 ${activeChannelIndex + 1}` }} · 地图定位</strong><span>搜索结果、地图选点与中心坐标集中在这里处理。</span></div><div class="channel-location-action"><el-button :loading="channelLocationLoading[activeChannelIndex]" @click="searchChannelLocation(activeChannel, activeChannelIndex)">{{ activeChannel.latitude && activeChannel.longitude ? '重新搜索地点' : '搜索地点' }}</el-button><el-button v-if="activeChannel.latitude && activeChannel.longitude" text type="danger" @click="clearChannelLocation(activeChannel, activeChannelIndex)">清除位置</el-button></div></header><div v-if="channelMapLoading[activeChannelIndex]" class="channel-map-loading">正在加载已选地点…</div><div v-else-if="channelMapPreview[activeChannelIndex]" class="channel-map-picker" title="点击地图设置频道中心" @click="selectChannelMapPoint(activeChannel, activeChannelIndex, $event)"><img class="channel-map-preview" :src="channelMapPreview[activeChannelIndex]" alt="频道范围地图预览，点击可选点" /><div v-if="activeChannel.latitude && activeChannel.longitude && !channelLocationResults[activeChannelIndex]?.length" class="channel-radius-ring" :style="selectedRadiusStyle(activeChannel)"></div><button v-if="activeChannel.latitude && activeChannel.longitude && !channelLocationResults[activeChannelIndex]?.length" class="channel-selected-map-pin" type="button" :aria-label="`${channelSelectedLocation[activeChannelIndex]?.title || '频道中心'}，已选`" @click.stop><span>●</span></button><template v-for="marker in mapMarkerPositions(activeChannel, activeChannelIndex)" :key="marker.location.id"><button class="channel-map-marker" :class="{ active: channelMapActiveMarker[activeChannelIndex] === marker.location.id }" type="button" :style="{ left: `${marker.left}%`, top: `${marker.top}%` }" :aria-label="marker.location.title" @click.stop="toggleChannelMapMarker(activeChannelIndex, marker.location.id)"><span>{{ marker.label }}</span></button><div v-if="channelMapActiveMarker[activeChannelIndex] === marker.location.id" class="channel-map-callout" :style="{ left: `${marker.left}%`, top: `${marker.top}%` }" @click.stop><strong>{{ marker.location.title }}</strong><span>{{ marker.location.district }} · {{ marker.location.address }}</span><button type="button" @click="selectChannelLocation(activeChannel, activeChannelIndex, marker.location)">设为频道中心</button></div></template></div><small v-if="activeChannel.latitude && activeChannel.longitude">已选中心：{{ channelSelectedLocation[activeChannelIndex]?.title || '地图选点' }} · {{ activeChannel.latitude.toFixed(6) }}, {{ activeChannel.longitude.toFixed(6) }} · 半径 {{ activeChannel.radiusKm }} 公里</small><small v-else>点击地图可直接选点；搜索结果会以大头钉显示在图上。</small><div v-if="channelLocationResults[activeChannelIndex]?.length" class="admin-search-results channel-location-results"><button v-for="(location, locationIndex) in channelLocationResults[activeChannelIndex]" :key="location.id" type="button" @click="selectChannelLocation(activeChannel, activeChannelIndex, location)"><span><strong>{{ String.fromCharCode(65 + locationIndex) }} · {{ location.title }}</strong><small>{{ location.district }} · {{ location.address }}</small></span><em>{{ location.distance ? `${(location.distance / 1000).toFixed(1)}km` : '选用' }}</em></button></div><label class="channel-radius-field"><span>地图关联半径</span><el-input-number v-model="activeChannel.radiusKm" :min="0.1" :max="100" :precision="1" :step="0.5" controls-position="right" /><small>保存后，中心点半径内的房源会自动关联到此频道。</small></label></section>
                <div v-else class="config-empty"><strong>还没有精选频道</strong><span>添加频道后，用户可以从首页直接进入对应房源列表。</span><el-button type="primary" plain :icon="Plus" @click="addChannel">添加第一个频道</el-button></div>
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
            </div>

            <aside class="config-preview-column">
              <div class="preview-heading"><div><strong>{{ pagePresentationOptions.find(page => page.key === configPage)?.label }}预览</strong></div><el-tag :type="configDirty ? 'warning' : 'success'" effect="light">{{ configDirty ? '未发布' : '线上版本' }}</el-tag></div>
              <div class="config-preview-actions">
                <el-button :icon="RefreshLeft" :disabled="!configDirty || savingConfig" @click="resetConfig">恢复</el-button>
                <el-button type="primary" :loading="savingConfig" :disabled="!configDirty || configErrors.length > 0" @click="saveConfig">保存并发布</el-button>
              </div>
              <div v-if="configPage === 'home'" class="phone-preview">
                <div class="phone-status"><span>9:41</span><span>● ● ●</span></div>
                <div class="mini-navigation">{{ publicConfig.heroTitle || '小皮租房' }}</div>
                <div class="mini-content">
                  <section class="mini-hero"><span class="mini-city">武汉⌄</span><strong>{{ publicConfig.pagePresentations.home.title || '今天想住哪里？' }}</strong><i></i><div class="mini-search">{{ publicConfig.pagePresentations.home.subtitle || '搜索小区、地铁或商圈' }} <b>搜索</b></div></section>
                  <div class="mini-locations"><span class="active">附近</span><span v-for="(item, index) in publicConfig.locations.slice(0, 3)" :key="`preview-location-${index}`">{{ item.label || '未命名' }}</span><em v-if="!publicConfig.locations.length">尚未配置</em></div>
                  <div class="mini-budget"><b>租金预算</b><span>¥1,500 以下</span><span>¥1,500–2,500</span></div>
                  <div class="mini-section-title">精选频道</div>
                  <div class="mini-channels"><article v-for="(item, index) in publicConfig.featuredChannels.slice(0, 3)" :key="`preview-channel-${index}`" :class="`preview-tone-${item.icon || index}`"><b><i :class="`preview-mark-${item.icon || index}`"></i></b><strong>{{ item.title || '未命名' }}</strong><span>{{ item.subtitle || '频道说明' }}</span></article><em v-if="!publicConfig.featuredChannels.length">尚未配置</em></div>
                  <div class="mini-recommend"><strong>推荐</strong><span>整租</span><span>合租</span><b>查看房源</b></div>
                  <div class="mini-listing"><div></div><section><strong>正在加载附近好房</strong><span>真实房源 · 可靠服务</span><b>¥ 租金/月</b></section></div>
                </div>
                <div class="mini-tabbar"><span>⌂<small>首页</small></span><span>▤<small>房源</small></span><span>○<small>我的</small></span></div>
              </div>
              <div v-else class="selected-page-preview" :style="publicConfig.pagePresentations[configPage].backgroundImage ? { backgroundImage: `linear-gradient(rgba(247,246,241,.88),rgba(247,246,241,.96)), url(${publicConfig.pagePresentations[configPage].backgroundImage})` } : {}"><span>小程序页面</span><strong>{{ publicConfig.pagePresentations[configPage].title || pagePresentationOptions.find(page => page.key === configPage)?.label }}</strong><p>{{ publicConfig.pagePresentations[configPage].subtitle || '辅助文本将在页面标题区域展示' }}</p><div></div></div>
              <p class="preview-note">预览只反映当前选择的页面配置，实际尺寸会随设备自适应。</p>
            </aside>
          </div>
        </template>

        <template v-else>
          <section class="page-header">
            <div><span>业务管理</span><h1>{{ tab }}</h1><p>查看线上业务数据，状态信息与小程序使用同一套 API。</p></div>
            <el-button :loading="loading" @click="load">刷新数据</el-button>
          </section>
          <div class="stats-grid list-stats">
            <section class="surface compact-stat"><span>{{ currentMetric }}</span><strong>{{ rows.length }}</strong></section>
            <section class="surface compact-stat"><span>今日新增</span><strong>0</strong></section>
            <section class="surface compact-stat"><span>待处理事项</span><strong>{{ tab === '线索管理' ? leads.length : 0 }}</strong></section>
          </div>
          <section class="surface table-panel">
            <div class="table-heading"><div><h2>{{ tab }}列表</h2><p>共 {{ rows.length }} 条记录</p></div></div>
            <el-table v-loading="loading" :data="rows" empty-text="暂无数据">
              <el-table-column prop="name" label="名称" min-width="180" />
              <el-table-column prop="info" label="信息" min-width="240" />
              <el-table-column label="状态" width="140"><template #default="scope"><el-tag :type="statusType(scope.row.status)" effect="light">{{ scope.row.status || '未知' }}</el-tag></template></el-table-column>
              <el-table-column label="操作" width="120" align="center"><template #default><el-button link type="primary">查看</el-button></template></el-table-column>
            </el-table>
          </section>
        </template>
  </AdminLayout>
</template>
