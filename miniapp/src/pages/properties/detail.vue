<script setup lang="ts">
import { onLoad, onShow, onShareAppMessage } from "@dcloudio/uni-app";
import { computed, nextTick, ref } from "vue";
import { request, resolveAssetUrl } from "../../api/client";
import {
  getPublicConfig,
  deleteProperty,
	bindWechatPhone,
	createRentalRequest,
	getWechatUserProfile,
  listFavorites,
  listProperties,
	recordContactEvent,
  recordFootprint,
  setFavorite,
  type Property,
} from "../../api/properties";
import { getSession, hasRole } from "../../auth/session";
import { amenityIcon } from "../../utils/amenities";
import { callPhone } from "../../utils/phone";
import { usePagePresentation } from "../../utils/presentation";
import { listingDoorplate, listingLayout, listingTagValue, uniqueListingTags } from "../../utils/listing-title";
import { clearPendingAction, peekPendingAction, savePendingAction } from "../../utils/pending-action";
const item = ref<Property | null>(null),
  contactPhone = ref(""),
  brandName = ref("小皮租房"),
  loading = ref(true),
  loadFailed = ref(false),
  favorite = ref(false),
  appointmentOpen = ref(false),
  appointmentSubmitting = ref(false),
  appointmentBinding = ref(false),
  boundPhone = ref(""),
  appointmentName = ref(""),
  appointmentTime = ref("今天方便");
const appointmentTimes = ["今天方便", "明天方便", "工作日晚上", "周末方便", "其他时间"];
const relatedRadiusKm = ref(3);
const defaultDetailFacilities = [
  { key: "refrigerator", label: "冰箱", group: "家具家电", enabled: true }, { key: "washer", label: "洗衣机", group: "家具家电", enabled: true },
  { key: "air_conditioner", label: "空调", group: "家具家电", enabled: true }, { key: "water_heater", label: "热水器", group: "家具家电", enabled: true },
  { key: "television", label: "电视", group: "家具家电", enabled: true }, { key: "broadband", label: "宽带", group: "家具家电", enabled: true },
  { key: "gas", label: "燃气", group: "厨房", enabled: true }, { key: "hood", label: "油烟机", group: "厨房", enabled: true },
  { key: "sofa", label: "沙发", group: "家具家电", enabled: true }, { key: "wardrobe", label: "衣柜", group: "家具家电", enabled: true },
  { key: "desk", label: "书桌", group: "家具家电", enabled: true }, { key: "bed", label: "床", group: "家具家电", enabled: true },
];
const detailConfig = ref({
  modules: [
    { key: "overview", title: "房屋概况", enabled: true }, { key: "location", title: "位置与周边", enabled: true },
    { key: "facilities", title: "房屋设施", enabled: true }, { key: "description", title: "房源简介", enabled: true }, { key: "nearby", title: "附近房源", enabled: true },
  ],
  nearby: { radiusKm: 3, relation: "distance", sort: "distance", limit: 4 },
  facilities: defaultDetailFacilities,
});
// Public detail labels use the same global set as public list cards.  An
// operator sees exactly that content plus the doorplate when it is enabled
// for the operator audience; no other detail-only role divergence is allowed.
const customerDetailTagFields = ref<string[]>(["community", "layout", "rentType", "fee"]);
const operatorDoorplateEnabled = ref(false);
const customerShareTagFields = ref<string[]>(["community", "layout", "rentType", "fee"]);
const customerPosterTagFields = ref<string[]>(["community", "layout", "rentType", "fee"]);
const detailTagOrder = ref<string[]>(["layout", "rentType", "listingSource", "fee", "floor", "area"]);
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
    brandName.value = config?.heroTitle || brandName.value;
    relatedRadiusKm.value = Number(config?.relatedPropertyRadiusKm) || 3;
    detailConfig.value = {
      modules: config?.propertyDetail?.modules?.length ? config.propertyDetail.modules : detailConfig.value.modules,
      nearby: { ...detailConfig.value.nearby, ...(config?.propertyDetail?.nearby || {}) },
      facilities: config?.propertyDetail?.facilities?.length ? config.propertyDetail.facilities : defaultDetailFacilities,
    };
    featuredChannels.value = config?.featuredChannels || [];
    const publicFields = Array.isArray(config?.listingTitle?.enabledFields)
      ? config!.listingTitle!.enabledFields!
      : ["community", "layout", "rentType", "fee"];
    customerDetailTagFields.value = Array.isArray(config?.listingTitle?.customerDetailFields)
      ? config!.listingTitle!.customerDetailFields!
      : publicFields;
    customerShareTagFields.value = Array.isArray(config?.listingTitle?.customerShareFields)
      ? config!.listingTitle!.customerShareFields!
      : customerDetailTagFields.value;
    customerPosterTagFields.value = Array.isArray(config?.listingTitle?.customerPosterFields)
      ? config!.listingTitle!.customerPosterFields!
      : customerShareTagFields.value;
    operatorDoorplateEnabled.value = Boolean(config?.listingTitle?.adminEnabledFields?.includes("unit"));
    if (Array.isArray(config?.listingTitle?.detailTagOrder)) detailTagOrder.value = config!.listingTitle!.detailTagOrder!;
    shareCardConfig.value = {
      titlePrefix: String(config?.shareCard?.titlePrefix || ""),
      titleFields: config?.shareCard?.titleFields?.length ? config.shareCard.titleFields : ["community", "layout"],
      posterFields: config?.shareCard?.posterFields?.length ? config.shareCard.posterFields : ["rentMonthly", "depositPayment", "community", "areaSqm", "floorInfo", "availableDate"],
    };
    if (getSession()?.role === "user") {
      favorite.value = (await listFavorites().catch((): string[] => [])).includes(id);
		const profile = await getWechatUserProfile().catch(() => null);
		boundPhone.value = String(profile?.phone || uni.getStorageSync("xiaopi_bound_phone") || "");
		if (!appointmentName.value) appointmentName.value = String(profile?.nickname || "").trim();
	}
		resumePendingAction();
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
onShow(() => resumePendingAction());
const mediaUrl = resolveAssetUrl;
const propertyCover = (property: Property) => {
  const cover = property.media?.find((media) => media.type === "image");
  return cover ? mediaUrl(cover.url) : "";
};
const mediaItems = computed(() => {
  const media = item.value?.media || [];
  // Keep the entry point consistent with list cards: a property video is the
  // lead medium, followed by the ordered image set.
  return [...media.filter((entry) => entry.type === "video"), ...media.filter((entry) => entry.type !== "video")];
});
const mediaLabel = (media: { type: string }, index: number) =>
  media.type === "video" ? "视频" : `实拍 ${index + 1}`;
