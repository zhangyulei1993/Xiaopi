<script setup lang="ts">
import { isPendingLead } from '../config/operationalMetrics'
import { computed, ref } from 'vue'
import type { Lead } from '../types/domain'

const props = defineProps<{ leads: Lead[]; loading: boolean; filter: string }>()
const emit = defineEmits<{ refresh: []; filter: [value: string] }>()
const visibleLeads = computed(() => props.filter === 'pending' ? props.leads.filter(isPendingLead) : props.filter === 'today' ? props.leads.filter(item => { const date = new Date(item.createdAt); const now = new Date(); return date.toDateString() === now.toDateString() }) : props.leads)
const selectedLead = ref<Lead | null>(null)
const todayCount = computed(() => props.leads.filter(item => {
  const date = new Date(item.createdAt)
  const now = new Date()
  return date.getFullYear() === now.getFullYear() && date.getMonth() === now.getMonth() && date.getDate() === now.getDate()
}).length)
const pendingCount = computed(() => props.leads.filter(isPendingLead).length)
const statusLabels: Record<string, string> = { new: '待处理', contacted: '已联系', inspection_pending: '待看房', converted: '已转房源', closed: '已关闭', matched: '已匹配', completed: '已完成', invalid: '无效' }
const statusType = (status: string) => status === 'converted' || status === 'contacted' || status === 'completed' ? 'success' : status === 'new' || status === 'inspection_pending' ? 'warning' : 'info'
const formatTime = (value: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
function openDetail(item: Lead) {
  selectedLead.value = item
}
</script>

<template>
  <section class="page-header"><div><span>业务管理</span><h1>线索管理</h1><p>查看用户提交的房源线索和跟进记录。</p></div><el-button :loading="loading" @click="emit('refresh')">刷新数据</el-button></section>
  <div class="stats-grid list-stats"><section class="surface compact-stat"><span>线索总数</span><strong>{{ leads.length }}</strong></section><section class="surface compact-stat"><span>今日新增</span><strong>{{ todayCount }}</strong></section><section class="surface compact-stat"><span>待处理（待处理/待看房）</span><strong>{{ pendingCount }}</strong></section></div>
  <section class="surface table-panel"><div class="table-heading"><div><h2>线索列表</h2><p>显示 {{ visibleLeads.length }} / {{ leads.length }} 条</p></div><el-select :model-value="filter" aria-label="筛选线索" @change="emit('filter', $event)"><el-option label="全部线索" value="all"/><el-option label="待跟进" value="pending"/><el-option label="今日新增" value="today"/></el-select></div><el-table v-loading="loading" :data="visibleLeads" empty-text="暂无符合条件的房源线索">
    <el-table-column label="提交人" min-width="150"><template #default="scope">{{ scope.row.sourceName || '未填写姓名' }}<br><span class="muted-table-text">{{ scope.row.sourcePhone || '未留电话' }}</span></template></el-table-column>
    <el-table-column label="房源信息" min-width="190"><template #default="scope">{{ scope.row.community || '未填写小区' }}<br><span class="muted-table-text">{{ scope.row.location || '未填写地区' }} · {{ scope.row.layout || '户型未填写' }}</span></template></el-table-column>
    <el-table-column label="期望月租" width="120"><template #default="scope">{{ scope.row.expectedRentMonthly ? `${scope.row.expectedRentMonthly} 元` : '未填写' }}</template></el-table-column>
    <el-table-column label="状态" width="130"><template #default="scope"><el-tag :type="statusType(scope.row.status)" effect="light">{{ statusLabels[scope.row.status] || scope.row.status || '未知' }}</el-tag></template></el-table-column>
    <el-table-column label="提交时间" min-width="170"><template #default="scope">{{ formatTime(scope.row.createdAt) }}</template></el-table-column>
    <el-table-column label="操作" width="100" align="center"><template #default="scope"><el-button link type="primary" @click="openDetail(scope.row)">查看详情</el-button></template></el-table-column>
  </el-table></section>
  <el-dialog :model-value="Boolean(selectedLead)" title="线索详情" width="560px" @close="selectedLead = null">
    <template v-if="selectedLead"><el-descriptions :column="2" border><el-descriptions-item label="提交人">{{ selectedLead.sourceName || '未填写' }}</el-descriptions-item><el-descriptions-item label="联系电话">{{ selectedLead.sourcePhone || '未填写' }}</el-descriptions-item><el-descriptions-item label="小区">{{ selectedLead.community || '未填写' }}</el-descriptions-item><el-descriptions-item label="地区">{{ selectedLead.location || '未填写' }}</el-descriptions-item><el-descriptions-item label="户型">{{ selectedLead.layout || '未填写' }}</el-descriptions-item><el-descriptions-item label="期望月租">{{ selectedLead.expectedRentMonthly ? `${selectedLead.expectedRentMonthly} 元` : '未填写' }}</el-descriptions-item><el-descriptions-item label="补充说明" :span="2">{{ selectedLead.note || '无' }}</el-descriptions-item><el-descriptions-item v-if="selectedLead.convertedPropertyId" label="关联房源 ID" :span="2">{{ selectedLead.convertedPropertyId }}</el-descriptions-item><el-descriptions-item label="提交时间" :span="2">{{ formatTime(selectedLead.createdAt) }}</el-descriptions-item><el-descriptions-item label="跟进状态">{{ statusLabels[selectedLead.status] || selectedLead.status || '未知' }}</el-descriptions-item><el-descriptions-item label="跟进备注" :span="2">{{ selectedLead.followUpNote || '无' }}</el-descriptions-item></el-descriptions></template>
    <template #footer><el-button @click="selectedLead = null">关闭</el-button></template>
  </el-dialog>
</template>

<style scoped>
.muted-table-text { color: var(--el-text-color-secondary); font-size: 12px; }
</style>
