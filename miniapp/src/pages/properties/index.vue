<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { API_ORIGIN } from "../../api/client";
import {
  getPublicConfig,
  listFavorites,
  listProperties,
  setFavorite,
  type Property,
} from "../../api/properties";
import { getSession } from "../../auth/session";
import searchIcon from "../../static/icons/search.svg";
import filterIcon from "../../static/icons/filter.svg";
import heartIcon from "../../static/icons/heart.svg";

const properties = ref<Property[]>([]),
  favoriteIDs = ref<string[]>([]),
  keyword = ref(""),
  selectedArea = ref("全部");
const selectedPrice = ref(0),
  showFavoritesOnly = ref(false),
  configuredLocations = ref<{ label: string; keyword: string }[]>([]),
  pagePresentation = ref({ title: "找一处合适的家", subtitle: "按地区和预算筛选", backgroundImage: "" }),
  channelFilter = ref<{
    keyword: string;
    latitude: number;
    longitude: number;
    radiusKm: number;
    title?: string;
  } | null>(null),
  loading = ref(false),
  loadFailed = ref(false);
const priceRanges = [
  { label: "不限价格", min: 0, max: 0 },
  { label: "≤1500", min: 0, max: 1500 },
  { label: "1500–2500", min: 1500, max: 2500 },
  { label: "2500–3500", min: 2500, max: 3500 },
  { label: "≥3500", min: 3500, max: 0 },
];
const areas = computed(() =>
  configuredLocations.value.length
    ? [{ label: "全部", keyword: "" }, ...configuredLocations.value]
    : [{ label: "全部", keyword: "" }],
);
const filteredProperties = computed(() => {
  const query = keyword.value.trim().toLowerCase(),
    price = priceRanges[selectedPrice.value];
  return properties.value.filter((item) => {
    const location = areas.value.find(
      (area) => area.label === selectedArea.value,
    );
    const channelDistance =
      channelFilter.value &&
      channelFilter.value.latitude &&
      channelFilter.value.longitude &&
      item.latitude &&
      item.longitude
        ? distanceKm(
            channelFilter.value.latitude,
            channelFilter.value.longitude,
            item.latitude,
            item.longitude,
          )
        : null;
    const channelMatched =
      !channelFilter.value ||
      (channelDistance !== null
        ? channelDistance <= channelFilter.value.radiusKm
        : !channelFilter.value.keyword ||
          item.location?.includes(channelFilter.value.keyword));
    return (
      channelMatched &&
      (!showFavoritesOnly.value || favoriteIDs.value.includes(item.id)) &&
      (!location?.keyword || item.location?.includes(location.keyword)) &&
      (!price.min || item.rentMonthly >= price.min) &&
      (!price.max || item.rentMonthly <= price.max) &&
      (!query ||
        `${item.title} ${item.community} ${item.location} ${item.layout}`
          .toLowerCase()
          .includes(query))
    );
  });
});
const distanceKm = (lat1: number, lon1: number, lat2: number, lon2: number) => {
  const rad = Math.PI / 180,
    dLat = (lat2 - lat1) * rad,
    dLon = (lon2 - lon1) * rad;
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(dLon / 2) ** 2;
  return 6371 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
};
const load = async () => {
  loading.value = true;
  loadFailed.value = false;
  try {
    const [items, config] = await Promise.all([
      listProperties(),
      getPublicConfig().catch(() => null),
    ]);
    properties.value = items;
    configuredLocations.value = config?.locations || [];
    pagePresentation.value = config?.pagePresentations?.properties || pagePresentation.value;
    if (getSession()?.role === "user")
      favoriteIDs.value = await listFavorites().catch(() => []);
    const pending = String(uni.getStorageSync("xiaopi_property_filter") || "");
    if (pending) {
      try {
        const parsed: unknown = JSON.parse(pending);
        if (parsed && typeof parsed === "object") {
          const filter = parsed as Record<string, unknown>;
          if (typeof filter.area === "string") selectedArea.value = filter.area;
          if (typeof filter.priceIndex === "number")
            selectedPrice.value = Math.max(0, Math.min(priceRanges.length - 1, filter.priceIndex));
          if (typeof filter.keyword === "string") keyword.value = filter.keyword;
          if (
            typeof filter.latitude === "number" &&
            typeof filter.longitude === "number" &&
            typeof filter.radiusKm === "number"
          ) {
            channelFilter.value = {
              keyword: typeof filter.keyword === "string" ? filter.keyword : "",
              latitude: filter.latitude,
              longitude: filter.longitude,
              radiusKm: filter.radiusKm,
              title: typeof filter.title === "string" ? filter.title : undefined,
            };
          }
        } else keyword.value = pending;
      } catch {
        keyword.value = pending;
      }
      uni.removeStorageSync("xiaopi_property_filter");
    }
    if (uni.getStorageSync("xiaopi_show_favorites")) {
      showFavoritesOnly.value = true;
      uni.removeStorageSync("xiaopi_show_favorites");
    }
  } catch (error) {
    loadFailed.value = true;
    uni.showToast({
      title: error instanceof Error ? error.message : "房源加载失败",
      icon: "none",
    });
  } finally {
    loading.value = false;
  }
};
onShow(load);
const open = (id: string) =>
    uni.navigateTo({ url: `/pages/properties/detail?id=${id}` }),
  requestHome = () => uni.navigateTo({ url: "/pages/requests/index" });
