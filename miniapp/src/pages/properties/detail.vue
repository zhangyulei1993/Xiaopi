<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { API_ORIGIN, request } from "../../api/client";
import {
  getPublicConfig,
  listFavorites,
  listProperties,
  setFavorite,
  type Property,
} from "../../api/properties";
import { getSession, hasRole } from "../../auth/session";
import { amenityIcon } from "../../utils/amenities";
import { usePagePresentation } from "../../utils/presentation";
const item = ref<Property | null>(null),
  contactPhone = ref(""),
  loading = ref(true),
  loadFailed = ref(false),
  favorite = ref(false);
const relatedRadiusKm = ref(3);
const allProperties = ref<Property[]>([]);
const activeMediaIndex = ref(0);
const playingVideoIndex = ref(-1);
const mapExpanded = ref(false);
const { presentation, loadPresentation, pageStyle } = usePagePresentation(
  "propertyDetail",
  "房源详情",
  "真实房源，放心查看",
);
const load = async (id = "") => {
  loading.value = true;
  loadFailed.value = false;
  try {
    await loadPresentation();
    const [items, config] = await Promise.all([
      listProperties(),
      getPublicConfig().catch(() => null),
    ]);
    allProperties.value = items;
    item.value = items.find((property) => property.id === id) || null;
    activeMediaIndex.value = 0;
    playingVideoIndex.value = -1;
    mapExpanded.value = false;
    contactPhone.value = config?.contactPhone || "";
    relatedRadiusKm.value = Number(config?.relatedPropertyRadiusKm) || 3;
    if (getSession()?.role === "user")
      favorite.value = (await listFavorites().catch(() => [])).includes(id);
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
onLoad((query) => load(String(query?.id || "")));
const mediaUrl = (url: string) =>
  url.startsWith("http") ? url : `${API_ORIGIN}${url}`;
const mediaItems = computed(() => item.value?.media || []);
const videoPoster = computed(() => {
  const image = mediaItems.value.find((media) => media.type === "image");
  return image ? mediaUrl(image.url) : "";
});
const handleMediaChange = (event: { detail: { current: number } }) => {
  activeMediaIndex.value = event.detail.current;
  playingVideoIndex.value = -1;
};
const previewImage = (url: string) => {
  const urls = mediaItems.value
    .filter((media) => media.type === "image")
    .map((media) => mediaUrl(media.url));
  if (urls.length) uni.previewImage({ current: mediaUrl(url), urls });
};
const amenities = computed(() => {
  const match = item.value?.description?.match(/^配套设施：([^\n]*)\n?/);
  if (!match?.[1]) return [];
  return match[1]
    .split(/[、,，]/)
    .map((name) => name.trim())
    .filter(Boolean)
    .map((name) => ({
      name,
      icon: amenityIcon(name),
    }));
});
const propertyDescription = computed(() =>
  String(item.value?.description || "")
    .replace(/^配套设施：[^\n]*\n?/, "")
    .trim(),
);
const hasLocation = computed(() =>
  Boolean(Number(item.value?.latitude) && Number(item.value?.longitude)),
);
const isAdmin = computed(() => hasRole("admin"));
const distanceKm = (lat1: number, lon1: number, lat2: number, lon2: number) => {
  const rad = Math.PI / 180;
  const dLat = (lat2 - lat1) * rad;
  const dLon = (lon2 - lon1) * rad;
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(dLon / 2) ** 2;
  return 6371 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
};
const nearbyProperties = computed(() => {
  if (!item.value || !hasLocation.value) return [];
  return allProperties.value
    .filter(
      (property) =>
        property.id !== item.value?.id &&
        property.status === "published" &&
        Number(property.latitude) &&
        Number(property.longitude),
    )
    .map((property) => ({
      ...property,
      distance: distanceKm(
        Number(item.value?.latitude),
        Number(item.value?.longitude),
        Number(property.latitude),
        Number(property.longitude),
      ),
    }))
    .filter((property) => property.distance <= relatedRadiusKm.value)
    .sort((left, right) => left.distance - right.distance)
    .slice(0, 4);
});
const propertyMarkers = computed(() =>
  hasLocation.value && item.value
    ? [
        {
          id: 1,
          latitude: Number(item.value.latitude),
          longitude: Number(item.value.longitude),
          width: 30,
          height: 30,
          callout: {
            content: item.value.community || "房源位置",
            display: "ALWAYS",
            padding: 6,
            borderRadius: 6,
            color: "#173f3c",
            bgColor: "#ffffff",
          },
        },
      ]
    : [],
);
const openPropertyLocation = () => {
  if (!item.value || !hasLocation.value) return;
  uni.openLocation({
    latitude: Number(item.value.latitude),
    longitude: Number(item.value.longitude),
    name: item.value.community || item.value.title,
    address: item.value.location,
    scale: 16,
  });
};
const requireUser = () => {
  if (getSession()?.role === "user") return true;
  if (getSession()?.role === "admin") return false;
  uni.navigateTo({ url: "/pages/auth/login" });
  return false;
};
const toggle = async () => {
  if (!item.value || !requireUser()) return;
  const next = !favorite.value;
  try {
    await setFavorite(item.value.id, next);
    favorite.value = next;
    uni.showToast({ title: next ? "已收藏" : "已取消收藏" });
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "操作失败",
      icon: "none",
    });
  }
};
const reserve = () => {
  if (!item.value || !requireUser()) return;
  uni.navigateTo({
    url: `/pages/requests/index?propertyId=${item.value.id}&title=${encodeURIComponent(item.value.title || item.value.community)}`,
  });
};
const openProperty = (id: string) =>
  uni.redirectTo({ url: `/pages/properties/detail?id=${id}` });
