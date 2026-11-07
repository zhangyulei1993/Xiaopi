<script setup lang="ts">
import { onLoad, onShareAppMessage, onShareTimeline } from "@dcloudio/uni-app";
import { computed, nextTick, ref } from "vue";
import { request, resolveAssetUrl } from "../../api/client";
import {
  getPublicConfig,
  deleteProperty,
  listFavorites,
  listProperties,
  recordFootprint,
  setFavorite,
  type Property,
} from "../../api/properties";
import { getSession, hasRole } from "../../auth/session";
import { amenityIcon } from "../../utils/amenities";
import { callPhone } from "../../utils/phone";
import { usePagePresentation } from "../../utils/presentation";
import { listingTagValue, uniqueListingTags } from "../../utils/listing-title";
const item = ref<Property | null>(null),
  contactPhone = ref(""),
  loading = ref(true),
  loadFailed = ref(false),
  favorite = ref(false);
const relatedRadiusKm = ref(3);
const detailConfig = ref({
  modules: [
    { key: "overview", title: "房屋概况", enabled: true }, { key: "location", title: "位置与周边", enabled: true },
    { key: "facilities", title: "房屋设施与环境", enabled: true }, { key: "description", title: "房源描述", enabled: true }, { key: "nearby", title: "附近房源", enabled: true },
  ],
  nearby: { radiusKm: 3, relation: "distance", sort: "distance", limit: 4 },
  facilities: [] as Array<{ key: string; label: string; group: string; enabled: boolean }>,
});
// Public detail labels use the same global set as public list cards.  An
// operator sees exactly that content plus the doorplate when it is enabled
// for the operator audience; no other detail-only role divergence is allowed.
const customerDetailTagFields = ref<string[]>(["community", "layout", "rentType", "fee"]);
const operatorDoorplateEnabled = ref(false);
const customerShareTagFields = ref<string[]>(["community", "layout", "rentType", "fee"]);
const shareCardConfig = ref({
  titlePrefix: "",
  titleFields: ["community", "layout"],
  posterFields: ["rentMonthly", "depositPayment", "community", "areaSqm", "floorInfo", "availableDate"],
});
const allProperties = ref<Property[]>([]);
const featuredChannels = ref<Array<{ latitude?: number; longitude?: number; radiusKm?: number }>>([]);
const activeMediaIndex = ref(0);
const playingVideoIndex = ref(-1);
const videoDimensions = ref<Record<number, { width: number; height: number }>>({});
const mapExpanded = ref(false);
const shareCardImage = ref("");
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
	if (item.value && getSession()?.role === "user") await recordFootprint(id).catch(() => null);
    activeMediaIndex.value = 0;
    playingVideoIndex.value = -1;
    videoDimensions.value = {};
    mapExpanded.value = false;
    contactPhone.value = config?.contactPhone || "";
    relatedRadiusKm.value = Number(config?.relatedPropertyRadiusKm) || 3;
    detailConfig.value = {
      modules: config?.propertyDetail?.modules?.length ? config.propertyDetail.modules : detailConfig.value.modules,
      nearby: { ...detailConfig.value.nearby, ...(config?.propertyDetail?.nearby || {}) },
      facilities: config?.propertyDetail?.facilities || [],
    };
    featuredChannels.value = config?.featuredChannels || [];
    customerDetailTagFields.value = config?.listingTitle?.customerDetailFields?.length
      ? config.listingTitle.customerDetailFields
      : (config?.listingTitle?.enabledFields?.length ? config.listingTitle.enabledFields : ["community", "layout", "rentType", "fee"]);
    customerShareTagFields.value = config?.listingTitle?.customerAllowedFields?.length
      ? config.listingTitle.customerAllowedFields
      : customerDetailTagFields.value;
    operatorDoorplateEnabled.value = Boolean(config?.listingTitle?.adminEnabledFields?.includes("unit"));
    shareCardConfig.value = {
      titlePrefix: String(config?.shareCard?.titlePrefix || ""),
      titleFields: config?.shareCard?.titleFields?.length ? config.shareCard.titleFields : ["community", "layout"],
      posterFields: config?.shareCard?.posterFields?.length ? config.shareCard.posterFields : ["rentMonthly", "depositPayment", "community", "areaSqm", "floorInfo", "availableDate"],
    };
    if (getSession()?.role === "user")
      favorite.value = (await listFavorites().catch((): string[] => [])).includes(id);
  } catch (error) {
    loadFailed.value = true;
    uni.showToast({
      title: error instanceof Error ? error.message : "房源加载失败",
      icon: "none",
    });
  } finally {
    loading.value = false;
    await nextTick();
    void generateShareCard();
  }
};
onLoad((query) => {
  uni.showShareMenu({ menus: ["shareAppMessage", "shareTimeline"] });
  load(String(query?.id || ""));
});
const mediaUrl = resolveAssetUrl;
const propertyCover = (property: Property) => {
  const cover = property.media?.find((media) => media.type === "image");
  return cover ? mediaUrl(cover.url) : "";
};
const mediaItems = computed(() => item.value?.media || []);
const mediaLabel = (media: { type: string }, index: number) =>
  media.type === "video" ? "视频" : `实拍 ${index + 1}`;