const isAdmin = computed(() => hasRole("admin"));
const detailTags = computed(() => {
  const current = item.value;
  if (!current) return [];
  const fields = [...detailTagOrder.value, ...customerDetailTagFields.value].filter((field, index, all) => field !== "community" && field !== "unit" && customerDetailTagFields.value.includes(field) && all.indexOf(field) === index);
  return uniqueListingTags(fields.map((field) => listingTagValue(current, field)));
});
const nearbyTags = (property: Property) => uniqueListingTags([
  property.rentType || "",
  property.listingSource || "",
  listingLayout(property),
  property.areaSqm ? `${property.areaSqm}㎡` : "",
]).slice(0, 3);
const adminDoorplate = computed(() =>
  isAdmin.value && operatorDoorplateEnabled.value && item.value ? listingDoorplate(item.value) : "",
);
const detailModule = (key: string) => detailConfig.value.modules.find((module) => module.key === key);
// The first-screen transaction summary is a structural part of the client
// detail page. Back-office settings can supply content and toggle optional
// modules, but cannot remove or restyle this primary UI.
const detailModuleEnabled = (key: string) => key === "overview" || detailModule(key)?.enabled !== false;
const detailModuleTitle = (key: string, fallback: string) => {
  const title = detailModule(key)?.title;
  if (key === "facilities" && (!title || title === "房屋设施与环境")) return "房屋设施";
  if (key === "description" && (!title || title === "房源描述")) return "房源简介";
  return title || fallback;
};
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
  mediaItems.value[activeMediaIndex.value]?.type === "video" ? 720 : 440,
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
  layout: { label: "户型", value: (property) => listingLayout(property) },
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
const isPublicShareField = (key: string, surface: "share" | "poster" = "share") =>
  !shareFieldTagMap[key] || (surface === "poster" ? customerPosterTagFields.value : customerShareTagFields.value).includes(shareFieldTagMap[key]);
