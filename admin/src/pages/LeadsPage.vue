<script setup lang="ts">
import type { Lead } from '../types/domain'
defineProps<{ leads: Lead[]; loading: boolean }>()
const emit = defineEmits<{ refresh: [] }>()
const statusType = (status: string) => status === '正常' || status === 'PUBLISHED' ? 'success' : status === 'PENDING' ? 'warning' : 'info'
</script>

<template>
  <section class="page-header"><div><span>业务管理</span><h1>线索管理</h1><p>查看用户提交的房源线索，优先处理状态异常或信息不完整的记录。</p></div><el-button :loading="loading" @click="emit('refresh')">刷新数据</el-button></section>
  <div class="stats-grid list-stats"><section class="surface compact-stat"><span>线索总数</span><strong>{{ leads.length }}</strong></section><section class="surface compact-stat"><span>今日新增</span><strong>0</strong></section><section class="surface compact-stat"><span>待处理事项</span><strong>{{ leads.length }}</strong></section></div>
  <section class="surface table-panel"><div class="table-heading"><div><h2>线索列表</h2><p>共 {{ leads.length }} 条记录</p></div></div><el-table v-loading="loading" :data="leads" empty-text="暂无数据"><el-table-column prop="sourceName" label="名称" min-width="180"/><el-table-column label="信息" min-width="240"><template #default="scope">{{ scope.row.community }} · {{ scope.row.sourcePhone }}</template></el-table-column><el-table-column label="状态" width="140"><template #default="scope"><el-tag :type="statusType(scope.row.status)" effect="light">{{ scope.row.status || '未知' }}</el-tag></template></el-table-column><el-table-column label="操作" width="120" align="center"><template #default><el-button link type="primary">查看</el-button></template></el-table-column></el-table></section>
</template>
