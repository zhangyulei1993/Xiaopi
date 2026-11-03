<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { API_ORIGIN } from "../../api/client";
import { getPublicConfig, listProperties, type Property, type PublicConfig } from "../../api/properties";
import pinIcon from "../../static/icons/pin.svg";
import searchIcon from "../../static/icons/search.svg";
import heartIcon from "../../static/icons/heart.svg";
import editIcon from "../../static/icons/edit.svg";
import heroFallback from "../../static/home-lifestyle-hero.png";
import { getSession } from "../../auth/session";

const defaultConfig: PublicConfig = { heroTitle: "小皮租房", heroSubtitle: "在武汉，住得更省心", heroDescription: "真实房源 · 可靠服务", notice: "", contactPhone: "", relatedPropertyRadiusKm: 3, locations: [{ label: "金融港", keyword: "金融港" }, { label: "光谷天地", keyword: "光谷天地" }], featuredChannels: [{ icon: "▣", title: "地铁沿线", subtitle: "通勤省心", keyword: "地铁", radiusKm: 5 }, { icon: "◇", title: "高校周边", subtitle: "生活方便", keyword: "高校", radiusKm: 5 }, { icon: "□", title: "职场优选", subtitle: "离公司更近", keyword: "金融港", radiusKm: 5 }] };
const priceRanges = [
  { label: "¥1,500 以下", min: 0, max: 1500 },
  { label: "¥1,500–2,500", min: 1500, max: 2500 },
  { label: "¥2,500–4,000", min: 2500, max: 4000 },
  { label: "¥4,000+", min: 4000, max: 0 },
];
const properties = ref<Property[]>([]);
const config = ref<PublicConfig>(defaultConfig);
const activeKind = ref("推荐");
const selectedArea = ref("全部");
const selectedPrice = ref<number | null>(null);
const isAdmin = ref(false);
const kindOptions = ["推荐", "整租", "合租", "一居", "两居", "三居+"];
const channelStyle = (icon: string, index: number) => {
  const legacy: Record<string, string> = { "🏢": "work", "🌳": "campus", "🌆": "life" };
  return legacy[icon] || ["commute", "campus", "work", "life"].includes(icon) ? (legacy[icon] || icon) : ["commute", "campus", "work", "life"][index % 4];
};
const homeProperties = computed(() => properties.value.filter((item) => {
  const subject = `${item.title || ""} ${item.layout || ""} ${item.location || ""}`;
  const range = selectedPrice.value === null ? null : priceRanges[selectedPrice.value];
  const kindMatched = activeKind.value === "推荐" || subject.includes(activeKind.value);
  const areaMatched = selectedArea.value === "全部" || subject.includes(selectedArea.value);
  const priceMatched = !range || (!range.min || item.rentMonthly >= range.min) && (!range.max || item.rentMonthly <= range.max);
  return kindMatched && areaMatched && priceMatched;
}));
const homePresentation = computed(() => config.value.pagePresentations?.home || { title: "今天想住哪里？", subtitle: "搜索小区、地铁或商圈", backgroundImage: "" });
const presentationImage = (path?: string) => path ? (path.startsWith("http") ? path : `${API_ORIGIN.replace(/\/api$/, "")}${path}`) : heroFallback;
const coverUrl = (item?: Property) => {
  const cover = item?.media?.find((media) => media.type === "image")?.url;
  return cover ? (cover.startsWith("http") ? cover : `${API_ORIGIN}${cover}`) : "";
};
onShow(async () => {
  isAdmin.value = getSession()?.role === "admin";
  const [propertyResult, configResult] = await Promise.allSettled([listProperties(), getPublicConfig()]);
  if (propertyResult.status === "fulfilled") properties.value = propertyResult.value;
  if (configResult.status === "fulfilled") {
    config.value = { ...defaultConfig, ...configResult.value };
    uni.setNavigationBarTitle({ title: config.value.heroTitle || "小皮租房" });
  }
});
const open = (id: string) => uni.navigateTo({ url: `/pages/properties/detail?id=${id}` });
const browse = () => uni.switchTab({ url: "/pages/properties/index" });
const browseFiltered = () => {
  uni.setStorageSync("xiaopi_property_filter", JSON.stringify({
    area: selectedArea.value,
    priceIndex: selectedPrice.value === null ? 0 : selectedPrice.value + 1,
  }));
  browse();
};
const requestHome = () => uni.navigateTo({ url: "/pages/requests/index" });
const openChannel = (channel: PublicConfig["featuredChannels"][number]) => {
  uni.setStorageSync("xiaopi_property_filter", JSON.stringify({ keyword: channel.keyword, latitude: channel.latitude || 0, longitude: channel.longitude || 0, radiusKm: channel.radiusKm || 5, title: channel.title }));
  browse();
};
</script>

