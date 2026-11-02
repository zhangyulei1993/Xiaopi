<script setup lang="ts">
import { Collection, DataAnalysis, House, Key, Location, User, UserFilled } from '@element-plus/icons-vue'

defineProps<{ active: string }>()
const emit = defineEmits<{ select: [value: string] }>()

const groups = [
  {
    label: '业务总览',
    items: [
      { key: '运营总览', label: '运营总览', icon: DataAnalysis },
      { key: '房源审核', label: '房源审核', icon: House },
      { key: '线索管理', label: '线索管理', icon: Collection },
      { key: '微信用户', label: '微信用户', icon: User },
    ],
  },
  {
    label: '内容与运营',
    items: [{ key: '小程序配置', label: '小程序配置', icon: Location }],
  },
  {
    label: '平台设置',
    items: [
      { key: '管理员列表', label: '管理员列表', icon: UserFilled },
      { key: '临时访问码', label: '临时访问码', icon: Key },
    ],
  },
]
</script>

<template>
  <aside class="sidebar">
    <div class="brand">
      <div class="brand-mark">小</div>
      <div>
        <strong>小皮租房</strong>
        <span>运营管理后台</span>
      </div>
    </div>
    <el-menu :default-active="active" class="menu" @select="emit('select', $event)">
      <el-menu-item-group v-for="group in groups" :key="group.label" :title="group.label">
        <el-menu-item v-for="item in group.items" :key="item.key" :index="item.key">
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
        </el-menu-item>
      </el-menu-item-group>
    </el-menu>
    <div class="sidebar-foot">
      <span>API 服务</span>
      <strong><i />150.158.157.213</strong>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  position: sticky;
  top: 0;
  display: flex;
  min-height: 100dvh;
  flex-direction: column;
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  background:
    radial-gradient(circle at 0 0, rgba(59, 110, 168, 0.18), transparent 210px),
    linear-gradient(180deg, #15243a 0%, #122033 100%);
  color: #fff;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 72px;
  padding: 0 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.12);
}

.brand-mark {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 12px;
  background: linear-gradient(135deg, #f2d39a, var(--color-warm-gold));
  box-shadow: 0 10px 24px rgba(200, 164, 93, 0.22);
  color: #17263e;
  font-weight: 800;
}

.brand strong { display: block; letter-spacing: 0.01em; }
.brand span { display: block; margin-top: 2px; color: rgba(255, 255, 255, 0.62); font-size: 12px; }
.menu { flex: 1; padding: 16px 10px 20px; border-right: 0; background: transparent; }

:deep(.el-menu-item-group__title) {
  padding: 14px 10px 7px;
  color: rgba(255, 255, 255, 0.42);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

:deep(.el-menu-item) {
  height: 46px;
  margin-bottom: 6px;
  border-radius: 14px;
  color: rgba(255, 255, 255, 0.76);
  transition: background 0.18s ease, color 0.18s ease, transform 0.18s ease;
}

:deep(.el-menu-item:hover),
:deep(.el-menu-item.is-active) {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
  transform: translateX(2px);
}

:deep(.el-menu-item.is-active) { box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08); }
.sidebar-foot { margin: 12px 20px 22px; padding-top: 16px; border-top: 1px solid rgba(255, 255, 255, 0.1); }
.sidebar-foot span { display: block; color: rgba(255, 255, 255, 0.42); font-size: 11px; letter-spacing: 0.08em; }
.sidebar-foot strong { display: flex; align-items: center; gap: 8px; margin-top: 8px; color: rgba(255, 255, 255, 0.76); font-size: 12px; font-weight: 600; }
.sidebar-foot i { width: 7px; height: 7px; border-radius: 50%; background: #76c9a8; box-shadow: 0 0 0 4px rgba(118, 201, 168, 0.12); }

@media (max-width: 900px) {
  .sidebar { position: static; min-width: 0; max-width: 100vw; min-height: auto; overflow: hidden; }
  .brand { min-height: 64px; }
  .menu { display: flex; width: 100%; max-width: 100vw; height: 58px; min-height: 58px; flex: none; gap: 8px; padding: 9px 12px; overflow-x: auto; scrollbar-width: none; }
  .menu::-webkit-scrollbar { display: none; }
  :deep(.el-menu-item-group) { flex: 0 0 auto; }
  :deep(.el-menu-item-group__title) { display: none; }
  :deep(.el-menu-item-group > ul) { display: flex; gap: 6px; }
  :deep(.el-menu-item) { height: 40px; margin: 0; padding: 0 14px; white-space: nowrap; }
  :deep(.el-menu-item:hover),
  :deep(.el-menu-item.is-active) { transform: none; }
  .sidebar-foot { display: none; }
}
</style>