const editProperty = () => {
  if (!item.value) return;
  uni.navigateTo({ url: `/pages/admin/property-edit?id=${item.value.id}` });
};
const publishFromPreview = () => {
  if (!item.value || !["draft", "offline", "pending_review"].includes(item.value.status))
    return;
  uni.showModal({
    title: "确认发布房源",
    content: "该房源会立即在用户端显示。",
    confirmText: "确认发布",
    success: async (result) => {
      if (!result.confirm || !item.value) return;
      try {
        await request("/v1/properties/status", {
          method: "PATCH",
          data: {
            propertyId: item.value.id,
            status: "published",
            note: "管理员预览确认后发布",
          },
        });
        item.value.status = "published";
        uni.showToast({ title: "房源已发布" });
      } catch (error) {
        uni.showToast({
          title: error instanceof Error ? error.message : "发布失败",
          icon: "none",
        });
      }
    },
  });
};
const contact = () => {
  if (!getSession()) {
    uni.navigateTo({ url: "/pages/auth/login" });
    return;
  }
  if (!contactPhone.value) {
    uni.showToast({ title: "管理员电话尚未配置", icon: "none" });
    return;
  }
  uni.makePhoneCall({ phoneNumber: contactPhone.value });
};
</script>
<template>
  <view v-if="loading" class="page state">正在加载房源...</view
  ><view v-else-if="loadFailed" class="page state"
    >房源加载失败，请返回重试</view
  ><view v-else-if="item" class="page detail-page" :style="pageStyle"
    ><view class="page-presentation-head"><view class="page-presentation-title">{{ presentation.title }}</view><text>{{ presentation.subtitle }}</text></view
    ><view v-if="!mediaItems.length" class="detail-cover detail-cover-empty"
      ><text class="detail-cover-icon">⌂</text><text>暂无实拍图</text><small>联系小皮了解房源详情</small></view
    ><view v-else class="media-gallery"
      ><swiper class="media-swiper" circular :current="activeMediaIndex" @change="handleMediaChange">
        <swiper-item v-for="(media, index) in mediaItems" :key="`${media.url}-${index}`">
          <image
            v-if="media.type === 'image'"
            class="media-image"
            :src="mediaUrl(media.url)"
            mode="aspectFill"
            @click="previewImage(media.url)" />
          <view v-else class="video-stage">
            <video
              class="media-video"
              :src="mediaUrl(media.url)"
              :poster="videoPoster"
              controls
              show-play-btn
              show-progress
              show-fullscreen-btn
              enable-progress-gesture
              object-fit="contain"
              @play="playingVideoIndex = index"
              @pause="playingVideoIndex = -1"
              @ended="playingVideoIndex = -1"
            ></video>
          </view>
        </swiper-item>
      </swiper
      ><view class="media-counter">{{ activeMediaIndex + 1 }} / {{ mediaItems.length }}</view
      ><view v-if="mediaItems[activeMediaIndex]?.type === 'video'" class="media-kind">视频</view></view
    ><view class="card overview"
      ><view class="overview-top"
        ><view class="price">{{ item.rentMonthly }} <small>元/月</small></view
        ><view v-if="!isAdmin" class="heart" :class="{ active: favorite }" @click="toggle"
          >{{ favorite ? "♥" : "♡"
          }}<small>{{ favorite ? "已关注" : "关注" }}</small></view
        ></view
      ><view class="title">{{ item.title || item.community }}</view
      ><view class="muted">{{ item.layout }} · {{ item.location }}</view
      ><scroll-view class="key-facts" scroll-x
        ><view class="key-facts-row"><view><strong>{{ item.layout || "户型待确认" }}</strong><small>户型</small></view
        ><view><strong>{{ item.community || "小区待确认" }}</strong><small>所在小区</small></view
        ><view><strong>{{ item.feeText || "费用待确认" }}</strong><small>费用说明</small></view></view></scroll-view
      ></view
    ><view v-if="hasLocation" class="card location-card"
      ><view class="location-heading"
        ><view
          ><strong>位置与周边</strong><small>{{ item.location }}</small></view
        ><text @click="openPropertyLocation">打开地图</text></view
      ><view class="map-summary" @click="mapExpanded = !mapExpanded"
        ><view><strong>{{ mapExpanded ? "收起地图" : "查看地图位置" }}</strong><small>已保存房源定位</small></view
        ><text>{{ mapExpanded ? "⌃" : "›" }}</text></view
      ><map
        v-if="mapExpanded"
        class="detail-map"
        :latitude="Number(item.latitude)"
        :longitude="Number(item.longitude)"
        :markers="propertyMarkers"
        :scale="16"
      /><button v-if="mapExpanded" class="navigate-button" @click="openPropertyLocation">
        ⌁ 导航去这里
      </button></view
    ><view class="card"
      ><view class="title small">房源信息</view
      ><view class="row"
        ><text>小区</text><text>{{ item.community }}</text></view
      ><view class="row"
        ><text>位置</text><text>{{ item.location }}</text></view
      ><view class="row"
        ><text>户型</text><text>{{ item.layout }}</text></view
      ></view
    ><view v-if="amenities.length" class="card amenity-card"
      ><view class="title small">配套设施</view
      ><view class="amenity-grid"
        ><view
          v-for="amenity in amenities"
          :key="amenity.name"
          class="amenity-item"
          ><view class="amenity-icon"
            ><image :src="amenity.icon" mode="aspectFit" /></view
          ><text class="amenity-name">{{ amenity.name }}</text></view
        ></view
      ></view
    ><view class="card"
      ><view class="title small">房源描述</view
      ><view class="desc">{{ propertyDescription || "暂无描述" }}</view></view
    ><view v-if="nearbyProperties.length" class="card nearby-card"
      ><view class="nearby-heading"
        ><view><view class="title small">附近房源</view><small>距此房源 {{ relatedRadiusKm }} 公里内</small></view
        ><text>{{ nearbyProperties.length }} 套</text></view
      ><view
        v-for="property in nearbyProperties"
        :key="property.id"
        class="nearby-item"
        @click="openProperty(property.id)"
        ><view><strong>{{ property.title || property.community }}</strong><small>{{ property.community }} · {{ property.layout }}</small></view
        ><view class="nearby-price"><strong>{{ property.rentMonthly }} 元/月</strong><small>{{ property.distance.toFixed(1) }}km</small></view
      ></view></view
    ><view class="bottom-space" /><view v-if="isAdmin" class="action-bar admin-action-bar"
      ><view class="contact-action" @click="editProperty"
        ><text>✎</text><small>继续编辑</small></view
      ><view
        v-if="['draft', 'offline', 'pending_review'].includes(item.status)"
        class="reserve"
        @click="publishFromPreview"
        >确认发布</view
      ><view v-else class="reserve disabled">已发布</view></view
    ><view v-else class="action-bar"
      ><view class="contact-action" @click="contact"
        ><text>☎</text><small>联系小皮</small></view
      ><view class="reserve" @click="reserve">预约房源</view></view
    ></view
  ><view v-else class="page state">房源不存在或已下架</view>
