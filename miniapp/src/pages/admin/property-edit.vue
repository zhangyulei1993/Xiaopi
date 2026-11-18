<script setup lang="ts">
import { usePropertyDoorplate } from "./usePropertyDoorplate";
import { watchPostEffect } from "vue";
import { readPrivateCache } from "../../auth/private-cache";
import { permissionAudit } from "../../utils/permission-audit";
import { onLoad, onShow } from "@dcloudio/uni-app";
import { computed, nextTick, reactive, ref, watch } from "vue";
import { API_ORIGIN, safeUploadUrl, request, resolveAssetUrl } from "../../api/client";
import {
  createProperty,
  deleteProperty,
  getPublicConfig,
  listProperties,
  replacePropertyMedia,
  searchLocations,
  updateProperty,
  type LocationSearchItem,
  type Property,
  type PropertyMedia,
} from "../../api/properties";
import { getSession, hasRole } from "../../auth/session";
import { amenityIcon } from "../../utils/amenities";
import { listingDoorplate, listingTagValue, publicListingTitle } from "../../utils/listing-title";
import { usePagePresentation } from "../../utils/presentation";

type PendingImage = {
  path: string;
  width: number;
  height: number;
  size: number;
  originalPath: string;
  originalWidth: number;
  originalHeight: number;
  originalSize: number;
};
const MAX_IMAGE_SIZE = 10 * 1024 * 1024;
const MIN_IMAGE_EDGE = 640;
const imageRatioIsAccepted = (width: number, height: number) => {
  const ratio = width / height;
  return ratio >= 0.5 && ratio <= 2.4;
};
const id = ref(""),
  currentStatus = ref("draft"),
  activeSection = ref(0),
  saving = ref(false),
  loading = ref(false),
  uploadProgress = ref(""),
  images = ref<PendingImage[]>([]),
  videos = ref<string[]>([]),
  playingVideoId = ref(""),
  existingMedia = ref<PropertyMedia[]>([]),
  pendingCoverSelected = ref(false),
  reviewProperties = ref<Property[]>([]),
  stepScrollLeft = ref(0),
  reviewPerspective = ref<"customer" | "admin">("customer");
const { presentation, loadPresentation, pageStyle } = usePagePresentation(
  "adminPropertyEdit",
  "房源编辑",
  "完善资料后即可保存发布",
);
const currentLocation = ref<{ latitude: number; longitude: number } | null>(null);
const locationQuery = ref("");
const locationSearching = ref(false);
type LocationCandidate = LocationSearchItem & { saved?: boolean; source: "library" | "map" };
const locationResults = ref<LocationCandidate[]>([]);
const locationMapSearched = ref(false);
let locationSearchTimer: ReturnType<typeof setTimeout> | undefined;
let locationSearchVersion = 0;
const form = reactive<Partial<Property>>({
  title: "",
  marketingPhrase: "",
  community: "",
  location: "",
  buildingNo: "",
  unitNo: "",
  roomNo: "",
  latitude: 0,
  longitude: 0,
  layout: "",
  actualLayout: "",
  wholeLayout: "",
  rentMonthly: 0,
  feeText: "无中介费",
  rentType: "整租",
  listingSource: "直租",
  remainingLeaseMonths: 0,
  transferDeposit: 0,
  availableDate: "拎包入住",
  depositPayment: "押一付一",
  utilitiesText: "民水民电",
  areaSqm: 0,
  coverFocusX: 50,
  coverFocusY: 50,
  actualAreaSqm: 0,
  wholeAreaSqm: 0,
  floorInfo: "",
  orientation: "",
  elevator: "",
  metroMinutes: 0,
  verificationDate: "",
  rulesText: "",
  description: "",
  ownerId: "xiaopi-admin",
});
const customerDisplayTags = ref<string[]>(["layout", "rentType", "listingSource", "fee"]);
const adminDisplayTags = ref<string[]>(["unit", "layout", "rentType", "fee"]);
const previewListTagOrder = ref<string[]>(["listingSource", "rentType", "layout", "fee", "floor", "area"]);
const previewListTagTopFields = ref<string[]>(["rentType", "listingSource"]);
const roomCount = ref("1"),
  customRoomCount = ref(""),
  hallCount = ref("1"),
  rentType = ref("整租"),
  roomType = ref("主卧"),
  kitchenCount = ref("1"),
  bathroomCount = ref("1"),
  balcony = ref("0"),
  toiletType = ref("独卫"),
  bayWindow = ref("有飘窗"),
  privateBathroom = ref(true),
  hasBayWindow = ref(true);
const {
  buildingNo,
  unitNo,
  roomNo,
  selectedFloor,
  buildingRules,
  manualDoorplate,
  doorplateEntryMode,
  structureStep,
  buildingOptions,
  hasDoorplateCatalog,
  automaticFloorInfo,
  unitOptions,
  roomOptions,
  generatedRoomNo,
  loadBuildingRules,
  selectBuildingRule,
  chooseUnit,
  chooseRoom,
  canReturnToStructureStep,
  returnToStructureStep,
  returnToPreviousStructureStep,
  openStructurePicker,
  switchDoorplateEntryMode,
  structureSummary,
  confirmManualDoorplate,
} = usePropertyDoorplate(form);

const defaultAmenityOptions = ["冰箱", "洗衣机", "空调", "热水器", "电视", "宽带", "燃气", "油烟机", "沙发", "衣柜", "书桌", "床"];
const amenities = ref(["冰箱", "洗衣机", "空调", "热水器"]);
const amenityOptions = ref([...defaultAmenityOptions]);
const configuredDetailModules = ref<{ key: string; title: string; enabled: boolean }[]>([
  { key: "overview", title: "房屋概况", enabled: true },
  { key: "location", title: "位置与周边", enabled: true },
  { key: "facilities", title: "房屋设施", enabled: true },
  { key: "description", title: "房源简介", enabled: true },
  { key: "nearby", title: "附近房源", enabled: true },
]);
// Preserve data from an existing property when an operator temporarily turns a
// module off. It is hidden from the entry form, not destructively erased.
const preservedAmenities = ref<string[]>([]);
const preservedDescription = ref("");
const detailModuleEnabled = (key: string) =>
  key === "overview" || configuredDetailModules.value.find((module) => module.key === key)?.enabled !== false;
const rentTypes = ["整租", "合租"],
  roomTypes = ["主卧", "次卧", "单间", "床位"],
  toilets = ["独卫", "非独卫"],
  windows = ["有飘窗", "无飘窗"],
  paymentCycles = ["付一", "付二", "付三", "付六"],
  utilityOptions = ["民水民电", "民水民电 · 物业另计", "水电物业全包", "商水商电"];
