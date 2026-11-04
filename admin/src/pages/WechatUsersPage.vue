<script setup lang="ts">
import type { WechatUser } from '../types/domain'

defineProps<{ users: WechatUser[]; loading: boolean }>()
const emit = defineEmits<{ refresh: [] }>()
const formatTime = (value: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
</script>

<template>
  <section class="page-header"><div><span>用户与权限</span><h1>微信用户</h1></div><el-button :loading="loading" @click="emit('refresh')">刷新数据</el-button></section>
  <section class="surface table-panel">
    <div class="table-heading"><div><h2>微信用户列表</h2><p>共 {{ users.length }} 人，已绑定手机号 {{ users.filter((item) => item.phone).length }} 人</p></div></div>
    <el-table v-loading="loading" :data="users" empty-text="暂无微信用户">
      <el-table-column label="用户" min-width="220"><template #default="scope"><div class="user-cell"><el-avatar :size="38" :src="scope.row.avatarUrl">微</el-avatar><div><strong>{{ scope.row.nickname || '微信用户' }}</strong><span>{{ scope.row.id }}</span></div></div></template></el-table-column>
      <el-table-column label="手机号" min-width="160"><template #default="scope"><span v-if="scope.row.phone">{{ scope.row.phone }}</span><el-tag v-else type="info">未绑定</el-tag></template></el-table-column>
      <el-table-column label="状态" width="120"><template #default="scope"><el-tag :type="scope.row.status === 'active' ? 'success' : 'danger'">{{ scope.row.status === 'active' ? '正常' : '已停用' }}</el-tag></template></el-table-column>
      <el-table-column label="首次登录" min-width="180"><template #default="scope">{{ formatTime(scope.row.createdAt) }}</template></el-table-column>
    </el-table>
  </section>
</template>