</template>
<style scoped>
.state {
  text-align: center;
  padding-top: 180rpx;
  color: #78a39d;
}
.detail-page {
  padding-bottom: 150rpx;
}
.detail-cover {
  height: 420rpx;
  border-radius: 28rpx;
  background: #dff6ef;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 160rpx;
}
.media-list {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
}
.media-image,
.media-video {
  width: 100%;
  height: 420rpx;
  border-radius: 28rpx;
  background: #dff6ef;
}
.overview-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}
.location-card {
  padding: 28rpx;
}
.location-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20rpx;
  margin-bottom: 18rpx;
}
.location-heading strong,
.location-heading small {
  display: block;
}
.location-heading strong {
  color: #173f3c;
  font-size: 32rpx;
}
.nearby-card {
  padding: 28rpx;
}
.nearby-heading,
.nearby-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
}
.nearby-heading {
  margin-bottom: 12rpx;
}
.nearby-heading small,
.nearby-item small {
  display: block;
  margin-top: 6rpx;
  color: #78a39d;
  font-size: 22rpx;
}
.nearby-heading > text {
  color: #0e8177;
  font-size: 23rpx;
}
.nearby-item {
  min-height: 104rpx;
  border-top: 1rpx solid #edf4f1;
}
.nearby-item > view:first-child {
  flex: 1;
  min-width: 0;
}
.nearby-item strong {
  color: #173f3c;
  font-size: 27rpx;
}
.nearby-price {
  text-align: right;
  flex: none;
}
.nearby-price strong {
  color: #d47d32;
  font-size: 24rpx;
}
.location-heading small {
  margin-top: 8rpx;
  color: #78a39d;
  font-size: 21rpx;
}
.location-heading > text {
  flex: none;
  color: #0e8177;
  font-size: 24rpx;
  font-weight: 700;
}
.detail-map {
  width: 100%;
  height: 380rpx;
  overflow: hidden;
  border-radius: 18rpx;
}
.navigate-button {
  margin: 18rpx 0 0;
  border: 0;
  border-radius: 14rpx;
  background: #edf5f2;
  color: #0e8177;
  font-size: 25rpx;
  font-weight: 700;
}
.navigate-button::after {
  border: 0;
}
.price small {
  font-size: 22rpx;
}
.heart {
  display: flex;
  align-items: center;
  gap: 8rpx;
  color: #638b85;
  font-size: 40rpx;
}
.heart small {
  font-size: 22rpx;
}
.heart.active {
  color: #e44d66;
}
.small {
  font-size: 34rpx;
  margin-bottom: 20rpx;
}
.tags {
  display: flex;
  gap: 16rpx;
  margin-top: 24rpx;
}
.tags text {
  padding: 12rpx 22rpx;
  border-radius: 999rpx;
  background: #e9faf5;
  color: #0f8177;
}
.row {
  display: flex;
  justify-content: space-between;
  padding: 22rpx 0;
  border-bottom: 1rpx solid #eef5f2;
  color: #6e9a93;
}
.row text:last-child {
  color: #073b3a;
}
.amenity-card {
  padding-bottom: 32rpx;
}
.amenity-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 26rpx 14rpx;
}
.amenity-item {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: center;
  gap: 12rpx;
}
.amenity-icon {
  display: flex;
  width: 72rpx;
  height: 72rpx;
  align-items: center;
  justify-content: center;
  border: 1rpx solid #d5eee7;
  border-radius: 22rpx;
  background: #eaf8f4;
  box-shadow: 0 8rpx 20rpx rgba(26, 111, 100, 0.08);
}
.amenity-icon image {
  width: 42rpx;
  height: 42rpx;
}
.amenity-name {
  max-width: 100%;
  overflow: hidden;
  color: #315f59;
  font-size: 24rpx;
  line-height: 1.25;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.desc {
  line-height: 1.8;
  color: #57857e;
}
.action-bar {
  position: fixed;
  z-index: 5;
  left: 0;
  right: 0;
  bottom: 0;
  display: grid;
  grid-template-columns: 180rpx 1fr;
  gap: 18rpx;
  padding: 20rpx 32rpx calc(20rpx + env(safe-area-inset-bottom));
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 -8rpx 30rpx rgba(15, 70, 65, 0.1);
}
.contact-action {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  color: #315f5a;
}
.contact-action text {
  font-size: 31rpx;
}
.contact-action small {
  font-size: 21rpx;
}
.reserve {
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 999rpx;
  background: #0e8177;
  color: #fff;
  font-size: 30rpx;
  font-weight: 700;
}
.reserve.disabled {
  background: #b9cbc7;
}

/* Keep detail information-first while matching the light community palette. */
.detail-page { padding-bottom:164rpx; }.detail-cover,.media-image,.media-video { border-radius:24rpx; background:#dce4d5; }.detail-cover { height:440rpx; }.media-list { gap:14rpx; }.media-image,.media-video { height:440rpx; }.overview { margin-top:18rpx; border:0; border-radius:24rpx; }.overview .title { margin-top:18rpx; font-size:40rpx; }.overview .muted { margin-top:8rpx; font-size:25rpx; }.overview .price { color:#d76440; }.heart { color:#657162; }.heart.active { color:#d76440; }.tags text { border-radius:8rpx; background:#edf0e6; color:#5b6956; }.location-card,.nearby-card { border:0; border-radius:24rpx; }.location-heading strong { color:#29382d; }.location-heading > text,.navigate-button { color:#536a45; }.navigate-button { border-radius:12rpx; background:#e9eee0; }.detail-map { border-radius:16rpx; }.row { border-color:#eeece7; }.row text:first-child { color:#8a9389; }.desc { color:#64705f; }.amenity-icon { background:#edf0e6; }.nearby-heading > text,.nearby-price strong { color:#d76440; }.nearby-item { border-color:#eeece7; }.action-bar { background:rgba(255,255,255,.97); box-shadow:0 -6rpx 24rpx rgba(45,55,43,.07); }.contact-action { color:#5a6657; }.reserve { border-radius:16rpx; background:#40543f; }.reserve.disabled { background:#b5bdb0; }
.detail-page .page-presentation-head{margin-bottom:28rpx}.detail-page .overview{margin-top:22rpx}.detail-page .card{margin-bottom:24rpx}.overview .title{line-height:1.35}.amenity-grid{row-gap:14rpx}
.overview .price{font-size:50rpx;line-height:1.1}.overview .price small{font-size:23rpx}.overview .tags{margin-top:16rpx}.detail-page .title.small{font-size:33rpx;line-height:1.3}.row{min-height:82rpx;padding:24rpx 0;align-items:center}.row text:last-child{max-width:70%;line-height:1.45;text-align:right}.desc{margin-top:14rpx;font-size:26rpx;line-height:1.72}.location-heading small{margin-top:8rpx;line-height:1.45}.nearby-item{padding:22rpx 0}.nearby-item small{margin-top:7rpx;line-height:1.4}.action-bar{padding-top:18rpx;padding-bottom:calc(18rpx + env(safe-area-inset-bottom))}
.media-gallery{position:relative;overflow:hidden;border-radius:24rpx;background:#dce4d5}.media-swiper{width:100%;height:440rpx}.media-gallery .media-image,.media-gallery .media-video,.video-stage{display:block;width:100%;height:100%;border-radius:0}.video-stage{position:relative;background:#40543f}.play-video{position:absolute;top:50%;left:50%;display:flex;align-items:center;gap:12rpx;padding:18rpx 26rpx;border-radius:999rpx;transform:translate(-50%,-50%);background:rgba(25,37,27,.76);color:#fff}.play-video text{font-size:28rpx;line-height:1}.play-video small{font-size:24rpx;white-space:nowrap}.media-counter,.media-kind{position:absolute;bottom:22rpx;padding:9rpx 16rpx;border-radius:999rpx;background:rgba(24,34,25,.64);color:#fff;font-size:22rpx;font-variant-numeric:tabular-nums}.media-counter{right:22rpx}.media-kind{left:22rpx}.detail-cover-empty{flex-direction:column;gap:10rpx;background:linear-gradient(145deg,#e8e2d5,#d7dfcd);color:#465440;font-size:31rpx;font-weight:650}.detail-cover-empty .detail-cover-icon{font-size:116rpx;line-height:1}.detail-cover-empty small{color:#788071;font-size:23rpx;font-weight:400}.overview{padding:30rpx 28rpx}.overview-top .price{flex:1;min-width:0;white-space:nowrap}.heart{flex:none}.overview .title{margin-top:16rpx;color:#263227;font-size:40rpx;font-weight:750;letter-spacing:-.6rpx}.overview .muted{margin-top:9rpx;color:#6d766b;font-size:26rpx}.key-facts{width:100%;margin-top:24rpx;white-space:normal}.key-facts-row{display:inline-flex;gap:12rpx;white-space:nowrap}.key-facts-row>view{box-sizing:border-box;width:224rpx;padding:16rpx 14rpx;border-radius:14rpx;background:#eeece5}.key-facts strong{display:block;overflow:hidden;color:#384438;font-size:24rpx;font-weight:650;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.key-facts small{display:block;margin-top:7rpx;color:#8a9087;font-size:19rpx}.map-summary{display:flex;align-items:center;justify-content:space-between;padding:20rpx 0 2rpx}.map-summary strong,.map-summary small{display:block}.map-summary strong{color:#425240;font-size:27rpx}.map-summary small{margin-top:6rpx;color:#879087;font-size:21rpx}.map-summary>text{color:#596b52;font-size:38rpx;line-height:1}.play-video:active,.map-summary:active{opacity:.8}.heart:active,.reserve:active,.contact-action:active{transform:scale(.98);opacity:.86}
</style>