const paymentCycle = ref("付一");
const noAgentFee = ref(true);
const transferMonthOptions = [1, 2, 3, 6, 12];
const transferDepositMultiplier = ref(1);
const transferDepositAmount = computed(() =>
  Math.max(0, Number(form.rentMonthly) || 0) * transferDepositMultiplier.value,
);
const effectiveRoomCount = computed(() => roomCount.value === "custom" ? Math.max(1, Number(customRoomCount.value) || 1) : Number(roomCount.value));
const wholeLayoutText = computed(() => [
  `${effectiveRoomCount.value}室`,
  hallCount.value !== "0" ? `${hallCount.value}厅` : "",
  kitchenCount.value === "1" ? "1厨" : "",
  bathroomCount.value !== "0" ? `${bathroomCount.value}卫` : "",
  balcony.value !== "0" ? `${balcony.value}阳台` : "",
].filter(Boolean).join(""));
type FeaturedChannel = {
  title: string;
  keyword: string;
  latitude?: number;
  longitude?: number;
  radiusKm?: number;
};
const financialHarbor = {
  title: "金融港",
  keyword: "金融港",
  latitude: 30.45604,
  longitude: 114.425337,
  radiusKm: 3,
};
const channels = ref<FeaturedChannel[]>([financialHarbor]);
const selectedChannelIndex = ref(0);
const associationRadiusKm = ref(3);
const mapExpanded = ref(false);
const selectedChannel = computed(
  () => channels.value[selectedChannelIndex.value] || financialHarbor,
);
const channelLatitude = computed(
  () => Number(selectedChannel.value.latitude) || financialHarbor.latitude,
);
const channelLongitude = computed(
  () => Number(selectedChannel.value.longitude) || financialHarbor.longitude,
);
const channelRadiusKm = computed(
  () => associationRadiusKm.value || 3,
);
const channelTitles = computed(() =>
  channels.value.map((channel) => channel.title),
);
const isLiveEditing = computed(() => ["published", "available"].includes(currentStatus.value));
const requiredReady = computed(() =>
  Boolean(
    (generatedTitle.value.trim() || String(form.title || "").trim() || String(form.community || "").trim()) &&
    form.community?.trim() &&
    form.location?.trim() &&
    form.layout?.trim() &&
    geoReady.value &&
    Number(form.rentMonthly) > 0 &&
    Boolean(form.depositPayment?.trim()) &&
    (form.listingSource !== "转租" || Number(form.remainingLeaseMonths) > 0),
  ),
);
const sectionSteps = ["位置门牌", "户型出租", "环境配套", "租金入住", "素材发布"];
const syncStepViewport = (index: number) => {
  // Keep a stable three-step window: previous, current, next. At both ends
  // the window pins to the edge, so the fifth step is never implied off-screen.
  const visibleSteps = 3;
  const stepWidth = 80;
  const firstVisible = Math.max(0, Math.min(index - 1, sectionSteps.length - visibleSteps));
  nextTick(() => {
    stepScrollLeft.value = firstVisible * stepWidth;
  });
};
const goToSection = (index: number) => {
  activeSection.value = Math.max(0, Math.min(sectionSteps.length - 1, index));
  syncStepViewport(activeSection.value);
  uni.pageScrollTo({ scrollTop: 0, duration: 220 });
};
watch(activeSection, syncStepViewport, { immediate: true });
const geoReady = computed(() =>
  Boolean(Number(form.latitude) && Number(form.longitude)),
);
const mapLatitude = computed(() =>
  geoReady.value
    ? Number(form.latitude)
    : currentLocation.value?.latitude || channelLatitude.value,
);
const mapLongitude = computed(() =>
  geoReady.value
    ? Number(form.longitude)
    : currentLocation.value?.longitude || channelLongitude.value,
);
const channelCircles = computed(() => [
  {
    latitude: channelLatitude.value,
    longitude: channelLongitude.value,
    radius: channelRadiusKm.value * 1000,
    color: "#397E7833",
    fillColor: "#397E7808",
    strokeWidth: 1,
  },
]);
const mapMarkers = computed(() =>
  geoReady.value
    ? [
        {
          id: 1,
          latitude: Number(form.latitude),
          longitude: Number(form.longitude),
          width: 30,
          height: 30,
          callout: {
            content: form.community || form.location || "房源位置",
            display: "BYCLICK",
            padding: 6,
            borderRadius: 6,
            color: "#29382d",
            bgColor: "#ffffff",
          },
        },
      ]
    : [],
);
const distanceKm = (
  latitude1: number,
  longitude1: number,
  latitude2: number,
  longitude2: number,
) => {
  const toRadians = (value: number) => (value * Math.PI) / 180;
  const latitudeDelta = toRadians(latitude2 - latitude1);
  const longitudeDelta = toRadians(longitude2 - longitude1);
  const a =
    Math.sin(latitudeDelta / 2) ** 2 +
    Math.cos(toRadians(latitude1)) *
      Math.cos(toRadians(latitude2)) *
      Math.sin(longitudeDelta / 2) ** 2;
  return 6371 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
};
const propertyDistance = computed(() =>
  geoReady.value
    ? distanceKm(
        channelLatitude.value,
        channelLongitude.value,
        Number(form.latitude),
        Number(form.longitude),
      )
    : 0,
);
const selectNearestChannel = (latitude: number, longitude: number) => {
  if (!latitude || !longitude || !channels.value.length) return;
  const candidates = channels.value
    .map((channel, index) => ({ channel, index }))
    .filter(({ channel }) => Number.isFinite(Number(channel.latitude)) && Number.isFinite(Number(channel.longitude)) && Number(channel.latitude) !== 0 && Number(channel.longitude) !== 0);
  if (!candidates.length) return;
  selectedChannelIndex.value = candidates.reduce((nearest, candidate) =>
    distanceKm(latitude, longitude, Number(candidate.channel.latitude), Number(candidate.channel.longitude)) <
    distanceKm(latitude, longitude, Number(nearest.channel.latitude), Number(nearest.channel.longitude))
      ? candidate
      : nearest,
  ).index;
};
const selectedChannelDistance = computed(() =>
  geoReady.value
    ? distanceKm(channelLatitude.value, channelLongitude.value, Number(form.latitude), Number(form.longitude))
    : 0,
);
const propertyInChannel = computed(
  () => geoReady.value && propertyDistance.value <= channelRadiusKm.value,
);
const loadChannels = async () => {
  try {
    const config = await getPublicConfig();
    associationRadiusKm.value = Number(config.relatedPropertyRadiusKm) || 3;
    customerDisplayTags.value = (config.listingTitle?.enabledFields || [])
      .filter((key) => key !== "community");
    adminDisplayTags.value = (config.listingTitle?.adminEnabledFields || [])
      .filter((key) => key !== "community");
    if (Array.isArray(config.listingTitle?.listTagOrder)) previewListTagOrder.value = config.listingTitle.listTagOrder;
    if (Array.isArray(config.listingTitle?.listTagTopFields)) previewListTagTopFields.value = config.listingTitle.listTagTopFields;
    marketingPhrases.value = (config.listingTitle?.marketingPhrases || [])
      .map((item) => String(item || "").trim())
      .filter(Boolean);
    if (config.featuredChannels?.length) {
      channels.value = config.featuredChannels;
      if (geoReady.value) selectNearestChannel(Number(form.latitude), Number(form.longitude));
    }
  } catch {
    channels.value = [financialHarbor];
  }
};
const selectPointOnMap = (event: {
  detail?: { latitude?: number; longitude?: number };
}) => {
  const latitude = Number(event.detail?.latitude);
  const longitude = Number(event.detail?.longitude);
  if (!latitude || !longitude) return;
  form.latitude = Number(latitude.toFixed(7));
  form.longitude = Number(longitude.toFixed(7));
  selectNearestChannel(form.latitude, form.longitude);
};
const runLocationSearch = async (version = ++locationSearchVersion, mapOnly = false) => {
  if (locationSearchTimer) clearTimeout(locationSearchTimer);
  locationSearchTimer = undefined;
  const keyword = locationQuery.value.trim();
  if (keyword.length < 2) {
    if (version === locationSearchVersion) {
      locationResults.value = [];
      locationSearching.value = false;
    }
    return;
  }
  locationSearching.value = true;
  locationMapSearched.value = false;
  try {
    const [catalog, locations] = mapOnly ? [{}, {}] as const : await Promise.all([
      request<Record<string, number>>("/v1/admin/doorplates").catch(() => ({})),
      request<Record<string, { address: string; latitude: number; longitude: number }>>("/v1/admin/community-locations").catch(() => ({})),
    ]);
    const savedLocations: Record<string, { address: string; latitude: number; longitude: number }> = locations;
    // Prefer the catalog standard spelling and show one suggestion for
    // punctuation/spacing variants already present in historical records.
    const namesByIdentity = new Map<string, string>();
    for (const name of [...Object.keys(catalog), ...Object.keys(locations), ...reviewProperties.value.map(item => item.community).filter(Boolean)]) {
      const identity = normalizeCommunityName(name);
      if (identity && !namesByIdentity.has(identity)) namesByIdentity.set(identity, name);
    }
    const normalizedKeyword = normalizeCommunityName(keyword);
    const matches = [...namesByIdentity.values()].filter(name => normalizeCommunityName(name).includes(normalizedKeyword));
    matches.sort((left, right) => {
      const rank = (name: string) => normalizeCommunityName(name) === normalizedKeyword ? 0 : normalizeCommunityName(name).startsWith(normalizedKeyword) ? 1 : 2;
      return rank(left) - rank(right) || left.localeCompare(right, "zh-CN");
    });
    const saved = matches.map(title => {
      const matching = reviewProperties.value.filter(item => communityNameMatches(item.community, title));
      const previous = matching.find(item => Number(item.latitude) && Number(item.longitude)) || matching[0];
      const savedLocation = Object.entries(savedLocations).find(([name]) => communityNameMatches(name, title))?.[1];
      const location = savedLocation || previous;
      const address = savedLocation?.address || previous?.location || "";
      const isPlaceholder = /QA 地图占位地址|测试环境/.test(address);
      return {
        id: `saved:${title}`, title, saved: true, source: "library" as const, district: "",
        address: isPlaceholder ? "" : address, latitude: isPlaceholder ? 0 : Number(location?.latitude) || 0,
        longitude: isPlaceholder ? 0 : Number(location?.longitude) || 0,
      };
    });
    // Doorplate rows only identify the community structure; they do not imply
    // that an address/coordinate has been recorded. Enrich exact local matches
    // from the map when location metadata is missing.
    const exactLibraryMatch = saved.find(item => normalizeCommunityName(item.title) === normalizedKeyword);
    const hasExactPosition = Boolean(exactLibraryMatch?.address && exactLibraryMatch.latitude && exactLibraryMatch.longitude);
    // Keep the dropdown useful for approximate or mistyped input: show local
    // catalog matches first, then supplement them with Tencent map candidates
    // unless the community is an exact, already-geocoded catalog match.
    const shouldSearchMap = mapOnly || !exactLibraryMatch || !hasExactPosition;
    locationMapSearched.value = shouldSearchMap;
    const mapResults = shouldSearchMap
      ? await searchLocations(keyword, channelLatitude.value, channelLongitude.value).then(items =>
          items.map(item => ({ ...item, saved: false, source: "map" as const }))).catch(error => {
          if (!saved.length) throw error;
          return [];
        })
      : [];
    if (version !== locationSearchVersion || keyword !== locationQuery.value.trim()) return;
    if (mapOnly) {
      locationResults.value = mapResults;
    } else if (exactLibraryMatch && !hasExactPosition) {
      const exactMapMatch = mapResults.find(item => normalizeCommunityName(item.title) === normalizedKeyword);
      const mergedExact = exactMapMatch
        ? [{ ...exactMapMatch, id: exactLibraryMatch.id, title: exactLibraryMatch.title }]
        : [];
      const remainingMap = mapResults.filter(item => !exactMapMatch || normalizeCommunityName(item.title) !== normalizedKeyword);
      locationResults.value = [...mergedExact, ...saved.filter(item => !exactMapMatch || item.id !== exactLibraryMatch.id), ...remainingMap];
    } else if (saved.length) {
      const mergedSaved = saved.map(item => {
        const mapMatch = mapResults.find(candidate => normalizeCommunityName(candidate.title) === normalizeCommunityName(item.title));
        if (!mapMatch || (item.address && item.latitude && item.longitude)) return item;
        return { ...mapMatch, id: item.id, title: item.title, saved: true, source: "map" as const };
      });
      const localNames = new Set(mergedSaved.filter(item => item.source === "library").map(item => normalizeCommunityName(item.title)));
      locationResults.value = [...mergedSaved, ...mapResults.filter(item => !localNames.has(normalizeCommunityName(item.title)))];
    } else {
      locationResults.value = mapResults;
    }
  } catch (error) {
    if (version !== locationSearchVersion || keyword !== locationQuery.value.trim()) return;
    locationResults.value = [];
    locationMapSearched.value = true;
    uni.showToast({
      title: error instanceof Error ? error.message : "地址搜索失败",
      icon: "none",
    });
  } finally {
    if (version === locationSearchVersion) locationSearching.value = false;
  }
};
const invalidateLocationSearch = () => {
  locationSearchVersion += 1;
  if (locationSearchTimer) clearTimeout(locationSearchTimer);
  locationSearchTimer = undefined;
  locationSearching.value = false;
  locationResults.value = [];
  locationMapSearched.value = false;
};
const normalizeCommunityName = (value: string) => value.trim().toLocaleLowerCase().replace(/[^\p{L}\p{N}]/gu, "");
const communityNameMatches = (left: string, right: string) =>
  Boolean(normalizeCommunityName(left)) && normalizeCommunityName(left) === normalizeCommunityName(right);
