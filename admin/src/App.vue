<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ArrowDown, ArrowUp, Collection, Delete, House, Location, Plus, RefreshLeft, User } from '@element-plus/icons-vue'

import AdminSidebar from './components/AdminSidebar.vue'
import AdminTopbar from './components/AdminTopbar.vue'

type LocationConfig = { label: string; keyword: string }
type ChannelConfig = { icon: string; title: string; subtitle: string; keyword: string; latitude: number; longitude: number; radiusKm: number }
type PublicConfig = {
  heroTitle: string
  heroSubtitle: string
  heroDescription: string
  notice: string
  contactPhone: string
  relatedPropertyRadiusKm: number
  locations: LocationConfig[]
  featuredChannels: ChannelConfig[]
}
type Property = { title: string; community: string; status: string }
type Lead = { sourceName: string; community: string; sourcePhone: string; status: string }
type RentalRequest = { id: string; requestType: string; propertyId: string; phone: string; location: string; layout: string; budgetMin: number; budgetMax: number; moveInDate: string; note: string; status: string; createdAt: string }
type WechatUser = { id: string; nickname: string; avatarUrl: string; phone: string; status: string; createdAt: string }
type AdminUser = { id: number; userId: string; nickname: string; avatarUrl: string; phone: string; role: string; status: string; createdBy: string; createdAt: string }
type TemporaryAccess = { id: number; phone: string; accessCode: string; status: string; expiresAt: string; createdBy: string; createdAt: string }
type TabKey = '运营总览' | '房源审核' | '线索管理' | '租客需求' | '微信用户' | '小程序配置' | '管理员列表' | '临时访问码'

const apiBase = import.meta.env.VITE_API_BASE || 'http://150.158.157.213/api'
const loggedIn = ref(Boolean(localStorage.getItem('xiaopi_admin_token')))
const tab = ref<TabKey>('运营总览')
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
const assigningAdmin = ref(false)
const adminPhone = ref('')
const adminCandidate = ref<WechatUser | null>(null)
const adminSearchOpen = ref(false)
const adminMessage = ref('')
const adminFailed = ref(false)
const properties = ref<Property[]>([])
const leads = ref<Lead[]>([])
const rentalRequests = ref<RentalRequest[]>([])
const users = ref<WechatUser[]>([])
const admins = ref<AdminUser[]>([])
const temporaryAccess = ref<TemporaryAccess[]>([])
const temporaryPhone = ref('')
const temporaryHours = ref(168)
const generatingTemporaryCode = ref(false)
const generatedTemporaryCode = ref('')
const temporaryMessage = ref('')
const temporaryFailed = ref(false)
const publicConfig = reactive<PublicConfig>({ heroTitle: '', heroSubtitle: '', heroDescription: '', notice: '', contactPhone: '', relatedPropertyRadiusKm: 3, locations: [], featuredChannels: [] })
const channelStyleOptions = [
  { value: 'commute', label: '通勤' },
  { value: 'campus', label: '校园' },
  { value: 'work', label: '职场' },
  { value: 'life', label: '生活' },
]
const normalizeChannelStyle = (value: string, index: number) => {
  const legacy: Record<string, string> = { '🏢': 'work', '🌳': 'campus', '🌆': 'life', '🏠': 'life' }
  return legacy[value] || (channelStyleOptions.some((item) => item.value === value) ? value : channelStyleOptions[index % channelStyleOptions.length].value)
}

