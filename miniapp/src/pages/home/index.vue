<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { API_ORIGIN } from "../../api/client";
import { getPublicConfig, listProperties, type Property, type PublicConfig } from "../../api/properties";

const defaultConfig: PublicConfig = { heroTitle: "小皮租房", heroSubtitle: "在武汉，住得更省心", heroDescription: "真实房源 · 可靠服务", notice: "签约前请核对合同条款与费用明细，保障自身权益。", contactPhone: "", relatedPropertyRadiusKm: 3, locations: [], featuredChannels: [{ icon: "🏢", title: "金融港", subtitle: "通勤优选", keyword: "金融港", latitude: 30.45604, longitude: 114.425337, radiusKm: 10 }, { icon: "🌳", title: "光谷", subtitle: "品质房源", keyword: "光谷", radiusKm: 5 }, { icon: "🌆", title: "街道口", subtitle: "生活便利", keyword: "街道口", radiusKm: 5 }] };
const properties = ref<Property[]>([]);
const config = ref<PublicConfig>(defaultConfig);
const featuredProperty = computed(() => properties.value[0]);
const coverUrl = (item?: Property) => {
  const cover = item?.media?.find((media) => media.type === "image")?.url;
  return cover ? (cover.startsWith("http") ? cover : `${API_ORIGIN}${cover}`) : "";
};
onShow(async () => {
  const [propertyResult, configResult] = await Promise.allSettled([listProperties(), getPublicConfig()]);
  if (propertyResult.status === "fulfilled") properties.value = propertyResult.value;
  if (configResult.status === "fulfilled") config.value = { ...defaultConfig, ...configResult.value };
});
const open = (id: string) => uni.navigateTo({ url: `/pages/properties/detail?id=${id}` });
const browse = () => uni.switchTab({ url: "/pages/properties/index" });
const requestHome = () => uni.navigateTo({ url: "/pages/requests/index" });
const openChannel = (channel: PublicConfig["featuredChannels"][number]) => {
  uni.setStorageSync("xiaopi_property_filter", JSON.stringify({ keyword: channel.keyword, latitude: channel.latitude || 0, longitude: channel.longitude || 0, radiusKm: channel.radiusKm || 5, title: channel.title }));
  browse();
};
</script>

<template>
  <view class="page home-page">
    <view class="topbar"><strong>{{ config.heroTitle }}</strong><text>武汉 · 光谷⌄</text></view>
    <view class="intro"><view><text>今天想住哪里？</text><small>新上 {{ properties.length }} 套真实房源</small></view><view class="intro-mark">xp</view></view>
    <view class="search-entry" @click="browse"><text>⌕</text><span>搜索小区、地铁、商圈</span><b>找房</b></view>
    <view class="quick-actions"><view @click="browse"><b>⌖</b><span>地图找房</span><small>按位置看</small></view><view @click="requestHome"><b>✦</b><span>帮我找房</span><small>说说你的需求</small></view></view>
    <view v-if="config.notice" class="notice"><text>提示</text><span>{{ config.notice }}</span></view>
    <view class="section-head"><strong>本周上新</strong><text @click="browse">全部房源 →</text></view>
    <view v-if="featuredProperty" class="feature" @click="open(featuredProperty.id)">
      <image v-if="coverUrl(featuredProperty)" :src="coverUrl(featuredProperty)" mode="aspectFill" />
      <view v-else class="feature-placeholder"><text>NEW</text><b>{{ featuredProperty.community || "小皮精选" }}</b></view>
      <view class="feature-info"><text>本周推荐</text><strong>{{ featuredProperty.title || featuredProperty.community }}</strong><small>{{ featuredProperty.layout }} · {{ featuredProperty.location }}</small><b>¥ {{ featuredProperty.rentMonthly }}<i>/ 月</i></b></view>
    </view>
    <view v-else class="feature"><view class="feature-placeholder"><text>NEW</text><b>正在整理新房源</b></view></view>
    <view class="section-head channel-head"><strong>住进你的生活圈</strong><text>精选频道</text></view>
    <scroll-view scroll-x class="channels-scroll"><view class="channels"><view v-for="channel in config.featuredChannels" :key="`${channel.title}-${channel.keyword}`" class="channel" @click="openChannel(channel)"><text>{{ channel.icon || "⌂" }}</text><strong>{{ channel.title }}</strong><small>{{ channel.subtitle }}</small><b>查看 →</b></view></view></scroll-view>
  </view>
</template>