const locationQueryAlreadySelected = computed(() => Boolean(
  locationQuery.value.trim() && form.community &&
  normalizeCommunityName(locationQuery.value) === normalizeCommunityName(String(form.community)),
));
const canUseTypedCommunity = computed(() => {
  const keyword = locationQuery.value.trim();
  if (keyword.length < 2 || normalizeCommunityName(keyword) === normalizeCommunityName(String(form.community || ""))) return false;
  if (locationResults.value.length > 0) return false;
  return true;
});
const scheduleLocationSearch = () => {
  if (locationSearchTimer) clearTimeout(locationSearchTimer);
  const version = ++locationSearchVersion;
  locationSearching.value = false;
  locationResults.value = [];
  if (locationQuery.value.trim().length < 2) return;
  locationSearchTimer = setTimeout(() => void runLocationSearch(version), 450);
};
const searchMapLocationCandidates = () => {
  if (locationQuery.value.trim().length < 2) {
    uni.showToast({ title: "先输入小区或地点名称", icon: "none" });
    return;
  }
  void runLocationSearch(undefined, true);
};
const useTypedCommunity = async () => {
  const title = locationQuery.value.trim();
  if (title.length < 2) return;
  const changedCommunity = normalizeCommunityName(title) !== normalizeCommunityName(String(form.community || ""));
  invalidateLocationSearch();
  form.community = title;
  if (changedCommunity) {
    form.location = "";
    form.latitude = 0;
    form.longitude = 0;
    buildingRules.value = [];
    buildingNo.value = "";
    unitNo.value = "";
    roomNo.value = "";
    selectedFloor.value = "";
    form.floorInfo = "";
  }
  locationQuery.value = title;
  await loadBuildingRules();
  doorplateEntryMode.value = hasDoorplateCatalog.value ? doorplateEntryMode.value : "manual";
};
const applyLocationResult = async (item: LocationCandidate) => {
  invalidateLocationSearch();
  const changedCommunity = !communityNameMatches(form.community || "", item.title);
  form.community = item.title;
  const selectedAddress = [item.district, item.address].filter(Boolean).join(" · ");
  const mapPosition = item.source === "map";
  if (changedCommunity || (selectedAddress && (!item.saved || mapPosition || !form.location))) form.location = selectedAddress;
  if (changedCommunity || !item.saved || mapPosition || !geoReady.value) {
    form.latitude = Number(item.latitude.toFixed(7));
    form.longitude = Number(item.longitude.toFixed(7));
  }
  if (geoReady.value) selectNearestChannel(Number(form.latitude), Number(form.longitude));
  locationQuery.value = item.title;
  locationResults.value = [];
  if (changedCommunity) {
    buildingNo.value = "";
    unitNo.value = "";
    roomNo.value = "";
    selectedFloor.value = "";
    form.floorInfo = "";
  }
  await openStructurePicker();
};
const communityPositionOpen = ref(false);
const communityPositionSaving = ref(false);
const communityPosition = reactive({ address: "", latitude: "", longitude: "" });
watch(() => form.community, () => { communityPositionOpen.value = false; });
const openCommunityPosition = () => {
  const community = String(form.community || "").trim();
  if (!community) {
    uni.showToast({ title: "请先选择小区", icon: "none" });
    return;
  }
  communityPosition.address = form.location || "";
  communityPosition.latitude = geoReady.value ? String(form.latitude) : "";
  communityPosition.longitude = geoReady.value ? String(form.longitude) : "";
  communityPositionOpen.value = true;
};
const saveCommunityPosition = async () => {
  if (communityPositionSaving.value) return;
  const community = String(form.community || "").trim();
  const address = communityPosition.address.trim();
  const latitude = Number(communityPosition.latitude);
  const longitude = Number(communityPosition.longitude);
  if (!community || !address || !latitude || !longitude || Math.abs(latitude) > 90 || Math.abs(longitude) > 180) {
    uni.showToast({ title: "请填写地址和有效的经纬度", icon: "none" });
    return;
  }
  communityPositionSaving.value = true;
  try {
    await request(`/v1/admin/community-locations?community=${encodeURIComponent(community)}`, {
      method: "PUT",
      data: { address, latitude, longitude },
    });
    if (String(form.community || "").trim() !== community) return;
    form.location = address;
    form.latitude = latitude;
    form.longitude = longitude;
    selectNearestChannel(latitude, longitude);
    communityPositionOpen.value = false;
    uni.showToast({ title: "位置已保存，下次自动带入", icon: "none" });
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "保存位置失败", icon: "none" });
  } finally {
    communityPositionSaving.value = false;
  }
};
const choosePropertyLocation = () => {
	invalidateLocationSearch();
  uni.chooseLocation({
    latitude: mapLatitude.value,
    longitude: mapLongitude.value,
    success: (result) => {
      const selectedCommunity = String(result.name || "").trim();
      const changedCommunity = Boolean(
        selectedCommunity && selectedCommunity !== String(form.community || "").trim(),
      );
      if (changedCommunity) {
        form.community = selectedCommunity;
        locationQuery.value = selectedCommunity;
        locationResults.value = [];
        buildingRules.value = [];
        buildingNo.value = "";
        unitNo.value = "";
        roomNo.value = "";
        selectedFloor.value = "";
        form.floorInfo = "";

      }
      form.latitude = Number(result.latitude.toFixed(7));
      form.longitude = Number(result.longitude.toFixed(7));
      selectNearestChannel(form.latitude, form.longitude);
      form.location = result.address || selectedCommunity || form.location;
      void loadBuildingRules();
      if (form.community?.trim()) void openStructurePicker();
    },
    fail: (error) => {
      if (!String(error.errMsg || "").includes("cancel")) {
        uni.showToast({ title: "无法打开地图，请检查定位权限", icon: "none" });
      }
    },
  });
};
const mediaCount = computed(
  () =>
    existingMedia.value.filter((item) => item.type === "image").length +
    images.value.length,
);
const videoCount = computed(() => existingVideos.value.length + videos.value.length);
const existingImages = computed(() =>
  existingMedia.value.filter((item) => item.type === "image"),
);
const existingVideos = computed(() =>
  existingMedia.value.filter((item) => item.type === "video"),
);
const coverImageIndex = ref(0);
const orderedImages = computed(() => {
  const cover = images.value[coverImageIndex.value];
  return cover
    ? [cover, ...images.value.filter((_, index) => index !== coverImageIndex.value)]
    : images.value;
});
const sectionReady = computed(() => [
  Boolean(
    form.community?.trim() &&
    form.location?.trim() &&
    geoReady.value,
  ),
  Boolean(form.layout?.trim()),
  !detailModuleEnabled("facilities") || amenities.value.length > 0,
  Boolean(Number(form.rentMonthly) > 0 && form.depositPayment?.trim()),
  mediaCount.value > 0,
]);
const completionCount = computed(
  () => sectionReady.value.filter(Boolean).length,
);
const generatedTitle = computed(() => {
  const community = String(form.community || "").trim();
  return community || "选择小区后自动生成";
});
const marketingPhrases = ref<string[]>([]);
const marketingPhraseOptions = computed(() => ["不使用广告词", ...marketingPhrases.value]);
const marketingPhraseIndex = computed(() => Math.max(0, marketingPhrases.value.indexOf(String(form.marketingPhrase || "").trim()) + 1));
const customerGeneratedTitle = computed(() => {
  const base = generatedTitle.value;
  const phrase = String(form.marketingPhrase || "").trim();
  return phrase ? `${base} · ${phrase}` : base;
});
const chooseMarketingPhrase = (event: { detail?: { value?: string | number } }) => {
  form.marketingPhrase = marketingPhrases.value[Number(event.detail?.value) - 1] || "";
};
const displayTagValue = (key: string) => {
  if (key === "unit") return [buildingNo.value, unitNo.value && `${unitNo.value}单元`, roomNo.value && `${roomNo.value}室`].filter(Boolean).join("");
  if (key === "floor") return form.floorInfo || (selectedFloor.value ? `${selectedFloor.value}层` : "");
  if (key === "layout") return form.layout || (rentType.value === "合租" ? roomType.value : wholeLayoutText.value);
  if (key === "rentType") return rentType.value;
  if (key === "listingSource") return form.listingSource || "直租";
  if (key === "fee") return form.feeText || "无中介费";
  if (key === "area") return Number(form.areaSqm) ? `${form.areaSqm}㎡` : "";
  return "";
};
const tagLabels = (fields: string[], keys: string[]) => keys.filter((key) => fields.includes(key)).map((key) => displayTagValue(key)).filter(Boolean);
const previewTagFields = computed(() => previewListTagOrder.value.filter((key) => customerDisplayTags.value.includes(key)));
const previewPrimaryTagFields = computed(() => previewTagFields.value.filter((key) => previewListTagTopFields.value.includes(key)));
const previewSecondaryTagFields = computed(() => previewTagFields.value.filter((key) => !previewListTagTopFields.value.includes(key)));
const previewPrimaryTags = computed(() => tagLabels(customerDisplayTags.value, previewPrimaryTagFields.value));
const previewSecondaryTags = computed(() => tagLabels(customerDisplayTags.value, previewSecondaryTagFields.value));
// Admin cards use the same public information tag stack as the formal admin
// list. The admin-only configuration controls whether the doorplate appears.
const previewAdminPrimaryTags = computed(() => previewPrimaryTags.value);
const previewAdminSecondaryTags = computed(() => previewSecondaryTags.value);
const reviewCoverUrl = computed(() => {
  const pending = orderedImages.value[0]?.path;
  if (pending) return pending;
  const existing = existingMedia.value.find((item) => item.type === "image")?.url;
  return existing ? resolveAssetUrl(existing) : "";
});
const reviewLocation = computed(() => {
  const raw = String(form.location || "").trim();
  if (!raw) return "位置待补充";
  const district = raw.split(/[·・]/).map((part) => part.trim()).find((part) => /区$/.test(part));
  const landmark = raw.match(/[^省市区\s·・]*(?:大道|一路|二路|三路|路|街道|街|商圈)/)?.[0];
  return [district, landmark].filter(Boolean).join(" · ") || raw.split(/[·・]/)[0].trim();
});
const reviewReferenceProperties = computed(() => {
  const live = reviewProperties.value
    .filter((item) => item.id !== id.value && ["published", "available", "rented"].includes(item.status))
    .filter((item) => item.media?.some((media) => media.type === "image"));
  const currentArea = String(form.areaLabel || "").trim();
  return [...live]
    .sort((left, right) => Number(String(right.areaLabel || "") === currentArea) - Number(String(left.areaLabel || "") === currentArea))
    .slice(0, 2);
});
const referenceCoverUrl = (item: Property) => {
  const media = item.media?.find((entry) => entry.type === "image");
  return media?.url ? resolveAssetUrl(media.url) : "";
};
const referenceLocation = (item: Property) => String(item.areaLabel || item.location || "位置待补充").trim();
const referenceTags = (item: Property, fields: string[]) => fields
  .map((field) => listingTagValue(item, field))
  .filter(Boolean);
watch([() => form.community], () => {
  form.title = String(form.community || "").trim()
    ? generatedTitle.value
    : "";
}, { immediate: true });
const syncLayout = () => {
  toiletType.value = privateBathroom.value ? "独卫" : "非独卫";
  bayWindow.value = hasBayWindow.value ? "有飘窗" : "无飘窗";
  if (rentType.value === "整租") {
    form.layout = wholeLayoutText.value;
    form.actualLayout = "";
    form.actualAreaSqm = 0;
    form.wholeLayout = "";
    form.wholeAreaSqm = 0;
  } else if (!String(form.layout || "").trim() || form.rentType !== "合租") {
    form.layout = [roomType.value, toiletType.value, bayWindow.value].filter(Boolean).join(" · ");
  }
  form.rentType = rentType.value;
};
const pickLayoutValue = (values: string[], index: string | number) =>
  values[Number(index)] || values[0];
