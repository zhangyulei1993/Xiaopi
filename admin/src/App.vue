<script setup lang="ts">
import { computed, reactive, ref } from 'vue'

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

const apiBase = import.meta.env.VITE_API_BASE || 'http://127.0.0.1:18080/api'
const loggedIn = ref(Boolean(localStorage.getItem('xiaopi_admin_token')))
const tab = ref('房源审核')
const users = ref([{ name: '微信用户', phone: '未绑定', status: '正常' }])
const properties = ref<{ title: string; community: string; status: string }[]>([])
const leads = ref<{ sourceName: string; community: string; sourcePhone: string; status: string }[]>([])
const username = ref('admin')
const password = ref('')
const loginError = ref('')
const savingConfig = ref(false)
const configMessage = ref('')
const publicConfig = reactive<PublicConfig>({
  heroTitle: '',
  heroSubtitle: '',
  heroDescription: '',
  notice: '',
  contactPhone: '',
  locations: [],
  featuredChannels: [],
})

const api = async <T,>(path: string, options: RequestInit = {}) => {
  const token = localStorage.getItem('xiaopi_admin_token')
  const response = await fetch(`${apiBase}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  })
  const body = await response.json()
  if (!response.ok || body.code !== 0) throw new Error(body.message || '请求失败')
  return body.data as T
}

const applyConfig = (value: PublicConfig) => Object.assign(publicConfig, value)

const load = async () => {
  const [propertyData, leadData, configData] = await Promise.all([
    api<typeof properties.value>('/v1/properties'),
    api<typeof leads.value>('/v1/property-leads'),
    api<PublicConfig>('/v1/config/public'),
  ])
  properties.value = propertyData
  leads.value = leadData
  applyConfig(configData)
}

const login = async () => {
  loginError.value = ''
  try {
    const auth = await api<{ accessToken: string }>('/v1/admin/login', {
      method: 'POST',
      body: JSON.stringify({ username: username.value, password: password.value }),
    })
    localStorage.setItem('xiaopi_admin_token', auth.accessToken)
    loggedIn.value = true
    await load()
  } catch (error) {
    loginError.value = error instanceof Error ? error.message : '登录失败'
  }
}

const rows = computed(() => tab.value === '客户管理' ? users.value : tab.value === '线索管理' ? leads.value : properties.value)

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

const saveConfig = async () => {
  savingConfig.value = true
  configMessage.value = ''
  try {
    const value = await api<PublicConfig>('/v1/admin/config/public', {
      method: 'PUT',
      body: JSON.stringify(publicConfig),
    })
    applyConfig(value)
    configMessage.value = '小程序配置已保存并立即生效'
  } catch (error) {
    configMessage.value = error instanceof Error ? error.message : '保存失败'
  } finally {
    savingConfig.value = false
  }
}

if (loggedIn.value) load().catch(() => { loggedIn.value = false })
</script>

<template>
  <main v-if="!loggedIn" class="login">
    <section>
      <div class="logo">小皮租房</div>
      <p>运营管理后台</p>
      <form @submit.prevent="login">
        <input v-model="username" placeholder="管理员账号" />
        <input v-model="password" placeholder="管理员密码" type="password" />
        <div v-if="loginError" class="error">{{ loginError }}</div>
        <button type="submit">登录后台</button>
      </form>
    </section>
  </main>

  <main v-else class="layout">
    <aside>
      <div class="brand">小皮租房</div>
      <div v-for="item in ['房源审核','线索管理','客户管理','小程序配置']" :key="item" class="nav" :class="{ active: tab === item }" @click="tab = item">{{ item }}</div>
    </aside>
    <section class="content">
      <header><h1>{{ tab }}</h1><span>管理员</span></header>

      <template v-if="tab === '小程序配置'">
        <div class="config-card">
          <h2>首页基础信息</h2>
          <label>品牌标题<input v-model="publicConfig.heroTitle" /></label>
          <label>首页主标题<input v-model="publicConfig.heroSubtitle" /></label>
          <label>首页副标题<input v-model="publicConfig.heroDescription" /></label>
          <label>公告内容<textarea v-model="publicConfig.notice" rows="3"></textarea></label>
          <label>联系电话<input v-model="publicConfig.contactPhone" /></label>

          <h2>地理位置</h2>
          <p class="help">每行一个，格式：显示名称|搜索关键词</p>
          <textarea v-model="locationsText" rows="6" placeholder="光谷|光谷"></textarea>

          <h2>精选频道</h2>
          <p class="help">每行一个，格式：图标|标题|副标题|搜索关键词</p>
          <textarea v-model="channelsText" rows="7" placeholder="🏢|金融港|通勤优选|金融港"></textarea>

          <div v-if="configMessage" class="config-message">{{ configMessage }}</div>
          <button class="save" :disabled="savingConfig" @click="saveConfig">{{ savingConfig ? '保存中...' : '保存并发布' }}</button>
        </div>
      </template>

      <template v-else>
        <div class="stats">
          <div><b>{{ rows.length }}</b><small>{{ tab === '客户管理' ? '客户总数' : tab === '线索管理' ? '待处理线索' : '房源总数' }}</small></div>
          <div><b>0</b><small>今日新增</small></div>
          <div><b>0</b><small>待处理事项</small></div>
        </div>
        <div class="table">
          <div class="tr head"><span>名称</span><span>信息</span><span>状态</span><span>操作</span></div>
          <div v-for="row in rows" :key="row.name || row.title || row.community" class="tr">
            <span>{{ row.name || row.title || row.sourceName }}</span>
            <span>{{ row.phone || row.community || row.sourcePhone }}</span>
            <span>{{ row.status }}</span>
            <button>查看</button>
          </div>
        </div>
      </template>
    </section>
  </main>
</template>
