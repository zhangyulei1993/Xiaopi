<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { ref } from "vue";
import { API_ORIGIN } from "../../api/client";
import { getPublicConfig, listProperties, type Property, type PublicConfig } from "../../api/properties";

const defaultConfig: PublicConfig = { heroTitle: "小皮租房", heroSubtitle: "在武汉，住得更省心", heroDescription: "真实房源 · 可靠服务", notice: "", contactPhone: "", relatedPropertyRadiusKm: 3, locations: [{ label: "金融港", keyword: "金融港" }, { label: "光谷天地", keyword: "光谷天地" }], featuredChannels: [{ icon: "▣", title: "地铁沿线", subtitle: "通勤省心", keyword: "地铁", radiusKm: 5 }, { icon: "◇", title: "高校周边", subtitle: "生活方便", keyword: "高校", radiusKm: 5 }, { icon: "□", title: "职场优选", subtitle: "离公司更近", keyword: "金融港", radiusKm: 5 }] };
const priceRanges = ["¥1,500 以下", "¥1,500–2,500", "¥2,500–4,000", "¥4,000+"];
const properties = ref<Property[]>([]);
const config = ref<PublicConfig>(defaultConfig);
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
  <view class="home-page">
    <view class="hero">
      <view class="status"><text>9:41</text><text>⌁ ◉ ▰</text></view>
      <view class="city">●&nbsp; 武汉⌄</view>
      <view class="hero-copy"><strong>今天想住哪里？</strong><i></i></view>
      <view class="hero-search" @click="browse"><text>⌕</text><span>搜索小区 / 地铁 / 商圈 / 房源关键词</span><b>搜索</b></view>
    </view>
    <scroll-view scroll-x class="area-scroll"><view class="area-row"><text class="selected" @click="browse">● {{ config.locations[0]?.label || "金融港" }}</text><text v-for="location in config.locations.slice(1)" :key="location.label" @click="browse">{{ location.label }}</text><text @click="browse">附近</text></view></scroll-view>
    <scroll-view scroll-x class="price-scroll"><view class="price-row"><b>▣ 租金区间</b><text v-for="price in priceRanges" :key="price" @click="browse">{{ price }}</text><text @click="browse">⌄</text></view></scroll-view>

    <view class="section-head"><strong>精选频道</strong><text>查看更多 ›</text></view>
    <scroll-view scroll-x class="channel-scroll"><view class="channels"><view v-for="(channel, index) in config.featuredChannels" :key="channel.title" class="channel" :class="`tone-${index % 4}`" @click="openChannel(channel)"><text>{{ channel.icon || "⌂" }}</text><strong>{{ channel.title }}</strong><small>{{ channel.subtitle }}</small><b>›</b></view></view></scroll-view>

    <view class="recommend-head"><view><strong>推荐</strong><text>整租</text><text>合租</text><text>一居</text><text>两居</text><text>三居+</text></view><b @click="browse">⌘ 筛选</b></view>
    <view v-if="properties.length" class="property-list"><view v-for="item in properties.slice(0, 4)" :key="item.id" class="listing" @click="open(item.id)"><view class="listing-image"><image v-if="coverUrl(item)" :src="coverUrl(item)" mode="aspectFill"/><view v-else class="listing-placeholder"><span>{{ item.community?.slice(0, 2) || "小皮" }}</span></view><i>♡</i></view><view class="listing-info"><strong>{{ item.title || item.community }}</strong><small>{{ item.layout || "户型待补充" }} · {{ item.location }}</small><view class="tags"><text>真实核验</text><text>{{ item.feeText || "拎包入住" }}</text></view><b>¥ {{ item.rentMonthly }}<i>/ 月</i></b></view></view></view>
    <view v-else class="empty">新房源正在整理中</view>
    <view class="request-fab" @click="requestHome">✎&nbsp; 找房需求</view>
  </view>
</template>

