<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElTabs, ElTabPane } from 'element-plus/es/components/tabs/index.mjs'
import { ElPagination } from 'element-plus/es/components/pagination/index.mjs'
import 'element-plus/es/components/tabs/style/css'
import 'element-plus/es/components/pagination/style/css'
import type { FavoriteContact } from '../types/domain'

type ContactEvent = { id: string; propertyName?: string; nickname?: string; phone?: string; type: string; source?: string; createdAt?: string }
const props = defineProps<{ contacts: FavoriteContact[]; loading: boolean; api: <T>(path: string) => Promise<T> }>()
const events = ref<ContactEvent[]>([])
const eventsLoading = ref(false)
const eventsFailed = ref(false)
const page = ref(1)
const total = ref(0)
const pageSize = 20
let requestSequence = 0

async function loadEvents() {
  const sequence = ++requestSequence
  eventsLoading.value = true
  eventsFailed.value = false
  try {
    const result = await props.api<{ items: ContactEvent[]; total: number }>(`/v1/admin/contact-events/query?page=${page.value}&pageSize=${pageSize}`)
    if (sequence !== requestSequence) return
    total.value = result.total
    const lastPage = Math.max(1, Math.ceil(result.total / pageSize))
    if (page.value > lastPage) {
      page.value = lastPage
      return
    }
    events.value = result.items || []
  } catch {
    if (sequence === requestSequence) eventsFailed.value = true
  } finally {
    if (sequence === requestSequence) eventsLoading.value = false
  }
}
const tab = ref('favorites')
const emit = defineEmits<{ refresh: [] }>()
watch([tab, page], ([activeTab]) => {
  if (activeTab === 'contacts') void loadEvents()
})
function refresh() {
  if (tab.value === 'contacts') void loadEvents()
  else emit('refresh')
}
const formatTime = (value?: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
</script>

<template>
  <section class="page-header"><div><span>用户行为</span><h1>用户意向</h1></div><el-button :loading="tab === 'favorites' ? loading : eventsLoading" @click="refresh">刷新数据</el-button></section>
  <el-tabs v-model="tab"><el-tab-pane label="收藏记录" name="favorites" /><el-tab-pane label="联系记录" name="contacts" /></el-tabs>
  <section v-if="tab === 'favorites'" class="surface table-panel"><div class="table-heading"><div><h2>房源收藏记录</h2><p>收藏仅代表用户兴趣，不要求绑定手机号</p></div></div><el-table :data="contacts" empty-text="暂无收藏记录"><el-table-column label="用户" min-width="210"><template #default="scope"><div class="user-cell"><el-avatar :size="38" :src="scope.row.avatarUrl">微</el-avatar><div><strong>{{ scope.row.nickname || '微信用户' }}</strong><span>{{ scope.row.userId }}</span></div></div></template></el-table-column><el-table-column prop="propertyName" label="关注房源" min-width="220"/><el-table-column label="手机号" min-width="150"><template #default="scope">{{ scope.row.phone || '未绑定' }}</template></el-table-column><el-table-column label="关注时间" min-width="180"><template #default="scope">{{ formatTime(scope.row.createdAt) }}</template></el-table-column></el-table></section>
  <section v-else class="surface table-panel" v-loading="eventsLoading">
    <div class="table-heading"><div><h2>联系入口点击</h2><p>记录拨号和咨询入口的点击。</p></div></div>
    <el-alert v-if="eventsFailed" type="error" title="联系记录加载失败，请刷新数据。" :closable="false" />
    <el-table v-else :data="events" empty-text="暂无联系记录">
      <el-table-column label="类型" min-width="140"><template #default="{ row }">{{ row.type === 'phone_entry_click' ? '拨号入口点击' : row.type === 'consult_entry_click' ? '咨询入口点击' : row.type }}</template></el-table-column>
      <el-table-column label="用户" min-width="145"><template #default="{ row }">{{ row.nickname || '微信用户' }}</template></el-table-column>
      <el-table-column prop="propertyName" label="房源" min-width="180" />
      <el-table-column label="手机号" min-width="150"><template #default="{ row }">{{ row.phone || '未绑定' }}</template></el-table-column>
      <el-table-column label="点击时间" min-width="170"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
    </el-table>
    <el-pagination v-if="!eventsFailed && total > pageSize" v-model:current-page="page" :total="total" :page-size="pageSize" layout="total, prev, pager, next" :disabled="eventsLoading" style="margin-top: 16px" />
  </section>
</template>