const coverUrl = (item: Property) => {
  const cover = item.media?.find((media) => media.type === "image")?.url;
  return cover
    ? cover.startsWith("http")
      ? cover
      : `${API_ORIGIN}${cover}`
    : "";
};
const toggleFavorite = async (id: string) => {
  if (!getSession()) {
    uni.navigateTo({ url: "/pages/auth/login" });
    return;
  }
  if (getSession()?.role === "admin") {
    uni.showToast({ title: "管理员无需关注房源", icon: "none" });
    return;
  }
  const next = !favoriteIDs.value.includes(id);
  try {
    await setFavorite(id, next);
    favoriteIDs.value = next
      ? [...favoriteIDs.value, id]
      : favoriteIDs.value.filter((item) => item !== id);
    uni.showToast({ title: next ? "已关注" : "已取消关注" });
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "操作失败",
      icon: "none",
    });
  }
};
</script>
<template>
  <view class="page" :style="pagePresentation.backgroundImage ? { backgroundImage: `linear-gradient(rgba(247,246,241,.92), rgba(247,246,241,.96)), url(${pagePresentation.backgroundImage.startsWith('http') ? pagePresentation.backgroundImage : API_ORIGIN.replace(/\/api$/, '') + pagePresentation.backgroundImage})`, backgroundSize: 'cover', backgroundAttachment: 'fixed' } : {}"
    ><view class="location"
      >{{ pagePresentation.title || "找一处合适的家" }} <text>{{ filteredProperties.length }} 套可租</text></view
    ><view v-if="pagePresentation.subtitle" class="page-subtitle">{{ pagePresentation.subtitle }}</view
    ><view class="search card"
      ><image :src="searchIcon" mode="aspectFit" />
      ><input
        v-model="keyword"
        placeholder="搜索小区、位置或户型"
        confirm-type="search"
      /><text v-if="keyword" class="clear" @click="keyword = ''">×</text></view
    ><view
      class="favorite-filter"
      :class="{ active: showFavoritesOnly }"
      @click="showFavoritesOnly = !showFavoritesOnly"
      ><image :src="heartIcon" mode="aspectFit" />{{ showFavoritesOnly ? "只看已关注" : "只看关注" }}</view
    ><view class="filter-block"
      ><view class="filter-label">地区</view
      ><scroll-view scroll-x class="chips"
        ><text
          v-for="area in areas"
          :key="area.label"
          :class="{ active: selectedArea === area.label }"
          @click="selectedArea = area.label"
          >{{ area.label }}</text
        ></scroll-view
      ></view
    ><view class="filter-block"
      ><view class="filter-label">月租预算</view
      ><scroll-view scroll-x class="chips"
        ><text
          v-for="(price, index) in priceRanges"
          :key="price.label"
          :class="{ active: selectedPrice === index }"
          @click="selectedPrice = index"
          >{{ price.label }}</text
        ></scroll-view
      ></view
    ><view v-if="loading" class="state card">正在加载房源...</view
    ><view v-else-if="loadFailed" class="state card"
      ><view>房源加载失败</view
      ><button size="mini" @click="load">重新加载</button></view
    ><view v-else-if="filteredProperties.length" class="grid"
      ><view
        v-for="item in filteredProperties"
        :key="item.id"
        class="property card"
        @click="open(item.id)"
        ><view class="cover"
          ><image
            v-if="coverUrl(item)"
            :src="coverUrl(item)"
            mode="aspectFill"
          /><text v-else>小皮</text><view class="badge">可租</view
          ><view
            class="favorite"
            :class="{ active: favoriteIDs.includes(item.id) }"
            @click.stop="toggleFavorite(item.id)"
            ><image :src="heartIcon" mode="aspectFit" /></view
          ></view
        ><view class="title small">{{ item.title || item.community }}</view
        ><view class="muted summary"
          >{{ item.layout }} · {{ item.location }}</view
        ><view class="price"
          >{{ item.rentMonthly }} <small>元/月</small></view
        ></view
      ></view
    ><view v-else class="empty-match card"
      ><view class="empty-icon">⌕</view
      ><view class="empty-title">暂时没有完全匹配的房源</view
      ><view class="muted">留下地区、预算和入住需求，小皮会为你继续匹配。</view
      ><view class="request-button" @click="requestHome"
        >提交找房需求</view
      ></view
    ><view class="request-banner" @click="requestHome"
      ><view
        ><strong>没找到合适的？</strong
        ><text>告诉小皮你的预算和入住要求</text></view
      ><text><image :src="filterIcon" mode="aspectFit" />发布需求 ›</text></view
    ></view
  >