const shareFieldValue = (property: Property, key: string) => shareFieldMeta[key]?.value(property) || "";
const shareTitle = computed(() => {
  const current = item.value;
  if (!current) return `${brandName.value}房源`;
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
    // Friends and Moments deliberately use separate public-label sets.
    const shareKeys = ["rentMonthly", ...customerShareTagFields.value]
      .filter((key, index, values) => key !== "community" && values.indexOf(key) === index)
      .filter((key) => isPublicShareField(key));
    const rentEnabled = shareKeys.includes("rentMonthly");
    context.setFillStyle(rentEnabled ? "#d85f42" : "#26372c");
    context.setFontSize(32);
    context.fillText(compactText(rentEnabled ? shareFieldValue(current, "rentMonthly") : shareTitle.value, 24), 24, 324);
    const secondaryKey = shareKeys.find((key) => key !== "rentMonthly");
    if (rentEnabled && secondaryKey) {
      context.setFillStyle("#7b8579");
      context.setFontSize(18);
      context.fillText(compactText(shareFieldValue(current, secondaryKey), 14), 250, 324);
    }
    context.setFillStyle("#26372c");
    context.setFontSize(27);
    context.fillText(compactText(shareTitle.value, 19), 24, 366);
    const detailKeys = shareKeys.filter((key) => key !== "rentMonthly" && key !== secondaryKey).slice(0, 3);
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
onShareAppMessage(() => friendSharePayload());
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
const requireUser = (pendingAction?: "appointment" | "favorite") => {
  if (getSession()?.role === "user") return true;
  if (getSession()?.role === "admin") {
    uni.showToast({ title: "请切换到普通用户视角操作", icon: "none" });
    return false;
  }
  if (pendingAction && item.value) savePendingAction({ type: pendingAction, propertyId: item.value.id });
  uni.navigateTo({ url: `/pages/auth/login?reason=${pendingAction || "detail"}` });
  return false;
};
const toggle = async () => {
  if (!item.value || !requireUser("favorite")) return;
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
const maskedPhone = computed(() => {
  const phone = boundPhone.value;
  return phone.length >= 7 ? `${phone.slice(0, 3)}****${phone.slice(-4)}` : phone;
});
const openAppointment = async () => {
  if (!item.value || !requireUser("appointment")) return;
  if (!boundPhone.value) {
    const profile = await getWechatUserProfile().catch(() => null);
    boundPhone.value = String(profile?.phone || uni.getStorageSync("xiaopi_bound_phone") || "");
    if (!appointmentName.value) appointmentName.value = String(profile?.nickname || "").trim();
  }
  appointmentOpen.value = true;
};
const resumePendingAction = () => {
  const pending = peekPendingAction();
  if (!pending || getSession()?.role !== "user" || pending.propertyId !== item.value?.id) return;
  clearPendingAction();
  if (pending.type === "appointment") openAppointment();
  if (pending.type === "favorite" && !favorite.value) void toggle();
};
const bindAppointmentPhone = async (event: { detail: { code?: string } }) => {
  const code = event.detail.code;
  if (!code || appointmentBinding.value) return;
  appointmentBinding.value = true;
  try {
    const result = await bindWechatPhone(code);
    boundPhone.value = result.phone;
    uni.setStorageSync("xiaopi_bound_phone", result.phone);
    uni.showToast({ title: "手机号已绑定" });
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "手机号绑定失败", icon: "none" });
  } finally {
    appointmentBinding.value = false;
  }
};
const submitAppointment = async () => {
  if (!item.value || appointmentSubmitting.value) return;
  if (!boundPhone.value) {
    uni.showToast({ title: "请先绑定手机号", icon: "none" });
    return;
  }
  if (!appointmentName.value.trim() || !appointmentTime.value) {
    uni.showToast({ title: "请填写称谓并选择方便时间", icon: "none" });
    return;
  }
  appointmentSubmitting.value = true;
  try {
    await createRentalRequest({
      requestType: "appointment",
      propertyId: item.value.id,
      contactName: appointmentName.value.trim(),
      phone: boundPhone.value,
      preferredTime: appointmentTime.value,
      location: item.value.location || item.value.community || "",
      layout: listingLayout(item.value),
      budgetMin: Number(item.value.rentMonthly) || 0,
      budgetMax: Number(item.value.rentMonthly) || 0,
      moveInDate: "",
      note: "",
    });
    appointmentOpen.value = false;
    uni.showModal({ title: "预约已提交", content: "小皮会根据你选择的时间联系确认看房。", showCancel: false });
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "预约提交失败", icon: "none" });
  } finally {
    appointmentSubmitting.value = false;
  }
};
const recordContact = (type: "wechat_consult" | "phone_call") => {
  if (getSession()?.role !== "user" || !item.value) return;
  void recordContactEvent({ propertyId: item.value.id, type, source: "detail" }).catch(() => undefined);
};
const openProperty = (id: string) =>
  uni.redirectTo({ url: `/pages/properties/detail?id=${id}` });
