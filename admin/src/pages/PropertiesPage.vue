<script setup lang="ts">
import { House } from '@element-plus/icons-vue'
import type { Property } from '../types/domain'

type Status = { key: string; label: string; icon: string }
const props = defineProps<{ properties: Property[]; filteredProperties: Property[]; filter: string; statuses: Status[]; loading: boolean; normalizedStatus: (value: string) => string; statusLabel: (value: string) => string; cover: (value: Property) => string }>()
const emit = defineEmits<{ refresh: []; filter: [value: string]; status: [item: Property, value: string] }>()
</script>

<template>
  <section class="page-header"><div><span>房源管理</span><h1>房源列表</h1></div><el-button :loading="loading" @click="emit('refresh')">刷新数据</el-button></section>
  <div class="property-status-grid"><button class="surface property-status-card" :class="{ active: filter === 'all' }" @click="emit('filter', 'all')"><i>全</i><span>全部</span><strong>{{ properties.length }}</strong></button><button v-for="status in statuses" :key="status.key" class="surface property-status-card" :class="{ active: filter === status.key }" @click="emit('filter', status.key)"><i>{{ status.icon }}</i><span>{{ status.label }}</span><strong>{{ properties.filter(item => normalizedStatus(item.status) === status.key).length }}</strong></button></div>
  <section class="surface property-list-panel"><div class="table-heading"><div><h2>{{ filter === 'all' ? '全部房源' : statusLabel(filter) }}</h2><p>共 {{ filteredProperties.length }} 套</p></div></div><div v-if="filteredProperties.length" class="property-icon-list"><article v-for="item in filteredProperties" :key="item.id" class="property-icon-item"><div class="property-cover"><img v-if="cover(item)" :src="cover(item)"/><el-icon v-else><House /></el-icon></div><div class="property-copy"><strong>{{ item.community || item.title }}</strong><span>{{ item.layout || '户型待补充' }} · {{ item.location || '位置待补充' }}</span><b>{{ item.rentMonthly > 0 ? `${item.rentMonthly} 元/月` : '租金待补充' }}</b></div><el-tag>{{ statusLabel(item.status) }}</el-tag><el-select :model-value="normalizedStatus(item.status)" size="small" @change="emit('status', item, $event)"><el-option v-for="status in statuses" :key="status.key" :label="status.label" :value="status.key"/></el-select></article></div><el-empty v-else description="该状态下暂无房源"/></section>
</template>
