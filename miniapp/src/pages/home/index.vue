<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { resolveAssetUrl } from "../../api/client";
import { getPublicConfig, listProperties, type Property, type PublicConfig } from "../../api/properties";
import pinIcon from "../../static/icons/pin.svg";
import searchIcon from "../../static/icons/search.svg";
import heartIcon from "../../static/icons/heart.svg";
import editIcon from "../../static/icons/edit.svg";
// Keep the first-screen asset small: the source PNG was 1.7 MB and delayed
// first render on mobile networks. This 1440px JPEG is visually equivalent.
import heroFallback from "../../static/home-lifestyle-hero.jpg";
import { getSession } from "../../auth/session";
import { listingTagValue, publicListingTitle, uniqueListingTags } from "../../utils/listing-title";

const defaultConfig: PublicConfig = { heroTitle: "小皮租房", heroSubtitle: "在武汉，住得更省心", heroDescription: "真实房源 · 可靠服务", notice: "", contactPhone: "", relatedPropertyRadiusKm: 3, locations: [{ label: "金融港", keyword: "金融港" }, { label: "光谷天地", keyword: "光谷天地" }], featuredChannels: [{ icon: "▣", title: "地铁沿线", subtitle: "通勤省心", keyword: "地铁", radiusKm: 3 }, { icon: "◇", title: "高校周边", subtitle: "生活方便", keyword: "高校", radiusKm: 3 }, { icon: "□", title: "职场优选", subtitle: "离公司更近", keyword: "金融港", radiusKm: 3 }] };
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
const quickKinds = ["整租", "合租", "一居", "两居", "三居+"];
const roomCountOf = (item: Property) => {
  const match = `${item.layout || ""} ${item.title || ""}`.match(/(\d+)\s*室/);
  if (match) return Number(match[1]);
  const chinese = `${item.layout || ""} ${item.title || ""}`.match(/([一二两三四五六七八九])\s*居/);
  return chinese ? ({ 一: 1, 二: 2, 两: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9 }[chinese[1]] || 0) : 0;
};
const matchesKind = (item: Property, kind: string) => {
  if (kind === "推荐") return true;
  const text = `${item.title || ""} ${item.layout || ""} ${item.rentType || ""}`;
  // 兼容历史房源：旧数据没有 rentType 时，按整租展示，不让新筛选把存量房源全部筛掉。
  if (kind === "整租" && !item.rentType) return true;
  if (kind === "一居") return roomCountOf(item) === 1;
  if (kind === "两居") return roomCountOf(item) === 2;
  if (kind === "三居+") return roomCountOf(item) >= 3;
  return text.includes(kind);
};
const channelStyle = (icon: string, index: number) => {
  const legacy: Record<string, string> = { "🏢": "work", "🌳": "campus", "🌆": "life" };
  return legacy[icon] || ["commute", "campus", "work", "life"].includes(icon) ? (legacy[icon] || icon) : ["commute", "campus", "work", "life"][index % 4];
};
const distanceKm = (lat1: number, lon1: number, lat2: number, lon2: number) => {
  const rad = Math.PI / 180;
  const dLat = (lat2 - lat1) * rad;
  const dLon = (lon2 - lon1) * rad;
  const a = Math.sin(dLat / 2) ** 2 + Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(dLon / 2) ** 2;
  return 6371 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
};
const matchesArea = (item: Property, label: string) => {
  if (label === "全部") return true;
  if (item.areaLabel) return item.areaLabel === label;
  // Compatibility for records created before areaLabel existed: assign them to
  // exactly one nearest configured area, never to every overlapping radius.
  const candidates = config.value.featuredChannels.filter((area) => area.latitude && area.longitude);
  if (item.latitude && item.longitude && candidates.length) {
    const nearest = candidates.reduce((current, candidate) =>
      distanceKm(Number(candidate.latitude), Number(candidate.longitude), item.latitude!, item.longitude!) <
      distanceKm(Number(current.latitude), Number(current.longitude), item.latitude!, item.longitude!) ? candidate : current,
    );
    return nearest.title === label;
  }
  return false;
};
const homeProperties = computed(() => properties.value.filter((item) => {
  // 首页是找房入口，不是运营工作台。即使管理员登录时接口返回全量
  // 房源，也绝不能把草稿、已租或下架房源伪装成“可租”展示给浏览流。
  if (item.status !== "available") return false;
  const subject = `${item.title || ""} ${item.layout || ""} ${item.location || ""}`;
  const range = selectedPrice.value === null ? null : priceRanges[selectedPrice.value];
  const kindMatched = matchesKind(item, activeKind.value);
  const areaMatched = matchesArea(item, selectedArea.value);
  const priceMatched = !range || (!range.min || item.rentMonthly >= range.min) && (!range.max || item.rentMonthly <= range.max);
  return kindMatched && areaMatched && priceMatched;
}));
const homePresentation = computed(() => config.value.pagePresentations?.home || { title: "今天想住哪里？", subtitle: "搜索小区、地铁或商圈", backgroundImage: "" });
const presentationImage = (path?: string) => path ? resolveAssetUrl(path) : heroFallback;
const coverUrl = (item?: Property) => {
  const cover = item?.media?.find((media) => media.type === "image")?.url;
  return cover ? resolveAssetUrl(cover) : "";
};
const primaryLabels = (item: Property) => {
  const fields = isAdmin.value
    ? (config.value.listingTitle?.adminEnabledFields || [])
    : (item.displayTags || config.value.listingTitle?.enabledFields || []);
  return uniqueListingTags(["rentType", "listingSource"].filter((field) => fields.includes(field)).map((field) => listingTagValue(item, field)));
};
const secondaryLabels = (item: Property) => {
  const fields = isAdmin.value
    ? (config.value.listingTitle?.adminEnabledFields || [])
    : (item.displayTags || config.value.listingTitle?.enabledFields || []);
  return uniqueListingTags(["unit", "layout", "floor", "area", "fee"].filter((field) => fields.includes(field)).map((field) => listingTagValue(item, field)));
};
const listingTitle = (item: Property) => {
  return publicListingTitle(item, config.value.listingTitle?.enabledFields);
};
const cardLocation = (value: unknown) => {
  const raw = String(value || "").trim();
  if (!raw) return "位置待补充";
  const district = raw.split(/[·・]/).map((part) => part.trim()).find((part) => /区$/.test(part));
  const landmark = raw.match(/[^省市区\s·・]*(?:大道|一路|二路|三路|路|街道|街|商圈)/)?.[0];
  return [district, landmark].filter(Boolean).join(" · ") || raw.split(/[·・]/)[0].trim();
};
const listingSubtitle = (item: Property) => {
  return cardLocation(item.location);
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
    kind: activeKind.value === "推荐" ? "" : activeKind.value,
  }));
  browse();
};
const requestHome = () => uni.navigateTo({ url: "/pages/requests/index" });
const openChannel = (channel: PublicConfig["featuredChannels"][number]) => {
  uni.setStorageSync("xiaopi_property_filter", JSON.stringify({ area: channel.title }));
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
    <scroll-view scroll-x class="quick-kind-scroll" :show-scrollbar="false"><view class="quick-kind-row"><text class="quick-kind-label">住法</text><text v-for="kind in quickKinds" :key="kind" :class="{ active: activeKind === kind }" @click="activeKind = activeKind === kind ? '推荐' : kind">{{ kind }}</text></view></scroll-view>

    <view class="section-head"><strong>精选频道</strong><text @click="browse">全部频道 <b>›</b></text></view>
    <scroll-view scroll-x class="channel-scroll" :show-scrollbar="false"><view class="channels"><view v-for="(channel, index) in config.featuredChannels" :key="channel.title" class="channel" :class="`tone-${channelStyle(channel.icon, index)}`" @click="openChannel(channel)"><view class="channel-mark" :class="`mark-${channelStyle(channel.icon, index)}`"><view class="channel-glyph"></view></view><view class="channel-title">{{ channel.title }}</view><view class="channel-subtitle">{{ channel.subtitle || "发现附近好房" }}</view></view></view></scroll-view>
    <view v-if="!isAdmin" class="request-fab" @click="requestHome"><image :src="editIcon" mode="aspectFit" />说说找房需求</view>

    <view class="recommend-head"><strong>{{ activeKind === "推荐" ? "推荐房源" : `${activeKind}房源` }}</strong><b @click="browseFiltered">查看房源</b></view>
    <view v-if="homeProperties.length" class="property-list"><view v-for="item in homeProperties.slice(0, 4)" :key="item.id" class="listing" @click="open(item.id)"><view class="listing-image"><image v-if="coverUrl(item)" :src="coverUrl(item)" mode="aspectFill"/><view v-else class="listing-placeholder"><span>{{ item.community?.slice(0, 2) || "小皮" }}</span></view><view class="listing-status">可租</view><view v-if="!isAdmin" class="heart-button"><image :src="heartIcon" mode="aspectFit" /></view></view><view class="listing-info"><view class="home-listing-title">{{ listingTitle(item) }}</view><view class="home-listing-address">{{ listingSubtitle(item) }}</view><view class="tag-stack"><view class="tags primary-tags"><text v-for="label in primaryLabels(item)" :key="label">{{ label }}</text></view><view v-if="secondaryLabels(item).length" class="tags fee-tags"><text v-for="label in secondaryLabels(item)" :key="label">{{ label }}</text></view></view><view class="listing-price">¥ {{ item.rentMonthly }}<text class="price-unit">/月</text></view></view></view></view>
    <view v-else class="empty">新房源正在整理中</view>
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
.listing-status{position:absolute;top:12rpx;left:12rpx;padding:7rpx 11rpx;border-radius:10rpx;background:#167b70;color:#fff;font-size:20rpx;font-weight:700;line-height:1.1}.listing-info .tags text{background:#edf0dc;color:#536a45}.listing-info .tags text:nth-child(2){background:#f8eadf;color:#b7644b}.listing-price{display:inline-flex;align-items:baseline;gap:3rpx;margin-top:11rpx;color:#d66d50;font-size:36rpx;font-weight:800;line-height:1.1;white-space:nowrap}.listing-price .price-unit{color:#8b8a7f;font-size:19rpx;font-weight:400}

/* Keep the primary CTA in the document flow: it should never cover a channel or heading. */
.home-page{padding-bottom:42rpx}.request-fab{position:static;display:flex;align-items:center;justify-content:center;gap:8rpx;width:max-content;margin:28rpx 32rpx 0;padding:18rpx 22rpx;border:1rpx solid rgba(255,255,255,.18);border-radius:999rpx;background:#5e6d2e;color:#fff;box-shadow:0 10rpx 22rpx rgba(58,65,26,.18);font-size:25rpx;font-weight:700;line-height:1.2}.request-fab image{width:30rpx;height:30rpx}.recommend-head{margin-top:46rpx}.channel{flex:none}.channel-title,.channel-subtitle{max-width:100%}
.quick-kind-scroll{width:100%;white-space:nowrap}.quick-kind-row{display:inline-flex;align-items:center;gap:10rpx;padding:18rpx 30rpx 2rpx}.quick-kind-row text{padding:11rpx 18rpx;border:1rpx solid #e8e4db;border-radius:14rpx;background:#fffaf4;color:#697363;font-size:22rpx;line-height:1}.quick-kind-row .quick-kind-label{border:0;background:transparent;color:#4b583f;font-size:23rpx;font-weight:800}.quick-kind-row text.active{border-color:#5e6d2e;background:#e6ecd7;color:#40543f;font-weight:800}

/* Final visual baseline: keep the first screen on a single 32rpx grid and
   neutralise the legacy selector rules above while old data remains compatible. */
.home-page{padding-bottom:48rpx}
.hero{height:564rpx;padding:40rpx 32rpx 0}
.city{margin-top:0}
.hero-copy{margin-top:48rpx}
.hero-title{font-size:52rpx;line-height:1.24;letter-spacing:-2rpx}
.hero-accent{margin:16rpx 0 0 150rpx}
.hero-search{min-height:94rpx;margin-top:42rpx;padding:14rpx 14rpx 14rpx 22rpx}
.hero-search span{font-size:26rpx}
.hero-search b{padding:16rpx 23rpx;font-size:24rpx}
.area-row{padding:24rpx 32rpx 16rpx}
.price-row{margin:0 32rpx;padding:16rpx 8rpx}
.price-row b,.price-row text{padding:0 13rpx;font-size:21rpx}
.quick-kind-row{padding:18rpx 32rpx 4rpx}
.section-head{margin:54rpx 32rpx 24rpx}
.section-head strong{font-size:38rpx;letter-spacing:-1rpx}
.channels{gap:14rpx;padding:0 32rpx}
.channel{width:220rpx;height:170rpx;padding:18rpx}
.channel-title{margin-top:14rpx;font-size:27rpx}
.channel-subtitle{margin-top:7rpx;font-size:20rpx}
.request-fab{margin:28rpx 32rpx 0}
.recommend-head{margin:50rpx 32rpx 24rpx}
.recommend-head strong{font-size:38rpx}
.property-list{padding:0 32rpx}
.listing{gap:18rpx;margin-bottom:18rpx;padding:12rpx;border:1rpx solid #eee6dc;border-radius:20rpx;background:#fffdf9}
.listing-image{width:244rpx;height:200rpx;border-radius:14rpx}
.listing-info strong{display:-webkit-box;overflow:hidden;font-size:28rpx;line-height:1.35;text-overflow:ellipsis;white-space:normal;-webkit-box-orient:vertical;-webkit-line-clamp:2}
.listing{height:224rpx;box-sizing:border-box}
.listing-image{width:244rpx;height:200rpx}
.listing-info{display:flex;height:200rpx;flex-direction:column;padding:6rpx 4rpx 3rpx 0;box-sizing:border-box}
.home-listing-title{display:block;overflow:hidden;max-width:100%;color:#2d3d30;font-size:29rpx;font-weight:800;line-height:1.28;letter-spacing:-.4rpx;text-overflow:ellipsis;white-space:nowrap}
.home-admin-doorplate{display:block;overflow:hidden;margin-top:7rpx;color:#526553;font-size:20rpx;font-weight:700;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}
.home-listing-address{overflow:hidden;max-height:31rpx;margin-top:8rpx;color:#778176;font-size:22rpx;font-weight:400;line-height:1.4;text-overflow:ellipsis;white-space:nowrap}
.listing-info .tags{height:32rpx;gap:10rpx;margin-top:10rpx;white-space:nowrap}
.listing-info .tags text{padding:5rpx 8rpx;font-size:18rpx}
.listing-price{min-height:34rpx;margin-top:auto;font-size:34rpx}
/* Listing card acceptance: fixed rhythm, stronger title, quieter address. */
.listing{height:232rpx;gap:18rpx;padding:12rpx;border-color:#e9e1d7;border-radius:22rpx;background:#fffdf9;box-shadow:0 8rpx 22rpx rgba(91,69,44,.055)}
.listing-image{width:248rpx;height:208rpx;border-radius:16rpx}
.listing-info{height:208rpx;padding:5rpx 5rpx 4rpx 0}
.home-listing-title{font-size:30rpx;line-height:1.33;letter-spacing:-.3rpx}
.home-listing-address{max-height:34rpx;margin-top:10rpx;color:#707b70;font-size:24rpx;line-height:1.4}
.listing-info .tags{height:34rpx;gap:8rpx;margin-top:10rpx}
.listing-info .tags text{padding:6rpx 9rpx;border-radius:8rpx;font-size:18rpx}
.listing-price{min-height:36rpx;padding-top:8rpx;font-size:35rpx;line-height:1}.listing-price .price-unit{font-size:19rpx}
.listing:active{opacity:.9;transform:scale(.995)}
.tag-stack{display:flex;flex-direction:column;align-items:flex-start;gap:5rpx;margin-top:8rpx}.listing-info .tag-stack .tags{height:29rpx;margin:0}.listing-info .tag-stack .tags text{padding:4rpx 8rpx;font-size:19rpx;line-height:1.1}.listing-info .fee-tags text{background:#f8eadf;color:#b7644b}
</style>
