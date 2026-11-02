<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Collection, House, Location, User } from '@element-plus/icons-vue'

import AdminSidebar from './components/AdminSidebar.vue'
import AdminTopbar from './components/AdminTopbar.vue'

type LocationConfig = { label: string; keyword: string }
type ChannelConfig = { icon: string; title: string; subtitle: string; keyword: string }
type PublicConfig = {
  heroTitle: string
  heroSubtitle: string
  heroDescription: string
  notice: string
  contactPhone: string
  locations: LocationConfig[]
  featuredChannels: ChannelConfig[]
}
type Property = { title: string; community: string; status: string }
type Lead = { sourceName: string; community: string; sourcePhone: string; status: string }
type WechatUser = { id: string; nickname: string; avatarUrl: string; phone: string; status: string; createdAt: string }
type AdminUser = { id: number; userId: string; nickname: string; avatarUrl: string; phone: string; role: string; status: string; createdBy: string; createdAt: string }
type TemporaryAccess = { id: number; phone: string; status: string; expiresAt: string; createdBy: string; createdAt: string }
type TabKey = '运营总览' | '房源审核' | '线索管理' | '微信用户' | '小程序配置' | '管理员列表' | '临时访问码'

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
const assigningAdmin = ref(false)
const adminPhone = ref('')
const adminCandidate = ref<WechatUser | null>(null)
const adminMessage = ref('')
const adminFailed = ref(false)
const properties = ref<Property[]>([])
const leads = ref<Lead[]>([])
const users = ref<WechatUser[]>([])
const admins = ref<AdminUser[]>([])
const temporaryAccess = ref<TemporaryAccess[]>([])
const temporaryPhone = ref('')
const temporaryHours = ref(168)
const generatingTemporaryCode = ref(false)
const generatedTemporaryCode = ref('')
const temporaryMessage = ref('')
const temporaryFailed = ref(false)
const publicConfig = reactive<PublicConfig>({ heroTitle: '', heroSubtitle: '', heroDescription: '', notice: '', contactPhone: '', locations: [], featuredChannels: [] })

