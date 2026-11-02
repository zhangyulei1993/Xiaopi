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

const properties = ref<Property[]>([]),
  favoriteIDs = ref<string[]>([]),
  keyword = ref(""),
  selectedArea = ref("全部");
const selectedPrice = ref(0),
  showFavoritesOnly = ref(false),
  configuredLocations = ref<{ label: string; keyword: string }[]>([]),
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
    return (
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
    if (getSession()?.role === "user")
      favoriteIDs.value = await listFavorites().catch(() => []);
    const pending = String(uni.getStorageSync("xiaopi_property_filter") || "");
    if (pending) {
      keyword.value = pending;
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
  <view class="page"
    ><view class="location"
      >找一处合适的家 <text>{{ filteredProperties.length }} 套可租</text></view
    ><view class="search card"
      ><text>⌕</text
      ><input
        v-model="keyword"
        placeholder="搜索小区、位置或户型"
        confirm-type="search"
      /><text v-if="keyword" class="clear" @click="keyword = ''">×</text></view
    ><view class="favorite-filter" :class="{ active: showFavoritesOnly }" @click="showFavoritesOnly = !showFavoritesOnly">{{ showFavoritesOnly ? '♥ 只看已关注' : '♡ 只看关注' }}</view
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
          /><text v-else>🏠</text><view class="badge">可租</view
          ><view
            class="favorite"
            :class="{ active: favoriteIDs.includes(item.id) }"
            @click.stop="toggleFavorite(item.id)"
            >{{ favoriteIDs.includes(item.id) ? "♥" : "♡" }}</view
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
      ><text>帮我找房 ›</text></view
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
</style>
