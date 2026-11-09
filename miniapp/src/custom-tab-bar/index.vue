<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";

const items = [
  { text: "首页", route: "pages/home/index", icon: "/static/tabbar/home.png", activeIcon: "/static/tabbar/home-active.png" },
  { text: "房源", route: "pages/properties/index", icon: "/static/tabbar/house.png", activeIcon: "/static/tabbar/house-active.png" },
  { text: "我的", route: "pages/me/index", icon: "/static/tabbar/me.png", activeIcon: "/static/tabbar/me-active.png" },
];
const active = ref(0);
const sync = (route?: string | number) => {
  const current = typeof route === "string"
    ? route
    : (getCurrentPages()[getCurrentPages().length - 1] as { route?: string } | undefined)?.route || "";
  const index = items.findIndex((item) => item.route === current);
  if (index >= 0) active.value = index;
};
const select = (index: number) => {
  if (active.value === index) return;
  active.value = index;
  uni.switchTab({ url: `/${items[index].route}` });
};
onMounted(() => {
  sync();
  uni.$on("xiaopi-tab-active", sync);
});
onBeforeUnmount(() => uni.$off("xiaopi-tab-active", sync));
</script>

<template>
  <view class="tab-shell">
    <view class="tab-dock">
      <view v-for="(item, index) in items" :key="item.route" class="tab-item" :class="{ active: active === index }" @click="select(index)">
        <view class="tab-icon"><image :src="active === index ? item.activeIcon : item.icon" mode="aspectFit" /></view>
        <text>{{ item.text }}</text>
      </view>
    </view>
  </view>
</template>

<style scoped>
.tab-shell{position:fixed;z-index:99;right:0;bottom:0;left:0;padding:0 20rpx calc(12rpx + env(safe-area-inset-bottom));background:linear-gradient(transparent,rgba(247,246,241,.95) 38%,#f7f6f1 100%);pointer-events:none}.tab-dock{display:grid;grid-template-columns:repeat(3,1fr);align-items:center;min-height:100rpx;padding:8rpx;border:1rpx solid rgba(224,228,215,.96);border-radius:28rpx;background:rgba(255,253,249,.97);box-shadow:0 12rpx 30rpx rgba(45,57,43,.13);pointer-events:auto}.tab-item{display:flex;min-width:0;height:84rpx;flex-direction:column;align-items:center;justify-content:center;gap:3rpx;border-radius:20rpx;color:#899287}.tab-icon{display:grid;place-items:center;width:44rpx;height:40rpx;border-radius:13rpx}.tab-icon image{width:31rpx;height:31rpx}.tab-item text{font-size:19rpx;font-weight:600;line-height:1.2}.tab-item.active{color:#40543f;font-weight:800}.tab-item.active .tab-icon{background:#e7edcf;box-shadow:inset 0 0 0 1rpx rgba(114,134,83,.13)}.tab-item:active{transform:scale(.96);opacity:.8}
</style>
