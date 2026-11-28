<script setup lang="ts">
import { defaultShareCard, defaultListingTitle, defaultPropertyDetail } from "../config/workspaceDefaults"
import { basicConfigSections, canvasNodes, type BasicConfigTab, type DetailSectionTab } from "../config/workspaceCanvasNodes"
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus/es/components/message/index.mjs'
import { ArrowDown, ArrowUp, Delete, House, Plus, RefreshLeft } from '@element-plus/icons-vue'

import homeHeroFallback from '../../../miniapp/src/static/home-lifestyle-hero.jpg'
import editorMapPreview from '../assets/editor-map-preview.png'
import propertyPlaceholderIcon from '../../../miniapp/src/static/icons/property-placeholder.svg'
import propertyShareIcon from '../../../miniapp/src/static/icons/share-property.svg'
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
import SubscriptionJobsPage from './SubscriptionJobsPage.vue'
import PropertiesPage from './PropertiesPage.vue'
import MaterialLibraryPage from './MaterialLibraryPage.vue'
import RentalRequestsPage from './RentalRequestsPage.vue'
import LeadsPage from './LeadsPage.vue'
import { pathForTab, tabFromRoute } from '../router'
import type { AdminUser, ChannelConfig, FavoriteContact, Lead, LocationSearchItem, PagePresentation, Property, PublicConfig, RentalRequest, TemporaryAccess, WechatUser } from '../types/domain'
import { channelStyleOptions, defaultPagePresentations, normalizeChannelStyle, pagePresentationOptions } from '../config/navigation'
import { createAdminApi } from '../services/api'
import { captureDisabledTag, restoreDisabledTag, type DisabledTagState } from '../config/tagRestore'
import { configChanges, restoreChange, changeValue, changeScope, registerCopyScope, type ConfigChange, type ConfigPublication, type ConfigState } from '../config/configChanges'
import DoorplatesPage from './DoorplatesPage.vue'
import { pageCopyGroups } from '../config/pageCopyGroups'
import { normalizeCommunityIdentity } from '../utils/communityName'

type TabKey = '小区门牌数据' | '运营总览' | '房源列表' | '房源素材库' | '线索管理' | '租客需求' | '用户意向' | '通知任务' | '微信用户' | '小程序基础配置' | '小程序页面配置' | '后台管理员' | '小程序管理员' | '临时访问码'

const apiBase = import.meta.env.VITE_API_BASE || 'https://api.xiaopizufang.cn/api'
const apiLabel = new URL(apiBase, window.location.origin).host
const loggedIn = ref(Boolean(localStorage.getItem('xiaopi_admin_token')))
const route = useRoute()
const router = useRouter()
const tab = ref<TabKey>(tabFromRoute(route.params.tab) as TabKey)
type WebAccount = { username: string; enabled: boolean; super: boolean; mustChangePassword: boolean }
const webAccounts = ref<WebAccount[]>([])
const webAdminSuper = ref(sessionStorage.getItem('xiaopi_web_admin_super') === 'true')
const passwordNoticeKey = 'xiaopi_web_admin_initial_password'
const mustChangePassword = ref(sessionStorage.getItem(passwordNoticeKey) === 'true')
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
const initialPasswordNotice = '当前账号仍在使用初始密码，建议在「后台管理员」中修改密码。'
let passwordMessage: ReturnType<typeof ElMessage> | undefined
let errorMessage: ReturnType<typeof ElMessage> | undefined
watch([tab, loggedIn, mustChangePassword], ([, active, needsPassword]) => {
  passwordMessage?.close()
  if (active && needsPassword) {
    passwordMessage = ElMessage({ message: initialPasswordNotice, type: 'warning', duration: 4500, showClose: true })
  }
}, { immediate: true })
watch([tab, loggedIn, pageError], ([, active, error]) => {
  errorMessage?.close()
  if (active && error) {
    errorMessage = ElMessage({ message: error, type: 'error', duration: 4500, showClose: true })
  }
})

const savingConfig = ref(false)
const configMessage = ref('')
const configFailed = ref(false)
const savedConfigJSON = ref('')
const configLastSavedAt = ref('')
const configRevision = ref('')
const configConfirmOpen = ref(false)
const configHistoryOpen = ref(false)
const configHistoryLoading = ref(false)
const configHistoryHasMore = ref(false)
const configHistory = ref<ConfigPublication[]>([])
const configHistorySelected = ref<ConfigPublication | null>(null)
const configHistoryError = ref('')
const restoredFrom = ref('')
const configConflict = ref<ConfigState | null>(null)
const publishedReceipt = ref<ConfigPublication | null>(null)
const confirmationError = ref('')

const isMiniappConfig = computed(() => tab.value === '小程序基础配置' || tab.value === '小程序页面配置')

const configPage = ref('home')
const previewOverlay = ref<'page' | 'share'>('page')
const tagMagnifierOpen = ref(false)
type ConfigAudience = 'customer' | 'operator'
type DetailConfigTab = 'detail' | 'share'
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
const canvasEditSteps = ['房源信息', '图片与视频']
const canvasEditStep = ref(0)
const canvasQuickSettingsOpen = ref(false)
const canvasWorkbenchActivities = ref<{id:string;action:string;detail:string;createdAt?:string;category?:string;targetType?:string;targetId?:string}[]>([])
const canvasWorkbenchContacts = ref<{id:string;propertyName?:string;nickname?:string;phone?:string;type:string;source?:string;createdAt?:string}[]>([])
const canvasWorkbenchError = ref('')
const canvasReviewPerspective = ref<'customer' | 'admin'>('customer')
const canvasPageHistory = ref<string[]>([])
const canvasHomeSearchQuery = ref('')
const canvasAllChannelsOpen = ref(false)
const canvasHomeMapOpen = ref(false)
const canvasHomeMapActiveProperty = ref<Property | null>(null)
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
const canvasTargetWidth = ref(604 / window.devicePixelRatio)
let canvasFitObserver: ResizeObserver | undefined
const canvasAdminWorkbenchTab = ref<'leads' | 'requests' | 'interests'>('requests')
const canvasOperationLogsOpen = ref(false)
const canvasOperationLogsPage = ref(1)
const canvasOperationLogsCategory = ref('全部')
const canvasOperationLogsStart = ref('')
const canvasOperationLogsEnd = ref('')
const canvasNotice = ref('')
const canvasForm = ref(({ contactName: '', phone: '', location: '', layout: '', budgetMin: '0', budgetMax: '2500', moveInDate: '', preferredTime: '今天方便', note: '', sourceName: '', sourcePhone: '', community: '', expectedRent: '0' }))
const canvasDetailModal = ref<'appointment' | 'map' | ''>('')
const canvasMapExpanded = ref(false)
const canvasAccessPrompt = ref<{ title: string; content: string; destination: string } | null>(null)
const canvasDatePickerOpen = ref(false)
const localDateString = (date = new Date()) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
const canvasDatePickerValue = ref(localDateString())
const canvasPropertyDraft = ref<Record<string, string>>(({}))
const canvasLoginPhone = ref('')
const canvasLoginAccessCode = ref('')
const canvasPropertyFacilities = ref<string[]>([])
const canvasEditingExistingProperty = ref(false)
const canvasCropPreviewOpen = ref(false)
const canvasAvatarCropOpen = ref(false)
const canvasAvatarCropZoom = ref(100)
const canvasSampleImageAdded = ref(false)
const canvasSampleImageCropped = ref(false)
const canvasCropZoom = ref(130)
const canvasCropOffset = ref({ x: 0, y: 0 })
let canvasCropDragStart: { x: number; y: number; offsetX: number; offsetY: number } | null = null
const detailConfigTab = ref<DetailConfigTab>('detail')
const detailSectionTab = ref<DetailSectionTab>('overview')
const homeCopyEditing = ref(true)
const canPickHomeCopy = computed(() => tab.value === '小程序页面配置' && homeCopyEditing.value)
const selectedCanvasNodeID = ref('home.title')
const selectedCanvasNode = computed(() => canvasNodes[selectedCanvasNodeID.value])
const listingTagTabs = [
  { key: 'allowed', label: '可用标签' },
  { key: 'admin', label: '管理员标签' },
] as const
const sharePreviewType = ref<'friend' | 'timeline'>('friend')
const listingTagTab = ref<(typeof listingTagTabs)[number]['key']>('allowed')
const visibleListingTagTabs = computed(() => listingTagTabs.filter(item => item.key !== 'admin' || canvasIdentityState.value === 'admin'))
const activeListingTagTab = computed(() => visibleListingTagTabs.value.some(item => item.key === listingTagTab.value) ? listingTagTab.value : 'allowed')
const canvasRoleLabel = computed(() => ({ guest: '游客', user: '微信用户', admin: '小程序管理员' })[canvasIdentityState.value])
const basicConfigTab = ref<BasicConfigTab>('tags')
const previewFacilityCatalog = computed(() => tab.value === '小程序基础配置' && basicConfigTab.value === 'facilities')
const configPageMeta = computed(() => pagePresentationOptions.find(page => page.key === configPage.value))
const miniappNativeTitles: Record<string, string> = {
  home: '小皮租房', properties: '房源', requests: '找房需求', publish: '发布房源', me: '我的',
  footprints: '浏览足迹', requestHistory: '我的找房记录', authLogin: '登录',
  adminWorkbench: '小皮工作台', adminPropertyEdit: '编辑房源', adminMaterialLibrary: '房源素材库',
}
const canvasNativeTitle = computed(() => canvasAvatarCropOpen.value ? '调整头像' : canvasCropPreviewOpen.value ? '调整图片展示' : miniappNativeTitles[configPage.value] || '房源详情')
function selectDetailConfigTab(value: DetailConfigTab) {
  detailConfigTab.value = value
  locatePageEditor(value === 'share' ? 'detail-sharing' : 'detail-modules')
  previewOverlay.value = value === 'detail' ? 'page' : value
}
function selectDetailSectionTab(value: DetailSectionTab) {
  detailSectionTab.value = value
  if (tab.value === '小程序页面配置') locatePageEditor('detail-modules')
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
  if (value === 'facilities') {
    openFacilityPreview(false)
    return
  }
  selectConfigPage(value === 'brand' ? 'home' : 'properties')
}
function previewHomeRecommendation() {
  selectConfigPage('home')
  locatePageEditor('page-card-tags')
  void nextTick(() => document.querySelector('.canvas-home-phone .preview-property-card')?.scrollIntoView({ block: 'start', inline: 'nearest' }))
}

function selectListingTagTab(value: typeof listingTagTab.value) {
  listingTagTab.value = value
  switch (value) {
    case 'admin':
      selectConfigPage('properties')
      break
  }
}
function openFacilityPreview(editing: boolean) {
  if (editing) {
    switchCanvasIdentity('admin')
    selectConfigPage('adminPropertyEdit')
    canvasEditStep.value = 2
    return
  }
  selectConfigPage('propertyDetail')
  focusCanvasArea('facilities')
}
function toggleFacility(facility: { enabled: boolean }) { facility.enabled = !facility.enabled }
function removeDetailFacility(key: string) {
  if (publicConfig.propertyDetail.facilities.length <= 1) {
    configMessage.value = '至少保留一个设施类型'
    configFailed.value = true
    return
  }
  publicConfig.propertyDetail.facilities = publicConfig.propertyDetail.facilities.filter((facility) => facility.key !== key)
}
function showDetailNavigationPreview() {
  const scroll = canvasPreviewPhone.value?.querySelector('.detail-canvas-scroll')
  if (scroll && scroll.scrollTop <= 65) scroll.scrollTop = 66
}
function focusCanvasArea(area: DetailSectionTab) {
  selectedCanvasNodeID.value = `detail.${area}`
  detailSectionTab.value = area
  if (tab.value === '小程序页面配置') locatePageEditor('detail-modules')
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
  canvasAllChannelsOpen.value = false
  canvasPageHistory.value = []
  if (target.audience !== 'both' && target.audience !== configAudience.value) configAudience.value = target.audience as ConfigAudience
  configPage.value = target.key
  if (target.key === 'adminPropertyEdit') startCanvasPropertyEdit()
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
  configPage.value = value === 'operator' ? 'me' : 'home'
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
  if (tab.value === '小程序页面配置') {
    if (field === 'coverage' || field === 'status-labels') locatePageEditor(field)
    else if (field === 'request-guide' || field.startsWith('home-') || field.startsWith('bottom-') || field === 'page-copy') {
      const section = field === 'home-channelTitle' ? 'coverage' : field === 'home-recommendationTitle' ? 'page-card-tags' : 'page-copy'
      locatePageEditor(section)
    }
  }
  window.setTimeout(() => {
    const target = document.querySelector(`[data-config-field="${field}"]`) as HTMLElement | null
      || document.querySelector('.basic-config-panel, .config-section') as HTMLElement | null
    const details = target?.closest('details')
    if (details) details.open = true
    target?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    target?.querySelector<HTMLElement>('input, textarea, button')?.focus({ preventScroll: true })
  }, 80)
}
const pageEditorSection = ref('page-copy')
watch(configPage, () => locatePageEditor('page-copy'), { flush: 'sync' })

function locatePageEditor(field: string) {
  if (field === 'page-copy') {
    if (configPage.value === 'properties') canvasPageFilterReset()
    if (configPage.value === 'home') {
      canvasHomeBudget.value = '不限'
      canvasHomeKind.value = '全部'
    }
    if (configPage.value === 'adminWorkbench') canvasAdminWorkbenchTab.value = 'requests'
    if (configPage.value === 'adminPropertyEdit') canvasEditStep.value = 0
  }
  pageEditorSection.value = field
  if (configPage.value === 'properties' && field === 'properties-empty') {
    canvasPageFilterReset()
    revealPropertyEmptyCard()
  }
  if (configPage.value === 'propertyDetail' && field === 'page-copy') previewOverlay.value = 'page'
}
function revealPropertyEmptyCard() {
  void nextTick(() => {
    const scroll = canvasPreviewPhone.value?.querySelector('.page-mini-content')
    const empty = scroll?.querySelector('.canvas-properties-empty')
    if (scroll && empty) {
      const offset = empty.getBoundingClientRect().top - scroll.getBoundingClientRect().top
      scroll.scrollTo({ top: scroll.scrollTop + offset / canvasFitScale.value - 8, behavior: 'smooth' })
    }
  })
}

const operatorEmptyCopy = new Set(['当前筛选下暂无房源', '调整地区、预算或关键词，继续查看房源。', '重置筛选'])

const previewCopyScenario = ref('')
const previewStatusScenario = ref('')
const previewLoginReason = ref('')
const previewWorkbenchCopy = ref('')
watch([configPage, pageEditorSection], () => {
  previewCopyScenario.value = ''
  previewStatusScenario.value = ''
  previewLoginReason.value = ''
  previewWorkbenchCopy.value = ''
  canvasAccessPrompt.value = null
})

function revealCanvasCopy(field: string) {
  void nextTick(() => window.requestAnimationFrame(() => {
    const phone = canvasPreviewPhone.value
    if (!phone) return
    const text = canvasPageCopy(field).replace(/\{area\}/g, canvasAreaFilter.value).replace(/\{keyword\}/g, canvasSearchQuery.value)
    const target = [...phone.querySelectorAll<HTMLElement>('*')].find(el => el.getClientRects().length && (
      [...el.childNodes].some(node => node.nodeType === 3 && node.textContent?.includes(text)) || el.getAttribute('placeholder')?.includes(text)
    ))
    if (!target) return
    let scroll = target.parentElement
    while (scroll && scroll !== phone) {
      if (scroll.scrollHeight > scroll.clientHeight && /auto|scroll/.test(getComputedStyle(scroll).overflowY)) {
        const delta = (target.getBoundingClientRect().top - scroll.getBoundingClientRect().top) / canvasFitScale.value
        scroll.scrollTop += delta - Math.max(0, (scroll.clientHeight - target.offsetHeight) / 2)
        return
      }
      scroll = scroll.parentElement
    }
  }))
}
function previewPageCopyField(field: string) {
  revealCanvasCopy(field)
  const page = configPage.value
  previewCopyScenario.value = field
  if (page === 'home') {
    if (pageEditorSection.value === 'display-guidance') {
      canvasHomeBudget.value = field === '新房源正在整理中，稍后再来看看' ? '不限' : '≤1500'
      void nextTick(() => canvasPreviewPhone.value?.querySelector('.canvas-home-empty-state')?.scrollIntoView({ block: 'end' }))
    }
    return
  }
  if (page === 'propertyDetail') {
    void nextTick(() => canvasPreviewPhone.value?.querySelector(['暂无实拍图', '联系小皮了解房源详情'].includes(field) ? '.detail-canvas-cover' : ['暂无描述', '房东补充说明与居住规则'].includes(field) ? '.detail-canvas-description' : ['租赁信息', '费用与租期说明'].includes(field) ? '.detail-canvas-info' : field === '家具家电等可勾选配置' ? '.detail-canvas-facilities' : '.detail-canvas-nearby')?.scrollIntoView({ block: 'nearest' }))
    return
  }
  if (page === 'properties' && field.includes('{area}')) {
    canvasAreaFilter.value = areaEntryChannels.value[0]?.title || '片区'
  }
  if (page === 'properties' && field.includes('{keyword}')) canvasSearchQuery.value = '示例关键词'
  if (page === 'requests' && field === '登录后才能保存并跟进你的找房需求') {
    requireCanvasLogin('登录后提交', canvasPageCopy(field), 'authLogin')
    return
  }
  if (page === 'authLogin') {
    previewLoginReason.value = field
    return
  }
  if (page === 'me' && canvasIdentityState.value === 'guest' && pageEditorSection.value === 'notes-2') {
    const fields = activePageCopyGroup.value?.fields || []
    const pair = Math.floor(fields.indexOf(field) / 2) * 2
    if (pair >= 0) requireCanvasLogin(canvasPageCopy(fields[pair]), canvasPageCopy(fields[pair + 1]), 'authLogin')
    return
  }
  if (page === 'adminPropertyEdit') {
    if (field === '完善房源信息' && !canvasEditingExistingProperty.value) startCanvasPropertyEdit(previewPropertyOptions.value[0])
    if (field === '开始录入房源' && canvasEditingExistingProperty.value) startCanvasPropertyEdit()
    canvasEditStep.value = pageEditorSection.value === 'notes-3' ? 4 : pageEditorSection.value === 'notes-2' ? 2 : 0
    return
  }
  if (page === 'adminWorkbench') {
    previewWorkbenchCopy.value = field
    canvasAdminWorkbenchTab.value = field === '暂无用户意向' ? 'interests' : field === '暂无房源线索' ? 'leads' : 'requests'
    void nextTick(() => canvasPreviewPhone.value?.querySelector('.canvas-workbench-list')?.scrollIntoView({ block: 'nearest' }))
    return
  }
  if (configPage.value !== 'properties' || !['properties-empty', 'display-guidance'].includes(pageEditorSection.value)) return
  canvasPageFilterReset()
  if (field.includes('{area}')) canvasAreaFilter.value = areaEntryChannels.value[0]?.title || '片区'
  if (field.includes('{keyword}')) canvasSearchQuery.value = '示例关键词'
  if (field === '关注该片区新房') {
    canvasAreaFilter.value = areaEntryChannels.value[0]?.title || '片区'
  }
  revealPropertyEmptyCard()
}