const openAllProperties = () => uni.switchTab({ url: "/pages/properties/index" });
const goBack = () => {
  if (getCurrentPages().length > 1) {
    uni.navigateBack();
    return;
  }
  uni.switchTab({ url: "/pages/properties/index" });
};
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
const phoneCustomerService = () => {
  recordContact("phone_call");
  callPhone(contactPhone.value, "客服电话尚未配置");
};
</script>
<template>
  <view v-if="loading" class="page detail-skeleton"
    ><view class="skeleton-media"></view><view class="skeleton-card"><view></view><view></view><view></view></view><view class="skeleton-card short"><view></view><view></view></view></view
  ><view v-else-if="loadFailed" class="page state"
    >房源加载失败，请返回重试</view
  ><view v-else-if="item" class="page detail-page" :style="pageStyle">
    <canvas canvas-id="share-card-canvas" class="share-card-canvas" :width="600" :height="480"></canvas>
    <view class="detail-media-back" @click="goBack"><text>‹</text></view>
    <view v-if="!mediaItems.length" class="detail-cover detail-cover-empty"
      ><text class="detail-cover-icon">⌂</text><text>暂无实拍图</text><small>联系小皮了解房源详情</small></view
    ><view v-else class="media-gallery" :class="{ 'portrait-video': activeVideoIsPortrait, 'video-active': mediaItems[activeMediaIndex]?.type === 'video', 'video-playing': playingVideoIndex === activeMediaIndex }"
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
              object-fit="cover"
              @loadedmetadata="readVideoMetadata(index, $event)"
              @play="playingVideoIndex = index"
              @pause="playingVideoIndex = -1"
              @ended="playingVideoIndex = -1"
            ></video>
          </view>
          <view v-else class="media-loading">加载媒体中</view>
        </swiper-item>
      </swiper
      ><view v-if="mediaItems.length === 1 && mediaItems[activeMediaIndex]?.type !== 'video'" class="media-counter">1 / 1</view><scroll-view v-if="mediaItems.length > 1" class="media-strip" scroll-x show-scrollbar="false"
      ><view class="media-strip-row"><view
        v-for="(media, index) in mediaItems"
        :key="`thumb-${media.url}-${index}`"
        class="media-thumb"
        :class="{ active: activeMediaIndex === index, video: media.type === 'video' }"
        @click="selectMedia(index)"
        ><image v-if="media.type === 'image'" :src="mediaUrl(media.url)" mode="aspectFill" /><text v-else>视频</text></view></view
      ></scroll-view
      ></view
    ><view v-if="detailModuleEnabled('overview')" class="card overview"
      ><view v-if="isAdminPreview" class="admin-preview-tag">{{ adminPreviewLabel }}</view
      ><view class="overview-top"
        ><view class="price" :class="{ pending: priceText === '待补充' }">{{ priceText }} <text class="price-unit">{{ priceUnit }}</text></view
        ><view v-if="!isAdmin" class="heart" :class="{ active: favorite }" @click="toggle"
          >{{ favorite ? "♥" : "♡"
          }}<small>{{ favorite ? "已关注" : "关注" }}</small></view
        ></view
      ><view class="title">{{ item.title || item.community }}</view
      ><view v-if="isAdmin" class="muted">{{ item.location || "位置待补充" }}</view
      ><view v-if="adminDoorplate" class="admin-detail-doorplate">门牌定位 · {{ adminDoorplate }}</view
      ><view v-if="item.listingSource === '转租'" class="overview-meta-row"><view class="transfer-tag">转租 · 剩余 {{ item.remainingLeaseMonths || 0 }} 个月</view></view
      ><view v-if="!isAdmin" class="muted">{{ item.location || "位置待补充" }}</view
      ><view class="key-facts"><view><small class="kv-key">{{ item.rentType === '合租' && item.wholeAreaSqm ? '整套面积' : '面积' }}</small><strong class="kv-value">{{ item.rentType === '合租' && item.wholeAreaSqm ? `${item.wholeAreaSqm}㎡` : item.areaSqm ? `${item.areaSqm}㎡` : "待补充" }}</strong></view
        ><view><small class="kv-key">{{ item.rentType === '合租' && item.wholeLayout ? '整套户型' : '户型' }}</small><strong class="kv-value">{{ item.rentType === '合租' && item.wholeLayout ? item.wholeLayout : listingLayout(item) || "待补充" }}</strong></view
        ><view><small class="kv-key">楼层</small><strong class="kv-value">{{ item.floorInfo || "待补充" }}</strong></view
        ><view v-if="item.orientation"><small class="kv-key">朝向</small><strong class="kv-value">{{ item.orientation }}</strong></view
        ><view v-if="item.elevator"><small class="kv-key">电梯</small><strong class="kv-value">{{ item.elevator }}</strong></view
        ><view><small class="kv-key">租赁方式</small><strong class="kv-value">{{ item.rentType || "待补充" }}</strong></view
        ></view
      ></view
    ><button v-if="!isAdmin" class="appointment-entry" @click="openAppointment"><view><strong>预约看房</strong><text>选个方便时间，小皮来确认</text></view><text>›</text></button
    ><view v-if="item.depositPayment || item.feeText || item.utilitiesText || item.listingSource === '转租' || (item.rentType === '合租' && (item.layout || item.areaSqm))" class="card detail-info"
      ><view class="detail-section-heading compact"><view><view class="title small">租赁信息</view><small>费用与租期说明</small></view></view
      ><view class="detail-info-grid"
        ><view v-if="item.depositPayment"><small class="kv-key">押付方式</small><strong class="kv-value">{{ item.depositPayment }}</strong></view
        ><view v-if="item.feeText"><small class="kv-key">中介费</small><strong class="kv-value">{{ item.feeText }}</strong></view
        ><view v-if="item.utilitiesText" class="wide"><small class="kv-key">水电杂费</small><strong class="kv-value">{{ item.utilitiesText }}</strong></view
        ><view v-if="item.rentType === '合租' && item.layout"><small class="kv-key">实际使用空间</small><strong class="kv-value">{{ listingLayout(item) }}</strong></view
        ><view v-if="item.rentType === '合租' && item.areaSqm"><small class="kv-key">实际使用面积</small><strong class="kv-value">{{ item.areaSqm }}㎡</strong></view
        ><view v-if="item.listingSource === '转租'"><small class="kv-key">剩余租期</small><strong class="kv-value">{{ item.remainingLeaseMonths || 0 }} 个月</strong></view
        ><view v-if="item.listingSource === '转租'"><small class="kv-key">承接押金</small><strong class="kv-value">{{ item.transferDeposit ? `¥${item.transferDeposit}` : '无需承接' }}</strong></view></view
      ></view
    ><view v-if="detailModuleEnabled('location') && hasLocation" class="card location-card"
      ><view class="location-heading"
        ><view
          ><strong>{{ detailModuleTitle('location', '位置与周边') }}</strong><small>{{ item.location }}</small></view><text>已定位</text></view
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
      ><view class="detail-section-heading"><view><view class="title small">{{ detailModuleTitle('facilities', '房屋设施') }}</view><small>家具家电等可勾选配置</small></view><text>{{ amenities.length }} 项</text></view
      ><template v-if="amenityGroups.length"><view v-for="group in amenityGroups" :key="group.title" class="facility-group"
        ><view class="facility-group-label">{{ group.title }}</view
        ><view class="amenity-grid"
          ><view v-for="amenity in group.items" :key="amenity.name" class="amenity-item"
          ><view class="amenity-icon"><image :src="amenity.icon" mode="aspectFit" /></view
          ><text class="amenity-name">{{ amenity.name }}</text></view
        ></view></view
      ></template><view v-else class="amenity-empty">家具家电信息待补充，可联系小皮确认</view
      ></view
    ><view v-if="detailModuleEnabled('description')" class="card"
      ><view class="detail-section-heading compact"><view><view class="title small">{{ detailModuleTitle('description', '房源简介') }}</view><small>房东补充说明与居住规则</small></view></view
      ><view class="desc">{{ propertyDescription || "暂无描述" }}</view></view
    ><view v-if="detailModuleEnabled('nearby')" class="card nearby-card"
      ><view class="nearby-heading"
        ><view><view class="title small">{{ detailModuleTitle('nearby', '附近房源') }}</view></view
        ><text>{{ nearbyProperties.length ? `${nearbyProperties.length} 套` : "继续找房" }}</text></view
      ><view
        v-for="property in nearbyProperties"
        :key="property.id"
        class="nearby-item"
        @click="openProperty(property.id)"
        ><view class="nearby-cover"><image v-if="propertyCover(property)" :src="propertyCover(property)" mode="aspectFill" /><text v-else>小皮</text></view
        ><view class="nearby-content"><strong>{{ property.title || property.community }}</strong><view v-if="nearbyTags(property).length" class="nearby-tags"><text v-for="tag in nearbyTags(property)" :key="tag">{{ tag }}</text></view><view class="nearby-footer"><view class="nearby-price"><text>¥{{ property.rentMonthly }}<text class="nearby-price-unit">/月</text></text></view><small class="nearby-distance">距此 {{ property.distance.toFixed(1) }}km</small></view></view
      ></view
      ><view v-if="!nearbyProperties.length" class="nearby-empty"><strong>附近暂时没有其他可租房源</strong><text>可以继续查看其他地区的新房源</text><button @click="openAllProperties">查看全部房源</button></view
      ><view v-else class="nearby-more" @click="openAllProperties">查看更多房源 <text>›</text></view></view
    ><button v-if="!isAdmin" class="floating-share" open-type="share"><image src="/static/icons/share-property.svg" mode="aspectFit" /><text>分享</text></button
    ><view class="bottom-space" /><view v-if="isAdmin" class="admin-detail-bar"
      ><view class="admin-detail-lead" :class="{ 'share-disabled': item.status === 'offline' }"><button v-if="item.status !== 'offline'" class="admin-share-primary" open-type="share"><image src="/static/icons/share-property.svg" mode="aspectFit" /><text>分享好友</text></button><button class="admin-edit-primary" @click="editProperty">编辑房源</button></view
      ><view class="admin-detail-main"><button v-for="action in adminPrimaryActions" :key="action.status" class="admin-detail-primary" :class="action.tone" @click="updateStatusFromPreview(action.status, action.label)">{{ action.label }}</button></view
      ><view class="admin-detail-tools"><button class="admin-detail-tool delete" @click="deletePropertyFromPreview">删除房源</button></view></view
    ><view v-else class="action-bar user-action-bar"
      ><view class="detail-signature"><text class="detail-signature-mark">皮</text><strong>{{ brandName || '小皮租房' }}</strong></view
      ><button class="user-contact" open-type="contact" :show-message-card="true" :send-message-title="shareTitle" :send-message-path="propertyMessagePath" :send-message-img="shareImageUrl" :session-from="customerSessionFrom" @click="recordContact('wechat_consult')"><text>在线咨询</text></button
      ><button class="user-phone" @click="phoneCustomerService"><text>打电话</text></button></view
    ><view v-if="appointmentOpen" class="appointment-mask" @click.self="appointmentOpen = false"><view class="appointment-sheet"><view class="appointment-sheet-head"><view><strong>预约看房</strong><text>{{ item.title || item.community }}</text></view><text @click="appointmentOpen = false">×</text></view><template v-if="boundPhone"><view class="appointment-field"><text>怎么称呼您</text><input v-model.trim="appointmentName" maxlength="20" placeholder="例如：张女士" /></view><view class="appointment-field"><text>联系手机号</text><strong>{{ maskedPhone }}</strong></view><view class="appointment-field"><text>什么时候方便</text><view class="appointment-times"><text v-for="time in appointmentTimes" :key="time" :class="{ active: appointmentTime === time }" @click="appointmentTime = time">{{ time }}</text></view></view><button class="appointment-submit" :loading="appointmentSubmitting" @click="submitAppointment">提交预约</button></template><template v-else><view class="appointment-bind"><strong>先绑定手机号</strong><text>用于确认看房安排，只会展示脱敏号码。</text><button open-type="getPhoneNumber" :loading="appointmentBinding" @getphonenumber="bindAppointmentPhone">绑定手机号并继续</button></view></template></view></view
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
.admin-detail-lead { display:grid; grid-template-columns:1fr 1fr; gap:12rpx; }
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
.user-action-bar { display:grid; grid-template-columns:4fr 3fr 3fr; gap:12rpx; align-items:center; padding:14rpx 18rpx calc(14rpx + env(safe-area-inset-bottom)); border-top:1rpx solid #e7e9e2; background:rgba(255,255,255,.98); box-shadow:0 -10rpx 28rpx rgba(45,55,43,.08); }
.detail-signature{display:flex;min-width:0;align-items:center;gap:12rpx;padding:0 4rpx;color:#344636}.detail-signature-mark{display:grid;width:82rpx;height:82rpx;flex:none;place-items:center;border-radius:50%;background:#40543f;color:#fff;font-size:35rpx;font-weight:800}.detail-signature strong{display:block;overflow:hidden;font-size:23rpx;line-height:1.25;text-overflow:ellipsis;white-space:nowrap}
.user-contact,.user-phone { box-sizing:border-box; display:flex; min-width:0; min-height:82rpx; align-items:center; justify-content:center; margin:0; padding:0; border:0; border-radius:17rpx; line-height:1; }
.user-contact::after,.user-phone::after { border:0; }
.floating-share{position:fixed;z-index:7;right:24rpx;bottom:calc(118rpx + env(safe-area-inset-bottom));display:flex;align-items:center;justify-content:center;gap:7rpx;min-width:116rpx;height:64rpx;margin:0;padding:0 18rpx;border:1rpx solid #d7e1d2;border-radius:36rpx;background:rgba(255,253,249,.96);box-shadow:0 8rpx 20rpx rgba(43,59,45,.14);color:#40543f;line-height:1}.floating-share::after{border:0}.floating-share image{width:28rpx;height:28rpx}.floating-share text{font-size:21rpx;font-weight:700}
.user-contact{background:#40543f;color:#fff;font-weight:750}.user-phone{background:#748c69;color:#fff;font-weight:750}.user-contact text,.user-phone text{font-size:23rpx;white-space:nowrap}
.share-card-canvas{position:fixed;left:-2000px;top:-2000px;width:600px;height:480px;opacity:0;pointer-events:none}
.floating-share:active,.user-contact:active,.user-phone:active { opacity:.8; transform:scale(.985); }
.transfer-tag { display:inline-block; padding:7rpx 12rpx; border-radius:8rpx; background:#edf0dc; color:#536a45; font-size:20rpx; font-weight:700; }
/* Final palette and rhythm pass: remove legacy teal without changing detail flow. */
.state{color:#7e887d}.detail-cover,.media-image,.media-video{background:#e4e7de}.location-heading strong,.nearby-item strong{color:#29382d}.location-heading small,.nearby-heading small,.nearby-item small{color:#7e887d}.nearby-heading>text,.nearby-price strong{color:#d76440}.navigate-button{background:#edf1e8;color:#536a45}.tags{gap:10rpx}.tags text{padding:8rpx 13rpx;background:#edf0dc;color:#536a45;font-size:21rpx}.row{color:#7e887d}.row text:last-child{color:#29382d}.amenity-icon{border-color:#dfe7d8;background:#edf1e8;box-shadow:none}.amenity-name{color:#40543f}.desc{color:#64705f}.heart{color:#657162}.heart.active{color:#d76440}
/* Delivery-level detail hierarchy. */
.detail-page{padding:24rpx 24rpx 250rpx;background:#f6f4ee}.detail-page-head{margin:4rpx 6rpx 22rpx}.detail-page-head view{font-size:38rpx;letter-spacing:-1rpx}.detail-page-head text{margin-top:7rpx;font-size:22rpx}.media-gallery{border-radius:22rpx;box-shadow:0 10rpx 28rpx rgba(52,62,48,.08)}.media-swiper{height:460rpx}.media-strip{margin:16rpx 2rpx 6rpx}.detail-page .card{margin-bottom:20rpx;padding:26rpx;border:1rpx solid #e9e6df;border-radius:22rpx;background:#fffdf9}.overview{margin-top:20rpx}.overview .price{font-size:48rpx;letter-spacing:-1rpx}.overview .price .price-unit{margin-left:2rpx;font-size:22rpx}.overview .title{margin-top:18rpx;font-size:37rpx;line-height:1.34;letter-spacing:-.8rpx}.overview .muted{margin-top:10rpx;color:#7a8378;font-size:23rpx;line-height:1.5}.key-facts{gap:10rpx;margin-top:22rpx}.key-facts>view,.detail-info-grid>view{padding:16rpx 15rpx;border-radius:13rpx;background:#f0eee8}.key-facts small,.detail-info-grid small{font-size:19rpx}.key-facts strong,.detail-info-grid strong{margin-top:6rpx;font-size:23rpx}.detail-page .title.small{margin:0;font-size:31rpx;letter-spacing:-.4rpx}.detail-info .title{margin-bottom:18rpx}.detail-section-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:20rpx;margin-bottom:22rpx}.detail-section-heading small{display:block;margin-top:6rpx;color:#8a9287;font-size:20rpx;line-height:1.4}.detail-section-heading>text{flex:none;padding:6rpx 10rpx;border-radius:8rpx;background:#edf0dc;color:#536a45;font-size:19rpx;font-weight:700}.amenity-grid{grid-template-columns:repeat(4,minmax(0,1fr));gap:20rpx 12rpx}.amenity-icon{width:64rpx;height:64rpx;border-radius:18rpx}.amenity-icon image{width:38rpx;height:38rpx}.amenity-name{font-size:21rpx}.environment-list{display:grid;grid-template-columns:1fr 1fr;gap:10rpx;margin-top:22rpx}.environment-list>view{min-width:0;padding:15rpx 16rpx;border-radius:13rpx;background:#f0f2e9}.environment-list small,.environment-list strong{display:block}.environment-list small{color:#8a9287;font-size:18rpx}.environment-list strong{overflow:hidden;margin-top:5rpx;color:#40543f;font-size:22rpx;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.location-heading strong{font-size:30rpx}.location-heading small{font-size:22rpx}.map-summary{padding-top:18rpx}.desc{margin-top:16rpx;font-size:25rpx;line-height:1.75}.service-card{padding:20rpx 22rpx!important}.nearby-item strong{font-size:25rpx}
.detail-page{padding:0 24rpx 250rpx}.detail-page-head{display:none}.media-gallery{margin:0 -24rpx;border-radius:0;box-shadow:none}.media-strip{position:absolute;z-index:3;right:18rpx;bottom:16rpx;left:18rpx;width:auto;margin:0;white-space:nowrap;transition:opacity .18s ease,transform .18s ease}.media-strip-row{gap:8rpx;padding:0}.media-thumb{width:66rpx;height:48rpx;border:1rpx solid rgba(255,255,255,.72);border-radius:8rpx;box-shadow:0 3rpx 10rpx rgba(0,0,0,.2)}.media-thumb.video{width:auto;min-width:66rpx;padding:0 10rpx;background:rgba(34,50,37,.82);font-size:17rpx}.media-thumb.active{border-color:#fff;box-shadow:0 0 0 2rpx rgba(64,84,63,.72),0 3rpx 10rpx rgba(0,0,0,.2)}.media-thumb small{display:none}.media-counter{bottom:16rpx;right:18rpx;font-size:18rpx}.media-gallery.video-active .media-strip{bottom:104rpx}.media-gallery.video-playing .media-strip{pointer-events:none;opacity:0;transform:translateY(12rpx)}
.detail-page{position:relative}.detail-media-back{position:absolute;z-index:6;top:76rpx;left:22rpx;display:grid;width:58rpx;height:58rpx;place-items:center;border:1rpx solid rgba(255,255,255,.56);border-radius:50%;background:rgba(23,31,24,.38);box-shadow:0 3rpx 12rpx rgba(0,0,0,.16)}.detail-media-back text{color:#fff;font-size:54rpx;font-weight:300;line-height:48rpx;transform:translateY(-3rpx)}.detail-media-back:active{opacity:.75;transform:scale(.96)}
.detail-module-kicker{margin-top:18rpx;color:#80907a;font-size:20rpx;font-weight:700;letter-spacing:1rpx;line-height:1.2}.detail-section-heading.compact{margin-bottom:18rpx}.location-heading>text{flex:none;padding:6rpx 10rpx;border-radius:8rpx;background:#edf0dc;color:#536a45;font-size:19rpx;font-weight:700}
.facility-group-label{margin:0 0 16rpx;color:#667362;font-size:21rpx;font-weight:700;letter-spacing:.5rpx}.facility-group-label.environment-label{margin-top:24rpx}.amenity-grid{padding-bottom:2rpx}.environment-list{margin-top:0}.nearby-card{padding:26rpx!important}.nearby-heading{margin-bottom:18rpx}.nearby-item{display:grid;grid-template-columns:170rpx minmax(0,1fr);gap:18rpx;min-height:0;padding:18rpx 0;border-top:1rpx solid #eeece7}.nearby-cover{display:grid;width:170rpx;height:126rpx;overflow:hidden;place-items:center;border-radius:15rpx;background:#e9ede3;color:#687563;font-size:23rpx;font-weight:700}.nearby-cover image{display:block;width:100%;height:100%}.nearby-content{display:flex;min-width:0;flex-direction:column}.nearby-content>strong{display:-webkit-box;overflow:hidden;color:#29382d;font-size:25rpx;line-height:1.35;-webkit-box-orient:vertical;-webkit-line-clamp:2}.nearby-content>small{overflow:hidden;margin-top:7rpx;color:#7e887d;font-size:20rpx;line-height:1.3;text-overflow:ellipsis;white-space:nowrap}.nearby-footer{display:flex;align-items:flex-end;justify-content:space-between;margin-top:auto}.nearby-footer>text{color:#879084;font-size:19rpx}.nearby-price{text-align:left}.nearby-price strong{color:#d76440;font-size:28rpx;line-height:1}.nearby-price strong small{display:inline;margin:0 0 0 2rpx;color:#9b8d83;font-size:17rpx;font-weight:500}.nearby-more{border-radius:12rpx;background:#f2f4ed;line-height:58rpx}
.nearby-heading{margin-bottom:14rpx}.nearby-item{grid-template-columns:184rpx minmax(0,1fr);gap:14rpx;margin-bottom:12rpx;padding:10rpx;border:1rpx solid #e9e1d7;border-radius:16rpx;background:#fffdf9;box-shadow:0 5rpx 14rpx rgba(72,70,50,.04)}.nearby-cover{position:relative;width:184rpx;height:138rpx;border-radius:11rpx}.nearby-cover>small{position:absolute;top:7rpx;left:7rpx;margin:0;padding:4rpx 7rpx;border-radius:6rpx;background:#168d82;color:#fff;font-size:17rpx;font-weight:750;line-height:1}.nearby-content{min-height:138rpx;padding:2rpx 2rpx 1rpx 0}.nearby-content>strong{font-size:24rpx;line-height:1.32;-webkit-line-clamp:2}.nearby-content>.nearby-area{margin-top:5rpx;color:#778176;font-size:19rpx;line-height:1.25}.nearby-tags{display:flex;gap:5rpx;overflow:hidden;margin-top:7rpx;white-space:nowrap}.nearby-tags text{flex:none;padding:4rpx 6rpx;border-radius:5rpx;background:#edf0dc;color:#536a45;font-size:16rpx;line-height:1.1}.nearby-tags text:nth-child(2n){background:#f8eadf;color:#b7644b}.nearby-price strong{font-size:27rpx}.nearby-more{margin-top:2rpx;line-height:62rpx}
.nearby-item{align-items:stretch}.nearby-cover{align-self:stretch;height:auto;min-height:138rpx}.nearby-cover>small{display:none}.nearby-footer>.nearby-distance{display:block;overflow:hidden;margin:0;color:#879084;font-size:18rpx;line-height:1.25;text-overflow:ellipsis;white-space:nowrap}.nearby-content>.nearby-area{display:none}
.nearby-price>text{display:inline;white-space:nowrap;color:#d76440;font-size:27rpx;font-weight:700;line-height:1}.nearby-price .nearby-price-unit{display:inline;margin-left:2rpx;color:#9b8d83;font-size:17rpx;font-weight:500;white-space:nowrap}
/* Detail facts are data, not cards: use compact two-column key/value pairs. */
.key-facts,.detail-info-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:0 34rpx;margin-top:22rpx}.key-facts>view,.detail-info-grid>view{display:flex;align-items:baseline;gap:16rpx;min-width:0;min-height:56rpx;padding:9rpx 0;border:0;border-radius:0;background:transparent}.key-facts> .wide,.detail-info-grid>.wide{grid-column:span 2}.key-facts small,.detail-info-grid small{flex:none;margin:0;color:#9aa09a;font-size:22rpx;white-space:nowrap}.key-facts strong,.detail-info-grid strong{min-width:0;margin:0;color:#384438;font-size:24rpx;font-weight:650;line-height:1.45;text-align:left;word-break:break-all}.detail-info-grid{margin-top:0}
/* Strengthen key/value contrast so field names recede behind their values. */
.kv-key{color:#939b93!important;font-size:20rpx!important;font-weight:400!important;line-height:1.5!important}.kv-value{color:#334033!important;font-size:24rpx!important;font-weight:400!important;line-height:1.45!important}
.whole-home-context{display:flex;flex-wrap:wrap;align-items:center;gap:8rpx 16rpx;margin-top:15rpx;padding-top:14rpx;border-top:1rpx dashed #e3e5de;color:#677166;font-size:20rpx;line-height:1.5}.whole-home-context small{color:#98a098;font-size:19rpx}.whole-home-context text{color:#677166}
/* Give each module heading and its explanatory copy a deliberate visual pause. */
.detail-section-heading .title.small{margin:0 0 10rpx!important;line-height:1.32}.detail-section-heading>view>small{margin-top:0!important;line-height:1.5}.amenity-card .facility-group:first-child .facility-group-label{margin-top:10rpx}
/* Operator-only doorplate is metadata, not part of the public tag stream. */
.overview-meta-row{display:flex;flex-wrap:wrap;align-items:center;gap:8rpx;margin-top:12rpx}.detail-tag-stack{display:flex;flex-direction:column;align-items:flex-start;gap:8rpx;margin-top:14rpx}.detail-tags-inline{margin-top:0}.detail-tags{display:flex;flex-wrap:wrap;gap:10rpx;margin:0}.detail-tags text{flex:none;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.admin-detail-doorplate{display:block;box-sizing:border-box;width:max-content;max-width:100%;overflow:hidden;margin-top:13rpx;padding:7rpx 11rpx;border:1rpx solid #d8e2d2;border-radius:9rpx;background:#f3f7ee;color:#40543f;font-size:21rpx;font-weight:700;line-height:1.25;text-overflow:ellipsis;white-space:nowrap}
.detail-tags-classic .detail-tags,.detail-tags-grouped{padding:0;border-radius:0;background:transparent;gap:8rpx}.detail-tags-classic .detail-tags text,.detail-tags-grouped text{background:#fff!important;border:1rpx solid #dce4d5}
.appointment-entry{box-sizing:border-box;display:flex;width:calc(100% - 48rpx);align-items:center;justify-content:space-between;margin:-2rpx 24rpx 24rpx;padding:22rpx 26rpx;border:1rpx solid #dce7d4;border-radius:20rpx;background:#eef4e9;color:#40543f;text-align:left}.appointment-entry::after{border:0}.appointment-entry view{display:flex;min-width:0;flex-direction:column;gap:6rpx}.appointment-entry strong{font-size:28rpx;line-height:1.2}.appointment-entry view text{color:#7b8974;font-size:21rpx;line-height:1.35}.appointment-entry>text{font-size:38rpx;line-height:1}.appointment-mask{position:fixed;z-index:30;inset:0;display:flex;align-items:flex-end;background:rgba(20,30,22,.46)}.appointment-sheet{box-sizing:border-box;width:100%;padding:30rpx 30rpx calc(30rpx + env(safe-area-inset-bottom));border-radius:30rpx 30rpx 0 0;background:#fffdf9}.appointment-sheet-head{display:flex;align-items:flex-start;justify-content:space-between;margin-bottom:26rpx}.appointment-sheet-head view{display:flex;min-width:0;flex-direction:column;gap:8rpx}.appointment-sheet-head strong{color:#2c3b2e;font-size:36rpx;line-height:1.2}.appointment-sheet-head view text{overflow:hidden;color:#7a8378;font-size:23rpx;line-height:1.4;text-overflow:ellipsis;white-space:nowrap}.appointment-sheet-head>text{width:54rpx;height:54rpx;border-radius:50%;background:#f0f1eb;color:#667260;font-size:42rpx;line-height:49rpx;text-align:center}.appointment-field{padding:22rpx 0;border-top:1rpx solid #ecebe5}.appointment-field>text{display:block;margin-bottom:15rpx;color:#697568;font-size:22rpx}.appointment-field input{height:78rpx;padding:0 20rpx;border-radius:14rpx;background:#f4f5ef;color:#2f3d30;font-size:28rpx}.appointment-field strong{color:#334032;font-size:30rpx;font-weight:600}.appointment-times{display:flex;flex-wrap:wrap;gap:12rpx}.appointment-times text{padding:13rpx 17rpx;border:1rpx solid #dfe5d9;border-radius:12rpx;background:#fff;color:#667260;font-size:22rpx;line-height:1}.appointment-times text.active{border-color:#40543f;background:#40543f;color:#fff}.appointment-submit,.appointment-bind button{margin-top:26rpx;border-radius:16rpx;background:#40543f;color:#fff;font-size:28rpx;font-weight:750}.appointment-submit::after,.appointment-bind button::after{border:0}.appointment-bind{display:flex;flex-direction:column;gap:13rpx;padding:16rpx 0}.appointment-bind strong{color:#334032;font-size:30rpx}.appointment-bind text{color:#7b8579;font-size:23rpx;line-height:1.55}.appointment-bind button{width:100%}
</style>