<style scoped>
.home-page { padding-top:36rpx; }.topbar,.intro,.section-head{display:flex;align-items:center;justify-content:space-between}.topbar strong{color:#26372b;font-size:34rpx;font-weight:800;letter-spacing:-1rpx}.topbar text{color:#778078;font-size:23rpx}.intro{margin:42rpx 0 30rpx}.intro>view:first-child text,.intro>view:first-child small{display:block}.intro>view:first-child text{color:#26372b;font-size:52rpx;font-weight:900;letter-spacing:-2rpx}.intro>view:first-child small{margin-top:12rpx;color:#829085;font-size:24rpx}.intro-mark{display:flex;width:86rpx;height:86rpx;align-items:center;justify-content:center;border-radius:28rpx 28rpx 8rpx;background:#d9e3bb;color:#4e643c;font-size:29rpx;font-weight:900}.search-entry{display:flex;align-items:center;gap:16rpx;padding:15rpx 16rpx 15rpx 26rpx;border:1rpx solid #e4e2dc;border-radius:22rpx;background:#fff;box-shadow:0 12rpx 32rpx rgba(52,60,45,.05)}.search-entry>text{font-size:42rpx;color:#5d6d62}.search-entry span{flex:1;color:#9aa19b;font-size:26rpx}.search-entry b{padding:15rpx 22rpx;border-radius:15rpx;background:#40543f;color:#fff;font-size:25rpx}.quick-actions{display:grid;grid-template-columns:1fr 1fr;gap:16rpx;margin:22rpx 0}.quick-actions>view{min-height:142rpx;padding:23rpx;border-radius:24rpx;overflow:hidden;background:#e9eee0}.quick-actions>view:last-child{background:#f2e4d9}.quick-actions b,.quick-actions span,.quick-actions small{display:block}.quick-actions b{color:#586f45;font-size:34rpx}.quick-actions>view:last-child b{color:#bd755c}.quick-actions span{margin-top:13rpx;color:#2d3a2e;font-size:28rpx;font-weight:800}.quick-actions small{margin-top:5rpx;color:#7f897d;font-size:21rpx}.notice{display:flex;gap:14rpx;margin:20rpx 0 42rpx;color:#7e837d;font-size:22rpx;line-height:1.55}.notice text{flex:none;padding:2rpx 10rpx;height:31rpx;border-radius:8rpx;background:#e4e8de;color:#61705b;font-size:19rpx}.notice span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.section-head{margin:0 2rpx 20rpx}.section-head strong{color:#29382d;font-size:36rpx;letter-spacing:-1rpx}.section-head text{color:#748071;font-size:23rpx}.feature{position:relative;height:392rpx;overflow:hidden;border-radius:30rpx;background:#dbe3d3}.feature image{width:100%;height:100%}.feature-placeholder{display:flex;height:100%;padding:38rpx;box-sizing:border-box;flex-direction:column;justify-content:space-between;background:linear-gradient(145deg,#dde7c2,#9db489);color:#35513a}.feature-placeholder text{font-size:21rpx;letter-spacing:3rpx}.feature-placeholder b{max-width:440rpx;font-size:48rpx;line-height:1.15;letter-spacing:-2rpx}.feature-info{position:absolute;left:0;right:0;bottom:0;padding:24rpx 28rpx;background:linear-gradient(transparent,rgba(28,38,28,.84));color:#fff}.feature-info text,.feature-info strong,.feature-info small,.feature-info b{display:block}.feature-info text{width:max-content;padding:6rpx 12rpx;border-radius:8rpx;background:#b9ce78;color:#31402d;font-size:19rpx}.feature-info strong{margin-top:10rpx;font-size:33rpx}.feature-info small{margin-top:5rpx;color:rgba(255,255,255,.76);font-size:22rpx}.feature-info b{margin-top:8rpx;color:#ffd397;font-size:31rpx}.feature-info i{font-style:normal;font-size:20rpx}.channel-head{margin-top:48rpx}.channels-scroll{width:100%;white-space:nowrap}.channels{display:inline-flex;gap:16rpx;padding-bottom:10rpx}.channel{width:224rpx;min-height:210rpx;padding:24rpx;border:1rpx solid #e8e5df;border-radius:24rpx;box-sizing:border-box;background:#fff;white-space:normal}.channel>text{font-size:42rpx}.channel strong,.channel small,.channel b{display:block}.channel strong{margin-top:17rpx;color:#334134;font-size:29rpx}.channel small{margin-top:5rpx;color:#8a9389;font-size:21rpx}.channel b{margin-top:20rpx;color:#556c45;font-size:21rpx}
</style>