<style scoped>
.home-page{min-height:100vh;background:#f7eee2;color:#354126;padding-bottom:120rpx}.hero{height:546rpx;padding:20rpx 30rpx 0;box-sizing:border-box;background:linear-gradient(90deg,rgba(249,233,215,.97) 0%,rgba(249,233,215,.56) 53%,rgba(249,233,215,.06) 100%),url("../../static/home-lifestyle-hero.png") center/cover}.status{display:flex;justify-content:space-between;color:#252d25;font-size:22rpx;font-weight:700}.city{display:inline-block;margin-top:28rpx;padding:13rpx 20rpx;border-radius:999rpx;background:rgba(255,255,255,.65);color:#445031;font-size:26rpx;font-weight:700}.hero-copy{margin-top:40rpx}.hero-copy strong{display:block;font-size:52rpx;letter-spacing:-3rpx}.hero-copy i{display:block;width:150rpx;height:9rpx;margin:15rpx 0 32rpx 110rpx;border-radius:999rpx;background:#d66d50;transform:rotate(-3deg)}.hero-search{display:flex;align-items:center;gap:14rpx;padding:14rpx 15rpx 14rpx 26rpx;border-radius:24rpx;background:rgba(255,255,255,.9);box-shadow:0 12rpx 26rpx rgba(91,65,40,.08)}.hero-search>text{font-size:42rpx}.hero-search span{flex:1;color:#808078;font-size:24rpx;white-space:nowrap}.hero-search b{padding:17rpx 24rpx;border-radius:18rpx;background:#d66d50;color:#fff;font-size:26rpx}.area-scroll,.price-scroll,.channel-scroll{width:100%;white-space:nowrap}.area-row{display:inline-flex;gap:14rpx;padding:22rpx 28rpx 18rpx}.area-row text{padding:14rpx 24rpx;border-radius:999rpx;background:#fbf5ee;color:#414538;font-size:25rpx}.area-row .selected{background:#5e6d2e;color:#fff}.price-row{display:inline-flex;align-items:center;margin:0 28rpx;padding:17rpx 18rpx;border-radius:20rpx;background:#fffaf4;box-shadow:0 6rpx 18rpx rgba(94,72,48,.05)}.price-row b,.price-row text{padding:0 16rpx;border-right:1rpx solid #e3ddd4;font-size:23rpx}.price-row b{color:#4d5639}.price-row text:last-child{border:0}.section-head{display:flex;justify-content:space-between;align-items:center;margin:52rpx 30rpx 22rpx}.section-head strong{font-size:39rpx;letter-spacing:-2rpx}.section-head text{font-size:23rpx;color:#737b5c}.channels{display:inline-flex;gap:16rpx;padding:0 30rpx}.channel{position:relative;width:190rpx;height:170rpx;padding:20rpx;box-sizing:border-box;border-radius:22rpx;color:#fff;overflow:hidden}.channel>text{display:block;font-size:34rpx}.channel strong,.channel small,.channel b{display:block}.channel strong{margin-top:20rpx;font-size:27rpx}.channel small{margin-top:5rpx;font-size:19rpx;color:rgba(255,255,255,.82)}.channel b{position:absolute;bottom:16rpx;left:20rpx;width:31rpx;height:31rpx;border-radius:50%;background:rgba(255,255,255,.9);color:#63713a;text-align:center;line-height:29rpx}.tone-0{background:linear-gradient(135deg,#dd8d6f,#bd624c)}.tone-1{background:linear-gradient(135deg,#778b46,#415a2f)}.tone-2{background:linear-gradient(135deg,#d4ad78,#a9855a)}.tone-3{background:linear-gradient(135deg,#a2a77a,#68704f)}.recommend-head{display:flex;justify-content:space-between;align-items:center;margin:54rpx 30rpx 22rpx}.recommend-head>view{display:flex;align-items:center;gap:26rpx}.recommend-head strong{font-size:39rpx;letter-spacing:-2rpx}.recommend-head strong:after{display:block;width:62rpx;height:7rpx;margin-top:7rpx;border-radius:999rpx;background:#d66d50;content:""}.recommend-head text{font-size:25rpx;color:#707467}.recommend-head>b{padding:13rpx 17rpx;border-radius:18rpx;background:#fff8ee;color:#454b3e;font-size:23rpx}.property-list{padding:0 24rpx}.listing{display:flex;gap:22rpx;margin-bottom:20rpx;padding:14rpx;border-radius:26rpx;background:#fffaf4;box-shadow:0 10rpx 24rpx rgba(100,75,46,.05)}.listing-image{position:relative;flex:none;width:330rpx;height:224rpx;border-radius:18rpx;overflow:hidden;background:#dfd4be}.listing-image image{width:100%;height:100%}.listing-image i{position:absolute;top:13rpx;right:13rpx;width:48rpx;height:48rpx;border-radius:50%;background:rgba(65,55,40,.36);color:#fff;font-size:34rpx;font-style:normal;text-align:center;line-height:46rpx}.listing-placeholder{display:flex;width:100%;height:100%;align-items:center;justify-content:center;background:linear-gradient(145deg,#e4cdaa,#b9a58c);color:#fff}.listing-placeholder span{padding:12rpx 17rpx;border:1rpx solid rgba(255,255,255,.5);font-size:28rpx}.listing-info{min-width:0;flex:1;padding:5rpx 5rpx 0 0}.listing-info strong,.listing-info small,.listing-info>b{display:block}.listing-info strong{overflow:hidden;color:#34392e;font-size:30rpx;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.listing-info small{margin-top:12rpx;color:#70766c;font-size:22rpx;line-height:1.45}.tags{display:flex;gap:9rpx;margin-top:12rpx;overflow:hidden}.tags text{flex:none;padding:7rpx 10rpx;border-radius:8rpx;background:#edf0dc;color:#63703b;font-size:18rpx}.tags text:last-child{background:#f9e5dc;color:#b7644b}.listing-info>b{margin-top:15rpx;color:#d66d50;font-size:35rpx}.listing-info>b i{font-size:20rpx;font-style:normal}.empty{margin:0 30rpx;padding:80rpx 0;border-radius:24rpx;background:#fffaf4;color:#8c8b80;text-align:center}.request-fab{position:fixed;z-index:4;right:30rpx;bottom:38rpx;padding:20rpx 27rpx;border-radius:999rpx;background:#5e6d2e;color:#fff;box-shadow:0 10rpx 28rpx rgba(58,65,26,.28);font-size:28rpx;font-weight:700}
</style>
