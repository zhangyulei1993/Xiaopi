<script setup lang="ts">
import type { FavoriteContact } from '../types/domain'

defineProps<{ contacts: FavoriteContact[]; loading: boolean }>()
const emit = defineEmits<{ refresh: [] }>()
const formatTime = (value: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
</script>

<template>
  <section class="page-header"><div><span>用户行为</span><h1>用户意向</h1></div><el-button :loading="loading" @click="emit('refresh')">刷新数据</el-button></section>
  <section class="surface table-panel"><div class="table-heading"><div><h2>房源收藏记录</h2><p>收藏仅代表用户兴趣，不要求绑定手机号</p></div></div><el-table :data="contacts" empty-text="暂无收藏记录"><el-table-column label="用户" min-width="210"><template #default="scope"><div class="user-cell"><el-avatar :size="38" :src="scope.row.avatarUrl">微</el-avatar><div><strong>{{ scope.row.nickname || '微信用户' }}</strong><span>{{ scope.row.userId }}</span></div></div></template></el-table-column><el-table-column prop="propertyName" label="关注房源" min-width="220"/><el-table-column label="手机号" min-width="150"><template #default="scope">{{ scope.row.phone || '未绑定' }}</template></el-table-column><el-table-column label="关注时间" min-width="180"><template #default="scope">{{ formatTime(scope.row.createdAt) }}</template></el-table-column></el-table></section>
</template>
