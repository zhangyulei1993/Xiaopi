<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { request, resolveAssetUrl } from "../../api/client";
import {
  getPublicConfig,
  createProperty,
  deleteProperty,
  listFavorites,
  listProperties,
  setFavorite,
  type Property,
} from "../../api/properties";
import { getSession } from "../../auth/session";
import { clearPendingAction, peekPendingAction, savePendingAction } from "../../utils/pending-action";
import searchIcon from "../../static/icons/search.svg";
import filterIcon from "../../static/icons/filter.svg";
import heartIcon from "../../static/icons/heart.svg";
import propertyPlaceholderIcon from "../../static/icons/property-placeholder.svg";
import { listingDoorplate, listingTagValue, publicListingTitle, uniqueListingTags } from "../../utils/listing-title";
import { normalizeCommunityName } from "../../utils/community-name";

const properties = ref<Property[]>([]),
  favoriteIDs = ref<string[]>([]),
  keyword = ref(""),
  selectedArea = ref("全部"),
  selectedKind = ref(""),
  selectedStatus = ref("all"),
  currentRole = ref("");
const selectedPrice = ref(0),
  showFavoritesOnly = ref(false),
  showMoreFilters = ref(false),
  configuredLocations = ref<{ label: string; keyword: string }[]>([]),
  configuredChannels = ref<Array<{ title: string; keyword: string; community?: string; latitude?: number; longitude?: number; radiusKm?: number }>>([]),
  pagePresentation = ref({ copy: {} as Record<string, string>, searchPlaceholder: "", operatorTitle: "", title: "找一处合适的家", subtitle: "按地区和预算筛选", backgroundImage: "" }),
  channelFilter = ref<{
    keyword: string;
    latitude: number;
    longitude: number;
    radiusKm: number;
    title?: string;
    community?: string;
  } | null>(null),
  loading = ref(false),
  loadFailed = ref(false),
  workingID = ref("");
const listingTitleFields = ref<string[]>(["community", "floor", "layout", "rentType", "area"]);
const adminDoorplateEnabled = ref(true);
const configuredStatusLabels = ref<Record<string, string>>({});
const listTagLayout = ref<"grouped" | "wrap">("grouped");
const listTagOrder = ref<string[]>(["rentType", "listingSource", "layout", "fee", "floor", "area"]);
const listTagTopFields = ref<string[]>(["rentType", "listingSource"]);
const priceRanges = [
  { label: "不限价格", min: 0, max: 0 },
  { label: "≤1500", min: 0, max: 1500 },
  { label: "1500–2500", min: 1500, max: 2500, minExclusive: true },
  { label: "2500–3500", min: 2500, max: 3500, minExclusive: true, maxExclusive: true },
  { label: "≥3500", min: 3500, max: 0 },
];
const areas = computed(() =>
  configuredLocations.value.length
    ? [{ label: "全部", keyword: "" }, ...configuredLocations.value]
    : [{ label: "全部", keyword: "" }],
);
const pinnedAreaChannels = (config: Awaited<ReturnType<typeof getPublicConfig>> | null) => {
  const channels = config?.featuredChannels || [];
  const flagged = channels.filter(channel => channel.showInAreaEntry);
  if (flagged.length) return flagged.slice(0, 4);
  const labels = new Set((config?.locations || []).map(location => location.label.trim()).filter(Boolean));
  const matches = channels.filter(channel => labels.has(channel.title.trim()));
  return (matches.length ? matches : channels.slice(0, 3)).slice(0, 4);
};
const isAdmin = computed(() => currentRole.value === "admin");
const adminStatusOptions = [
  { value: "all", label: "全部" },
  { value: "draft", label: "草稿" },
  { value: "pending_review", label: "待审核" },
  { value: "available", label: "可租" },
  { value: "published", label: "待设可租" },
  { value: "rented", label: "已租" },
  { value: "offline", label: "已下架" },
];
const statusLabel = (status: string) => {
  const adminLabel = adminStatusOptions.find(option => option.value === status)?.label;
  return (isAdmin.value ? adminLabel : configuredStatusLabels.value[status] || adminLabel) || status;
};
const adminStatusMatched = (item: Property) =>
  selectedStatus.value === "all" || item.status === selectedStatus.value;
