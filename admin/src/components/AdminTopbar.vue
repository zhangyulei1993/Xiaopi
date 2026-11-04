<script setup lang="ts">
import { SwitchButton } from '@element-plus/icons-vue'

defineProps<{ title: string }>()
defineEmits<{ logout: [] }>()
</script>

<template>
  <header class="topbar">
    <div>
      <strong>{{ title }}</strong>
      <span>小皮租房运营平台</span>
    </div>
    <div class="actions">
      <div class="admin-identity">
        <span class="avatar">管</span>
        <div><strong>管理员</strong><small>平台运营</small></div>
      </div>
      <el-button text :icon="SwitchButton" @click="$emit('logout')">退出</el-button>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 72px;
  padding: 12px 28px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(255, 255, 255, 0.78);
  backdrop-filter: blur(20px);
}
.topbar > div:first-child > strong { display: block; color: var(--color-primary); font-size: 20px; letter-spacing: -0.01em; }
.topbar > div:first-child > span { display: block; margin-top: 3px; color: var(--color-text-secondary); font-size: 12px; }
.actions, .admin-identity { display: flex; align-items: center; gap: 12px; }
.admin-identity { padding-right: 14px; border-right: 1px solid var(--color-border); }
.avatar { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 10px; background: var(--color-heritage-green-soft); color: var(--color-heritage-green); font-weight: 800; }
.admin-identity strong, .admin-identity small { display: block; }
.admin-identity strong { color: var(--color-text-primary); font-size: 13px; }
.admin-identity small { margin-top: 2px; color: var(--color-text-secondary); font-size: 11px; }
@media (max-width: 640px) { .topbar { padding: 12px 16px; } .admin-identity { display: none; } }
</style>