const setRentType = (value: string | number) => {
  rentType.value = typeof value === "number" || /^\d+$/.test(String(value))
    ? pickLayoutValue(rentTypes, value)
    : String(value) as "整租" | "合租";
  if (rentType.value === "合租" && (!roomType.value || roomType.value === "整套")) roomType.value = "主卧";
  syncLayout();
};
const setListingSource = (source: "直租" | "转租") => {
  form.listingSource = source;
  if (source === "转租" && !form.remainingLeaseMonths) form.remainingLeaseMonths = 3;
  if (source === "转租" && !transferDepositMultiplier.value) transferDepositMultiplier.value = 1;
  if (source === "直租") {
    form.remainingLeaseMonths = 0;
    form.transferDeposit = 0;
  }
};
const setTransferMonths = (months: number) => {
  form.remainingLeaseMonths = Math.min(36, Math.max(1, Math.round(months || 1)));
};
const handleTransferMonthsInput = (event: { detail: number | string | { value?: number | string } }) => {
  const rawValue = typeof event.detail === "object" ? event.detail.value : event.detail;
  setTransferMonths(Number(rawValue));
};
const adjustTransferMonths = (delta: number) => setTransferMonths(Number(form.remainingLeaseMonths || 1) + delta);
const setTransferDeposit = (multiplier: number) => {
  transferDepositMultiplier.value = multiplier;
  form.transferDeposit = (Number(form.rentMonthly) || 0) * multiplier;
};
const setSharedRoomType = (value: string) => {
  roomType.value = value;
  syncLayout();
};
const setPaymentCycle = (cycle: string) => {
  paymentCycle.value = cycle;
  form.depositPayment = `押一${cycle}`;
};
const setUtilities = (index: string | number) => {
  form.utilitiesText = pickLayoutValue(utilityOptions, index);
};
const toggleNoAgentFee = (event: { detail?: { value?: string[] } }) => {
  noAgentFee.value = Boolean(event.detail?.value?.length);
  form.feeText = noAgentFee.value ? "无中介费" : "有中介费";
};
const toggleElevator = (event: { detail?: { value?: string[] } }) => {
  form.elevator = event.detail?.value?.length ? "有电梯" : "";
};
const togglePrivateBathroom = (event: { detail?: { value?: string[] } }) => {
  privateBathroom.value = Boolean(event.detail?.value?.length);
  syncLayout();
};
const toggleBayWindow = (event: { detail?: { value?: string[] } }) => {
  hasBayWindow.value = Boolean(event.detail?.value?.length);
  syncLayout();
};
const parseLayout = (value = "", rentTypeHint = "") => {
  const parts = value.split(/\s*[·・|｜]\s*/).map((part) => part.trim()).filter(Boolean);
  const legacyRentType = parts.find((part) => part === "整租" || part === "合租");
  const resolvedRentType = rentTypeHint === "整租" || rentTypeHint === "合租"
    ? rentTypeHint
    : legacyRentType || (parts.some((part) => roomTypes.includes(part)) ? "合租" : "整租");
  if (resolvedRentType === "整租") {
    const roomHall = parts[0] || "";
    const numberOf = (suffix: string, fallback: number) => Number(roomHall.match(new RegExp(`(\\d+)${suffix}`))?.[1] || fallback);
    const legacyNumber = (value: string, fallback: number) => ({ 一: 1, 两: 2, 三: 3, 四: 4, 五: 5 }[value[0]] || fallback);
    const roomMatch = roomHall.match(/(\d+|[一二三四五])室/);
    roomCount.value = String(roomMatch ? (/\d/.test(roomMatch[1]) ? Number(roomMatch[1]) : legacyNumber(roomMatch[1], 1)) : 1);
    hallCount.value = String(numberOf("厅", roomHall.includes("无厅") ? 0 : 1));
    kitchenCount.value = String(numberOf("厨", roomHall.includes("厨") || parts.some((part) => part.includes("厨房")) ? 1 : 0));
    bathroomCount.value = String(numberOf("卫", 1));
    balcony.value = String(numberOf("阳台", roomHall.includes("阳台") || parts.some((part) => part.includes("阳台")) ? 1 : 0));
    rentType.value = "整租";
  } else {
    rentType.value = "合租";
    roomType.value = parts.find((part) => roomTypes.includes(part)) || "主卧";
    toiletType.value = parts.find((part) => part === "独卫" || part === "非独卫") || "独卫";
    bayWindow.value = parts.find((part) => part === "有飘窗" || part === "无飘窗") || "有飘窗";
    privateBathroom.value = toiletType.value === "独卫";
    hasBayWindow.value = bayWindow.value === "有飘窗";
  }
};
const loadProperty = async () => {
  if (!id.value) {
    reviewProperties.value = await listProperties().catch(() => []);
    return;
  }
  loading.value = true;
  try {
    const properties = await listProperties();
    reviewProperties.value = properties;
    const property = properties.find(
      (item) => item.id === id.value,
    );
    if (!property) throw new Error("房源不存在");
    Object.assign(form, property);
    if (!geoReady.value && property.community) {
      const locations = await request<Record<string, { address: string; latitude: number; longitude: number }>>("/v1/admin/community-locations").catch(() => null);
      const savedPosition = Object.entries(locations || {}).find(([name]) => communityNameMatches(name, property.community))?.[1];
      if (savedPosition && form.community === property.community && !geoReady.value) {
        form.location = form.location?.trim() || savedPosition.address;
        form.latitude = savedPosition.latitude;
        form.longitude = savedPosition.longitude;
      }
    }
    paymentCycle.value = paymentCycles.find((item) => String(property.depositPayment || "").includes(item)) || "付一";
    form.listingSource = property.listingSource || "直租";
    transferDepositMultiplier.value = Number(property.transferDeposit) && Number(property.rentMonthly)
      ? Math.max(1, Math.round(Number(property.transferDeposit) / Number(property.rentMonthly)))
      : 1;
    noAgentFee.value = !String(property.feeText || "").includes("有中介费") || String(property.feeText || "").includes("无中介费");
    buildingNo.value = property.buildingNo || "";
    unitNo.value = property.unitNo || "";
    roomNo.value = property.roomNo || "";
    selectedFloor.value = /^\d{3,}$/.test(roomNo.value) ? roomNo.value.slice(0, -2) : "";
    locationQuery.value = property.community || property.location || "";
    currentStatus.value = property.status;
    existingMedia.value = property.media || [];
    parseLayout(property.layout, property.rentType);
    const boundChannelIndex = channels.value.findIndex((channel) => channel.title === property.areaLabel);
    if (boundChannelIndex >= 0) selectedChannelIndex.value = boundChannelIndex;
    else if (geoReady.value) selectNearestChannel(Number(form.latitude), Number(form.longitude));
    const match = property.description.match(/^配套设施：([^\n]*)\n?/);
    preservedAmenities.value = match?.[1] ? match[1].split("、").filter(Boolean) : [];
    preservedDescription.value = property.description.replace(/^配套设施：[^\n]*\n?/, "");
    amenities.value = detailModuleEnabled("facilities") ? [...preservedAmenities.value] : [];
    form.description = detailModuleEnabled("description") ? preservedDescription.value : "";
    await loadBuildingRules();
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "加载失败",
      icon: "none",
    });
  } finally {
    loading.value = false;
  }
};
const loadFacilityCatalog = async () => {
  const config = await getPublicConfig().catch(() => null);
  if (config?.propertyDetail?.modules?.length) {
    configuredDetailModules.value = config.propertyDetail.modules;
  }
  const options = config?.propertyDetail?.facilities
    ?.filter((facility) => facility.enabled)
    .map((facility) => facility.label)
    .filter(Boolean);
  amenityOptions.value = detailModuleEnabled("facilities")
    ? (options ? Array.from(new Set(options)) : [...defaultAmenityOptions])
    : [];
};
onLoad(async (query) => {
  permissionAudit("onLoad", "property-editor");
  const allowed = hasRole("admin");
  permissionAudit("guard", "property-editor", allowed);
  if (!allowed) {
    uni.showToast({ title: "暂无运营权限", icon: "none" });
    permissionAudit("redirect", "property-editor", "pages/me/index");
    uni.switchTab({ url: "/pages/me/index" });
    return;
  }
  id.value = String(query?.id || "");
  await Promise.all([loadPresentation(), loadFacilityCatalog()]);
  syncLayout();
  await loadChannels();
  await loadProperty();
  uni.getLocation({
    type: "gcj02",
    success: (result) => {
      currentLocation.value = {
        latitude: Number(result.latitude.toFixed(7)),
        longitude: Number(result.longitude.toFixed(7)),
      };
    },
  });
});
onShow(() => {
  permissionAudit("onShow", "property-editor");
  const session = getSession();
  permissionAudit("guard", "property-editor-crop", session?.role === "admin");
  if (session?.role !== "admin") return;
  const result = readPrivateCache<{ index?: number; path?: string; width?: number; height?: number; size?: number }>(
    session, "admin", "xiaopi-property-image-crop-result", key => uni.getStorageSync(key),
    present => permissionAudit("private-read", "property-crop-cache", present));
  if (!result?.path || typeof result.index !== "number") return;
  const image = images.value[result.index];
  if (image) {
    images.value.splice(result.index, 1, {
      ...image,
      path: result.path,
      width: Number(result.width) || 1200,
      height: Number(result.height) || 900,
      size: Number(result.size) || image.size,
    });
    images.value = [...images.value];
    uni.showToast({ title: "已更新裁切图片", icon: "success" });
  }
  uni.removeStorageSync("xiaopi-property-image-crop-result");
  uni.removeStorageSync("xiaopi-property-image-crop-result:owner");
});
const pick = (type: "image" | "video") => {
  const count =
    type === "image"
      ? Math.max(0, 9 - mediaCount.value)
      : 1 - videoCount.value;
  if (count <= 0) {
    uni.showToast({
      title: type === "image" ? "最多上传 9 张图片" : "最多上传 1 个视频",
      icon: "none",
    });
    return;
  }
  uni.chooseMedia({
    count,
    mediaType: [type],
    sourceType: ["album", "camera"],
    maxDuration: 60,
    sizeType: ["compressed"],
    success: (res) => {
      void processPickedMedia(type, res.tempFiles as any[]);
    },
  });
};
const imageInfo = (path: string) =>
  new Promise<{ width: number; height: number }>((resolve, reject) =>
    uni.getImageInfo({
      src: path,
      success: (result) => resolve({ width: result.width, height: result.height }),
      fail: reject,
    }),
  );
