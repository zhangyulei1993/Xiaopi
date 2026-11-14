<script setup lang="ts">
import { computed } from 'vue'
import { Collection, House, User } from '@element-plus/icons-vue'

const props = defineProps<{
  state: 'loading' | 'ready' | 'error'
  errorMessage: string
  propertyCount: number
  pendingLeadCount: number
  newRequestCount: number
  favoriteCount: number
  userCount: number
}>()
const emit = defineEmits<{ navigate: [tab: string]; refresh: [] }>()
const cards = computed(() => [
  { label: '房源总数', count: props.propertyCount, detail: '后台可见的全部房源', tab: '房源列表', icon: House },
  { label: '待跟进线索', count: props.pendingLeadCount, detail: '待处理、待看房', tab: '线索管理', icon: Collection },
  { label: '待联系租客需求', count: props.newRequestCount, detail: '与需求列表的待处理筛选一致', tab: '租客需求', icon: Collection },
  { label: '近期收藏记录', count: props.favoriteCount, detail: '最近最多 200 条，非全量总数', tab: '用户意向', icon: Collection },
  { label: '微信用户', count: props.userCount, detail: '微信用户列表中的记录', tab: '微信用户', icon: User },
])
</script>

<template>
  <section class="page-header">
    <div><span>业务总览</span><h1>运营总览</h1><p>查看业务记录和待跟进事项。</p></div>
    <el-button :loading="state === 'loading'" @click="emit('refresh')">刷新数据</el-button>
  </section>
  <el-alert v-if="state === 'error'" :title="`运营数据加载失败：${errorMessage || '请刷新重试'}`" type="error" show-icon :closable="false" />
  <div v-loading="state === 'loading'" class="stats-grid dashboard-grid" :aria-busy="state === 'loading'">
    <button v-for="card in cards" :key="card.tab" class="surface stat-card" :disabled="state !== 'ready'" @click="emit('navigate', card.tab)">
      <el-icon><component :is="card.icon" /></el-icon>
      <span>{{ card.label }}</span>
      <strong>{{ state === 'ready' ? card.count : '—' }}</strong>
      <small>{{ card.detail }}</small>
    </button>
  </div>
</template>