async function api<T>(path: string, options: RequestInit = {}) {
  const token = localStorage.getItem('xiaopi_admin_token')
  const response = await fetch(`${apiBase}${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers },
  })
  const body = await response.json()
  if (!response.ok || body.code !== 0) throw new Error(body.message || '请求失败')
  return body.data as T
}

const applyConfig = (value: PublicConfig) => Object.assign(publicConfig, value)

async function load() {
  loading.value = true
  pageError.value = ''
  try {
    const [propertyData, leadData, configData, userData, adminData, temporaryData] = await Promise.all([
      api<Property[]>('/v1/properties'),
      api<Lead[]>('/v1/property-leads'),
      api<PublicConfig>('/v1/config/public'),
      api<WechatUser[]>('/v1/admin/users'),
      api<AdminUser[]>('/v1/admin/admin-users'),
      api<TemporaryAccess[]>('/v1/admin/temporary-access'),
    ])
    properties.value = propertyData || []
    leads.value = leadData || []
    users.value = userData || []
    admins.value = adminData || []
    temporaryAccess.value = temporaryData || []
    applyConfig(configData)
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

const locationsText = computed({
  get: () => publicConfig.locations.map((item) => `${item.label}|${item.keyword}`).join('\n'),
  set: (value: string) => {
    publicConfig.locations = value.split('\n').map((line) => line.trim()).filter(Boolean).map((line) => {
      const [label, keyword = label] = line.split('|').map((item) => item.trim())
      return { label, keyword }
    })
  },
})

const channelsText = computed({
  get: () => publicConfig.featuredChannels.map((item) => `${item.icon}|${item.title}|${item.subtitle}|${item.keyword}`).join('\n'),
  set: (value: string) => {
    publicConfig.featuredChannels = value.split('\n').map((line) => line.trim()).filter(Boolean).map((line) => {
      const [icon = '🏠', title = '', subtitle = '', keyword = title] = line.split('|').map((item) => item.trim())
      return { icon, title, subtitle, keyword }
    })
  },
})

async function saveConfig() {
  savingConfig.value = true
  configMessage.value = ''
  configFailed.value = false
  try {
    const value = await api<PublicConfig>('/v1/admin/config/public', { method: 'PUT', body: JSON.stringify(publicConfig) })
    applyConfig(value)
    configMessage.value = '配置已保存，小程序重新进入后即可获取最新内容。'
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
}

function searchAdminCandidate() {
  const phone = adminPhone.value.replace(/[^+\d]/g, '')
  adminCandidate.value = users.value.find((item) => item.phone === phone) || null
  adminFailed.value = !adminCandidate.value
  adminMessage.value = adminCandidate.value ? '已找到微信用户，请确认后指定管理员。' : '没有找到绑定该手机号的微信用户。'
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
    temporaryMessage.value = '访问码已生成。请通过可信渠道发送给对应用户，离开本页后将不再显示明文。'
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
        <p>管理房源、客户线索与小程序首页内容。</p>
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
            <div><span class="dashboard-kicker">XIAOPI OPERATIONS</span><h1>租住业务运营总览</h1><p>集中查看房源、线索与用户状态，并维护小程序对外展示的地区和精选频道。</p></div>
            <div class="hero-meter"><span>当前待跟进线索</span><strong>{{ leads.length }}</strong></div>
          </section>
          <div v-loading="loading" class="stats-grid dashboard-grid">
            <button class="surface stat-card" @click="tab = '房源审核'"><el-icon><House /></el-icon><span>房源总数</span><strong>{{ properties.length }}</strong><em>查看房源审核</em></button>
            <button class="surface stat-card" @click="tab = '线索管理'"><el-icon><Collection /></el-icon><span>客户线索</span><strong>{{ leads.length }}</strong><em>跟进租住意向</em></button>
            <button class="surface stat-card" @click="tab = '微信用户'"><el-icon><User /></el-icon><span>微信用户</span><strong>{{ users.length }}</strong><em>查看手机号绑定</em></button>
            <button class="surface stat-card" @click="tab = '小程序配置'"><el-icon><Location /></el-icon><span>首页地区</span><strong>{{ publicConfig.locations.length }}</strong><em>维护展示内容</em></button>
          </div>
        </template>

        <template v-else-if="tab === '微信用户'">
          <section class="page-header">
            <div><span>用户与权限</span><h1>微信用户</h1><p>展示已通过小程序微信登录的用户；绑定手机号后，才可以按手机号指定管理员。</p></div>
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

        <template v-else-if="tab === '管理员列表'">
          <section class="page-header">
            <div><span>平台设置</span><h1>管理员列表</h1><p>先按手机号搜索已绑定的微信用户，确认用户身份后再授予运营权限。</p></div>
          </section>
          <section class="surface admin-search-panel">
            <div class="admin-search-heading"><h2>搜索微信用户</h2><p>只有已经出现在“微信用户”列表并绑定手机号的用户可以成为管理员。</p></div>
            <div class="assign-control"><el-input v-model.trim="adminPhone" placeholder="输入完整手机号" clearable @input="resetAdminSearch" @keyup.enter="searchAdminCandidate" /><el-button type="primary" :disabled="!adminPhone" @click="searchAdminCandidate">搜索用户</el-button></div>
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
            <div><span>平台设置</span><h1>临时访问码</h1><p>微信登录暂不可用期间，为指定手机号生成独立访问码。用户使用手机号和访问码登录，不会与其他用户串号。</p></div>
          </section>
          <section class="surface temporary-access-panel">
            <div class="temporary-form">
              <el-input v-model.trim="temporaryPhone" placeholder="用户手机号" clearable />
              <el-input-number v-model="temporaryHours" :min="1" :max="720" controls-position="right" />
              <span class="hours-unit">小时有效</span>
              <el-button type="primary" :loading="generatingTemporaryCode" :disabled="!temporaryPhone" @click="generateTemporaryAccess">生成访问码</el-button>
            </div>
            <div v-if="generatedTemporaryCode" class="generated-code"><span>本次访问码</span><strong>{{ generatedTemporaryCode }}</strong><small>请现在复制并发送，后台不会再次显示明文。</small></div>
          </section>
          <el-alert v-if="temporaryMessage" class="config-alert" :title="temporaryMessage" :type="temporaryFailed ? 'error' : 'success'" show-icon :closable="false" />
          <section class="surface table-panel">
            <div class="table-heading"><div><h2>访问码记录</h2><p>重新生成会立即替换同一手机号的旧访问码</p></div></div>
            <el-table :data="temporaryAccess" empty-text="暂无临时访问码">
              <el-table-column prop="phone" label="手机号" min-width="160" />
              <el-table-column label="状态" width="120"><template #default="scope"><el-tag :type="scope.row.status === 'active' && new Date(scope.row.expiresAt) > new Date() ? 'success' : 'info'">{{ scope.row.status === 'revoked' ? '已撤销' : new Date(scope.row.expiresAt) <= new Date() ? '已过期' : '有效' }}</el-tag></template></el-table-column>
              <el-table-column label="有效期至" min-width="180"><template #default="scope">{{ formatTime(scope.row.expiresAt) }}</template></el-table-column>
              <el-table-column prop="createdBy" label="创建人" min-width="140" />
              <el-table-column label="操作" width="100" align="center"><template #default="scope"><el-popconfirm v-if="scope.row.status === 'active'" title="确认撤销该手机号的访问码？" @confirm="revokeTemporaryAccess(scope.row)"><template #reference><el-button link type="danger">撤销</el-button></template></el-popconfirm><span v-else>-</span></template></el-table-column>
            </el-table>
          </section>
        </template>

        <template v-else-if="tab === '小程序配置'">
          <section class="page-header">
            <div><span>内容与运营</span><h1>小程序配置</h1><p>配置首页品牌信息、公告、地理位置和精选频道，保存后由线上 API 统一下发。</p></div>
            <el-button type="primary" size="large" :loading="savingConfig" @click="saveConfig">保存并发布</el-button>
          </section>
          <el-alert v-if="configMessage" class="config-alert" :title="configMessage" :type="configFailed ? 'error' : 'success'" show-icon :closable="false" />
          <div v-loading="loading" class="config-grid">
            <section class="surface form-section">
              <div class="section-heading"><span>01</span><div><h2>首页基础信息</h2><p>控制小程序首页首屏文案与联系方式。</p></div></div>
              <el-form label-position="top">
                <el-form-item label="品牌标题"><el-input v-model="publicConfig.heroTitle" /></el-form-item>
                <el-form-item label="首页主标题"><el-input v-model="publicConfig.heroSubtitle" /></el-form-item>
                <el-form-item label="首页副标题"><el-input v-model="publicConfig.heroDescription" /></el-form-item>
                <el-form-item label="公告内容"><el-input v-model="publicConfig.notice" type="textarea" :rows="3" /></el-form-item>
                <el-form-item label="联系电话"><el-input v-model="publicConfig.contactPhone" /></el-form-item>
              </el-form>
            </section>
            <div class="config-side">
              <section class="surface form-section"><div class="section-heading"><span>02</span><div><h2>地理位置</h2><p>每行：显示名称|搜索关键词</p></div></div><el-input v-model="locationsText" type="textarea" :rows="7" placeholder="光谷|光谷" /></section>
              <section class="surface form-section"><div class="section-heading"><span>03</span><div><h2>精选频道</h2><p>每行：图标|标题|副标题|搜索关键词</p></div></div><el-input v-model="channelsText" type="textarea" :rows="8" placeholder="🏢|金融港|通勤优选|金融港" /></section>
            </div>
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