const isAdmin = computed(() => hasRole("admin"));
const detailTags = computed(() => {
  const current = item.value;
  if (!current) return [];
  const fields = [
    ...(isAdmin.value && operatorDoorplateEnabled.value ? ["unit"] : []),
    ...customerDetailTagFields.value.filter((field) => field !== "community" && field !== "unit"),
  ];
  return uniqueListingTags(fields.map((field) => listingTagValue(current, field)));
});
const detailModule = (key: string) => detailConfig.value.modules.find((module) => module.key === key);
const detailModuleEnabled = (key: string) => detailModule(key)?.enabled !== false;
const detailModuleTitle = (key: string, fallback: string) => detailModule(key)?.title || fallback;
const priceText = computed(() =>
  Number(item.value?.rentMonthly) > 0 ? String(item.value?.rentMonthly) : "待补充",
);
const priceUnit = computed(() =>
  Number(item.value?.rentMonthly) > 0 ? "元/月" : "租金",
);
const isAdminPreview = computed(
  () => isAdmin.value && !["published", "available"].includes(item.value?.status || ""),
);
const adminPreviewLabel = computed(() => {
  const status = item.value?.status;
  if (status === "offline") return "已下架预览";
  if (status === "pending_review") return "待发布预览";
  return "草稿预览";
});
const adminPrimaryActions = computed(() => {
  switch (item.value?.status) {
    case "draft":
    case "pending_review":
      return [{ status: "available", label: "发布房源", tone: "primary" }];
    case "published":
      return [
        { status: "available", label: "设为可租", tone: "primary" },
        { status: "rented", label: "标记已租", tone: "secondary" },
        { status: "offline", label: "下架", tone: "secondary" },
      ];
    case "available":
      return [
        { status: "rented", label: "标记已租", tone: "primary" },
        { status: "offline", label: "下架", tone: "secondary" },
      ];
    case "rented":
      return [{ status: "available", label: "设为可租", tone: "primary" }];
    case "offline":
      return [
        { status: "available", label: "上架可租", tone: "primary" },
        { status: "rented", label: "设为已租", tone: "secondary" },
      ];
    default:
      return [];
  }
});
const activeVideoIsPortrait = computed(() => {
  const active = mediaItems.value[activeMediaIndex.value];
  const dimensions = videoDimensions.value[activeMediaIndex.value];
  return Boolean(
    active?.type === "video" &&
      dimensions?.height &&
      dimensions.height > dimensions.width,
  );
});
const mediaViewportHeight = computed(() =>
  activeVideoIsPortrait.value ? 720 : 440,
);
const videoPoster = computed(() => {
  const image = mediaItems.value.find((media) => media.type === "image");
  return image ? mediaUrl(image.url) : "";
});
const shouldLoadMedia = (index: number) => {
  const count = mediaItems.value.length;
  if (count < 4) return true;
  const current = activeMediaIndex.value;
  return Math.abs(index - current) <= 1 || (index === 0 && current === count - 1) || (index === count - 1 && current === 0);
};
const shareFieldMeta: Record<string, { label: string; value: (property: Property) => string }> = {
  community: { label: "小区", value: (property) => property.community || "" },
  layout: { label: "户型", value: (property) => String(property.layout || "").split("·")[0] },
  roomNo: { label: "门牌", value: (property) => [property.buildingNo, property.unitNo, property.roomNo].filter(Boolean).join("") },
  rentMonthly: { label: "月租", value: (property) => Number(property.rentMonthly) > 0 ? `¥${property.rentMonthly}/月` : "租金面议" },
  listingSource: { label: "来源", value: (property) => property.listingSource || "直租" },
  rentType: { label: "出租", value: (property) => property.rentType || "整租" },
  depositPayment: { label: "押付", value: (property) => property.depositPayment || "押付面议" },
  areaSqm: { label: "面积", value: (property) => property.areaSqm ? `${property.areaSqm}㎡` : "待补充" },
  floorInfo: { label: "楼层", value: (property) => property.floorInfo || "待补充" },
  availableDate: { label: "入住", value: (property) => property.availableDate || "拎包入住" },
};
const shareFieldTagMap: Record<string, string> = { layout: "layout", roomNo: "unit", listingSource: "listingSource", rentType: "rentType", areaSqm: "area" };
const isPublicShareField = (key: string) => !shareFieldTagMap[key] || customerShareTagFields.value.includes(shareFieldTagMap[key]);
const shareFieldValue = (property: Property, key: string) => shareFieldMeta[key]?.value(property) || "";
const shareTitle = computed(() => {
  const current = item.value;
  if (!current) return "小皮租房房源";
  const values = shareCardConfig.value.titleFields.filter(isPublicShareField).map((key) => shareFieldValue(current, key)).filter(Boolean);
  return [shareCardConfig.value.titlePrefix, ...values].filter(Boolean).join(" · ") || current.community || "精选房源";
});
const shareImageUrl = computed(() => {
  if (shareCardImage.value) return shareCardImage.value;
  const current = item.value;
  const cover = current?.media?.find((media) => media.type === "image");
  return cover ? mediaUrl(cover.url) : "";
});
const compactText = (value: string, maxLength: number) =>
  value.length > maxLength ? `${value.slice(0, maxLength - 1)}…` : value;