<template>
  <view class="home-page">
    <view class="hero">
      <image class="hero-bg" :src="presentationImage(homePresentation.backgroundImage)" mode="aspectFill" />
      <view class="city"><image :src="pinIcon" mode="aspectFit" />武汉<text>⌄</text></view>
      <view class="hero-copy">
        <view class="hero-title">{{ homePresentation.title || "今天想住哪里？" }}</view>
        <view class="hero-accent"></view>
      </view>
      <view class="hero-search" @click="browse"><image :src="searchIcon" mode="aspectFit" /><span>{{ homePresentation.subtitle || "搜索小区、地铁或商圈" }}</span><b>搜索</b></view>
    </view>
    <scroll-view scroll-x class="area-scroll" :show-scrollbar="false"><view class="area-row"><text :class="{ selected: selectedArea === '全部' }" @click="selectedArea = '全部'"><image :src="pinIcon" mode="aspectFit" />附近</text><text v-for="location in config.locations" :key="location.label" :class="{ selected: selectedArea === location.label }" @click="selectedArea = location.label">{{ location.label }}</text></view></scroll-view>
    <scroll-view scroll-x class="price-scroll" :show-scrollbar="false"><view class="price-row"><b>租金预算</b><text v-for="(price, index) in priceRanges" :key="price.label" :class="{ selected: selectedPrice === index }" @click="selectedPrice = selectedPrice === index ? null : index">{{ price.label }}</text></view></scroll-view>

    <view class="section-head"><strong>精选频道</strong><text @click="browse">全部频道 <b>›</b></text></view>
    <scroll-view scroll-x class="channel-scroll" :show-scrollbar="false"><view class="channels"><view v-for="(channel, index) in config.featuredChannels" :key="channel.title" class="channel" :class="`tone-${channelStyle(channel.icon, index)}`" @click="openChannel(channel)"><view class="channel-mark" :class="`mark-${channelStyle(channel.icon, index)}`"><view class="channel-glyph"></view></view><view class="channel-title">{{ channel.title }}</view><view class="channel-subtitle">{{ channel.subtitle || "发现附近好房" }}</view></view></view></scroll-view>

    <view class="recommend-head"><scroll-view scroll-x :show-scrollbar="false" class="kind-scroll"><view class="kind-row"><text v-for="kind in kindOptions" :key="kind" :class="{ active: activeKind === kind }" @click="activeKind = kind">{{ kind }}</text></view></scroll-view><b @click="browseFiltered">查看房源</b></view>
    <view v-if="homeProperties.length" class="property-list"><view v-for="item in homeProperties.slice(0, 4)" :key="item.id" class="listing" @click="open(item.id)"><view class="listing-image"><image v-if="coverUrl(item)" :src="coverUrl(item)" mode="aspectFill"/><view v-else class="listing-placeholder"><span>{{ item.community?.slice(0, 2) || "小皮" }}</span></view><view v-if="!isAdmin" class="heart-button"><image :src="heartIcon" mode="aspectFit" /></view></view><view class="listing-info"><strong>{{ item.title || item.community }}</strong><small>{{ item.layout || "户型待补充" }} · {{ item.location }}</small><view class="tags"><text>真实核验</text><text>{{ item.feeText || "拎包入住" }}</text></view><b>¥ {{ item.rentMonthly }}<i>/月</i></b></view></view></view>
    <view v-else class="empty">新房源正在整理中</view>
    <view v-if="!isAdmin" class="request-fab" @click="requestHome"><image :src="editIcon" mode="aspectFit" />说说找房需求</view>
  </view>
</template>