</template>
<style scoped>
.location {
  font-size: 36rpx;
  color: #173f3c;
  margin: 28rpx 0 22rpx;
  font-weight: 800;
}
.location text {
  float: right;
  color: #0e8177;
  font-size: 24rpx;
  font-weight: 500;
}
.search {
  display: flex;
  align-items: center;
  gap: 16rpx;
  color: #8aa7a0;
  margin-bottom: 20rpx;
}
.search input {
  flex: 1;
  color: #073b3a;
}
.clear {
  padding: 6rpx 14rpx;
  font-size: 34rpx;
}
.favorite-filter {
  display: inline-block;
  margin-bottom: 8rpx;
  padding: 11rpx 20rpx;
  border: 1rpx solid #d7e8e3;
  border-radius: 999rpx;
  color: #638b85;
  font-size: 23rpx;
}
.favorite-filter.active {
  border-color: #efc7cf;
  background: #fff4f6;
  color: #d84862;
}
.filter-block {
  display: flex;
  align-items: center;
  gap: 14rpx;
  margin: 14rpx 0;
}
.filter-label {
  flex: 0 0 100rpx;
  color: #567e79;
  font-size: 24rpx;
  font-weight: 700;
}
.chips {
  white-space: nowrap;
  min-width: 0;
}
.chips text {
  display: inline-block;
  padding: 12rpx 22rpx;
  background: #fff;
  border-radius: 999rpx;
  color: #6b9891;
  margin-right: 12rpx;
  font-size: 24rpx;
}
.chips .active {
  background: #d9f7ef;
  color: #0f8177;
  font-weight: 700;
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18rpx;
  margin-top: 26rpx;
}
.property {
  padding: 0;
  overflow: hidden;
  margin: 0;
}
.cover {
  height: 230rpx;
  background: linear-gradient(145deg, #cdebe5, #86b8ae);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 90rpx;
  position: relative;
  overflow: hidden;
}
.cover image {
  width: 100%;
  height: 100%;
}
.badge {
  position: absolute;
  top: 14rpx;
  left: 14rpx;
  color: #fff;
  font-size: 20rpx;
  padding: 7rpx 13rpx;
  background: #0e8177;
  border-radius: 999rpx;
}
.favorite {
  position: absolute;
  top: 12rpx;
  right: 12rpx;
  display: flex;
  width: 58rpx;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.92);
  color: #557e78;
  font-size: 36rpx;
}
.favorite.active {
  color: #e44d66;
}
.small {
  font-size: 29rpx;
  margin: 18rpx 18rpx 8rpx;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.summary,
.property > .price {
  margin-left: 18rpx;
  margin-right: 18rpx;
}
.summary {
  min-height: 68rpx;
  font-size: 23rpx;
}
.property > .price {
  font-size: 34rpx;
  margin-top: 12rpx;
  margin-bottom: 20rpx;
}
.property > .price small {
  font-size: 22rpx;
}
.state,
.empty-match {
  padding: 76rpx 30rpx;
  text-align: center;
  color: #78a39d;
}
.empty-icon {
  font-size: 70rpx;
}
.empty-title {
  margin: 12rpx 0;
  font-size: 31rpx;
  font-weight: 700;
  color: #173f3c;
}
.request-button {
  display: inline-block;
  margin-top: 28rpx;
  padding: 18rpx 32rpx;
  border-radius: 999rpx;
  background: #0e8177;
  color: #fff;
}
.request-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  margin: 28rpx 0 8rpx;
  padding: 28rpx;
  border-radius: 28rpx;
  background: linear-gradient(135deg, #173f3c, #287168);
  color: #fff;
}
.request-banner strong,
.request-banner view text {
  display: block;
}
.request-banner view text {
  margin-top: 8rpx;
  color: rgba(255, 255, 255, 0.68);
  font-size: 22rpx;
}
.request-banner > text {
  flex: none;
  color: #f4d9a1;
  font-size: 25rpx;
  font-weight: 700;
}

/* Community-facing visual layer: warm, compact, and photo-first. */
.location { margin:18rpx 0 24rpx; color:#29382d; font-size:44rpx; letter-spacing:-2rpx; }
.location text { color:#83907e; font-size:22rpx; font-weight:500; }
.search { margin-bottom:16rpx; padding:19rpx 22rpx; border:1rpx solid #e6e2da; border-radius:20rpx; box-shadow:none; }
.search input { color:#334034; font-size:26rpx; }
.favorite-filter { margin:2rpx 0 14rpx; border-color:#d9ded2; border-radius:12rpx; color:#697564; }
.favorite-filter.active { border-color:#e9c9bc; background:#fcf1ed; color:#cc654b; }
.filter-block { margin:16rpx 0; gap:8rpx; }.filter-label { flex-basis:88rpx; color:#5a6757; font-size:22rpx; }.chips text { padding:12rpx 19rpx; margin-right:10rpx; border:1rpx solid #e8e5df; border-radius:12rpx; background:#fff; color:#6f796e; }.chips .active { border-color:#cbd7ad; background:#e6ecd7; color:#42523b; }
.grid { grid-template-columns:1fr 1fr; gap:14rpx; margin-top:24rpx; }.property { border:0; border-radius:18rpx; background:transparent; overflow:visible; }.cover { height:250rpx; border-radius:18rpx; background:#dde5d4; }.badge { top:12rpx; left:12rpx; padding:6rpx 10rpx; border-radius:8rpx; background:#536a45; }.favorite { top:10rpx; right:10rpx; width:54rpx; height:54rpx; border-radius:50%; color:#536a45; }.small { margin:15rpx 2rpx 5rpx; color:#29382d; font-size:28rpx; }.summary,.property>.price { margin-left:2rpx; margin-right:2rpx; }.summary { min-height:56rpx; color:#899188; font-size:21rpx; }.property>.price { margin-top:7rpx; margin-bottom:10rpx; color:#d76440; font-size:32rpx; }.property>.price small { font-size:20rpx; }.request-banner { margin-top:34rpx; padding:26rpx; border-radius:20rpx; background:#40543f; }.request-banner>text { color:#dce7ac; }

/* Search results use the same calm, photo-first rhythm as the home page. */
.location{margin:18rpx 0 22rpx;font-size:42rpx;line-height:1.15}.location text{padding-top:10rpx;font-size:22rpx}.search{gap:13rpx;padding:18rpx 20rpx;border-color:#ece5da;border-radius:18rpx}.search image{width:32rpx;height:32rpx;flex:none}.search input{font-size:26rpx}.favorite-filter{display:flex;align-items:center;gap:7rpx;margin:2rpx 0 16rpx;padding:10rpx 15rpx;border-radius:12rpx;font-size:22rpx}.favorite-filter image{width:24rpx;height:24rpx}.favorite-filter:not(.active) image{filter:invert(39%) sepia(10%) saturate(709%) hue-rotate(50deg)}.filter-block{margin:14rpx 0}.filter-label{font-size:22rpx}.chips text{padding:11rpx 17rpx;font-size:22rpx}.grid{display:block;margin-top:26rpx}.property{position:relative;display:flex;gap:18rpx;align-items:stretch;margin-bottom:18rpx;padding:12rpx;border:1rpx solid #eee6dc;border-radius:20rpx;background:#fffdf9;overflow:hidden}.cover{flex:none;width:244rpx;height:192rpx;border-radius:14rpx}.cover>text{color:#fff;font-size:27rpx}.badge{top:9rpx;left:9rpx;padding:5rpx 8rpx;font-size:18rpx}.favorite{top:9rpx;right:9rpx;width:44rpx;height:44rpx;background:rgba(47,44,34,.33)}.favorite image{width:28rpx;height:28rpx}.favorite.active{background:#d66d50}.property .small{min-width:0;flex:1;margin:9rpx 0 0;font-size:28rpx;line-height:1.3}.property .summary{position:absolute;display:-webkit-box;overflow:hidden;margin:50rpx 8rpx 0 274rpx;min-height:0;color:#7e867d;font-size:20rpx;line-height:1.45;-webkit-line-clamp:2;-webkit-box-orient:vertical}.property>.price{position:absolute;margin:139rpx 0 0 274rpx;color:#d76440;font-size:32rpx}.property>.price small{margin-left:3rpx;color:#8a8f85;font-size:19rpx}.request-banner{margin:32rpx 0 12rpx;padding:25rpx 24rpx;border-radius:18rpx}.request-banner>text{display:flex;align-items:center;gap:5rpx;font-size:22rpx}.request-banner>text image{width:22rpx;height:22rpx;filter:brightness(0) invert(1)}
.page-subtitle{margin:-15rpx 0 20rpx;color:#7e867d;font-size:22rpx}
.location{margin:18rpx 0 12rpx}.page-subtitle{margin:0 0 24rpx;line-height:1.55}.search{margin-bottom:20rpx}.filter-block{margin:18rpx 0}.grid{margin-top:30rpx}.request-banner{margin-top:40rpx}
.location{display:flex;align-items:flex-end;justify-content:space-between;gap:18rpx;font-size:44rpx;line-height:1.18}.location text{float:none;flex:none;padding:0 0 4rpx;font-size:22rpx;line-height:1.2}.filter-label{margin-bottom:10rpx;font-weight:600;line-height:1.25}.chips text{line-height:1.2}.property .small{font-weight:700}.property .summary{line-height:1.5}.request-banner strong{font-size:29rpx;line-height:1.3}.request-banner>view>text{display:block;margin-top:7rpx;line-height:1.45}
</style>