const processPickedMedia = async (type: "image" | "video", files: any[]) => {
  if (type === "video") {
    const video = files[0];
    if (!video) return;
    if (Number(video.size || 0) > 20 * 1024 * 1024) {
      uni.showToast({ title: "视频需小于20MB", icon: "none" });
      return;
    }
    if (Number(video.duration || 0) > 60) {
      uni.showToast({ title: "视频最长60秒", icon: "none" });
      return;
    }
    videos.value = [video.tempFilePath];
    return;
  }
  const accepted: PendingImage[] = [];
  let rejected = 0;
  for (const file of files) {
    const path = String(file.tempFilePath || "");
    if (!path || Number(file.size || 0) > MAX_IMAGE_SIZE) {
      rejected++;
      continue;
    }
    try {
      const info = await imageInfo(path);
      if (
        Math.min(info.width, info.height) < MIN_IMAGE_EDGE ||
        !imageRatioIsAccepted(info.width, info.height)
      ) {
        rejected++;
        continue;
      }
      accepted.push({
        path,
        width: info.width,
        height: info.height,
        size: Number(file.size || 0),
        originalPath: path,
        originalWidth: info.width,
        originalHeight: info.height,
        originalSize: Number(file.size || 0),
      });
    } catch {
      rejected++;
    }
  }
  if (accepted.length) {
    images.value = [...images.value, ...accepted];
  }
  if (rejected) {
    uni.showToast({
      title: "已跳过不合规格图片",
      icon: "none",
    });
  }
};
const cropImage = (index: number) => {
  const image = images.value[index];
  if (!image) return;
  uni.setStorageSync("xiaopi-property-image-crop-source", {
    index,
    path: image.originalPath || image.path,
    width: image.originalWidth || image.width,
    height: image.originalHeight || image.height,
    size: image.originalSize || image.size,
  });
  uni.navigateTo({ url: "/pages/admin/image-crop" });
};
const previewPickedImages = (index: number) => {
  const current = images.value[index]?.path;
  if (!current) return;
  uni.previewImage({ current, urls: images.value.map((image) => image.path) });
};
const previewExistingImage = (url: string) => {
  const urls = existingImages.value.map((item) => mediaUrl(item.url));
  uni.previewImage({ current: mediaUrl(url), urls });
};
const toggleVideo = (videoId: string) => {
  const context = uni.createVideoContext(videoId);
  if (playingVideoId.value === videoId) {
    context.pause();
    playingVideoId.value = "";
    return;
  }
  if (playingVideoId.value) uni.createVideoContext(playingVideoId.value).pause();
  context.play();
  playingVideoId.value = videoId;
};
const markVideoPlaying = (videoId: string) => {
  if (playingVideoId.value && playingVideoId.value !== videoId)
    uni.createVideoContext(playingVideoId.value).pause();
  playingVideoId.value = videoId;
};
const markVideoPaused = (videoId: string) => {
  if (playingVideoId.value === videoId) playingVideoId.value = "";
};
const setCover = (index: number) => {
  if (index !== 0) {
    const [cover] = images.value.splice(index, 1);
    images.value.unshift(cover);
  }
  coverImageIndex.value = 0;
  pendingCoverSelected.value = true;
  uni.showToast({ title: "已设为封面" });
};
const moveImage = (index: number, direction: -1 | 1) => {
  const target = index + direction;
  if (target < 0 || target >= images.value.length) return;
  const items = [...images.value];
  [items[index], items[target]] = [items[target], items[index]];
  images.value = items;
  if (coverImageIndex.value === index) coverImageIndex.value = target;
  else if (coverImageIndex.value === target) coverImageIndex.value = index;
};
const removeImage = (index: number) => {
  images.value.splice(index, 1);
  if (coverImageIndex.value >= images.value.length)
    coverImageIndex.value = Math.max(0, images.value.length - 1);
};
const removeExistingMedia = (id: string) => {
  existingMedia.value = existingMedia.value.filter((item) => item.id !== id);
};
const moveExistingImage = (id: string, direction: -1 | 1) => {
  const imagesOnly = existingMedia.value.filter((item) => item.type === "image");
  const index = imagesOnly.findIndex((item) => item.id === id);
  const target = index + direction;
  if (index < 0 || target < 0 || target >= imagesOnly.length) return;
  [imagesOnly[index], imagesOnly[target]] = [imagesOnly[target], imagesOnly[index]];
  existingMedia.value = [
    ...imagesOnly,
    ...existingMedia.value.filter((item) => item.type !== "image"),
  ];
};
const setExistingCover = (id: string) => {
  const imagesOnly = existingMedia.value.filter((item) => item.type === "image");
  const target = imagesOnly.find((item) => item.id === id);
  if (!target) return;
  existingMedia.value = [
    target,
    ...imagesOnly.filter((item) => item.id !== id),
    ...existingMedia.value.filter((item) => item.type !== "image"),
  ];
  pendingCoverSelected.value = false;
  uni.showToast({ title: "已设为封面" });
};
const upload = (path: string, type: string, propertyId: string, sort: number) =>
  new Promise<PropertyMedia>((resolve, reject) =>
    uni.uploadFile({
      url: safeUploadUrl(`${API_ORIGIN}/api/v1/media/upload`),
      filePath: path,
      name: "file",
      header: { Authorization: `Bearer ${getSession()?.accessToken || ""}` },
      formData: { propertyId, mediaType: type, sort: String(sort) },
      success: (res) => {
        let message = "素材上传失败";
        try {
          const body = JSON.parse(String(res.data || "{}"));
          if (body?.message) message = body.message;
          if (res.statusCode >= 200 && res.statusCode < 300 && body?.code === 0 && body?.data?.id) {
            resolve(body.data as PropertyMedia);
            return;
          }
        } catch {}
        reject(new Error(message));
      },
      fail: (error) => reject(new Error(error.errMsg || "素材上传失败")),
    }),
  );
const completedUploads = new Map<string, PropertyMedia>();
type SaveMode = "draft" | "preview" | "publish";
const save = async (mode: SaveMode = "draft") => {
  if (saving.value) return;
  const finalTitle = generatedTitle.value.trim() || String(form.title || "").trim() || String(form.community || "").trim();
  const finalCommunity = String(form.community || "").trim();
  const finalRent = Number(form.rentMonthly);
  const minimumDraftReady = Boolean(finalTitle && finalCommunity && Number.isFinite(finalRent) && finalRent >= 0);
  if (!minimumDraftReady) {
    uni.showToast({
      title: "草稿至少填写标题和小区",
      icon: "none",
    });
    return;
  }
  if (mode !== "draft" && !requiredReady.value) {
    uni.showToast({
      title: "请完成标题、小区、位置、地图定位、户型和月租",
      icon: "none",
    });
    return;
  }
  if (mode !== "draft" && mediaCount.value === 0) {
    uni.showToast({
      title: "发布前请至少上传一张实拍图片",
      icon: "none",
    });
    return;
  }
  saving.value = true;
  uploadProgress.value = "保存房源信息";
  try {
    const details = detailModuleEnabled("description")
      ? String(form.description || "").trim()
      : preservedDescription.value;
    const selectedAmenities = detailModuleEnabled("facilities")
      ? amenities.value
      : preservedAmenities.value;
    // List tags are now global presentation rules from the admin backend.
    // Do not carry a legacy per-property displayTags value back into updates.
    const { displayTags: _legacyDisplayTags, ...propertyFields } = form;
    const payload = {
      ...propertyFields,
      title: finalTitle,
      marketingPhrase: String(form.marketingPhrase || "").trim(),
      listingSource: form.listingSource || "直租",
      actualLayout: "",
      actualAreaSqm: 0,
      wholeLayout: rentType.value === "合租" ? String(form.wholeLayout || "").trim() : "",
      wholeAreaSqm: rentType.value === "合租" ? Number(form.wholeAreaSqm) || 0 : 0,
      remainingLeaseMonths: form.listingSource === "转租" ? Number(form.remainingLeaseMonths) || 0 : 0,
      transferDeposit: form.listingSource === "转租" ? transferDepositAmount.value : 0,
      community: finalCommunity,
      location: form.location?.trim(),
      areaLabel: selectedChannel.value.title,
      buildingNo: buildingNo.value.trim(),
      unitNo: unitNo.value.trim(),
      roomNo: roomNo.value.trim(),
      description: [selectedAmenities.length ? `配套设施：${selectedAmenities.join("、")}` : "", details].filter(Boolean).join("\n"),
    };
    const property = id.value
      ? await updateProperty({ ...payload, id: id.value })
      : await createProperty(payload);
    id.value = property.id;
    const queue = [
      ...orderedImages.value.map((image, index) => ({
        path: image.path,
        type: "image",
        sort: existingMedia.value.length + index,
      })),
      ...videos.value.map((path) => ({
        path,
        type: "video",
        sort: existingMedia.value.length + images.value.length,
      })),
    ];
    const uploaded: PropertyMedia[] = [];
    for (let index = 0; index < queue.length; index++) {
      uploadProgress.value = `上传素材 ${index + 1}/${queue.length}`;
      const item = queue[index];
      const key = JSON.stringify([property.id, item.type, item.path]);
      let media = completedUploads.get(key);
      if (!media) {
        media = await upload(item.path, item.type, property.id, item.sort);
        completedUploads.set(key, media);
      }
      uploaded.push(media);
    }
    const uploadedImages = uploaded.filter((item) => item.type === "image");
    const uploadedVideos = uploaded.filter((item) => item.type === "video");
    const existingImages = existingMedia.value.filter((item) => item.type === "image");
    const existingVideos = existingMedia.value.filter((item) => item.type === "video");
    // Video is the lead media whenever it exists, so list cards and the detail
    // gallery agree on what the customer sees first.
    const finalImages = pendingCoverSelected.value
      ? [...uploadedImages, ...existingImages]
      : [...existingImages, ...uploadedImages];
    const finalMedia = [...existingVideos, ...uploadedVideos, ...finalImages];
    await replacePropertyMedia(property.id, finalMedia.map((item) => item.id));
    completedUploads.clear();
    existingMedia.value = finalMedia;
    images.value = [];
    coverImageIndex.value = 0;
    pendingCoverSelected.value = false;
    videos.value = [];
    if (mode === "publish") {
      uploadProgress.value = "正在发布房源";
      await request("/v1/properties/status", {
        method: "PATCH",
        data: { propertyId: property.id, status: "available", note: "房源编辑页直接发布" },
      });
      currentStatus.value = "available";
    } else {
      currentStatus.value = property.status;
    }
    const savedAsLive = ["published", "available"].includes(property.status);
    uni.showModal({
      title: mode === "publish" ? "房源已发布" : mode === "preview" ? (savedAsLive ? "修改已保存" : "草稿已保存") : (savedAsLive ? "修改已保存" : "草稿已保存"),
      content: mode === "publish" ? "该房源已上架为可租状态。" : mode === "preview" ? (savedAsLive ? "现在查看用户可见的公开效果。" : "现在以草稿预览核对房源详情。") : (savedAsLive ? "房源继续保持可租状态。" : "可随时返回继续编辑。"),
      showCancel: false,
      success: () => {
        if (mode === "preview" || mode === "publish") {
          uni.navigateTo({ url: `/pages/properties/detail?id=${property.id}` });
        }
      },
    });
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "保存失败",
      icon: "none",
    });
  } finally {
    saving.value = false;
    uploadProgress.value = "";
  }
};
const confirmPublish = () => {
  if (saving.value) return;
  if (!geoReady.value) {
    goToSection(0);
    openCommunityPosition();
    return;
  }
  if (!requiredReady.value || mediaCount.value === 0) {
    uni.showToast({ title: "请先完成必填信息并上传至少一张实拍图", icon: "none" });
    return;
  }
  uni.showModal({
    title: "确认直接发布",
    content: "发布后将立即以“可租”状态展示给用户，仍可在房源管理中编辑或下架。",
    confirmText: "确认发布",
    confirmColor: "#40543f",
    success: (result) => {
      if (result.confirm) save("publish");
    },
  });
};
const deleteDraft = () => {
  if (!id.value || currentStatus.value !== "draft") return;
  uni.showModal({
    title: "删除草稿",
    content: "草稿及其已上传的图片、视频将被永久删除，无法恢复。",
    confirmText: "删除",
    confirmColor: "#c45843",
    success: async (result) => {
      if (!result.confirm || !id.value) return;
      try {
        await deleteProperty(id.value);
        uni.showToast({ title: "草稿已删除" });
        setTimeout(() => uni.navigateBack(), 450);
      } catch (error) {
        uni.showToast({
          title: error instanceof Error ? error.message : "删除失败",
          icon: "none",
        });
      }
    },
  });
};
const mediaUrl = resolveAssetUrl;
if (import.meta.env.MODE === "test") {
  watchPostEffect(() => permissionAudit("render-state", "property-editor", Boolean(form.id || existingMedia.value.length)));
}