const kindOptions = ["整租", "合租", "一居", "两居", "三居+"];
const roomCountOf = (item: Property) => {
  const match = `${item.layout || ""} ${item.title || ""}`.match(/(\d+)\s*室/);
  if (match) return Number(match[1]);
  const chinese = `${item.layout || ""} ${item.title || ""}`.match(/([一二两三四五六七八九])\s*居/);
  return chinese ? ({ 一: 1, 二: 2, 两: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9 }[chinese[1]] || 0) : 0;
};
const kindMatched = (item: Property) => {
  if (!selectedKind.value) return true;
  const text = `${item.title || ""} ${item.layout || ""} ${item.rentType || ""}`;
  // 兼容历史房源：旧数据没有 rentType 时，按整租展示，不让新筛选把存量房源全部筛掉。
  if (selectedKind.value === "整租" && !item.rentType) return true;
  if (selectedKind.value === "一居") return roomCountOf(item) === 1;
  if (selectedKind.value === "两居") return roomCountOf(item) === 2;
  if (selectedKind.value === "三居+") return roomCountOf(item) >= 3;
  return text.includes(selectedKind.value);
};
const subscriptionAreaName = computed(() => selectedArea.value !== "全部" ? selectedArea.value : channelFilter.value?.title || "");
const extraFilterCount = computed(() => Number(selectedPrice.value !== 0) + Number(Boolean(selectedKind.value)));
const hasNonFavoriteFilters = computed(() => Boolean(
  keyword.value.trim() || selectedArea.value !== "全部" || selectedPrice.value !== 0 ||
  selectedKind.value || channelFilter.value || (isAdmin.value && selectedStatus.value !== "all"),
));
const resetFilters = () => {
  keyword.value = "";
  selectedArea.value = "全部";
  selectedPrice.value = 0;
  selectedKind.value = "";
  selectedStatus.value = "all";
  channelFilter.value = null;
  showFavoritesOnly.value = false;
};
const selectArea = (label: string) => {
  selectedArea.value = label;
  channelFilter.value = null;
};
const filteredProperties = computed(() => {
  const query = keyword.value.trim().toLowerCase(),
    price = priceRanges[selectedPrice.value];
  return properties.value.filter((item) => {
    const selectedAreaChannel = configuredChannels.value.find(channel => channel.title === selectedArea.value);
    let areaMatched = selectedArea.value === "全部" || item.areaLabel === selectedArea.value;
    if (!areaMatched && selectedAreaChannel?.community && normalizeCommunityName(item.community || "") === normalizeCommunityName(selectedAreaChannel.community)) areaMatched = true;
    if (!areaMatched && selectedArea.value !== "全部" && item.latitude && item.longitude) {
      const candidates = configuredChannels.value.filter((area) => area.latitude && area.longitude);
      if (candidates.length) {
        const nearest = candidates.reduce((current, candidate) =>
          distanceKm(Number(candidate.latitude), Number(candidate.longitude), item.latitude!, item.longitude!) <
          distanceKm(Number(current.latitude), Number(current.longitude), item.latitude!, item.longitude!) ? candidate : current,
        );
        areaMatched = nearest.title === selectedArea.value && distanceKm(Number(nearest.latitude), Number(nearest.longitude), item.latitude, item.longitude) <= Number(nearest.radiusKm || 3);
      }
    }
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
        ? (Boolean(channelFilter.value.community) && normalizeCommunityName(item.community || "") === normalizeCommunityName(channelFilter.value.community || "")) || channelDistance <= channelFilter.value.radiusKm
        : channelFilter.value.community
          ? normalizeCommunityName(item.community || "") === normalizeCommunityName(channelFilter.value.community)
          : !channelFilter.value.keyword || item.location?.includes(channelFilter.value.keyword));
    return (
      (!isAdmin.value || adminStatusMatched(item)) &&
      channelMatched &&
      kindMatched(item) &&
      (!showFavoritesOnly.value || favoriteIDs.value.includes(item.id)) &&
      areaMatched &&
      (price.min === 0 || (price.minExclusive ? item.rentMonthly > price.min : item.rentMonthly >= price.min)) &&
      (!price.max || (price.maxExclusive ? item.rentMonthly < price.max : item.rentMonthly <= price.max)) &&
      (!query ||
        `${item.title} ${item.community} ${item.location} ${item.layout}`
          .toLowerCase()
          .includes(query))
    );
  });
});
const customerMasonryColumns = computed(() => {
  const columns: Property[][] = [[], []];
  const heights = [0, 0];
  filteredProperties.value.forEach((item) => {
    const copyLength = `${listingTitle(item)} ${cardTags(item).join(' ')}`.length;
    const estimate = 360 + Math.ceil(copyLength / 12) * 38;
    const index = heights[0] <= heights[1] ? 0 : 1;
    columns[index].push(item);
    heights[index] += estimate;
  });
  return columns;
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
  const previousRole = currentRole.value;
  currentRole.value = getSession()?.role || "";
  const adminStatus = String(uni.getStorageSync("xiaopi_admin_property_status") || "");
  if (adminStatus) {
    uni.removeStorageSync("xiaopi_admin_property_status");
    selectedStatus.value = currentRole.value === "admin" && adminStatusOptions.some((item) => item.value === adminStatus)
      ? adminStatus
      : "all";
  } else if (previousRole !== currentRole.value) {
    selectedStatus.value = "all";
  }
  // Clear user-only state whenever the active identity changes. This prevents
  // favorites from a previous user session leaking into guest/admin views.
  if (currentRole.value !== "user") {
    favoriteIDs.value = [];
    showFavoritesOnly.value = false;
  }
  // A tenant-only action must not survive a switch to an operator account.
  if (currentRole.value !== "user" && peekPendingAction()) clearPendingAction();
  try {
    const [items, config] = await Promise.all([
      listProperties(),
      getPublicConfig().catch(() => null),
    ]);
    properties.value = items;
    const areaChannels = pinnedAreaChannels(config);
    configuredLocations.value = areaChannels.map(channel => ({ label: channel.title, keyword: channel.community || channel.keyword }));
    configuredChannels.value = areaChannels;
    configuredStatusLabels.value = { ...(config?.statusLabels || {}), pending_review: config?.statusLabels?.pendingReview || "待审核" };
    // [] is an explicit configuration: do not restore hidden public labels.
    if (Array.isArray(config?.listingTitle?.enabledFields)) {
      listingTitleFields.value = config!.listingTitle!.enabledFields!;
    }
    adminDoorplateEnabled.value = Boolean(config?.listingTitle?.adminEnabledFields?.includes("unit"));
    listTagLayout.value = config?.listingTitle?.listTagLayout === "wrap" ? "wrap" : "grouped";
    if (Array.isArray(config?.listingTitle?.listTagOrder)) listTagOrder.value = config!.listingTitle!.listTagOrder!;
    if (Array.isArray(config?.listingTitle?.listTagTopFields)) listTagTopFields.value = config!.listingTitle!.listTagTopFields!;
    pagePresentation.value = { ...pagePresentation.value, ...(config?.pagePresentations?.properties || {}) };
    if (getSession()?.role === "user")
      favoriteIDs.value = await listFavorites().catch(() => []);
    const pending = String(uni.getStorageSync("xiaopi_property_filter") || "");
    if (pending) {
      try {
        const parsed: unknown = JSON.parse(pending);
        if (parsed && typeof parsed === "object") {
          const filter = parsed as Record<string, unknown>;
          const hasCoordinateFilter =
            typeof filter.latitude === "number" &&
            typeof filter.longitude === "number" &&
            typeof filter.radiusKm === "number" && Number(filter.latitude) !== 0 && Number(filter.longitude) !== 0;
          const hasCommunityFilter = typeof filter.community === "string" && Boolean(filter.community.trim());
          if (typeof filter.area === "string") selectedArea.value = filter.area;
          if (typeof filter.priceIndex === "number")
            selectedPrice.value = Math.max(0, Math.min(priceRanges.length - 1, filter.priceIndex));
          // A configured channel with coordinates is a radius filter. Keeping its
          // fallback keyword in the search box would apply a second, unrelated
          // text filter and incorrectly hide properties inside the circle.
          if (!hasCoordinateFilter && !hasCommunityFilter && typeof filter.keyword === "string") keyword.value = filter.keyword;
          if (typeof filter.kind === "string" && kindOptions.includes(filter.kind)) selectedKind.value = filter.kind;
          if (hasCoordinateFilter || hasCommunityFilter) {
            channelFilter.value = {
              keyword: typeof filter.keyword === "string" ? filter.keyword : "",
              latitude: hasCoordinateFilter ? Number(filter.latitude) : 0,
              longitude: hasCoordinateFilter ? Number(filter.longitude) : 0,
              radiusKm: Number(filter.radiusKm),
              title: typeof filter.title === "string" ? filter.title : undefined,
              community: hasCommunityFilter ? String(filter.community) : undefined,
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
const resumePendingAction = async () => {
  const pending = peekPendingAction();
  if (!pending || getSession()?.role !== "user") return;
  clearPendingAction();
  if (pending.type === "favorite" && pending.propertyId) {
    await toggleFavorite(pending.propertyId);
    return;
  }
  if (pending.type === "area_subscription" && pending.area) {
    uni.navigateTo({ url: `/pages/requests/index?mode=area&area=${encodeURIComponent(pending.area)}` });
  }
};
onShow(async () => {
  await load();
  await resumePendingAction();
});
const open = (id: string) =>
    uni.navigateTo({ url: `/pages/properties/detail?id=${id}` }),
  requestHome = () => uni.navigateTo({ url: "/pages/requests/index" });
const coverUrl = (item: Property) => {
  const cover = item.media?.find((media) => media.type === "image")?.url;
  return cover ? resolveAssetUrl(cover) : "";
};
const masonryMedia = (item: Property) =>
  item.media?.find((media) => media.type === "video") || item.media?.find((media) => media.type === "image");
const coverObjectPosition = (item: Property) => {
  const x = [0, 50, 100].includes(Number(item.coverFocusX)) ? Number(item.coverFocusX) : 50;
  const y = [0, 50, 100].includes(Number(item.coverFocusY)) ? Number(item.coverFocusY) : 50;
  return `${x}% ${y}%`;
};
const orderedFields = () => [...listTagOrder.value, ...listingTitleFields.value].filter((field, index, all) => field !== "community" && field !== "unit" && listingTitleFields.value.includes(field) && all.indexOf(field) === index);
const primaryLabels = (item: Property) => uniqueListingTags(orderedFields().filter(field => listTagTopFields.value.includes(field)).map((field) => compactCardTag(field, item)));
const compactCardTag = (field: string, item: Property) => {
  const value = listingTagValue(item, field);
  // A list card needs recognisable information, not the whole record. The
  // detailed layout remains intact on the detail page.
  if (field === "layout") return value.match(/\d+室(?:\d+厅)?/)?.[0] || value;
  return value;
};
const secondaryLabels = (item: Property) => {
  return uniqueListingTags(orderedFields().filter(field => !listTagTopFields.value.includes(field)).map((field) => compactCardTag(field, item)));
};
const cardTags = (item: Property) => uniqueListingTags([...primaryLabels(item), ...secondaryLabels(item)]);
const adminDoorplate = (item: Property) =>
  isAdmin.value && adminDoorplateEnabled.value ? listingDoorplate(item) : "";
const listingTitle = (item: Property) => {
  return isAdmin.value
    ? String(item.community || item.title || "房源").trim()
    : publicListingTitle(item, listingTitleFields.value);
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
const channelLabel = (item: Property) => {
  const direct = configuredChannels.value.find((channel) => channel.title && channel.title === item.areaLabel);
  if (direct) return direct.title;
  if (!item.latitude || !item.longitude) return "";
  return configuredChannels.value
    .filter((channel) => channel.title && channel.latitude && channel.longitude)
    .map((channel) => ({ channel, distance: distanceKm(Number(channel.latitude), Number(channel.longitude), item.latitude!, item.longitude!) }))
    .filter(({ channel, distance }) => distance <= Number(channel.radiusKm || 3))
    .sort((left, right) => left.distance - right.distance)[0]?.channel.title || "";
};
const toggleFavorite = async (id: string) => {
  if (!getSession()) {
    savePendingAction({ type: "favorite", propertyId: id });
    uni.navigateTo({ url: "/pages/auth/login?reason=favorite" });
    return;
  }
  if (getSession()?.role === "admin") {
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
const editProperty = (id: string) => uni.navigateTo({ url: `/pages/admin/property-edit?id=${id}` });
const copyProperty = async (property: Property) => {
  if (workingID.value) return;
  workingID.value = property.id;
  try {
    const copy = await createProperty({
      title: `${property.title || property.community} 副本`,
      community: property.community,
      location: property.location,
      latitude: property.latitude,
      longitude: property.longitude,
      layout: property.layout,
      rentMonthly: property.rentMonthly,
      feeText: property.feeText,
      description: property.description,
    });
    uni.showToast({ title: "已创建房源副本" });
    await load();
    editProperty(copy.id);
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "复制失败", icon: "none" });
  } finally {
    workingID.value = "";
  }
};
const changeStatus = async (property: Property, status: string) => {
  if (workingID.value) return;
  const current = statusLabel(property.status);
  const target = statusLabel(status);
  uni.showModal({
    title: `${current} → ${target}`,
    content: `确认将房源状态从「${current}」改为「${target}」吗？${status === "rented" ? "确认后将从可租房源中移除，并归入已租。" : status === "available" ? "确认后将作为可租房源展示给用户。" : status === "offline" ? "下架后，用户端将不再展示该房源。" : ""}`,
    confirmText: `确认${target}`,
    success: async (result) => {
      if (!result.confirm) return;
      workingID.value = property.id;
      try {
        await request("/v1/properties/status", { method: "PATCH", data: { propertyId: property.id, status, note: `房源管理：${current}改为${target}` } });
        uni.showToast({ title: `已改为${target}` });
        await load();
      } catch (error) {
        uni.showToast({ title: error instanceof Error ? error.message : "操作失败", icon: "none" });
      } finally { workingID.value = ""; }
    },
  });
};
const removeProperty = (property: Property) => {
  const isLiveListing = ["published", "available", "rented"].includes(property.status);
  uni.showModal({
    title: isLiveListing ? "确认删除房源" : "删除房源",
    content: isLiveListing
      ? "该房源曾对外展示。删除会移除图片、视频、收藏和浏览记录；已提交的预约会保留，但不再关联此房源。此操作无法恢复。"
      : "将删除房源及其图片、视频，无法恢复。",
    confirmText: "删除",
    confirmColor: "#c45843",
    success: async (result) => {
    if (!result.confirm) return;
    try { await deleteProperty(property.id); uni.showToast({ title: "房源已删除" }); await load(); }
    catch (error) { uni.showToast({ title: error instanceof Error ? error.message : "删除失败", icon: "none" }); }
  }});
};
const subscribeArea = () => {
	const area = subscriptionAreaName.value;
	if (!area) { uni.showToast({ title: "请先选择或搜索片区", icon: "none" }); return; }
	if (!getSession()) {
      savePendingAction({ type: "area_subscription", area });
      uni.navigateTo({ url: "/pages/auth/login?reason=area_subscription" });
      return;
    }
	if (isAdmin.value) return;
	uni.navigateTo({ url: `/pages/requests/index?mode=area&area=${encodeURIComponent(area)}` });
};
const requestPrompt = computed(() => showFavoritesOnly.value
  ? copyText("还没有找到关注的房源")
  : subscriptionAreaName.value
    ? copyText("{area} 暂时没有合适的房源").replace(/\{area\}/g, subscriptionAreaName.value)
    : keyword.value.trim()
      ? copyText("没有找到“{keyword}”相关房源").replace(/\{keyword\}/g, keyword.value.trim())
      : copyText("暂时没有符合条件的房源"));
const requestPromptDescription = computed(() => showFavoritesOnly.value
  ? copyText("先浏览房源并点亮爱心，关注的房源会集中保存在这里。")
  : copyText("可以调整筛选条件，或告诉小皮你的预算和入住要求。"));
const handleRequestPrompt = () => {
  if (showFavoritesOnly.value) {
    resetFilters();
  } else if (subscriptionAreaName.value) {
    subscribeArea();
  } else {
    requestHome();
  }
};

function copyText(fallback: string) { return pagePresentation.value?.copy?.[fallback] || fallback }
</script>
<template>
  <view class="page" :style="pagePresentation.backgroundImage ? { backgroundImage: `linear-gradient(rgba(247,246,241,.92), rgba(247,246,241,.96)), url(${resolveAssetUrl(pagePresentation.backgroundImage)})`, backgroundSize: 'cover', backgroundAttachment: 'fixed' } : {}"
    ><view class="location"
      >{{ isAdmin ? (pagePresentation.operatorTitle || "房源管理") : pagePresentation.title || "找一处合适的家" }} <text>{{ filteredProperties.length }} 套{{ isAdmin ? "房源" : "可租" }}</text></view
    ><view v-if="pagePresentation.subtitle" class="page-subtitle">{{ pagePresentation.subtitle }}</view
    ><view class="search card"
      ><image :src="searchIcon" mode="aspectFit" />
      <input
        v-model="keyword"
        :placeholder="pagePresentation.searchPlaceholder || '搜索小区、位置或户型'"
        confirm-type="search"
      /><text v-if="keyword" class="clear" @click="keyword = ''">×</text></view
    ><view v-if="isAdmin" class="admin-create-property" @click="editProperty('')"><text>＋</text><strong>录入新房源</strong><small>保存为草稿后继续完善</small></view
    ><view
      v-if="!isAdmin"
      class="favorite-filter"
      :class="{ active: showFavoritesOnly }"
      @click="showFavoritesOnly = !showFavoritesOnly"
      ><image :src="heartIcon" mode="aspectFit" />{{ showFavoritesOnly ? "只看已关注" : "只看关注" }}</view
    ><view class="filter-block"
      v-if="isAdmin"
      :class="{ 'admin-filter-row': isAdmin }"
      ><view class="filter-label">状态</view
      ><scroll-view scroll-x class="chips"><text v-for="status in adminStatusOptions" :key="status.value" :class="{ active: selectedStatus === status.value }" @click="selectedStatus = status.value">{{ status.label }}</text></scroll-view></view
    ><view class="filter-block" :class="{ 'admin-filter-row': isAdmin }"
      ><view class="filter-label">地区</view
      ><scroll-view scroll-x class="chips"
        ><text
          v-for="area in areas"
          :key="area.label"
          :class="{ active: selectedArea === area.label }"
          @click="selectArea(area.label)"
          >{{ area.label }}</text
        ></scroll-view
      ></view
    ><view class="filter-more-toggle" :class="{ active: showMoreFilters || extraFilterCount > 0 }" @click="showMoreFilters = !showMoreFilters"
      ><view><image :src="filterIcon" mode="aspectFit" />更多筛选<text v-if="extraFilterCount" class="filter-count">{{ extraFilterCount }}</text></view><text class="filter-toggle-caption">{{ selectedPrice ? priceRanges[selectedPrice].label : "预算不限" }} · {{ selectedKind || "住法不限" }}</text><text class="filter-chevron" :class="{ expanded: showMoreFilters }">⌄</text></view
    ><template v-if="showMoreFilters"><view class="filter-block"
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
    ><view class="filter-block"
      ><view class="filter-label">住法</view
      ><scroll-view scroll-x class="chips"
        ><text :class="{ active: !selectedKind }" @click="selectedKind = ''">不限</text
        ><text v-for="kind in kindOptions" :key="kind" :class="{ active: selectedKind === kind }" @click="selectedKind = selectedKind === kind ? '' : kind">{{ kind }}</text
      ></scroll-view
      ></view></template
    ><view v-if="loading" class="listing-skeletons"><view v-for="index in 2" :key="index" class="listing-skeleton"><view class="skeleton-cover" /><view class="skeleton-copy"><view /><view /><view /></view></view></view
    ><view v-else-if="loadFailed" class="state card"
      ><view>房源加载失败</view
      ><button size="mini" @click="load">重新加载</button></view
    ><view v-else-if="filteredProperties.length" class="grid" :class="{ 'customer-masonry-grid': !isAdmin }"
      ><template v-if="!isAdmin"><view v-for="(column, columnIndex) in customerMasonryColumns" :key="`column-${columnIndex}`" class="masonry-column"><view v-for="item in column" :key="item.id" class="masonry-property" @click="open(item.id)"><view class="masonry-cover"><video v-if="masonryMedia(item)?.type === 'video'" :src="resolveAssetUrl(masonryMedia(item)?.url || '')" :style="{ objectPosition: coverObjectPosition(item) }" muted :controls="false" :show-center-play-btn="false" object-fit="cover" /><image v-else-if="masonryMedia(item)?.url" :src="resolveAssetUrl(masonryMedia(item)?.url || '')" :style="{ objectPosition: coverObjectPosition(item) }" mode="aspectFill" /><view v-else class="property-placeholder"><image :src="propertyPlaceholderIcon" mode="aspectFit" /><text>{{ copyText("暂无实拍图") }}</text></view><view class="badge" :class="`status-${item.status}`">可租</view><view v-if="masonryMedia(item)?.type === 'video'" class="masonry-video-play">▶</view><view v-if="channelLabel(item)" class="cover-location">⌖ {{ channelLabel(item) }}</view><view class="favorite" :class="{ active: favoriteIDs.includes(item.id) }" @click.stop="toggleFavorite(item.id)"><image :src="heartIcon" mode="aspectFit" /></view></view><view class="masonry-copy" :class="{ 'without-tags': !cardTags(item).length }"><view class="property-listing-title">{{ listingTitle(item) }}</view><view v-if="cardTags(item).length" class="masonry-tags"><text v-for="label in cardTags(item)" :key="label">{{ label }}</text></view><view class="price">¥ {{ item.rentMonthly }}<text class="price-unit">/月</text></view></view></view></view></template><view
        v-else
        v-for="item in filteredProperties"
        :key="item.id"
        class="property card"
        :class="{ 'admin-property': isAdmin }"
        @click="open(item.id)"
        ><view class="property-main"
          ><view class="cover"
            ><video v-if="masonryMedia(item)?.type === 'video'" :src="resolveAssetUrl(masonryMedia(item)?.url || '')" :poster="coverUrl(item)" :style="{ objectPosition: coverObjectPosition(item) }" muted :controls="false" :show-center-play-btn="false" object-fit="cover" />
            <image
              v-else-if="masonryMedia(item)?.url"
              :src="resolveAssetUrl(masonryMedia(item)?.url || '')"
              :style="{ objectPosition: coverObjectPosition(item) }" mode="aspectFill"
            /><view v-else class="property-placeholder"><image :src="propertyPlaceholderIcon" mode="aspectFit" /><text>{{ copyText("暂无实拍图") }}</text></view><view class="badge" :class="`status-${item.status}`">{{ isAdmin ? statusLabel(item.status) : "可租" }}</view
            ><view v-if="masonryMedia(item)?.type === 'video'" class="cover-video" aria-label="包含视频">▶</view
            ><view v-if="channelLabel(item)" class="cover-location">⌖ {{ channelLabel(item) }}</view
            ><view
              v-if="!isAdmin"
              class="favorite"
              :class="{ active: favoriteIDs.includes(item.id) }"
              @click.stop="toggleFavorite(item.id)"
              ><image :src="heartIcon" mode="aspectFit" /></view
            ></view
          ><view class="property-copy"
          ><view class="property-listing-title">{{ listingTitle(item) }}</view
          ><view v-if="adminDoorplate(item)" class="admin-doorplate">门牌 · {{ adminDoorplate(item) }}</view
          ><view class="service-tag-stack" :class="`tag-layout-${listTagLayout}`"><template v-if="listTagLayout === 'grouped'"><view class="service-tags primary-tags"><text v-for="label in primaryLabels(item)" :key="label">{{ label }}</text></view><view v-if="secondaryLabels(item).length" class="service-tags fee-tags"><text v-for="label in secondaryLabels(item)" :key="label">{{ label }}</text></view></template><view v-else class="service-tags all-tags"><text v-for="label in cardTags(item)" :key="label">{{ label }}</text></view></view
          ><view class="price"
            >¥ {{ item.rentMonthly }}<text class="price-unit">/月</text></view
          ></view
        ></view
        ><view v-if="isAdmin" class="admin-card-actions" @click.stop
          ><view class="admin-inline-actions"><template v-if="['draft', 'pending_review'].includes(item.status)"><button class="action-primary" @click.stop="editProperty(item.id)">编辑并发布</button></template><template v-else-if="item.status === 'published'"><button class="action-edit" @click.stop="editProperty(item.id)">编辑</button><button class="action-primary" :loading="workingID === item.id" @click.stop="changeStatus(item, 'available')">改为可租</button><button class="action-rented" :loading="workingID === item.id" @click.stop="changeStatus(item, 'rented')">改为已租</button><button class="action-offline" :loading="workingID === item.id" @click.stop="changeStatus(item, 'offline')">下架</button></template><template v-else-if="item.status === 'available'"><button class="action-edit" @click.stop="editProperty(item.id)">编辑</button><button class="action-rented" :loading="workingID === item.id" @click.stop="changeStatus(item, 'rented')">改为已租</button><button class="action-offline" :loading="workingID === item.id" @click.stop="changeStatus(item, 'offline')">下架</button></template><template v-else-if="item.status === 'rented'"><button class="action-edit" @click.stop="editProperty(item.id)">编辑</button><button class="action-primary" :loading="workingID === item.id" @click.stop="changeStatus(item, 'available')">改为可租</button></template><template v-else><button class="action-edit" @click.stop="editProperty(item.id)">编辑</button><button class="action-primary" :loading="workingID === item.id" @click.stop="changeStatus(item, 'available')">改为可租</button><button class="action-rented" :loading="workingID === item.id" @click.stop="changeStatus(item, 'rented')">改为已租</button></template><button class="action-copy" :loading="workingID === item.id" @click.stop="copyProperty(item)">复制</button><button class="action-delete-inline" @click.stop="removeProperty(item)">删除</button></view></view
        ></view
      ></view
    ><view v-if="!loading && !loadFailed && !filteredProperties.length" class="empty-match card" :class="{ 'favorites-empty': showFavoritesOnly }"
      ><view class="empty-icon">⌕</view
      ><view class="empty-title">{{ showFavoritesOnly ? requestPrompt : isAdmin ? copyText("当前筛选下暂无房源") : requestPrompt }}</view
      ><view class="muted">{{ isAdmin ? copyText("调整地区、预算或关键词，继续查看房源。") : requestPromptDescription }}</view
      ><view v-if="!isAdmin" class="request-button" @click="handleRequestPrompt"
        >{{ showFavoritesOnly ? "浏览全部房源" : subscriptionAreaName ? "关注该片区新房" : "提交找房需求" }}</view
      ><view v-else class="request-button secondary" @click="resetFilters">重置筛选</view
      ><view v-if="!isAdmin && !showFavoritesOnly && hasNonFavoriteFilters" class="reset-filter-link" @click="resetFilters">清除筛选条件</view
      ></view
    ><view v-if="!loading && !loadFailed && filteredProperties.length && !isAdmin && !showFavoritesOnly" class="request-banner" @click="subscriptionAreaName ? subscribeArea() : requestHome()"
      ><view
        ><strong>{{ subscriptionAreaName ? `关注${subscriptionAreaName}新房源` : "还没找到合适的？" }}</strong
        ><text>{{ subscriptionAreaName ? "有新房源时及时通知你" : "告诉小皮你的预算和入住要求" }}</text></view
      ><text><image :src="filterIcon" mode="aspectFit" />{{ subscriptionAreaName ? "立即关注 ›" : "发布需求 ›" }}</text></view
    ></view
  >
</template>
<style scoped>
/* Legacy declarations below are intentionally reset by the canonical listing layer. */
.location {
  font-size: 36rpx;
  color: #29382d;
  margin: 28rpx 0 22rpx;
  font-weight: 800;
}
.location text {
  float: right;
  color: #40543f;
  font-size: 24rpx;
  font-weight: 500;
}
.search {
  display: flex;
  align-items: center;
  gap: 16rpx;
  color: #879183;
  margin-bottom: 20rpx;
}
.search input {
  flex: 1;
  color: #29382d;
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
  color: #40543f;
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
.cover image,.cover video {
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
  background: #40543f;
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
  color: #7e887d;
}
.empty-icon {
  font-size: 70rpx;
}
.empty-title {
  margin: 12rpx 0;
  font-size: 31rpx;
  font-weight: 700;
  color: #29382d;
}
.request-button {
  display: inline-block;
  margin-top: 28rpx;
  padding: 18rpx 32rpx;
  border-radius: 999rpx;
  background: #40543f;
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
  background: linear-gradient(135deg, #29382d, #536a45);
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
.area-subscribe{display:flex;align-items:center;justify-content:space-between;gap:20rpx;margin-top:24rpx;border:0;background:#e9eddc}.area-subscribe strong,.area-subscribe text{display:block}.area-subscribe strong{font-size:27rpx}.area-subscribe text{margin-top:7rpx;color:#7b8579;font-size:21rpx}.area-subscribe b{flex:none;color:#526442;font-size:22rpx}
.location{margin:18rpx 0 12rpx}.page-subtitle{margin:0 0 24rpx;line-height:1.55}.search{margin-bottom:20rpx}.filter-block{margin:18rpx 0}.grid{margin-top:30rpx}.request-banner{margin-top:40rpx}
.location{display:flex;align-items:flex-end;justify-content:space-between;gap:18rpx;font-size:44rpx;line-height:1.18}.location text{float:none;flex:none;padding:0 0 4rpx;font-size:22rpx;line-height:1.2}.filter-label{margin-bottom:10rpx;font-weight:600;line-height:1.25}.chips text{line-height:1.2}.property .small{font-weight:700}.property .summary{line-height:1.5}.request-banner strong{font-size:29rpx;line-height:1.3}.request-banner>view>text{display:block;margin-top:7rpx;line-height:1.45}
.location{min-width:0}.location>text:first-child{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.property-copy{display:flex;min-width:0;flex:1;flex-direction:column;padding:9rpx 4rpx 3rpx 0}.property .small{margin:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.property .summary{position:static;display:-webkit-box;margin:9rpx 0 0;min-height:0;-webkit-line-clamp:2;-webkit-box-orient:vertical}.property>.property-copy>.price{position:static;margin:12rpx 0 0;font-size:34rpx;line-height:1.1}.property>.property-copy>.price small{font-size:19rpx}.property{min-height:216rpx;box-sizing:border-box}
.property>.property-copy>.price{margin-top:auto;padding-top:12rpx}
.cover-benefit{position:absolute;bottom:0;left:0;padding:8rpx 11rpx;border-radius:0 10rpx 0 0;background:#d66d50;color:#fff;font-size:18rpx;font-weight:700;line-height:1.1}.service-tags{display:flex;gap:7rpx;overflow:hidden;margin-top:9rpx;white-space:nowrap}.service-tags text{flex:none;padding:5rpx 8rpx;border-radius:7rpx;background:#edf0dc;color:#536a45;font-size:18rpx;line-height:1.2}.service-tags text:nth-child(2){background:#f8eadf;color:#b7644b}.property>.property-copy>.service-tags+.price{margin-top:auto;padding-top:8rpx}
.property>.property-copy>.price .price-unit{margin-left:3rpx;color:#8a8f85;font-size:19rpx;font-weight:400}

/* Canonical listing layer: one layout, one spacing scale, no positional price hacks. */
.location {
  display:flex; align-items:flex-end; justify-content:space-between; gap:18rpx;
  min-width:0; margin:26rpx 0 10rpx; color:#29382d;
  font-size:44rpx; font-weight:800; line-height:1.18; letter-spacing:-2rpx;
}
.location > text:first-child { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.location text { float:none; flex:none; padding:0 0 4rpx; color:#7b8778; font-size:22rpx; font-weight:600; line-height:1.2; letter-spacing:0; }
.page-subtitle { margin:0 0 26rpx; color:#7e867d; font-size:23rpx; line-height:1.55; }
.search { display:flex; align-items:center; gap:14rpx; margin:0 0 16rpx; padding:18rpx 20rpx; border:1rpx solid #e8e3da; border-radius:18rpx; background:#fffdf9; box-shadow:none; }
.search image { width:32rpx; height:32rpx; flex:none; }
.search input { flex:1; min-width:0; color:#334034; font-size:26rpx; }
.clear { flex:none; padding:4rpx 8rpx; color:#7a8576; font-size:32rpx; line-height:1; }
.favorite-filter { display:flex; align-items:center; gap:8rpx; width:max-content; margin:0 0 18rpx; padding:10rpx 14rpx; border:1rpx solid #d9ded2; border-radius:12rpx; color:#697564; font-size:22rpx; line-height:1.2; }
.favorite-filter image { width:24rpx; height:24rpx; }
.favorite-filter.active { border-color:#e9c9bc; background:#fcf1ed; color:#cc654b; }
.filter-block { display:block; margin:18rpx 0; }
.filter-label { margin-bottom:10rpx; color:#5a6757; font-size:22rpx; font-weight:700; line-height:1.25; }
.chips { width:100%; white-space:nowrap; }
.chips text { display:inline-block; box-sizing:border-box; min-height:58rpx; margin-right:10rpx; padding:16rpx 18rpx; border:1rpx solid #e8e5df; border-radius:12rpx; background:#fffdf9; color:#6f796e; font-size:22rpx; line-height:1.15; }
.chips .active { border-color:#cbd7ad; background:#e6ecd7; color:#42523b; font-weight:700; }
.grid { display:block; margin-top:30rpx; }
.property { position:relative; display:flex; align-items:stretch; gap:18rpx; min-height:216rpx; margin:0 0 18rpx; padding:12rpx; overflow:hidden; border:1rpx solid #eee6dc; border-radius:20rpx; background:#fffdf9; box-sizing:border-box; }
.cover { position:relative; flex:none; width:244rpx; height:200rpx; overflow:hidden; border-radius:14rpx; background:#dde5d4; }
.cover image { width:100%; height:100%; }
.cover > text { display:grid; width:100%; height:100%; place-items:center; color:#fff; font-size:27rpx; }
.badge { top:9rpx; left:9rpx; padding:5rpx 8rpx; border-radius:8rpx; background:#536a45; color:#fff; font-size:18rpx; line-height:1.1; }
.favorite { top:9rpx; right:9rpx; width:44rpx; height:44rpx; background:rgba(47,44,34,.33); }
.favorite image { width:28rpx; height:28rpx; }.favorite.active { background:#d66d50; }
.cover-benefit { bottom:0; left:0; padding:8rpx 11rpx; border-radius:0 10rpx 0 0; background:#d66d50; color:#fff; font-size:18rpx; font-weight:700; line-height:1.1; }
.property-copy { display:flex; min-width:0; flex:1; flex-direction:column; padding:6rpx 4rpx 3rpx 0; }
.property-listing-title { display:block; overflow:hidden; max-width:100%; margin:0; color:#2d3d30; font-size:29rpx; font-weight:800; line-height:1.28; letter-spacing:-.4rpx; text-overflow:ellipsis; white-space:nowrap; }
.admin-doorplate{display:block;overflow:hidden;margin-top:7rpx;color:#526553;font-size:21rpx;font-weight:700;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}
.property-listing-address { overflow:hidden; max-height:31rpx; margin:8rpx 0 0; color:#778176; font-size:22rpx; font-weight:400; line-height:1.4; text-overflow:ellipsis; white-space:nowrap; }
.service-tags { display:flex; height:32rpx; gap:10rpx; overflow:hidden; margin-top:10rpx; white-space:nowrap; }
.service-tags text { flex:none; padding:5rpx 8rpx; border-radius:7rpx; background:#edf0dc; color:#536a45; font-size:18rpx; line-height:1.2; }.service-tags text:nth-child(2) { background:#f8eadf; color:#b7644b; }
.property > .property-copy > .price { display:inline-flex; width:max-content; align-items:baseline; gap:3rpx; margin:auto 0 0; padding-top:10rpx; color:#d76440; font-size:34rpx; font-weight:800; line-height:1.1; white-space:nowrap; }
.property > .property-copy > .price .price-unit { color:#8a8f85; font-size:20rpx; font-weight:400; }
.property { height:224rpx; min-height:224rpx; }
.property .summary { margin-top:8rpx; color:#7f897e; font-size:18rpx; line-height:1.4; -webkit-line-clamp:1; }
.area-subscribe { margin-top:26rpx; padding:24rpx; border:0; border-radius:18rpx; background:#e9eddc; }.area-subscribe strong { font-size:27rpx; }.area-subscribe text { font-size:21rpx; line-height:1.45; }.area-subscribe b { color:#526442; font-size:22rpx; }
.empty-match { margin-top:24rpx; }.request-banner { margin:34rpx 0 12rpx; padding:25rpx 24rpx; border-radius:18rpx; background:#40543f; }.request-banner strong { font-size:29rpx; line-height:1.3; }.request-banner > view > text { margin-top:7rpx; line-height:1.45; }.request-banner > text { font-size:22rpx; }
/* Admin listing layer: lifecycle controls remain inside the same property card. */
.property-main { display:flex; align-items:stretch; gap:18rpx; min-width:0; }
.property-main .cover { flex:none; }
.property-main .property-copy { min-width:0; flex:1; }
.admin-property { display:block; height:auto; min-height:0; padding:14rpx; overflow:visible; border-color:#e5e9df; border-radius:22rpx; background:#fffefa; }
.admin-property .property-main { height:200rpx; min-height:200rpx; }
.badge.status-draft,.badge.status-pending_review { background:#a4773f; }
.badge.status-rented { background:#69737d; }
.badge.status-offline { background:#8a7566; }
.admin-card-actions { min-height:70rpx; margin-top:14rpx; padding:13rpx 0 0; border-top:1rpx solid #eceee8; }
.admin-create-property{display:flex;align-items:center;gap:12rpx;margin:14rpx 0 18rpx;padding:16rpx 20rpx;border:1rpx solid #dfe6d8;border-radius:16rpx;background:#f8faf5;color:#40543f}.admin-create-property>text{display:grid;width:42rpx;height:42rpx;place-items:center;border-radius:50%;background:#e7edcf;font-size:30rpx;line-height:1}.admin-create-property strong{font-size:23rpx}.admin-create-property small{margin-left:auto;color:#869083;font-size:19rpx}
.admin-inline-actions { display:flex; min-width:0; flex-wrap:wrap; align-items:center; gap:8rpx; }
.admin-inline-actions button { min-width:96rpx; flex:1 1 96rpx; margin:0; padding:0 8rpx; border:1rpx solid transparent; border-radius:9rpx; font-size:20rpx; font-weight:650; line-height:50rpx; white-space:nowrap; }
.admin-inline-actions button::after { border:0; }
.action-edit { border-color:#dfe5da!important; background:#fff!important; color:#5d6b58!important; }
.action-primary { border-color:#40543f!important; background:#40543f!important; color:#fff!important; }
.action-rented { border-color:#dbe1e5!important; background:#edf1f3!important; color:#53616c!important; }
.action-offline { border-color:#eadbd0!important; background:#fff8f3!important; color:#9b694d!important; }
.action-delete-inline { border-color:#efdcd4!important; background:#fff8f5!important; color:#b76c57!important; }
.action-copy { border-color:#dce4d8!important; background:#f4f7f1!important; color:#63735e!important; }
.admin-inline-actions button:active { opacity:.78; transform:scale(.985); }
.listing-skeletons{margin-top:30rpx}.listing-skeleton{display:flex;height:224rpx;gap:18rpx;margin-bottom:18rpx;padding:12rpx;border-radius:20rpx;background:#fffdf9;box-sizing:border-box}.skeleton-cover,.skeleton-copy>view{background:linear-gradient(100deg,#ebece6 25%,#f6f5f0 42%,#ebece6 58%);background-size:200% 100%;animation:listing-skeleton-shift 1.2s linear infinite}.skeleton-cover{width:244rpx;height:200rpx;flex:none;border-radius:14rpx}.skeleton-copy{min-width:0;flex:1;padding-top:12rpx}.skeleton-copy>view{height:28rpx;margin-bottom:18rpx;border-radius:8rpx}.skeleton-copy>view:nth-child(2){width:72%;height:20rpx}.skeleton-copy>view:nth-child(3){width:48%;height:34rpx;margin-top:52rpx}@keyframes listing-skeleton-shift{to{background-position:-200% 0}}
.property:active{opacity:.92}.chips text:active,.favorite-filter:active{transform:scale(.98)}
/* Listing card acceptance: match home while preserving admin controls. */
.property{height:232rpx;min-height:232rpx;gap:18rpx;padding:12rpx;border-color:#e7e5dd;border-radius:22rpx;background:#fffdf9;box-shadow:0 7rpx 20rpx rgba(55,65,50,.045)}
.cover{width:248rpx;height:208rpx;border-radius:16rpx}
.property-copy{height:208rpx;padding:5rpx 5rpx 4rpx 0;box-sizing:border-box}
.property-listing-title{font-size:30rpx;line-height:1.33;letter-spacing:-.3rpx}
.property-listing-address{max-height:34rpx;margin-top:10rpx;color:#707b70;font-size:24rpx;line-height:1.4}
.service-tags{height:34rpx;gap:8rpx;margin-top:10rpx}.service-tags text{padding:6rpx 9rpx;border-radius:8rpx;font-size:18rpx}
.property>.property-copy>.price{min-height:36rpx;padding-top:8rpx;font-size:35rpx;line-height:1}.property>.property-copy>.price .price-unit{font-size:19rpx}
.admin-property{height:auto;min-height:0;padding:14rpx;border-radius:22rpx}.admin-property .property-main{height:208rpx;min-height:208rpx}.admin-property .property-copy{height:208rpx}.admin-card-actions{margin-top:14rpx;padding-top:14rpx}.admin-inline-actions{gap:9rpx}.admin-inline-actions button{min-width:98rpx;min-height:54rpx;border-radius:10rpx;font-size:21rpx;line-height:52rpx}
/* Preserve the information-dense list while giving each cover more visual weight. */
.cover{width:276rpx;height:208rpx}.property-main{gap:16rpx}.cover-video{position:absolute;top:10rpx;right:10rpx;display:grid;width:38rpx;height:38rpx;place-items:center;border:1rpx solid rgba(255,255,255,.72);border-radius:50%;box-sizing:border-box;background:rgba(34,44,35,.58);color:#fff;font-size:18rpx;font-weight:700;line-height:1;padding-left:2rpx}.cover-location{position:absolute;right:0;bottom:0;left:0;overflow:hidden;padding:20rpx 10rpx 8rpx;background:linear-gradient(transparent,rgba(27,35,28,.66));color:#fff;font-size:18rpx;line-height:1.2;text-overflow:ellipsis;white-space:nowrap}.admin-property .property-main{height:208rpx;min-height:208rpx}
.service-tag-stack{display:flex;flex-direction:column;align-items:flex-start;gap:5rpx;margin-top:8rpx}.service-tag-stack .service-tags{height:29rpx;margin:0}.service-tag-stack .service-tags text{padding:4rpx 8rpx;font-size:19rpx;line-height:1.1}.service-tag-stack .fee-tags text{background:#f8eadf;color:#b7644b}
/* Admin cards carry a doorplate, but it is metadata rather than another tag. */
.admin-property .property-main{height:auto;min-height:244rpx;align-items:flex-start}
.admin-property .property-copy{height:auto;min-height:244rpx}
.admin-property .admin-doorplate{margin-top:8rpx;padding:5rpx 9rpx;border:1rpx solid #d8e2d2;border-radius:8rpx;background:#f3f7ee;color:#40543f;font-size:19rpx;font-weight:700;line-height:1.2}
.admin-property .service-tag-stack{gap:5rpx;margin-top:8rpx}
.admin-property .service-tags{max-width:100%;overflow:hidden}
.tag-layout-wrap .all-tags{display:flex;height:auto;max-height:58rpx;flex-wrap:wrap;gap:5rpx 7rpx;overflow:hidden}.tag-layout-wrap .all-tags text{padding:4rpx 8rpx;font-size:19rpx}.tag-layout-wrap .all-tags text:nth-child(2n){background:#f8eadf;color:#b7644b}
/* Customer browsing uses two independently flowing columns; management keeps its action-oriented row cards. */
.customer-masonry-grid{display:flex;align-items:flex-start;gap:16rpx;margin-top:30rpx}.masonry-column{min-width:0;flex:1}.masonry-property{overflow:hidden;margin-bottom:16rpx;border:1rpx solid #e9e2d8;border-radius:20rpx;background:#fffdf9;box-shadow:0 7rpx 18rpx rgba(55,65,50,.05)}.masonry-cover{position:relative;width:100%;aspect-ratio:3/4;overflow:hidden;background:#dde5d4}.masonry-cover image{width:100%;height:100%}.masonry-cover>text{display:grid;width:100%;height:100%;place-items:center;color:#fff;font-size:25rpx}.masonry-cover .favorite{top:10rpx;right:10rpx;width:42rpx;height:42rpx}.masonry-cover .favorite image{width:26rpx;height:26rpx}.masonry-video-play{position:absolute;top:50%;left:50%;display:grid;width:62rpx;height:62rpx;place-items:center;border:2rpx solid rgba(255,255,255,.88);border-radius:50%;background:rgba(28,38,30,.5);color:#fff;font-size:28rpx;line-height:1;transform:translate(-50%,-50%);padding-left:4rpx;box-sizing:border-box}.masonry-copy{padding:15rpx 14rpx 16rpx}.masonry-copy .property-listing-title{display:-webkit-box;overflow:hidden;font-size:26rpx;line-height:1.35;white-space:normal;-webkit-box-orient:vertical;-webkit-line-clamp:2}.masonry-copy .property-listing-address{display:-webkit-box;max-height:none;margin-top:8rpx;font-size:19rpx;line-height:1.45;white-space:normal;-webkit-box-orient:vertical;-webkit-line-clamp:2}.masonry-tags{display:flex;flex-wrap:wrap;gap:6rpx;margin-top:10rpx}.masonry-tags text{padding:5rpx 7rpx;border-radius:6rpx;background:#edf0dc;color:#536a45;font-size:17rpx;line-height:1.15}.masonry-tags text:nth-child(2n){background:#f8eadf;color:#b7644b}.masonry-copy .price{display:inline-flex;margin-top:14rpx;color:#d76440;font-size:30rpx;font-weight:800;line-height:1}.masonry-copy .price .price-unit{margin-left:3rpx;color:#8a8f85;font-size:18rpx;font-weight:400}
.masonry-copy.without-tags .price{margin-top:10rpx}
.masonry-cover video{width:100%;height:100%}

/* Search controls: keep the primary area filter visible and tuck secondary
   filters behind one compact, stateful disclosure row. */
.filter-more-toggle{display:flex;align-items:center;gap:10rpx;min-height:64rpx;margin:8rpx 0 14rpx;padding:0 4rpx;color:#586555;font-size:22rpx;line-height:1.2}
.filter-more-toggle>view{display:flex;align-items:center;gap:8rpx;flex:none;font-weight:700}
.filter-more-toggle image{width:27rpx;height:27rpx;opacity:.78}
.filter-more-toggle.active>view{color:#40543f}
.filter-count{display:grid;min-width:28rpx;height:28rpx;place-items:center;border-radius:50%;background:#e6ecd7;color:#42523b;font-size:18rpx}
.filter-toggle-caption{min-width:0;flex:1;overflow:hidden;color:#8a9187;font-size:20rpx;text-align:right;text-overflow:ellipsis;white-space:nowrap}
.filter-chevron{flex:none;color:#7c8778;font-size:28rpx;transition:transform .18s ease}
.filter-chevron.expanded{transform:rotate(180deg)}
.filter-more-toggle+.filter-block{margin-top:8rpx}
.empty-match{padding:58rpx 30rpx 52rpx;border:1rpx solid #eee8df;border-radius:22rpx;background:#fffdf9}
.empty-icon{display:grid;width:72rpx;height:72rpx;margin:0 auto 16rpx;place-items:center;border-radius:50%;background:#f0f2e9;color:#68765d;font-size:42rpx;line-height:1}
.empty-title{margin:0 0 10rpx;font-size:29rpx;line-height:1.35}
.empty-match .muted{font-size:22rpx;line-height:1.6}
.request-button{display:inline-flex;align-items:center;justify-content:center;min-height:76rpx;margin-top:25rpx;padding:0 34rpx;border-radius:14rpx;font-size:24rpx;font-weight:700;line-height:1.2}
.request-button.secondary{border:1rpx solid #d9ded2;background:#fffdf9;color:#526442}
.reset-filter-link{margin-top:22rpx;color:#7e897c;font-size:21rpx;text-decoration:underline}
.request-banner{min-height:104rpx;box-sizing:border-box}
.service-tag-stack .service-tags text{font-size:20rpx;line-height:1.2;padding:5rpx 9rpx}
.service-tag-stack .service-tags{height:36rpx}
.masonry-tags text{font-size:20rpx;line-height:1.2;padding:6rpx 8rpx}
.masonry-tags{gap:7rpx}
.masonry-cover .favorite{top:8rpx;right:8rpx;width:82rpx;height:82rpx;}
.property-placeholder{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:7rpx;background:linear-gradient(145deg,#edf0e8,#dfe6d8);color:#697763;}
.property-placeholder image{width:88rpx;height:72rpx;opacity:.78;}
.property-placeholder text{font-size:21rpx;line-height:1.2;}
.admin-inline-actions button{display:flex;align-items:center;justify-content:center;box-sizing:border-box;min-height:82rpx;padding:8rpx;line-height:1.2;text-align:center;}
.filter-block.admin-filter-row{flex-wrap:nowrap;align-items:center;gap:8rpx;margin:7rpx 0;}
.admin-filter-row .filter-label{flex:0 0 76rpx;margin:0 0 12rpx;font-size:20rpx;line-height:1.2;white-space:nowrap;}
.admin-filter-row .chips{flex:1;height:64rpx;}
.admin-filter-row .chips text{padding:12rpx 13rpx;margin-right:7rpx;font-size:20rpx;line-height:1.2;}
</style>
