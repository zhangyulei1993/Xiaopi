<script setup lang="ts">
import { Collection, House, Location, User } from '@element-plus/icons-vue'

defineProps<{ loading: boolean; propertyCount: number; leadCount: number; userCount: number; locationCount: number }>()
const emit = defineEmits<{ navigate: [tab: string] }>()
</script>

<template>
  <section class="dashboard-hero">
    <div><span class="dashboard-kicker">XIAOPI OPERATIONS</span><h1>运营总览</h1></div>
    <div class="hero-meter"><span>当前待跟进线索</span><strong>{{ leadCount }}</strong></div>
  </section>
  <div v-loading="loading" class="stats-grid dashboard-grid">
    <button class="surface stat-card" @click="emit('navigate', '房源列表')"><el-icon><House /></el-icon><span>房源总数</span><strong>{{ propertyCount }}</strong></button>
    <button class="surface stat-card" @click="emit('navigate', '线索管理')"><el-icon><Collection /></el-icon><span>客户线索</span><strong>{{ leadCount }}</strong></button>
    <button class="surface stat-card" @click="emit('navigate', '微信用户')"><el-icon><User /></el-icon><span>微信用户</span><strong>{{ userCount }}</strong></button>
    <button class="surface stat-card" @click="emit('navigate', '小程序基础配置')"><el-icon><Location /></el-icon><span>首页地区</span><strong>{{ locationCount }}</strong></button>
  </div>
</template>