<style scoped>
.home-page{min-height:100vh;background:#f7eee2;color:#354126;padding-bottom:120rpx}.hero{position:relative;height:546rpx;padding:20rpx 30rpx 0;box-sizing:border-box;overflow:hidden;background:#f9e9d7}.hero-bg{position:absolute;inset:0;width:100%;height:100%;z-index:0}.hero:after{position:absolute;inset:0;z-index:1;background:linear-gradient(90deg,rgba(249,233,215,.98) 0%,rgba(249,233,215,.64) 53%,rgba(249,233,215,.08) 100%);content:""}.hero>view{position:relative;z-index:2}.status{display:flex;justify-content:space-between;color:#252d25;font-size:22rpx;font-weight:700}.city{display:inline-block;margin-top:28rpx;padding:13rpx 20rpx;border-radius:999rpx;background:rgba(255,255,255,.65);color:#445031;font-size:26rpx;font-weight:700}.hero-copy{margin-top:40rpx}.hero-copy strong{display:block;font-size:52rpx;letter-spacing:-3rpx}.hero-copy i{display:block;width:150rpx;height:9rpx;margin:15rpx 0 32rpx 110rpx;border-radius:999rpx;background:#d66d50;transform:rotate(-3deg)}.hero-search{display:flex;align-items:center;gap:14rpx;padding:14rpx 15rpx 14rpx 26rpx;border-radius:24rpx;background:rgba(255,255,255,.9);box-shadow:0 12rpx 26rpx rgba(91,65,40,.08)}.hero-search>text{font-size:42rpx}.hero-search span{flex:1;color:#808078;font-size:24rpx;white-space:nowrap}.hero-search b{padding:17rpx 24rpx;border-radius:18rpx;background:#d66d50;color:#fff;font-size:26rpx}.area-scroll,.price-scroll,.channel-scroll{width:100%;white-space:nowrap}.area-row{display:inline-flex;gap:14rpx;padding:22rpx 28rpx 18rpx}.area-row text{padding:14rpx 24rpx;border-radius:999rpx;background:#fbf5ee;color:#414538;font-size:25rpx}.area-row .selected{background:#5e6d2e;color:#fff}.price-row{display:inline-flex;align-items:center;margin:0 28rpx;padding:17rpx 18rpx;border-radius:20rpx;background:#fffaf4;box-shadow:0 6rpx 18rpx rgba(94,72,48,.05)}.price-row b,.price-row text{padding:0 16rpx;border-right:1rpx solid #e3ddd4;font-size:23rpx}.price-row b{color:#4d5639}.price-row text:last-child{border:0}.section-head{display:flex;justify-content:space-between;align-items:center;margin:52rpx 30rpx 22rpx}.section-head strong{font-size:39rpx;letter-spacing:-2rpx}.section-head text{font-size:23rpx;color:#737b5c}.channels{display:inline-flex;gap:16rpx;padding:0 30rpx}.channel{position:relative;width:190rpx;height:170rpx;padding:20rpx;box-sizing:border-box;border-radius:22rpx;color:#fff;overflow:hidden}.channel>text{display:block;font-size:34rpx}.channel strong,.channel small,.channel b{display:block}.channel strong{margin-top:20rpx;font-size:27rpx}.channel small{margin-top:5rpx;font-size:19rpx;color:rgba(255,255,255,.82)}.channel b{position:absolute;bottom:16rpx;left:20rpx;width:31rpx;height:31rpx;border-radius:50%;background:rgba(255,255,255,.9);color:#63713a;text-align:center;line-height:29rpx}.tone-0{background:linear-gradient(135deg,#dd8d6f,#bd624c)}.tone-1{background:linear-gradient(135deg,#778b46,#415a2f)}.tone-2{background:linear-gradient(135deg,#d4ad78,#a9855a)}.tone-3{background:linear-gradient(135deg,#a2a77a,#68704f)}.recommend-head{display:flex;justify-content:space-between;align-items:center;margin:54rpx 30rpx 22rpx}.recommend-head>view{display:flex;align-items:center;gap:26rpx}.recommend-head strong{font-size:39rpx;letter-spacing:-2rpx}.recommend-head strong:after{display:block;width:62rpx;height:7rpx;margin-top:7rpx;border-radius:999rpx;background:#d66d50;content:""}.recommend-head text{font-size:25rpx;color:#707467}.recommend-head>b{padding:13rpx 17rpx;border-radius:18rpx;background:#fff8ee;color:#454b3e;font-size:23rpx}.property-list{padding:0 24rpx}.listing{display:flex;gap:22rpx;margin-bottom:20rpx;padding:14rpx;border-radius:26rpx;background:#fffaf4;box-shadow:0 10rpx 24rpx rgba(100,75,46,.05)}.listing-image{position:relative;flex:none;width:330rpx;height:224rpx;border-radius:18rpx;overflow:hidden;background:#dfd4be}.listing-image image{width:100%;height:100%}.listing-image i{position:absolute;top:13rpx;right:13rpx;width:48rpx;height:48rpx;border-radius:50%;background:rgba(65,55,40,.36);color:#fff;font-size:34rpx;font-style:normal;text-align:center;line-height:46rpx}.listing-placeholder{display:flex;width:100%;height:100%;align-items:center;justify-content:center;background:linear-gradient(145deg,#e4cdaa,#b9a58c);color:#fff}.listing-placeholder span{padding:12rpx 17rpx;border:1rpx solid rgba(255,255,255,.5);font-size:28rpx}.listing-info{min-width:0;flex:1;padding:5rpx 5rpx 0 0}.listing-info strong,.listing-info small,.listing-info>b{display:block}.listing-info strong{overflow:hidden;color:#34392e;font-size:30rpx;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.listing-info small{margin-top:12rpx;color:#70766c;font-size:22rpx;line-height:1.45}.tags{display:flex;gap:9rpx;margin-top:12rpx;overflow:hidden}.tags text{flex:none;padding:7rpx 10rpx;border-radius:8rpx;background:#edf0dc;color:#63703b;font-size:18rpx}.tags text:last-child{background:#f9e5dc;color:#b7644b}.listing-info>b{margin-top:15rpx;color:#d66d50;font-size:35rpx}.listing-info>b i{font-size:20rpx;font-style:normal}.empty{margin:0 30rpx;padding:80rpx 0;border-radius:24rpx;background:#fffaf4;color:#8c8b80;text-align:center}.request-fab{position:fixed;z-index:4;right:30rpx;bottom:38rpx;padding:20rpx 27rpx;border-radius:999rpx;background:#5e6d2e;color:#fff;box-shadow:0 10rpx 28rpx rgba(58,65,26,.28);font-size:28rpx;font-weight:700}

/* Detail pass: a single spacing scale, native-style icons, and deliberate type hierarchy. */
.hero{height:570rpx;padding:42rpx 32rpx 0}.hero:after{background:linear-gradient(90deg,rgba(249,233,215,.98) 0%,rgba(249,233,215,.78) 57%,rgba(249,233,215,.16) 100%)}.city{display:flex;align-items:center;gap:8rpx;margin-top:0;padding:12rpx 18rpx;background:rgba(255,255,255,.76);font-size:25rpx;line-height:1}.city image{width:28rpx;height:28rpx}.city text{margin-left:2rpx;font-size:19rpx}.hero-copy{margin-top:52rpx}.hero-copy strong{display:-webkit-box;overflow:hidden;max-width:570rpx;color:#334025;font-size:56rpx;font-weight:900;line-height:1.22;letter-spacing:-3rpx;text-shadow:0 2rpx 0 rgba(255,255,255,.18);-webkit-line-clamp:2;-webkit-box-orient:vertical}.hero-copy i{width:116rpx;height:8rpx;margin:17rpx 0 37rpx 178rpx}.hero-search{gap:16rpx;padding:15rpx 14rpx 15rpx 22rpx;border:1rpx solid rgba(255,255,255,.76);border-radius:28rpx;box-shadow:0 12rpx 26rpx rgba(93,66,40,.08)}.hero-search image{width:37rpx;height:37rpx;flex:none}.hero-search span{overflow:hidden;font-size:25rpx;letter-spacing:-.4rpx;text-overflow:ellipsis;white-space:nowrap}.hero-search b{padding:16rpx 25rpx;border-radius:19rpx;font-size:25rpx;font-weight:700}.area-row{gap:12rpx;padding:22rpx 30rpx 18rpx}.area-row text{padding:13rpx 22rpx;border:1rpx solid #eee5da;font-size:24rpx;line-height:1}.area-row .selected{display:flex;align-items:center;gap:6rpx;border-color:#5e6d2e}.area-row .selected image{width:25rpx;height:25rpx;filter:brightness(0) invert(1)}.price-row{margin:0 30rpx;padding:17rpx 8rpx;border:1rpx solid #eee6dc;border-radius:18rpx;box-shadow:none}.price-row b,.price-row text{padding:0 14rpx;font-size:22rpx;line-height:31rpx}.price-row b{display:block;color:#43513c;font-size:24rpx;white-space:nowrap}.section-head{margin:52rpx 32rpx 23rpx}.section-head strong{font-size:38rpx}.section-head text{font-size:23rpx}.section-head text b{font-size:30rpx;font-weight:400}.channels{gap:14rpx;padding:0 32rpx}.channel{width:214rpx;height:192rpx;padding:18rpx 18rpx 16rpx;border-radius:20rpx}.channel-mark{display:flex;align-items:center;justify-content:center;width:48rpx;height:48rpx;border-radius:14rpx;background:rgba(255,255,255,.18);font-size:25rpx}.channel strong{margin-top:18rpx;font-size:28rpx;letter-spacing:-1rpx}.channel small{overflow:hidden;margin-top:5rpx;font-size:20rpx;text-overflow:ellipsis}.channel b{position:static;width:auto;height:auto;margin-top:10rpx;background:none;color:rgba(255,255,255,.9);font-size:19rpx;line-height:1}.recommend-head{gap:14rpx;margin:56rpx 32rpx 22rpx}.kind-scroll{min-width:0;flex:1;white-space:nowrap}.kind-row{display:inline-flex;gap:26rpx;align-items:center}.recommend-head text{position:relative;padding:0 1rpx;font-size:27rpx}.recommend-head text.active{color:#354126;font-size:38rpx;font-weight:800;letter-spacing:-2rpx}.recommend-head text.active:after{position:absolute;bottom:-12rpx;left:1rpx;width:58rpx;height:7rpx;border-radius:99rpx;background:#d66d50;content:""}.recommend-head>b{display:block;flex:none;padding:13rpx 16rpx;border:1rpx solid #eee4d8;border-radius:18rpx;font-size:22rpx;line-height:1}.property-list{padding:0 24rpx}.listing{gap:18rpx;margin-bottom:18rpx;padding:12rpx;border:1rpx solid rgba(239,228,215,.82);border-radius:22rpx;box-shadow:none}.listing-image{width:300rpx;height:210rpx;border-radius:16rpx}.heart-button{position:absolute;top:12rpx;right:12rpx;display:flex;align-items:center;justify-content:center;width:48rpx;height:48rpx;border-radius:50%;background:rgba(48,45,35,.32)}.heart-button image{width:30rpx;height:30rpx}.listing-info{padding:5rpx 4rpx 0 0}.listing-info strong{font-size:29rpx;line-height:1.35}.listing-info small{display:-webkit-box;overflow:hidden;margin-top:8rpx;font-size:20rpx;line-height:1.5;-webkit-line-clamp:2;-webkit-box-orient:vertical;white-space:normal}.tags{gap:7rpx;margin-top:9rpx}.tags text{padding:6rpx 9rpx;font-size:18rpx}.listing-info>b{margin-top:11rpx;font-size:34rpx;letter-spacing:-1rpx}.listing-info>b i{margin-left:3rpx;font-size:19rpx;color:#8b8a7f}.request-fab{right:28rpx;bottom:132rpx;display:flex;align-items:center;gap:8rpx;padding:19rpx 23rpx;border:1rpx solid rgba(255,255,255,.18);font-size:26rpx}.request-fab image{width:32rpx;height:32rpx}

/* Acceptance pass: every choice must visibly select and carry into the results page. */
.price-row text.selected{border-radius:12rpx;background:#5e6d2e;color:#fff;font-weight:700}.channel{width:228rpx;height:166rpx;padding:18rpx}.channel strong{overflow:hidden;margin-top:15rpx;font-size:28rpx;line-height:1.2;text-overflow:ellipsis;white-space:nowrap}.channel small{display:block;overflow:hidden;margin-top:7rpx;font-size:21rpx;line-height:1.25;text-overflow:ellipsis;white-space:nowrap}.channel-mark{position:relative;width:44rpx;height:44rpx}.channel-mark i{display:block;width:22rpx;height:22rpx;border:3rpx solid rgba(255,255,255,.92);border-radius:50%;box-sizing:border-box}.channel-mark i:after{display:block;width:8rpx;height:8rpx;margin:4rpx;border-radius:50%;background:rgba(255,255,255,.92);content:""}.channel-mark.mark-campus i{border-radius:4rpx}.channel-mark.mark-campus i:after{width:12rpx;height:3rpx;margin:7rpx 2rpx;border-radius:3rpx}.channel-mark.mark-work i{border:0;border-radius:0;background:linear-gradient(90deg,rgba(255,255,255,.92) 0 28%,transparent 28% 37%,rgba(255,255,255,.92) 37% 63%,transparent 63% 72%,rgba(255,255,255,.92) 72%)}.channel-mark.mark-work i:after{display:none}.channel-mark.mark-life i{border-radius:50% 50% 6rpx 6rpx;transform:rotate(45deg)}.channel-mark.mark-life i:after{margin:4rpx}.tone-commute{background:linear-gradient(135deg,#dd8d6f,#bd624c)}.tone-campus{background:linear-gradient(135deg,#778b46,#415a2f)}.tone-work{background:linear-gradient(135deg,#d4ad78,#a9855a)}.tone-life{background:linear-gradient(135deg,#8a9b8b,#536e61)}.recommend-head>b{background:#40543f;color:#fff;border-color:#40543f;font-weight:700}
.hero-title{display:-webkit-box;overflow:hidden;max-width:570rpx;color:#334025;font-size:56rpx;font-weight:900;line-height:1.22;letter-spacing:-3rpx;text-shadow:0 2rpx 0 rgba(255,255,255,.18);-webkit-line-clamp:2;-webkit-box-orient:vertical}.hero-accent{width:116rpx;height:8rpx;margin:18rpx 0 0 178rpx;border-radius:999rpx;background:#d66d50;transform:rotate(-3deg)}.hero-search{margin-top:38rpx}
.hero-search{min-height:92rpx;box-sizing:border-box}.hero-search span{font-size:26rpx;line-height:1.35}.hero-search b{line-height:1.2}.section-head{margin-top:58rpx;margin-bottom:26rpx}.section-head strong{font-size:40rpx;line-height:1.2}.recommend-head{margin-top:62rpx;margin-bottom:26rpx}.listing-info strong{font-size:30rpx}.listing-info>b{font-size:36rpx}.request-fab{bottom:calc(116rpx + env(safe-area-inset-bottom))}
.channel-glyph{display:block;width:22rpx;height:22rpx;border:3rpx solid rgba(255,255,255,.92);border-radius:50%;box-sizing:border-box}.channel-glyph:after{display:block;width:8rpx;height:8rpx;margin:4rpx;border-radius:50%;background:rgba(255,255,255,.92);content:""}.mark-campus .channel-glyph{border-radius:4rpx}.mark-campus .channel-glyph:after{width:12rpx;height:3rpx;margin:7rpx 2rpx;border-radius:3rpx}.mark-work .channel-glyph{border:0;border-radius:0;background:linear-gradient(90deg,rgba(255,255,255,.92) 0 28%,transparent 28% 37%,rgba(255,255,255,.92) 37% 63%,transparent 63% 72%,rgba(255,255,255,.92) 72%)}.mark-work .channel-glyph:after{display:none}.mark-life .channel-glyph{border-radius:50% 50% 6rpx 6rpx;transform:rotate(45deg)}.mark-life .channel-glyph:after{margin:4rpx}.channel-title{display:block;overflow:hidden;margin-top:15rpx;color:#fff;font-size:28rpx;font-weight:700;line-height:1.2;letter-spacing:-1rpx;text-overflow:ellipsis;white-space:nowrap}.channel-subtitle{display:block;overflow:hidden;margin-top:7rpx;color:rgba(255,255,255,.82);font-size:21rpx;line-height:1.25;text-overflow:ellipsis;white-space:nowrap}
</style>