const generateShareCard = async () => {
  const current = item.value;
  const cover = current?.media?.find((media) => media.type === "image");
  if (!current || !cover) return;
  try {
    const source = mediaUrl(cover.url);
    const info = await new Promise<UniApp.GetImageInfoSuccessData>((resolve, reject) =>
      uni.getImageInfo({ src: source, success: resolve, fail: reject }),
    );
    const width = 600;
    const height = 480;
    const imageHeight = 276;
    const context = uni.createCanvasContext("share-card-canvas");
    const sourceRatio = info.width / info.height;
    const targetRatio = width / imageHeight;
    let sx = 0;
    let sy = 0;
    let sw = info.width;
    let sh = info.height;
    if (sourceRatio > targetRatio) {
      sw = info.height * targetRatio;
      sx = (info.width - sw) / 2;
    } else {
      sh = info.width / targetRatio;
      sy = (info.height - sh) / 2;
    }
    context.setFillStyle("#ffffff");
    context.fillRect(0, 0, width, height);
    context.drawImage(info.path, sx, sy, sw, sh, 0, 0, width, imageHeight);
    const posterKeys = shareCardConfig.value.posterFields.filter(isPublicShareField);
    const rentEnabled = posterKeys.includes("rentMonthly");
    context.setFillStyle(rentEnabled ? "#d85f42" : "#26372c");
    context.setFontSize(32);
    context.fillText(compactText(rentEnabled ? shareFieldValue(current, "rentMonthly") : shareTitle.value, 24), 24, 324);
    const secondaryKey = posterKeys.find((key) => key !== "rentMonthly");
    if (rentEnabled && secondaryKey) {
      context.setFillStyle("#7b8579");
      context.setFontSize(18);
      context.fillText(compactText(shareFieldValue(current, secondaryKey), 14), 250, 324);
    }
    context.setFillStyle("#26372c");
    context.setFontSize(27);
    context.fillText(compactText(shareTitle.value, 19), 24, 366);
    const detailKeys = posterKeys.filter((key) => key !== "rentMonthly" && key !== secondaryKey).slice(0, 3);
    const columns = detailKeys.map((key) => [shareFieldMeta[key]?.label || "信息", shareFieldValue(current, key)]);
    columns.forEach(([label, value], index) => {
      const x = 24 + index * 192;
      context.setFillStyle("#92988f");
      context.setFontSize(17);
      context.fillText(label, x, 409);
      context.setFillStyle("#26372c");
      context.setFontSize(22);
      context.fillText(compactText(value, 8), x, 446);
    });
    await new Promise<void>((resolve) => context.draw(false, resolve));
    const result = await new Promise<UniApp.CanvasToTempFilePathRes>((resolve, reject) =>
      uni.canvasToTempFilePath({
        canvasId: "share-card-canvas",
        width,
        height,
        destWidth: 1200,
        destHeight: 960,
        fileType: "jpg",
        quality: 0.9,
        success: resolve,
        fail: reject,
      }),
    );
    shareCardImage.value = result.tempFilePath;
  } catch {
    shareCardImage.value = "";
  }
};
const propertyMessagePath = computed(
  () => `/pages/properties/detail?id=${encodeURIComponent(item.value?.id || "")}`,
);
const customerSessionFrom = computed(() =>
  JSON.stringify({
    source: "property_detail",
    propertyId: item.value?.id || "",
    community: item.value?.community || "",
  }),
);
const friendSharePayload = () => {
  const current = item.value;
  return {
    title: shareTitle.value,
    path: propertyMessagePath.value,
    imageUrl: shareImageUrl.value,
  };
};
const timelineSharePayload = () => ({
  title: shareTitle.value,
  query: `id=${encodeURIComponent(item.value?.id || "")}`,
  imageUrl: shareImageUrl.value,
});
onShareAppMessage(() => friendSharePayload());
onShareTimeline(() => timelineSharePayload());
const handleMediaChange = (event: { detail: { current: number } }) => {
  activeMediaIndex.value = event.detail.current;
  playingVideoIndex.value = -1;
};
const selectMedia = (index: number) => {
  activeMediaIndex.value = index;
  playingVideoIndex.value = -1;
};
const readVideoMetadata = (
  index: number,
  event: { detail?: { width?: number; height?: number } } | Event,
) => {
  const detail = "detail" in event ? event.detail : undefined;
  const width = Number(detail?.width);
  const height = Number(detail?.height);
  if (!width || !height) return;
  videoDimensions.value = {
    ...videoDimensions.value,
    [index]: { width, height },
  };
};
const previewImage = (url: string) => {
  const urls = mediaItems.value
    .filter((media) => media.type === "image")
    .map((media) => mediaUrl(media.url));
  if (urls.length) uni.previewImage({ current: mediaUrl(url), urls });
};
const amenities = computed(() => {
  const match = String(item.value?.description || "").match(/配套设施[ \t]*[：:][ \t]*([^\n]*)/);
  if (!match?.[1]) return [];
  const enabledLabels = detailConfig.value.facilities.filter((facility) => facility.enabled).map((facility) => facility.label);
  return match[1]
    .split(/[、,，]/)
    .map((name) => name.trim())
    .filter((name) => name && (!enabledLabels.length || enabledLabels.includes(name)))
    .map((name) => ({
      name,
      icon: amenityIcon(name),
      group: detailConfig.value.facilities.find((facility) => facility.label === name)?.group || "其他设施",
    }));
});
const amenityGroups = computed(() => {
  const groups = new Map<string, typeof amenities.value>();
  amenities.value.forEach((amenity) => {
    const current = groups.get(amenity.group) || [];
    current.push(amenity);
    groups.set(amenity.group, current);
  });
  return Array.from(groups, ([title, items]) => ({ title, items }));
});
const environmentHighlights = computed(() => {
  const current = item.value;
  if (!current) return [];
  return [
    current.utilitiesText && { label: "水电", value: current.utilitiesText },
    { label: "楼内", value: current.elevator || "步梯" },
    Number(current.metroMinutes) > 0 && { label: "通勤", value: `步行至地铁 ${current.metroMinutes} 分钟` },
  ].filter(Boolean) as Array<{ label: string; value: string }>;
});
const propertyDescription = computed(() =>
  String(item.value?.description || "")
    .replace(/配套设施[ \t]*[：:][ \t]*[^\n]*\n?/, "")
    .trim(),
);
const hasLocation = computed(() =>
  Boolean(Number(item.value?.latitude) && Number(item.value?.longitude)),
);
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
  const candidates = allProperties.value
    .filter(
      (property) =>
        property.id !== item.value?.id &&
        ["published", "available"].includes(property.status) &&
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
    .filter((property) => property.distance <= Number(detailConfig.value.nearby.radiusKm || relatedRadiusKm.value));
  const sameChannel = (property: Property) => featuredChannels.value.some((channel) => {
    if (!channel.latitude || !channel.longitude || !item.value?.latitude || !item.value?.longitude) return false;
    const radius = Number(channel.radiusKm) || 3;
    return distanceKm(Number(item.value.latitude), Number(item.value.longitude), Number(channel.latitude), Number(channel.longitude)) <= radius
      && distanceKm(Number(property.latitude), Number(property.longitude), Number(channel.latitude), Number(channel.longitude)) <= radius;
  });
  return candidates
    .filter((property) => detailConfig.value.nearby.relation !== "channel" || sameChannel(property))
    .sort((left, right) => detailConfig.value.nearby.sort === "newest"
      ? String(right.id).localeCompare(String(left.id))
      : left.distance - right.distance)
    .slice(0, Number(detailConfig.value.nearby.limit) || 4);
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
            color: "#29382d",
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
const openAllProperties = () => uni.switchTab({ url: "/pages/properties/index" });
const editProperty = () => {
  if (!item.value) return;
  uni.navigateTo({ url: `/pages/admin/property-edit?id=${item.value.id}` });
};
const updateStatusFromPreview = (nextStatus: string, label: string) => {
  if (!item.value) return;
  uni.showModal({
    title: `确认${label}`,
    content: nextStatus === "offline" ? "下架后，用户端将不再展示该房源。" : nextStatus === "rented" ? "标记后，房源会归入已租列表。" : "该状态会立即同步到房源管理。",
    confirmText: "确认",
    success: async (result) => {
      if (!result.confirm || !item.value) return;
      try {
        await request("/v1/properties/status", {
          method: "PATCH",
          data: {
            propertyId: item.value.id,
            status: nextStatus,
            note: `管理员详情页${label}`,
          },
        });
        item.value.status = nextStatus;
        uni.showToast({ title: "状态已更新" });
      } catch (error) {
        uni.showToast({
          title: error instanceof Error ? error.message : "发布失败",
          icon: "none",
        });
      }
    },
  });
};
const deletePropertyFromPreview = () => {
  if (!item.value) return;
  const isLiveListing = ["published", "available", "rented"].includes(item.value.status);
  uni.showModal({
    title: isLiveListing ? "确认删除房源" : "删除房源",
    content: isLiveListing
      ? "该房源曾对外展示。删除会移除图片、视频、收藏和浏览记录；已提交的预约会保留，但不再关联此房源。"
      : "房源及其图片、视频将永久删除，无法恢复。",
    confirmText: "删除",
    confirmColor: "#c45843",
    success: async (result) => {
      if (!result.confirm || !item.value) return;
      try {
        await deleteProperty(item.value.id);
        uni.showToast({ title: "房源已删除" });
        setTimeout(() => uni.navigateBack(), 450);
      } catch (error) {
        uni.showToast({ title: error instanceof Error ? error.message : "删除失败", icon: "none" });
      }
    },
  });
};
const phoneCustomerService = () =>
  callPhone(contactPhone.value, "客服电话尚未配置");
</script>
<template>
  <view v-if="loading" class="page detail-skeleton"
    ><view class="skeleton-media"></view><view class="skeleton-card"><view></view><view></view><view></view></view><view class="skeleton-card short"><view></view><view></view></view></view
  ><view v-else-if="loadFailed" class="page state"
    >房源加载失败，请返回重试</view
  ><view v-else-if="item" class="page detail-page" :style="pageStyle">
    <canvas canvas-id="share-card-canvas" class="share-card-canvas" :width="600" :height="480"></canvas>
    <view class="detail-page-head"><view>{{ presentation.title }}</view><text>{{ presentation.subtitle }}</text></view
    ><view v-if="!mediaItems.length" class="detail-cover detail-cover-empty"
      ><text class="detail-cover-icon">⌂</text><text>暂无实拍图</text><small>联系小皮了解房源详情</small></view
    ><view v-else class="media-gallery" :class="{ 'portrait-video': activeVideoIsPortrait }"
      ><swiper class="media-swiper" :style="{ height: `${mediaViewportHeight}rpx` }" circular :current="activeMediaIndex" @change="handleMediaChange">
        <swiper-item v-for="(media, index) in mediaItems" :key="`${media.url}-${index}`">
          <image
            v-if="media.type === 'image' && shouldLoadMedia(index)"
            class="media-image"
            :src="mediaUrl(media.url)"
            lazy-load
            mode="aspectFill"
            @click="previewImage(media.url)" />
          <view v-else-if="media.type === 'video' && shouldLoadMedia(index)" class="video-stage">
            <video
              class="media-video"
              :src="mediaUrl(media.url)"
              :poster="videoPoster"
              controls
              show-play-btn
              show-progress
              show-fullscreen-btn
              enable-progress-gesture
              preload="metadata"
              object-fit="contain"
              @loadedmetadata="readVideoMetadata(index, $event)"
              @play="playingVideoIndex = index"
              @pause="playingVideoIndex = -1"
              @ended="playingVideoIndex = -1"
            ></video>
          </view>
          <view v-else class="media-loading">加载媒体中</view>
        </swiper-item>
      </swiper
      ><view v-if="mediaItems[activeMediaIndex]?.type !== 'video'" class="media-counter">{{ activeMediaIndex + 1 }} / {{ mediaItems.length }}</view></view
    ><scroll-view v-if="mediaItems.length > 1" class="media-strip" scroll-x show-scrollbar="false"
      ><view class="media-strip-row"><view
        v-for="(media, index) in mediaItems"
        :key="`thumb-${media.url}-${index}`"
        class="media-thumb"
        :class="{ active: activeMediaIndex === index, video: media.type === 'video' }"
        @click="selectMedia(index)"
        ><image v-if="media.type === 'image'" :src="mediaUrl(media.url)" mode="aspectFill" /><text v-else>视频</text><small>{{ mediaLabel(media, index) }}</small></view></view
      ></scroll-view
    ><view v-if="detailModuleEnabled('overview')" class="card overview"
      ><view v-if="isAdminPreview" class="admin-preview-tag">{{ adminPreviewLabel }}</view
      ><view class="overview-top"
        ><view class="price" :class="{ pending: priceText === '待补充' }">{{ priceText }} <text class="price-unit">{{ priceUnit }}</text></view
        ><view v-if="!isAdmin" class="heart" :class="{ active: favorite }" @click="toggle"
          >{{ favorite ? "♥" : "♡"
          }}<small>{{ favorite ? "已关注" : "关注" }}</small></view
        ></view
      ><view class="title small detail-overview-heading">{{ detailModuleTitle('overview', '房屋概况') }}</view
      ><view class="title">{{ item.title || item.community }}</view
      ><view v-if="item.listingSource === '转租'" class="transfer-tag">转租 · 剩余 {{ item.remainingLeaseMonths || 0 }} 个月</view
      ><view class="muted">{{ item.location || "位置待补充" }}</view
      ><view v-if="detailTags.length" class="tags detail-tags"><text v-for="tag in detailTags" :key="tag">{{ tag }}</text></view
      ><view class="key-facts"><view><small>面积</small><strong>{{ item.areaSqm ? `${item.areaSqm}㎡` : "待补充" }}</strong></view
        ><view><small>户型</small><strong>{{ item.layout || "待补充" }}</strong></view
        ><view><small>楼层</small><strong>{{ item.floorInfo || "待补充" }}</strong></view
        ><view><small>朝向</small><strong>{{ item.orientation || "待补充" }}</strong></view
        ></view
      ></view
    ><button v-if="!isAdmin" class="card service-card" open-type="contact" :show-message-card="true" :send-message-title="shareTitle" :send-message-path="propertyMessagePath" :send-message-img="shareImageUrl" :session-from="customerSessionFrom"><view class="service-avatar">皮</view><view class="service-copy"><strong>小皮租房服务</strong><text>咨询费用、入住时间和看房安排</text></view><view class="service-status">在线咨询 <text>›</text></view></button
    ><view v-if="detailModuleEnabled('overview') && (item.depositPayment || item.utilitiesText || item.floorInfo || item.orientation || item.elevator || item.metroMinutes || item.rulesText || item.verificationDate || item.listingSource === '转租')" class="card detail-info"
      ><view class="title small">入住与房屋信息</view
      ><view class="detail-info-grid"
        ><view v-if="item.depositPayment"><small>押付方式</small><strong>{{ item.depositPayment }}</strong></view
        ><view v-if="item.listingSource === '转租'"><small>剩余租期</small><strong>{{ item.remainingLeaseMonths || 0 }} 个月</strong></view
        ><view v-if="item.listingSource === '转租'"><small>承接押金</small><strong>{{ item.transferDeposit ? `¥${item.transferDeposit}` : '无需承接' }}</strong></view
        ><view v-if="item.utilitiesText"><small>水电杂费</small><strong>{{ item.utilitiesText }}</strong></view
        ><view v-if="item.availableDate"><small>可入住</small><strong>{{ item.availableDate }}</strong></view
        ><view v-if="item.metroMinutes"><small>步行至地铁</small><strong>{{ item.metroMinutes }} 分钟</strong></view
        ><view v-if="item.rulesText" class="wide"><small>入住规则</small><strong>{{ item.rulesText }}</strong></view
        ><view v-if="item.verificationDate" class="wide"><small>最近核验</small><strong>{{ item.verificationDate }}</strong></view></view
      ></view
    ><view v-if="detailModuleEnabled('location') && hasLocation" class="card location-card"
      ><view class="location-heading"
        ><view
          ><strong>{{ detailModuleTitle('location', '位置与周边') }}</strong><small>{{ item.location }}</small></view></view
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
    ><view v-if="detailModuleEnabled('facilities')" class="card amenity-card"
      ><view class="detail-section-heading"><view><view class="title small">{{ detailModuleTitle('facilities', '房屋设施与环境') }}</view><small>家具家电、楼内条件与入住信息</small></view><text>{{ amenities.length + environmentHighlights.length }} 项</text></view
      ><template v-if="amenityGroups.length"><view v-for="group in amenityGroups" :key="group.title" class="facility-group"
        ><view class="facility-group-label">{{ group.title }}</view
        ><view class="amenity-grid"
          ><view v-for="amenity in group.items" :key="amenity.name" class="amenity-item"
          ><view class="amenity-icon"><image :src="amenity.icon" mode="aspectFit" /></view
          ><text class="amenity-name">{{ amenity.name }}</text></view
        ></view></view
      ></template><view v-else class="amenity-empty">家具家电信息待补充，可联系小皮确认</view
      ><view v-if="environmentHighlights.length" class="facility-group-label environment-label">居住条件</view
      ><view v-if="environmentHighlights.length" class="environment-list"><view v-for="feature in environmentHighlights" :key="`${feature.label}-${feature.value}`"><small>{{ feature.label }}</small><strong>{{ feature.value }}</strong></view></view
      ></view
    ><view v-if="detailModuleEnabled('description')" class="card"
      ><view class="title small">{{ detailModuleTitle('description', '房源描述') }}</view
      ><view class="desc">{{ propertyDescription || "暂无描述" }}</view></view
    ><view v-if="detailModuleEnabled('nearby')" class="card nearby-card"
      ><view class="nearby-heading"
        ><view><view class="title small">{{ detailModuleTitle('nearby', '附近房源') }}</view><small>距此房源 {{ detailConfig.nearby.radiusKm || relatedRadiusKm }} 公里内</small></view
        ><text>{{ nearbyProperties.length ? `${nearbyProperties.length} 套` : "继续找房" }}</text></view
      ><view
        v-for="property in nearbyProperties"
        :key="property.id"
        class="nearby-item"
        @click="openProperty(property.id)"
        ><view class="nearby-cover"><image v-if="propertyCover(property)" :src="propertyCover(property)" mode="aspectFill" /><text v-else>小皮</text></view
        ><view class="nearby-content"><strong>{{ property.title || property.community }}</strong><small>{{ property.layout }} · {{ property.areaLabel || property.community }}</small><view class="nearby-footer"><view class="nearby-price"><strong>¥{{ property.rentMonthly }}<small>/月</small></strong></view><text>{{ property.distance.toFixed(1) }}km</text></view></view
      ></view
      ><view v-if="!nearbyProperties.length" class="nearby-empty"><strong>附近暂时没有其他可租房源</strong><text>可以继续查看其他地区的新房源</text><button @click="openAllProperties">查看全部房源</button></view
      ><view v-else class="nearby-more" @click="openAllProperties">查看更多房源 <text>›</text></view></view
    ><view class="bottom-space" /><view v-if="isAdmin" class="admin-detail-bar"
      ><view class="admin-detail-lead" :class="{ 'share-disabled': item.status === 'offline' }"><button v-if="item.status !== 'offline'" class="admin-share-primary" open-type="share"><image src="/static/icons/share-property.svg" mode="aspectFit" /><text>分享房源</text></button><button class="admin-edit-primary" @click="editProperty">编辑房源</button></view
      ><view class="admin-detail-main"><button v-for="action in adminPrimaryActions" :key="action.status" class="admin-detail-primary" :class="action.tone" @click="updateStatusFromPreview(action.status, action.label)">{{ action.label }}</button></view
      ><view class="admin-detail-tools"><button class="admin-detail-tool delete" @click="deletePropertyFromPreview">删除房源</button></view></view
    ><view v-else class="action-bar user-action-bar"
      ><view class="service-persona"><view class="service-persona-avatar">皮</view><view><strong>小皮客服</strong><small>房源咨询</small></view></view
      ><button class="user-share" open-type="share"><image src="/static/icons/share-property.svg" mode="aspectFit" /><text>分享</text></button
      ><button class="user-contact" open-type="contact" :show-message-card="true" :send-message-title="shareTitle" :send-message-path="propertyMessagePath" :send-message-img="shareImageUrl" :session-from="customerSessionFrom"><text>在线咨询</text></button
      ><button class="user-phone" @click="phoneCustomerService"><text>打电话</text></button></view
    ></view
  ><view v-else class="page state">房源不存在或已下架</view>
</template>
<style scoped>
.state {
  text-align: center;
  padding-top: 180rpx;
  color: #7e887d;
}
.detail-page {
  padding-bottom: 150rpx;
}
.media-loading {
  display: flex;
  width: 100%;
  height: 100%;
  align-items: center;
  justify-content: center;
  background: #edf1e9;
  color: #7c8878;
  font-size: 24rpx;
}
.share-button {
  flex: none;
  margin: 0 0 0 12rpx;
  padding: 0 16rpx;
  border: 1rpx solid #dce6d9;
  border-radius: 12rpx;
  background: #f6f9f3;
  color: #52634f;
  font-size: 21rpx;
  line-height: 54rpx;
}
.share-button::after { border: 0; }
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
  color: #29382d;
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
  color: #7e887d;
  font-size: 22rpx;
}
.nearby-heading > text {
  color: #40543f;
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
  color: #29382d;
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
  color: #7e887d;
  font-size: 21rpx;
}
.location-heading > text {
  flex: none;
  color: #40543f;
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
  background: #eef2e7;
  color: #40543f;
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
  color: #40543f;
}
.row {
  display: flex;
  justify-content: space-between;
  padding: 22rpx 0;
  border-bottom: 1rpx solid #eef5f2;
  color: #6e9a93;
}
.row text:last-child {
  color: #29382d;
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
.amenity-empty{padding:22rpx;border-radius:14rpx;background:#f4f3ed;color:#879084;font-size:22rpx;line-height:1.5;text-align:center}.nearby-empty{padding:30rpx 18rpx 10rpx;text-align:center}.nearby-empty strong,.nearby-empty text{display:block}.nearby-empty strong{color:#40543f;font-size:25rpx}.nearby-empty text{margin-top:9rpx;color:#8a9287;font-size:21rpx}.nearby-empty button{width:240rpx;margin:22rpx auto 0;border:0;border-radius:14rpx;background:#edf1e8;color:#40543f;font-size:23rpx;line-height:70rpx}.nearby-empty button::after{border:0}.nearby-more{margin-top:12rpx;padding-top:20rpx;border-top:1rpx solid #eeece7;color:#536a45;font-size:23rpx;text-align:center}.nearby-more text{margin-left:5rpx;font-size:28rpx}
.desc {
  line-height: 1.8;
  color: #64705f;
}
.action-bar {
  position: fixed;
  z-index: 5;
  left: 0;
  right: 0;
  bottom: 0;
  display: grid;
  grid-template-columns: 180rpx 1fr;
  align-items: stretch;
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
  color: #52644c;
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
  min-height: 84rpx;
  border-radius: 999rpx;
  background: #40543f;
  color: #fff;
  font-size: 30rpx;
  font-weight: 700;
}
.reserve.disabled {
  background: #b9cbc7;
}
.user-action-bar{grid-template-columns:180rpx 116rpx 1fr}.admin-action-bar{grid-template-columns:132rpx 112rpx 1fr}.admin-action-bar.has-delete{grid-template-columns:112rpx 96rpx 112rpx 1fr}.reserve-secondary{border:1rpx solid #cbd3c5!important;background:#f2f4ed!important;color:#40543f!important;line-height:84rpx}.contact-primary{gap:12rpx}.contact-primary text{font-size:28rpx}.share-action,.delete-action{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:3rpx;margin:0;padding:0;border:0;background:transparent;color:#596857;line-height:1}.share-action::after{border:0}.share-action text,.delete-action text{font-size:30rpx;line-height:1}.share-action small,.delete-action small{font-size:19rpx;line-height:1.2}.delete-action{color:#b76551}

/* Keep detail information-first while matching the light community palette. */
.detail-page { padding-bottom:238rpx; }.detail-cover,.media-image,.media-video { border-radius:24rpx; background:#dce4d5; }.detail-cover { height:440rpx; }.media-list { gap:14rpx; }.media-image,.media-video { height:440rpx; }.overview { margin-top:18rpx; border:0; border-radius:24rpx; }.overview .title { margin-top:18rpx; font-size:40rpx; }.overview .muted { margin-top:8rpx; font-size:25rpx; }.overview .price { color:#d76440; }.heart { color:#657162; }.heart.active { color:#d76440; }.tags text { border-radius:8rpx; background:#edf0e6; color:#5b6956; }.location-card,.nearby-card { border:0; border-radius:24rpx; }.location-heading strong { color:#29382d; }.location-heading > text,.navigate-button { color:#536a45; }.navigate-button { border-radius:12rpx; background:#e9eee0; }.detail-map { border-radius:16rpx; }.row { border-color:#eeece7; }.row text:first-child { color:#8a9389; }.desc { color:#64705f; }.amenity-icon { background:#edf0e6; }.nearby-heading > text,.nearby-price strong { color:#d76440; }.nearby-item { border-color:#eeece7; }.action-bar { background:rgba(255,255,255,.97); box-shadow:0 -6rpx 24rpx rgba(45,55,43,.07); }.contact-action { color:#5a6657; }.reserve { border-radius:16rpx; background:#40543f; }.reserve.disabled { background:#b5bdb0; }
.detail-page .page-presentation-head{margin-bottom:28rpx}.detail-page .overview{margin-top:22rpx}.detail-page .card{margin-bottom:24rpx}.overview .title{line-height:1.35}.amenity-grid{row-gap:14rpx}
.overview .price{font-size:50rpx;line-height:1.1}.overview .price small{font-size:23rpx}.overview .tags{margin-top:16rpx}.detail-page .title.small{font-size:33rpx;line-height:1.3}.row{min-height:82rpx;padding:24rpx 0;align-items:center}.row text:last-child{max-width:70%;line-height:1.45;text-align:right}.desc{margin-top:14rpx;font-size:26rpx;line-height:1.72}.location-heading small{margin-top:8rpx;line-height:1.45}.nearby-item{padding:22rpx 0}.nearby-item small{margin-top:7rpx;line-height:1.4}.action-bar{padding-top:18rpx;padding-bottom:calc(18rpx + env(safe-area-inset-bottom))}
.media-gallery{position:relative;overflow:hidden;border-radius:24rpx;background:#dce4d5}.media-swiper{width:100%;height:440rpx}.media-gallery .media-image,.media-gallery .media-video,.video-stage{display:block;width:100%;height:100%;border-radius:0}.video-stage{position:relative;background:#40543f}.play-video{position:absolute;top:50%;left:50%;display:flex;align-items:center;gap:12rpx;padding:18rpx 26rpx;border-radius:999rpx;transform:translate(-50%,-50%);background:rgba(25,37,27,.76);color:#fff}.play-video text{font-size:28rpx;line-height:1}.play-video small{font-size:24rpx;white-space:nowrap}.media-counter,.media-kind{position:absolute;bottom:22rpx;padding:9rpx 16rpx;border-radius:999rpx;background:rgba(24,34,25,.64);color:#fff;font-size:22rpx;font-variant-numeric:tabular-nums}.media-counter{right:22rpx}.media-kind{left:22rpx}.detail-cover-empty{flex-direction:column;gap:10rpx;background:linear-gradient(145deg,#e8e2d5,#d7dfcd);color:#465440;font-size:31rpx;font-weight:650}.detail-cover-empty .detail-cover-icon{font-size:116rpx;line-height:1}.detail-cover-empty small{color:#788071;font-size:23rpx;font-weight:400}.overview{padding:30rpx 28rpx}.overview-top .price{flex:1;min-width:0;white-space:nowrap}.heart{flex:none}.overview .title{margin-top:16rpx;color:#263227;font-size:40rpx;font-weight:750;letter-spacing:-.6rpx}.overview .muted{margin-top:9rpx;color:#6d766b;font-size:26rpx}.key-facts{width:100%;margin-top:24rpx;white-space:normal}.key-facts-row{display:inline-flex;gap:12rpx;white-space:nowrap}.key-facts-row>view{box-sizing:border-box;width:224rpx;padding:16rpx 14rpx;border-radius:14rpx;background:#eeece5}.key-facts strong{display:block;overflow:hidden;color:#384438;font-size:24rpx;font-weight:650;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.key-facts small{display:block;margin-top:7rpx;color:#8a9087;font-size:19rpx}.map-summary{display:flex;align-items:center;justify-content:space-between;padding:20rpx 0 2rpx}.map-summary strong,.map-summary small{display:block}.map-summary strong{color:#425240;font-size:27rpx}.map-summary small{margin-top:6rpx;color:#879087;font-size:21rpx}.map-summary>text{color:#596b52;font-size:38rpx;line-height:1}.play-video:active,.map-summary:active{opacity:.8}.heart:active,.reserve:active,.contact-action:active{transform:scale(.98);opacity:.86}
.detail-skeleton { padding: 28rpx 24rpx; }.skeleton-media,.skeleton-card { border-radius: 24rpx; background: linear-gradient(100deg,#ecebe4 25%,#f6f4ee 42%,#ecebe4 58%); background-size: 200% 100%; animation: skeleton-shift 1.2s linear infinite; }.skeleton-media { height: 440rpx; }.skeleton-card { margin-top: 24rpx; padding: 28rpx; }.skeleton-card > view { width: 72%; height: 30rpx; margin-bottom: 18rpx; border-radius: 12rpx; background: rgba(255,255,255,.62); }.skeleton-card > view:nth-child(2) { width: 46%; }.skeleton-card > view:nth-child(3) { width: 88%; margin-bottom: 0; }.skeleton-card.short > view { width: 56%; }.skeleton-card.short > view:nth-child(2) { width: 78%; margin-bottom: 0; } @keyframes skeleton-shift { to { background-position: -200% 0; } }
.detail-kicker { margin: 6rpx 4rpx 18rpx; color: #7b8579; font-size: 23rpx; }.media-strip { margin: 14rpx 0 4rpx; white-space: nowrap; }.media-strip-row { display: inline-flex; gap: 12rpx; padding: 0 2rpx; }.media-thumb { width: 104rpx; height: 76rpx; overflow: hidden; border: 2rpx solid transparent; border-radius: 12rpx; background: #e8ebe3; box-sizing: border-box; }.media-thumb image { width: 100%; height: 100%; }.media-thumb.video { display: flex; align-items: center; justify-content: center; background: #40543f; color: #fff; font-size: 20rpx; }.media-thumb.active { border-color: #536a45; box-shadow: 0 0 0 2rpx #dce7d5; }.admin-preview-tag { display: inline-block; margin-bottom: 14rpx; padding: 7rpx 12rpx; border-radius: 8rpx; background: #f8eadf; color: #b55b37; font-size: 20rpx; font-weight: 650; }.overview .price.pending { color: #73826d; font-size: 39rpx; }.overview .price.pending small { color: #8d9789; }.key-facts,.detail-info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12rpx; }.key-facts > view,.detail-info-grid>view { min-width: 0; padding: 16rpx; border-radius: 14rpx; background: #eeece5; }.key-facts > .wide,.detail-info-grid>.wide { grid-column: span 2; }.key-facts strong,.detail-info-grid strong { white-space: normal; text-overflow: clip; }.key-facts small,.detail-info-grid small { margin-top: 0; }.detail-info .title{margin-bottom:20rpx}.detail-info-grid small{display:block;color:#8a9087;font-size:19rpx}.detail-info-grid strong{display:block;margin-top:7rpx;color:#384438;font-size:24rpx;line-height:1.4}.location-heading { margin-bottom: 8rpx; }
.overview .price .price-unit{font-size:23rpx}.overview .price.pending .price-unit{color:#8d9789}
.detail-page-head{margin:8rpx 4rpx 18rpx}.detail-page-head view{color:#29382d;font-size:40rpx;font-weight:800;line-height:1.25;letter-spacing:-1rpx}.detail-page-head text{display:block;margin-top:8rpx;color:#7b8579;font-size:23rpx;line-height:1.45}
.media-thumb{position:relative}.media-thumb small{position:absolute;right:5rpx;bottom:4rpx;max-width:90%;overflow:hidden;padding:3rpx 5rpx;border-radius:6rpx;background:rgba(25,37,27,.64);color:#fff;font-size:16rpx;line-height:1;text-overflow:ellipsis;white-space:nowrap}.service-card{box-sizing:border-box;display:flex;width:100%;align-items:center;gap:16rpx;margin-right:0;margin-left:0;padding:22rpx 24rpx;border:1rpx solid #e6eadb;background:#fbfcf7;text-align:left;line-height:1.3}.service-card::after{border:0}.service-card:active{background:#f1f4ed;transform:scale(.99)}.service-avatar{display:grid;width:62rpx;height:62rpx;flex:none;place-items:center;border-radius:50%;background:#40543f;color:#fff;font-size:30rpx;font-weight:800}.service-copy{min-width:0;flex:1}.service-copy strong,.service-copy text{display:block}.service-copy strong{color:#33452f;font-size:26rpx}.service-copy text{overflow:hidden;margin-top:5rpx;color:#7b8579;font-size:20rpx;text-overflow:ellipsis;white-space:nowrap}.service-status{display:flex;flex:none;align-items:center;gap:5rpx;color:#5d744d;font-size:20rpx;font-weight:700}.service-status text{font-size:28rpx;line-height:1}
/* Admin detail actions: lifecycle decisions are prominent; tools stay quiet and separate. */
.admin-detail-bar { position:fixed; z-index:6; right:0; bottom:0; left:0; padding:16rpx 28rpx calc(16rpx + env(safe-area-inset-bottom)); border-top:1rpx solid #e7e9e2; background:rgba(255,255,255,.98); box-shadow:0 -10rpx 28rpx rgba(45,55,43,.08); }
.admin-detail-lead { display:grid; grid-template-columns:minmax(0,1fr) 180rpx; gap:12rpx; }
.admin-detail-lead.share-disabled { grid-template-columns:1fr; }
.admin-share-primary,.admin-edit-primary { box-sizing:border-box; display:flex; min-width:0; min-height:82rpx; align-items:center; justify-content:center; gap:10rpx; margin:0; padding:0 18rpx; border-radius:15rpx; font-size:26rpx; font-weight:750; line-height:1; }
.admin-share-primary { border:1rpx solid #40543f; background:#40543f; color:#fff; }
.admin-share-primary image { width:29rpx; height:29rpx; filter:brightness(0) invert(1); }
.admin-edit-primary { border:1rpx solid #d9e0d4; background:#f1f4ed; color:#4e6048; }
.admin-share-primary::after,.admin-edit-primary::after { border:0; }
.admin-detail-main { display:grid; grid-auto-flow:column; grid-auto-columns:minmax(0,1fr); gap:10rpx; margin-top:10rpx; }
.admin-detail-primary { min-width:0; margin:0; padding:0 16rpx; border:1rpx solid transparent; border-radius:15rpx; font-size:26rpx; font-weight:750; line-height:82rpx; }
.admin-detail-primary::after,.admin-detail-tool::after { border:0; }
.admin-detail-primary.primary { border-color:#40543f; background:#40543f; color:#fff; }
.admin-detail-primary.secondary { border-color:#dce3d7; background:#f2f5ef; color:#52624b; }
.admin-detail-tools { display:flex; align-items:center; justify-content:center; gap:0; margin-top:10rpx; }
.admin-detail-tool { position:relative; min-width:0; margin:0; padding:4rpx 22rpx; border:0; border-radius:0; background:transparent; color:#687464; font-size:20rpx; line-height:1.3; }
.admin-detail-tool + .admin-detail-tool::before { position:absolute; top:8rpx; bottom:8rpx; left:0; width:1rpx; background:#e0e4dc; content:""; }
.admin-detail-tool.delete { color:#b96d57; }
.admin-share-primary:active,.admin-edit-primary:active,.admin-detail-primary:active,.admin-detail-tool:active { opacity:.78; transform:scale(.985); }

/* Tenant detail actions: identify the service first, then expose the three direct actions. */
.user-action-bar { display:grid; grid-template-columns:minmax(168rpx,1.2fr) 88rpx minmax(142rpx,1fr) minmax(132rpx,.92fr); gap:10rpx; align-items:center; padding:14rpx 22rpx calc(14rpx + env(safe-area-inset-bottom)); border-top:1rpx solid #e7e9e2; background:rgba(255,255,255,.98); box-shadow:0 -10rpx 28rpx rgba(45,55,43,.08); }
.service-persona{display:flex;min-width:0;align-items:center;gap:11rpx}.service-persona-avatar{display:grid;width:62rpx;height:62rpx;flex:none;place-items:center;border-radius:18rpx;background:#40543f;color:#fff;font-size:28rpx;font-weight:800}.service-persona>view:last-child{min-width:0}.service-persona strong,.service-persona small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.service-persona strong{color:#29382d;font-size:23rpx;line-height:1.25}.service-persona small{margin-top:5rpx;color:#8a9287;font-size:18rpx;line-height:1.2}
.user-share,.user-contact,.user-phone { box-sizing:border-box; display:flex; min-width:0; min-height:76rpx; align-items:center; justify-content:center; margin:0; padding:0; border:0; border-radius:17rpx; line-height:1; }
.user-share::after,.user-contact::after,.user-phone::after { border:0; }
.user-share{flex-direction:column;gap:4rpx;background:transparent;color:#40543f}.user-share image{width:31rpx;height:31rpx}.user-share text{font-size:19rpx;font-weight:700;white-space:nowrap}
.user-contact{background:#40543f;color:#fff;font-weight:750}.user-phone{background:#748c69;color:#fff;font-weight:750}.user-contact text,.user-phone text{font-size:23rpx;white-space:nowrap}
.share-card-canvas{position:fixed;left:-2000px;top:-2000px;width:600px;height:480px;opacity:0;pointer-events:none}
.user-share:active,.user-contact:active,.user-phone:active { opacity:.8; transform:scale(.985); }
.transfer-tag { display:inline-block; margin-top:12rpx; padding:7rpx 12rpx; border-radius:8rpx; background:#edf0dc; color:#536a45; font-size:20rpx; font-weight:700; }
/* Final palette and rhythm pass: remove legacy teal without changing detail flow. */
.state{color:#7e887d}.detail-cover,.media-image,.media-video{background:#e4e7de}.location-heading strong,.nearby-item strong{color:#29382d}.location-heading small,.nearby-heading small,.nearby-item small{color:#7e887d}.nearby-heading>text,.nearby-price strong{color:#d76440}.navigate-button{background:#edf1e8;color:#536a45}.tags{gap:10rpx}.tags text{padding:8rpx 13rpx;background:#edf0dc;color:#536a45;font-size:21rpx}.row{color:#7e887d}.row text:last-child{color:#29382d}.amenity-icon{border-color:#dfe7d8;background:#edf1e8;box-shadow:none}.amenity-name{color:#40543f}.desc{color:#64705f}.heart{color:#657162}.heart.active{color:#d76440}
/* Delivery-level detail hierarchy. */
.detail-page{padding:24rpx 24rpx 250rpx;background:#f6f4ee}.detail-page-head{margin:4rpx 6rpx 22rpx}.detail-page-head view{font-size:38rpx;letter-spacing:-1rpx}.detail-page-head text{margin-top:7rpx;font-size:22rpx}.media-gallery{border-radius:22rpx;box-shadow:0 10rpx 28rpx rgba(52,62,48,.08)}.media-swiper{height:460rpx}.media-strip{margin:16rpx 2rpx 6rpx}.detail-page .card{margin-bottom:20rpx;padding:26rpx;border:1rpx solid #e9e6df;border-radius:22rpx;background:#fffdf9}.overview{margin-top:20rpx}.overview .price{font-size:48rpx;letter-spacing:-1rpx}.overview .price .price-unit{margin-left:2rpx;font-size:22rpx}.overview .title{margin-top:18rpx;font-size:37rpx;line-height:1.34;letter-spacing:-.8rpx}.overview .muted{margin-top:10rpx;color:#7a8378;font-size:23rpx;line-height:1.5}.key-facts{gap:10rpx;margin-top:22rpx}.key-facts>view,.detail-info-grid>view{padding:16rpx 15rpx;border-radius:13rpx;background:#f0eee8}.key-facts small,.detail-info-grid small{font-size:19rpx}.key-facts strong,.detail-info-grid strong{margin-top:6rpx;font-size:23rpx}.detail-page .title.small{margin:0;font-size:31rpx;letter-spacing:-.4rpx}.detail-info .title{margin-bottom:18rpx}.detail-section-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:20rpx;margin-bottom:22rpx}.detail-section-heading small{display:block;margin-top:6rpx;color:#8a9287;font-size:20rpx;line-height:1.4}.detail-section-heading>text{flex:none;padding:6rpx 10rpx;border-radius:8rpx;background:#edf0dc;color:#536a45;font-size:19rpx;font-weight:700}.amenity-grid{grid-template-columns:repeat(4,minmax(0,1fr));gap:20rpx 12rpx}.amenity-icon{width:64rpx;height:64rpx;border-radius:18rpx}.amenity-icon image{width:38rpx;height:38rpx}.amenity-name{font-size:21rpx}.environment-list{display:grid;grid-template-columns:1fr 1fr;gap:10rpx;margin-top:22rpx}.environment-list>view{min-width:0;padding:15rpx 16rpx;border-radius:13rpx;background:#f0f2e9}.environment-list small,.environment-list strong{display:block}.environment-list small{color:#8a9287;font-size:18rpx}.environment-list strong{overflow:hidden;margin-top:5rpx;color:#40543f;font-size:22rpx;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.location-heading strong{font-size:30rpx}.location-heading small{font-size:22rpx}.map-summary{padding-top:18rpx}.desc{margin-top:16rpx;font-size:25rpx;line-height:1.75}.service-card{padding:20rpx 22rpx!important}.nearby-item strong{font-size:25rpx}
.facility-group-label{margin:0 0 16rpx;color:#667362;font-size:21rpx;font-weight:700;letter-spacing:.5rpx}.facility-group-label.environment-label{margin-top:24rpx}.amenity-grid{padding-bottom:2rpx}.environment-list{margin-top:0}.nearby-card{padding:26rpx!important}.nearby-heading{margin-bottom:18rpx}.nearby-item{display:grid;grid-template-columns:170rpx minmax(0,1fr);gap:18rpx;min-height:0;padding:18rpx 0;border-top:1rpx solid #eeece7}.nearby-cover{display:grid;width:170rpx;height:126rpx;overflow:hidden;place-items:center;border-radius:15rpx;background:#e9ede3;color:#687563;font-size:23rpx;font-weight:700}.nearby-cover image{display:block;width:100%;height:100%}.nearby-content{display:flex;min-width:0;flex-direction:column}.nearby-content>strong{display:-webkit-box;overflow:hidden;color:#29382d;font-size:25rpx;line-height:1.35;-webkit-box-orient:vertical;-webkit-line-clamp:2}.nearby-content>small{overflow:hidden;margin-top:7rpx;color:#7e887d;font-size:20rpx;line-height:1.3;text-overflow:ellipsis;white-space:nowrap}.nearby-footer{display:flex;align-items:flex-end;justify-content:space-between;margin-top:auto}.nearby-footer>text{color:#879084;font-size:19rpx}.nearby-price{text-align:left}.nearby-price strong{color:#d76440;font-size:28rpx;line-height:1}.nearby-price strong small{display:inline;margin:0 0 0 2rpx;color:#9b8d83;font-size:17rpx;font-weight:500}.nearby-more{border-radius:12rpx;background:#f2f4ed;line-height:58rpx}
</style>
