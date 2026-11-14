<script setup lang="ts">
import { isNewRentalRequest } from '../config/operationalMetrics'
import { ref } from 'vue'
import type { RentalRequest } from '../types/domain'
const props = defineProps<{ requests: RentalRequest[]; loading: boolean; filter: string }>()
const emit = defineEmits<{ refresh: []; filter: [value: string] }>()
const visibleRequests = () => props.filter === 'new' ? props.requests.filter(isNewRentalRequest) : props.requests
const selected = ref<RentalRequest | null>(null)
const statuses: Record<string, string> = { new: '待处理', pending: '待处理', contacted: '已联系', matched: '已匹配', completed: '已完成', closed: '已关闭', invalid: '无效', inspection_pending: '待看房' }
const types: Record<string, string> = { appointment: '预约看房', area_subscription: '片区新房预约', general: '帮我找房' }
const formatTime = (value: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
</script>

<template>
  <section class="page-header"><div><span>业务管理</span><h1>租客需求</h1></div><el-button :loading="loading" @click="emit('refresh')">刷新数据</el-button></section>
  <div class="stats-grid list-stats"><section class="surface compact-stat"><span>需求总数</span><strong>{{ requests.length }}</strong></section><section class="surface compact-stat"><span>预约房源</span><strong>{{ requests.filter(item => item.requestType === 'appointment').length }}</strong></section><section class="surface compact-stat"><span>自主找房</span><strong>{{ requests.filter(item => item.requestType === 'general').length }}</strong></section></div>
  <section class="surface table-panel"><div class="table-heading"><div><h2>需求记录</h2><p>显示 {{ visibleRequests().length }} / {{ requests.length }} 条；优先联系新提交的用户</p></div><el-select :model-value="filter" aria-label="筛选需求" @change="emit('filter', $event)"><el-option label="全部需求" value="all"/><el-option label="待处理" value="new"/></el-select></div><el-table :data="visibleRequests()" empty-text="暂无符合条件的租客需求"><el-table-column label="类型" width="130"><template #default="scope"><el-tag :type="scope.row.requestType === 'appointment' ? 'warning' : 'success'">{{ scope.row.requestType === 'appointment' ? '预约看房' : scope.row.requestType === 'area_subscription' ? '片区新房预约' : '帮我找房' }}</el-tag></template></el-table-column><el-table-column prop="phone" label="联系电话" min-width="145"><template #default="scope">{{ scope.row.phone || '待绑定' }}</template></el-table-column><el-table-column label="地区与户型" min-width="170"><template #default="scope">{{ scope.row.location || '未指定地区' }}<br/><span class="muted-table-text">{{ scope.row.layout || '户型不限' }}</span></template></el-table-column><el-table-column label="月租预算" min-width="130"><template #default="scope">{{ scope.row.budgetMin || 0 }}–{{ scope.row.budgetMax || '不限' }} 元</template></el-table-column><el-table-column label="补充要求" min-width="180"><template #default="scope"><span>{{ scope.row.note || '—' }}</span><small v-if="scope.row.followUpNote" class="muted-table-text">跟进：{{ scope.row.followUpNote }}</small></template></el-table-column><el-table-column label="跟进状态" min-width="110"><template #default="{ row }">{{ statuses[row.status] || row.status || '未知' }}</template></el-table-column><el-table-column label="操作" width="100"><template #default="{ row }"><el-button link type="primary" @click="selected = row">查看详情</el-button></template></el-table-column><el-table-column label="提交时间" min-width="165"><template #default="scope">{{ formatTime(scope.row.createdAt) }}</template></el-table-column></el-table></section>
  <el-dialog :model-value="!!selected" title="租客需求详情" width="min(560px, 92vw)" @close="selected = null">
    <el-descriptions v-if="selected" :column="2" border>
      <el-descriptions-item label="类型">{{ types[selected.requestType] || selected.requestType }}</el-descriptions-item>
      <el-descriptions-item label="联系电话">{{ selected.phone || '待绑定' }}</el-descriptions-item>
      <el-descriptions-item label="地区">{{ selected.location || '未指定' }}</el-descriptions-item>
      <el-descriptions-item label="户型">{{ selected.layout || '不限' }}</el-descriptions-item>
      <el-descriptions-item label="月租预算">{{ selected.budgetMin || 0 }}–{{ selected.budgetMax || '不限' }} 元</el-descriptions-item>
      <el-descriptions-item label="跟进状态">{{ statuses[selected.status] || selected.status || '未知' }}</el-descriptions-item>
      <el-descriptions-item label="补充要求" :span="2">{{ selected.note || '—' }}</el-descriptions-item>
      <el-descriptions-item label="跟进备注" :span="2">{{ selected.followUpNote || '—' }}</el-descriptions-item>
      <el-descriptions-item label="提交时间" :span="2">{{ formatTime(selected.createdAt) }}</el-descriptions-item>
    </el-descriptions>
    <template #footer><el-button @click="selected = null">关闭</el-button></template>
  </el-dialog>
</template>