function pickBottomPageCopy(field: string) {
  if (canPickHomeCopy.value) focusConfigField(field)
}
function openCanvasNode(id: string) {
  const node = canvasNodes[id]
  if (!node) return
  selectedCanvasNodeID.value = id
  if (node.tab === 'basic') {
    selectTab('小程序基础配置')
    selectBasicConfigTab(node.basicTab || 'tags')
    focusConfigField(node.field)
    return
  }
  const sourcePage = configPage.value
  selectTab('小程序页面配置')
  if (node.audience !== 'both') configAudience.value = node.audience
  const targetPage = node.page || ((id === 'page.heading' || id === 'brand') ? sourcePage : 'home')
  if (targetPage !== sourcePage) selectConfigPage(targetPage)
  selectedCanvasNodeID.value = id
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
const channelLocationRepairOpen = ref<Record<number, boolean>>({})
const doorplateCommunityCounts = ref<Record<string, number>>({})
const savedCommunityLocations = ref<Record<string, { address: string; latitude: number; longitude: number }>>({})
const channelCommunityOptions = computed(() => Object.entries(doorplateCommunityCounts.value).sort(([left], [right]) => left.localeCompare(right, 'zh-CN')))
const canvasCommonCommunities = computed(() => {
  const counts = new Map<string, number>()
  for (const property of properties.value) if (property.community) counts.set(property.community, (counts.get(property.community) || 0) + 1)
  return [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 4).map(([name]) => name)
})
const canvasFilteredCommunities = computed(() => {
  const query = canvasCommunityQuery.value.trim().toLocaleLowerCase()
  const options = channelCommunityOptions.value.map(([name]) => name)
  return query ? options.filter(name => name.toLocaleLowerCase().includes(query)) : options
})
const canvasDoorplateBuildings = computed(() => [...new Set(canvasDoorplateRows.value.map(row => row.building).filter(Boolean))])
const canvasDoorplateUnits = computed(() => [...new Set(canvasDoorplateRows.value.filter(row => row.building === canvasSelectedBuilding.value).map(row => row.unit).filter(Boolean))])
const canvasDoorplateRooms = computed(() => canvasDoorplateRows.value.filter(row => row.building === canvasSelectedBuilding.value && row.unit === canvasSelectedUnit.value).map(row => row.room).filter(Boolean))
const canvasMaterialCommunity = (item: Property) => (item as Property & { materialCategory?: string }).materialCategory || item.community
const canvasMaterialRoomCount = (item: Property) => Number(String(item.layout || '').match(/(\d+)室/)?.[1] || 0)
const canvasFilteredMaterials = computed(() => canvasMaterials.value.filter(item => {
  const converted = Boolean((item as Property & { convertedPropertyId?: string }).convertedPropertyId)
  if (canvasMaterialShowConverted.value !== converted) return false
  if (canvasMaterialSelectedCategory.value !== '全部' && canvasMaterialCommunity(item) !== canvasMaterialSelectedCategory.value) return false
  if (canvasMaterialLayout.value !== '全部') {
    const rooms = Number(canvasMaterialLayout.value.replace(/[^0-9]/g, ''))
    const itemRooms = canvasMaterialRoomCount(item)
    if (canvasMaterialLayout.value === '六室+' ? itemRooms < 6 : itemRooms !== rooms) return false
  }
  return true
}))
const canvasMaterialCategoryOptions = computed(() => [...new Set([...canvasMaterialCategories.value, ...canvasMaterials.value.map(canvasMaterialCommunity).filter(Boolean)])])
const canvasOperationLogCategories = ['全部', '房源、素材与门牌', '客户与需求', '消息通知']
const canvasOperationLogBucket = (type = '') => ({ property: '房源、素材与门牌', material: '房源、素材与门牌', building_rule: '房源、素材与门牌', lead: '客户与需求', request: '客户与需求', subscription: '消息通知' } as Record<string, string>)[type] || '其他'
const canvasFilteredOperationLogs = computed(() => {
  const records = canvasWorkbenchActivities.value.filter(item => {
    if (canvasOperationLogsCategory.value !== '全部' && canvasOperationLogBucket(item.targetType) !== canvasOperationLogsCategory.value) return false
    const date = item.createdAt?.slice(0, 10) || ''
    if (canvasOperationLogsStart.value && date && date < canvasOperationLogsStart.value) return false
    if (canvasOperationLogsEnd.value && date && date > canvasOperationLogsEnd.value) return false
    return true
  })
  return records
})
const canvasCommunityPickerOpen = ref(false)
const canvasCommunityQuery = ref('')
const canvasCommunityMapResults = ref<LocationSearchItem[]>([])
const canvasCommunityMapLoading = ref(false)
const canvasCommunityMapSearched = ref(false)
const canvasDoorplatePickerOpen = ref(false)
const canvasDoorplateMode = ref<'catalog' | 'manual'>('catalog')
const canvasDoorplateRows = ref<{ building: string; unit: string; room: string }[]>([])
const canvasDoorplateStep = ref<'building' | 'unit' | 'room'>('building')
const canvasSelectedBuilding = ref('')
const canvasSelectedUnit = ref('')
const canvasMaterials = ref<Property[]>([])
const canvasMaterialCategories = ref<string[]>([])
const canvasMaterialSelectedCategory = ref('全部')
const canvasMaterialLayout = ref('全部')
const canvasMaterialShowConverted = ref(false)
const canvasMaterialShowPrice = ref(false)
const canvasMaterialFormOpen = ref(false)
const canvasMaterialSampleMedia = ref(false)
const canvasMaterialDraft = reactive({ community: '', title: '', layout: '一室', rentType: '整租', rentMonthly: '', areaSqm: '', description: '' })
const canvasMaterialCategoryDialogOpen = ref(false)
const canvasMaterialNewCategory = ref('')
const canvasMaterialCategoryDrafts = reactive<Record<string, string>>({})
const activeChannelIndex = ref(0)
const areaEntryChannels = computed(() => publicConfig.featuredChannels.filter(item => item.showInAreaEntry))
const activeChannel = computed(() => publicConfig.featuredChannels[activeChannelIndex.value])
function channelSetupStatus(item: ChannelConfig) {
  if (!item.title.trim()) return '待填写'
  if (!item.community) return '待选小区'
  return item.latitude && item.longitude ? '已关联位置' : '已选小区'
}
const facilityCatalog = [
  { key: 'refrigerator', label: '冰箱', group: '家具家电', icon: '▥' }, { key: 'washer', label: '洗衣机', group: '家具家电', icon: '◉' },
  { key: 'air_conditioner', label: '空调', group: '家具家电', icon: '❄' }, { key: 'water_heater', label: '热水器', group: '家具家电', icon: '♨' },
  { key: 'television', label: '电视', group: '家具家电', icon: '▣' }, { key: 'broadband', label: '宽带', group: '家具家电', icon: '⌁' },
  { key: 'gas', label: '燃气', group: '厨房', icon: '⌁' }, { key: 'hood', label: '油烟机', group: '厨房', icon: '⌂' },
  { key: 'sofa', label: '沙发', group: '家具家电', icon: '▰' }, { key: 'wardrobe', label: '衣柜', group: '家具家电', icon: '▤' },
  { key: 'desk', label: '书桌', group: '家具家电', icon: '▱' }, { key: 'bed', label: '床', group: '家具家电', icon: '▰' },
  { key: 'elevator', label: '电梯', group: '居住条件', icon: '↕' }, { key: 'private_bathroom', label: '独立卫生间', group: '居住条件', icon: '◌' },
  { key: 'balcony', label: '阳台', group: '居住条件', icon: '▱' }, { key: 'smart_lock', label: '智能门锁', group: '安全', icon: '◈' }, { key: 'parking', label: '车位', group: '配套服务', icon: 'P' },
  { key: 'microwave', label: '微波炉', group: '厨房', icon: '' },
  { key: 'dishwasher', label: '洗碗机', group: '厨房', icon: '' },
  { key: 'dryer', label: '烘干机', group: '家具家电', icon: '' },
  { key: 'curtains', label: '窗帘', group: '家具家电', icon: '' },
  { key: 'dining_table', label: '餐桌', group: '家具家电', icon: '' },
  { key: 'range', label: '电磁炉', group: '厨房', icon: '' },
]
const facilityToAdd = ref('')
const availableFacilityCatalog = computed(() => facilityCatalog.filter(item =>
  !['elevator', 'private_bathroom', 'balcony'].includes(item.key) &&
  !publicConfig.propertyDetail.facilities.some(facility => facility.key === item.key && facility.enabled),
))
const detailFacilityIcon = (key: string) => {
  const icons: Record<string, string> = {
    refrigerator: new URL('../../../miniapp/src/static/amenities/fridge.svg', import.meta.url).href,
    washer: new URL('../../../miniapp/src/static/amenities/washer.svg', import.meta.url).href,
    air_conditioner: new URL('../../../miniapp/src/static/amenities/air-conditioner.svg', import.meta.url).href,
    water_heater: new URL('../../../miniapp/src/static/amenities/water-heater.svg', import.meta.url).href,
    television: new URL('../../../miniapp/src/static/amenities/tv.svg', import.meta.url).href,
    broadband: new URL('../../../miniapp/src/static/amenities/wifi.svg', import.meta.url).href,
    gas: new URL('../../../miniapp/src/static/amenities/gas.svg', import.meta.url).href,
    hood: new URL('../../../miniapp/src/static/amenities/hood.svg', import.meta.url).href,
    sofa: new URL('../../../miniapp/src/static/amenities/sofa.svg', import.meta.url).href,
    wardrobe: new URL('../../../miniapp/src/static/amenities/wardrobe.svg', import.meta.url).href,
    desk: new URL('../../../miniapp/src/static/amenities/desk.svg', import.meta.url).href,
    bed: new URL('../../../miniapp/src/static/amenities/bed.svg', import.meta.url).href,
    microwave: new URL('../../../miniapp/src/static/amenities/microwave.svg', import.meta.url).href,
    dishwasher: new URL('../../../miniapp/src/static/amenities/dishwasher.svg', import.meta.url).href,
    dryer: new URL('../../../miniapp/src/static/amenities/dryer.svg', import.meta.url).href,
    curtains: new URL('../../../miniapp/src/static/amenities/curtains.svg', import.meta.url).href,
    dining_table: new URL('../../../miniapp/src/static/amenities/dining_table.svg', import.meta.url).href,
    range: new URL('../../../miniapp/src/static/amenities/range.svg', import.meta.url).href,
    smart_lock: new URL('../../../miniapp/src/static/amenities/smart-lock.svg', import.meta.url).href,
    parking: new URL('../../../miniapp/src/static/amenities/parking.svg', import.meta.url).href,
  }
  return icons[key] || new URL('../../../miniapp/src/static/amenities/generic.svg', import.meta.url).href
}
const normalizeDetailFacilities = (facilities: ReturnType<typeof defaultPropertyDetail>['facilities']) =>
  facilities.map(facility => ({ ...facility, label: facility.label.trim(), group: facility.group.trim() }))
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
  return previewDoorplate(item)
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
  if (areaLabel === label) return true
  const selectedAreaChannel = publicConfig.featuredChannels.find((channel) => channel.title === label)
  if (selectedAreaChannel?.community && normalizeCommunityIdentity(item.community || '') === normalizeCommunityIdentity(selectedAreaChannel.community)) return true
  const candidates = publicConfig.featuredChannels.filter((channel) => channel.latitude && channel.longitude)
  if (!item.latitude || !item.longitude || !candidates.length) return false
  const nearest = candidates.reduce((current, candidate) => previewDistanceKm(Number(candidate.latitude), Number(candidate.longitude), Number(item.latitude), Number(item.longitude)) < previewDistanceKm(Number(current.latitude), Number(current.longitude), Number(item.latitude), Number(item.longitude)) ? candidate : current)
  return nearest.title === label && previewDistanceKm(Number(nearest.latitude), Number(nearest.longitude), Number(item.latitude), Number(item.longitude)) <= Number(nearest.radiusKm || 3)
}
function previewMatchesChannel(item: Property) {
  const channel = publicConfig.featuredChannels.find((candidate) => candidate.title === canvasChannelFilter.value)
  if (!channel) return true
  const channelHasCoordinates = Boolean(channel.latitude && channel.longitude)
  const itemHasCoordinates = Boolean(item.latitude && item.longitude)
  const sameCommunity = Boolean(channel.community && normalizeCommunityIdentity(item.community || '') === normalizeCommunityIdentity(channel.community))
  if (channelHasCoordinates && itemHasCoordinates) {
    const distance = previewDistanceKm(Number(channel.latitude), Number(channel.longitude), Number(item.latitude), Number(item.longitude))
    return sameCommunity || distance <= Number(channel.radiusKm || 3)
  }
  if (channel.community) return sameCommunity
  return !channel.keyword || String(item.location || '').includes(channel.keyword)
}
function previewOperationalStatus(status: string | undefined, kind: 'lead' | 'request') {
  const labels: Record<string, string> = kind === 'request'
    ? { new: '待处理', pending: '待处理', contacted: '已联系', matched: '已匹配', completed: '已完成', closed: '已关闭', invalid: '无效', inspection_pending: '待看房' }
    : { new: '待联系', contacted: '已联系', converted: '已转房源', closed: '已关闭' }
  return status ? labels[status] || status : '待处理'
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
const canvasHomeBudget = ref('不限')
const canvasHomeKind = ref('全部')
const previewHomeProperties = computed(() => previewPropertyOptions.value.filter((item) => {
  const budget = canvasHomeBudget.value
  const rent = item.rentMonthly
  return previewMatchesKind(item, canvasHomeKind.value)
    && (budget !== '≤1500' || rent <= 1500)
    && (budget !== '1500–2500' || (rent > 1500 && rent <= 2500))
    && (budget !== '2500–3500' || (rent > 2500 && rent < 3500))
    && (budget !== '≥3500' || rent >= 3500)
}))
const previewProperties = computed(() => {
  if (previewStatusScenario.value && pageEditorSection.value === 'status-labels' && previewPropertyOptions.value[0]) {
    return [{ ...previewPropertyOptions.value[0], status: previewStatusScenario.value }]
  }
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
    if (!previewMatchesChannel(item)) return false
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
const previewFavoritesEmpty = computed(() => configPage.value === 'properties' && tab.value === '小程序页面配置' && pageEditorSection.value === 'favorites-empty')
const previewPropertyEmpty = computed(() => previewFavoritesEmpty.value || (configPage.value === 'properties' && tab.value === '小程序页面配置' && ['properties-empty', 'display-guidance'].includes(pageEditorSection.value)))
const canvasPropertyEmptyTitle = computed(() => (canvasFavoriteOnly.value || previewFavoritesEmpty.value)
  ? canvasPageCopy('还没有找到关注的房源')
  : configAudience.value === 'operator'
    ? canvasPageCopy('当前筛选下暂无房源')
  : canvasAreaFilter.value !== '全部'
    ? canvasPageCopy('{area} 暂时没有合适的房源').replace(/\{area\}/g, canvasAreaFilter.value)
    : canvasSearchQuery.value.trim()
      ? canvasPageCopy('没有找到“{keyword}”相关房源').replace(/\{keyword\}/g, canvasSearchQuery.value.trim())
      : canvasPageCopy('暂时没有符合条件的房源'))
const canvasPropertyEmptyAction = computed(() => (canvasFavoriteOnly.value || previewFavoritesEmpty.value)
  ? canvasPageCopy('浏览全部房源')
  : canvasAreaFilter.value !== '全部'
    ? canvasPageCopy('关注该片区新房')
    : canvasPageCopy('提交找房需求'))
function handleCanvasPropertyEmptyAction() {
  if (canvasFavoriteOnly.value) {
    canvasPageFilterReset()
  } else if (canvasAreaFilter.value !== '全部' && canvasIdentityState.value === 'guest') {
    navigateCanvasPage('authLogin')
  } else {
    navigateCanvasPage('requests')
  }
}
function buildPreviewMasonry(items: Property[], heightPerLine: number) {
  const columns: Property[][] = [[], []]
  const heights = [0, 0]
  for (const item of items) {
    const copyLength = `${previewListingTitle(item)} ${previewTagGroups(item).all.join(' ')}`.length
    const index = heights[0] <= heights[1] ? 0 : 1
    columns[index].push(item)
    heights[index] += 360 + Math.ceil(copyLength / 12) * heightPerLine
  }
  return columns
}
const previewCustomerMasonryColumns = computed(() => buildPreviewMasonry(previewProperties.value, 38))
const previewHomeMasonryColumns = computed(() => buildPreviewMasonry(previewHomeProperties.value.slice(0, 6), 36))
const previewSelectedPropertyID = ref('')
const previewPrimaryProperty = computed(() => previewPropertyOptions.value.find((item) => item.id === previewSelectedPropertyID.value) || previewProperties.value[0] || previewPropertyOptions.value[0])
const canvasHomeMapMarkers = computed(() => {
  const grouped = new Map<string, Property[]>()
  for (const item of previewPropertyOptions.value) {
    if (!Number(item.latitude) || !Number(item.longitude)) continue
    const key = item.community || item.title || item.id
    grouped.set(key, [...(grouped.get(key) || []), item])
  }
  const groups = [...grouped.entries()].map(([community, items]) => ({ community, items, latitude: items.reduce((sum, item) => sum + Number(item.latitude), 0) / items.length, longitude: items.reduce((sum, item) => sum + Number(item.longitude), 0) / items.length }))
  if (!groups.length) return []
  const minLat = Math.min(...groups.map(item => item.latitude)); const maxLat = Math.max(...groups.map(item => item.latitude))
  const minLng = Math.min(...groups.map(item => item.longitude)); const maxLng = Math.max(...groups.map(item => item.longitude))
  return groups.map(item => ({ ...item, x: maxLng === minLng ? 50 : 8 + (item.longitude - minLng) / (maxLng - minLng) * 82, y: maxLat === minLat ? 50 : 8 + (maxLat - item.latitude) / (maxLat - minLat) * 78, property: item.items[0] }))
})
const canvasHomeMapPropertyCount = computed(() => previewPropertyOptions.value.filter(item => Number(item.latitude) && Number(item.longitude)).length)
const canvasDataState = computed<CanvasDataState>(() => {
  if (loadingSections.value.includes('房源')) return 'loading'
  if (failedLoadSections.value.includes('房源')) return 'error'
  if (['home', 'properties'].includes(configPage.value)) return previewPropertyOptions.value.length ? 'normal' : 'empty'
  if (configPage.value === 'propertyDetail') return previewPrimaryProperty.value || previewFacilityCatalog.value ? 'normal' : 'empty'
  return 'normal'
})
function setCanvasNotice(message = '此画布只读，未提交任何业务数据。') {
  canvasNotice.value = message
  window.setTimeout(() => {
    if (canvasNotice.value === message) canvasNotice.value = ''
  }, 2800)
}
function updateCanvasFit() {
  const stage = canvasPreviewStage.value
  const phone = canvasPreviewPhone.value
  if (!stage || !phone) return
  canvasTargetWidth.value = 604 / window.devicePixelRatio
  const width = parseFloat(getComputedStyle(phone).width) || 203
  const height = parseFloat(getComputedStyle(phone).height) || 441
  canvasFitScale.value = Math.min(canvasTargetWidth.value / width, stage.clientWidth / width, Math.max(0, stage.clientHeight - 38) / height)
}
function navigateCanvasPage(key: string) {
  if (key === configPage.value) return
  const current = configPage.value
  selectConfigPage(key)
  canvasPageHistory.value.push(current)
}
function searchFromCanvasHome() {
  canvasPageFilterReset()
  canvasSearchQuery.value = canvasHomeSearchQuery.value.trim()
  switchCanvasTab('properties')
}
function selectCanvasChannel(item: ChannelConfig) {
  canvasPageFilterReset()
  canvasChannelFilter.value = item.title
  canvasAllChannelsOpen.value = false
  switchCanvasTab('properties')
}
function switchCanvasTab(key: string) {
  canvasPageHistory.value = []
  selectConfigPage(key)
}
function isCanvasTabActive(key: string) {
  return configPage.value === key
}
function backCanvasPage() {
  const previous = canvasPageHistory.value.pop()
  if (previous) { selectConfigPage(previous); return }
  selectConfigPage(configAudience.value === 'customer' ? 'properties' : 'me')
}
function switchCanvasIdentity(value: 'guest' | 'user' | 'admin') {
  canvasIdentityState.value = value
  const audience: ConfigAudience = value === 'admin' ? 'operator' : 'customer'
  configAudience.value = audience
  canvasPageHistory.value = []
  canvasPageFilterReset()
  selectConfigPage('home')
}
function canvasPageFilterReset() {
  canvasHomeBudget.value = '不限'
  canvasHomeKind.value = '全部'
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
  if (configAudience.value === 'customer' && canvasIdentityState.value === 'guest' && configPage.value === 'requests') {
    canvasAccessPrompt.value = { title: '登录后提交', content: '登录后才能保存并跟进你的找房需求', destination: 'authLogin' }
    return
  }
  canvasDetailModal.value = ''
  setCanvasNotice('预览模式不会提交、修改或创建业务数据。')
}
function requireCanvasLogin(title: string, content: string, destination: string) {
  if (canvasIdentityState.value !== 'guest') {
    navigateCanvasPage(destination)
    return
  }
  canvasAccessPrompt.value = { title, content, destination }
}
function confirmCanvasLogin() {
  const destination = canvasAccessPrompt.value?.destination || 'authLogin'
  canvasAccessPrompt.value = null
  navigateCanvasPage(destination)
}
function confirmCanvasDate() {
  canvasForm.value.moveInDate = canvasDatePickerValue.value
  canvasDatePickerOpen.value = false
}
function updateCanvasCommunity(event: Event) {
  const community = String((event.target as HTMLInputElement).value || '')
  const previousCommunity = canvasPropertyDraft.value.community || ''
  const currentTitle = canvasPropertyDraft.value.title || ''
  canvasPropertyDraft.value.community = community
  if (!currentTitle || currentTitle === previousCommunity) canvasPropertyDraft.value.title = community
}
function openCanvasCommunityPicker() {
  canvasCommunityQuery.value = ''
  canvasCommunityMapResults.value = []
  canvasCommunityMapSearched.value = false
  canvasCommunityPickerOpen.value = true
}
function selectCanvasLibraryCommunity(name: string) {
  const location = savedCommunityLocations.value[name]
  canvasPropertyDraft.value.community = name
  canvasPropertyDraft.value.title = canvasPropertyDraft.value.title || name
  canvasPropertyDraft.value.location = location?.address || ''
  canvasPropertyDraft.value.latitude = location?.latitude ? String(location.latitude) : ''
  canvasPropertyDraft.value.longitude = location?.longitude ? String(location.longitude) : ''
  canvasPropertyDraft.value.buildingNo = ''
  canvasPropertyDraft.value.unitNo = ''
  canvasPropertyDraft.value.roomNo = ''
  canvasCommunityPickerOpen.value = false
  canvasDoorplateRows.value = []
  canvasDoorplateMode.value = 'catalog'
  setCanvasNotice(location?.address ? '已选择小区库记录，位置已带入。' : '已选择小区；该小区位置待补充。')
}
async function searchCanvasCommunityMap() {
  const keyword = canvasCommunityQuery.value.trim()
  if (keyword.length < 2 || canvasCommunityMapLoading.value) return
  canvasCommunityMapLoading.value = true
  canvasCommunityMapSearched.value = true
  try {
    canvasCommunityMapResults.value = await api<LocationSearchItem[]>(`/v1/admin/location/search?keyword=${encodeURIComponent(keyword)}&latitude=30.45604&longitude=114.425337`)
  } catch (error) {
    canvasCommunityMapResults.value = []
    setCanvasNotice(error instanceof Error ? error.message : '地图搜索失败，请重试。')
  } finally { canvasCommunityMapLoading.value = false }
}
function selectCanvasMapCommunity(item: LocationSearchItem) {
  canvasPropertyDraft.value.community = item.title
  canvasPropertyDraft.value.title = canvasPropertyDraft.value.title || item.title
  canvasPropertyDraft.value.location = item.address || item.district || ''
  canvasPropertyDraft.value.latitude = String(item.latitude || '')
  canvasPropertyDraft.value.longitude = String(item.longitude || '')
  canvasPropertyDraft.value.buildingNo = ''
  canvasPropertyDraft.value.unitNo = ''
  canvasPropertyDraft.value.roomNo = ''
  canvasCommunityPickerOpen.value = false
  canvasDoorplateRows.value = []
  canvasDoorplateMode.value = 'manual'
  setCanvasNotice('已选择地图小区；请自定义输入门栋、单元和房号。')
}
async function openCanvasDoorplatePicker() {
  if (!canvasPropertyDraft.value.community) { setCanvasNotice('请先选择小区。'); return }
  try {
    canvasDoorplateRows.value = await api<{ building: string; unit: string; room: string }[]>(`/v1/admin/doorplates?community=${encodeURIComponent(canvasPropertyDraft.value.community)}`)
  } catch { canvasDoorplateRows.value = [] }
  canvasDoorplateMode.value = canvasDoorplateBuildings.value.length ? 'catalog' : 'manual'
  canvasSelectedBuilding.value = canvasPropertyDraft.value.buildingNo || ''
  canvasSelectedUnit.value = canvasPropertyDraft.value.unitNo || ''
  canvasDoorplateStep.value = canvasSelectedBuilding.value ? (canvasSelectedUnit.value ? 'room' : 'unit') : 'building'
  canvasDoorplatePickerOpen.value = true
}
function selectCanvasDoorplate(building: string, unit = '', room = '') {
  if (canvasDoorplateStep.value === 'building') {
    canvasSelectedBuilding.value = building
    canvasSelectedUnit.value = ''
    canvasDoorplateStep.value = 'unit'
  } else if (canvasDoorplateStep.value === 'unit') {
    canvasSelectedUnit.value = unit
    canvasDoorplateStep.value = 'room'
  } else {
    canvasPropertyDraft.value.buildingNo = canvasSelectedBuilding.value
    canvasPropertyDraft.value.unitNo = canvasSelectedUnit.value
    canvasPropertyDraft.value.roomNo = room
    canvasDoorplatePickerOpen.value = false
  }
}
function saveCanvasManualDoorplate() { canvasDoorplatePickerOpen.value = false }

function startCanvasPropertyEdit(item?: Property) {
  previewSelectedPropertyID.value = item?.id || ''
  canvasEditingExistingProperty.value = Boolean(item)
  const defaultLayout = item?.layout || '1室1厅1厨1卫'
  const roomCount = defaultLayout.match(/(\d+)室/)?.[1] || '1'
  const hallCount = defaultLayout.match(/(\d+)厅/)?.[1] || '1'
  const kitchenCount = defaultLayout.match(/(\d+)厨/)?.[1] || '1'
  const bathroomCount = defaultLayout.match(/(\d+)卫/)?.[1] || '1'
  const balconyCount = defaultLayout.match(/(\d+)阳台/)?.[1] || '0'
  const savedAmenities = item?.description?.match(/^配套设施：([^\n]*)/)?.[1]?.split('、').filter(Boolean)
  canvasQuickSettingsOpen.value = false
  canvasCommunityPickerOpen.value = false
  canvasDoorplatePickerOpen.value = false
  canvasCropPreviewOpen.value = false
  canvasSampleImageAdded.value = false
  canvasSampleImageCropped.value = false
  canvasCropZoom.value = 130
  Object.assign(canvasPropertyDraft.value, {
    title: item?.title || item?.community || '', community: item?.community || '', location: item?.location || '', latitude: item?.latitude ? String(item.latitude) : '', longitude: item?.longitude ? String(item.longitude) : '', buildingNo: item?.buildingNo || '', unitNo: item?.unitNo || '', roomNo: item?.roomNo || '', roomSequence: item?.roomSequence ? String(item.roomSequence) : '0', rentType: item?.rentType || '整租', listingSource: item?.listingSource || '直租', layout: defaultLayout, wholeLayout: item?.wholeLayout || defaultLayout, wholeAreaSqm: item?.wholeAreaSqm ? String(item.wholeAreaSqm) : '', layoutChosen: item ? 'true' : '', roomCountChosen: item ? 'true' : '', rooms: roomCount, halls: hallCount, kitchens: kitchenCount === '0' ? '无' : '有', bathrooms: bathroomCount, balconies: balconyCount === '0' ? '无' : balconyCount, roomType: defaultLayout.split(/[·・|｜]/)[0] || '主卧', privateBathroom: defaultLayout.includes('独卫') ? 'true' : '', hasBayWindow: defaultLayout.includes('有飘窗') ? 'true' : '', areaSqm: item?.areaSqm ? String(item.areaSqm) : '', floorInfo: item?.floorInfo || '', orientation: item?.orientation || '', elevator: item?.elevator || '', rentMonthly: item?.rentMonthly != null ? String(item.rentMonthly) : '', feeText: item?.feeText || '无中介费', depositPayment: item?.depositPayment || '押一付一', availableDate: item?.availableDate || '拎包入住', utilitiesText: item?.utilitiesText || '民水民电', description: item?.description?.replace(/^配套设施：[^\n]*\n?/, '') || '', rulesText: item?.rulesText || '', coverFocusX: String(item?.coverFocusX ?? 50), coverFocusY: String(item?.coverFocusY ?? 50), imageCount: String(item?.media?.filter(media => media.type === 'image').length || 0), videoCount: String(item?.media?.filter(media => media.type === 'video').length || 0), channel: item?.areaLabel || areaEntryChannels.value[0]?.title || '', media: '',
  })
  canvasPropertyFacilities.value = savedAmenities || (item ? [] : publicConfig.propertyDetail.facilities.filter(facility => facility.enabled).slice(0, 4).map(facility => facility.label))
  canvasEditStep.value = 0
  navigateCanvasPage('adminPropertyEdit')
}
function addCanvasSampleImage() {
  if (canvasIdentityState.value !== 'admin') {
    setCanvasNotice('仅管理员可添加房源图片。')
    return
  }
  const count = Number(canvasPropertyDraft.value.imageCount) || 0
  if (count >= 9) {
    setCanvasNotice('房源最多添加 9 张图片。')
    return
  }
  canvasSampleImageAdded.value = true
  canvasSampleImageCropped.value = false
  canvasPropertyDraft.value.imageCount = String(count + 1)
  setCanvasNotice('已在画布添加示例图片，可继续预览裁切流程。')
}
function removeCanvasSampleImage() {
  if (!canvasSampleImageAdded.value) return
  canvasSampleImageAdded.value = false
  canvasSampleImageCropped.value = false
  canvasPropertyDraft.value.imageCount = String(Math.max(0, (Number(canvasPropertyDraft.value.imageCount) || 0) - 1))
}
function openCanvasCropPreview() {
  if (canvasIdentityState.value !== 'admin') {
    setCanvasNotice('仅管理员可调整房源图片。')
    return
  }
  canvasCropZoom.value = canvasSampleImageCropped.value ? 150 : 130
  canvasCropOffset.value = { x: 0, y: 0 }
  canvasCropPreviewOpen.value = true
}
function startCanvasCropDrag(event: PointerEvent) {
  const target = event.currentTarget as HTMLElement
  target.setPointerCapture?.(event.pointerId)
  canvasCropDragStart = { x: event.clientX, y: event.clientY, ...canvasCropOffset.value }
}
function moveCanvasCropDrag(event: PointerEvent) {
  if (!canvasCropDragStart) return
  canvasCropOffset.value = {
    x: canvasCropDragStart.offsetX + event.clientX - canvasCropDragStart.x,
    y: canvasCropDragStart.offsetY + event.clientY - canvasCropDragStart.y,
  }
}
function endCanvasCropDrag() { canvasCropDragStart = null }
function resetCanvasCropPosition() {
  canvasCropZoom.value = 100
  canvasCropOffset.value = { x: 0, y: 0 }
}
function applyCanvasCropPreview() {
  canvasSampleImageCropped.value = true
  canvasCropPreviewOpen.value = false
  setCanvasNotice('示例裁切已应用到画布预览，未上传或保存数据。')
}
function updateCanvasWholeLayout(key: string, value: string) {
  canvasPropertyDraft.value[key] = value
  canvasPropertyDraft.value.layoutChosen = 'true'
  if (key === 'rooms') canvasPropertyDraft.value.roomCountChosen = 'true'
  const rooms = canvasPropertyDraft.value.rooms === '手动' ? (canvasPropertyDraft.value.customRooms || '1') : (canvasPropertyDraft.value.rooms || '1')
  const halls = canvasPropertyDraft.value.halls || '0'
  const kitchen = canvasPropertyDraft.value.kitchens === '无' ? '0' : '1'
  const bathrooms = canvasPropertyDraft.value.bathrooms || '1'
  const balconies = canvasPropertyDraft.value.balconies === '无' ? '0' : (canvasPropertyDraft.value.balconies || '0')
  const wholeLayout = [
    `${Math.max(1, Number(rooms) || 1)}室`,
    Number(halls) ? `${halls}厅` : '',
    Number(kitchen) ? `${kitchen}厨` : '',
    Number(bathrooms) ? `${bathrooms}卫` : '',
    Number(balconies) ? `${balconies}阳台` : '',
  ].filter(Boolean).join('')
  canvasPropertyDraft.value.wholeLayout = wholeLayout
  canvasPropertyDraft.value.layout = canvasPropertyDraft.value.rentType === '合租'
    ? [canvasPropertyDraft.value.roomType || '主卧', canvasPropertyDraft.value.privateBathroom ? '独卫' : '', canvasPropertyDraft.value.hasBayWindow ? '有飘窗' : ''].filter(Boolean).join(' · ')
    : wholeLayout
  const selectedRoomCount = Number(rooms) || 1
  if (Number(canvasPropertyDraft.value.roomSequence) > selectedRoomCount) canvasPropertyDraft.value.roomSequence = '0'
}
function toggleCanvasPropertyFacility(label: string) {
  canvasPropertyFacilities.value = canvasPropertyFacilities.value.includes(label)
    ? canvasPropertyFacilities.value.filter(item => item !== label)
    : [...canvasPropertyFacilities.value, label]
}
const canvasSelectedRoomCount = computed(() => Math.min(9, Math.max(1,
  Number(canvasPropertyDraft.value.rooms === '手动' ? canvasPropertyDraft.value.customRooms : canvasPropertyDraft.value.rooms) || 1,
)))
const canvasLayoutReady = computed(() => Boolean(
  canvasPropertyDraft.value.layoutChosen
  && canvasPropertyDraft.value.roomCountChosen
  && canvasPropertyDraft.value.layout?.trim()
  && (canvasPropertyDraft.value.rooms !== '手动' || /^[1-9]$/.test(canvasPropertyDraft.value.customRooms || '')),
))
const canvasDraftProperty = computed(() => ({
  id: 'current', community: canvasPropertyDraft.value.community, title: canvasPropertyDraft.value.community,
  rentMonthly: Number(canvasPropertyDraft.value.rentMonthly) || 0,
  areaSqm: Number(canvasPropertyDraft.value.areaSqm) || 0,
  layout: canvasPropertyDraft.value.layout, rentType: canvasPropertyDraft.value.rentType,
  listingSource: canvasPropertyDraft.value.listingSource, feeText: canvasPropertyDraft.value.feeText,
  floorInfo: canvasPropertyDraft.value.floorInfo, buildingNo: canvasPropertyDraft.value.buildingNo,
  unitNo: canvasPropertyDraft.value.unitNo, roomNo: canvasPropertyDraft.value.roomNo,
  roomSequence: Number(canvasPropertyDraft.value.roomSequence) || 0,
  wholeLayout: canvasPropertyDraft.value.wholeLayout,
  wholeAreaSqm: Number(canvasPropertyDraft.value.wholeAreaSqm) || 0,
  media: [
    ...(canvasEditingExistingProperty.value ? previewPrimaryProperty.value?.media || [] : []),
    ...(canvasSampleImageAdded.value ? [{ type: 'image', url: homeHeroFallback }] : []),
  ],
  coverFocusX: Number(canvasPropertyDraft.value.coverFocusX ?? 50),
  coverFocusY: Number(canvasPropertyDraft.value.coverFocusY ?? 50),
} as Property))
function previewDoorplate(item?: Property) {
  const base = [item?.buildingNo, item?.unitNo, item?.roomNo].filter(Boolean).join('') || '待补充'
  return item?.rentType === '合租' && item.roomSequence ? base + ' · ' + item.roomSequence + '号房' : base
}
const canvasReviewCards = computed(() => [{
  id: 'current', title: canvasPropertyDraft.value.community || '选择小区后自动生成',
  doorplate: previewDoorplate(canvasDraftProperty.value),
  location: canvasPropertyDraft.value.channel || '位置待补充', rent: canvasPropertyDraft.value.rentMonthly || 0,
  tags: previewTagGroups(canvasDraftProperty.value).all, cover: canvasSampleImageAdded.value || canvasEditingExistingProperty.value ? previewCoverStyle(canvasDraftProperty.value, 0, true) : {}, hasCover: Boolean(canvasDraftProperty.value.media?.length),
}, ...properties.value
  .filter(item => item.id !== previewSelectedPropertyID.value && ['published', 'available', 'rented'].includes(item.status) && item.media?.some(media => media.type === 'image'))
  .sort((left, right) => Number(right.areaLabel === canvasPropertyDraft.value.channel) - Number(left.areaLabel === canvasPropertyDraft.value.channel))
  .slice(0, 2).map(item => ({
  id: item.id, title: item.title || item.community || '房源',
  doorplate: previewDoorplate(item),
  location: previewChannelLabel(item), rent: item.rentMonthly,
  tags: previewTagGroups(item).all, cover: previewCoverStyle(item, 0, true), hasCover: true,
}))])
const canvasDraftTitle = computed(() => canvasPropertyDraft.value.title || canvasPropertyDraft.value.community || '房源小区')
const canvasPropertyStepReady = computed(() => [
  Boolean(canvasPropertyDraft.value.community && canvasPropertyDraft.value.location && canvasPropertyDraft.value.latitude && canvasPropertyDraft.value.longitude && canvasLayoutReady.value && canvasPropertyDraft.value.depositPayment),
  true,
])
const canvasPropertyCompletionCount = computed(() => canvasPropertyStepReady.value.filter(Boolean).length)
const canvasDetailScrolled = ref(false)
const canvasFootprintClearOpen = ref(false)
const canvasFootprints = ref<{ property: Property; count: number; viewedAt: Date }[]>([])
const canvasEditorRequiredReady = computed(() =>
  canvasPropertyStepReady.value[0]
  && Boolean(canvasPropertyDraft.value.community?.trim() && canvasPropertyDraft.value.location?.trim() && canvasLayoutReady.value)
  && Boolean(Number.isFinite(Number(canvasPropertyDraft.value.rentMonthly)) && Number(canvasPropertyDraft.value.rentMonthly) >= 0 && canvasPropertyDraft.value.depositPayment)
  && (canvasPropertyDraft.value.rentType !== '合租' || !canvasPropertyDraft.value.roomSequence || Number(canvasPropertyDraft.value.roomSequence) <= canvasSelectedRoomCount.value)
  && (canvasPropertyDraft.value.listingSource !== '转租' || Number(canvasPropertyDraft.value.remainingLeaseMonths) > 0),
)
const canvasPublishValidationIssues = computed(() => {
  const issues: string[] = []
  if (!canvasPropertyDraft.value.community?.trim()) issues.push('请选择小区')
  if (!canvasPropertyDraft.value.location?.trim() || !canvasPropertyDraft.value.latitude || !canvasPropertyDraft.value.longitude) issues.push('完成位置选择并获取地图坐标')
  if (!canvasLayoutReady.value) issues.push('完成房屋户型选择')
  if (canvasPropertyDraft.value.rentType === '合租' && !canvasPropertyDraft.value.wholeLayout?.trim()) issues.push('选择合租房源的整套户型')
  if (canvasPropertyDraft.value.rentType === '合租' && Number(canvasPropertyDraft.value.roomSequence) > canvasSelectedRoomCount.value) issues.push('出租房间序号不能超过整套室数')
  if (!Number.isFinite(Number(canvasPropertyDraft.value.rentMonthly)) || Number(canvasPropertyDraft.value.rentMonthly) < 0) issues.push('填写有效月租')
  if (!canvasPropertyDraft.value.depositPayment?.trim()) issues.push('选择押付方式')
  if (canvasPropertyDraft.value.listingSource === '转租' && Number(canvasPropertyDraft.value.remainingLeaseMonths) <= 0) issues.push('填写转租剩余租期')
  return issues
})
function attemptCanvasPublish() {
  if (canvasPublishValidationIssues.value.length) {
    const issues = canvasPublishValidationIssues.value
    if (issues.some(issue => issue.includes('小区') || issue.includes('地图位置'))) canvasEditStep.value = 0
    else if (issues.some(issue => issue.includes('户型') || issue.includes('房间序号'))) {
      canvasEditStep.value = 0
      canvasQuickSettingsOpen.value = true
    } else canvasEditStep.value = 2
    setCanvasNotice('发布前还需要：' + issues.join('、'))
    return
  }
  setCanvasNotice('预览模式不会提交、修改或创建业务数据；当前信息已通过发布校验。')
}
function openPreviewProperty(item?: Property) {
  if (!item) return
  if (canvasIdentityState.value === 'user') {
    const existing = canvasFootprints.value.find(entry => entry.property.id === item.id)
    canvasFootprints.value = [
      { property: item, count: (existing?.count || 0) + 1, viewedAt: new Date() },
      ...canvasFootprints.value.filter(entry => entry.property.id !== item.id),
    ]
  }
  canvasDetailScrolled.value = false
  previewSelectedPropertyID.value = item.id
  canvasMediaIndex.value = 0
  canvasMapExpanded.value = false
  navigateCanvasPage('propertyDetail')
}
function stepCanvasMedia(direction: number) {
  const media = previewPrimaryProperty.value?.media || []
  if (!media.length) return
  canvasMediaIndex.value = (canvasMediaIndex.value + direction + media.length) % media.length
}
const previewIdentity = computed(() => {
  if (configAudience.value === 'customer' && canvasIdentityState.value === 'guest') return { name: canvasPageCopy('登录后开始找房'), caption: canvasPageCopy('同步收藏、足迹和找房需求'), mark: '小' }
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
  return String(item?.title || item?.community || '房源').trim()
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
  if (previewFacilityCatalog.value) return publicConfig.propertyDetail.facilities.filter(facility => facility.enabled)
  const names = new Set(match?.[1]?.split(/[、,，]/).map(item => item.trim()).filter(Boolean) || [])
  return publicConfig.propertyDetail.facilities.filter(facility => facility.enabled && (
    names.has(facility.label) || facilityCatalog.some(preset => preset.key === facility.key && names.has(preset.label))
  ))
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
  if (['附近暂时没有其他可租房源', '可以继续查看其他地区的新房源'].includes(previewCopyScenario.value)) return []
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
function previewDisplayAddress(item?: Property | null) {
  const community = String(item?.community || '').trim()
  const rawAddress = String(item?.location || '').trim()
  const address = /^(位置待补充|地址待补充)$/.test(rawAddress) ? '' : rawAddress
  if (!address) return community || rawAddress || '位置待补充'
  if (!community) return address
  const normalize = (value: string) => value.replace(/[\s·・,，。\-—]/g, '').toLowerCase()
  return normalize(address).includes(normalize(community)) ? address : `${address} · ${community}`
}
function previewCoverStyle(item?: Property, mediaIndex = 0, plainPlaceholder = false) {
  // Use actual media where available. For records without a photo, mirror the
  // Mini Program's explicit community-name placeholder instead of a blank block.
  const media = item?.media?.[mediaIndex]
  const cover = media?.type === 'image' ? media.url : item?.media?.find((entry) => entry.type === 'image')?.url
  if (cover) return { backgroundImage: `url(${cover})`, backgroundSize: 'cover', backgroundPosition: 'center' }
  if (plainPlaceholder) return { backgroundImage: 'linear-gradient(145deg, #e4cdaa, #b9a58c)', backgroundSize: 'cover', backgroundPosition: 'center' }
  const label = (item?.community || '小皮').slice(0, 2).replace(/[&<>]/g, value => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[value] || value))
  const placeholder = `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="240" viewBox="0 0 320 240"><defs><linearGradient id="g" x2="1" y2="1"><stop stop-color="#e4cdaa"/><stop offset="1" stop-color="#b9a58c"/></linearGradient></defs><rect width="320" height="240" fill="url(#g)"/>${plainPlaceholder ? '' : '<rect x="103" y="96" width="114" height="48" rx="8" fill="none" stroke="#ffffffaa"/>'}<text x="160" y="127" fill="white" font-size="22" text-anchor="middle" font-family="sans-serif">${label}</text></svg>`
  return { backgroundImage: `url("data:image/svg+xml,${encodeURIComponent(placeholder)}")`, backgroundSize: 'cover', backgroundPosition: 'center' }
}
const shareFieldOptions = [
  { key: 'community', label: '小区名称' }, { key: 'layout', label: '户型 / 房间' }, { key: 'roomNo', label: '门牌信息' },
  { key: 'rentMonthly', label: '月租' }, { key: 'listingSource', label: '直租 / 转租' }, { key: 'rentType', label: '整租 / 合租' },
  { key: 'depositPayment', label: '押付方式' }, { key: 'areaSqm', label: '面积' }, { key: 'floorInfo', label: '楼层' }, { key: 'availableDate', label: '入住时间' },
]
const shareFieldListingMap: Record<string, string> = { layout: 'layout', roomNo: 'unit', listingSource: 'listingSource', rentType: 'rentType', areaSqm: 'area', floorInfo: 'floor' }
const shareFieldEnabledForCustomer = (key: string) => !shareFieldListingMap[key] || publicConfig.listingTitle.customerShareFields.includes(shareFieldListingMap[key])
const invalidShareTitleFields = computed(() => publicConfig.shareCard.titleFields.filter(key => !shareFieldEnabledForCustomer(key)))
function removeInvalidShareTitleFields() {
  const fields = publicConfig.shareCard.titleFields.filter(shareFieldEnabledForCustomer)
  publicConfig.shareCard.titleFields = fields.length ? fields : ['community']
}
const publicConfig = reactive<PublicConfig>({ heroTitle: '', contactPhone: '', relatedPropertyRadiusKm: 3, locations: [], featuredChannels: [], pagePresentations: defaultPagePresentations(), shareCard: defaultShareCard(), listingTitle: defaultListingTitle(), propertyDetail: defaultPropertyDetail(), statusLabels: { draft: '草稿', pendingReview: '待审核', published: '已发布', available: '可租', rented: '已租', offline: '已下架' } })

const createConfigId = (prefix: string) => `${prefix}_${globalThis.crypto?.randomUUID?.() || `${Date.now()}_${Math.random().toString(36).slice(2)}`}`

const api = createAdminApi({ baseUrl: apiBase, onUnauthorized: () => { loggedIn.value = false; password.value = '' } })
async function loadCanvasWorkbench() {
  canvasWorkbenchError.value = ''
  try {
    const [logs, contacts] = await Promise.all([
      api<typeof canvasWorkbenchActivities.value>('/v1/admin/operation-logs'),
      api<typeof canvasWorkbenchContacts.value>('/v1/admin/contact-events'),
    ])
    canvasWorkbenchActivities.value = logs || []
    canvasWorkbenchContacts.value = contacts || []
  } catch (error) { canvasWorkbenchError.value = error instanceof Error ? error.message : '工作台数据加载失败' }
}
async function loadCanvasMaterials() {
  const [materials, categories] = await Promise.allSettled([
    api<Property[]>('/v1/admin/material-library'),
    api<string[]>('/v1/admin/material-library/categories'),
  ])
  if (materials.status === 'fulfilled') canvasMaterials.value = materials.value || []
  if (categories.status === 'fulfilled') canvasMaterialCategories.value = categories.value || []
  if (canvasMaterialSelectedCategory.value !== '全部' && !canvasMaterialCategories.value.includes(canvasMaterialSelectedCategory.value)) canvasMaterialSelectedCategory.value = '全部'
}
function beginCanvasMaterialCreate() {
  canvasMaterialDraft.community = canvasMaterialCategories.value[0] || ''
  canvasMaterialDraft.title = ''
  canvasMaterialDraft.layout = '一室'
  canvasMaterialDraft.rentType = '整租'
  canvasMaterialDraft.rentMonthly = ''
  canvasMaterialDraft.areaSqm = ''
  canvasMaterialDraft.description = ''
  canvasMaterialSampleMedia.value = false
  canvasMaterialFormOpen.value = true
}
function openCanvasMaterialCategories() {
  for (const name of canvasMaterialCategoryOptions.value) canvasMaterialCategoryDrafts[name] = name
  canvasMaterialCategoryDialogOpen.value = true
}
function addCanvasMaterialCategory() {
  const name = canvasMaterialNewCategory.value.trim()
  if (!name || canvasMaterialCategoryOptions.value.includes(name)) return
  canvasMaterialCategories.value = [...canvasMaterialCategories.value, name]
  canvasMaterialCategoryDrafts[name] = name
  canvasMaterialNewCategory.value = ''
  setCanvasNotice('分类已加入画布预览；尚未写入服务器。')
}
function renameCanvasMaterialCategory(from: string) {
  const to = (canvasMaterialCategoryDrafts[from] || '').trim()
  if (!to || to === from || canvasMaterialCategoryOptions.value.includes(to)) return
  canvasMaterialCategories.value = canvasMaterialCategories.value.map(name => name === from ? to : name)
  canvasMaterials.value = canvasMaterials.value.map(item => canvasMaterialCommunity(item) === from ? { ...item, materialCategory: to } as Property : item)
  canvasMaterialCategoryDialogOpen.value = false
  setCanvasNotice('分类名称已在画布预览中更新；尚未写入服务器。')
}
function deleteCanvasMaterial(item: Property) {
  canvasMaterials.value = canvasMaterials.value.filter(record => record.id !== item.id)
  setCanvasNotice('素材已从画布预览移除；未删除服务器数据。')
}
function saveCanvasMaterialDraft() {
  if (!canvasMaterialDraft.community) { setCanvasNotice('请先选择小区分类。'); return }
  if (!canvasMaterialSampleMedia.value) { setCanvasNotice('素材至少需要一张图片或一个视频。'); return }
  const sample = previewPropertyOptions.value.find(item => item.media?.length)
  if (!sample?.media?.length) { setCanvasNotice('当前预览没有可用媒体，画布不会上传文件。'); return }
  const draft: Property & { materialCategory: string } = {
    id: `canvas-material-${Date.now()}`, title: canvasMaterialDraft.title || canvasMaterialDraft.community,
    community: canvasMaterialDraft.community, materialCategory: canvasMaterialDraft.community,
    location: savedCommunityLocations.value[canvasMaterialDraft.community]?.address || sample.location || '',
    latitude: sample.latitude, longitude: sample.longitude, layout: canvasMaterialDraft.layout,
    rentType: canvasMaterialDraft.rentType, rentMonthly: Number(canvasMaterialDraft.rentMonthly) || 0,
    areaSqm: Number(canvasMaterialDraft.areaSqm) || undefined, description: canvasMaterialDraft.description,
    status: 'material', media: sample.media?.slice(0, 2) || [],
  }
  canvasMaterials.value = [draft, ...canvasMaterials.value]
  canvasMaterialFormOpen.value = false
  canvasMaterialSelectedCategory.value = '全部'
  canvasMaterialShowConverted.value = false
  setCanvasNotice('已在画布预览中添加素材示例；没有写入业务数据。')
}
function convertCanvasMaterial(item: Property) {
  startCanvasPropertyEdit(item)
  setCanvasNotice('已将素材信息带入房源编辑画布；发布操作不会写入业务数据。')
}
watch(configPage, value => {
  if (value === 'adminWorkbench') void loadCanvasWorkbench()
  if (value === 'adminMaterialLibrary') void loadCanvasMaterials()
})
watch(canvasEditStep, async index => {
  await nextTick()
  const phone = canvasPreviewPhone.value
  const steps = phone?.querySelector<HTMLElement>('.preview-steps')
  if (steps) steps.scrollLeft = Math.max(0, Math.min(index - 1, 1)) * 40.4
  const content = phone?.querySelector<HTMLElement>('.page-mini-content')
  if (content) content.scrollTop = 0
})


const cloneConfig = (value: PublicConfig): PublicConfig => JSON.parse(JSON.stringify(value))
const applyConfig = (value: PublicConfig, markSaved = false) => {
  const next = cloneConfig(value)
  next.relatedPropertyRadiusKm = next.relatedPropertyRadiusKm || 3
  next.listingTitle = { ...defaultListingTitle(), ...(next.listingTitle || {}) }
  next.listingTitle.marketingPhrases = (next.listingTitle.marketingPhrases || []).map(item => String(item || '').trim()).filter(Boolean)
  next.locations = (next.locations || []).map(item => ({ ...item, id: item.id || createConfigId('loc') }))
  next.featuredChannels = (next.featuredChannels || []).map((item, index) => ({ ...item, id: item.id || createConfigId('channel'), latitude: item.latitude || 0, longitude: item.longitude || 0, icon: normalizeChannelStyle(item.icon, index), radiusKm: item.radiusKm || 3 }))
  // Upgrade old configs: locations used to mirror channel titles by position.
  if (!next.featuredChannels.some(item => item.showInAreaEntry)) {
    const labels = new Set((next.locations || []).map(item => item.label.trim()).filter(Boolean))
    const matched = next.featuredChannels.filter(item => labels.has(item.title.trim()))
    ;(matched.length ? matched : next.featuredChannels.slice(0, 3)).slice(0, 4).forEach(item => { item.showInAreaEntry = true })
  }
  next.locations = next.featuredChannels.filter(item => item.showInAreaEntry).map(item => ({ id: item.id, label: item.title, keyword: item.community || item.keyword }))
  const savedPresentations = next.pagePresentations || {}
  next.pagePresentations = { ...savedPresentations }
  for (const [key, defaults] of Object.entries(defaultPagePresentations())) {
    next.pagePresentations[key] = { ...defaults, ...savedPresentations[key] }
  }
  const home = next.pagePresentations.home
  home.nearbyLabel ||= '附近'
  home.channelTitle ||= '精选频道'
  home.allChannelsText ||= '全部频道'
  home.recommendationTitle ||= '推荐房源'
  home.viewAllText ||= '查看房源'
  const properties = next.pagePresentations.properties
  properties.operatorTitle ||= '房源管理'
  properties.searchPlaceholder ||= '搜索小区、位置或户型'
  const me = next.pagePresentations.me
  me.eyebrow ||= '个人中心'
  me.operatorSubtitle ||= '运营管理与待办事项'
  next.shareCard = { ...defaultShareCard(), ...(next.shareCard || {}) }
  next.listingTitle = { ...defaultListingTitle(), ...(next.listingTitle || {}) }
  next.propertyDetail = { ...defaultPropertyDetail(), ...(next.propertyDetail || {}) }
  next.statusLabels = Object.assign({ draft: '草稿', pendingReview: '待审核', published: '已发布', available: '可租', rented: '已租', offline: '已下架' }, next.statusLabels || {})
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
  disabledCustomerTagPlacements.clear()
  Object.assign(publicConfig, next)
  preloadChannelMaps()
  if (markSaved) savedConfigJSON.value = JSON.stringify(publicConfig)
}

const loadingSections = ref<string[]>([])
const failedLoadSections = ref<string[]>([])
let workspaceLoadSequence = 0

async function load() {
  const sequence = ++workspaceLoadSequence
  overviewState.value = 'loading'
  overviewError.value = ''
  pageError.value = ''
  failedLoadSections.value = []
  const overviewSections = ['房源', '线索', '租客需求', '用户意向', '微信用户']
  loadingSections.value = [...overviewSections, '小程序配置', '管理员', '临时访问码', '门牌数据', '小区位置']

  async function read<T>(section: string, path: string, apply: (data: T) => void) {
    try {
      const data = await api<T>(path)
      if (sequence === workspaceLoadSequence) apply(data)
    } catch {
      if (sequence === workspaceLoadSequence) {
        failedLoadSections.value.push(section)
        pageError.value = `部分数据加载失败：${failedLoadSections.value.join('、')}。请刷新重试。`
      }
    } finally {
      if (sequence === workspaceLoadSequence) loadingSections.value = loadingSections.value.filter(name => name !== section)
    }
  }

  await Promise.all([
    Promise.all([
      read<Property[]>('房源', '/v1/properties', data => { properties.value = data || [] }),
      read<Lead[]>('线索', '/v1/property-leads', data => { leads.value = data || [] }),
      read<RentalRequest[]>('租客需求', '/v1/admin/rental-requests', data => { rentalRequests.value = data || [] }),
      read<FavoriteContact[]>('用户意向', '/v1/admin/favorite-contacts', data => { favoriteContacts.value = data || [] }),
      read<WechatUser[]>('微信用户', '/v1/admin/users', data => { users.value = data || [] }),
    ]).then(() => {
      if (sequence !== workspaceLoadSequence) return
      overviewState.value = overviewSections.every(section => failedLoadSections.value.includes(section)) ? 'error' : 'ready'
      if (overviewState.value === 'error') overviewError.value = '运营概况加载失败，请刷新重试。'
    }),
    read<ConfigState>('小程序配置', '/v1/admin/config/state', data => {
      if (configDirty.value || savingConfig.value) return
      applyConfig(data.config, true)
      configRevision.value = data.revision
    }),
    read<AdminUser[]>('管理员', '/v1/admin/admin-users', data => { admins.value = data || [] }),
    read<TemporaryAccess[]>('临时访问码', '/v1/admin/temporary-access', data => { temporaryAccess.value = data || [] }),
    read<Record<string, number>>('门牌数据', '/v1/admin/doorplates', data => { doorplateCommunityCounts.value = data || {} }),
    read<Record<string, { address: string; latitude: number; longitude: number }>>('小区位置', '/v1/admin/community-locations', data => { savedCommunityLocations.value = data || {} }),
  ])
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
    sessionStorage.setItem(passwordNoticeKey, String(mustChangePassword.value))
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
  workspaceLoadSequence++
  loadingSections.value = []
  savedConfigJSON.value = ''
  sessionStorage.removeItem('xiaopi_web_admin_super')
  sessionStorage.removeItem(passwordNoticeKey)
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
  if (overviewState.value !== 'loading') void load().catch(() => undefined)
}

function selectTab(value: string, filter?: string) {
  if (value === '后台管理员' && webAdminSuper.value) void refreshWebAccounts()
  const next = value as TabKey
  tab.value = next
  void router.push({ name: 'admin', params: { tab: pathForTab(next) }, query: filter ? { filter } : {} })
}

watch(() => route.params.tab, value => { tab.value = tabFromRoute(value) as TabKey })

router.afterEach((to, _from, failure) => {
  if (!failure) tab.value = tabFromRoute(to.params.tab) as TabKey
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
  if (status === 'available') return ''
  if (status === 'offline') return '已下架预览'
  if (status === 'pending_review') return '待审核预览'
  if (status === 'rented') return '已租预览'
  if (status === 'published') return '待设可租预览'
  return '草稿预览'
})
const previewAdminStatusActions = computed(() => {
  switch (previewPrimaryProperty.value?.status) {
    case 'draft':
    case 'pending_review': return []
    case 'published': return [{ status: 'available', label: '改为可租', tone: 'primary' }, { status: 'rented', label: '改为已租', tone: 'secondary' }, { status: 'offline', label: '下架', tone: 'secondary' }]
    case 'available': return [{ status: 'rented', label: '改为已租', tone: 'primary' }, { status: 'offline', label: '下架', tone: 'secondary' }]
    case 'rented': return [{ status: 'available', label: '改为可租', tone: 'primary' }]
    case 'offline': return [{ status: 'available', label: '改为可租', tone: 'primary' }, { status: 'rented', label: '改为已租', tone: 'secondary' }]
    default: return []
  }
})
const filteredProperties = computed(() => propertyFilter.value === 'all' ? properties.value : properties.value.filter(item => normalizedPropertyStatus(item.status) === propertyFilter.value))
const statusLabel = (status: string) => propertyStatuses.find(item => item.key === normalizedPropertyStatus(status))?.label || status
async function setPropertyStatus(item: Property, status: string) {
  try { await api('/v1/properties/status', { method: 'PATCH', body: JSON.stringify({ propertyId: item.id, status, note: `后台调整为${statusLabel(status)}` }) }); item.status = status }
  catch (error) { pageError.value = error instanceof Error ? error.message : '房源状态更新失败' }
}
const propertyToEdit = ref<Property | null>(null)
function continueMaterialProperty(item: Property) {
  propertyFilter.value = 'all'
  const exists = properties.value.some(property => property.id === item.id)
  if (exists) properties.value = properties.value.map(property => property.id === item.id ? item : property)
  else properties.value = [item, ...properties.value]
  propertyToEdit.value = item
  selectTab('房源列表')
}
function applyPropertyEdit(item: Property) {
  properties.value = properties.value.map(current => current.id === item.id ? { ...item, status: current.status } : current)
}
function removePropertyFromList(id: string) {
  properties.value = properties.value.filter(item => item.id !== id)
}
const propertyCover = (item: Property) => {
  const url = item.media?.find(media => media.type === 'image')?.url || ''
  if (!url || /^https?:\/\//.test(url)) return url
  return `${apiBase.replace(/\/api\/?$/, '')}${url.startsWith('/') ? url : `/${url}`}`
}

const configDirty = computed(() => Boolean(savedConfigJSON.value) && configChanges(JSON.parse(savedConfigJSON.value), publicConfig).length > 0)
watch(configDirty, dirty => { if (!dirty) restoredFrom.value = '' })
const configErrors = computed(() => {
  const errors: string[] = []
  if (!publicConfig.heroTitle.trim()) errors.push('品牌标题不能为空')
  for (const [key, presentation] of Object.entries(publicConfig.pagePresentations)) if (!presentation.title.trim()) errors.push(`${pagePresentationOptions.find(page=>page.key===key)?.label || key}页面标题不能为空`)
  if (publicConfig.contactPhone && !/^\+?\d{6,20}$/.test(publicConfig.contactPhone.trim())) errors.push('联系电话格式不正确')
  if (!publicConfig.relatedPropertyRadiusKm || publicConfig.relatedPropertyRadiusKm < 0.5 || publicConfig.relatedPropertyRadiusKm > 20) errors.push('附近房源关联距离需在 0.5 到 20 公里之间')
  if (!publicConfig.featuredChannels.length) errors.push('至少配置一个精选频道')
  if (publicConfig.featuredChannels.length > 12) errors.push('精选频道不能超过 12 个')
  if (areaEntryChannels.value.length < 3 || areaEntryChannels.value.length > 4) errors.push('请置顶 3 到 4 个精选频道作为地区入口')
  publicConfig.featuredChannels.forEach((item, index) => {
    const hasMapAnchor = Number(item.latitude) !== 0 && Number(item.longitude) !== 0
    if (!item.title.trim() || !item.subtitle.trim() || !item.keyword.trim() || (!item.community && !hasMapAnchor)) errors.push(`第 ${index + 1} 个精选频道需要填写名称、副标题，并选择门牌库小区或保留地图定位`)
    if (!item.radiusKm || item.radiusKm <= 0 || item.radiusKm > 100) errors.push(`第 ${index + 1} 个精选频道关联半径需在 0 到 100 公里之间`)
    if ((item.latitude === 0) !== (item.longitude === 0) || item.latitude < -90 || item.latitude > 90 || item.longitude < -180 || item.longitude > 180) errors.push(`第 ${index + 1} 个精选频道中心坐标不正确`)
  })
  const channelKeywords = publicConfig.featuredChannels.map((item) => item.keyword.trim()).filter(Boolean)
  if (new Set(channelKeywords).size !== channelKeywords.length) errors.push('精选频道搜索关键词不能重复')
  if (!publicConfig.shareCard.titleFields.length || publicConfig.shareCard.titleFields.length > 4) errors.push('分享标题需选择 1 到 4 个字段')
  if (invalidShareTitleFields.value.length) errors.push('分享标题含有未在客户标签中启用的字段')
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
  const item = availableFacilityCatalog.value.find(candidate => candidate.key === key)
  if (!item) return
  const existing = publicConfig.propertyDetail.facilities.find(facility => facility.key === key)
  if (existing) {
    existing.enabled = true
  } else {
    if (publicConfig.propertyDetail.facilities.length >= 24) return
    publicConfig.propertyDetail.facilities.push({ key: item.key, label: item.label, group: item.group, enabled: true })
  }
  facilityToAdd.value = ''
}
const shareTitlePreview = computed(() => {
  const examples: Record<string, string> = { community: '拿铁公寓', layout: '两室一厅', roomNo: '5栋3单元101室', rentMonthly: '¥2300/月', listingSource: '直租', rentType: '整租', depositPayment: '押一付一', areaSqm: '68㎡', floorInfo: '中层', availableDate: '拎包入住' }
  return [publicConfig.shareCard.titlePrefix, ...publicConfig.shareCard.titleFields.filter(shareFieldEnabledForCustomer).map(key => examples[key])].filter(Boolean).join(' · ')
})
const canvasShareTitle = computed(() => {
  const item = previewPrimaryProperty.value
  if (!item) return `${publicConfig.heroTitle || '小皮租房'}房源`
  const values: Record<string, string> = {
    community: item.community || '', layout: item.layout || '',
    roomNo: [item.buildingNo, item.unitNo, item.roomNo].filter(Boolean).join(''),
    rentMonthly: `¥${item.rentMonthly}/月`, listingSource: item.listingSource || '',
    rentType: item.rentType || '', depositPayment: item.depositPayment || '',
    areaSqm: item.areaSqm ? `${item.areaSqm}㎡` : '待补充',
    floorInfo: item.floorInfo || '待补充', availableDate: item.availableDate || '拎包入住',
  }
  return [publicConfig.shareCard.titlePrefix, ...publicConfig.shareCard.titleFields.filter(shareFieldEnabledForCustomer).map(key => values[key])].filter(Boolean).join(' · ') || item.community || '精选房源'
})
function toggleListingTitleField(key: string, enabled: boolean) {
  if (key === 'community') return
  if (!publicConfig.listingTitle.customerAllowedFields.includes(key)) return
  const fields = publicConfig.listingTitle.enabledFields
  if (enabled && !fields.includes(key)) fields.push(key)
  if (!enabled) publicConfig.listingTitle.enabledFields = fields.filter(item => item !== key)
}
function toggleCustomerDetailField(key: string, enabled: boolean) {
  if (key === 'community') return
  if (!publicConfig.listingTitle.customerAllowedFields.includes(key)) return
  const fields = publicConfig.listingTitle.customerDetailFields
  if (enabled && !fields.includes(key)) fields.push(key)
  if (!enabled) publicConfig.listingTitle.customerDetailFields = fields.filter(item => item !== key)
}
function toggleCustomerShareField(key: string, enabled: boolean) {
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
const customerTagPlacements = ['enabledFields', 'customerDetailFields', 'customerShareFields', 'customerPosterFields'] as const
const disabledCustomerTagPlacements = new Map<string, DisabledTagState>()

function toggleCustomerAllowedListingField(key: string, enabled: boolean) {
  if (key === 'community') return
  if (key === 'unit' && enabled && !window.confirm('开启“门牌标签”后，客户视角可将精确门牌号作为展示标签，可能暴露住址信息。确认开启吗？')) return
  const rules = publicConfig.listingTitle
  if (enabled) {
    const previous = disabledCustomerTagPlacements.get(key)
    if (previous) restoreDisabledTag(publicConfig, key, previous)
    else {
      if (!rules.customerAllowedFields.includes(key)) rules.customerAllowedFields.push(key)
      if (key !== 'unit' && !rules.enabledFields.includes(key)) rules.enabledFields.push(key)
    }
    disabledCustomerTagPlacements.delete(key)
    return
  }
  const previous = captureDisabledTag(publicConfig, disabledCustomerTagPlacements.values().next().value)
  disabledCustomerTagPlacements.set(key, previous)
  rules.customerAllowedFields = rules.customerAllowedFields.filter(item => item !== key)
  for (const placement of customerTagPlacements) {
    rules[placement] = rules[placement].filter(item => item !== key)
  }
  if (publicConfig.shareCard.titleFields.some(field => shareFieldListingMap[field] === key)) {
    const fields = publicConfig.shareCard.titleFields.filter(field => shareFieldListingMap[field] !== key)
    publicConfig.shareCard.titleFields = fields.length ? fields : ['community']
  }
  if (publicConfig.shareCard.posterFields.some(field => shareFieldListingMap[field] === key)) {
    publicConfig.shareCard.posterFields = publicConfig.shareCard.posterFields.filter(field => shareFieldListingMap[field] !== key)
    for (const fallback of ['rentMonthly', 'depositPayment', 'community', 'availableDate']) {
      if (publicConfig.shareCard.posterFields.length >= 3) break
      if (!publicConfig.shareCard.posterFields.includes(fallback)) publicConfig.shareCard.posterFields.push(fallback)
    }
  }
  previous.generatedTitle = publicConfig.shareCard.titleFields.filter(field => !previous.title.includes(field))
  previous.generatedPoster = publicConfig.shareCard.posterFields.filter(field => !previous.poster.includes(field))
  const states = [...disabledCustomerTagPlacements.values()]
  const titleFallbacks = [...new Set(states.flatMap(state => state.generatedTitle))]
  const posterFallbacks = [...new Set(states.flatMap(state => state.generatedPoster))]
  for (const state of states) { state.generatedTitle = titleFallbacks; state.generatedPoster = posterFallbacks }
}
function toggleAdminDoorplate(enabled: boolean) {
  publicConfig.listingTitle.adminEnabledFields = enabled ? ['unit'] : []
}

function addChannel() {
  if (publicConfig.featuredChannels.length >= 12) return
  publicConfig.featuredChannels.push({ id: createConfigId('channel'), icon: 'commute', title: '', subtitle: '', community: '', keyword: '', latitude: 0, longitude: 0, radiusKm: 3, showInAreaEntry: false })
  activeChannelIndex.value = publicConfig.featuredChannels.length - 1
}

function toggleChannelAreaEntry(item: ChannelConfig, enabled: boolean) {
  if (enabled && areaEntryChannels.value.length >= 4) {
    item.showInAreaEntry = false
    configMessage.value = '地区入口最多置顶 4 个精选频道。'
    return
  }
  item.showInAreaEntry = enabled
  publicConfig.locations = publicConfig.featuredChannels.filter(channel => channel.showInAreaEntry).map(channel => ({ id: channel.id, label: channel.title, keyword: channel.community || channel.keyword }))
}

function syncLocationFromChannel(_index?: number) {
  publicConfig.locations = publicConfig.featuredChannels.filter(item => item.showInAreaEntry).map(item => ({ id: item.id, label: item.title, keyword: item.community || item.keyword }))
}

function moveCoverageItem(index: number, offset: -1 | 1) {
  moveItem(publicConfig.featuredChannels, index, offset)
  publicConfig.locations = publicConfig.featuredChannels.filter(channel => channel.showInAreaEntry).map(channel => ({ id: channel.id, label: channel.title, keyword: channel.community || channel.keyword }))
  activeChannelIndex.value = Math.max(0, Math.min(activeChannelIndex.value, publicConfig.featuredChannels.length - 1))
}

function selectChannelCommunity(item: ChannelConfig, index: number, community: string) {
  const selected = community.trim()
  item.community = selected
  item.keyword = selected
  const location = savedCommunityLocations.value[selected]
  item.latitude = location?.latitude || 0
  item.longitude = location?.longitude || 0
  channelLocationResults.value = { ...channelLocationResults.value, [index]: [] }
  channelLocationRepairOpen.value = { ...channelLocationRepairOpen.value, [index]: false }
  channelMapPreview.value = { ...channelMapPreview.value, [index]: '' }
  channelSelectedLocation.value = {
    ...channelSelectedLocation.value,
    [index]: selected ? {
      id: `community:${selected}`, title: selected, address: location?.address || '尚未补充小区地址', district: '门牌库小区',
      latitude: item.latitude, longitude: item.longitude,
    } : undefined,
  }
  if (item.latitude && item.longitude) void loadChannelMapPreview(index, item.latitude, item.longitude)
}

function clearChannelLocation(item: ChannelConfig, index: number) {
  item.latitude = 0
  item.longitude = 0
  channelLocationResults.value = { ...channelLocationResults.value, [index]: [] }
  const community = item.community || ''
  channelSelectedLocation.value = { ...channelSelectedLocation.value, [index]: community ? { id: `community:${community}`, title: community, address: savedCommunityLocations.value[community]?.address || '尚未补充小区地址', district: '门牌库小区', latitude: 0, longitude: 0 } : undefined }
  channelMapPreview.value = { ...channelMapPreview.value, [index]: '' }
}

function invalidateChannelLocation(item: ChannelConfig, index: number) {
  if (item.latitude || item.longitude) clearChannelLocation(item, index)
}

async function searchChannelLocation(item: ChannelConfig, index: number) {
  const keyword = (item.community || item.keyword || item.title).trim()
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

async function openChannelLocationRepair(item: ChannelConfig, index: number) {
  channelLocationRepairOpen.value = { ...channelLocationRepairOpen.value, [index]: true }
  if (!channelLocationResults.value[index]?.length) await searchChannelLocation(item, index)
}

async function saveChannelCommunityLocation(item: ChannelConfig, index: number, location: LocationSearchItem) {
  const community = item.community.trim()
  if (!community || !location.address || !Number(location.latitude) || !Number(location.longitude)) {
    pageError.value = '地图结果缺少地址或坐标，无法补充小区位置'
    return
  }
  try {
    const saved = await api<Record<string, { address: string; latitude: number; longitude: number }>>(
      `/v1/admin/community-locations?community=${encodeURIComponent(community)}`,
      { method: 'PUT', body: JSON.stringify({ address: location.address, latitude: Number(location.latitude), longitude: Number(location.longitude) }) },
    )
    savedCommunityLocations.value = saved || { ...savedCommunityLocations.value, [community]: { address: location.address, latitude: Number(location.latitude), longitude: Number(location.longitude) } }
    item.latitude = Number(location.latitude.toFixed(7))
    item.longitude = Number(location.longitude.toFixed(7))
    channelLocationResults.value = { ...channelLocationResults.value, [index]: [] }
    channelLocationRepairOpen.value = { ...channelLocationRepairOpen.value, [index]: false }
    channelSelectedLocation.value = { ...channelSelectedLocation.value, [index]: location }
    channelMapPreview.value = { ...channelMapPreview.value, [index]: '' }
    void loadChannelMapPreview(index, item.latitude, item.longitude)
    configMessage.value = `已补充「${community}」的位置，地址和坐标已写入小区门牌数据。`
  } catch (error) {
    pageError.value = error instanceof Error ? error.message : '小区位置保存失败'
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
    if (item.latitude && item.longitude) void loadChannelMapPreview(index, item.latitude, item.longitude)
  })
}

function removeChannel(index: number) {
  publicConfig.featuredChannels.splice(index, 1)
  publicConfig.locations = publicConfig.featuredChannels.filter(item => item.showInAreaEntry).map(item => ({ id: item.id, label: item.title, keyword: item.community || item.keyword }))
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
  restoredFrom.value = ''
  configMessage.value = '已恢复为最近一次发布的配置。'
  configFailed.value = false
}

const pendingConfigChanges = computed(() => savedConfigJSON.value ? configChanges(JSON.parse(savedConfigJSON.value), publicConfig) : [])
const selectedHistoryChanges = computed(() => configHistorySelected.value ? configChanges(configHistorySelected.value.before, configHistorySelected.value.after) : [])
const conflictingChanges = computed(() => {
  if (!configConflict.value || !savedConfigJSON.value) return []
  const baseline = JSON.parse(savedConfigJSON.value)
  const remote = configChanges(baseline, configConflict.value.config)
  return pendingConfigChanges.value.filter(local => remote.some(other => local.paths.some(p => other.paths.some(q => p[0] === q[0] && (p[0] !== 'pagePresentations' || (p[1] === q[1] && p.slice(2).join('/') === q.slice(2).join('/')))))))
})
function cancelConfigChange(change: ConfigChange) {
  restoreChange(publicConfig, JSON.parse(savedConfigJSON.value), change)
  confirmationError.value = ''
}
function defaultPageCopy(field: string) {
  const presentation = publicConfig.pagePresentations[configPage.value]
  delete presentation.copy?.[field]
  if (presentation.copy && !Object.keys(presentation.copy).length) delete presentation.copy
}
function inspectConfigChange(change: ConfigChange) {
  configConfirmOpen.value = false
  configHistoryOpen.value = false
  const path = change.paths[0]
  if (change.roles.startsWith('仅管理员') || change.roles === '管理员') switchCanvasIdentity('admin')
  else if (change.roles.startsWith('仅游客') || change.roles === '游客') switchCanvasIdentity('guest')
  else if (canvasIdentityState.value === 'admin' && !change.roles.includes('管理员') && !change.roles.includes('全角色')) switchCanvasIdentity('user')
  if (change.page === 'global') {
    if (path[0] === 'locations' || path[0] === 'featuredChannels' || path[0] === 'relatedPropertyRadiusKm') {selectTab('小程序页面配置');selectConfigPage('home');locatePageEditor('coverage')}
    else if (path[0] === 'shareCard') { selectTab('小程序页面配置'); selectConfigPage('propertyDetail'); selectDetailConfigTab('share') }
    else if (path[0] === 'propertyDetail' && path[1] !== 'facilities') {selectTab('小程序页面配置');selectConfigPage('propertyDetail');locatePageEditor('detail-modules')}
    else if (path[0] === 'statusLabels') {selectTab('小程序页面配置');selectConfigPage('properties');locatePageEditor('status-labels')}
    else {selectTab('小程序基础配置');selectBasicConfigTab(path[0] === 'propertyDetail' ? 'facilities' : path[0] === 'heroTitle' || path[0] === 'contactPhone' ? 'brand' : 'tags');selectConfigPage(path[0] === 'propertyDetail' ? 'propertyDetail' : 'home')}
  } else {
    selectTab('小程序页面配置');selectConfigPage(change.page)
    if (path[2] === 'copy') {
      const group = pageCopyGroups[change.page]?.find(g => g.fields.includes(path[3]))
      if (group) locatePageEditor(group.key)
      void nextTick(() => previewPageCopyField(path[3]))
    } else locatePageEditor('page-copy')
  }
}
function saveConfig() { confirmationError.value = ''; publishedReceipt.value = null; configConfirmOpen.value = true }
async function confirmPublishConfig() {
  if (savingConfig.value || configErrors.value.length || !pendingConfigChanges.value.length || configConflict.value) return
  savingConfig.value = true; confirmationError.value = ''; configFailed.value = false
  publicConfig.locations = publicConfig.featuredChannels.filter(item => item.showInAreaEntry).map(item => ({ id: item.id, label: item.title, keyword: item.community || item.keyword }))
  const snapshot = JSON.parse(JSON.stringify(publicConfig)) as PublicConfig
  let receipt: ConfigPublication | undefined
  try {
    const entry = await api<ConfigPublication>('/v1/admin/config/publish', { method: 'POST', body: JSON.stringify({ config: snapshot, expectedRevision: configRevision.value, restoredFrom: restoredFrom.value }) })
    // The receipt is already durable. Mark it saved even if the subsequent
    // read fails, so retrying a read cannot duplicate a successful publish.
    receipt = entry
    publishedReceipt.value = entry
    applyConfig(entry.after, true); configRevision.value = entry.revision; restoredFrom.value = ''
    configLastSavedAt.value = formatTime(entry.createdAt)
    const state = await api<ConfigState>('/v1/admin/config/state')
    if (state.revision !== entry.revision) {
      configConflict.value = state
      configMessage.value = `发布 ${entry.id} 已写入，当前线上已有更新版本。`
      return
    }
    applyConfig(state.config, true); configRevision.value = state.revision
    configConfirmOpen.value = false; configMessage.value = `配置已发布并回读确认 · ${entry.id}。小程序重新进入对应页面后获取最新配置。`
  } catch (error) {
    if (!receipt && (error as {status?:number}).status === 409) {
      try { configConflict.value = await api<ConfigState>('/v1/admin/config/state') } catch { /* Preserve the original error and the draft. */ }
    }
    confirmationError.value = receipt ? `发布 ${receipt.id} 已写入，回读未完成，请重新加载配置和历史核对。` : error instanceof Error ? error.message : '发布失败'
    configMessage.value = confirmationError.value; configFailed.value = !receipt
  } finally { savingConfig.value = false }
}
function useLatestConfig() {
  if (!configConflict.value) return
  const state = configConflict.value
  // Preserve nonconflicting draft changes; conflicting fields use the server
  // values. The new diff must still be confirmed before another publish.
  const draft = JSON.parse(JSON.stringify(publicConfig)) as PublicConfig
  const local = pendingConfigChanges.value.filter(change => !conflictingChanges.value.includes(change))
  applyConfig(state.config, true); configRevision.value = state.revision
  for (const change of local) restoreChange(publicConfig, draft, change)
  configConflict.value = null
  if (publishedReceipt.value) {
    confirmationError.value = ''
    configConfirmOpen.value = false
    configMessage.value = `发布 ${publishedReceipt.value.id} 已写入，已加载当前线上版本。`
  } else {
    confirmationError.value = '已采用线上冲突项，保留其他本地修改；请重新核对清单。'
  }
}
async function openConfigHistory(more = false) {
  configHistoryOpen.value = true; configHistoryLoading.value = true; configHistoryError.value = ''
  try {
    const cursor = more ? configHistory.value[configHistory.value.length - 1]?.id : ''
    const entries = await api<ConfigPublication[]>(`/v1/admin/config/history${cursor ? `?before=${encodeURIComponent(cursor)}` : ''}`)
    configHistory.value = more ? [...configHistory.value, ...entries] : entries
    configHistoryHasMore.value = entries.length === 50
    if (!more) configHistorySelected.value = entries[0] || null
  } catch (error) { configHistoryError.value = error instanceof Error ? error.message : '读取历史失败' }
  finally { configHistoryLoading.value = false }
}
function restoreHistory(change?: ConfigChange, side: 'before' | 'after' = 'after') {
  const entry = configHistorySelected.value
  if (!entry || entry.status === 'failed') return
  if (configDirty.value && !window.confirm('当前有未发布修改。历史恢复可能覆盖对应字段，是否继续？')) return
  if (change) restoreChange(publicConfig, entry[side], change)
  else applyConfig(entry[side])
  restoredFrom.value = configDirty.value ? entry.id : ''; configHistoryOpen.value = false; saveConfig()
}
function protectConfigDraft(event: BeforeUnloadEvent) {
  if (!configDirty.value) return
  event.preventDefault(); event.returnValue = ''
}
const unregisterConfigGuard = router.beforeEach((to, from) => {
  if (!configDirty.value || !String(from.params.tab).startsWith('miniapp-') || String(to.params.tab).startsWith('miniapp-')) return true
  if (window.confirm('有未发布的小程序配置修改，离开后不会发布。是否继续离开？')) return true
  tab.value = tabFromRoute(from.params.tab) as TabKey
  return false
})

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
    window.addEventListener('resize', updateCanvasFit)
    window.addEventListener('beforeunload', protectConfigDraft)
    updateCanvasFit()
  })
})
watch(canvasPreviewStage, stage => {
  canvasFitObserver?.disconnect()
  if (stage) {
    canvasFitObserver ??= new ResizeObserver(updateCanvasFit)
    canvasFitObserver.observe(stage)
  }
  updateCanvasFit()
}, { flush: 'post' })
onBeforeUnmount(() => {
  passwordMessage?.close()
  errorMessage?.close()
  canvasFitObserver?.disconnect()
  window.removeEventListener('resize', updateCanvasFit)
  window.removeEventListener('beforeunload', protectConfigDraft)
  unregisterConfigGuard()
})

function copyFieldsForRole(page: string, role: string, group: {key:string;label:string;fields:string[]}) {
    let fields = group.fields
    if (page === 'me') {
      if (['游客身份卡', '登录引导'].includes(group.label) && role !== 'guest') return []
      if ((group.label === '运营入口') !== (role === 'admin')) return []
      if (role === 'user') fields = fields.filter(field => !['无需登录', '登录后查看', '提交时登录'].includes(field))
    }
    if (page === 'properties') {
      if (group.key === 'favorites-empty' && role !== 'user') return []
      if (group.key === 'display-guidance' && role === 'admin') return []
      if (group.key === 'properties-empty') fields = fields.filter(field => operatorEmptyCopy.has(field) === (role === 'admin'))
    }
    if (page === 'home' && role === 'admin') fields = fields.filter(field => field !== '说说找房需求')
    if (page === 'footprints') {
      fields = role === 'admin' ? [] : fields.filter(field => (field === '还没有浏览记录') === (role === 'user'))
    }
    if (page === 'requestHistory' && (role === 'admin' || (group.label === '未登录') !== (role === 'guest'))) return []
    if (page === 'requests' && role !== 'guest') fields = fields.filter(field => field !== '登录后才能保存并跟进你的找房需求')
    return fields
}
for (const [page, groups] of Object.entries(pageCopyGroups)) for (const group of groups) for (const field of group.fields) {
  registerCopyScope(page,field,['guest','user','admin'].filter(role=>{const audience=pagePresentationOptions.find(item=>item.key===page)?.audience;return !(audience==='operator'&&role!=='admin') && !(audience==='customer'&&role==='admin') && copyFieldsForRole(page,role,group).includes(field)}).map(role=>({guest:'游客',user:'微信用户',admin:'管理员'}[role]!)))
}
const currentPageCopyGroups = computed(() => (pageCopyGroups[configPage.value] || []).flatMap(group => {
  const fields = copyFieldsForRole(configPage.value,canvasIdentityState.value,group)
  return fields.length ? [{...group,fields}] : []
}))

const activePageCopyGroup = computed(() => currentPageCopyGroups.value.find(group => group.key === pageEditorSection.value))
function canvasPageCopy(fallback: string, key = fallback) {
  return publicConfig.pagePresentations[configPage.value]?.copy?.[key] || fallback
}
function setPageCopy(field: string, value: string) {
  const presentation = publicConfig.pagePresentations[configPage.value]
  const copy = { ...presentation.copy }
  if (value.trim()) copy[field] = value
  else delete copy[field]
  if (Object.keys(copy).length) presentation.copy = copy
  else delete presentation.copy
  if ((configPage.value === 'me' && pageEditorSection.value === 'notes-2') || (configPage.value === 'requests' && field === '登录后才能保存并跟进你的找房需求')) previewPageCopyField(field)
}
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

  <AdminLayout v-else :active="isMiniappConfig ? '小程序页面配置' : tab" :title="isMiniappConfig ? '小程序配置' : tab" :api-label="apiLabel" @select="selectTab" @logout="logout">

        <DashboardPage v-if="tab === '运营总览'" :state="overviewState" :error-message="overviewError" :property-count="properties.length" :pending-lead-count="leads.filter(isPendingLead).length" :new-request-count="rentalRequests.filter(isNewRentalRequest).length" :favorite-count="favoriteContacts.length" :user-count="users.length" @navigate="openOverviewMetric" @refresh="refreshOverview" />

        <WechatUsersPage v-else-if="tab === '微信用户'" :users="users" :loading="loadingSections.includes('微信用户')" @refresh="load" />

        <RentalRequestsPage v-else-if="tab === '租客需求'" :api="api" @saved="item => rentalRequests = rentalRequests.map(row => row.id === item.id ? item : row)" :requests="rentalRequests" :loading="loadingSections.includes('租客需求')" :filter="requestFilter" @refresh="load" @filter="value => router.replace({ query: value === 'new' ? { filter: value } : {} })" />

        <IntentionsPage v-else-if="tab === '用户意向'" :contacts="favoriteContacts" :api="api" :loading="loadingSections.includes('用户意向')" @refresh="load" />
        <SubscriptionJobsPage v-else-if="tab === '通知任务'" :api="api" />

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

        <PropertiesPage :requested-edit="propertyToEdit" @edit-opened="propertyToEdit = null" :channels="publicConfig.featuredChannels" :api="api" :api-base="apiBase" v-else-if="tab === '房源列表'" :properties="properties" :filtered-properties="filteredProperties" :filter="propertyFilter" :statuses="propertyStatuses" :loading="loadingSections.includes('房源')" :normalized-status="normalizedPropertyStatus" :status-label="statusLabel" :cover="propertyCover" :nearby-radius="Number(publicConfig.propertyDetail.nearby.radiusKm || 3)" :nearby-limit="Number(publicConfig.propertyDetail.nearby.limit || 4)" @refresh="load" @filter="propertyFilter = $event" @status="setPropertyStatus" @saved="applyPropertyEdit" @deleted="removePropertyFromList" />
        <MaterialLibraryPage v-else-if="tab === '房源素材库'" @continue-edit="continueMaterialProperty" :api="api" :api-base="apiBase" />

        <template v-else-if="isMiniappConfig">
          <div class="miniapp-config-shell" :style="{ '--canvas-target-width': canvasTargetWidth + 'px' }">
          <div class="miniapp-config-main">
          <section class="page-header">
            <div><span>内容与运营</span><h1>小程序配置</h1><p>维护全局设置和页面内容，在右侧画布查看效果。</p></div>
          </section>
          <section class="surface config-workspace-toolbar">
          <nav class="basic-config-tabs config-mode-tabs" aria-label="小程序配置编辑入口">
            <button type="button" :class="{ active: tab === '小程序基础配置' }" :aria-pressed="tab === '小程序基础配置'" @click="selectTab('小程序基础配置')">全局设置</button>
            <button type="button" :class="{ active: tab === '小程序页面配置' }" :aria-pressed="tab === '小程序页面配置'" @click="selectTab('小程序页面配置')">按页面编辑</button>
            <label class="canvas-preview-control">预览角色<select v-model="canvasIdentityState" @change="switchCanvasIdentity(canvasIdentityState)"><option value="guest">游客</option><option value="user">微信用户</option><option value="admin">小程序管理员</option></select></label>
          </nav>
            <div class="publish-state" :title="configLastSavedAt ? `最近发布：${configLastSavedAt}` : undefined">
              <i :class="{ dirty: configDirty }" />
              <div><strong>{{ configDirty ? '有未发布修改' : '已加载配置' }}</strong></div>
            </div>
            <div class="config-toolbar-actions"><el-button @click="openConfigHistory()">发布历史</el-button><el-button :icon="RefreshLeft" :disabled="!configDirty || savingConfig" @click="resetConfig">恢复</el-button><el-button type="primary" :loading="savingConfig" :disabled="!configDirty || configErrors.length > 0" @click="saveConfig">保存并发布</el-button></div>
          </section>
          <el-dialog v-model="configConfirmOpen" title="发布改动确认" width="min(960px, 92vw)" class="config-publish-dialog" :close-on-click-modal="false" :show-close="!savingConfig" :close-on-press-escape="!savingConfig" append-to-body>
            <p class="config-review-summary">共 {{ pendingConfigChanges.length }} 项改动。影响范围由配置字段确定，切换预览角色不会改变发布范围。关联项将一起撤销。</p>
            <el-alert v-if="confirmationError" :title="confirmationError" type="warning" :closable="false" />
            <section v-if="configConflict" class="config-conflict-panel">
              <strong v-if="publishedReceipt">发布 {{ publishedReceipt.id }} 已写入</strong>
              <strong v-else>线上版本已更新，本次发布已阻止</strong>
              <template v-if="publishedReceipt">
                <p>本次发布版本 {{ publishedReceipt.revision }}，当前线上版本 {{ configConflict.revision }}。</p>
                <el-button @click="useLatestConfig">加载当前线上版本</el-button>
              </template>
              <template v-else>
              <p v-if="conflictingChanges.length">冲突项：{{ conflictingChanges.map(item => item.label).join('；') }}</p><p v-else>线上修改了其他字段，可以保留本地修改并更新发布基线。</p>
              <article v-for="change in conflictingChanges" :key="change.id"><b>{{ change.label }}</b><div class="config-change-values"><div><small>本地修改</small><pre>{{ changeValue(change.after) }}</pre></div><div><small>线上当前值</small><pre>{{ changeValue(configChanges(JSON.parse(savedConfigJSON), configConflict.config).find(item => item.id === change.id)?.after) }}</pre></div></div></article>
              <el-button @click="useLatestConfig">采用线上冲突项并保留其他修改</el-button>
              </template>
            </section>
            <div class="config-review-list">
              <article v-for="change in pendingConfigChanges" :key="change.id" class="config-change-card">
                <header><div><strong>{{ change.label }}</strong><span>{{ change.roles }}</span></div><div><el-button size="small" :disabled="savingConfig" @click="inspectConfigChange(change)">查看效果</el-button><el-button size="small" :disabled="savingConfig" @click="cancelConfigChange(change)">撤销此项</el-button></div></header>
                <div class="config-change-values"><div><small>修改前</small><pre>{{ changeValue(change.before) }}</pre></div><div><small>修改后</small><pre>{{ changeValue(change.after) }}</pre></div></div>
              </article>
              <el-empty v-if="!pendingConfigChanges.length" description="没有待发布修改" :image-size="60" />
            </div>
            <template #footer><el-button :disabled="savingConfig" @click="configConfirmOpen = false">返回编辑</el-button><el-button type="primary" :loading="savingConfig" :disabled="!pendingConfigChanges.length || configErrors.length > 0 || !!configConflict" @click="confirmPublishConfig">确认发布 {{ pendingConfigChanges.length }} 项</el-button></template>
          </el-dialog>
          <el-dialog v-model="configHistoryOpen" title="配置发布历史" width="min(1080px, 94vw)" class="config-publish-dialog" append-to-body>
            <el-alert v-if="configHistoryError" :title="configHistoryError" type="error" :closable="false" />
            <div v-loading="configHistoryLoading" class="config-history-layout">
              <aside><button v-for="entry in configHistory" :key="entry.id" type="button" :class="{ active: configHistorySelected?.id === entry.id }" @click="configHistorySelected = entry"><strong>{{ formatTime(entry.createdAt) }}</strong><span>{{ entry.operator }}</span><small>{{ entry.status === 'failed' ? '失败 · 未生效' : '成功发布' }} · {{ entry.id }}{{ entry.restoredFrom ? ' · 历史恢复发布' : '' }}</small></button><el-button v-if="configHistoryHasMore" @click="openConfigHistory(true)">加载更早记录</el-button><el-empty v-if="!configHistory.length" description="暂无成功发布记录" :image-size="50" /></aside>
              <section v-if="configHistorySelected"><div class="config-history-heading"><strong>{{ selectedHistoryChanges.length }} 项改动</strong><el-button :disabled="configHistorySelected.status === 'failed'" @click="restoreHistory()">恢复该版本为待发布修改</el-button></div><el-alert v-if="configHistorySelected.status === 'failed'" :title="configHistorySelected.error || '失败记录，未生效，不可恢复为已发布版本'" type="warning" :closable="false" /><p v-if="configHistorySelected.restoredFrom">恢复来源：{{ configHistorySelected.restoredFrom }}</p>
                <article v-for="change in selectedHistoryChanges" :key="change.id" class="config-change-card"><header><div><strong>{{ change.label }}</strong><span>{{ change.roles }}</span></div><div><el-button size="small" :disabled="configHistorySelected.status === 'failed'" @click="restoreHistory(change,'before')">恢复修改前</el-button><el-button size="small" :disabled="configHistorySelected.status === 'failed'" @click="restoreHistory(change)">恢复修改后</el-button></div></header><div class="config-change-values"><div><small>修改前</small><pre>{{ changeValue(change.before) }}</pre></div><div><small>修改后</small><pre>{{ changeValue(change.after) }}</pre></div></div></article>
              </section>
            </div>
          </el-dialog>
          <el-alert v-if="configMessage" class="config-alert" :title="configMessage" :type="configFailed ? 'error' : 'success'" show-icon :closable="false" />
          <section v-if="configErrors.length" class="config-validation">
            <strong>发布前需要处理 {{ configErrors.length }} 项</strong>
            <ul><li v-for="error in configErrors" :key="error">{{ error }}</li></ul>
            <el-button v-if="invalidShareTitleFields.length" @click="removeInvalidShareTitleFields">从分享标题移除 {{ invalidShareTitleFields.map(shareFieldLabel).join('、') }}</el-button>
          </section>
          <template v-if="tab === '小程序基础配置'">
          <nav class="basic-config-tabs global-config-sections config-workspace-tabs" aria-label="全局设置分类">
            <button v-for="section in basicConfigSections" :key="section.key" type="button"
              :class="{ active: basicConfigTab === section.key }" :aria-pressed="basicConfigTab === section.key"
              @click="selectBasicConfigTab(section.key)">{{ section.label }}</button>
          </nav>
          <div :key="basicConfigTab" class="basic-settings-editor">
          <section v-if="basicConfigTab === 'brand'" class="surface form-section basic-config-panel">
            <div class="section-heading"><div><h2>品牌与联系</h2><p>小程序各页面共用。</p></div></div>
            <el-form data-config-field="brand-name" label-position="top" class="basic-config-form"><div class="form-row two-columns"><el-form-item label="品牌名称" required><el-input v-model="publicConfig.heroTitle" maxlength="20" show-word-limit placeholder="例如：小皮租房" /><div class="form-tip">统一用于小程序顶部导航、微信登录、房源详情服务入口与分享落款。</div></el-form-item><el-form-item label="服务联系电话"><el-input v-model.trim="publicConfig.contactPhone" maxlength="20" placeholder="用于房源详情联系" /><div class="form-tip">统一用于房源详情“打电话”和找房需求页的电话入口；不影响在线客服。</div></el-form-item></div></el-form>
          </section>
          <section v-else-if="basicConfigTab === 'tags'" data-config-field="tags" class="surface form-section basic-config-panel">
            <div class="section-heading"><div><h2>房源标签规则</h2><p>管理公开标签范围与管理员门牌。卡片排版和分享标签在对应页面编辑。</p></div></div>
            <nav class="basic-config-tabs listing-tag-tabs" aria-label="房源标签设置分类">
              <button v-for="item in visibleListingTagTabs" :key="item.key" type="button"
                :class="{ active: activeListingTagTab === item.key }" :aria-pressed="activeListingTagTab === item.key"
                @click="selectListingTagTab(item.key)">{{ item.label }}</button>
            </nav>
                        <div v-if="activeListingTagTab === 'allowed'" class="share-field-editor listing-title-editor">
              <header><strong>客户可用标签</strong><span>重新勾选会恢复本次编辑中取消前的展示位置。首次开放的普通标签默认显示在列表；管理员门牌可在「管理员标签」中设置。</span></header>
              <div class="share-field-options"><button v-for="field in listingTitleFieldOptions.filter(item => item.key !== 'community')" :key="`basic-customer-allowed-${field.key}`" type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.customerAllowedFields.includes(field.key) }" :aria-pressed="publicConfig.listingTitle.customerAllowedFields.includes(field.key)" @click="toggleCustomerAllowedListingField(field.key, !publicConfig.listingTitle.customerAllowedFields.includes(field.key))"><i>{{ publicConfig.listingTitle.customerAllowedFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>
            </div>
            <div v-if="activeListingTagTab === 'admin'" class="share-field-editor listing-title-editor">
              <header><strong>管理员额外标签</strong><span>管理员与客户复用公开标签；只可额外显示精确门牌，列表和详情同步生效。</span></header>
              <div class="share-field-options"><button type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.adminEnabledFields.includes('unit') }" :aria-pressed="publicConfig.listingTitle.adminEnabledFields.includes('unit')" @click="toggleAdminDoorplate(!publicConfig.listingTitle.adminEnabledFields.includes('unit'))"><i>{{ publicConfig.listingTitle.adminEnabledFields.includes('unit') ? '✓' : '' }}</i><span>额外显示门牌</span></button></div>
            </div>
          </section>
          <section v-else data-config-field="facilities" class="surface form-section basic-config-panel facility-library-panel">
            <div class="section-heading"><div><h2>房屋设施库</h2><p>点击整张卡片启用或停用。启用项会进入管理员录入页的设施勾选，并可在房源详情展示；停用不删除历史房源数据。</p></div></div>
            <div class="facility-add-row"><el-button @click="openFacilityPreview(false)">预览详情设施</el-button><el-button v-if="canvasIdentityState === 'admin'" @click="openFacilityPreview(true)">预览编辑配套</el-button><span>详情预览使用当前设施库的示例配套。</span></div>
            <div class="facility-library-summary"><b>已启用 {{ publicConfig.propertyDetail.facilities.filter(item => item.enabled).length }} / {{ publicConfig.propertyDetail.facilities.length }} 项</b></div>
            <div class="facility-config-list facility-library-list">
              <article v-for="facility in publicConfig.propertyDetail.facilities" :key="facility.key" class="facility-choice" :class="{ checked: facility.enabled }" @click="toggleFacility(facility)">
                <button type="button" class="facility-check" :aria-pressed="facility.enabled" @click.stop="toggleFacility(facility)">{{ facility.enabled ? '✓' : '' }}</button>
                <i class="facility-option-icon"><img :src="detailFacilityIcon(facility.key)" alt=""></i>
                <span><strong>{{ facility.label }}</strong><small>{{ facility.group }} · <code>{{ facility.key }}</code></small></span>
                <el-popconfirm title="移除后可从预置列表重新添加，确认移除？" @confirm="removeDetailFacility(facility.key)"><template #reference><button type="button" class="facility-edit facility-remove" @click.stop>移除</button></template></el-popconfirm>
              </article>
            </div>
            <div class="facility-add-row">
              <span>添加预置设施</span>
              <el-select v-model="facilityToAdd" filterable clearable aria-label="搜索预置设施"
                :placeholder="availableFacilityCatalog.length ? '搜索名称或分类' : '预置设施均已启用'"
                :disabled="!availableFacilityCatalog.length">
                <el-option v-for="facility in availableFacilityCatalog" :key="facility.key" :value="facility.key"
                  :label="`${facility.label} · ${facility.group} · ${facility.key}${publicConfig.propertyDetail.facilities.some(item => item.key === facility.key) ? '（已停用）' : ''}`"
                  :disabled="publicConfig.propertyDetail.facilities.length >= 24 && !publicConfig.propertyDetail.facilities.some(item => item.key === facility.key)">
                  <img class="facility-preset-icon" :src="detailFacilityIcon(facility.key)" alt="">
                  <span>{{ facility.label }} · {{ facility.group }} · <code>{{ facility.key }}</code></span>
                </el-option>
              </el-select>
              <el-button type="primary" plain :disabled="!facilityToAdd" @click="addDetailFacility(facilityToAdd)">添加或启用</el-button>
            </div>
          </section>
          </div>
          </template>
          <template v-else>
          <div v-loading="loadingSections.includes('小程序配置')" class="config-workspace">
            <div class="config-editor">
                <nav class="basic-config-tabs page-editor-pages config-workspace-tabs" aria-label="当前编辑页面">
                  <button v-for="item in pagePresentationOptions.filter(item => item.audience === 'both' || item.audience === configAudience || item.key === configPage)"
                    :key="item.key" type="button" :class="{ active: configPage === item.key }" :aria-pressed="configPage === item.key"
                    @click="selectConfigPage(item.key)">{{ item.label }}</button>
                </nav>
              <div class="surface page-editor-panel" role="region" aria-label="所选页面的编辑内容">
                <nav class="basic-config-tabs page-editor-index config-workspace-tabs" aria-label="当前页面编辑区">
                <button type="button" :aria-pressed="pageEditorSection === 'page-copy'" :class="{ active: pageEditorSection === 'page-copy' }" @click="locatePageEditor('page-copy')">{{ configPage === 'home' ? '首屏与搜索' : '文案与背景' }}</button>
                <button v-if="configPage === 'home'" type="button" :aria-pressed="pageEditorSection === 'coverage'" :class="{ active: pageEditorSection === 'coverage' }" @click="locatePageEditor('coverage')">地区与频道</button>
                <button v-if="configPage === 'properties'" type="button" :aria-pressed="pageEditorSection === 'status-labels'" :class="{ active: pageEditorSection === 'status-labels' }" @click="locatePageEditor('status-labels')">状态名称</button>
                <button v-if="['home', 'properties'].includes(configPage)" type="button" :aria-pressed="pageEditorSection === 'page-card-tags'" :class="{ active: pageEditorSection === 'page-card-tags' }" @click="locatePageEditor('page-card-tags')">{{ configPage === 'home' ? '推荐房源' : '列表卡片标签' }}</button>
                <button v-if="configPage === 'propertyDetail'" type="button"
                  :class="{ active: pageEditorSection === 'detail-modules' }" :aria-pressed="pageEditorSection === 'detail-modules'"
                  @click="selectDetailSectionTab(detailSectionTab)">详情模块</button>
                <button v-if="configPage === 'propertyDetail'" type="button"
                  :class="{ active: pageEditorSection === 'detail-sharing' }" :aria-pressed="pageEditorSection === 'detail-sharing'"
                  @click="selectDetailConfigTab('share')">微信分享</button>
                <button v-for="group in currentPageCopyGroups" :key="group.key" type="button"
                  :class="{ active: pageEditorSection === group.key }" :aria-pressed="pageEditorSection === group.key"
                  @click="locatePageEditor(group.key)">{{ group.label }}</button>
                <label v-if="['home', 'properties', 'me'].includes(configPage)" class="home-copy-mode home-editor-pick"><input v-model="homeCopyEditing" type="checkbox"> 点选文案</label>
              </nav>
              <section v-if="configPage === 'propertyDetail' && pageEditorSection === 'detail-sharing'" class="surface form-section config-section">
<header class="detail-subheading"><strong>微信分享</strong><span>分别选择好友和朋友圈分享可显示的标签。</span><el-button text @click="selectConfigPage('propertyDetail'); previewOverlay = 'share'">预览分享卡</el-button></header><div class="listing-tag-preview-switch"><el-button size="small" :type="sharePreviewType === 'friend' ? 'primary' : 'default'" @click="sharePreviewType = 'friend'; selectConfigPage('propertyDetail'); previewOverlay = 'share'">好友分享</el-button><el-button size="small" :type="sharePreviewType === 'timeline' ? 'primary' : 'default'" @click="sharePreviewType = 'timeline'; selectConfigPage('propertyDetail'); previewOverlay = 'share'">朋友圈分享</el-button></div>
              <div class="share-field-options"><button v-for="field in listingTitleFieldOptions.filter(item => item.key !== 'community' && publicConfig.listingTitle.customerAllowedFields.includes(item.key))" :key="`basic-customer-share-${field.key}`" type="button" class="field-check" :class="{ checked: (sharePreviewType === 'friend' ? publicConfig.listingTitle.customerShareFields : publicConfig.listingTitle.customerPosterFields).includes(field.key) }" :aria-pressed="(sharePreviewType === 'friend' ? publicConfig.listingTitle.customerShareFields : publicConfig.listingTitle.customerPosterFields).includes(field.key)" @click="(sharePreviewType === 'friend' ? toggleCustomerShareField : toggleCustomerPosterField)(field.key, !(sharePreviewType === 'friend' ? publicConfig.listingTitle.customerShareFields : publicConfig.listingTitle.customerPosterFields).includes(field.key))"><i>{{ (sharePreviewType === 'friend' ? publicConfig.listingTitle.customerShareFields : publicConfig.listingTitle.customerPosterFields).includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>
              </section>
              <section v-if="configPage === 'home' && tab === '小程序页面配置'" v-show="pageEditorSection === 'page-copy'" data-config-field="page-copy" class="surface form-section config-section">
                <div class="section-heading"><div><h2>首屏与搜索</h2></div></div>
                <el-form label-position="top" class="basic-config-form">
                  <el-form-item label="首页主标题" required><div data-config-field="home-title"><el-input v-model="publicConfig.pagePresentations.home.title" maxlength="30" show-word-limit placeholder="例如：今天想住哪里？" /></div></el-form-item>
                  <el-form-item label="搜索提示"><div data-config-field="home-search"><el-input v-model="publicConfig.pagePresentations.home.subtitle" maxlength="60" show-word-limit placeholder="例如：搜索小区、地铁或商圈" /></div></el-form-item>
                  <el-form-item label="首页背景图"><el-input v-model="publicConfig.pagePresentations.home.backgroundImage" placeholder="上传后自动填入，也可填写图片地址" /><div class="presentation-actions"><label class="upload-control"><input type="file" accept="image/jpeg,image/png,image/webp" @change="uploadPresentationBackground('home', $event)" />上传背景图</label><button v-if="publicConfig.pagePresentations.home.backgroundImage" @click="publicConfig.pagePresentations.home.backgroundImage = ''">移除背景图</button></div></el-form-item>
                </el-form>
              </section>



              <section v-if="activePageCopyGroup" class="surface form-section config-section">
                <div class="section-heading"><div><h2>{{ activePageCopyGroup.label }}</h2><small>影响范围按字段确定，与当前预览角色无关。清空文案会恢复原文。</small></div></div>
                <el-form label-position="top" class="basic-config-form">
                  <el-form-item v-for="field in activePageCopyGroup.fields" :key="field" :label="field"><div v-if="field.includes('{area}') || field.includes('{keyword}')" class="form-tip">保留花括号占位符，预览时会替换为地区或关键词。</div>
                    <div class="config-field-scope"><span>{{ changeScope(['pagePresentations',configPage,'copy',field]) }}</span><button type="button" @click="defaultPageCopy(field)">恢复默认</button></div><el-input :model-value="canvasPageCopy(field)" @focus="previewPageCopyField(field)" @update:model-value="setPageCopy(field, $event)" maxlength="120" show-word-limit />
                  </el-form-item>
                </el-form>
              </section>
              <section v-if="configPage !== 'home'" v-show="pageEditorSection === 'page-copy'" data-config-field="page-copy" class="surface form-section config-section selected-page-config">
                <div class="section-heading config-section-heading"><div><h2>{{ pagePresentationOptions.find(page => page.key === configPage)?.label }} · 文案与背景</h2><p>编辑当前页面的展示内容。</p></div></div>
                <el-form label-position="top" class="basic-config-form">
                  <el-form-item v-if="configPage === 'me'" label="个人中心栏目名"><div data-config-field="bottom-eyebrow"><el-input v-model="publicConfig.pagePresentations.me.eyebrow" placeholder="个人中心" /></div></el-form-item>
                  <el-form-item  :label="configPage === 'propertyDetail' ? '滚动导航标题' : '页面标题'"><div data-config-field="bottom-title"><el-input v-if="configPage === 'properties' && configAudience === 'operator'" v-model="publicConfig.pagePresentations.properties.operatorTitle" placeholder="房源管理" maxlength="30" show-word-limit /><el-input v-else @focus="configPage === 'propertyDetail' && showDetailNavigationPreview()" v-model="publicConfig.pagePresentations[configPage].title" maxlength="30" show-word-limit /></div></el-form-item>
                  <el-form-item v-if="configPage === 'requests'" label="找房引导文案"><div data-config-field="request-guide"><el-input :model-value="canvasPageCopy('说清需求，比反复筛选更高效')" @update:model-value="setPageCopy('说清需求，比反复筛选更高效', $event)" maxlength="60" show-word-limit /></div></el-form-item>
                  <el-form-item v-if="configPage !== 'propertyDetail'" label="辅助文本"><div data-config-field="bottom-subtitle"><el-input v-if="configPage === 'me' && configAudience === 'operator'" v-model="publicConfig.pagePresentations.me.operatorSubtitle" placeholder="运营管理与待办事项" maxlength="60" show-word-limit /><el-input v-else v-model="publicConfig.pagePresentations[configPage].subtitle" maxlength="60" show-word-limit /></div></el-form-item>
                  <el-form-item v-if="configPage === 'properties'" label="搜索提示"><div data-config-field="bottom-search"><el-input v-model="publicConfig.pagePresentations.properties.searchPlaceholder" placeholder="搜索小区、位置或户型" maxlength="60" show-word-limit /></div></el-form-item>
                  <el-form-item label="背景图"><el-input v-model="publicConfig.pagePresentations[configPage].backgroundImage" placeholder="上传后自动填入，也可填写图片地址" /></el-form-item>
                  <div class="presentation-actions"><label class="upload-control"><input type="file" accept="image/jpeg,image/png,image/webp" @change="uploadPresentationBackground(configPage, $event)" />上传背景图</label><button v-if="publicConfig.pagePresentations[configPage].backgroundImage" @click="publicConfig.pagePresentations[configPage].backgroundImage = ''">移除背景图</button></div>
                </el-form>
              </section>

          <div v-if="['home', 'properties'].includes(configPage)" v-show="['coverage', 'status-labels'].includes(pageEditorSection)" class="basic-settings-editor page-shared-settings">
          <section v-if="configPage === 'properties'" class="surface form-section basic-config-panel">
            <div class="section-heading"><div><h2>房源状态显示名称</h2><p>切换到管理员预览可编辑全部状态名称。</p></div></div>
            <details class="basic-settings-details" open><summary>标签名称</summary><el-form data-config-field="status-labels" label-position="top" class="basic-config-form"><div class="basic-status-label-grid"><el-form-item v-if="canvasIdentityState === 'admin'" label="草稿"><el-input v-model="publicConfig.statusLabels.draft" @focus="previewStatusScenario = 'draft'" /></el-form-item><el-form-item v-if="canvasIdentityState === 'admin'" label="待审核"><el-input v-model="publicConfig.statusLabels.pendingReview" @focus="previewStatusScenario = 'pending_review'" /></el-form-item><el-form-item v-if="canvasIdentityState === 'admin'" label="已发布"><el-input v-model="publicConfig.statusLabels.published" @focus="previewStatusScenario = 'published'" /></el-form-item><el-form-item label="可租"><el-input v-model="publicConfig.statusLabels.available" @focus="previewStatusScenario = 'available'" /></el-form-item><el-form-item v-if="canvasIdentityState === 'admin'" label="已租"><el-input v-model="publicConfig.statusLabels.rented" @focus="previewStatusScenario = 'rented'" /></el-form-item><el-form-item v-if="canvasIdentityState === 'admin'" label="已下架"><el-input v-model="publicConfig.statusLabels.offline" @focus="previewStatusScenario = 'offline'" /></el-form-item></div><div class="form-tip">用于房源状态标签的显示。小程序管理列表的「待处理」「在租」「待补资料」是筛选项。</div></el-form></details>
          </section>
          <section v-if="configPage === 'home'" data-config-field="coverage" class="surface form-section basic-config-panel">
            <div class="section-heading"><div><h2>地区与关联范围</h2><p>维护首页地区入口、频道卡片和房源关联规则。</p></div></div>
            <el-form label-position="top" class="basic-config-form">
              <el-form-item label="附近入口"><div data-config-field="home-nearbyLabel"><el-input v-model="publicConfig.pagePresentations.home.nearbyLabel" placeholder="例如：附近" /></div></el-form-item>
              <el-form-item label="频道区标题"><div data-config-field="home-channelTitle"><el-input v-model="publicConfig.pagePresentations.home.channelTitle" placeholder="例如：精选频道" /></div></el-form-item>
              <el-form-item label="频道入口文案"><div data-config-field="home-allChannelsText"><el-input v-model="publicConfig.pagePresentations.home.allChannelsText" placeholder="例如：全部频道" /></div></el-form-item>
            </el-form>
            <el-form label-position="top" class="basic-config-form"><el-form-item label="房源关联默认距离"><el-select v-model="publicConfig.relatedPropertyRadiusKm" style="width:100%"><el-option v-for="distance in [1, 2, 3, 5, 10]" :key="distance" :label="`${distance} 公里`" :value="distance" /></el-select><div class="form-tip">只作为首页“附近”的默认范围；频道地图和详情页推荐各自有独立范围，不会被这里覆盖。</div></el-form-item></el-form>
            <div class="basic-config-note"><b>距离的作用范围</b><span>默认距离用于录入时的地区关联，也用作详情附近推荐的备用距离。精选频道使用各自的地图半径。</span></div>
            <div class="basic-coverage-editor">
              <div class="section-heading config-section-heading"><div><h2>精选频道与地区入口</h2><p>频道是唯一配置来源；勾选置顶即可加入首页地区入口和房源列表筛选。选择 3–4 个置顶频道。</p></div><div class="channel-entry-count">地区入口 {{ areaEntryChannels.length }}/4</div><el-button type="primary" plain :icon="Plus" :disabled="publicConfig.featuredChannels.length >= 12" @click="addChannel">新增频道</el-button></div>
              <div v-if="publicConfig.featuredChannels.length" class="channel-editor-tabs"><button v-for="(item, index) in publicConfig.featuredChannels" :key="`basic-channel-tab-${index}`" type="button" :class="{ active: activeChannelIndex === index, configured: item.community, pinned: item.showInAreaEntry }" @click="activeChannelIndex = index"><strong>{{ item.title || '未命名频道' }}</strong><small>{{ item.community || '待选小区' }}</small><em v-if="item.showInAreaEntry">置顶</em></button></div>
              <div v-if="publicConfig.featuredChannels.length" class="channel-editor-grid">
                <article v-for="(item, index) in publicConfig.featuredChannels" v-show="activeChannelIndex === index" :key="`basic-channel-${index}`" class="channel-editor-card active">
                  <header class="channel-editor-toolbar"><span>当前频道</span><div class="item-actions"><el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="频道上移" @click="moveCoverageItem(index, -1)" /><el-button text :icon="ArrowDown" :disabled="index === publicConfig.featuredChannels.length - 1" aria-label="频道下移" @click="moveCoverageItem(index, 1)" /><el-popconfirm title="确认删除这个精选频道？" @confirm="removeChannel(index)"><template #reference><el-button text type="danger" :icon="Delete" aria-label="删除" /></template></el-popconfirm></div></header>
                  <div class="channel-pin-promotion" :class="{ selected: item.showInAreaEntry }"><span class="channel-pin-icon">⌖</span><div><strong>置顶为地区入口</strong><small>置顶后同时显示在首页快捷入口和房源列表筛选中</small></div><el-switch :model-value="Boolean(item.showInAreaEntry)" :disabled="!item.showInAreaEntry && areaEntryChannels.length >= 4" aria-label="置顶为地区入口" @update:model-value="toggleChannelAreaEntry(item, Boolean($event))" /></div>

                  <div class="channel-fields channel-fields-simple"><label><span>频道名称</span><el-input v-model="item.title" maxlength="12" placeholder="例如：金融港" @input="syncLocationFromChannel(index)" /></label><label><span>简短说明</span><el-input v-model="item.subtitle" maxlength="20" placeholder="例如：通勤方便" /></label></div>
                </article>
              </div>
              <section v-if="activeChannel" class="channel-map-workbench channel-community-workbench"><header><div><strong>关联小区</strong><span>优先选择门牌库小区；历史频道可继续使用已有地图定位。</span></div></header><label class="channel-community-field"><span>小区门牌库</span><el-select :model-value="activeChannel.community || ''" filterable clearable :disabled="!channelCommunityOptions.length" placeholder="搜索并选择已有小区" @change="selectChannelCommunity(activeChannel, activeChannelIndex, String($event || ''))"><el-option v-for="[community, count] in channelCommunityOptions" :key="community" :label="`${community} · ${count} 个门牌`" :value="community" /></el-select><small v-if="!channelCommunityOptions.length">门牌库暂无小区数据，请先在“小区门牌数据”中添加。</small><small v-else-if="!activeChannel.community && activeChannel.latitude && activeChannel.longitude">此频道已有有效地图定位，可保留原定位继续使用。</small></label><div v-if="activeChannel.community" class="channel-community-summary"><div><strong>{{ activeChannel.community }}</strong><span>{{ savedCommunityLocations[activeChannel.community]?.address || '地址待补充' }}</span><small v-if="activeChannel.latitude && activeChannel.longitude">{{ activeChannel.latitude.toFixed(6) }}, {{ activeChannel.longitude.toFixed(6) }}</small></div><el-button size="small" :type="savedCommunityLocations[activeChannel.community]?.address ? 'default' : 'primary'" @click="openChannelLocationRepair(activeChannel, activeChannelIndex)">{{ savedCommunityLocations[activeChannel.community]?.address ? '地图校准' : '地图补地址' }}</el-button></div><div v-if="channelLocationRepairOpen[activeChannelIndex]" class="channel-location-repair"><div class="channel-location-repair-head"><strong>地图搜索「{{ activeChannel.community }}」</strong><el-button link @click="channelLocationRepairOpen[activeChannelIndex] = false; channelLocationResults[activeChannelIndex] = []">收起</el-button></div><div v-if="channelLocationLoading[activeChannelIndex]" class="channel-location-loading">正在搜索地图地点…</div><div v-else-if="channelLocationResults[activeChannelIndex]?.length" class="channel-location-results"><button v-for="location in channelLocationResults[activeChannelIndex]" :key="location.id" type="button" @click="saveChannelCommunityLocation(activeChannel, activeChannelIndex, location)"><strong>{{ location.title }}</strong><span>{{ location.address || '地址信息待补充' }}</span><small>{{ location.district }}<template v-if="location.distance != null"> · {{ location.distance.toFixed(1) }} km</template></small></button></div><div v-else class="channel-location-empty"><span>没有找到地点，检查小区名称后重试。</span><el-button size="small" @click="searchChannelLocation(activeChannel, activeChannelIndex)">重新搜索</el-button></div></div><div v-if="channelMapLoading[activeChannelIndex]" class="channel-map-loading">正在加载位置预览…</div><img v-else-if="channelMapPreview[activeChannelIndex] && activeChannel.latitude && activeChannel.longitude" class="channel-map-preview channel-map-preview-compact" :src="channelMapPreview[activeChannelIndex]" alt="频道地图定位预览" /><details class="channel-advanced"><summary>卡片样式与关联范围</summary><div class="channel-fields"><label class="icon-field"><span>卡片样式</span><el-select v-model="activeChannel.icon"><el-option v-for="style in channelStyleOptions" :key="style.value" :label="style.label" :value="style.value" /></el-select></label><label class="channel-radius-field"><span>关联半径（公里）</span><el-input-number v-model="activeChannel.radiusKm" :min="0.1" :max="100" :precision="1" :step="0.5" controls-position="right" /></label></div></details></section>
              <div v-else class="config-empty"><strong>还没有精选频道</strong><span>添加频道后，用户可以从首页直接进入对应房源列表。</span><el-button type="primary" plain :icon="Plus" @click="addChannel">添加第一个频道</el-button></div>
            </div>
          </section>
          </div>
              <section v-if="['home', 'properties'].includes(configPage)" v-show="pageEditorSection === 'page-card-tags'" data-config-field="page-card-tags" class="surface form-section config-section">
              <el-form v-if="configPage === 'home'" label-position="top" class="basic-config-form">
                <el-form-item label="推荐区标题"><div data-config-field="home-recommendationTitle"><el-input v-model="publicConfig.pagePresentations.home.recommendationTitle" placeholder="例如：推荐房源" /></div></el-form-item>
                <el-form-item label="房源列表入口文案"><div data-config-field="home-viewAllText"><el-input v-model="publicConfig.pagePresentations.home.viewAllText" placeholder="例如：查看房源" /></div></el-form-item>
              </el-form>
              <header class="section-heading config-section-heading"><div><h2>{{ configPage === 'home' ? '推荐卡片数据标签' : '列表卡片数据标签' }}</h2><p>两处共用标签显隐与排版。</p></div><div class="listing-tag-preview-switch" aria-label="列表标签预览位置"><el-button size="small" :type="configPage === 'home' ? 'primary' : 'default'" :aria-pressed="configPage === 'home'" @click="previewHomeRecommendation()">首页推荐</el-button><el-button size="small" :type="configPage === 'properties' ? 'primary' : 'default'" :aria-pressed="configPage === 'properties'" @click="selectConfigPage('properties'); locatePageEditor('page-card-tags')">房源列表</el-button></div></header>
              <div class="share-field-options"><button v-for="field in listingTitleFieldOptions.filter(item => item.key !== 'community' && publicConfig.listingTitle.customerAllowedFields.includes(item.key))" :key="`basic-customer-list-${field.key}`" type="button" class="field-check" :class="{ checked: publicConfig.listingTitle.enabledFields.includes(field.key) }" :aria-pressed="publicConfig.listingTitle.enabledFields.includes(field.key)" @click="toggleListingTitleField(field.key, !publicConfig.listingTitle.enabledFields.includes(field.key))"><i>{{ publicConfig.listingTitle.enabledFields.includes(field.key) ? '✓' : '' }}</i><span>{{ field.label }}</span></button></div>                <div class="layout-choice-row">
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

              </section>


              <section v-if="configPage === 'propertyDetail' && pageEditorSection === 'detail-modules'" class="surface detail-config-workspace" aria-label="房源详情配置工作区">
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
              <section v-if="configPage === 'propertyDetail' && pageEditorSection === 'detail-modules'" class="surface form-section config-section share-card-config">
                <div class="section-heading config-section-heading"><div><h2>房源详情模块</h2><p>这里配置录入和详情展示模块；标签规则统一在“全局设置 → 房源标签规则”维护。</p></div><el-button plain @click="selectTab('小程序基础配置'); selectBasicConfigTab('tags')">配置标签规则</el-button></div>
                  <div v-if="detailSectionTab === 'overview'" data-config-field="detail-overview" class="share-field-editor listing-title-editor">
                    <header><strong>固定首屏摘要</strong><span>这不是可关闭模块。面积、户型、楼层等房屋概况始终在这里展示；合租录入整套参数后，这里显示整套户型、整套面积。当前出租房间的实际空间与面积则用于列表标签，并在下方“租赁信息”展示。</span></header>
                    <div class="basic-config-note"><b>列表标签与详情信息已分层</b><span>列表可使用标签吸引与筛选；详情首屏以对称参数为主，不继承普通列表标签。房源简介、位置与通勤、房屋设施、附近推荐在下方作为独立模块管理。</span></div>
                    <div class="basic-config-note"><b>详情页与列表标签分层</b><span>详情首屏仅在转租时突出“剩余租期”等特殊提醒；整租/合租、面积、户型、楼层等在参数区对称展示，中介费在租赁信息中说明。</span><el-button plain @click="selectTab('小程序基础配置'); selectBasicConfigTab('tags')">调整列表标签</el-button></div>
                  </div>
                  <div v-else-if="detailSectionTab === 'location'" data-config-field="detail-location" class="share-field-editor listing-title-editor">
                    <header><strong>位置与周边</strong><span>小区与地图坐标用于归属地区和附近房源关联，始终保留；本模块控制通勤等周边补充字段及详情展示。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-module-form"><el-form-item label="模块标题"><el-input v-model="detailModule('location').title" maxlength="12" /></el-form-item></el-form>
                  </div>
                  <div v-else-if="detailSectionTab === 'facilities'" data-config-field="detail-facilities" class="share-field-editor listing-title-editor">
                    <header><strong>房屋设施</strong><span>本页只控制该模块是否出现在录入页和详情页；设施的名称、分类、可勾选项统一由“全局设置”维护。水电杂费属于首屏租赁信息，不归入本模块。</span></header>
                    <el-form label-position="top" class="basic-config-form detail-module-form"><el-form-item label="模块标题"><el-input v-model="detailModule('facilities').title" maxlength="12" /></el-form-item></el-form>
                    <div class="basic-config-note"><b>当前设施库：{{ publicConfig.propertyDetail.facilities.filter(item => item.enabled).length }} 项已启用</b><span>进入“全局设置 → 房屋设施库”后，点击整张设施卡即可调整录入和详情可用项。</span><el-button plain @click="selectTab('小程序基础配置'); selectBasicConfigTab('facilities')">配置设施库</el-button></div>
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
              <section v-if="configAudience === 'customer' && configPage === 'propertyDetail' && pageEditorSection === 'detail-sharing'" class="surface form-section config-section share-card-config">
                <div class="section-heading config-section-heading"><div><h2>微信分享卡</h2><p>好友分享不区分角色，统一采用客户公开标签；右侧画布显示聊天中的实际卡片效果。</p></div></div>
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

          </template>
          </div>

            <aside class="config-preview-column" :class="{ 'canvas-without-shortcuts': tab === '小程序基础配置' || configPage === 'home' }" aria-label="小程序实时画布">
              <nav v-if="tab === '小程序页面配置' && configPage !== 'home'" class="canvas-settings-shortcuts" aria-label="画布配置快捷入口">
                <label v-if="['home', 'properties', 'me'].includes(configPage)" class="home-copy-mode"><input v-model="homeCopyEditing" type="checkbox"> 点选页面文案</label>
                <button type="button" @click="openCanvasNode('status')">状态名称</button>
                <button type="button" @click="openCanvasNode('coverage')">地区与频道</button>
              </nav>
              <div ref="canvasPreviewStage" class="canvas-preview-stage">
              <div v-if="configPage === 'home'" ref="canvasPreviewPhone" class="phone-preview canvas-home-phone" :class="[`canvas-identity-${canvasIdentityState}`, { 'canvas-operator': configAudience === 'operator' }]" :style="{ zoom: canvasFitScale, transform: 'none' }">
                <div class="phone-status"><span>22:18</span><span>4G ▰</span></div>
                <div class="mini-navigation">{{ publicConfig.heroTitle || '小皮租房' }}</div>
                <div class="mini-content" :style="publicConfig.pagePresentations.home.backgroundImage ? { backgroundImage: `linear-gradient(rgba(247,246,241,.88),rgba(247,246,241,.96)), url(${publicConfig.pagePresentations.home.backgroundImage})` } : {}">
                  <section v-if="canvasAllChannelsOpen" class="canvas-all-channels">
                    <header><button type="button" aria-label="返回首页" @click="canvasAllChannelsOpen = false">‹</button><div><strong>精选频道</strong><small>选择一个频道，查看对应范围内的房源</small></div></header>
                    <div class="canvas-all-channel-grid"><button v-for="(item, index) in publicConfig.featuredChannels" :key="`all-preview-channel-${index}`" type="button" :class="`preview-tone-${item.icon || index}`" @click="selectCanvasChannel(item)"><b><i :class="`preview-mark-${item.icon || index}`"></i></b><strong>{{ item.title || '未命名' }}</strong><span>{{ item.subtitle || '频道说明' }}</span><em>查看房源 ›</em></button><div v-if="!publicConfig.featuredChannels.length" class="canvas-all-channels-empty">暂时还没有精选频道</div></div>
                  </section>
                  <template v-else>
                  <div v-if="canvasNotice" class="canvas-preview-notice" role="status">{{ canvasNotice }}</div>
                  <section class="mini-hero" :style="{ backgroundImage: `linear-gradient(90deg, #f9e9d7 0%, #f9e9d7a3 57%, #f9e9d729 100%), url(${publicConfig.pagePresentations.home.backgroundImage || homeHeroFallback}), url(${homeHeroFallback})` }"><span class="mini-city" @click="canvasAreaFilter = canvasAreaFilter === '全部' ? (areaEntryChannels[0]?.title || '全部') : '全部'"><i class="canvas-city-pin" aria-hidden="true">⌖</i><span>武汉</span><small>⌄</small></span><strong class="canvas-editable-copy" title="首页主标题" @click="canPickHomeCopy && openCanvasNode('home.title')">{{ publicConfig.pagePresentations.home.title || '今天想住哪里？' }}</strong><i></i><div class="mini-search"><span class="canvas-search-icon" aria-hidden="true">⌕</span><input v-model="canvasHomeSearchQuery" class="canvas-search-input" :placeholder="publicConfig.pagePresentations.home.subtitle || '搜索小区、地铁或商圈'" @keyup.enter="searchFromCanvasHome()"><button type="button" @click="searchFromCanvasHome()">{{ canvasPageCopy("搜索") }}</button></div><button type="button" class="canvas-home-map-entry" @click="canvasHomeMapOpen = true; canvasHomeMapActiveProperty = null">⌖ 地图找房 <span>›</span></button></section>
                  <div class="mini-locations"><button type="button" :class="{ active: canvasAreaFilter === '全部' }" @click="canvasAreaFilter = '全部'; switchCanvasTab('properties')">{{ publicConfig.pagePresentations.home.nearbyLabel || '附近' }}</button><button v-for="(item, index) in areaEntryChannels" :key="`preview-location-${index}`" type="button" :class="{ active: canvasAreaFilter === item.title }" @click="canvasAreaFilter = item.title; switchCanvasTab('properties')">{{ item.title || '未命名' }}</button><em v-if="!areaEntryChannels.length">尚未配置</em></div>
                  <div class="mini-budget"><b>租金预算</b><button v-for="budget in ['≤1500', '1500–2500', '2500–3500', '≥3500']" :key="budget" type="button" :class="{ selected: canvasHomeBudget === budget }" @click="canvasHomeBudget = canvasHomeBudget === budget ? '不限' : budget">{{ budget }}</button></div>
                  <div class="mini-budget mini-kind"><b>住法</b><button v-for="kind in ['整租', '合租', '一居', '两居', '三居+']" :key="kind" type="button" :class="{ selected: canvasHomeKind === kind }" @click="canvasHomeKind = canvasHomeKind === kind ? '全部' : kind">{{ kind }}</button></div>
                  <div class="mini-section-title"><strong class="canvas-editable-copy" title="点击编辑首页分区文案" @click="canPickHomeCopy && openCanvasNode('home.channelTitle')">{{ publicConfig.pagePresentations.home.channelTitle || '精选频道' }}</strong><b role="button" tabindex="0" @click="canvasPageFilterReset(); canvasAllChannelsOpen = true">{{ publicConfig.pagePresentations.home.allChannelsText || '全部频道' }}</b></div>
                  <div class="mini-channels"><button v-for="(item, index) in publicConfig.featuredChannels" :key="`preview-channel-${index}`" type="button" :class="`preview-tone-${item.icon || index}`" @click="canvasChannelFilter = item.title; switchCanvasTab('properties')"><b><i :class="`preview-mark-${item.icon || index}`"></i></b><strong>{{ item.title || '未命名' }}</strong><span>{{ item.subtitle || '频道说明' }}</span></button><em v-if="!publicConfig.featuredChannels.length">尚未配置</em></div>
                <div class="mini-recommend"><strong class="canvas-editable-copy" title="点击编辑首页分区文案" @click="canPickHomeCopy && openCanvasNode('home.recommendationTitle')">{{ publicConfig.pagePresentations.home.recommendationTitle || '推荐房源' }}</strong><b @click="canvasPageFilterReset(); switchCanvasTab('properties')">{{ publicConfig.pagePresentations.home.viewAllText || '查看房源' }}</b></div>
                  <div v-if="pageEditorSection === 'display-guidance' && previewCopyScenario" class="canvas-home-empty-state"><strong>{{ canvasPageCopy(previewCopyScenario === '新房源正在整理中，稍后再来看看' ? '暂时没有可租房源' : '没有找到符合条件的房源') }}</strong><small>{{ canvasPageCopy(previewCopyScenario === '新房源正在整理中，稍后再来看看' ? '新房源正在整理中，稍后再来看看' : '试试放宽筛选条件，看看更多房源') }}</small><button v-if="previewCopyScenario !== '新房源正在整理中，稍后再来看看'" type="button" @click="previewCopyScenario = ''">{{ canvasPageCopy('清除筛选') }}</button></div>
                  <div v-else-if="pageEditorSection === 'notes-1' && previewCopyScenario" class="preview-empty"><b>{{ canvasPageCopy(previewCopyScenario) }}</b></div>
                  <div v-else-if="canvasDataState !== 'normal'" class="preview-empty">{{ canvasDataState === 'loading' ? '正在加载房源…' : canvasDataState === 'error' ? '房源暂时加载失败 · 检查网络后重新加载' : '暂时没有可租房源 · 新房源正在整理中' }}</div>
                  <div v-if="canvasDataState === 'normal' && !previewCopyScenario && configAudience === 'customer' && previewHomeProperties.length" class="preview-home-masonry">
                    <div v-for="(column, index) in previewHomeMasonryColumns" :key="index" class="preview-home-column"><article v-for="item in column" :key="item.id" class="preview-property-card preview-masonry-card" @click="openPreviewProperty(item)"><div class="preview-card-cover" :style="previewCoverStyle(item, 0, true)"><span v-if="!item.media?.length" class="canvas-home-placeholder-initials">{{ (item.community || '小皮').slice(0, 2) }}</span><em>{{ previewStatusLabel(item.status) }}</em><i v-if="item.media?.some(media => media.type === 'video')" class="preview-video-play">▶</i><small v-if="previewChannelLabel(item)" class="preview-cover-location">⌖ {{ previewChannelLabel(item) }}</small><button type="button" class="canvas-favorite-toggle" :class="{ active: canvasPreviewFavorites.includes(item.id) }" @click.stop="toggleCanvasFavorite(item.id)">{{ canvasPreviewFavorites.includes(item.id) ? '♥' : '♡' }}</button></div><section><b>{{ previewListingTitle(item) }}</b><small v-if="previewTagGroups(item).all.length" class="preview-tag-row all"><i v-for="tag in [...new Set(previewTagGroups(item).all)]" :key="tag">{{ tag }}</i></small><strong>¥ {{ item.rentMonthly }}<em>/月</em></strong></section></article></div>
                  </div>
                  <article v-else-if="canvasDataState === 'normal' && !previewCopyScenario && configAudience !== 'customer' && previewHomeProperties.length" class="preview-operator-home-list"><div v-for="item in previewHomeProperties.slice(0, 4)" :key="item.id" class="preview-property-card preview-home-property-card" @click="openPreviewProperty(item)"><div class="preview-card-cover" :style="previewCoverStyle(item, 0, true)"><span v-if="!item.media?.length" class="canvas-home-placeholder-initials">{{ (item.community || '小皮').slice(0, 2) }}</span><em>{{ previewStatusLabel(item.status) }}</em><i v-if="item.media?.some(media => media.type === 'video')" class="preview-video-play">▶</i><small v-if="previewChannelLabel(item)" class="preview-cover-location">⌖ {{ previewChannelLabel(item) }}</small></div><section><b>{{ previewListingTitle(item) }}</b><small v-if="publicConfig.listingTitle.adminEnabledFields.includes('unit') && (item.buildingNo || item.unitNo || item.roomNo || item.roomSequence)" class="preview-operator-doorplate">门牌 · {{ previewDoorplate(item) }}</small><div class="preview-tag-stack" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`"><template v-if="publicConfig.listingTitle.listTagLayout === 'grouped'"><small v-if="previewTagGroups(item).primary.length" class="preview-tag-row primary"><i v-for="tag in previewTagGroups(item).primary" :key="tag">{{ tag }}</i></small><small v-if="previewTagGroups(item).secondary.length" class="preview-tag-row secondary"><i v-for="tag in previewTagGroups(item).secondary" :key="tag">{{ tag }}</i></small></template><small v-else-if="previewTagGroups(item).all.length" class="preview-tag-row all"><i v-for="tag in previewTagGroups(item).all" :key="tag">{{ tag }}</i></small></div><strong>¥ {{ item.rentMonthly }}<em>/月</em></strong></section></div></article>
                  <div v-else-if="canvasDataState === 'normal' && !previewCopyScenario" class="preview-empty">{{ canvasPageCopy("暂时没有可租房源") }}</div>
                  </template>
                </div>
                <div v-if="canvasHomeMapOpen" class="preview-overlay canvas-map-overlay" @click="canvasHomeMapOpen = false; canvasHomeMapActiveProperty = null"><section class="canvas-home-map-sheet" role="dialog" aria-modal="true" aria-label="地图找房" @click.stop><header><button type="button" aria-label="返回首页" @click="canvasHomeMapOpen = false; canvasHomeMapActiveProperty = null">‹</button><div><strong>地图找房</strong><small>按小区查看附近房源</small></div><span>{{ canvasHomeMapMarkers.length }} 个小区</span></header><div class="canvas-home-map" :style="{ backgroundImage: `url(${editorMapPreview})` }"><button v-for="marker in canvasHomeMapMarkers" :key="marker.community" type="button" class="canvas-home-map-marker" :class="{ active: canvasHomeMapActiveProperty?.id === marker.property.id }" :style="{ left: `${marker.x}%`, top: `${marker.y}%` }" @click="canvasHomeMapActiveProperty = marker.property"><b>{{ marker.items.length }}</b><span>{{ marker.community }}</span></button><div v-if="!canvasHomeMapMarkers.length" class="canvas-map-no-markers">当前数据没有可定位的房源</div></div><footer><strong>已定位房源</strong><span>{{ canvasHomeMapPropertyCount }} 套</span></footer><button v-if="canvasHomeMapActiveProperty" type="button" class="canvas-home-map-selected" @click="openPreviewProperty(canvasHomeMapActiveProperty)"><span><b>{{ canvasHomeMapActiveProperty.community }}</b><small>{{ canvasHomeMapActiveProperty.title }} · ¥{{ canvasHomeMapActiveProperty.rentMonthly }}/月</small></span><strong>查看房源 ›</strong></button><p v-else class="canvas-map-hint">点击地图标记查看该小区房源</p></section></div>
                <button v-if="configPage === 'home' && configAudience === 'customer'" type="button" class="canvas-request-fab" @click="navigateCanvasPage('requests')">✎ <span>{{ canvasPageCopy("说说找房需求") }}</span></button>
                <div class="mini-tabbar"><button :class="{ active: isCanvasTabActive('home') }" @click="switchCanvasTab('home')"><img :src="isCanvasTabActive('home') ? homeTabActiveIcon : homeTabIcon" alt=""><small>首页</small></button><button :class="{ active: isCanvasTabActive('properties') }" @click="switchCanvasTab('properties')"><img :src="isCanvasTabActive('properties') ? propertyTabActiveIcon : propertyTabIcon" alt=""><small>房源</small></button><button :class="{ active: isCanvasTabActive('me') }" @click="switchCanvasTab('me')"><img :src="isCanvasTabActive('me') ? meTabActiveIcon : meTabIcon" alt=""><small>我的</small></button></div>
              </div>
              <div v-else ref="canvasPreviewPhone" class="phone-preview page-phone-preview" :class="[`preview-${configPage}`, `canvas-identity-${canvasIdentityState}`, { 'canvas-operator': configAudience === 'operator' }]" :style="{ zoom: canvasFitScale, transform: 'none' }">
                <div class="phone-status"><span>22:18</span><span>4G ▰</span></div>
                <div v-if="configPage !== 'propertyDetail'" class="mini-navigation"><button v-if="!['home', 'properties', 'me'].includes(configPage)" type="button" aria-label="返回" @click="backCanvasPage">‹</button><span>{{ canvasNativeTitle }}</span></div>
                <div class="page-mini-content" :style="publicConfig.pagePresentations[configPage].backgroundImage ? { backgroundImage: `linear-gradient(rgba(247,246,241,.9),rgba(247,246,241,.96)), url(${publicConfig.pagePresentations[configPage].backgroundImage})` } : {}">
                  <div v-if="canvasNotice" class="canvas-preview-notice" role="status">{{ canvasNotice }}</div>
                  <header v-if="!['requests', 'authLogin'].includes(configPage)" class="canvas-editable-copy" :class="{ 'canvas-me-heading': configPage === 'me', 'canvas-listing-heading': configPage === 'properties', 'canvas-history-heading': configPage === 'requestHistory' }" title="点击编辑本页标题与辅助文本" @click="!['properties', 'me'].includes(configPage) && openCanvasNode('page.heading')"><template v-if="configPage === 'properties'"><div class="canvas-listing-title-row"><strong @click.stop="pickBottomPageCopy('bottom-title')">{{ configAudience === 'operator' ? (publicConfig.pagePresentations.properties.operatorTitle || '房源管理') : publicConfig.pagePresentations[configPage].title }}</strong><span class="canvas-listing-count">{{ previewPropertyEmpty ? 0 : previewProperties.length }} {{ configAudience === 'operator' ? '套房源' : '套可租' }}</span></div><span @click.stop="pickBottomPageCopy('bottom-subtitle')">{{ publicConfig.pagePresentations[configPage].subtitle }}</span></template><template v-else-if="configPage === 'me'"><div class="canvas-me-heading-row"><div><small @click.stop="pickBottomPageCopy('bottom-eyebrow')">{{ publicConfig.pagePresentations.me.eyebrow || '个人中心' }}</small><strong @click.stop="pickBottomPageCopy('bottom-title')">{{ publicConfig.pagePresentations[configPage].title }}</strong></div><span @click.stop="pickBottomPageCopy('bottom-subtitle')">{{ configAudience === 'operator' ? (publicConfig.pagePresentations.me.operatorSubtitle || '运营管理与待办事项') : publicConfig.pagePresentations[configPage].subtitle }}</span></div></template><template v-else-if="configPage === 'requestHistory'"><div><strong>{{ publicConfig.pagePresentations[configPage].title }}</strong><span>{{ publicConfig.pagePresentations[configPage].subtitle }}</span></div><div class="canvas-history-count"><strong>{{ canvasIdentityState === 'admin' ? '—' : 0 }}</strong><small>{{ canvasIdentityState === 'admin' ? '需授权' : '进行中' }}</small></div></template><template v-else><strong>{{ publicConfig.pagePresentations[configPage].title }}</strong><span>{{ publicConfig.pagePresentations[configPage].subtitle }}</span></template></header>
                  <div v-if="canvasDataState === 'loading'" class="preview-empty"><b>正在加载…</b><small>这是小程序列表页的加载状态。</small></div>
                  <div v-else-if="canvasDataState === 'error'" class="preview-empty"><b>{{ canvasPageCopy("数据加载失败", canvasPageCopy("房源暂时加载失败")) }}</b><small>{{ canvasPageCopy("检查网络后重新加载。") }}</small><button type="button" @click="load().catch(() => undefined)">重新加载</button></div>
                  <div v-else-if="canvasDataState === 'empty' && !previewPropertyEmpty && configPage === 'properties' && configAudience === 'customer'" class="canvas-properties-empty"><span class="canvas-properties-empty-icon">⌕</span><strong>{{ canvasPropertyEmptyTitle }}</strong><small>{{ (canvasFavoriteOnly || previewFavoritesEmpty) ? canvasPageCopy('先浏览房源并点亮爱心，关注的房源会集中保存在这里。') : canvasPageCopy('可以调整筛选条件，或告诉小皮你的预算和入住要求。') }}</small><button type="button" @click="previewFavoritesEmpty ? locatePageEditor('page-copy') : handleCanvasPropertyEmptyAction()">{{ canvasPropertyEmptyAction }}</button><a v-if="canvasHasPropertyFilters && !canvasFavoriteOnly" href="#" @click.prevent="canvasPageFilterReset">{{ canvasPageCopy('清除筛选条件') }}</a></div>
                  <div v-else-if="canvasDataState === 'empty' && !previewPropertyEmpty" class="preview-empty"><b>当前没有可展示记录</b><small>清除筛选条件或稍后再试。</small></div>
                  <template v-else-if="configPage === 'properties'">
                    <div class="preview-search canvas-list-search"><span class="canvas-search-icon" aria-hidden="true">⌕</span><input v-model="canvasSearchQuery" :placeholder="publicConfig.pagePresentations.properties.searchPlaceholder || '搜索小区、位置或户型'"><button v-if="canvasSearchQuery" type="button" @click="canvasSearchQuery = ''">×</button></div>
                    <template v-if="configAudience === 'operator'"><button class="canvas-admin-add" @click="startCanvasPropertyEdit()">＋ 录入新房源 <small>保存为草稿后继续完善</small></button><strong class="canvas-admin-filter-label">状态</strong><div class="preview-filter canvas-filter-row"><button v-for="status in canvasAdminPropertyFilters" :key="status.key" type="button" :class="{ active: canvasStatusFilter === status.key }" @click="canvasStatusFilter = status.key">{{ status.label }}</button></div><strong class="canvas-admin-filter-label">地区</strong><div class="preview-filter canvas-filter-row"><button v-for="area in ['全部', ...areaEntryChannels.map(item => item.title)]" :key="area" :class="{ active: canvasAreaFilter === area }" @click="canvasAreaFilter = area">{{ area }}</button></div><details class="canvas-admin-more">
                      <summary>☷ 更多筛选 <small>预算{{ canvasBudgetFilter }} · 住法{{ canvasRentTypeFilter === '全部' ? '不限' : canvasRentTypeFilter }}</small></summary>
                      <strong class="canvas-admin-filter-label">月租预算</strong>
                      <div class="preview-filter canvas-filter-row"><button v-for="budget in ['不限', '≤1500', '1500–2500', '2500–3500', '≥3500']" :key="budget" :class="{ active: canvasBudgetFilter === budget }" @click="canvasBudgetFilter = budget">{{ budget === '不限' ? '不限价格' : budget }}</button></div>
                      <strong class="canvas-admin-filter-label">住法</strong>
                      <div class="preview-filter canvas-filter-row"><button v-for="kind in ['全部', '整租', '合租', '一居', '两居', '三居+']" :key="kind" :class="{ active: canvasRentTypeFilter === kind }" @click="canvasRentTypeFilter = kind">{{ kind === '全部' ? '不限' : kind }}</button></div>
                    </details></template>
                    <template v-else><button type="button" class="canvas-favorite-filter" :class="{ active: canvasFavoriteOnly }" @click="canvasFavoriteOnly = !canvasFavoriteOnly"><span aria-hidden="true">♡</span>{{ canvasFavoriteOnly ? '只看已关注' : '只看关注' }}</button><div class="canvas-area-filter"><strong>地区</strong><div class="preview-filter canvas-filter-row"><button type="button" :class="{ active: canvasAreaFilter === '全部' }" @click="canvasAreaFilter = '全部'">全部</button><button v-for="area in areaEntryChannels" :key="area.id || area.title" type="button" :class="{ active: canvasAreaFilter === area.title }" @click="canvasAreaFilter = area.title">{{ area.title }}</button></div></div><button type="button" class="canvas-property-filter-toggle" :aria-expanded="canvasPropertyFiltersOpen" @click="canvasPropertyFiltersOpen = !canvasPropertyFiltersOpen"><span><i aria-hidden="true">☷</i> 更多筛选</span><small>{{ canvasBudgetFilter === '不限' ? '预算不限' : canvasBudgetFilter }} · {{ canvasRentTypeFilter === '全部' ? '住法不限' : canvasRentTypeFilter }}</small><b :class="{ expanded: canvasPropertyFiltersOpen }">⌄</b></button><div v-if="canvasPropertyFiltersOpen" class="canvas-property-extra-filters"><div><strong>月租预算</strong><div class="preview-filter canvas-filter-row"><button v-for="budget in ['不限', '≤1500', '1500–2500', '2500–3500', '≥3500']" :key="budget" type="button" :class="{ active: canvasBudgetFilter === budget }" @click="canvasBudgetFilter = budget">{{ budget === '不限' ? '不限价格' : budget }}</button></div></div><div><strong>住法</strong><div class="preview-filter canvas-filter-row"><button v-for="kind in ['全部', '整租', '合租', '一居', '两居', '三居+']" :key="kind" type="button" :class="{ active: canvasRentTypeFilter === kind }" @click="canvasRentTypeFilter = kind">{{ kind === '全部' ? '不限' : kind }}</button></div></div></div></template>
                    <div v-if="configAudience === 'customer'" class="preview-masonry-flow">
                      <div v-for="(column, columnIndex) in previewCustomerMasonryColumns" :key="`property-column-${columnIndex}`" class="preview-masonry-column">
                        <article v-for="item in column" :key="item.id" class="preview-property-card preview-masonry-card" @click="openPreviewProperty(item)"><div class="preview-card-cover" :class="{ 'preview-cover-placeholder': !item.media?.length }" :style="previewCoverStyle(item)"><template v-if="!item.media?.length"><img class="canvas-property-placeholder" :src="propertyPlaceholderIcon" alt=""><small class="preview-placeholder-label">{{ canvasPageCopy("暂无实拍图") }}</small></template><em>{{ previewStatusLabel(item.status) }}</em><i v-if="item.media?.some(media => media.type === 'video')" class="preview-video-play">▶</i><small v-if="previewChannelLabel(item)" class="preview-cover-location">⌖ {{ previewChannelLabel(item) }}</small><button type="button" class="canvas-favorite-toggle" :class="{ active: canvasPreviewFavorites.includes(item.id) }" @click.stop="toggleCanvasFavorite(item.id)">{{ canvasPreviewFavorites.includes(item.id) ? '♥' : '♡' }}</button></div><section><b>{{ previewListingTitle(item) }}</b><div class="preview-tag-stack" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`"><template v-if="publicConfig.listingTitle.listTagLayout === 'grouped'"><small v-if="previewTagGroups(item).primary.length" class="preview-tag-row primary"><i v-for="tag in previewTagGroups(item).primary" :key="tag">{{ tag }}</i></small><small v-if="previewTagGroups(item).secondary.length" class="preview-tag-row secondary"><i v-for="tag in previewTagGroups(item).secondary" :key="tag">{{ tag }}</i></small></template><small v-else-if="previewTagGroups(item).all.length" class="preview-tag-row all"><i v-for="tag in previewTagGroups(item).all" :key="tag">{{ tag }}</i></small></div><strong>¥ {{ item.rentMonthly }}<em>/月</em></strong></section></article>
                      </div>
                    </div>
                    <div v-else class="preview-property-flow">
                      <article v-for="item in previewProperties" :key="item.id" class="preview-property-card canvas-admin-listing" @click="openPreviewProperty(item)"><div class="preview-card-cover" :class="{ 'preview-cover-placeholder': !item.media?.length }" :style="previewCoverStyle(item)"><template v-if="!item.media?.length"><img class="canvas-property-placeholder" :src="propertyPlaceholderIcon" alt=""><small class="preview-placeholder-label">{{ canvasPageCopy("暂无实拍图") }}</small></template><em>{{ previewStatusLabel(item.status) }}</em></div><section><b>{{ previewListingTitle(item) }}</b><small v-if="configAudience === 'operator' && publicConfig.listingTitle.adminEnabledFields.includes('unit') && (item.buildingNo || item.unitNo || item.roomNo || item.roomSequence)" class="preview-operator-doorplate">门牌 · {{ previewDoorplate(item) }}</small><div class="preview-tag-stack" :class="`preview-tag-layout-${publicConfig.listingTitle.listTagLayout}`"><template v-if="publicConfig.listingTitle.listTagLayout === 'grouped'"><small v-if="previewTagGroups(item).primary.length" class="preview-tag-row primary"><i v-for="tag in previewTagGroups(item).primary" :key="tag">{{ tag }}</i></small><small v-if="previewTagGroups(item).secondary.length" class="preview-tag-row secondary"><i v-for="tag in previewTagGroups(item).secondary" :key="tag">{{ tag }}</i></small></template><small v-else-if="previewTagGroups(item).all.length" class="preview-tag-row all"><i v-for="tag in previewTagGroups(item).all" :key="tag">{{ tag }}</i></small></div><strong>¥ {{ item.rentMonthly }}<em>/月</em></strong></section><footer class="canvas-admin-list-actions"><button @click.stop="startCanvasPropertyEdit(item)">编辑</button><button v-for="action in ['已租', '下架', '复制', '删除']" :key="action" @click.stop="submitCanvasPreview">{{ action }}</button></footer></article>
                    </div>
                    <div v-if="!previewProperties.length || previewPropertyEmpty" class="canvas-properties-empty"><span class="canvas-properties-empty-icon">⌕</span><strong>{{ canvasPropertyEmptyTitle }}</strong><small>{{ configAudience === 'operator' ? canvasPageCopy('调整地区、预算或关键词，继续查看房源。') : (canvasFavoriteOnly || previewFavoritesEmpty) ? canvasPageCopy('先浏览房源并点亮爱心，关注的房源会集中保存在这里。') : canvasPageCopy('可以调整筛选条件，或告诉小皮你的预算和入住要求。') }}</small><button type="button" @click="previewFavoritesEmpty ? locatePageEditor('page-copy') : configAudience === 'operator' ? canvasPageFilterReset() : handleCanvasPropertyEmptyAction()">{{ configAudience === 'operator' ? canvasPageCopy('重置筛选') : canvasPropertyEmptyAction }}</button><a v-if="(canvasHasPropertyFilters || pageEditorSection === 'properties-empty') && configAudience === 'customer' && !canvasFavoriteOnly" href="#" @click.prevent="canvasPageFilterReset">{{ canvasPageCopy('清除筛选条件') }}</a></div>
                  </template>
                  <template v-if="canvasDataState === 'normal' && configPage === 'propertyDetail'">
                    <div class="detail-canvas" :class="{ operator: configAudience === 'operator' }">
                      <div v-if="canvasDetailScrolled" class="canvas-detail-sticky"><i class="canvas-detail-sticky-capsule" aria-hidden="true">•••　⊙</i><button type="button" aria-label="返回" @click="backCanvasPage">‹</button><span class="canvas-editable-copy" @click="focusConfigField('bottom-title')">{{ publicConfig.pagePresentations.propertyDetail.title || '房源详情' }}</span></div>
                      <div class="detail-canvas-scroll" @scroll.passive="canvasDetailScrolled = ($event.currentTarget as HTMLElement).scrollTop > 65">
                        <div class="detail-canvas-cover" :class="{ 'is-placeholder': !previewPrimaryProperty?.media?.length }" :style="previewCoverStyle(previewPrimaryProperty, canvasMediaIndex)"><video v-if="previewPrimaryProperty?.media?.[canvasMediaIndex]?.type === 'video'" :src="previewPrimaryProperty.media[canvasMediaIndex].url" :poster="previewPrimaryProperty.media.find(media => media.type === 'image')?.url" controls playsinline preload="metadata" @click.stop/><div class="detail-canvas-media-nav"><button type="button" @click="backCanvasPage">‹</button><i>•••　⊙</i></div><div v-if="!previewPrimaryProperty?.media?.length" class="detail-canvas-empty-media"><img :src="propertyPlaceholderIcon" alt=""><b>{{ canvasPageCopy("暂无实拍图") }}</b><small>{{ canvasPageCopy('联系小皮了解房源详情') }}</small></div><div v-if="previewPrimaryProperty?.media?.length" class="detail-canvas-media-tabs"><button v-if="previewPrimaryProperty.media.some(media => media.type === 'video')" type="button" :class="{ active: previewPrimaryProperty.media[canvasMediaIndex]?.type === 'video' }" @click="canvasMediaIndex = previewPrimaryProperty.media.findIndex(media => media.type === 'video')">视频</button><button v-if="previewPrimaryProperty.media.some(media => media.type === 'image')" type="button" :class="{ active: previewPrimaryProperty.media[canvasMediaIndex]?.type === 'image' }" @click="canvasMediaIndex = previewPrimaryProperty.media.findIndex(media => media.type === 'image')">照片 {{ previewPrimaryProperty.media[canvasMediaIndex]?.type === 'image' ? previewPrimaryProperty.media.slice(0, canvasMediaIndex + 1).filter(media => media.type === 'image').length : 1 }}/{{ previewPrimaryProperty.media.filter(media => media.type === 'image').length }}</button><span class="detail-canvas-media-pager"><button type="button" aria-label="上一张" @click="stepCanvasMedia(-1)">‹</button>{{ canvasMediaIndex + 1 }}/{{ previewPrimaryProperty.media.length }}<button type="button" aria-label="下一张" @click="stepCanvasMedia(1)">›</button></span></div></div>
                        <section v-if="detailModuleEnabled('overview')" data-canvas-area="overview" class="detail-canvas-card overview-card" @click="focusCanvasArea('overview')">
                          <em v-if="configAudience === 'operator' && previewAdminStatusLabel" class="detail-canvas-status-preview">{{ previewAdminStatusLabel }}</em>
                          <div class="detail-canvas-price">{{ previewPrimaryProperty?.rentMonthly || 0 }}<small>元/月</small><button v-if="configAudience === 'customer'" type="button" :class="{ active: canvasPreviewFavorites.includes(previewPrimaryProperty?.id || '') }" @click="previewPrimaryProperty && toggleCanvasFavorite(previewPrimaryProperty.id)">{{ canvasPreviewFavorites.includes(previewPrimaryProperty?.id || '') ? '♥' : '♡' }} <span>{{ canvasPreviewFavorites.includes(previewPrimaryProperty?.id || '') ? '已关注' : '关注' }}</span></button></div>
                          <strong class="detail-canvas-title">{{ previewPrimaryProperty ? (configAudience === 'operator' ? previewPrimaryProperty.title || previewPrimaryProperty.community : previewListingTitle(previewPrimaryProperty)) : '房源详情' }}</strong>
                          <span class="detail-canvas-address">{{ previewDisplayAddress(previewPrimaryProperty) }}</span>
                          <small v-if="previewOperatorDoorplate" class="preview-operator-doorplate">门牌定位 · {{ previewOperatorDoorplate }}</small>
                          <div v-if="previewPrimaryProperty?.listingSource === '转租'" class="detail-canvas-meta"><em>转租 · 剩余 {{ previewPrimaryProperty?.remainingLeaseMonths || 0 }} 个月</em></div>
                          <div class="detail-canvas-facts"><i><small>{{ previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.wholeAreaSqm ? '整套面积' : '面积' }}</small>{{ previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.wholeAreaSqm ? `${previewPrimaryProperty.wholeAreaSqm}㎡` : previewPrimaryProperty?.areaSqm ? `${previewPrimaryProperty.areaSqm}㎡` : '待补充' }}</i><i><small>{{ previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.wholeLayout ? '整套户型' : '户型' }}</small>{{ previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.wholeLayout ? previewPrimaryProperty.wholeLayout : previewPrimaryProperty?.layout || '待补充' }}</i><i><small>楼层</small>{{ previewPrimaryProperty?.floorInfo || '待补充' }}</i><i v-if="previewPrimaryProperty?.orientation"><small>朝向</small>{{ previewPrimaryProperty.orientation }}</i><i v-if="previewPrimaryProperty?.elevator"><small>电梯</small>{{ previewPrimaryProperty.elevator }}</i><i><small>租赁方式</small>{{ previewPrimaryProperty?.rentType || '待补充' }}</i></div>
                        </section>
                        <section v-if="previewPrimaryProperty?.depositPayment || previewPrimaryProperty?.feeText || previewPrimaryProperty?.utilitiesText || previewPrimaryProperty?.listingSource === '转租' || (previewPrimaryProperty?.rentType === '合租' && (previewPrimaryProperty?.layout || previewPrimaryProperty?.areaSqm))" class="detail-canvas-card detail-canvas-info"><b>{{ canvasPageCopy('租赁信息') }}</b><small class="detail-canvas-section-copy">{{ canvasPageCopy('费用与租期说明') }}</small><div><i v-if="previewPrimaryProperty?.depositPayment"><small>押付方式</small>{{ previewPrimaryProperty.depositPayment }}</i><i v-if="previewPrimaryProperty?.feeText"><small>中介费</small>{{ previewPrimaryProperty.feeText }}</i><i v-if="previewPrimaryProperty?.utilitiesText" class="wide"><small>水电杂费</small>{{ previewPrimaryProperty.utilitiesText }}</i><i v-if="previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.layout"><small>使用空间</small>{{ previewPrimaryProperty.layout }}</i><i v-if="previewPrimaryProperty?.rentType === '合租' && previewPrimaryProperty?.areaSqm"><small>使用面积</small>{{ previewPrimaryProperty.areaSqm }}㎡</i><i v-if="previewPrimaryProperty?.listingSource === '转租'"><small>剩余租期</small>{{ previewPrimaryProperty.remainingLeaseMonths || 0 }} 个月</i><i v-if="previewPrimaryProperty?.listingSource === '转租' && previewPrimaryProperty?.transferDeposit"><small>承接押金</small>{{ previewPrimaryProperty.transferDeposit }} 元</i></div></section>
                        <section v-if="detailModuleEnabled('location') && previewHasLocation" data-canvas-area="location" class="detail-canvas-card detail-canvas-location"><header><span><b>{{ detailModuleTitle('location', '位置与周边') }}</b><small>{{ previewPrimaryProperty?.location || '已保存房源定位' }}</small></span><i>已定位</i></header><button type="button" class="canvas-detail-map-toggle" @click="canvasMapExpanded = !canvasMapExpanded"><span><strong>{{ canvasMapExpanded ? '收起地图' : '查看地图位置' }}</strong><small>已保存房源定位</small></span><i>{{ canvasMapExpanded ? '⌃' : '›' }}</i></button><div v-if="canvasMapExpanded" class="canvas-detail-map"><div class="canvas-detail-map-grid"><span>⌖</span><b>{{ previewPrimaryProperty?.community || '房源位置' }}</b><small>{{ previewPrimaryProperty?.latitude || '—' }}，{{ previewPrimaryProperty?.longitude || '—' }}</small></div><button type="button" @click="setCanvasNotice('画布预览不会启动外部地图导航。')">⌁ 导航去这里</button></div></section>
                        <section v-if="detailModuleEnabled('facilities')" data-canvas-area="facilities" class="detail-canvas-card detail-canvas-facilities" @click="focusCanvasArea('facilities')"><header><span><b>{{ detailModuleTitle('facilities', '房屋设施') }}</b><small>{{ canvasPageCopy('家具家电等可勾选配置') }}</small></span><i>{{ previewDetailAmenities.length }} 项</i></header><template v-if="previewDetailFacilityGroups.length"><div v-for="group in previewDetailFacilityGroups" :key="group.title" class="detail-canvas-facility-group"><small>{{ group.title }}</small><div><i v-for="facility in group.items" :key="facility.key"><b><img :src="detailFacilityIcon(facility.key)" alt=""></b>{{ facility.label }}</i></div></div></template><p v-else>{{ canvasPageCopy("家具家电信息待补充，可联系小皮确认") }}</p></section>
                        <section v-if="detailModuleEnabled('description')" data-canvas-area="description" class="detail-canvas-card detail-canvas-description" @click="focusCanvasArea('description')"><b>{{ detailModuleTitle('description', '房源简介') }}</b><small class="detail-canvas-section-copy">{{ canvasPageCopy('房东补充说明与居住规则') }}</small><p>{{ (previewCopyScenario === '暂无描述' ? '' : previewDescription(previewPrimaryProperty)) || canvasPageCopy("暂无描述") }}</p></section>
                        <section v-if="detailModuleEnabled('nearby') && previewHasLocation" data-canvas-area="nearby" class="detail-canvas-card detail-canvas-nearby"><header><span><b class="canvas-editable-copy" @click="focusCanvasArea('nearby')">{{ detailModuleTitle('nearby', '附近房源') }}</b></span><i>{{ previewNearbyProperties.length ? `${previewNearbyProperties.length} 套` : '继续找房' }}</i></header><article v-for="property in previewNearbyProperties" :key="property.id" @click="openPreviewProperty(property)"><div :style="previewCoverStyle(property)"><img v-if="property.media?.some(media => media.type === 'image')" :src="property.media.find(media => media.type === 'image')?.url" alt=""></div><span><b>{{ previewListingTitle(property) }}</b><small>{{ property.layout || property.community }}</small><em>{{ previewTagGroups(property).all.slice(0, 3).join(' · ') }}</em><strong>¥{{ property.rentMonthly }}<i>/月</i><small>{{ property.distance < 1 ? `${Math.round(property.distance * 1000)}m` : `${property.distance.toFixed(1)}km` }}</small></strong></span></article><button v-if="previewNearbyProperties.length" type="button" class="detail-canvas-nearby-more" @click="navigateCanvasPage('properties')">查看更多房源 ›</button><div v-if="!previewNearbyProperties.length" class="canvas-nearby-empty-state"><strong>{{ canvasPageCopy('附近暂时没有其他可租房源') }}</strong><small>{{ canvasPageCopy('可以继续查看其他地区的新房源') }}</small><button type="button" @click="navigateCanvasPage('properties')">查看全部房源</button></div></section>
                      </div>
                      <template v-if="configAudience === 'customer'"><button class="detail-canvas-floating-appointment" type="button" @click="canvasIdentityState === 'guest' ? navigateCanvasPage('authLogin') : (canvasDetailModal = 'appointment')"><b>预约</b><small>看房</small></button><button class="detail-canvas-floating-share" type="button" @click="selectDetailConfigTab('share')"><img :src="propertyShareIcon" alt="">分享</button><div class="detail-canvas-actions customer-actions"><span class="detail-canvas-signature"><i>皮</i><b>{{ publicConfig.heroTitle || '小皮租房' }}</b></span><button type="button" @click="setCanvasNotice('预览不会发起客服会话。')">在线咨询</button><button type="button" @click="setCanvasNotice(`客服电话：${publicConfig.contactPhone || '暂未配置'}`)">打电话</button></div></template>
                      <div v-else class="detail-canvas-actions operator-actions"><div class="detail-canvas-admin-lead"><button type="button" v-if="previewPrimaryProperty?.status === 'available'" @click="selectDetailConfigTab('share')">分享</button><button type="button" @click="startCanvasPropertyEdit(previewPrimaryProperty)">编辑</button></div><div class="detail-canvas-admin-status"><button v-for="(action, index) in previewAdminStatusActions" :key="action.status" type="button" :class="action.tone" @click="submitCanvasPreview"><span>{{ action.label }}</span><small v-if="index === 0">（当前{{ previewPrimaryProperty?.status === 'pending_review' ? '待审核' : previewPrimaryProperty?.status === 'published' ? '待设为可租' : previewPrimaryProperty?.status === 'offline' ? '已下架' : previewPrimaryProperty?.status === 'rented' ? '已租' : previewPrimaryProperty?.status === 'draft' ? '草稿' : '可租' }}）</small></button><span v-if="!previewAdminStatusActions.length" class="detail-canvas-admin-no-transition">当前{{ previewPrimaryProperty?.status === 'pending_review' ? '待审核' : '草稿' }} · {{ previewPrimaryProperty?.status === 'draft' ? '完善后即可发布' : '无需变更' }}</span></div><button class="detail-canvas-delete" type="button" @click="setCanvasNotice('预览不会删除房源。')">删除房源</button></div>
                    </div>
                  </template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'requests'"><section class="request-canvas-hero canvas-editable-copy" title="点击编辑页面文案" @click="openCanvasNode('page.heading')"><strong>{{ publicConfig.pagePresentations.requests.title || '小皮帮你找房' }}</strong><b data-canvas-copy="request-guide" @click.stop="focusConfigField('request-guide')">{{ canvasPageCopy('说清需求，比反复筛选更高效') }}</b><small>{{ publicConfig.pagePresentations.requests.subtitle || '告诉我们你的找房需求' }}</small></section><form class="preview-form-card canvas-form request-canvas-form" @submit.prevent="submitCanvasPreview"><div v-if="previewCopyScenario === '例如：张女士'" class="request-canvas-field"><label>怎么称呼您 *</label><input v-model="canvasForm.contactName" :placeholder="canvasPageCopy('例如：张女士')"></div><div v-if="previewCopyScenario === '例如：工作日晚上、周末上午，或想先了解的问题'" class="request-canvas-field"><label>方便看房时间 *</label><input v-model="canvasForm.preferredTime" :placeholder="canvasPageCopy('例如：工作日晚上、周末上午，或想先了解的问题')"></div><div class="request-canvas-field"><label>联系手机号 *</label><span class="request-canvas-phone" :class="{ 'is-bound': canvasIdentityState === 'user' || canvasForm.phone }">{{ canvasIdentityState === 'user' ? '已绑定微信手机号' : canvasForm.phone ? `已绑定 ${canvasForm.phone.slice(0, 3)}****${canvasForm.phone.slice(-4)}` : '提交时需绑定微信手机号' }}</span></div><div class="request-canvas-field"><label>意向地区 *</label><input v-model="canvasForm.location" :placeholder='canvasPageCopy("例如：佛祖岭、金融港")'></div><div class="request-canvas-field"><label>期望户型</label><input v-model="canvasForm.layout" :placeholder='canvasPageCopy("例如：一室一厅 / 合租主卧")'></div><div class="request-canvas-field"><label>月租预算</label><div class="canvas-budget-inputs"><input v-model="canvasForm.budgetMin" inputmode="numeric" placeholder="最低"><span>至</span><input v-model="canvasForm.budgetMax" inputmode="numeric" placeholder="最高"><span>元</span></div></div><div class="request-canvas-field"><label>预计入住</label><button class="canvas-picker-field" type="button" @click="canvasDatePickerValue = canvasForm.moveInDate || localDateString(); canvasDatePickerOpen = true">{{ canvasForm.moveInDate || '选择日期' }} <i>›</i></button></div><div class="request-canvas-field request-canvas-textarea"><label>补充要求</label><textarea v-model="canvasForm.note" maxlength="300" :placeholder="canvasPageCopy('通勤地点、整租/合租、楼层、宠物、采光等')"></textarea></div></form><small class="request-canvas-privacy">提交即表示同意小皮使用已绑定手机号联系你，仅用于本次找房服务。</small><button class="request-canvas-submit" type="button" @click="submitCanvasPreview">提交找房需求</button><button class="request-canvas-contact" type="button" @click="setCanvasNotice('联系入口只用于预览，不会拨打电话。')">想马上确认房源？联系小皮 ›</button></template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'publish'"><div class="publish-canvas-tip"><strong>提供房源线索</strong><span>提交基础信息后，小皮会电话联系并实地核验。正式房源由运营人员审核发布。</span></div><form class="preview-form-card canvas-form publish-canvas-form" @submit.prevent="submitCanvasPreview"><label>联系人</label><input v-model="canvasForm.sourceName" :placeholder='canvasPageCopy("怎么称呼你")'><label>联系电话</label><input v-model="canvasForm.sourcePhone" inputmode="tel" maxlength="11" placeholder="方便小皮联系"><label>小区</label><input v-model="canvasForm.community" :placeholder='canvasPageCopy("请输入小区名称")'><label>位置</label><input v-model="canvasForm.location" :placeholder='canvasPageCopy("区域 / 地铁 / 商圈")'><label>户型</label><input v-model="canvasForm.layout" :placeholder='canvasPageCopy("例如：一室一厅")'><label>预期租金</label><input v-model="canvasForm.expectedRent" inputmode="numeric" placeholder="请输入租金"><label>补充说明</label><textarea v-model="canvasForm.note" maxlength="500" placeholder="入住时间、房屋情况等"></textarea></form><button type="button" class="publish-submit-button" @click="submitCanvasPreview">提交房源线索</button></template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'me'">
                    <div v-if="canvasNotice" class="canvas-preview-notice" role="status">{{ canvasNotice }}</div>
                    <div v-if="configAudience === 'customer'" class="preview-profile" :class="{ guest: canvasIdentityState === 'guest', 'logged-in': canvasIdentityState !== 'guest' }" @click="canvasIdentityState === 'guest' && navigateCanvasPage('authLogin')">
                      <div class="preview-profile-main">
                        <i :class="{ 'canvas-avatar-preview-trigger': canvasIdentityState !== 'guest' }" @click.stop="canvasIdentityState !== 'guest' && (canvasAvatarCropOpen = true)">{{ previewIdentity.mark }}</i>
                        <div v-if="canvasIdentityState === 'guest'"><b>{{ previewIdentity.name }}</b><span>{{ previewIdentity.caption }}</span></div>
                        <div v-else class="preview-profile-user-copy"><b>{{ previewIdentity.name }}</b><small>普通用户</small><span>已绑定手机号</span></div>
                        <small>{{ canvasIdentityState === 'guest' ? '›' : '' }}</small>
                      </div>
                      <div class="preview-profile-bottom"><span>{{ canvasIdentityState === 'guest' ? '点击直接微信登录' : '点头像或昵称即可修改资料' }}</span><b>{{ canvasIdentityState === 'guest' ? '进入' : '已登录' }}</b></div>
                    </div>
                    <template v-if="configAudience === 'customer'">
                      <h4><strong>{{ canvasPageCopy("我的找房") }}</strong><span>{{ canvasPageCopy("常用服务") }}</span></h4>
                      <div class="preview-tool-grid">
                        <article @click="navigateCanvasPage('properties')"><div class="preview-tool-copy"><b>全部房源</b><small>{{ canvasPageCopy("无需登录") }}</small></div></article>
                        <article @click="canvasIdentityState === 'guest' ? requireCanvasLogin('登录后查看关注', '关注记录会同步到你的微信账号', 'authLogin') : (canvasFavoriteOnly = true, navigateCanvasPage('properties'))"><div class="preview-tool-copy"><b>我的关注</b><small>{{ canvasIdentityState === 'guest' ? canvasPageCopy("登录后查看") : '收藏的房源' }}</small></div></article>
                        <article @click="canvasIdentityState === 'guest' ? requireCanvasLogin('登录后查看足迹', '登录后可查看最近浏览过的房源', 'authLogin') : navigateCanvasPage('footprints')"><div class="preview-tool-copy"><b>浏览足迹</b><small>{{ canvasIdentityState === 'guest' ? canvasPageCopy("登录后查看") : '最近看过' }}</small></div></article>
                        <article @click="navigateCanvasPage('requests')"><div class="preview-tool-copy"><b>找房需求</b><small>{{ canvasPageCopy("提交时登录") }}</small></div></article>
                      </div>
                      <article class="preview-menu-row" @click="canvasIdentityState === 'guest' ? requireCanvasLogin('登录后查看找房记录', '预约、订阅和找房需求会同步到你的微信账号', 'authLogin') : navigateCanvasPage('requestHistory')"><span>{{ canvasPageCopy("我的预约与找房记录") }}</span><i>›</i><small>{{ canvasIdentityState === 'guest' ? '登录后可查看' : '查看预约、订阅和跟进进度' }}</small></article>
                      <article class="preview-menu-row" @click="navigateCanvasPage('publish')"><span>{{ canvasPageCopy("有房源想推荐？") }}</span><i>›</i><small>无需登录，提交后电话核验</small></article>
                      <button v-if="canvasIdentityState === 'user'" class="canvas-user-logout" type="button" @click="canvasIdentityState = 'guest'">退出登录</button>
                    </template>
                    <template v-else>
                      <div class="canvas-admin-identity"><i>管</i><div><b>{{ previewIdentity.name }}</b> <small>管理员</small><span>{{ canvasForm.phone ? '已绑定 ' + canvasForm.phone.replace(/^(...).*?(....)$/, '$1****$2') : '运营身份已启用' }}</span></div><button type="button" @click="submitCanvasPreview">可编辑</button></div>
                      <div class="canvas-admin-heading"><strong>运营工作台</strong><button type="button" @click="canvasAdminWorkbenchTab = 'requests'; navigateCanvasPage('adminWorkbench')">查看全部 ›</button></div>
                      <div class="preview-stats">
                        <button @click="canvasStatusFilter = 'draft'; navigateCanvasPage('properties')">{{ properties.filter(item => item.status === 'draft').length }}<small>房源草稿</small></button>
                        <button @click="canvasAdminWorkbenchTab = 'requests'; navigateCanvasPage('adminWorkbench')">{{ previewAdminPendingCount }}<small>待联系</small></button>
                        <button @click="canvasStatusFilter = 'incomplete'; navigateCanvasPage('properties')">{{ previewIncompletePropertyCount }}<small>待补资料</small></button>
                      </div>
                      <div class="canvas-admin-shortcuts">
                        <button @click="startCanvasPropertyEdit()"><i>＋</i>录入房源<small>{{ canvasPageCopy("发布可租房源") }}</small></button>
                        <button @click="canvasStatusFilter = 'all'; navigateCanvasPage('properties')"><i>▦</i>房源管理<small>{{ canvasPageCopy("查看和编辑房源") }}</small></button>
                        <button @click="navigateCanvasPage('adminMaterialLibrary')"><i>▧</i>房源素材库<small>快速保存与整理素材</small></button>
                        <button @click="canvasAdminWorkbenchTab = 'requests'; navigateCanvasPage('adminWorkbench')"><i>◌</i>客户需求<small>{{ canvasPageCopy("预约与找房需求") }}</small></button>
                        <button @click="canvasAdminWorkbenchTab = 'interests'; navigateCanvasPage('adminWorkbench')"><i>♡</i>用户意向<small>{{ favoriteContacts.length }} 条关注记录</small></button>
                      </div>
                      <div class="canvas-admin-session"><button @click="navigateCanvasPage('authLogin')">访问码登录</button><button @click="submitCanvasPreview">退出登录</button></div>
                    </template>
                  </template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'footprints'">
                    <div v-if="canvasIdentityState === 'user' && canvasFootprints.length && !(tab === '小程序页面配置' && pageEditorSection === 'footprints-empty')" class="canvas-footprints-list">
                      <button class="canvas-footprints-clear" type="button" @click="canvasFootprintClearOpen = true">清空</button>
                      <button v-for="entry in canvasFootprints" :key="entry.property.id" type="button" class="canvas-footprint-card" @click="openPreviewProperty(entry.property)">
                        <span class="canvas-footprint-cover" :style="previewCoverStyle(entry.property)"><i v-if="entry.property.media?.[0]?.type === 'video'">▶</i></span>
                        <span class="canvas-footprint-copy"><strong>{{ entry.property.community || entry.property.title }}</strong><span>{{ entry.property.layout }} · {{ entry.property.location }}</span><span><b>{{ entry.property.rentMonthly }}</b> 元/月</span><small>{{ entry.count }} 次浏览 · {{ entry.viewedAt.toLocaleDateString() }}</small></span>
                      </button>
                    </div>
                    <section v-else class="canvas-private-page" :class="{ 'canvas-user-empty': canvasIdentityState === 'user' }"><template v-if="canvasIdentityState === 'guest'"><strong>{{ canvasPageCopy("登录后查看浏览足迹") }}</strong><small>{{ canvasPageCopy("最近看过的房源会保存在你的账号中。") }}</small><button type="button" @click="navigateCanvasPage('authLogin')">微信登录</button></template><template v-else-if="canvasIdentityState === 'admin'"><strong>{{ canvasPageCopy("登录后查看浏览足迹") }}</strong><small>{{ canvasPageCopy("最近看过的房源会保存在你的账号中。") }}</small><button type="button" @click="navigateCanvasPage('authLogin')">微信登录</button></template><template v-else><strong>{{ canvasPageCopy("还没有浏览记录") }}</strong></template></section></template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'requestHistory'"><section class="canvas-private-page" :class="{ 'canvas-user-empty': canvasIdentityState === 'user' }"><template v-if="canvasIdentityState === 'guest'"><strong>{{ canvasPageCopy("登录后查看找房记录") }}</strong><small>{{ canvasPageCopy("预约和找房需求会保存在你的微信账号中。") }}</small><button type="button" @click="navigateCanvasPage('authLogin')">微信登录</button></template><template v-else-if="canvasIdentityState === 'admin'"><strong>当前 Admin 未绑定微信用户身份</strong><small>个人预约和找房记录需要对应微信用户身份读取；画布不生成示例记录。</small><button type="button" @click="navigateCanvasPage('requests')">填写找房需求</button></template><template v-else><strong>{{ canvasPageCopy("暂时没有找房记录") }}</strong><small>{{ canvasPageCopy("提交需求或预约看房后，会在这里查看进度。") }}</small><button type="button" @click="navigateCanvasPage('requests')">{{ canvasPageCopy('去找房') }}</button></template></section></template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'adminWorkbench'">
                    <div v-if="canvasNotice" class="canvas-preview-notice" role="status">{{ canvasNotice }}</div>
                    <div class="preview-admin-card"><b>房源总览</b><button type="button" @click="load().then(loadCanvasWorkbench).catch(() => undefined)">刷新</button></div>
                    <div class="preview-stats"><button type="button" @click="canvasStatusFilter = 'draft'; navigateCanvasPage('properties')">{{ properties.filter(item => item.status === 'draft').length }}<small>草稿</small></button><button type="button" @click="canvasAdminWorkbenchTab = 'leads'">{{ previewAdminPendingCount }}<small>待联系</small></button><button type="button" @click="canvasStatusFilter = 'incomplete'; navigateCanvasPage('properties')">{{ previewIncompletePropertyCount }}<small>待补资料</small></button></div>
                    <article class="canvas-work-activity"><header><strong>操作记录</strong><button type="button" @click="canvasOperationLogsOpen = true; canvasOperationLogsPage = 1">查看全部 ›</button></header><button v-for="item in canvasWorkbenchActivities.slice(0,3)" :key="item.id" type="button" @click="canvasOperationLogsOpen = true; canvasOperationLogsPage = 1">{{ item.action }} · {{ item.detail || item.targetType }}</button><p v-if="!canvasWorkbenchActivities.length">暂无操作记录</p></article>
                    <nav class="preview-admin-tabs"><button type="button" :class="{ active: canvasAdminWorkbenchTab === 'requests' }" @click="canvasAdminWorkbenchTab = 'requests'">客户需求 {{ rentalRequests.length }}</button><button type="button" :class="{ active: canvasAdminWorkbenchTab === 'interests' }" @click="canvasAdminWorkbenchTab = 'interests'">用户意向 {{ favoriteContacts.length + canvasWorkbenchContacts.length }}</button><button type="button" :class="{ active: canvasAdminWorkbenchTab === 'leads' }" @click="canvasAdminWorkbenchTab = 'leads'">房源线索 {{ leads.length }}</button></nav>
                    <div v-if="canvasWorkbenchError" class="preview-empty">{{ canvasWorkbenchError }}</div>
                    <section v-else-if="canvasAdminWorkbenchTab === 'leads'" class="canvas-workbench-list">
                      <article v-for="item in previewWorkbenchCopy === '暂无房源线索' ? [] : leads" :key="item.id" class="canvas-operator-work-card">
                        <header><strong>{{ item.community }}</strong><small>{{ previewOperationalStatus(item.status, 'lead') }}</small></header>
                        <small>{{ item.sourceName || '未填写姓名' }} · {{ item.sourcePhone }}</small>
                        <small>{{ item.location }} · {{ item.layout }} · {{ item.expectedRentMonthly }} 元/月</small>
                        <p>{{ previewWorkbenchCopy === '暂无补充说明' ? canvasPageCopy("暂无补充说明") : item.note || canvasPageCopy("暂无补充说明") }}</p>
                        <footer><button @click="submitCanvasPreview">联系客户</button><button @click="submitCanvasPreview">标记已联系</button><button @click="submitCanvasPreview">转为房源草稿</button></footer>
                      </article>
                      <div v-if="previewWorkbenchCopy === '暂无房源线索' || !leads.length" class="preview-empty">{{ canvasPageCopy("暂无房源线索") }}</div>
                    </section>
                    <section v-else-if="canvasAdminWorkbenchTab === 'requests'" class="canvas-workbench-list">
                      <article v-for="item in previewWorkbenchCopy === '暂无客户需求' ? [] : rentalRequests" :key="item.id" class="canvas-operator-work-card">
                        <header><strong>{{ item.requestType === 'appointment' ? '预约看房' : item.requestType === 'area_subscription' ? '片区新房预约' : '找房需求' }}</strong><small>{{ previewOperationalStatus(item.status, 'request') }}</small></header>
                        <div v-if="item.requestType === 'appointment'" class="canvas-work-request-property" :class="{ unavailable: !properties.some(property => property.id === item.propertyId) }" @click="properties.find(property => property.id === item.propertyId) ? openPreviewProperty(properties.find(property => property.id === item.propertyId)!) : submitCanvasPreview()"><span>预约房源</span><strong>{{ item.propertyTitle || item.location || '房源信息待确认' }}</strong><small>{{ properties.some(property => property.id === item.propertyId) ? '查看房源 ›' : '房源已下架或无法查看' }}</small></div>
                        <small v-if="item.requestType === 'appointment'">{{ item.contactName || '称谓待补充' }} · {{ item.preferredTime || '时间待确认' }}</small>
                        <small>{{ item.location || '地区待确认' }} · {{ item.layout || '户型不限' }}</small>
                        <div class="canvas-work-meta"><span>¥ {{ item.budgetMin || 0 }}–{{ item.budgetMax || '不限' }} / 月</span><span>{{ item.moveInDate || '入住时间待定' }}</span></div>
                        <p v-if="item.note">{{ item.note }}</p>
                        <footer><button @click="submitCanvasPreview">联系客户</button><button @click="submitCanvasPreview">标记已联系</button><button @click="canvasStatusFilter = 'all'; navigateCanvasPage('properties')">匹配房源</button></footer>
                      </article>
                      <div v-if="previewWorkbenchCopy === '暂无客户需求' || !rentalRequests.length" class="preview-empty">{{ canvasPageCopy("暂无客户需求") }}</div>
                    </section>
                    <section v-else class="canvas-workbench-list">
                      <article v-for="item in previewWorkbenchCopy === '暂无用户意向' ? [] : canvasWorkbenchContacts" :key="item.id" class="canvas-operator-work-card"><header><strong>{{ item.propertyName || '房源' }}</strong><small>{{ item.type === 'phone_entry_click' ? '拨号入口点击' : '咨询入口点击' }}</small></header><small>{{ item.nickname || '微信用户' }} · {{ item.phone || '手机号待绑定' }}</small><div class="canvas-work-meta"><span>{{ item.source === 'detail' ? '详情页触发' : item.source || '页面触发' }}</span><span>{{ item.createdAt ? new Date(item.createdAt).toLocaleString() : '' }}</span></div><footer><button :disabled="!item.phone" @click="submitCanvasPreview">{{ item.phone ? '联系用户' : '等待绑定手机号' }}</button></footer></article>
                      <article v-for="item in previewWorkbenchCopy === '暂无用户意向' ? [] : favoriteContacts" :key="`${item.userId}-${item.propertyId}`" class="canvas-operator-work-card"><header><strong>{{ item.propertyName }}</strong><small>已关注</small></header><small>{{ item.nickname || '微信用户' }}</small><div class="canvas-work-meta"><span>{{ item.phone || '手机号待绑定' }}</span><span>{{ new Date(item.createdAt).toLocaleDateString() }}</span></div><footer><button :disabled="!item.phone" @click="submitCanvasPreview">{{ item.phone ? '联系用户' : '等待绑定手机号' }}</button></footer></article><div v-if="previewWorkbenchCopy === '暂无用户意向' || (!favoriteContacts.length && !canvasWorkbenchContacts.length)" class="preview-empty">{{ canvasPageCopy('暂无用户意向') }}</div>
                    </section>
                  </template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'adminMaterialLibrary'">
                    <div class="canvas-material-summary"><div><strong>{{ canvasMaterials.length }}</strong><small>素材</small></div><div><strong>{{ canvasMaterialCategoryOptions.length }}</strong><small>分类</small></div><div><strong>{{ canvasMaterials.reduce((sum, item) => sum + (item.media?.length || 0), 0) }}</strong><small>媒体</small></div></div>
                    <section v-if="canvasMaterialFormOpen" class="canvas-material-entry"><header><strong>新增素材</strong><button type="button" @click="canvasMaterialFormOpen = false">收起</button></header><label>小区分类<select v-model="canvasMaterialDraft.community"><option value="">选择分类</option><option v-for="name in canvasMaterialCategoryOptions" :key="name" :value="name">{{ name }}</option></select></label><div class="canvas-material-media-preview"><div v-if="canvasMaterialSampleMedia && previewPropertyOptions.find(item => item.media?.length)?.media?.[0]" :style="previewCoverStyle(previewPropertyOptions.find(item => item.media?.length), 0, true)"><span>媒体示例</span></div><button type="button" @click="canvasMaterialSampleMedia = !canvasMaterialSampleMedia">{{ canvasMaterialSampleMedia ? '移除示例媒体' : '＋ 选择预览媒体' }}</button><small>画布不上传文件，仅预览素材布局</small></div><div class="canvas-material-form-grid"><label>出租方式<select v-model="canvasMaterialDraft.rentType"><option>整租</option><option>合租</option></select></label><label>户型<input v-model="canvasMaterialDraft.layout" placeholder="如：2室1厅"></label><label>月租<input v-model="canvasMaterialDraft.rentMonthly" inputmode="numeric" placeholder="元/月"></label><label>面积<input v-model="canvasMaterialDraft.areaSqm" inputmode="decimal" placeholder="㎡"></label><label class="wide">素材名称<input v-model="canvasMaterialDraft.title" placeholder="默认使用小区名称"></label><label class="wide">备注<textarea v-model="canvasMaterialDraft.description" placeholder="记录房源特点"></textarea></label></div><button class="canvas-material-save" type="button" @click="saveCanvasMaterialDraft">保存素材预览</button></section>
                    <template v-else>
                      <nav class="canvas-material-toolbar"><button type="button" :class="{ active: !canvasMaterialShowConverted }" @click="canvasMaterialShowConverted = false">素材库</button><button type="button" :class="{ active: canvasMaterialShowConverted }" @click="canvasMaterialShowConverted = true">已转房源</button><button type="button" :class="{ active: canvasMaterialSelectedCategory === '全部' }" @click="canvasMaterialSelectedCategory = '全部'">全部分类</button><button v-for="name in canvasMaterialCategoryOptions" :key="name" type="button" :class="{ active: canvasMaterialSelectedCategory === name }" @click="canvasMaterialSelectedCategory = canvasMaterialSelectedCategory === name ? '全部' : name">{{ name }}</button><button type="button" class="canvas-material-manage" @click="openCanvasMaterialCategories">分类管理</button><button type="button" class="canvas-material-new" @click="beginCanvasMaterialCreate">＋ 新增素材</button></nav>
                      <div v-if="canvasFilteredMaterials.length && !canvasMaterialShowConverted" class="canvas-material-layouts"><strong>户型</strong><button v-for="name in ['全部','一室','两室','三室','四室','五室','六室+']" :key="name" type="button" :class="{ active: canvasMaterialLayout === name }" @click="canvasMaterialLayout = name">{{ name }}</button></div>
                      <label v-if="canvasFilteredMaterials.length" class="canvas-material-price-toggle"><input v-model="canvasMaterialShowPrice" type="checkbox"> 显示价格</label>
                      <article v-for="item in canvasFilteredMaterials" :key="item.id" class="canvas-material-card"><div class="canvas-material-cover" :style="previewCoverStyle(item, 0, true)"><span v-if="!item.media?.length">暂无图片或视频</span><em>{{ item.media?.some(media => media.type === 'video') ? '含视频' : `${item.media?.filter(media => media.type === 'image').length || 0} 图` }}</em></div><section><b>{{ item.title || item.community }}</b><small>{{ canvasMaterialCommunity(item) }} · {{ item.layout || '户型待补' }}<template v-if="canvasMaterialShowPrice"> · {{ item.rentMonthly }}元/月</template></small><small v-if="item.convertedPropertyId" class="canvas-material-converted">已转为房源草稿，素材已保留</small><footer><button type="button" @click="convertCanvasMaterial(item)">{{ item.convertedPropertyId ? '查看房源草稿' : '转为房源草稿' }}</button><button type="button" @click="deleteCanvasMaterial(item)">删除</button></footer></section></article>
                      <div v-if="!canvasFilteredMaterials.length" class="canvas-material-empty"><strong>{{ canvasMaterials.length ? (canvasMaterialShowConverted ? '还没有已转房源的素材' : '当前筛选下没有素材') : '还没有素材' }}</strong><button type="button" @click="beginCanvasMaterialCreate">＋ 新增素材</button><small>预览中的操作不会修改服务器数据</small></div>
                    </template>
                    <div v-if="canvasMaterialCategoryDialogOpen" class="canvas-material-manager-mask" @click="canvasMaterialCategoryDialogOpen = false"><section class="canvas-material-manager" role="dialog" aria-modal="true" aria-label="小区分类管理" @click.stop><header><strong>小区分类</strong><button type="button" aria-label="关闭" @click="canvasMaterialCategoryDialogOpen = false">×</button></header><label v-for="name in canvasMaterialCategoryOptions" :key="name"><input v-model="canvasMaterialCategoryDrafts[name]" :aria-label="`修改${name}分类名`"><button type="button" @click="renameCanvasMaterialCategory(name)">保存</button><small>{{ canvasMaterials.filter(item => canvasMaterialCommunity(item) === name).length }} 份素材</small></label><p v-if="!canvasMaterialCategoryOptions.length">还没有小区分类</p><div class="canvas-material-add-category"><input v-model="canvasMaterialNewCategory" placeholder="输入小区名称"><button type="button" @click="addCanvasMaterialCategory">＋ 添加分类</button></div><small>画布内修改仅用于预览，不会改动真实分类。</small></section></div>
                  </template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'adminPropertyEdit'">
                    <div v-if="canvasNotice" class="canvas-preview-notice" role="status">{{ canvasNotice }}</div>
                    <div class="canvas-editor-hero"><div><b>{{ canvasEditingExistingProperty ? canvasPageCopy("完善房源信息") : canvasPageCopy("开始录入房源") }}</b></div><strong>{{ canvasPropertyCompletionCount }}/2<small>已完成</small></strong></div>
                    <nav class="preview-steps" aria-label="房源编辑步骤">
                      <button v-for="(step, index) in canvasEditSteps" :key="step" type="button" :class="{ active: canvasEditStep === index, done: canvasPropertyStepReady[index] }" @click="canvasEditStep = index"><i>{{ canvasPropertyStepReady[index] ? '✓' : index + 1 }}</i><span>{{ step }}</span></button>
                    </nav>
                    <div class="preview-edit-card">
                      <template v-if="canvasEditStep === 0">
                        <div class="canvas-section-head"><b>01</b><span><strong>基础信息</strong></span></div>
                        <div class="canvas-edit-community-heading"><label>位置与门牌</label></div>
                        <button type="button" class="canvas-community-select" @click="openCanvasCommunityPicker"><span><b>{{ canvasPropertyDraft.community || '选择小区' }}</b><small>{{ canvasPropertyDraft.community ? canvasPropertyDraft.location || '位置待补充' : '小区库优先；找不到可在弹窗内搜索地图' }}</small></span><strong>{{ canvasPropertyDraft.community ? '更换 ›' : '选择 ›' }}</strong></button>
                        <button v-if="canvasPropertyDraft.community && (!canvasPropertyDraft.latitude || !canvasPropertyDraft.longitude)" type="button" class="canvas-community-repair" @click="openCanvasCommunityPicker(); canvasCommunityQuery = canvasPropertyDraft.community">补充小区位置</button>
                        <div class="canvas-edit-field canvas-property-title-field"><label>房源名称</label><input v-model="canvasPropertyDraft.title" maxlength="40" placeholder="默认使用小区名称，可自定义"></div>
                        <button v-if="canvasPropertyDraft.community" type="button" class="canvas-doorplate-select" @click="openCanvasDoorplatePicker"><span><b>{{ canvasPropertyDraft.buildingNo || canvasPropertyDraft.unitNo || canvasPropertyDraft.roomNo ? '已选门牌' : '门牌信息（选填）' }}</b><small>{{ [canvasPropertyDraft.buildingNo, canvasPropertyDraft.unitNo && `${canvasPropertyDraft.unitNo}单元`, canvasPropertyDraft.roomNo && `${canvasPropertyDraft.roomNo}室`].filter(Boolean).join('') || '选择已有门牌或自定义输入' }}</small></span><strong>{{ canvasPropertyDraft.roomNo ? '重选 ›' : '选择 ›' }}</strong></button>
                        <div class="canvas-area-heading"><b>归属地区</b><small>保存后按当前选择归类</small></div>
                        <button type="button" class="canvas-area-choice" @click="canvasPropertyDraft.channel = canvasPropertyDraft.channel ? '' : (areaEntryChannels[0]?.title || '')"><span>{{ canvasPropertyDraft.channel || areaEntryChannels[0]?.title || '选择归属地区' }}<small>选择小区后推荐最近地区</small></span><strong>调整 ›</strong></button>
                        <button type="button" class="canvas-area-choice" @click="canvasQuickSettingsOpen = !canvasQuickSettingsOpen"><span><b>房屋与出租信息</b><small>{{ canvasPropertyDraft.layout || '设置户型、出租方式和房间序号' }}</small></span><strong>{{ canvasQuickSettingsOpen ? '收起' : '快捷设置 ›' }}</strong></button>
                        <div v-if="canvasQuickSettingsOpen" class="canvas-quick-settings">
                          <div class="canvas-rent-type-tabs"><button v-for="kind in ['整租', '合租']" :key="kind" :class="{ active: canvasPropertyDraft.rentType === kind }" @click="canvasPropertyDraft.rentType = kind; canvasPropertyDraft.layoutChosen = ''">{{ kind }}<small>{{ kind === '整租' ? '发布整套房源' : '发布单个房间' }}</small></button></div>
                          <div class="canvas-listing-source"><span>出租来源</span><div class="canvas-editor-options"><button v-for="source in ['直租', '转租']" :key="source" :class="{ active: canvasPropertyDraft.listingSource === source }" @click="canvasPropertyDraft.listingSource = source">{{ source }}</button></div></div>
                          <div class="canvas-layout-options"><div v-for="row in [{ key:'rooms', label:canvasPropertyDraft.rentType === '合租' ? '整套室数' : '房间', values:['1','2','3','4','手动'] },{key:'halls',label:'客厅',values:['0','1','2','3']},{key:'kitchens',label:'厨房',values:['有','无']},{key:'bathrooms',label:'卫生间',values:['1','2','3','4']},{key:'balconies',label:'阳台',values:['无','1','2','3','4']} ]" :key="row.key" class="canvas-editor-option-row"><label>{{ row.label }}</label><div class="canvas-editor-options"><button v-for="value in row.values" :key="value" :class="{ active: canvasPropertyDraft[row.key] === value }" @click="updateCanvasWholeLayout(row.key, value)">{{ value === '0' || value === '无' ? '无' : value === '手动' ? '自定义' : row.key === 'rooms' ? `${value}室` : row.key === 'halls' ? `${value}厅` : row.key === 'bathrooms' ? `${value}卫` : row.key === 'balconies' ? `${value}个` : value }}</button><input v-if="row.key === 'rooms' && canvasPropertyDraft.rooms === '手动'" v-model="canvasPropertyDraft.customRooms" type="number" min="1" max="9" placeholder="1–9室" @input="updateCanvasWholeLayout('customRooms', canvasPropertyDraft.customRooms)"></div></div></div>
                          <div v-if="canvasPropertyDraft.rentType === '合租'" class="canvas-shared-room"><small>本次出租房型与房间序号</small><div class="canvas-editor-options"><button v-for="room in ['主卧','次卧','单间','床位']" :key="room" :class="{ active: canvasPropertyDraft.roomType === room }" @click="canvasPropertyDraft.roomType = room; canvasPropertyDraft.layout = [room, canvasPropertyDraft.privateBathroom ? '独卫' : '', canvasPropertyDraft.hasBayWindow ? '有飘窗' : ''].filter(Boolean).join(' · ')">{{ room }}</button></div><div v-if="canvasLayoutReady" class="canvas-editor-options"><button :class="{ active: !Number(canvasPropertyDraft.roomSequence) }" @click="canvasPropertyDraft.roomSequence = '0'">不指定</button><button v-for="number in canvasSelectedRoomCount" :key="number" :class="{ active: Number(canvasPropertyDraft.roomSequence) === number }" @click="canvasPropertyDraft.roomSequence = String(number)">{{ number }}号房</button></div></div>
                          <div class="canvas-editor-details"><label>面积（㎡）<input v-model="canvasPropertyDraft.areaSqm" type="number" placeholder="平方米"></label><label>楼层<input v-model="canvasPropertyDraft.floorInfo" placeholder="如：33楼"></label><label>朝向<input v-model="canvasPropertyDraft.orientation" placeholder="如：南"></label><label>步行至地铁（分钟）<input v-model="canvasPropertyDraft.metroMinutes" type="number" placeholder="分钟"></label></div>
                          <div class="canvas-editor-details canvas-quick-rent"><label>月租（元/月）<input v-model="canvasPropertyDraft.rentMonthly" type="number" placeholder="可填 0"></label><label>可入住日期<input v-model="canvasPropertyDraft.availableDate" placeholder="默认拎包入住"></label></div>
                          <div class="canvas-listing-source"><span>押付方式</span><div class="canvas-editor-options"><button v-for="cycle in ['押一付一','押一付二','押一付三','押一付六']" :key="cycle" :class="{ active: canvasPropertyDraft.depositPayment === cycle }" @click="canvasPropertyDraft.depositPayment = cycle">{{ cycle }}</button></div></div>
                          <div class="canvas-listing-source"><span>中介费与出租来源</span><div class="canvas-editor-options"><button :class="{ active: canvasPropertyDraft.feeText === '无中介费' }" @click="canvasPropertyDraft.feeText = canvasPropertyDraft.feeText === '无中介费' ? '有中介费' : '无中介费'">{{ canvasPropertyDraft.feeText || '无中介费' }}</button><button v-for="source in ['直租','转租']" :key="source" :class="{ active: canvasPropertyDraft.listingSource === source }" @click="canvasPropertyDraft.listingSource = source">{{ source }}</button></div></div>
                          <div v-if="canvasPropertyDraft.listingSource === '转租'" class="canvas-editor-details"><label>剩余租期（月）<input v-model="canvasPropertyDraft.remainingLeaseMonths" type="number" placeholder="填写剩余月数"></label></div>
                          <div class="canvas-term-block canvas-utility-block"><b>水电与杂费</b><div class="canvas-utility-options"><button v-for="option in ['民水民电','民水民电 · 物业另计','水电物业全包','商水商电']" :key="option" :class="{ active: canvasPropertyDraft.utilitiesText === option }" @click="canvasPropertyDraft.utilitiesText = option">{{ option }}</button></div></div>
                          <div v-if="detailModuleEnabled('facilities')" class="canvas-facility-grid canvas-quick-facilities"><label v-for="facility in publicConfig.propertyDetail.facilities.filter(item => item.enabled)" :key="facility.key" :class="{ active: canvasPropertyFacilities.includes(facility.label) }"><input type="checkbox" :checked="canvasPropertyFacilities.includes(facility.label)" @change="toggleCanvasPropertyFacility(facility.label)"><img :src="detailFacilityIcon(facility.key)" alt=""><span>{{ facility.label }}</span></label></div>
                          <template v-if="detailModuleEnabled('description')"><label class="canvas-rules-field">房屋说明（选填）<textarea v-model.trim="canvasPropertyDraft.description" maxlength="800" placeholder="补充房屋特点和入住要求"></textarea></label><label class="canvas-rules-field">入住规则（选填）<input v-model.trim="canvasPropertyDraft.rulesText" maxlength="200" placeholder="如：可做饭，宠物需沟通"></label></template>
                        </div>
                      </template>
                      <template v-else-if="false">
                        <div class="canvas-section-head"><b>02</b><span><strong>配套与描述</strong></span></div>
                        <div v-if="detailModuleEnabled('facilities')" class="canvas-facility-grid">
                          <label v-for="facility in publicConfig.propertyDetail.facilities.filter(item => item.enabled)" :key="facility.key" :class="{active:canvasPropertyFacilities.includes(facility.label)}">
                            <input type="checkbox" :checked="canvasPropertyFacilities.includes(facility.label)" @change="toggleCanvasPropertyFacility(facility.label)">
                            <img :src="detailFacilityIcon(facility.key)" alt=""><span>{{ facility.label }}</span>
                          </label>
                        </div>
                        <template v-if="detailModuleEnabled('description')">
                          <div class="canvas-description-field"><textarea v-model.trim="canvasPropertyDraft.description" maxlength="800" aria-label="房源描述" :placeholder='canvasPageCopy("填写采光、楼层、朝向、入住要求、周边交通等信息")'></textarea><small>{{ (canvasPropertyDraft.description || '').length }}/800</small></div>
                          <label class="canvas-rules-field">入住规则<input v-model.trim="canvasPropertyDraft.rulesText" maxlength="200" :placeholder='canvasPageCopy("如：可做饭，宠物需沟通，女生优先")'></label>
                        </template>
                        <div v-if="!detailModuleEnabled('facilities') && !detailModuleEnabled('description')" class="canvas-module-entry-disabled">当前后台未启用“房屋设施”或“房源简介”，此步无需填写。</div>
                      </template>
                      <template v-else-if="false">
                        <div class="canvas-section-head"><b>03</b><span><strong>租金与入住</strong><small>把价格、付款与入住条件集中填写</small></span></div>
                        <div class="canvas-rent-row"><label class="canvas-rent-field"><span>月租 *</span><span class="canvas-rent-input"><input v-model="canvasPropertyDraft.rentMonthly" inputmode="numeric" placeholder="如：2600"><small>元/月</small></span></label><div class="canvas-rent-field"><span>中介费</span><label class="canvas-fee-toggle"><input type="checkbox" v-model="canvasPropertyDraft.feeText" true-value="无中介费" false-value="有中介费">无中介费</label></div></div>
                        <div class="canvas-term-block"><b>押付方式 *</b><div class="canvas-payment-options"><span class="fixed">押一</span><button v-for="cycle in ['付一','付二','付三','付六']" :key="cycle" :class="{ active: canvasPropertyDraft.depositPayment === '押一' + cycle }" @click="canvasPropertyDraft.depositPayment = '押一' + cycle">{{ cycle }}</button></div></div>
                        <template v-if="canvasPropertyDraft.rentType === '合租'"><div class="canvas-shared-rent-fields"><label>使用空间<input v-model="canvasPropertyDraft.layout" maxlength="32" placeholder="如：次卧 · 独卫 · 有飘窗"></label><label>使用面积（㎡）<input v-model="canvasPropertyDraft.areaSqm" type="number" placeholder="如：21"></label></div></template>
                        <div class="canvas-date-utility-row"><label>可入住日期<input v-model="canvasPropertyDraft.availableDate" maxlength="32" placeholder="默认拎包入住，可填写具体日期"></label><div><span>水电与杂费</span><strong>{{ canvasPropertyDraft.utilitiesText || '请选择' }}</strong></div></div>
                        <div class="canvas-term-block canvas-utility-block"><b>水电与杂费</b><div class="canvas-utility-options"><button v-for="option in ['民水民电','民水民电 · 物业另计','水电物业全包','商水商电']" :key="option" :class="{ active: canvasPropertyDraft.utilitiesText === option }" @click="canvasPropertyDraft.utilitiesText = option">{{ option }}</button></div></div>
                        <div class="canvas-rent-summary"><span>对外展示</span><strong>{{ Number(canvasPropertyDraft.rentMonthly) || 0 }}/月</strong><small>{{ canvasPropertyDraft.depositPayment || '待填写押付方式' }}</small></div>
                      </template>
                      <template v-else>
                        <div class="canvas-section-head"><b>04</b><span><strong>图片与视频</strong></span></div>
                        <div class="canvas-media-card"><div class="canvas-media-guidance"><b>封面素材</b><span>上传一次即可：列表横向裁切、瀑布流竖向裁切；详情保持原比例。</span></div>
                          <div class="canvas-cover-focus"><b>封面焦点（可选）</b><small>默认居中；仅在主体靠边时调整，横图和竖图会共用。</small><div class="canvas-focus-options"><label>水平<select v-model="canvasPropertyDraft.coverFocusX"><option value="0">靠左</option><option value="50">居中</option><option value="100">靠右</option></select></label><label>垂直<select v-model="canvasPropertyDraft.coverFocusY"><option value="0">靠上</option><option value="50">居中</option><option value="100">靠下</option></select></label></div></div>
                          <div class="canvas-media-actions"><button type="button" @click="addCanvasSampleImage"><b>＋ 图片</b><span>{{ canvasPropertyDraft.imageCount || 0 }}/9</span></button><button type="button" @click="setCanvasNotice('画布预览不选择或上传本地媒体文件。')"><b>＋ 视频</b><span>{{ canvasPropertyDraft.videoCount || 0 }}/1</span></button></div>
                          <div v-if="canvasSampleImageAdded" class="canvas-sample-media-tile"><img :src="homeHeroFallback" alt="示例房源图片" :class="{ cropped: canvasSampleImageCropped }"><span>示例图片 · {{ canvasSampleImageCropped ? '已裁切' : '待裁切' }}</span><button type="button" @click="openCanvasCropPreview">{{ canvasSampleImageCropped ? '重裁' : '裁切' }}</button><button type="button" aria-label="移除示例图片" @click="removeCanvasSampleImage">×</button></div>
                        </div>
                        <p class="canvas-media-limit-hint">图片 ≥640px / ≤10MB · 视频 ≤40MB / 60秒</p>
                        <div class="canvas-title-composer"><b>房源名称</b><span>默认使用小区名称，可自定义；客户与管理员看到同一名称</span><strong>列表标题预览</strong><em>{{ canvasDraftTitle }}</em></div>
                        <div class="canvas-final-review"><header><span><b>发布前核对</b><small>可点上方步骤返回修改；图片和视频为选填</small></span><strong>{{ canvasPropertyCompletionCount }}/2</strong></header><p v-if="canvasPublishValidationIssues.length" class="canvas-publish-validation">发布前还需要：{{ canvasPublishValidationIssues.join('、') }}</p><p v-else class="canvas-publish-validation ready">房源信息已满足发布条件</p><div class="canvas-review-grid"><article><b>位置与房屋概况</b><small>{{ canvasPropertyDraft.community || '待选择' }}</small></article><article><b>房屋与出租方式</b><small>{{ canvasPropertyDraft.layout || '待填写' }}</small></article><article><b>租金入住</b><small>{{ canvasPropertyDraft.rentMonthly || 0 }}/月 {{ canvasPropertyDraft.depositPayment || '' }}</small></article><article><b>图片素材（选填）</b><small>{{ canvasPropertyDraft.imageCount || 0 }}/9 张图片 · {{ canvasPropertyDraft.videoCount || 0 }} 个视频</small></article></div>
                          <div class="canvas-listing-review"><b>{{ canvasPageCopy("列表效果核对") }}</b><small>以下为保存后首页与“房源”页的真实卡片布局</small><nav><button type="button" :class="{active:canvasReviewPerspective==='customer'}" @click="canvasReviewPerspective='customer'">普通用户 · 瀑布流</button><button type="button" :class="{active:canvasReviewPerspective==='admin'}" @click="canvasReviewPerspective='admin'">管理员 · 列表</button></nav><div class="canvas-listing-stage" :class="canvasReviewPerspective"><div v-for="columnIndex in (canvasReviewPerspective === 'customer' ? 2 : 1)" :key="columnIndex" class="canvas-review-column"><article v-for="item in canvasReviewCards.filter((_, index) => canvasReviewPerspective === 'admin' || index % 2 === columnIndex - 1)" :key="item.id" class="canvas-listing-preview" :class="{current:item.id==='current'}"><div class="canvas-listing-cover" :style="item.cover"><b v-if="item.id==='current' && !item.hasCover">小皮</b><span>可租</span><i v-if="item.id==='current'">当前编辑</i><small>⌖ {{ item.location }}</small></div><div class="canvas-listing-copy"><strong>{{ item.title }}</strong><small v-if="canvasReviewPerspective==='admin'">门牌 · {{ item.doorplate }}</small><div class="canvas-review-tags"><span v-for="(tag,index) in item.tags" :key="index">{{ tag }}</span></div><em>¥ {{ item.rent }}<small>/月</small></em></div></article></div></div></div>
                        </div>
                      </template>
                    </div>
                    <div v-if="canvasCommunityPickerOpen" class="preview-overlay canvas-directory-overlay" @click="canvasCommunityPickerOpen = false">
                      <section class="canvas-bottom-sheet canvas-community-directory" role="dialog" aria-modal="true" aria-label="选择小区" @click.stop>
                        <header><div><strong>选择小区</strong><small>已有记录优先；找不到可在这里搜索地图</small></div><button type="button" aria-label="关闭" @click="canvasCommunityPickerOpen = false">×</button></header>
                        <div class="canvas-community-search"><input v-model.trim="canvasCommunityQuery" placeholder="搜索小区库；也可输入地图关键词" @keyup.enter="searchCanvasCommunityMap"><button type="button" @click="searchCanvasCommunityMap">{{ canvasCommunityMapLoading ? '搜索中…' : '搜索地图' }}</button></div>
                        <section v-if="!canvasCommunityQuery && canvasCommonCommunities.length" class="canvas-community-picker-group"><b>常用小区</b><div><button v-for="name in canvasCommonCommunities" :key="`common-${name}`" type="button" @click="selectCanvasLibraryCommunity(name)">{{ name }}</button></div></section>
                        <section class="canvas-community-picker-group"><b>预设小区</b><div class="canvas-community-picker-grid"><button v-for="name in canvasFilteredCommunities" :key="name" type="button" @click="selectCanvasLibraryCommunity(name)"><strong>{{ name }}</strong><small>{{ doorplateCommunityCounts[name] || 0 }} 条门牌</small></button><p v-if="!canvasFilteredCommunities.length">没有匹配的小区库记录</p></div></section>
                        <section v-if="canvasCommunityMapResults.length || canvasCommunityMapLoading || canvasCommunityMapSearched" class="canvas-community-picker-group map-results"><b>地图结果</b><p v-if="canvasCommunityMapLoading">正在搜索地图地点…</p><p v-else-if="canvasCommunityMapSearched && !canvasCommunityMapResults.length">没有合适结果，检查名称后重试。</p><button v-for="item in canvasCommunityMapResults" :key="item.id" type="button" @click="selectCanvasMapCommunity(item)"><strong>{{ item.title }}</strong><small>{{ [item.district, item.address].filter(Boolean).join(' · ') || '地图地点' }}</small><em>选择</em></button></section>
                      </section>
                    </div>
                    <div v-if="canvasDoorplatePickerOpen" class="preview-overlay canvas-directory-overlay" @click="canvasDoorplatePickerOpen = false">
                      <section class="canvas-bottom-sheet canvas-doorplate-picker" role="dialog" aria-modal="true" aria-label="选择门牌" @click.stop>
                        <header><div><strong>{{ canvasDoorplateMode === 'catalog' ? '选择门牌' : '自定义门牌' }}</strong><small>{{ canvasPropertyDraft.community }} · {{ canvasDoorplateMode === 'catalog' ? '选择楼栋、单元和房号' : '按实际门牌输入，没有的层级可留空' }}</small></div><button type="button" aria-label="关闭" @click="canvasDoorplatePickerOpen = false">×</button></header>
                        <div v-if="canvasDoorplateRows.length" class="canvas-doorplate-mode"><button type="button" :class="{ active: canvasDoorplateMode === 'catalog' }" @click="canvasDoorplateMode = 'catalog'">选择已有门牌</button><button type="button" :class="{ active: canvasDoorplateMode === 'manual' }" @click="canvasDoorplateMode = 'manual'">自定义输入</button></div>
                        <template v-if="canvasDoorplateMode === 'catalog'"><div class="canvas-doorplate-step">{{ canvasDoorplateStep === 'building' ? '选择楼栋' : canvasDoorplateStep === 'unit' ? `${canvasSelectedBuilding} · 选择单元` : `${canvasSelectedBuilding}${canvasSelectedUnit} · 选择房号` }}</div><div class="canvas-doorplate-options"><button v-if="canvasDoorplateStep === 'building'" v-for="building in canvasDoorplateBuildings" :key="building" type="button" @click="selectCanvasDoorplate(building)">{{ building }}</button><button v-else-if="canvasDoorplateStep === 'unit'" v-for="unit in canvasDoorplateUnits" :key="unit" type="button" @click="selectCanvasDoorplate(canvasSelectedBuilding, unit)">{{ unit }}</button><button v-else v-for="room in canvasDoorplateRooms" :key="room" type="button" @click="selectCanvasDoorplate(canvasSelectedBuilding, canvasSelectedUnit, room)">{{ room }}室</button></div><button v-if="canvasDoorplateStep !== 'building'" class="canvas-doorplate-back" type="button" @click="canvasDoorplateStep = canvasDoorplateStep === 'room' ? 'unit' : 'building'">上一步</button></template>
                        <div v-else class="canvas-manual-doorplate"><label>楼栋<input v-model="canvasPropertyDraft.buildingNo" placeholder="如：3栋"></label><label>单元<input v-model="canvasPropertyDraft.unitNo" placeholder="没有单元可留空"></label><label>房号<input v-model="canvasPropertyDraft.roomNo" placeholder="如：3301"></label><button type="button" @click="saveCanvasManualDoorplate">完成</button></div>
                      </section>
                    </div>
                    <div class="preview-step-actions">
                      <button v-if="canvasEditStep > 0" type="button" @click="canvasEditStep--">上一步</button>
                      <button type="button" @click="submitCanvasPreview">{{ canvasEditingExistingProperty ? '保存修改' : '保存草稿' }}</button>
                      <button v-if="canvasEditStep < 1" type="button" @click="canvasEditStep++">下一步 · {{ canvasEditSteps[canvasEditStep + 1] }}</button>
                      <template v-else>
                        <button type="button" @click="attemptCanvasPublish">{{ canvasEditingExistingProperty ? '查看公开效果' : '预览草稿' }}</button>
                        <button v-if="!canvasEditingExistingProperty" type="button" @click="attemptCanvasPublish">直接发布</button>
                      </template>
                    </div>
                  </template>
                  <template v-else-if="canvasDataState === 'normal' && configPage === 'authLogin'">
                    <div class="preview-login-hero"><div class="preview-login-mark">小</div><small>{{ publicConfig.heroTitle || '小皮租房' }}</small><strong>访问码登录</strong><p>使用手机号和访问码继续</p></div>
                    <form class="preview-login-card canvas-form" @submit.prevent="submitCanvasPreview">
                      <header><div><b>手机号与访问码</b><span>{{ previewLoginReason || '登录后可同步关注、足迹与找房进度' }}</span></div></header>
                      <div class="preview-login-credentials"><label>手机号<input v-model="canvasLoginPhone" inputmode="tel" placeholder="手机号"></label><label>访问码<input v-model="canvasLoginAccessCode" inputmode="numeric" maxlength="6" placeholder="6 位数字"></label></div>
                      <button class="preview-code-login" type="submit">访问码登录</button>
                      <button class="preview-wechat-secondary" type="button" @click="setCanvasNotice('画布只展示微信登录入口，不会发起授权。')">使用微信登录</button>
                    </form>
                    <p class="preview-login-note">登录即表示同意小程序隐私保护说明</p>
                  </template>
                </div>
                <div v-if="['properties', 'me'].includes(configPage)" class="mini-tabbar"><button :class="{ active: configPage === 'home' }" @click="switchCanvasTab('home')"><img :src="configPage === 'home' ? homeTabActiveIcon : homeTabIcon" alt=""><small>首页</small></button><button :class="{ active: configPage === 'properties' }" @click="switchCanvasTab('properties')"><img :src="configPage === 'properties' ? propertyTabActiveIcon : propertyTabIcon" alt=""><small>房源</small></button><button :class="{ active: configPage === 'me' }" @click="switchCanvasTab('me')"><img :src="configPage === 'me' ? meTabActiveIcon : meTabIcon" alt=""><small>我的</small></button></div>
              <div v-if="configPage === 'propertyDetail' && previewOverlay === 'share'" class="preview-overlay canvas-share-overlay" @click="previewOverlay = 'page'"><section class="canvas-share-dialog" @click.stop><button class="canvas-share-close" type="button" aria-label="关闭分享预览" @click="previewOverlay = 'page'">×</button><article class="canvas-wechat-share-card"><header><i>皮</i><span>{{ publicConfig.heroTitle || '小皮租房' }}</span></header><strong>{{ sharePreviewType === 'friend' ? canvasShareTitle : (previewPrimaryProperty?.community || '精选房源') }}</strong><div class="canvas-wechat-share-image"><div :style="previewCoverStyle(previewPrimaryProperty)"></div><section><b>¥{{ previewPrimaryProperty?.rentMonthly || 0 }}/月</b><strong>{{ canvasShareTitle.slice(0,19) }}</strong><small>{{ (sharePreviewType === 'friend' ? previewShareTags(previewPrimaryProperty) : previewPosterTags(previewPrimaryProperty)).join('　') }}</small></section></div><footer>⌁ {{ sharePreviewType === 'friend' ? '好友分享 · 小程序' : '朋友圈分享预览' }}</footer></article></section></div>
              <div v-if="configPage === 'propertyDetail' && canvasDetailModal === 'appointment'" class="preview-overlay" @click="canvasDetailModal = ''"><form class="canvas-bottom-sheet canvas-form" @submit.prevent="submitCanvasPreview" @click.stop><header><div><strong>预约看房</strong><small>{{ previewPrimaryProperty ? previewListingTitle(previewPrimaryProperty) : '' }}</small></div><button type="button" aria-label="关闭" @click="canvasDetailModal = ''">×</button></header><label>怎么称呼您</label><input v-model="canvasForm.contactName" maxlength="20" :placeholder='canvasPageCopy("例如：张女士")'><label>联系手机号</label><strong class="canvas-appointment-phone">{{ canvasForm.phone ? canvasForm.phone.replace(/^(...).*(....)$/, '$1****$2') : '已绑定微信手机号' }}</strong><label>什么时候方便</label><div class="canvas-appointment-times"><button v-for="time in ['今天方便', '明天方便', '工作日晚上', '周末方便', '其他时间']" :key="time" type="button" :class="{ active: canvasForm.preferredTime === time }" @click="canvasForm.preferredTime = time">{{ time }}</button></div><button class="canvas-submit-button" type="submit">提交预约</button></form></div>
              <div v-if="configPage === 'propertyDetail' && canvasDetailModal === 'map'" class="preview-overlay" @click="canvasDetailModal = ''"><section class="canvas-bottom-sheet canvas-map-sheet" @click.stop><header><div><strong>房源位置</strong><small>{{ previewPrimaryProperty?.location }}</small></div><button type="button" aria-label="关闭" @click="canvasDetailModal = ''">×</button></header><div class="canvas-map-placeholder"><b>⌖</b><span>{{ previewPrimaryProperty?.community }}</span></div><p>坐标：{{ previewPrimaryProperty?.latitude || '未设置' }}，{{ previewPrimaryProperty?.longitude || '未设置' }}</p><button class="canvas-submit-button" type="button" @click="canvasDetailModal = ''">返回房源详情</button></section></div>
              <div v-if="canvasDatePickerOpen" class="preview-overlay canvas-date-overlay" @click="canvasDatePickerOpen = false"><section class="canvas-date-sheet" role="dialog" aria-modal="true" @click.stop><header><button type="button" @click="canvasDatePickerOpen = false">取消</button><strong>选择日期</strong><button type="button" @click="confirmCanvasDate">确定</button></header><input v-model="canvasDatePickerValue" type="date" :min="localDateString()"></section></div>
              <div v-if="configPage === 'footprints' && canvasFootprintClearOpen" class="preview-overlay canvas-access-overlay" @click="canvasFootprintClearOpen = false">
                <section class="canvas-access-dialog" role="dialog" aria-modal="true" @click.stop>
                  <strong>清空浏览足迹</strong><p>清空后无法恢复，确认继续？</p>
                  <footer><button type="button" @click="canvasFootprintClearOpen = false">取消</button><button type="button" @click="canvasFootprints = []; canvasFootprintClearOpen = false">确定</button></footer>
                </section>
              </div>
              <div v-if="canvasAccessPrompt" class="preview-overlay canvas-access-overlay" @click="canvasAccessPrompt = null"><section class="canvas-access-dialog" role="dialog" aria-modal="true" @click.stop><strong>{{ canvasAccessPrompt.title }}</strong><p>{{ canvasAccessPrompt.content }}</p><footer><button type="button" @click="canvasAccessPrompt = null">取消</button><button type="button" @click="confirmCanvasLogin">{{ configPage === 'requests' ? '去登录' : '微信登录' }}</button></footer></section></div>
                <section v-if="canvasCropPreviewOpen" class="canvas-crop-preview" role="dialog" aria-modal="true" aria-label="调整图片展示" @click.stop>
                  <header><button type="button" aria-label="返回图片列表" @click="canvasCropPreviewOpen = false">‹</button><strong>调整图片展示</strong><span></span></header>
                  <main><strong>调整展示位置</strong><p>单指拖动图片调整位置，双指捏合缩放。当前画面就是最终房源图。</p><div class="canvas-crop-stage" @pointerdown="startCanvasCropDrag" @pointermove="moveCanvasCropDrag" @pointerup="endCanvasCropDrag" @pointercancel="endCanvasCropDrag"><img :src="homeHeroFallback" alt="待裁切的房源示例图" :style="{ transform: `translate(${canvasCropOffset.x}px, ${canvasCropOffset.y}px) scale(${canvasCropZoom / 100})` }"><span>拖动图片调整位置</span></div><div class="canvas-crop-controls"><label>图片缩放 <b>{{ canvasCropZoom }}%</b></label><input v-model.number="canvasCropZoom" type="range" min="100" max="220" step="1"><button type="button" @click="resetCanvasCropPosition">恢复全图居中</button></div></main>
                  <footer><button type="button" @click="canvasCropPreviewOpen = false">取消</button><button type="button" @click="applyCanvasCropPreview">使用此裁切</button></footer>
                </section>
              </div>
              <section v-if="canvasAvatarCropOpen" class="canvas-crop-preview canvas-avatar-crop-preview" role="dialog" aria-modal="true" aria-label="调整头像" @click.stop>
                <header><button type="button" aria-label="返回个人中心" @click="canvasAvatarCropOpen = false">‹</button><strong>调整头像</strong><span></span></header>
                <main><strong>调整头像</strong><p>拖动并缩放图片，确认圆形区域内的头像效果。</p><div class="canvas-avatar-crop-stage"><span :style="{ transform: `scale(${canvasAvatarCropZoom / 100})` }">小</span><i></i></div><div class="canvas-crop-controls"><label>图片缩放 <b>{{ canvasAvatarCropZoom }}%</b></label><input v-model.number="canvasAvatarCropZoom" type="range" min="100" max="220" step="1"><small>画布展示裁切页面样式，不会上传或修改微信头像。</small></div></main>
                <footer><button type="button" @click="canvasAvatarCropOpen = false">取消</button><button type="button" @click="canvasAvatarCropOpen = false; setCanvasNotice('头像裁切效果仅供预览，画布不会上传头像。')">使用此头像</button></footer>
              </section>
              <div v-if="canvasOperationLogsOpen" class="preview-overlay canvas-directory-overlay" @click="canvasOperationLogsOpen = false">
                <section class="canvas-bottom-sheet canvas-operation-logs" role="dialog" aria-modal="true" aria-label="操作记录" @click.stop>
                  <header><div><strong>操作记录</strong><small>房源、客户跟进和消息通知等操作</small></div><button type="button" aria-label="关闭" @click="canvasOperationLogsOpen = false">×</button></header>
                  <div class="canvas-log-categories"><button v-for="category in canvasOperationLogCategories" :key="category" type="button" :class="{ active: canvasOperationLogsCategory === category }" @click="canvasOperationLogsCategory = category; canvasOperationLogsPage = 1">{{ category }}</button></div>
                  <div class="canvas-log-dates"><label>开始<input v-model="canvasOperationLogsStart" type="date" @change="canvasOperationLogsPage = 1"></label><label>结束<input v-model="canvasOperationLogsEnd" type="date" @change="canvasOperationLogsPage = 1"></label></div>
                  <div v-if="canvasFilteredOperationLogs.length" class="canvas-log-list"><article v-for="item in canvasFilteredOperationLogs.slice((canvasOperationLogsPage - 1) * 20, canvasOperationLogsPage * 20)" :key="item.id"><span>{{ canvasOperationLogBucket(item.targetType) }}</span><strong>{{ item.action }}</strong><small>{{ item.detail || '操作详情' }}</small><time>{{ item.createdAt ? new Date(item.createdAt).toLocaleString() : '时间记录' }}</time></article></div><p v-else class="canvas-log-empty">暂无符合条件的操作记录</p>
                  <footer><button type="button" :disabled="canvasOperationLogsPage <= 1" @click="canvasOperationLogsPage--">上一页</button><span>{{ canvasOperationLogsPage }} / {{ Math.max(1, Math.ceil(canvasFilteredOperationLogs.length / 20)) }}</span><button type="button" :disabled="canvasOperationLogsPage >= Math.ceil(canvasFilteredOperationLogs.length / 20)" @click="canvasOperationLogsPage++">下一页</button></footer>
                </section>
              </div>
              <footer class="canvas-preview-footer" aria-label="实时画布数据"><div class="canvas-data-source"><i :class="{ live: overviewState === 'ready', failed: overviewState === 'error' }"></i><span>{{ overviewState === 'loading' ? '正在载入数据…' : overviewState === 'error' ? `加载失败：${overviewError}` : `预览 · ${canvasRoleLabel}` }}</span><el-tooltip content="使用当前 Admin API 环境的业务数据。表单提交、删除、状态变更均不写入；微信用户私有数据需由对应用户身份访问。" placement="top"><button type="button" aria-label="预览说明">ⓘ</button></el-tooltip><button type="button" :disabled="loading || configDirty" :title="configDirty ? '请先发布或恢复草稿' : undefined" @click="load().then(loadCanvasWorkbench).catch(() => undefined)">刷新</button></div></footer>
              </div>
            </aside>
          </div>
        </template>

        <DoorplatesPage v-else-if="tab === '小区门牌数据'" :api="api" />
        <LeadsPage v-else-if="tab === '线索管理'" :leads="leads" :loading="loadingSections.includes('线索')" :filter="leadFilter" @refresh="load" @filter="value => router.replace({ query: value === 'pending' ? { filter: value } : {} })" />
  </AdminLayout>
</template>
