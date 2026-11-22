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
import { normalizeCommunityName } from "../../utils/community-name";
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
const MAX_VIDEO_SIZE = 40 * 1024 * 1024;
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
const locationQuery = ref("");
const locationSearching = ref(false);
const communityDirectory = ref<Record<string, number>>({});
const communityLocations = ref<Record<string, { address: string; latitude: number; longitude: number }>>({});
const communityPlans = ref<Record<string, { community?: string }>>({});
const communityPickerOpen = ref(false);
const communityDirectoryLoading = ref(false);
const communityDirectoryError = ref("");
const communityLocationSearching = ref(false);
const communityLocationSearchOpen = ref(false);
const communityLocationSearchError = ref("");
const communityLocationSearchNotice = ref("");
const communityLocationResults = ref<LocationSearchItem[]>([]);
const communityDirectoryQuery = ref("");
type LocationCandidate = LocationSearchItem & { saved?: boolean; source: "library" | "map"; buildingPoint?: boolean };
const locationResults = ref<LocationCandidate[]>([]);
const locationMapSearched = ref(false);
let locationSearchTimer: ReturnType<typeof setTimeout> | undefined;
let locationSearchVersion = 0;
const communityDirectoryOptions = computed(() => {
  const usage = new Map<string, { name: string; used: number; doors: number }>();
  for (const property of reviewProperties.value) {
    const name = String(property.community || "").trim();
    const key = normalizeCommunityName(name);
    if (!key || isBuildingPointName(name)) continue;
    const current = usage.get(key) || { name, used: 0, doors: 0 };
    current.used += 1;
    if (name.localeCompare(current.name, "zh-CN") < 0) current.name = name;
    usage.set(key, current);
  }
  for (const [name, doors] of Object.entries(communityDirectory.value)) {
    if (isBuildingPointName(name)) continue;
    const key = normalizeCommunityName(name);
    if (!key) continue;
    const current = usage.get(key) || { name, used: 0, doors: 0 };
    current.doors = Math.max(current.doors, doors);
    if (current.used === 0 && name.localeCompare(current.name, "zh-CN") < 0) current.name = name;
    usage.set(key, current);
  }
  for (const name of Object.keys(communityLocations.value)) {
    if (isBuildingPointName(name)) continue;
    const key = normalizeCommunityName(name);
    if (key && !usage.has(key)) usage.set(key, { name, used: 0, doors: 0 });
  }
  for (const [keyName, plan] of Object.entries(communityPlans.value)) {
    const name = String(plan.community || keyName).trim();
    if (isBuildingPointName(name)) continue;
    const key = normalizeCommunityName(name);
    if (key && !usage.has(key)) usage.set(key, { name, used: 0, doors: 0 });
  }
  const options = [...usage.values()].sort((a, b) => b.used - a.used || b.doors - a.doors || a.name.localeCompare(b.name, "zh-CN"));
  return options;
});
const commonCommunityOptions = computed(() => communityDirectoryOptions.value.slice(0, 4));
const filteredCommunityOptions = computed(() => {
  const query = normalizeCommunityName(communityDirectoryQuery.value);
  if (!query) return communityDirectoryOptions.value;
  return communityDirectoryOptions.value.filter(item => normalizeCommunityName(item.name).includes(query));
});
const pickerMapResults = computed(() => locationResults.value.filter(item => item.source === "map"));
const form = reactive<Partial<Property>>({
  title: "",
  community: "",
  location: "",
  buildingNo: "",
  unitNo: "",
  roomNo: "",
  roomSequence: 0,
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
const orientationOptions = ["南", "东南", "西南", "北", "东北", "西北", "东", "西", "南北通透", "东西通透"];
const orientationPickerOptions = [...orientationOptions, "其他", "清除朝向"];
const orientationCustomOpen = ref(false);
const quickMoreOpen = ref(false);
const roomCount = ref("1"),
  customRoomCount = ref(""),
  roomCountChosen = ref(false),
  roomLayoutChosen = ref(false),
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
const effectiveRoomCount = computed(() => Math.min(9, Math.max(1, roomCount.value === "custom" ? Number(customRoomCount.value) || 1 : Number(roomCount.value) || 1)));
const boundedRoomCount = computed(() => Math.min(9, effectiveRoomCount.value));
const roomLayoutReady = computed(() => Boolean(
  roomCountChosen.value && roomLayoutChosen.value && (roomCount.value !== "custom" || /^[1-9]$/.test(customRoomCount.value)),
));
const wholeLayoutText = computed(() => [
  `${effectiveRoomCount.value}室`,
  hallCount.value !== "0" ? `${hallCount.value}厅` : "",
  kitchenCount.value === "1" ? "1厨" : "",
  bathroomCount.value !== "0" ? `${bathroomCount.value}卫` : "",
  balcony.value !== "0" ? `${balcony.value}阳台` : "",
].filter(Boolean).join(""));
const applyWholeLayout = (value: string) => {
  const source = String(value || "");
  const numberOf = (suffix: string, fallback: number) => Number(source.match(new RegExp(`(\\d+)${suffix}`))?.[1] || fallback);
  const roomMatch = source.match(/(\d+|[一二三四五六七八九])室/);
  const legacyNumber = (part: string | undefined, fallback: number) => ({ 一: 1, 两: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9 }[part?.[0] || ""] || fallback);
  const parsedRoomCount = Math.min(9, Math.max(1, roomMatch ? (/\d/.test(roomMatch[1]) ? Number(roomMatch[1]) : legacyNumber(roomMatch[1], 1)) : 1));
  roomCount.value = parsedRoomCount > 4 ? "custom" : String(parsedRoomCount);
  customRoomCount.value = String(parsedRoomCount);
  hallCount.value = String(Math.min(3, Math.max(0, numberOf("厅", source.includes("无厅") ? 0 : 1))));
  kitchenCount.value = String(numberOf("厨", source.includes("厨") ? 1 : 0));
  bathroomCount.value = String(Math.min(4, Math.max(1, numberOf("卫", 1))));
  balcony.value = String(Math.min(4, Math.max(0, numberOf("阳台", source.includes("阳台") ? 1 : 0))));
};
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
const selectedChannelIndex = ref(-1);
const associationRadiusKm = ref(3);
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
const publishValidationIssues = computed(() => {
  const issues: string[] = [];
  if (!form.community?.trim()) issues.push("请选择小区");
  if (!form.location?.trim()) issues.push("补充房源地址");
  if (!geoReady.value) issues.push("获取房源地图位置");
  if (!form.layout?.trim() || !roomLayoutReady.value) issues.push("完成房屋户型选择");
  if (rentType.value === "合租" && !String(form.wholeLayout || "").trim()) issues.push("选择合租房源的整套户型");
  if (rentType.value === "合租" && Number(form.roomSequence) > boundedRoomCount.value) issues.push("出租房间序号不能超过整套室数");
  if (!Number.isFinite(Number(form.rentMonthly)) || Number(form.rentMonthly) < 0) issues.push("填写有效月租");
  if (!form.depositPayment?.trim()) issues.push("选择押付方式");
  if (form.listingSource === "转租" && Number(form.remainingLeaseMonths) <= 0) issues.push("填写转租剩余租期");
  return issues;
});
const requiredReady = computed(() => publishValidationIssues.value.length === 0);
const publishValidationMessage = computed(() => publishValidationIssues.value.length
  ? `发布前还需要：${publishValidationIssues.value.join("、")}`
  : "房源信息已满足发布条件");
const sectionSteps = ["房源信息", "图片与视频"];
const optionalDetailsOpen = ref(false);
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
  selectedChannelIndex.value = -1;
  if (!hasValidCoordinates(latitude, longitude)) return;
  let nearestDistance = Infinity;
  channels.value.forEach((channel, index) => {
    const lat = Number(channel.latitude);
    const lng = Number(channel.longitude);
    if (!hasValidCoordinates(lat, lng)) return;
    const distance = distanceKm(latitude, longitude, lat, lng);
    const radius = Number(channel.radiusKm) || associationRadiusKm.value;
    if (distance <= radius && distance < nearestDistance) {
      nearestDistance = distance;
      selectedChannelIndex.value = index;
    }
  });
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
    if (config.featuredChannels?.length) {
      channels.value = config.featuredChannels;
      if (geoReady.value) selectNearestChannel(Number(form.latitude), Number(form.longitude));
    }
  } catch {
    channels.value = [financialHarbor];
  }
};
const loadCommunityDirectory = async () => {
  const [catalog, locations, plans] = await Promise.allSettled([
    request<Record<string, number>>("/v1/admin/doorplates"),
    request<Record<string, { address: string; latitude: number; longitude: number }>>("/v1/admin/community-locations"),
    request<Record<string, { community?: string }>>("/v1/admin/doorplate-plans"),
  ]);
  if (catalog.status === "fulfilled") communityDirectory.value = catalog.value;
  if (locations.status === "fulfilled") communityLocations.value = locations.value;
  if (plans.status === "fulfilled") communityPlans.value = plans.value;
  const failed = [catalog, locations, plans].find(result => result.status === "rejected");
  if (failed?.status === "rejected") communityDirectoryError.value = failed.reason instanceof Error ? failed.reason.message : "部分小区数据加载失败";
};
const selectLibraryCommunity = async (name: string) => {
  const title = stripBuildingPointSuffix(name.trim());
  if (title.length < 2 || communityLocationSearching.value) return;
  const changed = !communityNameMatches(String(form.community || ""), title);
  const savedLocation = Object.entries(communityLocations.value).find(([community]) => communityNameMatches(community, title))?.[1];
  invalidateLocationSearch();
  form.community = title;
  communityLocationSearchOpen.value = false;
  communityLocationSearchError.value = "";
  communityLocationSearchNotice.value = "";
  communityLocationResults.value = [];
  locationQuery.value = title;
  locationResults.value = [];
  if (changed) {
    selectedChannelIndex.value = -1;
    form.location = "";
    form.latitude = 0;
    form.longitude = 0;
  }
  if (savedLocation) {
    if (savedLocation.address && (changed || !form.location || isPlaceholderAddress(String(form.location)))) form.location = savedLocation.address;
    if (hasValidCoordinates(Number(savedLocation.latitude), Number(savedLocation.longitude)) && (changed || !geoReady.value)) {
      form.latitude = Number(savedLocation.latitude);
      form.longitude = Number(savedLocation.longitude);
      selectNearestChannel(form.latitude, form.longitude);
    }
  }
  if (changed) {
    buildingRules.value = [];
    buildingNo.value = "";
    unitNo.value = "";
    roomNo.value = "";
    selectedFloor.value = "";
    form.floorInfo = "";
  }
  if (communityLocationNeedsRepair.value) {
    await searchCommunityLocation(true);
    return;
  }
  await finishLibrarySelection();
};
const finishLibrarySelection = async () => {
  communityLocationSearchOpen.value = false;
  communityPickerOpen.value = false;
  communityDirectoryQuery.value = "";
  await openStructurePicker();
};
const openLocationRepair = () => {
  communityPickerOpen.value = true;
  void searchCommunityLocation();
};
const openCommunityDirectory = () => {
  communityLocationSearchOpen.value = false;
  communityDirectoryQuery.value = "";
  locationQuery.value = "";
  locationResults.value = [];
  locationMapSearched.value = false;
  communityPickerOpen.value = true;
  communityDirectoryError.value = "";
  communityDirectoryLoading.value = true;
  void loadCommunityDirectory().catch((error) => {
    communityDirectoryError.value = error instanceof Error ? error.message : "小区列表加载失败";
  }).finally(() => { communityDirectoryLoading.value = false; });
};
const handleCommunityDirectoryInput = () => {
  locationQuery.value = communityDirectoryQuery.value;
  locationResults.value = [];
  locationMapSearched.value = false;
};
const searchMapInCommunityPicker = () => {
  locationQuery.value = communityDirectoryQuery.value.trim();
  if (locationQuery.value.length < 2) {
    uni.showToast({ title: "输入至少两个字再搜索地图", icon: "none" });
    return;
  }
  void runLocationSearch(undefined, true);
};
const runLocationSearch = async (version = ++locationSearchVersion, searchMap = false) => {
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
    const catalog = communityDirectory.value;
    const savedLocations = communityLocations.value;
    // Prefer the catalog standard spelling and show one suggestion for
    // punctuation/spacing variants already present in historical records.
    const namesByIdentity = new Map<string, string>();
    for (const name of [...Object.keys(catalog), ...Object.keys(savedLocations), ...reviewProperties.value.map(item => item.community).filter(Boolean)]) {
      if (isBuildingPointName(name)) continue;
      const identity = normalizeCommunityName(name);
      if (identity && !namesByIdentity.has(identity)) namesByIdentity.set(identity, name);
    }
    const normalizedKeyword = normalizeCommunityName(stripBuildingPointSuffix(keyword));
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
      const isPlaceholder = isPlaceholderAddress(address);
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
    const shouldSearchMap = searchMap;
    locationMapSearched.value = shouldSearchMap;
    const mapResults = shouldSearchMap
      ? await searchLocations(keyword, channelLatitude.value, channelLongitude.value).then(items => {
          const seen = new Map<string, LocationCandidate>();
          for (const item of items) {
            const title = stripBuildingPointSuffix(item.title);
            const identity = normalizeCommunityName(title);
            if (identity.length < 2) continue;
            const candidate = { ...item, title, buildingPoint: title !== item.title.trim(), saved: false, source: "map" as const };
            const existing = seen.get(identity);
            if (!existing || (existing.buildingPoint && !candidate.buildingPoint)) seen.set(identity, candidate);
          }
          return [...seen.values()];
        }).catch(error => {
          if (!saved.length) throw error;
          return [];
        })
      : [];
    if (version !== locationSearchVersion || keyword !== locationQuery.value.trim()) return;
    if (exactLibraryMatch && !hasExactPosition) {
      const exactMapMatch = mapResults.find(item => normalizeCommunityName(item.title) === normalizedKeyword);
      const mergedExact = exactMapMatch
        ? [{ ...exactMapMatch, id: exactLibraryMatch.id, title: exactLibraryMatch.title, saved: true }]
        : [];
      const remainingMap = mapResults.filter(item => !exactMapMatch || normalizeCommunityName(item.title) !== normalizedKeyword);
      locationResults.value = [...mergedExact, ...saved.filter(item => !exactMapMatch || item.id !== exactLibraryMatch.id), ...remainingMap];
    } else if (saved.length) {
      const mergedSaved = saved.map(item => {
        const mapMatch = mapResults.find(candidate => normalizeCommunityName(candidate.title) === normalizeCommunityName(item.title));
        if (!mapMatch || (item.address && item.latitude && item.longitude)) return item;
        return { ...mapMatch, id: item.id, title: item.title, saved: true, source: "map" as const };
      });
      const localNames = new Set(mergedSaved.map(item => normalizeCommunityName(item.title)));
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
const normalizeMapCommunityName = (value: string) => normalizeCommunityName(stripBuildingPointSuffix(value).replace(/小区$/u, ""));
const isPlaceholderAddress = (value: string) => /待补充|未定位|QA\s*地图占位地址|测试环境/.test(String(value || ""));
const stripBuildingPointSuffix = (value: string) => {
  let name = value.trim();
  const suffixes = [
    /(?:[-—·\s]*[0-9０-９零〇一二两三四五六七八九十百廿]+(?:[-—·][0-9０-９零〇一二两三四五六七八九十百廿]+)?\s*(?:单元|室))$/i,
    /(?:[-—·\s]*[0-9０-９零〇一二两三四五六七八九十百廿]+\s*(?:栋|幢|号楼|号栋|座))$/i,
  ];
  for (let pass = 0; pass < 4; pass += 1) {
    const before = name;
    for (const suffix of suffixes) name = name.replace(suffix, "");
    name = name.replace(/[-—·\s]+$/g, "").trim();
    if (name === before) break;
  }
  return name;
};
const isBuildingPointName = (name: string) => {
  const cleaned = stripBuildingPointSuffix(name);
  return cleaned !== name.trim();
};
const communityNameMatches = (left: string, right: string) =>
  Boolean(normalizeCommunityName(left)) && normalizeCommunityName(left) === normalizeCommunityName(right);
const locationQueryAlreadySelected = computed(() => Boolean(
  locationQuery.value.trim() && form.community &&
  normalizeCommunityName(stripBuildingPointSuffix(locationQuery.value)) === normalizeCommunityName(String(form.community)),
));
const canUseTypedCommunity = computed(() => {
  const keyword = locationQuery.value.trim();
  if (keyword.length < 2 || normalizeCommunityName(stripBuildingPointSuffix(keyword)) === normalizeCommunityName(String(form.community || ""))) return false;
  if (locationResults.value.length > 0) return false;
  return true;
});
const scheduleLocationSearch = () => {
  if (locationSearchTimer) clearTimeout(locationSearchTimer);
  const version = ++locationSearchVersion;
  locationResults.value = [];
  locationMapSearched.value = false;
  locationSearching.value = locationQuery.value.trim().length >= 2;
  if (!locationSearching.value) return;
  locationSearchTimer = setTimeout(() => void runLocationSearch(version), 450);
};
const searchMapLocationCandidates = () => {
  if (locationQuery.value.trim().length < 2) {
    uni.showToast({ title: "先输入小区或地点名称", icon: "none" });
    return;
  }
  void runLocationSearch(undefined, true);
};
const hasValidCoordinates = (latitude: number, longitude: number) =>
  Number.isFinite(latitude) && Number.isFinite(longitude) && latitude !== 0 && longitude !== 0 &&
  Math.abs(latitude) <= 90 && Math.abs(longitude) <= 180;
const persistCommunityLocation = async (community: string, address: string, latitude: number, longitude: number) => {
  if (!community.trim() || !address.trim() || isPlaceholderAddress(address) || !hasValidCoordinates(latitude, longitude)) return false;
  try {
    await request(`/v1/admin/community-locations?community=${encodeURIComponent(community)}`, {
      method: "PUT",
      data: { address: address.trim(), latitude, longitude },
    });
    communityLocations.value = {
      ...communityLocations.value,
      [community]: { address: address.trim(), latitude, longitude },
    };
    return true;
  } catch {
    return false;
  }
};
const selectedCommunityLibraryLocation = computed(() => {
  const community = String(form.community || "").trim();
  if (!community) return null;
  return Object.entries(communityLocations.value).find(([name]) => communityNameMatches(name, community))?.[1] || null;
});
const communityLocationNeedsRepair = computed(() => {
  const location = selectedCommunityLibraryLocation.value;
  return Boolean(form.community && (!location || isPlaceholderAddress(location.address) || !location.address?.trim() || !hasValidCoordinates(Number(location.latitude), Number(location.longitude))));
});
const searchCommunityLocation = async (autoSelect = false) => {
  const community = String(form.community || "").trim();
  if (!community || communityLocationSearching.value) return;
  communityLocationSearchOpen.value = true;
  communityLocationSearching.value = true;
  communityLocationSearchError.value = "";
  communityLocationSearchNotice.value = "";
  communityLocationResults.value = [];
  let exactMatch: LocationSearchItem | undefined;
  try {
    const results = await searchLocations(community, channelLatitude.value, channelLongitude.value);
    const valid = results.filter(item => hasValidCoordinates(Number(item.latitude), Number(item.longitude)) && !isBuildingPointName(item.title));
    const identity = normalizeMapCommunityName(community);
    const exact = valid.filter(item => normalizeMapCommunityName(item.title) === identity);
    if (autoSelect && exact.length === 1 && exact[0].address?.trim()) exactMatch = exact[0];
    communityLocationResults.value = exact.length ? exact : valid.slice(0, 8);
    if (!exact.length && valid.length) communityLocationSearchNotice.value = "没有完全同名的地图地点，请核对名称和地址后再选择。";
    if (!valid.length) communityLocationSearchError.value = "地图暂未返回可用位置，请稍后重试。";
  } catch (error) {
    communityLocationSearchError.value = error instanceof Error ? error.message : "地图位置搜索失败";
  } finally {
    communityLocationSearching.value = false;
  }
  if (exactMatch && communityPickerOpen.value && communityLocationSearchOpen.value) await selectCommunityLocation(exactMatch);
};
const selectCommunityLocation = async (item: LocationSearchItem) => {
  const community = String(form.community || "").trim();
  const latitude = Number(item.latitude);
  const longitude = Number(item.longitude);
  const address = [item.district, item.address].filter(Boolean).join(" · ") || item.title;
  if (!community || communityLocationSearching.value || !hasValidCoordinates(latitude, longitude)) return;
  communityLocationSearching.value = true;
  communityLocationSearchError.value = "";
  try {
    const saved = await persistCommunityLocation(community, address, latitude, longitude);
    if (!saved) throw new Error("保存小区位置失败");
    if (String(form.community || "").trim() === community) {
      form.location = address;
      form.latitude = Number(latitude.toFixed(7));
      form.longitude = Number(longitude.toFixed(7));
      selectNearestChannel(form.latitude, form.longitude);
      communityLocationSearchOpen.value = false;
      communityLocationResults.value = [];
      uni.showToast({ title: "小区位置已补齐并保存", icon: "success" });
      if (communityPickerOpen.value) await finishLibrarySelection();
    }
  } catch (error) {
    communityLocationSearchError.value = error instanceof Error ? error.message : "保存小区位置失败";
  } finally {
    communityLocationSearching.value = false;
  }
};
const locateSavedCommunity = async (community: string) => {
  try {
    // Doorplate imports contain building/unit/room structure, but often have
    // no geocode. Resolve that once when an admin chooses such a community.
    const candidates = await searchLocations(community, channelLatitude.value, channelLongitude.value);
    const identity = normalizeMapCommunityName(community);
    return candidates.find(item => normalizeMapCommunityName(item.title) === identity) || null;
  } catch {
    return null;
  }
};
const useTypedCommunity = async () => {
  const query = locationQuery.value.trim();
  const title = stripBuildingPointSuffix(query);
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
  communityPickerOpen.value = false;
  communityDirectoryQuery.value = "";
  doorplateEntryMode.value = hasDoorplateCatalog.value ? doorplateEntryMode.value : "manual";
};
const applyLocationResult = async (item: LocationCandidate) => {
  const title = stripBuildingPointSuffix(item.title);
  if (title.length < 2) { uni.showToast({ title: "请选择小区地点，楼栋在门牌中选择", icon: "none" }); return; }
  invalidateLocationSearch();
  const selectionVersion = locationSearchVersion;
  let selected = item;
  const itemHasPosition = hasValidCoordinates(Number(item.latitude), Number(item.longitude));
  if (item.saved && !itemHasPosition) {
    locationSearching.value = true;
    const mapLocation = await locateSavedCommunity(title);
    if (selectionVersion !== locationSearchVersion) return;
    locationSearching.value = false;
    if (mapLocation) selected = { ...mapLocation, id: item.id, title, saved: true, source: "map" };
  }
  const changedCommunity = !communityNameMatches(form.community || "", title);
  form.community = title;
  const selectedAddress = [selected.district, selected.address].filter(Boolean).join(" · ");
  const mapPosition = selected.source === "map";
  if (changedCommunity || (selectedAddress && (!item.saved || mapPosition || !form.location))) form.location = selectedAddress;
  if (changedCommunity || !item.saved || mapPosition || !geoReady.value) {
    form.latitude = hasValidCoordinates(Number(selected.latitude), Number(selected.longitude)) ? Number(Number(selected.latitude).toFixed(7)) : 0;
    form.longitude = hasValidCoordinates(Number(selected.latitude), Number(selected.longitude)) ? Number(Number(selected.longitude).toFixed(7)) : 0;
  }
  if (geoReady.value) selectNearestChannel(Number(form.latitude), Number(form.longitude));
  if ((mapPosition || item.saved) && geoReady.value) {
    const locationSaved = await persistCommunityLocation(title, selectedAddress || title, Number(form.latitude), Number(form.longitude));
    if (!locationSaved) uni.showToast({ title: "房源位置已更新；小区位置未能保存到小区库", icon: "none" });
  } else if (item.saved && !geoReady.value) {
    uni.showToast({ title: "门牌库已匹配；可在小区下方搜索补齐位置", icon: "none" });
  }
  locationQuery.value = title;
  locationResults.value = [];
  communityPickerOpen.value = false;
  communityDirectoryQuery.value = "";
  if (changedCommunity) {
    buildingNo.value = "";
    unitNo.value = "";
    roomNo.value = "";
    selectedFloor.value = "";
    form.floorInfo = "";
  }
  await openStructurePicker();
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
    form.layout?.trim() && roomLayoutReady.value &&
    geoReady.value && Number.isFinite(Number(form.rentMonthly)) &&
    Number(form.rentMonthly) >= 0 && form.depositPayment?.trim(),
  ),
  true,
]);
const completionCount = computed(
  () => sectionReady.value.filter(Boolean).length,
);
const generatedTitle = computed(() => {
  const title = String(form.title || "").trim();
  const community = String(form.community || "").trim();
  return title || community || "选择小区后自动生成";
});
const customerGeneratedTitle = computed(() => String(form.title || "").trim() || generatedTitle.value);
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
watch(() => form.community, (community, previousCommunity) => {
  const currentTitle = String(form.title || "").trim();
  const previousCommunityName = String(previousCommunity || "").trim();
  if (!currentTitle || currentTitle === previousCommunityName) {
    form.title = String(community || "").trim();
  }
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
  } else {
    form.layout = roomType.value ? [roomType.value, toiletType.value, bayWindow.value].filter(Boolean).join(" · ") : "";
  }
  if (rentType.value === "合租") {
    form.wholeLayout = wholeLayoutText.value;
    if (Number(form.roomSequence) > Math.min(effectiveRoomCount.value, 9)) form.roomSequence = 0;
  }
  form.rentType = rentType.value;
};
const setRoomCount = (value: string | number) => {
  if (String(value) === "custom") {
    if (!customRoomCount.value) customRoomCount.value = String(Math.min(9, Math.max(1, Number(roomCount.value) || 1)));
    roomCount.value = "custom";
    roomCountChosen.value = Boolean(customRoomCount.value);
  } else {
    roomCount.value = String(Math.min(9, Math.max(1, Number(value) || 1)));
    customRoomCount.value = roomCount.value;
    roomCountChosen.value = true;
  }
  roomLayoutChosen.value = true;
  syncLayout();
};
const setCustomRoomCount = (event: { detail?: { value?: string | number } }) => {
  const input = String(event.detail?.value ?? "").replace(/\D/g, "").slice(0, 1);
  customRoomCount.value = input ? String(Math.min(9, Math.max(1, Number(input)))) : "";
  roomCount.value = "custom";
  roomCountChosen.value = Boolean(customRoomCount.value);
  roomLayoutChosen.value = Boolean(customRoomCount.value);
  syncLayout();
};
const setOrientation = (value: string) => {
  if (value === "其他") {
    orientationCustomOpen.value = true;
    if (orientationOptions.includes(String(form.orientation || ""))) form.orientation = "";
    return;
  }
  form.orientation = value;
  orientationCustomOpen.value = false;
};
const setFloorInfo = (event: { detail?: { value?: string | number } }) => {
  const digits = String(event.detail?.value ?? "").replace(/\D/g, "").slice(0, 2);
  form.floorInfo = digits && Number(digits) > 0 ? `${Number(digits)}楼` : "";
};
const openHouseQuickSettings = () => {
  quickMoreOpen.value = Boolean(
    form.orientation || form.floorInfo || Number(form.areaSqm) ||
    Number(form.wholeAreaSqm) || Number(form.metroMinutes) || form.elevator,
  );
  structureStep.value = "quick";
};
const houseLayoutSummary = computed(() => [
  wholeLayoutText.value || "待设置户型",
  form.orientation || "朝向待补",
  form.floorInfo || automaticFloorInfo.value || "楼层待补",
  rentType.value,
  rentType.value === "合租" && (Number(form.roomSequence) ? `${form.roomSequence}号房` : "房间序号未指定"),
].filter(Boolean).join(" · "));
const setRoomSequence = (value: number) => {
  form.roomSequence = Math.max(0, Math.min(boundedRoomCount.value, Number(value) || 0));
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
    const legacyNumber = (value: string, fallback: number) => ({ 一: 1, 两: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9 }[value[0]] || fallback);
    const roomMatch = roomHall.match(/(\d+|[一二三四五六七八九])室/);
    const parsedRoomCount = Math.min(9, Math.max(1, roomMatch ? (/\d/.test(roomMatch[1]) ? Number(roomMatch[1]) : legacyNumber(roomMatch[1], 1)) : 1));
    roomCount.value = parsedRoomCount > 4 ? "custom" : String(parsedRoomCount);
    customRoomCount.value = String(parsedRoomCount);
    hallCount.value = String(numberOf("厅", roomHall.includes("无厅") ? 0 : 1));
    kitchenCount.value = String(numberOf("厨", roomHall.includes("厨") || parts.some((part) => part.includes("厨房")) ? 1 : 0));
    bathroomCount.value = String(numberOf("卫", 1));
    balcony.value = String(numberOf("阳台", roomHall.includes("阳台") || parts.some((part) => part.includes("阳台")) ? 1 : 0));
    rentType.value = "整租";
  } else {
    rentType.value = "合租";
    applyWholeLayout(String(form.wholeLayout || ""));
    roomType.value = parts.find((part) => roomTypes.includes(part)) || "";
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
      if (savedPosition && communityNameMatches(String(form.community || ""), property.community) && !geoReady.value) {
        const location = String(form.location || "").trim();
        if (!location || location === property.community || isPlaceholderAddress(location)) {
          form.location = savedPosition.address || location;
        }
        form.latitude = savedPosition.latitude;
        form.longitude = savedPosition.longitude;
      }
    }
    paymentCycle.value = paymentCycles.find((item) => String(property.depositPayment || "").includes(item)) || "付一";
    if (!form.depositPayment?.trim()) setPaymentCycle(paymentCycle.value);
    form.listingSource = property.listingSource || "直租";
    transferDepositMultiplier.value = Number(property.transferDeposit) && Number(property.rentMonthly)
      ? Math.max(1, Math.round(Number(property.transferDeposit) / Number(property.rentMonthly)))
      : 1;
    noAgentFee.value = !String(property.feeText || "").includes("有中介费") || String(property.feeText || "").includes("无中介费");
    buildingNo.value = property.buildingNo || "";
    unitNo.value = property.unitNo || "";
    roomNo.value = property.roomNo || "";
    form.roomSequence = Math.min(9, Math.max(0, Number(property.roomSequence) || 0));
    selectedFloor.value = /^\d{3,}$/.test(roomNo.value) ? roomNo.value.slice(0, -2) : "";
    locationQuery.value = property.community || property.location || "";
    currentStatus.value = property.status;
    existingMedia.value = property.media || [];
    parseLayout(property.layout, property.rentType);
    orientationCustomOpen.value = Boolean(property.orientation && !orientationOptions.includes(property.orientation));
    roomLayoutChosen.value = Boolean(String((property.rentType === "合租" ? property.wholeLayout : property.layout) || "").trim());
    roomCountChosen.value = roomLayoutChosen.value;
    if (property.rentType === "合租" && !roomType.value && (!property.layout || property.layout === property.wholeLayout)) {
      form.layout = "";
    }
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
  await Promise.all([loadPresentation(), loadFacilityCatalog(), loadCommunityDirectory()]);
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
    if (Number(video.size || 0) > MAX_VIDEO_SIZE) {
      uni.showToast({ title: "视频需小于40MB", icon: "none" });
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
  const finalTitle = String(form.title || "").trim() || String(form.community || "").trim();
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
    uni.showModal({ title: "房源信息还未完成", content: publishValidationMessage.value, showCancel: false });
    return;
  }
  saving.value = true;
  let saveStage = "保存房源信息";
  uploadProgress.value = saveStage;
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
      marketingPhrase: "",
      listingSource: form.listingSource || "直租",
      actualLayout: "",
      actualAreaSqm: 0,
      wholeLayout: rentType.value === "合租" ? String(form.wholeLayout || "").trim() : "",
      wholeAreaSqm: rentType.value === "合租" ? Number(form.wholeAreaSqm) || 0 : 0,
      remainingLeaseMonths: form.listingSource === "转租" ? Number(form.remainingLeaseMonths) || 0 : 0,
      transferDeposit: form.listingSource === "转租" ? transferDepositAmount.value : 0,
      community: finalCommunity,
      location: form.location?.trim(),
      areaLabel: geoReady.value ? channels.value[selectedChannelIndex.value]?.title || "" : "",
      buildingNo: buildingNo.value.trim(),
      unitNo: unitNo.value.trim(),
      roomNo: roomNo.value.trim(),
      roomSequence: rentType.value === "合租" ? Number(form.roomSequence) || 0 : 0,
      description: [selectedAmenities.length ? `配套设施：${selectedAmenities.join("、")}` : "", details].filter(Boolean).join("\n"),
    };
    const property = id.value
      ? await updateProperty({ ...payload, id: id.value })
      : await createProperty(payload);
    id.value = property.id;
    saveStage = "上传图片和视频";
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
      saveStage = "更新发布状态";
      uploadProgress.value = "正在发布房源";
      await request("/v1/properties/status", {
        method: "PATCH",
        data: { propertyId: property.id, status: "available", note: "房源编辑页直接发布" },
      });
      currentStatus.value = "available";
    } else {
      currentStatus.value = property.status;
    }
    if (id.value) uni.setStorageSync("xiaopi_property_updated_id", id.value);
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
    const reason = error instanceof Error ? error.message : "未知错误";
    uni.showModal({
      title: `${saveStage}未完成`,
      content: `${reason}\n已填写内容仍保留，可检查后重试。`,
      showCancel: false,
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
    if (form.community) openLocationRepair();
    return;
  }
  if (!requiredReady.value) {
    uni.showModal({ title: "房源信息还未完成", content: publishValidationMessage.value, showCancel: false });
    return;
  }
  uni.showModal({
    title: "确认发布房源",
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
        ><strong>{{ completionCount }}/{{ sectionSteps.length }}</strong><small>已完成</small></view
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
          ><view><strong>位置与门牌</strong></view></view
        ><view class="community-select-entry" @click="openCommunityDirectory"><view><strong>{{ form.community || "选择小区" }}</strong><small>{{ form.community ? (geoReady ? form.location || "地址待补充" : "位置待补充") : "" }}</small></view><b>{{ form.community ? "更换 ›" : "选择 ›" }}</b></view
        ><view v-if="form.community" class="field property-title-field"><text>房源名称</text><input v-model.trim="form.title" maxlength="40" placeholder="默认使用小区名称，可自定义" /></view><button v-if="form.community && communityLocationNeedsRepair" class="community-map-search-button" @click="openLocationRepair">补充小区位置</button
        ><view v-if="communityPickerOpen" class="community-directory-overlay" @click="communityPickerOpen = false">
          <view class="community-directory-sheet" @click.stop>
            <view class="community-directory-head"><view><strong>选择小区</strong><small>已有记录优先；找不到时可在这里搜索地图</small></view><text @click="communityPickerOpen = false">×</text></view>
            <view v-if="communityLocationSearchOpen" class="community-location-repair">
          <view class="community-location-repair-head"><strong>{{ form.community }} · 补充位置</strong><button :disabled="communityLocationSearching" @click="searchCommunityLocation()">{{ communityLocationSearching ? "搜索中…" : "重新搜索" }}</button></view>
          <view v-if="communityLocationSearchOpen" class="community-location-results">
            <view v-if="communityLocationSearchNotice" class="community-location-search-note">{{ communityLocationSearchNotice }}</view>
            <view v-if="communityLocationSearchError" class="community-location-search-error">{{ communityLocationSearchError }}</view>
            <view v-for="item in communityLocationResults" :key="item.id" class="community-location-result" @click="selectCommunityLocation(item)"><view><strong>{{ item.title }}</strong><small>{{ [item.district, item.address].filter(Boolean).join(" · ") || "地图地点" }}</small></view><text>{{ communityLocationSearching ? "保存中…" : "选择位置" }}</text></view>
            <view v-if="!communityLocationSearching && !communityLocationResults.length && !communityLocationSearchError" class="community-location-search-note">暂无结果</view>
          </view>
        </view>
            <button v-if="communityLocationSearchOpen" :disabled="communityLocationSearching" @click="finishLibrarySelection">稍后补充，继续选门牌</button>
            <template v-else>
            <view class="community-directory-search"><text>⌕</text><input v-model.trim="communityDirectoryQuery" confirm-type="search" placeholder="搜索小区库；也可输入地图关键词" @input="handleCommunityDirectoryInput" @confirm="searchMapInCommunityPicker" /></view>
            <view v-if="communityDirectoryLoading" class="community-directory-loading">正在加载小区库…</view>
            <view v-else class="community-picker-sections">
              <view v-if="communityDirectoryError" class="community-directory-load-error">{{ communityDirectoryError }}，当前显示已加载的小区。</view>
              <view v-if="communityDirectoryQuery" class="community-picker-section-title"><strong>小区库匹配</strong></view>
              <view v-if="!communityDirectoryQuery && commonCommunityOptions.length" class="community-picker-common"><button v-for="item in commonCommunityOptions" :key="item.name" @click="selectLibraryCommunity(item.name)">{{ item.name }}</button></view>
              <view v-if="!communityDirectoryQuery" class="community-picker-section-title"><strong>预设小区</strong></view>
              <scroll-view scroll-y class="community-directory-list" :show-scrollbar="false">
                <view class="community-directory-grid"><view v-for="item in filteredCommunityOptions" :key="item.name" class="community-community-card" @click="selectLibraryCommunity(item.name)"><strong>{{ item.name }}</strong></view></view>
                <view v-if="!filteredCommunityOptions.length" class="community-directory-empty">未找到匹配小区</view>
            <view v-if="communityDirectoryQuery.length >= 2" class="community-map-search-area">
              <view v-if="pickerMapResults.length" class="community-picker-section-title"><strong>地图结果</strong><small v-if="locationSearching">搜索中…</small></view>
              <view v-if="pickerMapResults.length" class="community-map-results"><view v-for="item in pickerMapResults" :key="item.id" class="community-directory-option map-result" @click="applyLocationResult(item)"><view><strong>{{ item.title }}</strong><small>{{ [item.district, item.address].filter(Boolean).join(" · ") || "地图地点" }}</small></view><text>地图结果 · 选择</text></view></view>
              <button class="community-map-search-button" :disabled="locationSearching" @click="searchMapInCommunityPicker">{{ locationSearching ? "正在搜索地图…" : locationMapSearched ? "重新搜索地图" : "搜索地图" }}</button>
              <view v-if="canUseTypedCommunity" class="location-custom-option"><view><strong>地图没有合适结果？</strong><small>可用输入名称继续录入，之后仍可补充位置</small></view><text @click="useTypedCommunity">使用名称 ›</text></view>
            </view>
              </scroll-view>
            </view>
            </template>
          </view>
        </view
        ><view v-if="form.community" class="unit-selector-card" :class="{ selected: Boolean(roomNo) }" @click="openStructurePicker"
          ><view><strong>{{ roomNo ? "已选门牌" : "门牌信息（选填）" }}</strong><text>{{ structureSummary }}</text></view><b>{{ roomNo ? "重选 ›" : "选择 ›" }}</b></view
          ><view v-if="form.community" class="house-layout-summary" @click="openHouseQuickSettings"
          ><view class="house-layout-summary-action"><text>快捷设置</text><b>›</b></view><view class="house-layout-summary-copy"><strong>房屋与出租信息</strong><small>{{ houseLayoutSummary }}</small><small>{{ Number(form.rentMonthly) || 0 }} 元/月 · {{ form.depositPayment || "押付待填写" }}</small></view></view
        ></view
      ><view v-show="activeSection === 1" class="section-card"
        ><view class="section-head"
          ><text>02</text
          ><view
            ><strong>图片与视频（选填）</strong></view
          ></view
        ><view class="media-guidance"
          ><strong>封面素材</strong
          ><text>最多 9 张图片、1 个视频</text></view
        ><view class="cover-focus-control"
          ><view><strong>封面焦点（选填）</strong><text>主体偏边时调整</text></view
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
          ><view class="pick-property-video" @click="pick('video')"
            ><strong>＋ 视频</strong><small>{{ videoCount }}/1</small></view
          ></view
        ></view
        ><view v-if="activeSection === 1" class="media-limit-hint"
        >图片 ≥640px / ≤10MB · 视频 ≤40MB / 60秒</view
      ><view v-if="activeSection === 1" class="title-composer final-title-composer"
        ><view><strong>列表展示规则</strong></view
        ><view class="auto-title"><text>房源名称预览</text><strong>{{ customerGeneratedTitle }}</strong></view></view>
      <view v-if="activeSection === 1" class="final-review">
        <view class="final-review-head"><view><strong>发布前核对</strong><small>可点上方步骤返回修改</small></view><text>{{ completionCount }}/{{ sectionSteps.length }}</text></view>
        <view class="final-review-grid">
          <view :class="{ ready: sectionReady[0] }"><text>位置与房屋概况</text><small>{{ form.community || "待选择" }}</small></view>
          <view :class="{ ready: sectionReady[0] }"><text>房屋与出租方式</text><small>{{ form.layout || "待填写" }}</small></view>
          <view :class="{ ready: sectionReady[0] }"><text>租金入住</text><small>{{ Number(form.rentMonthly) || 0 }}/月 {{ form.depositPayment || "" }}</small></view>
          <view :class="{ ready: sectionReady[1] }"><text>图片素材（选填）</text><small>{{ mediaCount }}/9 张图片 · {{ videoCount }} 个视频</small></view>
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
        ><view v-if="activeSection < sectionSteps.length - 1" class="step-actions" :class="{ 'three-actions': activeSection > 0 }"
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
          ><template v-else><button :disabled="saving" @click="save()">保存草稿</button><button :disabled="saving || !requiredReady" @click="save('preview')">预览草稿</button><button class="primary" :disabled="saving" @click="confirmPublish">{{ saving ? "处理中..." : "发布房源" }}</button></template
          ></view
        ></view
      ><view v-if="structureStep" class="structure-modal" @click="structureStep = ''"
        ><view class="structure-sheet" :class="{ 'quick-structure-sheet': structureStep === 'quick' }" @click.stop
          ><view class="structure-sheet-head"
          ><view><strong>{{ structureStep === "quick" ? ("房源快捷设置") : structureStep === "building" ? "选择楼栋" : structureStep === "unit" ? "选择单元" : "选择房号" }}</strong><small>{{ structureStep === "quick" ? (form.community ? `${form.community}${roomNo ? ` · ${structureSummary}` : ""}` : "户型、朝向、楼层与合租房型") : form.community }}</small></view><view class="structure-head-actions"><text v-if="structureStep !== 'building' && structureStep !== 'quick'" class="structure-back" @click="returnToPreviousStructureStep">‹ 返回</text><text class="structure-close" @click="structureStep = ''">×</text></view></view
          ><view v-if="structureStep !== 'quick'" class="structure-progress structure-progress-three"><text :class="{ active: structureStep === 'building', available: canReturnToStructureStep('building') }" @click="returnToStructureStep('building')">楼栋</text><text :class="{ active: structureStep === 'unit', available: canReturnToStructureStep('unit') }" @click="returnToStructureStep('unit')">单元</text><text :class="{ active: structureStep === 'room', available: canReturnToStructureStep('room') }" @click="returnToStructureStep('room')">房号</text></view
          ><view v-if="hasDoorplateCatalog && structureStep !== 'quick'" class="doorplate-entry-switch"><text :class="{ active: doorplateEntryMode === 'catalog' }" @click="switchDoorplateEntryMode('catalog')">选择已有门牌</text><text :class="{ active: doorplateEntryMode === 'manual' }" @click="switchDoorplateEntryMode('manual')">自定义输入</text></view
          ><scroll-view scroll-y class="structure-grid-scroll"
            ><view v-if="structureStep === 'quick'" class="doorplate-quick-settings">
              <view class="quick-setting-group">
                <view class="quick-group-title">租赁信息</view>
                <view class="quick-setting-pair quick-rental-grid">
                <view class="quick-setting-row"><text>租赁类型</text><view class="quick-choice-list"><button :class="{ active: rentType === '整租' }" @click="setRentType('整租')">整租</button><button class="choose-shared-rental" :class="{ active: rentType === '合租' }" @click="setRentType('合租')">合租</button></view></view>
                  <view class="quick-setting-row">
                    <text>月租 *</text>
                    <view class="quick-area-input">
                      <input v-model.number="form.rentMonthly" type="number" placeholder="如：2600" />
                      <small>元/月</small>
                    </view>
                  </view>
                <view class="quick-setting-row"><text>房源来源</text><view class="quick-choice-list"><button :class="{ active: form.listingSource === '直租' }" @click="setListingSource('直租')">直租</button><button :class="{ active: form.listingSource === '转租' }" @click="setListingSource('转租')">转租</button></view></view>
                  <view class="quick-setting-row option-field">
                    <text>中介费（选填）</text>
                    <checkbox-group @change="toggleNoAgentFee">
                      <label><checkbox value="no-agent-fee" :checked="noAgentFee" color="#40543f" />无中介费</label>
                    </checkbox-group>
                  </view>
                </view
        ><view class="term-block"><text>押付方式 *</text><view class="payment-options"><view class="payment-fixed">押一</view><view v-for="cycle in paymentCycles" :key="cycle" :class="{ active: paymentCycle === cycle }" @click="setPaymentCycle(cycle)">{{ cycle }}</view></view></view
        ><view v-if="form.listingSource === '转租'" class="transfer-settlement"><view class="transfer-heading"><view><strong>转租结算</strong><small>只记录剩余月数与押金，具体费用线下沟通</small></view><text>转租</text></view><view class="transfer-choice transfer-stepper"><text>剩余租期</text><view><button @click="adjustTransferMonths(-1)">−</button><input :value="form.remainingLeaseMonths || ''" type="number" maxlength="2" placeholder="月" @input="handleTransferMonthsInput" /><text>个月</text><button @click="adjustTransferMonths(1)">＋</button></view></view><view class="transfer-choice"><text>承接押金（选填）</text><view><button v-for="multiplier in [0, 1, 2]" :key="multiplier" :class="{ active: transferDepositMultiplier === multiplier }" @click="setTransferDeposit(multiplier)">{{ multiplier === 0 ? '无需承接' : `押${multiplier}` }}</button></view></view><view class="transfer-summary"><small>当前选择</small><strong>剩余 {{ form.remainingLeaseMonths || 0 }} 个月 · {{ transferDepositMultiplier ? `承接押金 ¥${transferDepositAmount}` : '无需承接押金' }}</strong></view></view
        ><view class="quick-move-in"
          ><view class="field"
            ><text>可入住日期（选填）</text
            ><input v-model.trim="form.availableDate" maxlength="32" placeholder="默认拎包入住，可填写具体日期" /></view></view
        ><view class="term-block"><text>水电与杂费（选填）</text><radio-group class="utility-options" @change="setUtilities($event.detail.value)"><label v-for="option in utilityOptions" :key="option" :class="{ active: form.utilitiesText === option }"><radio :value="String(utilityOptions.indexOf(option))" :checked="form.utilitiesText === option" color="#40543f" />{{ option }}</label></radio-group></view>
              </view>
              <view class="quick-setting-group">
                <view class="quick-group-title">户型与房屋结构</view>
              <view class="quick-setting-row quick-room-count"><text>{{ rentType === '合租' ? '整套户型' : '户型' }}</text><view class="quick-choice-list"><button v-for="count in [1, 2, 3, 4]" :key="count" :class="{ active: roomCountChosen && roomCount === String(count) }" @click="setRoomCount(count)">{{ count }}室</button><button :class="{ active: roomCount === 'custom' }" @click="setRoomCount('custom')">自定义</button><input v-if="roomCount === 'custom'" class="quick-custom-count" :value="customRoomCount" type="number" maxlength="1" placeholder="1-9" @input="setCustomRoomCount" /></view></view>
              <view class="quick-setting-pair">
                <view class="quick-setting-row"><text>客厅</text><view class="quick-choice-list"><button v-for="count in [0, 1, 2, 3]" :key="count" :class="{ active: roomLayoutChosen && hallCount === String(count) }" @click="hallCount = String(count); roomLayoutChosen = true; syncLayout()">{{ count === 0 ? '无' : `${count}厅` }}</button></view></view>
                <view class="quick-setting-row"><text>厨房</text><view class="quick-choice-list"><button v-for="item in [{ value: '1', label: '有' }, { value: '0', label: '无' }]" :key="item.value" :class="{ active: roomLayoutChosen && kitchenCount === item.value }" @click="kitchenCount = item.value; roomLayoutChosen = true; syncLayout()">{{ item.label }}</button></view></view>
                <view class="quick-setting-row"><text>卫生间</text><view class="quick-choice-list"><button v-for="count in [1, 2, 3, 4]" :key="count" :class="{ active: roomLayoutChosen && bathroomCount === String(count) }" @click="bathroomCount = String(count); roomLayoutChosen = true; syncLayout()">{{ count }}个</button></view></view>
                <view class="quick-setting-row"><text>阳台</text><view class="quick-choice-list"><button v-for="count in [0, 1, 2, 3, 4]" :key="count" :class="{ active: roomLayoutChosen && balcony === String(count) }" @click="balcony = String(count); roomLayoutChosen = true; syncLayout()">{{ count === 0 ? '无' : `${count}个` }}</button></view></view>
              </view>
              </view>
              <view v-if="rentType === '合租'" class="quick-setting-group">
                <view class="quick-group-title">合租房间</view>
                <view class="quick-setting-row quick-setting-row-wide"><text>本次出租房型</text><view class="quick-choice-list"><button v-for="option in roomTypes" :key="option" :class="{ active: roomType === option }" @click="setSharedRoomType(option)">{{ option }}</button></view></view>
                <view class="quick-setting-pair">
                <view class="quick-setting-row"><text>独立卫生间（选填）</text><view class="quick-choice-list"><button :class="{ active: privateBathroom }" @click="privateBathroom = true; toiletType = '独卫'; syncLayout()">有</button><button :class="{ active: !privateBathroom }" @click="privateBathroom = false; toiletType = '非独卫'; syncLayout()">无</button></view></view>
                <view class="quick-setting-row"><text>飘窗（选填）</text><view class="quick-choice-list"><button :class="{ active: hasBayWindow }" @click="hasBayWindow = true; bayWindow = '有飘窗'; syncLayout()">有</button><button :class="{ active: !hasBayWindow }" @click="hasBayWindow = false; bayWindow = '无飘窗'; syncLayout()">无</button></view></view>
                </view>
                <view v-if="roomLayoutReady" class="quick-setting-row room-sequence-quick"><text>出租房间（选填）</text><view class="quick-choice-list"><button :class="{ active: !form.roomSequence }" @click="setRoomSequence(0)">不指定</button><button v-for="number in boundedRoomCount" :key="number" :class="{ active: Number(form.roomSequence) === number }" @click="setRoomSequence(number)">{{ number }}号房</button></view></view>
              </view>
              <view class="quick-disclosure" @click="quickMoreOpen = !quickMoreOpen"><view><strong>更多房屋信息</strong><small>面积、朝向、楼层、电梯、地铁</small></view><text>{{ quickMoreOpen ? "收起　⌃" : "按需填写　⌄" }}</text></view>
              <view v-if="quickMoreOpen" class="quick-setting-group">
                <view class="quick-group-title">房屋补充信息</view>
              <view class="quick-setting-pair">
                <view class="quick-setting-row"><text>电梯（选填）</text><view class="quick-choice-list"><button :class="{ active: form.elevator === '有电梯' }" @click="form.elevator = '有电梯'">有电梯</button><button :class="{ active: form.elevator === '无电梯' }" @click="form.elevator = '无电梯'">无电梯</button></view></view>
                <view class="quick-setting-row"><text>楼层（选填）</text><view class="quick-floor-control"><text>{{ automaticFloorInfo ? `门牌推算 ${automaticFloorInfo}` : '可手动输入' }}</text><view class="quick-floor-input-wrap"><input class="quick-floor-input" type="number" maxlength="2" :value="String(form.floorInfo || automaticFloorInfo || '').replace('楼', '')" placeholder="楼层" @input="setFloorInfo" /><small>楼</small></view></view></view>
              </view>
              <view class="quick-setting-row quick-setting-row-wide">
                <text>{{ rentType === "合租" ? "出租房间朝向" : "朝向" }}（选填）</text>
                <picker :range="orientationPickerOptions"
                  :value="Math.max(0, orientationCustomOpen ? orientationOptions.length : orientationOptions.indexOf(form.orientation || ''))"
                  @change="setOrientation(orientationPickerOptions[Number($event.detail.value)] === '清除朝向' ? '' : orientationPickerOptions[Number($event.detail.value)])"
                >
                  <view class="quick-orientation-picker">{{ form.orientation || "选择朝向" }} <text>›</text></view>
                </picker>
                <input v-if="orientationCustomOpen" v-model.trim="form.orientation" class="quick-orientation-input" maxlength="32" placeholder="输入朝向" />
              </view>
              <view class="quick-setting-pair quick-setting-metrics">
                <view class="quick-setting-row"><text>{{ rentType === '合租' ? '出租面积' : '房屋面积' }}（选填）</text><view class="quick-area-input"><input v-model.number="form.areaSqm" type="number" placeholder="平方米" /><small>㎡</small></view></view>
                <view v-if="detailModuleEnabled('location')" class="quick-setting-row"><text>步行至地铁（选填）</text><view class="quick-area-input"><input v-model.number="form.metroMinutes" type="number" placeholder="分钟" /><small>分钟</small></view></view>
                <view v-if="rentType === '合租'" class="quick-setting-row"><text>整套面积（选填）</text><view class="quick-area-input"><input v-model.number="form.wholeAreaSqm" type="number" placeholder="平方米" /><small>㎡</small></view></view>
              </view>
              </view>
              <view v-if="detailModuleEnabled('facilities') || detailModuleEnabled('description')" class="quick-disclosure" @click="optionalDetailsOpen = !optionalDetailsOpen">
          <strong>配套、描述与入住规则（选填）</strong>
          <small>{{ optionalDetailsOpen ? "收起" : "展开" }}</small>
        </view>
        <template v-if="optionalDetailsOpen">
        <checkbox-group v-if="detailModuleEnabled('facilities')"
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
          ><text>入住规则（选填）</text
          ><input v-model.trim="form.rulesText" maxlength="200" :placeholder='copyText("如：可做饭，宠物需沟通，女生优先")' /></view
        ><view v-if="!detailModuleEnabled('facilities') && !detailModuleEnabled('description')" class="module-entry-disabled">当前后台未启用“房屋设施”或“房源简介”，此步无需填写。</view>
        </template>
            </view
            ><view v-else-if="!hasDoorplateCatalog || doorplateEntryMode === 'manual'" class="manual-doorplate-entry"><text v-if="!hasDoorplateCatalog">该小区暂无门牌库数据，可直接自定义填写。</text><text v-else>当前步骤：{{ structureStep === 'building' ? '楼栋' : structureStep === 'unit' ? '单元' : '房号' }}</text><input v-model="manualDoorplate" maxlength="24" :placeholder="structureStep === 'building' ? '输入楼栋，如 3' : structureStep === 'unit' ? '输入单元，如 1' : '输入房号，如 3301'" @confirm="confirmManualDoorplate" /><button :disabled="!manualDoorplate.trim()" @click="confirmManualDoorplate">{{ structureStep === 'room' ? '确认房号' : '确认并继续' }}</button></view
            ><view v-else-if="doorplateEntryMode === 'catalog' && structureStep === 'building'" class="structure-grid"><view v-for="rule in buildingOptions" :key="rule" class="structure-grid-item" :class="{ active: buildingNo === rule }" @click="selectBuildingRule(rule)">{{ rule }}</view></view
            ><view v-else-if="doorplateEntryMode === 'catalog' && structureStep === 'unit'" class="structure-grid"><view v-for="unit in unitOptions" :key="unit" class="structure-grid-item" :class="{ active: unitNo === unit }" @click="chooseUnit(unit)">{{ unit }} 单元</view></view
            ><view v-else-if="doorplateEntryMode === 'catalog'" class="structure-grid"><view v-for="room in roomOptions" :key="room.roomNo" class="structure-grid-item" :class="{ active: roomNo === room.roomNo }" @click="chooseRoom(room)">{{ room.roomNo }}</view></view
          ></scroll-view
          ><view class="structure-sheet-footer"><template v-if="structureStep === 'quick'"><button v-if="roomNo" @click="structureStep = 'room'">重选房号</button><button class="quick-done" @click="structureStep = ''">完成</button></template><template v-else><button v-if="structureStep === 'unit'" @click="chooseUnit('')">无单元，跳过</button><text>不填门牌也可以继续录入</text><button @click="structureStep = ''">暂不选择</button></template></view
        ></view
      ></view
      ></template
    ></view
  >
</template>
<style scoped src="./property-edit.css"></style>