async function api<T>(path: string, options: RequestInit = {}) {
  const token = localStorage.getItem('xiaopi_admin_token')
  const response = await fetch(`${apiBase}${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers },
  })
  const body = await response.json().catch(() => ({ message: '服务返回异常' }))
  if (response.status === 401 || response.status === 403) {
    localStorage.removeItem('xiaopi_admin_token')
    loggedIn.value = false
    password.value = ''
    throw new Error('登录已过期，请重新登录')
  }
  if (!response.ok || body.code !== 0) throw new Error(body.message || '请求失败')
  return body.data as T
}

const cloneConfig = (value: PublicConfig): PublicConfig => JSON.parse(JSON.stringify(value))
const applyConfig = (value: PublicConfig, markSaved = false) => {
  const next = cloneConfig(value)
  next.relatedPropertyRadiusKm = next.relatedPropertyRadiusKm || 3
  next.featuredChannels = (next.featuredChannels || []).map((item, index) => ({ latitude: 0, longitude: 0, ...item, icon: normalizeChannelStyle(item.icon, index), radiusKm: item.radiusKm || 5 }))
  Object.assign(publicConfig, next)
  if (markSaved) savedConfigJSON.value = JSON.stringify(publicConfig)
}

async function load() {
  loading.value = true
  pageError.value = ''
  try {
    const [propertyData, leadData, rentalData, configData, userData, adminData, temporaryData] = await Promise.all([
      api<Property[]>('/v1/properties'),
      api<Lead[]>('/v1/property-leads'),
      api<RentalRequest[]>('/v1/admin/rental-requests'),
      api<PublicConfig>('/v1/config/public'),
      api<WechatUser[]>('/v1/admin/users'),
      api<AdminUser[]>('/v1/admin/admin-users'),
      api<TemporaryAccess[]>('/v1/admin/temporary-access'),
    ])
    properties.value = propertyData || []
    leads.value = leadData || []
    rentalRequests.value = rentalData || []
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
    tab.value = '运营总览'
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

function selectTab(value: string) { tab.value = value as TabKey }

const rows = computed(() => {
  if (tab.value === '线索管理') return leads.value.map((item) => ({ name: item.sourceName, info: `${item.community} · ${item.sourcePhone}`, status: item.status }))
  return properties.value.map((item) => ({ name: item.title, info: item.community, status: item.status }))
})
const currentMetric = computed(() => tab.value === '线索管理' ? '线索总数' : '房源总数')
const statusType = (status: string) => status === '正常' || status === 'PUBLISHED' ? 'success' : status === 'PENDING' ? 'warning' : 'info'
const formatTime = (value: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'

const configDirty = computed(() => Boolean(savedConfigJSON.value) && JSON.stringify(publicConfig) !== savedConfigJSON.value)
const configErrors = computed(() => {
  const errors: string[] = []
  if (!publicConfig.heroTitle.trim()) errors.push('品牌标题不能为空')
  if (!publicConfig.heroSubtitle.trim()) errors.push('首页主标题不能为空')
  if (!publicConfig.heroDescription.trim()) errors.push('首页副标题不能为空')
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
  publicConfig.featuredChannels.push({ icon: 'commute', title: '', subtitle: '', keyword: '', latitude: 0, longitude: 0, radiusKm: 5 })
}

function removeChannel(index: number) {
  publicConfig.featuredChannels.splice(index, 1)
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

  <main v-else class="admin-shell">
    <AdminSidebar :active="tab" @select="selectTab" />
    <section class="admin-main">
      <AdminTopbar :title="tab" @logout="logout" />
      <div class="admin-content">
        <el-alert v-if="pageError" class="page-error" :title="pageError" type="error" show-icon :closable="false" />

        <template v-if="tab === '运营总览'">
          <section class="dashboard-hero">
            <div><span class="dashboard-kicker">XIAOPI OPERATIONS</span><h1>运营总览</h1></div>
            <div class="hero-meter"><span>当前待跟进线索</span><strong>{{ leads.length }}</strong></div>
          </section>
          <div v-loading="loading" class="stats-grid dashboard-grid">
            <button class="surface stat-card" @click="tab = '房源审核'"><el-icon><House /></el-icon><span>房源总数</span><strong>{{ properties.length }}</strong></button>
            <button class="surface stat-card" @click="tab = '线索管理'"><el-icon><Collection /></el-icon><span>客户线索</span><strong>{{ leads.length }}</strong></button>
            <button class="surface stat-card" @click="tab = '微信用户'"><el-icon><User /></el-icon><span>微信用户</span><strong>{{ users.length }}</strong></button>
            <button class="surface stat-card" @click="tab = '小程序配置'"><el-icon><Location /></el-icon><span>首页地区</span><strong>{{ publicConfig.locations.length }}</strong></button>
          </div>
        </template>

        <template v-else-if="tab === '微信用户'">
          <section class="page-header">
            <div><span>用户与权限</span><h1>微信用户</h1></div>
            <el-button :loading="loading" @click="load">刷新数据</el-button>
          </section>
          <section class="surface table-panel">
            <div class="table-heading"><div><h2>微信用户列表</h2><p>共 {{ users.length }} 人，已绑定手机号 {{ users.filter((item) => item.phone).length }} 人</p></div></div>
            <el-table v-loading="loading" :data="users" empty-text="暂无微信用户">
              <el-table-column label="用户" min-width="220">
                <template #default="scope"><div class="user-cell"><el-avatar :size="38" :src="scope.row.avatarUrl">微</el-avatar><div><strong>{{ scope.row.nickname || '微信用户' }}</strong><span>{{ scope.row.id }}</span></div></div></template>
              </el-table-column>
              <el-table-column label="手机号" min-width="160"><template #default="scope"><span v-if="scope.row.phone">{{ scope.row.phone }}</span><el-tag v-else type="info">未绑定</el-tag></template></el-table-column>
              <el-table-column label="状态" width="120"><template #default="scope"><el-tag :type="scope.row.status === 'active' ? 'success' : 'danger'">{{ scope.row.status === 'active' ? '正常' : '已停用' }}</el-tag></template></el-table-column>
              <el-table-column label="首次登录" min-width="180"><template #default="scope">{{ formatTime(scope.row.createdAt) }}</template></el-table-column>
            </el-table>
          </section>
        </template>

        <template v-else-if="tab === '租客需求'">
          <section class="page-header"><div><span>业务管理</span><h1>租客需求</h1></div><el-button :loading="loading" @click="load">刷新数据</el-button></section>
          <div class="stats-grid list-stats"><section class="surface compact-stat"><span>需求总数</span><strong>{{ rentalRequests.length }}</strong></section><section class="surface compact-stat"><span>预约房源</span><strong>{{ rentalRequests.filter(item => item.requestType === 'appointment').length }}</strong></section><section class="surface compact-stat"><span>自主找房</span><strong>{{ rentalRequests.filter(item => item.requestType === 'general').length }}</strong></section></div>
          <section class="surface table-panel"><div class="table-heading"><div><h2>需求记录</h2><p>优先联系新提交的用户，并按预算与区域推荐房源</p></div></div><el-table :data="rentalRequests" empty-text="暂无租客需求"><el-table-column label="类型" width="110"><template #default="scope"><el-tag :type="scope.row.requestType === 'appointment' ? 'warning' : 'success'">{{ scope.row.requestType === 'appointment' ? '预约房源' : '帮我找房' }}</el-tag></template></el-table-column><el-table-column prop="phone" label="联系电话" min-width="145"/><el-table-column label="地区与户型" min-width="190"><template #default="scope">{{ scope.row.location || '未指定地区' }}<br/><span class="muted-table-text">{{ scope.row.layout || '户型不限' }}</span></template></el-table-column><el-table-column label="月租预算" min-width="145"><template #default="scope">{{ scope.row.budgetMin || 0 }}–{{ scope.row.budgetMax || '不限' }} 元</template></el-table-column><el-table-column label="入住日期" min-width="125"><template #default="scope">{{ scope.row.moveInDate || '待沟通' }}</template></el-table-column><el-table-column prop="note" label="补充要求" min-width="220" show-overflow-tooltip/><el-table-column label="提交时间" min-width="180"><template #default="scope">{{ formatTime(scope.row.createdAt) }}</template></el-table-column></el-table></section>
        </template>

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
              <el-input v-model.trim="temporaryPhone" placeholder="用户手机号" clearable />
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
          <el-alert v-if="configMessage" class="config-alert" :title="configMessage" :type="configFailed ? 'error' : 'success'" show-icon :closable="false" />
          <section v-if="configErrors.length" class="config-validation">
            <strong>发布前需要处理 {{ configErrors.length }} 项</strong>
            <ul><li v-for="error in configErrors" :key="error">{{ error }}</li></ul>
          </section>
          <div v-loading="loading" class="config-workspace">
            <div class="config-editor">
              <section class="surface form-section config-section">
                <div class="section-heading"><span>01</span><div><h2>首页信息</h2></div></div>
                <el-form label-position="top" class="basic-config-form">
                  <div class="form-row two-columns">
                    <el-form-item label="品牌名称" required><el-input v-model="publicConfig.heroTitle" maxlength="20" show-word-limit placeholder="例如：小皮租房" /></el-form-item>
                    <el-form-item label="联系电话"><el-input v-model.trim="publicConfig.contactPhone" maxlength="20" placeholder="用于房源详情联系" /></el-form-item>
                  </div>
                  <el-form-item label="首页主标题" required><el-input v-model="publicConfig.heroSubtitle" maxlength="30" show-word-limit placeholder="一句话说明服务区域或核心价值" /></el-form-item>
                  <el-form-item label="首页副标题" required><el-input v-model="publicConfig.heroDescription" maxlength="40" show-word-limit placeholder="补充可信度或服务特色" /></el-form-item>
                  <el-form-item label="首页公告"><el-input v-model="publicConfig.notice" type="textarea" :rows="3" maxlength="120" show-word-limit placeholder="发布签约提醒、服务通知或风险提示" /></el-form-item>
                  <el-form-item label="附近房源关联距离">
                    <el-select v-model="publicConfig.relatedPropertyRadiusKm" placeholder="选择距离" style="width: 100%">
                      <el-option v-for="distance in [1, 2, 3, 5, 10]" :key="distance" :label="`${distance} 公里`" :value="distance" />
                    </el-select>
                    <div class="form-tip">按地图坐标自动关联。</div>
                  </el-form-item>
                </el-form>
              </section>

              <section class="surface form-section config-section">
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

              <section class="surface form-section config-section">
                <div class="section-heading config-section-heading"><span>03</span><div><h2>精选频道</h2></div><el-button type="primary" plain :icon="Plus" :disabled="publicConfig.featuredChannels.length >= 12" @click="addChannel">新增频道</el-button></div>
                <div v-if="publicConfig.featuredChannels.length" class="channel-editor-grid">
                  <article v-for="(item, index) in publicConfig.featuredChannels" :key="`channel-${index}`" class="channel-editor-card">
                    <header><div class="channel-order"><span>{{ channelStyleOptions.find((style) => style.value === item.icon)?.label || '频道' }}</span><strong>频道 {{ index + 1 }}</strong></div><div class="item-actions"><el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="上移" @click="moveItem(publicConfig.featuredChannels, index, -1)" /><el-button text :icon="ArrowDown" :disabled="index === publicConfig.featuredChannels.length - 1" aria-label="下移" @click="moveItem(publicConfig.featuredChannels, index, 1)" /><el-popconfirm title="确认删除这个精选频道？" @confirm="removeChannel(index)"><template #reference><el-button text type="danger" :icon="Delete" aria-label="删除" /></template></el-popconfirm></div></header>
                    <div class="channel-fields">
                      <label class="icon-field"><span>频道样式</span><el-select v-model="item.icon"><el-option v-for="style in channelStyleOptions" :key="style.value" :label="style.label" :value="style.value" /></el-select></label>
                      <label><span>标题</span><el-input v-model="item.title" maxlength="12" placeholder="例如：金融港" /></label>
                      <label><span>副标题</span><el-input v-model="item.subtitle" maxlength="20" placeholder="例如：通勤优选" /></label>
                      <label><span>搜索关键词（无坐标兜底）</span><el-input v-model="item.keyword" maxlength="20" placeholder="点击后筛选的关键词" /></label>
                      <label><span>频道中心纬度</span><el-input-number v-model="item.latitude" :min="-90" :max="90" :precision="7" :step="0.0001" controls-position="right" /></label>
                      <label><span>频道中心经度</span><el-input-number v-model="item.longitude" :min="-180" :max="180" :precision="7" :step="0.0001" controls-position="right" /></label>
                      <label><span>关联半径（公里）</span><el-input-number v-model="item.radiusKm" :min="0.1" :max="100" :precision="1" :step="0.5" controls-position="right" /></label>
                    </div>
                  </article>
                </div>
                <div v-else class="config-empty"><strong>还没有精选频道</strong><span>添加频道后，用户可以从首页直接进入对应房源列表。</span><el-button type="primary" plain :icon="Plus" @click="addChannel">添加第一个频道</el-button></div>
              </section>
            </div>

            <aside class="config-preview-column">
              <div class="preview-heading"><div><strong>小程序首页预览</strong></div><el-tag :type="configDirty ? 'warning' : 'success'" effect="light">{{ configDirty ? '未发布' : '线上版本' }}</el-tag></div>
              <div class="config-preview-actions">
                <el-button :icon="RefreshLeft" :disabled="!configDirty || savingConfig" @click="resetConfig">恢复</el-button>
                <el-button type="primary" :loading="savingConfig" :disabled="!configDirty || configErrors.length > 0" @click="saveConfig">保存并发布</el-button>
              </div>
              <div class="phone-preview">
                <div class="phone-status"><span>9:41</span><span>● ● ●</span></div>
                <div class="mini-navigation">{{ publicConfig.heroTitle || '小皮租房' }}</div>
                <div class="mini-content">
                  <section class="mini-hero"><span>{{ publicConfig.heroTitle || '品牌名称' }}</span><strong>{{ publicConfig.heroSubtitle || '首页主标题' }}</strong><p>{{ publicConfig.heroDescription || '首页副标题' }}</p><button>查找房源 ›</button></section>
                  <div v-if="publicConfig.notice" class="mini-notice"><b>公告</b><span>{{ publicConfig.notice }}</span></div>
                  <div class="mini-section-title">热门地区</div>
                  <div class="mini-locations"><span v-for="(item, index) in publicConfig.locations.slice(0, 6)" :key="`preview-location-${index}`">{{ item.label || '未命名' }}</span><em v-if="!publicConfig.locations.length">尚未配置</em></div>
                  <div class="mini-section-title">精选频道</div>
                  <div class="mini-channels"><article v-for="(item, index) in publicConfig.featuredChannels.slice(0, 4)" :key="`preview-channel-${index}`"><b>{{ item.icon || '🏠' }}</b><strong>{{ item.title || '未命名' }}</strong><span>{{ item.subtitle || '频道说明' }}</span></article><em v-if="!publicConfig.featuredChannels.length">尚未配置</em></div>
                </div>
                <div class="mini-tabbar"><span>⌂<small>首页</small></span><span>▤<small>房源</small></span><span>○<small>我的</small></span></div>
              </div>
              <p class="preview-note">预览用于确认信息层级，实际小程序会根据设备尺寸自适应。</p>
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
      </div>
    </section>
  </main>
</template>