function copyText(fallback: string) { return presentation.value?.copy?.[fallback] || fallback }
</script>
<template>
  <view class="page editor-page" :style="pageStyle"
    ><view class="page-presentation-head"><view class="page-presentation-title">{{ presentation.title }}</view><text>{{ presentation.subtitle }}</text></view
    ><view class="editor-hero"
      ><view
        ><text>{{ id ? copyText("完善房源信息") : copyText("开始录入房源") }}</text></view
      ><view class="completion"
        ><strong>{{ completionCount }}/5</strong><small>已完成</small></view
      ></view
    ><scroll-view class="step-nav" scroll-x :show-scrollbar="false" :scroll-left="stepScrollLeft" :scroll-with-animation="true"
      ><view class="step-track"
        ><view
          v-for="(step, index) in sectionSteps"
          :key="step"
          class="step-item"
          :class="{
            active: activeSection === index,
            done: sectionReady[index],
          }"
          @click="goToSection(index)"
          ><text>{{ sectionReady[index] ? "✓" : index + 1 }}</text
          ><small>{{ step }}</small></view
        ></view
      ></scroll-view
    ><view v-if="loading" class="card loading">正在加载房源...</view
    ><template v-if="!loading"
      ><view v-show="activeSection === 0" class="section-card"
        ><view class="section-head"
          ><text>01</text
          ><view
            ><strong>基础信息</strong></view
          ></view
        ><view class="map-section-heading community-heading"
          ><view><strong>小区名称 *</strong><small>优先匹配小区库；库外小区可搜索或地图选点</small></view></view
        ><view class="location-search community-search"
          ><view class="location-search-box"
            ><text>⌕</text
            ><input
              v-model.trim="locationQuery"
              :placeholder='copyText("输入小区名称、街道或写字楼")'
              confirm-type="search"
              @input="scheduleLocationSearch"
              @confirm="runLocationSearch()"
            /><small v-if="locationSearching">搜索中</small><text v-else class="location-search-action" @click="runLocationSearch()">搜索</text><text class="location-map-action" @click="searchMapLocationCandidates">地图选点</text></view
          ><view v-if="locationResults.length" class="location-results"
            ><view
              v-for="item in locationResults"
              :key="item.id"
              @click="applyLocationResult(item)"
              ><view
                ><strong>{{ item.title }} <text class="location-source">{{ item.saved && item.source === "map" ? "小区库 · 地图定位" : item.source === "library" ? "小区库" : "地图" }}</text></strong
                ><small>{{ [item.district, item.address].filter(Boolean).join(" · ") || (item.saved ? "库内小区 · 位置待补" : "") }}</small></view
              ><text v-if="item.distance"
                >{{ (item.distance / 1000).toFixed(1) }}km</text
			></view
            ></view
          ><view v-if="canUseTypedCommunity" class="location-custom-option"><view><strong>不在小区库？使用“{{ locationQuery.trim() }}”</strong><small>确认名称后，可单独选择门牌或地图位置</small></view><text @click="useTypedCommunity">使用名称 ›</text></view
          ><view v-else-if="locationQuery.length >= 2 && !locationSearching && !locationQueryAlreadySelected && !locationResults.some(item => item.source === 'map')" class="location-search-tip"
            >{{ locationMapSearched ? "地图暂未找到该地点，可直接在地图上选点" : "小区库暂无匹配，正在切换地图搜索" }}</view
          ><text v-if="locationQuery.length >= 2 && !locationSearching && !locationQueryAlreadySelected && !locationResults.some(item => item.source === 'map')" class="location-map-fallback" @click="runLocationSearch(undefined, true)">{{ locationMapSearched ? "重新搜索地图 ›" : "地图搜索 ›" }}</text
          ><view v-if="form.community" class="selected-community"><strong>已选择：{{ form.community }}</strong><view class="selected-location-state" :class="{ ready: geoReady }"><text>{{ geoReady ? "✓ 位置已获取" : "尚未获取位置" }}</text><small>{{ geoReady ? (form.location?.trim() || "地图未返回文字地址，可通过坐标确认位置") : "点击地图选点后，这里会显示地址和坐标" }}</small><small v-if="geoReady">坐标：{{ Number(form.latitude).toFixed(6) }}, {{ Number(form.longitude).toFixed(6) }}</small></view></view></view
        ><view v-if="form.community" class="unit-selector-card" :class="{ selected: Boolean(roomNo) }" @click="openStructurePicker"
          ><view><strong>{{ roomNo ? "已选门牌" : "门牌信息" }}</strong><text>{{ structureSummary }}</text></view><b>{{ roomNo ? "重选 ›" : "选择 ›" }}</b></view
        ><view class="field"
          ><text>客户广告词</text
          ><picker :range="marketingPhraseOptions" :value="marketingPhraseIndex" @change="chooseMarketingPhrase"
            ><view class="channel-picker marketing-phrase-picker"><view><strong>{{ form.marketingPhrase || "不使用广告词" }}</strong><small>仅用于客户标题；管理员仅显示房源原始数据</small></view><text>选择 ›</text></view></picker
          ></view
        ><view class="field"
          ><text>详细地址补充</text
          ><input
            v-model.trim="form.location"
            maxlength="60"
            :placeholder='copyText("选择小区后自动回填，可补充门牌信息")' /></view
        ><view class="channel-scope"
          ><view class="scope-heading"><text>归属地区</text><small>保存后按当前选择归类</small></view
          ><picker
            :range="channelTitles"
            :value="selectedChannelIndex"
            @change="selectedChannelIndex = Number($event.detail.value)"
            ><view class="channel-picker auto-channel-picker"
              ><view
                ><strong>{{ selectedChannel.title }}</strong
                ><small
                  >{{ geoReady ? `当前绑定：${selectedChannel.title}` : "选择小区后推荐最近地区" }}</small
                ></view
              ><text>调整 ›</text></view
            ></picker
          ><view v-if="geoReady" class="map-toggle" @click="mapExpanded = !mapExpanded"><text>{{ mapExpanded ? "收起地图" : "地图调整位置" }}</text><small>地图仅用于确认房源坐标</small></view></view
        ><view v-if="form.community && (!geoReady || communityPositionOpen)" class="field community-position"
          ><button v-if="!communityPositionOpen" @click="openCommunityPosition">补齐小区位置</button
          ><template v-else>
            <text>小区位置 · 保存后可复用</text>
            <input v-model="communityPosition.address" maxlength="60" placeholder="小区地址" />
            <input v-model="communityPosition.latitude" type="digit" placeholder="纬度（GCJ-02）" />
            <input v-model="communityPosition.longitude" type="digit" placeholder="经度（GCJ-02）" />
            <button :loading="communityPositionSaving" @click="saveCommunityPosition">保存小区位置</button>
            <button :disabled="communityPositionSaving" @click="communityPositionOpen = false">取消</button>
          </template></view
        ><view v-if="!geoReady || mapExpanded" class="map-picker"
          ><map
            class="property-map"
            :latitude="mapLatitude"
            :longitude="mapLongitude"
            :markers="mapMarkers"
              :circles="geoReady ? [] : channelCircles"
            :scale="geoReady ? 15 : 11"
            show-location
            @tap="selectPointOnMap"
          /><view class="map-picker-footer"
            ><view
              ><strong>{{ geoReady ? `归属地区：${selectedChannel.title}` : "请先选择小区位置" }}</strong
              ><small v-if="geoReady"
                >房源坐标 {{ Number(form.latitude).toFixed(6) }},
                {{ Number(form.longitude).toFixed(6) }}</small
              ><small v-else
                >地图以当前位置显示，也可以搜索小区或直接点击落点</small
              ></view
            ><text class="map-hint">轻触地图调整位置</text></view
          ></view
        ></view
      ><view v-show="activeSection === 1" class="section-card"
        ><view class="section-head"
          ><text>02</text
          ><view
            ><strong>出租方式与户型</strong></view
          ></view
        ><view class="rent-type-tabs"><view :class="{ active: rentType === '整租' }" @click="setRentType('整租')"><strong>整租</strong><small>发布整套房源</small></view><view :class="{ active: rentType === '合租' }" @click="setRentType('合租')"><strong>合租</strong><small>发布单个房间</small></view></view
        ><view class="listing-source"><text>出租来源</text><view class="listing-source-options"><view :class="{ active: form.listingSource === '直租' }" @click="setListingSource('直租')"><text>{{ form.listingSource === '直租' ? '✓' : '' }}</text>直租</view><view :class="{ active: form.listingSource === '转租' }" @click="setListingSource('转租')"><text>{{ form.listingSource === '转租' ? '✓' : '' }}</text>转租</view></view></view
        ><view class="layout-guidance">{{ rentType === "整租" ? "户型与面积就是整套房源的实际使用空间，会同步到列表标签。" : "户型与面积填写本次出租房间的实际使用空间，会同步到列表标签；整套参数仅用于详情概况。" }}</view
        ><view class="picker-grid"
          ><template v-if="rentType === '整租'"><view class="whole-rental-options"><view class="whole-option-row"><text>房间</text><view><button v-for="count in [1, 2, 3]" :key="count" :class="{ active: roomCount === String(count) }" @click="roomCount = String(count); syncLayout()">{{ count }}</button><button :class="{ active: roomCount === 'custom' }" @click="roomCount = 'custom'; syncLayout()">手动</button><input v-if="roomCount === 'custom'" v-model.number="customRoomCount" type="number" maxlength="2" placeholder="数" @input="syncLayout" /></view></view><view class="whole-option-row"><text>客厅</text><view><button v-for="count in [0, 1, 2, 3]" :key="count" :class="{ active: hallCount === String(count) }" @click="hallCount = String(count); syncLayout()">{{ count === 0 ? '-' : count }}</button></view></view><view class="whole-option-row"><text>厨房</text><view><button v-for="item in [{ value: '1', label: '有' }, { value: '0', label: '无' }]" :key="item.value" :class="{ active: kitchenCount === item.value }" @click="kitchenCount = item.value; syncLayout()">{{ item.label }}</button></view></view><view class="whole-option-row"><text>卫生间</text><view><button v-for="count in [1, 2, 3]" :key="count" :class="{ active: bathroomCount === String(count) }" @click="bathroomCount = String(count); syncLayout()">{{ count }}</button></view></view><view class="whole-option-row"><text>阳台</text><view><button v-for="count in [0, 1, 2]" :key="count" :class="{ active: balcony === String(count) }" @click="balcony = String(count); syncLayout()">{{ count === 0 ? '无' : count }}</button></view></view></view></template
          ><template v-else><view class="shared-rental-options"
            ><view class="shared-room-type"
              ><small>本次出租 · 房型</small
              ><view class="shared-room-type-options"
                ><label v-for="option in roomTypes" :key="option" :class="{ active: roomType === option }" @click.stop="setSharedRoomType(option)"
                  ><checkbox :checked="roomType === option" color="#40543f" />{{ option }}</label
                ></view></view
            ><view class="shared-feature-grid"
              ><view class="picker-item shared-checkbox-item"
                ><small>卫生间</small
                ><checkbox-group @change="togglePrivateBathroom"
                  ><label><checkbox value="private" :checked="privateBathroom" color="#40543f" />独立卫生间</label
                ></checkbox-group></view
              ><view class="picker-item shared-checkbox-item"
                ><small>飘窗（凸出窗台）</small
                ><checkbox-group @change="toggleBayWindow"
                  ><label><checkbox value="bay-window" :checked="hasBayWindow" color="#40543f" />有飘窗</label
                ></checkbox-group></view
            ></view></view></template
          ></view
        ><view class="layout-preview"
          ><text>房源摘要</text><strong>{{ form.layout }}</strong></view
        ><view v-if="detailModuleEnabled('overview') || detailModuleEnabled('location')" class="detail-grid"
          ><template v-if="detailModuleEnabled('overview')"><view v-if="rentType === '合租'" class="field"><text>整套户型（概况）</text><input v-model.trim="form.wholeLayout" maxlength="32" placeholder="如：3室2厅2卫1阳台" /></view
          ><view v-if="rentType === '合租'" class="field"><text>整套面积（㎡）</text><input v-model.number="form.wholeAreaSqm" type="number" placeholder="如：105" /></view
          ><view v-if="rentType === '整租'" class="field"><text>面积（㎡）</text><input v-model.number="form.areaSqm" type="number" placeholder="如：55" /></view
          ><view class="field auto-floor"><text>楼层</text><strong>{{ automaticFloorInfo || form.floorInfo || "选择门牌后自动生成" }}</strong></view
          ><view class="field"><text>朝向</text><input v-model.trim="form.orientation" maxlength="32" placeholder="如：南" /></view
          ><view class="field option-field"><text>楼内配置</text><checkbox-group @change="toggleElevator"><label><checkbox value="elevator" :checked="form.elevator === '有电梯'" color="#40543f" />有电梯</label></checkbox-group></view></template
          ><view v-if="detailModuleEnabled('location')" class="field metro-field"><text>步行至地铁（分钟）</text><input v-model.number="form.metroMinutes" type="number" placeholder="如：8" /></view></view
        ></view
      ><view v-show="activeSection === 2" class="section-card"
        ><view class="section-head"
          ><text>03</text
          ><view
            ><strong>配套与描述</strong></view
          ></view
        ><checkbox-group v-if="detailModuleEnabled('facilities')"
          class="amenity-grid"
          @change="amenities = [...amenities.filter(name => !amenityOptions.includes(name)), ...$event.detail.value]"
          ><label
            v-for="option in amenityOptions"
            :key="option"
            :class="{ active: amenities.includes(option) }"
            ><checkbox
              :value="option"
              :checked="amenities.includes(option)"
              color="#40543f"
            /><image
              class="amenity-option-icon"
              :src="amenityIcon(option)"
              mode="aspectFit"
            /><text>{{ option }}</text></label
          ></checkbox-group
        ><view v-if="detailModuleEnabled('description')" class="description">
          <textarea
            v-model.trim="form.description"
            maxlength="800"
            :placeholder='copyText("填写采光、楼层、朝向、入住要求、周边交通等信息")'
          ></textarea
          ><text>{{ String(form.description || "").length }}/800</text></view
        ><view v-if="detailModuleEnabled('description')" class="field"
          ><text>入住规则</text
          ><input v-model.trim="form.rulesText" maxlength="200" :placeholder='copyText("如：可做饭，宠物需沟通，女生优先")' /></view
        ><view v-if="!detailModuleEnabled('facilities') && !detailModuleEnabled('description')" class="module-entry-disabled">当前后台未启用“房屋设施”或“房源简介”，此步无需填写。</view>
        ></view>
      <view v-show="activeSection === 3" class="section-card"
        ><view class="section-head"
          ><text>04</text
          ><view><strong>租金与入住</strong><small>把价格、付款与入住条件集中填写</small></view
        ></view
        ><view class="rent-row"
          ><view class="field"
            ><text>月租 *</text
            ><view class="rent-input"><input v-model.number="form.rentMonthly" type="number" placeholder="如：2600" /><small>元/月</small></view></view
          ><view class="field option-field"
            ><text>中介费</text><checkbox-group @change="toggleNoAgentFee"><label><checkbox value="no-agent-fee" :checked="noAgentFee" color="#40543f" />无中介费</label></checkbox-group></view></view
        ><view class="term-block"><text>押付方式 *</text><view class="payment-options"><view class="payment-fixed">押一</view><view v-for="cycle in paymentCycles" :key="cycle" :class="{ active: paymentCycle === cycle }" @click="setPaymentCycle(cycle)">{{ cycle }}</view></view></view
        ><view v-if="rentType === '合租'" class="detail-grid shared-space-grid"><view class="field"><text>实际使用空间</text><input v-model.trim="form.layout" maxlength="32" placeholder="如：次卧 · 独卫 · 有飘窗" /></view><view class="field"><text>实际使用面积（㎡）</text><input v-model.number="form.areaSqm" type="number" placeholder="如：21" /></view></view
        ><view v-if="form.listingSource === '转租'" class="transfer-settlement"><view class="transfer-heading"><view><strong>转租结算</strong><small>只记录剩余月数与押金，具体费用线下沟通</small></view><text>转租</text></view><view class="transfer-choice transfer-stepper"><text>剩余租期</text><view><button @click="adjustTransferMonths(-1)">−</button><input :value="form.remainingLeaseMonths || ''" type="number" maxlength="2" placeholder="月" @input="handleTransferMonthsInput" /><text>个月</text><button @click="adjustTransferMonths(1)">＋</button></view></view><view class="transfer-choice"><text>承接押金</text><view><button v-for="multiplier in [0, 1, 2]" :key="multiplier" :class="{ active: transferDepositMultiplier === multiplier }" @click="setTransferDeposit(multiplier)">{{ multiplier === 0 ? '无需承接' : `押${multiplier}` }}</button></view></view><view class="transfer-summary"><small>当前选择</small><strong>剩余 {{ form.remainingLeaseMonths || 0 }} 个月 · {{ transferDepositMultiplier ? `承接押金 ¥${transferDepositAmount}` : '无需承接押金' }}</strong></view></view
        ><view class="rent-row detail-row"
          ><view class="field"
            ><text>可入住日期</text
            ><input v-model.trim="form.availableDate" maxlength="32" placeholder="默认拎包入住，可填写具体日期" /></view
          ><view class="field utility-summary"><text>水电与杂费</text><strong>{{ form.utilitiesText || "请选择" }}</strong></view></view
        ><view class="term-block"><text>水电与杂费</text><radio-group class="utility-options" @change="setUtilities($event.detail.value)"><label v-for="option in utilityOptions" :key="option" :class="{ active: form.utilitiesText === option }"><radio :value="String(utilityOptions.indexOf(option))" :checked="form.utilitiesText === option" color="#40543f" />{{ option }}</label></radio-group></view>
        <view class="rent-summary"><text>对外展示</text><strong>{{ Number(form.rentMonthly) || 0 }}/月</strong><span>{{ form.depositPayment || "待填写押付方式" }}</span></view>
        </view
      ><view v-show="activeSection === 4" class="section-card"
        ><view class="section-head"
          ><text>05</text
          ><view
            ><strong>图片与视频</strong></view
          ></view
        ><view class="media-guidance"
          ><strong>封面素材</strong
          ><text>上传一次即可：列表横向裁切、瀑布流竖向裁切；详情保持原比例。</text></view
        ><view class="cover-focus-control"
          ><view><strong>封面焦点（可选）</strong><text>默认居中；仅在主体靠边时调整，横图和竖图会共用。</text></view
          ><view class="focus-options"><view><text>水平</text><picker :range="['靠左', '居中', '靠右']" :value="[0, 50, 100].indexOf(Number(form.coverFocusX ?? 50))" @change="form.coverFocusX = [0, 50, 100][Number($event.detail.value)]"><view>{{ ['靠左', '居中', '靠右'][[0, 50, 100].indexOf(Number(form.coverFocusX ?? 50))] || '居中' }}</view></picker></view><view><text>垂直</text><picker :range="['靠上', '居中', '靠下']" :value="[0, 50, 100].indexOf(Number(form.coverFocusY ?? 50))" @change="form.coverFocusY = [0, 50, 100][Number($event.detail.value)]"><view>{{ ['靠上', '居中', '靠下'][[0, 50, 100].indexOf(Number(form.coverFocusY ?? 50))] || '居中' }}</view></picker></view></view></view
        ><view v-if="existingImages.length" class="uploaded-gallery"
          ><text class="media-group-title">已上传图片</text
          ><view class="photo-grid"
            ><view
              v-for="(media, index) in existingImages"
              :key="media.id"
              class="photo-card uploaded-photo"
              @click="previewExistingImage(media.url)"
              ><image :src="mediaUrl(media.url)" mode="aspectFill" /><text
                >{{ index === 0 ? "封面" : `已上传 ${index + 1}` }}</text
              ><view class="media-tools" @click.stop
                ><text v-if="index > 0" @click="setExistingCover(media.id)">封面</text
                ><text v-if="index > 0" @click="moveExistingImage(media.id, -1)">←</text
                ><text v-if="index < existingImages.length - 1" @click="moveExistingImage(media.id, 1)">→</text
                ><text class="remove" @click="removeExistingMedia(media.id)">×</text></view
              ></view
            ></view
          ></view
        ><view v-if="images.length" class="pending-gallery"
          ><text class="media-group-title">待上传</text
          ><view class="photo-grid"
            ><view
            v-for="(image, index) in images"
            :key="image.path"
            class="photo-card"
            :class="{ cover: coverImageIndex === index }"
            @click="previewPickedImages(index)"
            ><image :src="image.path" mode="aspectFill" /><text
              class="cover-mark"
              >{{ coverImageIndex === index ? "封面" : `${image.width}×${image.height}` }}</text
            ><view class="media-tools" @click.stop
              ><text @click="cropImage(index)">{{ image.path === image.originalPath ? "裁切" : "重裁" }}</text
              ><text v-if="coverImageIndex !== index" @click="setCover(index)">封面</text
              ><text v-if="index > 0" @click="moveImage(index, -1)">←</text
              ><text v-if="index < images.length - 1" @click="moveImage(index, 1)">→</text
              ><text class="remove" @click="removeImage(index)">×</text></view
            ></view
            ></view
          ></view
        ><view v-if="videos.length || existingVideos.length" class="video-preview-list"
          ><text class="media-group-title">房源视频</text
          ><view v-for="(media, index) in existingVideos" :key="media.id" class="video-entry"
            ><view class="video-card"><video
              :id="`existing-video-${index}`"
              :src="mediaUrl(media.url)"
              controls
              show-center-play-btn
              object-fit="contain"
              @play="markVideoPlaying(`existing-video-${index}`)"
              @pause="markVideoPaused(`existing-video-${index}`)"
              @ended="markVideoPaused(`existing-video-${index}`)"
            /><text>已上传视频</text><text class="remove" @click="removeExistingMedia(media.id)">×</text></view><button
              class="video-toggle"
              @click.stop="toggleVideo(`existing-video-${index}`)"
              >{{ playingVideoId === `existing-video-${index}` ? "暂停播放" : "播放视频" }}</button></view
          ><view
            v-for="(path, index) in videos"
            :key="path"
            class="video-entry"
            ><view class="video-card"><video
              :id="`pending-video-${index}`"
              :src="path"
              controls
              show-center-play-btn
              object-fit="contain"
              @play="markVideoPlaying(`pending-video-${index}`)"
              @pause="markVideoPaused(`pending-video-${index}`)"
              @ended="markVideoPaused(`pending-video-${index}`)"
            /><text
              class="remove"
              @click="videos.splice(index, 1)"
              >×</text
            ></view><button
              class="video-toggle"
              @click.stop="toggleVideo(`pending-video-${index}`)"
              >{{ playingVideoId === `pending-video-${index}` ? "暂停播放" : "播放视频" }}</button></view
          ></view
        ><view class="media-actions"
          ><view @click="pick('image')"
            ><strong>＋ 图片</strong><small>{{ mediaCount }}/9</small></view
          ><view @click="pick('video')"
            ><strong>＋ 视频</strong><small>{{ videoCount }}/1</small></view
          ></view
        ></view
        ><view v-if="activeSection === 4" class="media-limit-hint"
        >图片 ≥640px / ≤10MB · 视频 ≤20MB / 60秒</view
      ><view v-if="activeSection === 4" class="title-composer final-title-composer"
        ><view><strong>列表展示规则</strong><small>由后台“小程序配置”的客户与非客户视角统一控制，录入时无需单独选择</small></view
        ><view class="auto-title"><text>客户标题预览</text><strong>{{ customerGeneratedTitle }}</strong></view></view>
      <view v-if="activeSection === 4" class="final-review">
        <view class="final-review-head"><view><strong>发布前核对</strong><small>可点上方步骤返回修改</small></view><text>{{ completionCount }}/5</text></view>
        <view class="final-review-grid">
          <view :class="{ ready: sectionReady[0] }"><text>位置门牌</text><small>{{ form.community || "待选择" }}</small></view>
          <view :class="{ ready: sectionReady[1] }"><text>户型出租</text><small>{{ form.layout || "待填写" }}</small></view>
          <view :class="{ ready: sectionReady[3] }"><text>租金入住</text><small>{{ Number(form.rentMonthly) || 0 }}/月 {{ form.depositPayment || "" }}</small></view>
          <view :class="{ ready: sectionReady[4] }"><text>图片素材</text><small>{{ mediaCount }}/9 张图片 · {{ videoCount }} 个视频</small></view>
        </view>
        <view class="listing-compare"><view class="listing-compare-head"><strong>{{ copyText("列表效果核对") }}</strong><small>以下为保存后首页与“房源”页的真实卡片布局</small></view><view class="review-perspective-switch"><view :class="{ active: reviewPerspective === 'customer' }" @click="reviewPerspective = 'customer'">普通用户 · 瀑布流</view><view :class="{ active: reviewPerspective === 'admin' }" @click="reviewPerspective = 'admin'">管理员 · 列表</view></view>
          <swiper class="review-preview-swiper" :current="reviewPerspective === 'customer' ? 0 : 1" :duration="260" @change="reviewPerspective = Number($event.detail.current) === 0 ? 'customer' : 'admin'">
            <swiper-item class="review-preview-item">
              <view class="preview-home-masonry"><view class="preview-home-masonry-column"><view class="preview-home-masonry-card current"><view class="preview-home-masonry-cover"><image v-if="reviewCoverUrl" :src="reviewCoverUrl" :style="{ objectPosition: `${form.coverFocusX ?? 50}% ${form.coverFocusY ?? 50}%` }" mode="aspectFill"/><text v-else>小皮</text><view class="preview-listing-status">可租</view><view class="preview-current-label">当前编辑</view><view v-if="videoCount" class="preview-home-video-play">▶</view><view class="preview-home-cover-location">⌖ {{ form.areaLabel || reviewLocation }}</view></view><view class="preview-home-masonry-copy"><view class="preview-home-listing-title">{{ customerGeneratedTitle }}</view><view v-if="previewPrimaryTags.length || previewSecondaryTags.length" class="preview-home-masonry-tags"><text v-for="label in [...previewPrimaryTags, ...previewSecondaryTags]" :key="label">{{ label }}</text></view><view class="preview-listing-price">¥ {{ Number(form.rentMonthly) || 0 }}<text>/月</text></view></view></view><view v-for="property in reviewReferenceProperties.filter((_, index) => index % 2 === 1)" :key="property.id" class="preview-home-masonry-card"><view class="preview-home-masonry-cover"><image :src="referenceCoverUrl(property)" mode="aspectFill"/><view class="preview-listing-status">可租</view><view v-if="property.media?.some((media) => media.type === 'video')" class="preview-home-video-play">▶</view><view class="preview-home-cover-location">⌖ {{ property.areaLabel || referenceLocation(property) }}</view></view><view class="preview-home-masonry-copy"><view class="preview-home-listing-title">{{ publicListingTitle(property) }}</view><view class="preview-home-masonry-tags"><text v-for="label in referenceTags(property, previewTagFields)" :key="label">{{ label }}</text></view><view class="preview-listing-price">¥ {{ property.rentMonthly }}<text>/月</text></view></view></view></view><view class="preview-home-masonry-column"><view v-for="property in reviewReferenceProperties.filter((_, index) => index % 2 === 0)" :key="property.id" class="preview-home-masonry-card"><view class="preview-home-masonry-cover"><image :src="referenceCoverUrl(property)" mode="aspectFill"/><view class="preview-listing-status">可租</view><view v-if="property.media?.some((media) => media.type === 'video')" class="preview-home-video-play">▶</view><view class="preview-home-cover-location">⌖ {{ property.areaLabel || referenceLocation(property) }}</view></view><view class="preview-home-masonry-copy"><view class="preview-home-listing-title">{{ publicListingTitle(property) }}</view><view class="preview-home-masonry-tags"><text v-for="label in referenceTags(property, previewTagFields)" :key="label">{{ label }}</text></view><view class="preview-listing-price">¥ {{ property.rentMonthly }}<text>/月</text></view></view></view></view></view>
            </swiper-item>
            <swiper-item class="review-preview-item">
              <view class="preview-admin-list"><view class="preview-admin-listing current"><view class="preview-admin-image"><image v-if="reviewCoverUrl" :src="reviewCoverUrl" mode="aspectFill"/><text v-else>小皮</text><view class="preview-listing-status">可租</view><view class="preview-current-label">当前编辑</view><view v-if="videoCount" class="preview-admin-video-play">▶</view><view class="preview-admin-cover-location">⌖ {{ form.areaLabel || reviewLocation }}</view></view><view class="preview-admin-info"><view class="preview-home-listing-title">{{ generatedTitle }}</view><view class="preview-admin-doorplate">门牌 · {{ displayTagValue("unit") || "待补充" }}</view><view class="preview-admin-tags"><view v-if="previewAdminPrimaryTags.length" class="preview-tags primary"><text v-for="label in previewAdminPrimaryTags" :key="label">{{ label }}</text></view><view v-if="previewAdminSecondaryTags.filter(label => label !== displayTagValue('unit')).length" class="preview-tags fee"><text v-for="label in previewAdminSecondaryTags.filter(label => label !== displayTagValue('unit'))" :key="label">{{ label }}</text></view></view><view class="preview-listing-price">¥ {{ Number(form.rentMonthly) || 0 }}<text>/月</text></view></view></view><view v-for="property in reviewReferenceProperties" :key="property.id" class="preview-admin-listing"><view class="preview-admin-image"><image :src="referenceCoverUrl(property)" mode="aspectFill"/><view class="preview-listing-status">可租</view><view v-if="property.media?.some((media) => media.type === 'video')" class="preview-admin-video-play">▶</view><view class="preview-admin-cover-location">⌖ {{ property.areaLabel || referenceLocation(property) }}</view></view><view class="preview-admin-info"><view class="preview-home-listing-title">{{ property.community || property.title || "房源" }}</view><view class="preview-admin-doorplate">门牌 · {{ listingDoorplate(property) || "待补充" }}</view><view class="preview-admin-tags"><view class="preview-tags primary"><text v-for="label in referenceTags(property, previewPrimaryTagFields)" :key="label">{{ label }}</text></view><view class="preview-tags fee"><text v-for="label in referenceTags(property, previewSecondaryTagFields)" :key="label">{{ label }}</text></view></view><view class="preview-listing-price">¥ {{ property.rentMonthly }}<text>/月</text></view></view></view></view>
            </swiper-item>
          </swiper>
        </view>
        <text v-if="id && currentStatus === 'draft'" class="draft-delete-link" @click="deleteDraft">删除当前草稿</text>
      </view>
      <view class="save-space" /><view class="save-bar"
        ><view v-if="saving" class="progress">{{ uploadProgress }}</view
        ><view v-if="activeSection < 4" class="step-actions" :class="{ 'three-actions': activeSection > 0 }"
          ><button
            v-if="activeSection > 0"
            class="back-button"
            @click="goToSection(activeSection - 1)"
          >
            上一步</button
          ><button class="step-save-button" :disabled="saving" @click="save()">{{ isLiveEditing ? "保存修改" : "保存草稿" }}</button
          ><button
            class="primary next-button"
            @click="goToSection(activeSection + 1)"
          >
            下一步 · {{ sectionSteps[activeSection + 1] }}
          </button></view
        ><view v-else class="save-actions" :class="{ 'three-actions': !isLiveEditing }"
          ><template v-if="isLiveEditing"><button :disabled="saving" @click="save()">保存修改</button><button class="primary" :disabled="saving || !requiredReady" @click="save('preview')">查看公开效果</button></template
          ><template v-else><button :disabled="saving" @click="save()">保存草稿</button><button :disabled="saving || !requiredReady" @click="save('preview')">预览草稿</button><button class="primary" :disabled="saving" @click="confirmPublish">{{ saving ? "处理中..." : "直接发布" }}</button></template
          ></view
        ></view
      ><view v-if="structureStep" class="structure-modal" @click="structureStep = ''"
        ><view class="structure-sheet" @click.stop
          ><view class="structure-sheet-head"
            ><view><strong>{{ structureStep === "building" ? "选择楼栋" : structureStep === "unit" ? "选择单元" : "选择房号" }}</strong><small>{{ form.community }}</small></view><view class="structure-head-actions"><text v-if="structureStep !== 'building'" class="structure-back" @click="returnToPreviousStructureStep">‹ 返回</text><text class="structure-close" @click="structureStep = ''">×</text></view></view
          ><view class="structure-progress structure-progress-three"><text :class="{ active: structureStep === 'building', available: canReturnToStructureStep('building') }" @click="returnToStructureStep('building')">楼栋</text><text :class="{ active: structureStep === 'unit', available: canReturnToStructureStep('unit') }" @click="returnToStructureStep('unit')">单元</text><text :class="{ active: structureStep === 'room', available: canReturnToStructureStep('room') }" @click="returnToStructureStep('room')">房号</text></view
          ><view v-if="hasDoorplateCatalog" class="doorplate-entry-switch"><text :class="{ active: doorplateEntryMode === 'catalog' }" @click="switchDoorplateEntryMode('catalog')">选择已有门牌</text><text :class="{ active: doorplateEntryMode === 'manual' }" @click="switchDoorplateEntryMode('manual')">自定义输入</text></view
          ><scroll-view scroll-y class="structure-grid-scroll"
            ><view v-if="!hasDoorplateCatalog || doorplateEntryMode === 'manual'" class="manual-doorplate-entry"><text v-if="!hasDoorplateCatalog">该小区暂无门牌库数据，可直接自定义填写。</text><text v-else>当前步骤：{{ structureStep === 'building' ? '楼栋' : structureStep === 'unit' ? '单元' : '房号' }}</text><input v-model="manualDoorplate" maxlength="24" :placeholder="structureStep === 'building' ? '输入楼栋，如 3' : structureStep === 'unit' ? '输入单元，如 1' : '输入房号，如 3301'" @confirm="confirmManualDoorplate" /><button :disabled="!manualDoorplate.trim()" @click="confirmManualDoorplate">{{ structureStep === 'room' ? '确认房号' : '确认并继续' }}</button></view
            ><view v-else-if="doorplateEntryMode === 'catalog' && structureStep === 'building'" class="structure-grid"><view v-for="rule in buildingOptions" :key="rule" class="structure-grid-item" :class="{ active: buildingNo === rule }" @click="selectBuildingRule(rule)">{{ rule }}</view></view
            ><view v-else-if="doorplateEntryMode === 'catalog' && structureStep === 'unit'" class="structure-grid"><view v-for="unit in unitOptions" :key="unit" class="structure-grid-item" :class="{ active: unitNo === unit }" @click="chooseUnit(unit)">{{ unit }} 单元</view></view
            ><view v-else-if="doorplateEntryMode === 'catalog'" class="structure-grid"><view v-for="room in roomOptions" :key="room.roomNo" class="structure-grid-item" :class="{ active: roomNo === room.roomNo }" @click="chooseRoom(room)">{{ room.roomNo }}</view></view
          ></scroll-view
          ><view class="structure-sheet-footer"><button v-if="structureStep === 'unit'" @click="chooseUnit('')">无单元，跳过</button><text>不填门牌也可以继续录入</text><button @click="structureStep = ''">暂不选择</button></view
        ></view
      ></view
      ></template
    ></view
  >
</template>
<style scoped src="./property-edit.css"></style>
