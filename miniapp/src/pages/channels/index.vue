<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { ref } from "vue";
import { getPublicConfig, type PublicConfig } from "../../api/properties";

const channels = ref<PublicConfig["featuredChannels"]>([]);
const loading = ref(true);
const loadFailed = ref(false);

const channelStyle = (icon: string, index: number) => {
  const legacy: Record<string, string> = { "🏢": "work", "🌳": "campus", "🌆": "life" };
  return legacy[icon] || (["commute", "campus", "work", "life"].includes(icon) ? icon : ["commute", "campus", "work", "life"][index % 4]);
};

const loadChannels = async () => {
  loading.value = true;
  loadFailed.value = false;
  try {
    const config = await getPublicConfig();
    channels.value = config?.featuredChannels || [];
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
};

const selectChannel = (channel: PublicConfig["featuredChannels"][number]) => {
  const hasCoordinates = Number(channel.latitude) !== 0 && Number(channel.longitude) !== 0;
  uni.setStorageSync("xiaopi_property_filter", JSON.stringify({
    keyword: channel.keyword || "",
    ...(hasCoordinates ? { latitude: channel.latitude, longitude: channel.longitude } : {}),
    radiusKm: channel.radiusKm || 3,
    title: channel.title,
  }));
  uni.switchTab({ url: "/pages/properties/index" });
};

onShow(() => { void loadChannels(); });
</script>

<template>
  <view class="channels-page">
    <view class="page-heading">
      <text class="eyebrow">按主题发现附近房源</text>
      <text class="page-title">精选频道</text>
      <text class="page-description">选择一个频道，查看对应范围内的房源</text>
    </view>

    <view v-if="loading" class="page-state">正在加载频道…</view>
    <view v-else-if="loadFailed" class="page-state">
      <text>频道暂时无法加载</text>
      <button class="retry-button" @click="loadChannels">重新加载</button>
    </view>
    <view v-else-if="channels.length" class="channel-grid">
      <view
        v-for="(channel, index) in channels"
        :key="channel.id || channel.title || index"
        class="channel-card"
        :class="`tone-${channelStyle(channel.icon, index)}`"
        @click="selectChannel(channel)"
      >
        <view class="channel-mark" :class="`mark-${channelStyle(channel.icon, index)}`"><view class="channel-glyph"></view></view>
        <text class="channel-title">{{ channel.title }}</text>
        <text class="channel-subtitle">{{ channel.subtitle || "发现附近好房" }}</text>
        <text class="channel-arrow">›</text>
      </view>
    </view>
    <view v-else class="page-state">暂时还没有精选频道</view>
  </view>
</template>

<style scoped>
.channels-page{min-height:100vh;padding:30rpx 28rpx 56rpx;background:#f7eee2;color:#354126;box-sizing:border-box}
.page-heading{display:flex;flex-direction:column;margin:10rpx 4rpx 28rpx}
.eyebrow{color:#78806b;font-size:22rpx;letter-spacing:2rpx}
.page-title{margin-top:9rpx;font-size:42rpx;font-weight:800;line-height:1.25}
.page-description{margin-top:9rpx;color:#7b8174;font-size:24rpx}
.channel-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18rpx}
.channel-card{position:relative;display:flex;min-height:190rpx;flex-direction:column;align-items:flex-start;justify-content:center;overflow:hidden;padding:22rpx;border-radius:22rpx;box-sizing:border-box;box-shadow:0 8rpx 22rpx rgba(60,62,40,.09)}
.channel-card:after{position:absolute;right:-24rpx;bottom:-54rpx;width:160rpx;height:160rpx;border:1rpx solid rgba(255,255,255,.18);border-radius:50%;content:""}
.tone-commute{background:linear-gradient(135deg,#dd8d6f,#bd624c);color:#fff}
.tone-campus{background:linear-gradient(135deg,#778b46,#415a2f);color:#fff}
.tone-work{background:linear-gradient(135deg,#d4ad78,#a9855a);color:#fff}
.tone-life{background:linear-gradient(135deg,#8a9b8b,#536e61);color:#fff}
.channel-mark{display:grid;width:48rpx;height:48rpx;place-items:center;border-radius:15rpx;background:rgba(255,255,255,.2)}
.channel-glyph{position:relative;width:24rpx;height:24rpx;border:3rpx solid rgba(255,255,255,.95);border-radius:50%;box-sizing:border-box}
.channel-glyph:after{position:absolute;top:6rpx;left:6rpx;width:6rpx;height:6rpx;border-radius:50%;background:rgba(255,255,255,.95);content:""}
.mark-campus .channel-glyph{border-radius:4rpx}.mark-campus .channel-glyph:after{top:8rpx;left:3rpx;width:12rpx;height:3rpx;border-radius:3rpx}
.mark-work .channel-glyph{border:0;border-radius:0;background:linear-gradient(90deg,rgba(255,255,255,.95) 0 28%,transparent 28% 37%,rgba(255,255,255,.95) 37% 63%,transparent 63% 72%,rgba(255,255,255,.95) 72%)}
.mark-work .channel-glyph:after{display:none}
.mark-life .channel-glyph{border-radius:50% 50% 5rpx 50%;transform:rotate(45deg)}
.channel-title{max-width:100%;overflow:hidden;margin-top:16rpx;font-size:29rpx;font-weight:800;line-height:1.3;text-overflow:ellipsis;white-space:nowrap}
.channel-subtitle{max-width:100%;overflow:hidden;margin-top:6rpx;color:rgba(255,255,255,.82);font-size:22rpx;text-overflow:ellipsis;white-space:nowrap}
.channel-arrow{position:absolute;right:20rpx;bottom:16rpx;color:rgba(255,255,255,.85);font-size:34rpx;line-height:1}
.page-state{display:flex;min-height:300rpx;flex-direction:column;align-items:center;justify-content:center;gap:20rpx;color:#7b8174;font-size:26rpx}
.retry-button{margin:0;padding:12rpx 26rpx;border:1rpx solid #d9dfd0;border-radius:999rpx;background:#fffdf9;color:#40543f;font-size:24rpx;line-height:1.3}
.retry-button:after{border:0}
</style>
