<script setup lang="ts">
import { onLoad, onShow } from "@dcloudio/uni-app";
import { computed, reactive, ref, watch } from "vue";
import { API_ORIGIN, request, resolveAssetUrl } from "../../api/client";
import {
  createProperty,
  deleteProperty,
  getPublicConfig,
  listBuildingRules,
  listProperties,
  replacePropertyMedia,
  saveBuildingRule,
  searchLocations,
  updateProperty,
  type LocationSearchItem,
  type BuildingRule,
  type Property,
  type PropertyMedia,
} from "../../api/properties";
import { getSession, hasRole } from "../../auth/session";
import { amenityIcon } from "../../utils/amenities";
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
  pendingCoverSelected = ref(false);
const { presentation, loadPresentation, pageStyle } = usePagePresentation(
  "adminPropertyEdit",
  "房源编辑",
  "完善资料后即可保存发布",
);
const currentLocation = ref<{ latitude: number; longitude: number } | null>(null);
const locationQuery = ref("");
const locationSearching = ref(false);
const locationResults = ref<LocationSearchItem[]>([]);
let locationSearchTimer: ReturnType<typeof setTimeout> | undefined;
const form = reactive<Partial<Property>>({
  title: "",
  community: "",
  location: "",
  buildingNo: "",
  unitNo: "",
  roomNo: "",
  latitude: 0,
  longitude: 0,
  layout: "",
  rentMonthly: 0,
  feeText: "无中介费",
  rentType: "整租",
  availableDate: "拎包入住",
  depositPayment: "押一付一",
  utilitiesText: "民水民电",
  areaSqm: 0,
  floorInfo: "",
  orientation: "",
  elevator: "",
  metroMinutes: 0,
  verificationDate: "",
  rulesText: "",
  description: "",
  ownerId: "xiaopi-admin",
});
const roomCount = ref("一室"),
  hallCount = ref("一厅"),
  rentType = ref("整租"),
  roomType = ref("主卧"),
  bathroomCount = ref("一卫"),
  balcony = ref("有阳台"),
  toiletType = ref("独卫"),
  bayWindow = ref("有飘窗");
const buildingNo = ref("");
const unitNo = ref("");
const roomNo = ref("");
const selectedFloor = ref("");
const buildingRules = ref<BuildingRule[]>([]);
type StructureStep = "building" | "unit" | "room" | "";
const structureStep = ref<StructureStep>("");
const showStructureSetup = ref(false);
const ruleSaving = ref(false);
const ruleDraft = reactive({ unitCount: 3, floorStart: 1, floorCount: 33, householdsPerFloor: 6 });
const titleParts = ref(["community", "unit", "layout", "rentType"]);
const titlePartOptions = [
  { key: "community", label: "小区" },
  { key: "unit", label: "门牌" },
  { key: "floor", label: "楼层" },
  { key: "layout", label: "户型" },
  { key: "rentType", label: "整租/合租" },
  { key: "area", label: "面积" },
];
const toggleTitlePart = (key: string) => {
  titleParts.value = titleParts.value.includes(key)
    ? titleParts.value.filter((item) => item !== key)
    : [...titleParts.value, key];
};
const moveTitlePart = (key: string, direction: -1 | 1) => {
  const index = titleParts.value.indexOf(key);
  const nextIndex = index + direction;
  if (index < 0 || nextIndex < 0 || nextIndex >= titleParts.value.length) return;
  const next = [...titleParts.value];
  [next[index], next[nextIndex]] = [next[nextIndex], next[index]];
  titleParts.value = next;
};
const orderedTitlePartOptions = computed(() => [
  ...titleParts.value.map((key) => titlePartOptions.find((option) => option.key === key)).filter(Boolean),
  ...titlePartOptions.filter((option) => !titleParts.value.includes(option.key)),
] as typeof titlePartOptions);
const amenities = ref(["冰箱", "洗衣机", "空调", "热水器"]),
  amenityOptions = [
    "冰箱",
    "洗衣机",
    "空调",
    "热水器",
    "电视",
    "宽带",
    "燃气",
    "油烟机",
    "沙发",
    "衣柜",
    "书桌",
    "床",
  ];
const rooms = ["一室", "两室", "三室", "四室", "五室及以上"],
  halls = ["一厅", "两厅", "三厅", "无厅"],
  rentTypes = ["整租", "合租"],
  roomTypes = ["主卧", "次卧", "单间", "床位"],
  bathroomCounts = ["一卫", "两卫", "三卫及以上"],
  balconies = ["有阳台", "无阳台"],
  toilets = ["独卫", "非独卫"],
  windows = ["有飘窗", "无飘窗"],
  paymentCycles = ["付一", "付二", "付三", "付六"],
  utilityOptions = ["民水民电", "民水民电 · 物业另计", "水电物业全包", "商水商电"];
const paymentCycle = ref("付一");
const noAgentFee = ref(true);
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
const requiredReady = computed(() =>
  Boolean(
    form.title?.trim() &&
    form.community?.trim() &&
    form.location?.trim() &&
    form.layout?.trim() &&
    geoReady.value &&
    Number(form.rentMonthly) > 0 &&
    Boolean(form.depositPayment?.trim()),
  ),
);
const sectionSteps = ["位置门牌", "户型出租", "环境配套", "租金入住", "素材发布"];
const goToSection = (index: number) => {
  activeSection.value = Math.max(0, Math.min(sectionSteps.length - 1, index));
  uni.pageScrollTo({ scrollTop: 0, duration: 220 });
};
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
            color: "#173f3c",
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
  const nearestIndex = channels.value.reduce((bestIndex, channel, index) => {
    const best = channels.value[bestIndex];
    const bestDistance = Number(best?.latitude) && Number(best?.longitude)
      ? distanceKm(latitude, longitude, Number(best.latitude), Number(best.longitude))
      : Number.POSITIVE_INFINITY;
    const candidateDistance = Number(channel.latitude) && Number(channel.longitude)
      ? distanceKm(latitude, longitude, Number(channel.latitude), Number(channel.longitude))
      : Number.POSITIVE_INFINITY;
    return candidateDistance < bestDistance ? index : bestIndex;
  }, 0);
  if (Number.isFinite(Number(channels.value[nearestIndex]?.latitude)) && Number(channels.value[nearestIndex]?.latitude)) {
    selectedChannelIndex.value = nearestIndex;
  }
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
const runLocationSearch = async () => {
  const keyword = locationQuery.value.trim();
  if (keyword.length < 2) {
    locationResults.value = [];
    return;
  }
  locationSearching.value = true;
  try {
    const results = await searchLocations(
      keyword,
      channelLatitude.value,
      channelLongitude.value,
    );
    // 商户常把小区名写在括号中；录入房源时只保留可直接作为小区的地点。
    locationResults.value = results.filter((item) => !/[()（）]/.test(item.title || ""));
  } catch (error) {
    locationResults.value = [];
    uni.showToast({
      title: error instanceof Error ? error.message : "地址搜索失败",
      icon: "none",
    });
  } finally {
    locationSearching.value = false;
  }
};
const scheduleLocationSearch = () => {
  if (locationSearchTimer) clearTimeout(locationSearchTimer);
  locationSearchTimer = setTimeout(runLocationSearch, 450);
};
const applyLocationResult = (item: LocationSearchItem) => {
	const changedCommunity = form.community !== item.title;
  form.community = item.title;
  form.location = [item.district, item.address].filter(Boolean).join(" · ");
  form.latitude = Number(item.latitude.toFixed(7));
  form.longitude = Number(item.longitude.toFixed(7));
  selectNearestChannel(form.latitude, form.longitude);
  locationQuery.value = item.title;
  locationResults.value = [];
	if (changedCommunity) {
		buildingNo.value = "";
		unitNo.value = "";
		roomNo.value = "";
		selectedFloor.value = "";
		form.floorInfo = "";
		showStructureSetup.value = false;
	}
  void loadBuildingRules();
  structureStep.value = "building";
};
const choosePropertyLocation = () => {
  uni.chooseLocation({
    latitude: mapLatitude.value,
    longitude: mapLongitude.value,
    success: (result) => {
      form.latitude = Number(result.latitude.toFixed(7));
      form.longitude = Number(result.longitude.toFixed(7));
      selectNearestChannel(form.latitude, form.longitude);
      form.location = result.address || result.name || form.location;
		if (!form.community?.trim() && result.name) form.community = result.name;
      void loadBuildingRules();
		if (form.community?.trim()) structureStep.value = "building";
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
  amenities.value.length > 0,
  Boolean(Number(form.rentMonthly) > 0 && form.depositPayment?.trim()),
  mediaCount.value > 0,
]);
const completionCount = computed(
  () => sectionReady.value.filter(Boolean).length,
);
const generatedTitle = computed(() => {
  const community = String(form.community || "").trim();
  if (!community) return "选择小区后自动生成";
  const values: Record<string, string> = {
    community,
    unit: [buildingNo.value.trim(), unitNo.value.trim() && `${unitNo.value.trim()}单元`, roomNo.value.trim() && `${roomNo.value.trim()}室`].filter(Boolean).join(" · "),
    floor: selectedFloor.value ? `${selectedFloor.value}层` : "",
    layout: rentType.value === "合租" ? roomType.value : `${roomCount.value}${hallCount.value}`,
    rentType: rentType.value,
    area: Number(form.areaSqm) > 0 ? `${form.areaSqm}㎡` : "",
  };
  const parts = titleParts.value.map((key) => values[key]).filter(Boolean);
  return parts.join(" · ") || community;
});
watch([() => form.community, roomCount, hallCount, rentType, buildingNo, unitNo, selectedFloor, roomNo, () => form.areaSqm, titleParts], () => {
  form.title = String(form.community || "").trim()
    ? generatedTitle.value
    : "";
}, { immediate: true });
const defaultBuildingRules = Array.from({ length: 30 }, (_, index) => ({
  id: `default-building-${index + 1}`,
  community: "",
  buildingNo: `${index + 1}栋`,
  unitCount: 3,
  floorStart: 1,
  floorCount: 33,
  householdsPerFloor: 6,
}));
// Until a community has verified building data, use one predictable hierarchy
// for every selected community. Exact rules saved later replace only their
// matching building and leave the remainder usable.
const buildingOptions = computed(() => {
  const exactRules = new Map(buildingRules.value.map((rule) => [rule.buildingNo, rule]));
  const defaults = defaultBuildingRules.map((rule) => ({
    ...rule,
    community: String(form.community || ""),
    ...(exactRules.get(rule.buildingNo) || {}),
  }));
  const extras = buildingRules.value.filter((rule) => !defaultBuildingRules.some((item) => item.buildingNo === rule.buildingNo));
  return [...defaults, ...extras];
});
const selectedBuildingRule = computed(() =>
  buildingOptions.value.find((rule) => rule.buildingNo === buildingNo.value.trim()) || null,
);
const automaticFloorInfo = computed(() => {
  return selectedFloor.value ? `${selectedFloor.value}楼` : "";
});
const hasStructureRule = computed(() => Boolean(selectedBuildingRule.value));
const unitOptions = computed(() => {
  const total = selectedBuildingRule.value?.unitCount || 0;
  return Array.from({ length: total }, (_, index) => String(index + 1));
});
const roomOptions = computed(() => {
  const rule = selectedBuildingRule.value;
  if (!rule) return [];
  const floors = Array.from(
    { length: rule.floorCount },
    (_, index) => String(rule.floorStart + rule.floorCount - index - 1),
  );
  return floors.flatMap((floor) =>
    Array.from({ length: rule.householdsPerFloor }, (_, index) => {
      const household = String(index + 1).padStart(2, "0");
      return { floor, roomNo: `${floor}${household}` };
    }),
  );
});
const generatedRoomNo = computed(() => {
  return roomNo.value;
});
const loadBuildingRules = async () => {
  const community = String(form.community || "").trim();
  if (!community) {
    buildingRules.value = [];
    return;
  }
  buildingRules.value = await listBuildingRules(community).catch(() => []);
};
const selectBuildingRule = (rule: BuildingRule) => {
  if (!rule) return;
  buildingNo.value = rule.buildingNo;
  unitNo.value = "";
  selectedFloor.value = "";
  roomNo.value = "";
  form.floorInfo = "";
  structureStep.value = "unit";
};
const chooseUnit = (unit: string) => {
  unitNo.value = unit;
  selectedFloor.value = "";
  roomNo.value = "";
  form.floorInfo = "";
  structureStep.value = "room";
};
const chooseRoom = (room: { floor: string; roomNo: string }) => {
  selectedFloor.value = room.floor;
  roomNo.value = room.roomNo;
  form.floorInfo = automaticFloorInfo.value;
  structureStep.value = "";
};
const structureStepIndex = (step: StructureStep) =>
  (["building", "unit", "room"] as StructureStep[]).indexOf(step);
const canReturnToStructureStep = (step: StructureStep) => {
  if (step === "building") return true;
  if (step === "unit") return Boolean(buildingNo.value);
  return Boolean(buildingNo.value && unitNo.value);
};
const returnToStructureStep = (step: StructureStep) => {
  if (!step || !canReturnToStructureStep(step)) return;
  if (step === "building") {
    unitNo.value = "";
    selectedFloor.value = "";
    roomNo.value = "";
    form.floorInfo = "";
  } else if (step === "unit") {
    roomNo.value = "";
    selectedFloor.value = "";
    form.floorInfo = "";
  }
  structureStep.value = step;
};
const returnToPreviousStructureStep = () => {
  const currentIndex = structureStepIndex(structureStep.value);
  if (currentIndex <= 0) return;
  returnToStructureStep((["building", "unit", "room"] as StructureStep[])[currentIndex - 1]);
};
const openStructurePicker = () => {
  if (!form.community?.trim()) return;
  structureStep.value = buildingNo.value
    ? unitNo.value
      ? "room"
      : "unit"
    : "building";
};
const structureSummary = computed(() =>
  [buildingNo.value, unitNo.value && `${unitNo.value}单元`, roomNo.value && `${roomNo.value}室`]
    .filter(Boolean)
    .join("") || "未选择",
);
const saveCurrentBuildingRule = async () => {
  const community = String(form.community || "").trim();
  const building = buildingNo.value.trim();
  if (!community || !building) {
    uni.showToast({ title: "请先选择小区并填写楼栋", icon: "none" });
    return;
  }
  ruleSaving.value = true;
  try {
    const saved = await saveBuildingRule({ community, buildingNo: building, ...ruleDraft });
    const index = buildingRules.value.findIndex((rule) => rule.buildingNo === saved.buildingNo);
    if (index >= 0) buildingRules.value.splice(index, 1, saved);
    else buildingRules.value.push(saved);
    buildingRules.value = [...buildingRules.value];
    uni.showToast({ title: "楼栋规则已保存" });
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "保存失败", icon: "none" });
  } finally {
    ruleSaving.value = false;
  }
};
const syncLayout = () => {
  form.layout = rentType.value === "整租"
    ? `${roomCount.value}${hallCount.value} · 整租 · ${bathroomCount.value} · ${balcony.value}`
    : `合租 · ${roomType.value} · ${toiletType.value} · ${bayWindow.value}`;
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
const parseLayout = (value = "") => {
  const parts = value.split(" · ");
  if (parts[1] === "整租") {
    const roomHall = parts[0];
    const hall = roomHall.match(/(一厅|两厅|三厅|无厅)$/)?.[0] || "一厅";
    roomCount.value = roomHall.slice(0, -hall.length) || "一室";
    hallCount.value = hall;
    rentType.value = "整租";
    // 兼容旧版：户型 · 整租 · 整套 · 独卫 · 带飘窗。
    bathroomCount.value = parts[2] === "整套" ? (parts[3] || "一卫") : (parts[2] || "一卫");
    balcony.value = parts[2] === "整套" ? "有阳台" : (parts[3] || "有阳台");
  } else if (parts[0] === "合租") {
    rentType.value = "合租";
    roomType.value = parts[1] || "主卧";
    toiletType.value = parts[2] || "独卫";
    bayWindow.value = parts[3] || "有飘窗";
  } else if (parts[1] === "合租") {
    rentType.value = "合租";
    roomType.value = parts[2] || "主卧";
    toiletType.value = parts[3] || "独卫";
    bayWindow.value = parts[4] || "有飘窗";
  }
};
const loadProperty = async () => {
  if (!id.value) return;
  loading.value = true;
  try {
    const property = (await listProperties()).find(
      (item) => item.id === id.value,
    );
    if (!property) throw new Error("房源不存在");
    Object.assign(form, property);
    paymentCycle.value = paymentCycles.find((item) => String(property.depositPayment || "").includes(item)) || "付一";
    noAgentFee.value = !String(property.feeText || "").includes("有中介费") || String(property.feeText || "").includes("无中介费");
    buildingNo.value = property.buildingNo || "";
    unitNo.value = property.unitNo || "";
    roomNo.value = property.roomNo || "";
    selectedFloor.value = /^\d{3,}$/.test(roomNo.value) ? roomNo.value.slice(0, -2) : "";
    locationQuery.value = property.community || property.location || "";
    currentStatus.value = property.status;
    existingMedia.value = property.media || [];
    parseLayout(property.layout);
    const match = property.description.match(/^配套设施：([^\n]*)\n?/);
    amenities.value = match?.[1] ? match[1].split("、").filter(Boolean) : [];
    form.description = property.description.replace(/^配套设施：[^\n]*\n?/, "");
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
onLoad(async (query) => {
  if (!hasRole("admin")) {
    uni.showToast({ title: "暂无运营权限", icon: "none" });
    uni.switchTab({ url: "/pages/me/index" });
    return;
  }
  id.value = String(query?.id || "");
  await loadPresentation();
  syncLayout();
  loadChannels();
  loadProperty();
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
  const result = uni.getStorageSync("xiaopi-property-image-crop-result") as
    | { index?: number; path?: string; width?: number; height?: number; size?: number }
    | undefined;
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
      url: `${API_ORIGIN}/api/v1/media/upload`,
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
const save = async (openPreview = false) => {
  if (saving.value) return;
  const minimumDraftReady = Boolean(form.title?.trim() && form.community?.trim());
  if (!minimumDraftReady) {
    uni.showToast({
      title: "草稿至少填写标题和小区",
      icon: "none",
    });
    return;
  }
  if (openPreview && !requiredReady.value) {
    uni.showToast({
      title: "请完成标题、小区、位置、地图定位、户型和月租",
      icon: "none",
    });
    return;
  }
  if (openPreview && mediaCount.value === 0) {
    uni.showToast({
      title: "发布前请至少上传一张实拍图片",
      icon: "none",
    });
    return;
  }
  saving.value = true;
  uploadProgress.value = "保存房源信息";
  try {
    const details = String(form.description || "").trim();
    const payload = {
      ...form,
      title: generatedTitle.value,
      community: form.community?.trim(),
      location: form.location?.trim(),
      buildingNo: buildingNo.value.trim(),
      unitNo: unitNo.value.trim(),
      roomNo: roomNo.value.trim(),
      description: `配套设施：${amenities.value.join("、")}\n${details}`.trim(),
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
      uploaded.push(await upload(queue[index].path, queue[index].type, property.id, queue[index].sort));
    }
    const uploadedImages = uploaded.filter((item) => item.type === "image");
    const uploadedVideos = uploaded.filter((item) => item.type === "video");
    const finalMedia = pendingCoverSelected.value
      ? [...uploadedImages, ...existingMedia.value, ...uploadedVideos]
      : [...existingMedia.value, ...uploadedImages, ...uploadedVideos];
    await replacePropertyMedia(property.id, finalMedia.map((item) => item.id));
    existingMedia.value = finalMedia;
    images.value = [];
    coverImageIndex.value = 0;
    pendingCoverSelected.value = false;
    videos.value = [];
    currentStatus.value = property.status;
    uni.showModal({
      title: openPreview ? "房源已保存" : "草稿已保存",
      content: openPreview ? "确认无误后即可发布。" : "可继续编辑。",
      showCancel: false,
      success: () => {
        if (openPreview) {
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
</script>
<template>
  <view class="page editor-page" :style="pageStyle"
    ><view class="page-presentation-head"><view class="page-presentation-title">{{ presentation.title }}</view><text>{{ presentation.subtitle }}</text></view
    ><view class="editor-hero"
      ><view
        ><text>{{ id ? "完善房源信息" : "开始录入房源" }}</text></view
      ><view class="completion"
        ><strong>{{ completionCount }}/5</strong><small>已完成</small></view
      ></view
    ><scroll-view class="step-nav" scroll-x :show-scrollbar="false"
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
          ><view><strong>小区名称 *</strong><small>搜索并选择小区，自动获取地图坐标</small></view
          ><text @click="choosePropertyLocation">地图选点</text></view
        ><view class="location-search community-search"
          ><view class="location-search-box"
            ><text>⌕</text
            ><input
              v-model.trim="locationQuery"
              placeholder="输入小区名称、街道或写字楼"
              confirm-type="search"
              @input="scheduleLocationSearch"
              @confirm="runLocationSearch"
            /><small v-if="locationSearching">搜索中</small></view
          ><view v-if="locationResults.length" class="location-results"
            ><view
              v-for="item in locationResults"
              :key="item.id"
              @click="applyLocationResult(item)"
              ><view
                ><strong>{{ item.title }}</strong
                ><small>{{ item.district }} · {{ item.address }}</small></view
              ><text v-if="item.distance"
                >{{ (item.distance / 1000).toFixed(1) }}km</text
			></view
            ></view
          ><view v-else-if="locationQuery.length >= 2 && !locationSearching" class="location-search-tip"
            >继续输入名称，系统会显示武汉范围内的匹配地点</view
          ><view v-if="form.community" class="selected-community">已选择：{{ form.community }}</view></view
        ><view v-if="form.community" class="unit-selector-card" :class="{ selected: Boolean(roomNo) }" @click="openStructurePicker"
          ><view><strong>{{ roomNo ? "已选门牌" : "门牌信息" }}</strong><text>{{ structureSummary }}</text></view><b>{{ roomNo ? "重选 ›" : "选择 ›" }}</b></view
        ><view v-if="hasStructureRule" class="structure-setup-trigger" @click="showStructureSetup = !showStructureSetup"><text>{{ showStructureSetup ? "门牌规则设置" : `默认门牌规则：${selectedBuildingRule?.unitCount || 3} 单元 · ${selectedBuildingRule?.floorCount || 33} 层` }}</text><b>{{ showStructureSetup ? "收起" : "调整 ›" }}</b></view
        ><view v-if="hasStructureRule && showStructureSetup" class="rule-setup"
              ><view class="rule-setup-head"><text>门牌规则</text><small>保存后，同一小区下次录入将直接使用</small></view
              ><view class="rule-inputs"><view><small>单元数</small><input v-model.number="ruleDraft.unitCount" type="number" /></view><view><small>起始层</small><input v-model.number="ruleDraft.floorStart" type="number" /></view><view><small>总层数</small><input v-model.number="ruleDraft.floorCount" type="number" /></view><view><small>每层户数</small><input v-model.number="ruleDraft.householdsPerFloor" type="number" /></view></view
              ><button :loading="ruleSaving" :disabled="!buildingNo || ruleSaving" @click="saveCurrentBuildingRule">保存楼栋规则</button></view
		><view class="field"
          ><text>详细地址补充</text
          ><input
            v-model.trim="form.location"
            maxlength="60"
            placeholder="选择小区后自动回填，可补充门牌信息" /></view
        ><view class="channel-scope"
          ><view class="scope-heading"><text>自动关联</text><small>按小区位置匹配最近精选地区</small></view
          ><picker
            :range="channelTitles"
            :value="selectedChannelIndex"
            @change="selectedChannelIndex = Number($event.detail.value)"
            ><view class="channel-picker auto-channel-picker"
              ><view
                ><strong>{{ selectedChannel.title }}</strong
                ><small
                  >{{ geoReady ? `距该地区 ${selectedChannelDistance.toFixed(1)} 公里` : "选择小区后自动匹配" }} · 关联 {{ channelRadiusKm }} 公里</small
                ></view
              ><text>调整 ›</text></view
            ></picker
          ><view v-if="geoReady" class="map-toggle" @click="mapExpanded = !mapExpanded"><text>{{ mapExpanded ? "收起地图" : "地图调整位置" }}</text><small>{{ propertyInChannel ? "已在自动关联范围内" : "当前地点超出关联范围" }}</small></view></view
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
              ><strong :class="{ warning: geoReady && !propertyInChannel }">{{
                geoReady
                  ? propertyInChannel
                    ? `已在${selectedChannel.title}范围内`
                    : `超出${selectedChannel.title}范围`
                  : `请在${selectedChannel.title}圆形范围内选点`
              }}</strong
              ><small v-if="geoReady"
                >距频道中心 {{ propertyDistance.toFixed(2) }} 公里 ·
                {{ Number(form.latitude).toFixed(6) }},
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
        ><view class="layout-guidance">{{ rentType === "整租" ? "填写整套户型、卫生间与阳台信息。" : "填写本次出租的房间配置，不再重复填写整套户型。" }}</view
        ><view class="picker-grid"
          ><template v-if="rentType === '整租'"><picker
            :range="rooms"
            @change="roomCount = pickLayoutValue(rooms, $event.detail.value); syncLayout()"
            ><view class="picker-item"
              ><small>户型 · 卧室</small
              ><strong>{{ roomCount }}</strong></view
            ></picker
          ><picker
            :range="halls"
            @change="hallCount = pickLayoutValue(halls, $event.detail.value); syncLayout()"
            ><view class="picker-item"
              ><small>户型 · 客厅</small
              ><strong>{{ hallCount }}</strong></view
            ></picker
          ><picker
            :range="bathroomCounts"
            @change="bathroomCount = pickLayoutValue(bathroomCounts, $event.detail.value); syncLayout()"
            ><view class="picker-item"
              ><small>卫生间</small
              ><strong>{{ bathroomCount }}</strong></view
            ></picker
          ><picker
            :range="balconies"
            @change="balcony = pickLayoutValue(balconies, $event.detail.value); syncLayout()"
            ><view class="picker-item"
              ><small>阳台</small
              ><strong>{{ balcony }}</strong></view
            ></picker></template
          ><template v-else><picker
            :range="roomTypes"
            @change="roomType = pickLayoutValue(roomTypes, $event.detail.value); syncLayout()"
            ><view class="picker-item"
              ><small>本次出租 · 房型</small
              ><strong>{{ roomType }}</strong></view
            ></picker
          ><picker
            :range="toilets"
            @change="toiletType = pickLayoutValue(toilets, $event.detail.value); syncLayout()"
            ><view class="picker-item"
              ><small>卫生间</small
              ><strong>{{ toiletType }}</strong></view
            ></picker
          ><picker
            :range="windows"
            @change="bayWindow = pickLayoutValue(windows, $event.detail.value); syncLayout()"
            ><view class="picker-item"
              ><small>飘窗（凸出窗台）</small
              ><strong>{{ bayWindow === "有飘窗" ? "有" : "无" }}</strong></view
            ></picker></template
          ></view
        ><view class="layout-preview"
          ><text>房源摘要</text><strong>{{ form.layout }}</strong></view
        ><view class="detail-grid"
          ><view class="field"><text>面积（㎡）</text><input v-model.number="form.areaSqm" type="number" placeholder="如：55" /></view
          ><view class="field auto-floor"><text>楼层</text><strong>{{ automaticFloorInfo || form.floorInfo || "选择门牌后自动生成" }}</strong></view
          ><view class="field"><text>朝向</text><input v-model.trim="form.orientation" maxlength="32" placeholder="如：南" /></view
          ><view class="field option-field"><text>楼内配置</text><checkbox-group @change="toggleElevator"><label><checkbox value="elevator" :checked="form.elevator === '有电梯'" color="#0e8177" />有电梯</label></checkbox-group></view
          ><view class="field metro-field"><text>步行至地铁（分钟）</text><input v-model.number="form.metroMinutes" type="number" placeholder="如：8" /></view></view
        ></view
      ><view v-show="activeSection === 2" class="section-card"
        ><view class="section-head"
          ><text>03</text
          ><view
            ><strong>配套与描述</strong></view
          ></view
        ><checkbox-group
          class="amenity-grid"
          @change="amenities = $event.detail.value"
          ><label
            v-for="option in amenityOptions"
            :key="option"
            :class="{ active: amenities.includes(option) }"
            ><checkbox
              :value="option"
              :checked="amenities.includes(option)"
              color="#0e8177"
            /><image
              class="amenity-option-icon"
              :src="amenityIcon(option)"
              mode="aspectFit"
            /><text>{{ option }}</text></label
          ></checkbox-group
        ><view class="description">
          <textarea
            v-model.trim="form.description"
            maxlength="800"
            placeholder="填写采光、楼层、朝向、入住要求、周边交通等信息"
          ></textarea
          ><text>{{ String(form.description || "").length }}/800</text></view
        ><view class="field"
          ><text>入住规则</text
          ><input v-model.trim="form.rulesText" maxlength="200" placeholder="如：可做饭，宠物需沟通，女生优先" /></view
        ></view
      ><view v-show="activeSection === 3" class="section-card"
        ><view class="section-head"
          ><text>04</text
          ><view><strong>租金与入住</strong><small>把价格、付款与入住条件集中填写</small></view
        ></view
        ><view class="rent-row"
          ><view class="field"
            ><text>月租 *</text
            ><view class="rent-input"><input v-model.number="form.rentMonthly" type="number" placeholder="如：2600" /><small>元/月</small></view></view
          ><view class="field option-field"
            ><text>中介费</text><checkbox-group @change="toggleNoAgentFee"><label><checkbox value="no-agent-fee" :checked="noAgentFee" color="#0e8177" />无中介费</label></checkbox-group></view></view
        ><view class="term-block"><text>押付方式 *</text><view class="payment-options"><view class="payment-fixed">押一</view><view v-for="cycle in paymentCycles" :key="cycle" :class="{ active: paymentCycle === cycle }" @click="setPaymentCycle(cycle)">{{ cycle }}</view></view></view
        ><view class="rent-row detail-row"
          ><view class="field"
            ><text>可入住日期</text
            ><input v-model.trim="form.availableDate" maxlength="32" placeholder="默认拎包入住，可填写具体日期" /></view
          ><view class="field utility-summary"><text>水电与杂费</text><strong>{{ form.utilitiesText || "请选择" }}</strong></view></view
        ><view class="term-block"><text>水电与杂费</text><radio-group class="utility-options" @change="setUtilities($event.detail.value)"><label v-for="option in utilityOptions" :key="option" :class="{ active: form.utilitiesText === option }"><radio :value="String(utilityOptions.indexOf(option))" :checked="form.utilitiesText === option" color="#0e8177" />{{ option }}</label></radio-group></view>
        <view class="rent-summary"><text>对外展示</text><strong>¥ {{ Number(form.rentMonthly) || 0 }} <small>/月</small></strong><span>{{ form.depositPayment || "待填写押付方式" }}</span></view>
        </view
      ><view v-show="activeSection === 4" class="section-card"
        ><view class="section-head"
          ><text>05</text
          ><view
            ><strong>图片与视频</strong></view
          ></view
        ><view class="media-guidance"
          ><strong>封面图片</strong
          ><text>首张为封面 · 可重裁</text></view
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
        ><view><strong>发布标题</strong><small>点选信息；已选内容可用左右箭头调整拼接顺序</small></view
        ><view class="title-part-list"><view v-for="(option, index) in orderedTitlePartOptions" :key="option.key" class="title-part" :class="{ active: titleParts.includes(option.key) }"><view class="title-part-name" @click="toggleTitlePart(option.key)"><text>{{ titleParts.includes(option.key) ? "✓" : "+" }}</text>{{ option.label }}</view><view v-if="titleParts.includes(option.key)" class="title-part-actions"><text :class="{ disabled: index === 0 }" @click="moveTitlePart(option.key, -1)">←</text><text :class="{ disabled: index === titleParts.length - 1 }" @click="moveTitlePart(option.key, 1)">→</text></view></view></view>
        <view class="auto-title"><text>标题预览</text><strong>{{ generatedTitle || "请至少选择一项" }}</strong></view></view
      ><view class="save-space" /><view class="save-bar"
        ><view v-if="saving" class="progress">{{ uploadProgress }}</view
        ><view v-if="activeSection < 4" class="step-actions"
          ><button
            v-if="activeSection > 0"
            class="back-button"
            @click="goToSection(activeSection - 1)"
          >
            上一步</button
          ><view v-else class="draft-link" @click="save(false)">保存草稿</view
          ><button
            class="primary next-button"
            @click="goToSection(activeSection + 1)"
          >
            下一步 · {{ sectionSteps[activeSection + 1] }}
          </button></view
        ><view v-else class="save-actions"
          ><button v-if="id && currentStatus === 'draft'" class="delete-draft" :disabled="saving" @click="deleteDraft">删除草稿</button
          ><button :disabled="saving" @click="save(false)">保存草稿</button
          ><button
            class="primary"
            :disabled="saving || !requiredReady"
            @click="save(true)"
          >
            {{
              saving
                ? "处理中..."
                : "保存并查看"
            }}
          </button></view
        ></view
      ><view v-if="structureStep" class="structure-modal" @click="structureStep = ''"
        ><view class="structure-sheet" @click.stop
          ><view class="structure-sheet-head"
            ><view><strong>{{ structureStep === "building" ? "选择楼栋" : structureStep === "unit" ? "选择单元" : "选择房号" }}</strong><small>{{ form.community }}</small></view><view class="structure-head-actions"><text v-if="structureStep !== 'building'" class="structure-back" @click="returnToPreviousStructureStep">‹ 返回</text><text class="structure-close" @click="structureStep = ''">×</text></view></view
          ><view class="structure-progress structure-progress-three"><text :class="{ active: structureStep === 'building', available: canReturnToStructureStep('building') }" @click="returnToStructureStep('building')">楼栋</text><text :class="{ active: structureStep === 'unit', available: canReturnToStructureStep('unit') }" @click="returnToStructureStep('unit')">单元</text><text :class="{ active: structureStep === 'room', available: canReturnToStructureStep('room') }" @click="returnToStructureStep('room')">房号</text></view
          ><scroll-view scroll-y class="structure-grid-scroll"
            ><view v-if="structureStep === 'building'" class="structure-grid"><view v-for="rule in buildingOptions" :key="rule.id" class="structure-grid-item" :class="{ active: buildingNo === rule.buildingNo }" @click="selectBuildingRule(rule)">{{ rule.buildingNo }}</view></view
            ><view v-else-if="structureStep === 'unit'" class="structure-grid"><view v-for="unit in unitOptions" :key="unit" class="structure-grid-item" :class="{ active: unitNo === unit }" @click="chooseUnit(unit)">{{ unit }} 单元</view></view
            ><view v-else class="structure-grid"><view v-for="room in roomOptions" :key="room.roomNo" class="structure-grid-item" :class="{ active: roomNo === room.roomNo }" @click="chooseRoom(room)">{{ room.roomNo }}</view></view></scroll-view
          ><view class="structure-sheet-footer"><text>不填门牌也可以继续录入</text><button @click="structureStep = ''">暂不选择</button></view
        ></view
      ></view
      ></template
    ></view
  >
</template>
<style scoped>
.editor-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  margin: -28rpx -32rpx 0;
  padding: 38rpx 32rpx 64rpx;
  border-radius: 0 0 44rpx 44rpx;
  background:
    radial-gradient(
      circle at 88% 10%,
      rgba(229, 196, 133, 0.22),
      transparent 38%
    ),
    #173f3c;
  color: #fff;
}
.editor-hero view:first-child > text,
.editor-hero small {
  display: block;
}
.editor-hero view:first-child > text {
  font-size: 42rpx;
  font-weight: 800;
  letter-spacing: -1rpx;
}
.editor-hero small {
  margin-top: 10rpx;
  color: rgba(255, 255, 255, 0.68);
  font-size: 22rpx;
  line-height: 1.55;
}
.completion {
  flex: none;
  text-align: right;
}
.completion strong,
.completion small {
  display: block;
}
.completion strong {
  color: #f0cf8d;
  font-size: 36rpx;
  font-variant-numeric: tabular-nums;
}
.completion small {
  margin-top: 2rpx;
  color: rgba(255, 255, 255, 0.58);
  font-size: 19rpx;
}
.step-nav {
  position: relative;
  z-index: 2;
  width: auto;
  margin: -36rpx 0 24rpx;
  border-radius: 24rpx;
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 18rpx 42rpx rgba(23, 63, 60, 0.12);
}
.step-track {
  display: flex;
  min-width: 680rpx;
  padding: 14rpx 10rpx;
}
.step-item {
  display: flex;
  min-width: 150rpx;
  flex: 1;
  align-items: center;
  gap: 8rpx;
  padding: 12rpx 10rpx;
  color: #8aa7a0;
}
.step-item text {
  display: flex;
  width: 34rpx;
  height: 34rpx;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #edf4f1;
  font-size: 19rpx;
  font-weight: 700;
}
.step-item small {
  font-size: 20rpx;
  white-space: nowrap;
}
.step-item.active {
  color: #173f3c;
  font-weight: 700;
}
.step-item.active text {
  background: #173f3c;
  color: #fff;
}
.step-item.done:not(.active) text {
  background: #dcefe8;
  color: #0e8177;
}
.loading {
  text-align: center;
  color: #78a39d;
}
.section-card {
  margin-bottom: 20rpx;
  padding: 34rpx 30rpx 40rpx;
  border-radius: 26rpx;
  background: #fff;
  box-shadow: 0 12rpx 36rpx rgba(23, 63, 60, 0.055);
}
.section-head {
  display: flex;
  align-items: center;
  gap: 18rpx;
  margin-bottom: 18rpx;
}
.section-head > text {
  display: flex;
  width: 52rpx;
  height: 52rpx;
  align-items: center;
  justify-content: center;
  border-radius: 14rpx;
  background: #edf5f2;
  color: #0e8177;
  font-size: 21rpx;
  font-weight: 800;
}
.section-head strong,
.section-head small {
  display: block;
}
.section-head strong {
  color: #173f3c;
  font-size: 30rpx;
}
.section-head small {
  margin-top: 5rpx;
  color: #78a39d;
  font-size: 21rpx;
}
.field {
  margin-top: 18rpx;
  padding: 20rpx 22rpx;
  border: 1rpx solid #e3eeea;
  border-radius: 16rpx;
  background: #fbfdfc;
}
.field > text {
  display: block;
  margin-bottom: 12rpx;
  color: #6d8e89;
  font-size: 21rpx;
  font-weight: 600;
}
.field input {
  width: 100%;
  color: #073b3a;
  font-size: 27rpx;
}
.auto-title {
  background: #f1f7f4;
}
.auto-title strong {
  display: block;
  overflow: hidden;
  color: #173f3c;
  font-size: 28rpx;
  font-weight: 750;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.auto-title small {
  display: block;
  margin-top: 8rpx;
  color: #7d9992;
  font-size: 20rpx;
}
.final-title-composer { margin-top:18rpx; }
.final-title-composer .auto-title { margin-top:18rpx;padding:18rpx;border-radius:14rpx; }
.final-title-composer .auto-title text { display:block;margin-bottom:8rpx;color:#6d8e89;font-size:20rpx;font-weight:600; }
.title-part-list { display:flex;flex-wrap:wrap;gap:10rpx;margin-top:16rpx; }
.title-part { display:flex;align-items:center;overflow:hidden;border:1rpx solid #dce9e4;border-radius:11rpx;background:#fff;color:#708a84;font-size:20rpx; }
.title-part.active { border-color:#b8dfd4;background:#edf8f4;color:#0e8177;font-weight:700; }
.title-part-name { display:flex;align-items:center;gap:7rpx;padding:10rpx 12rpx; }
.title-part-name text { display:flex;width:26rpx;height:26rpx;align-items:center;justify-content:center;border-radius:50%;background:#f0f4f2;color:#78938e;font-size:18rpx; }
.title-part.active .title-part-name text { background:#0e8177;color:#fff; }
.title-part-actions { display:flex;align-items:center;border-left:1rpx solid #cce6de; }
.title-part-actions text { padding:10rpx 9rpx;color:#0e8177;font-size:22rpx;line-height:1; }
.title-part-actions text.disabled { opacity:.28; }
.rent-type-tabs { display:grid;grid-template-columns:1fr 1fr;gap:12rpx;padding:8rpx;border-radius:18rpx;background:#eef5f2; }
.rent-type-tabs>view { padding:18rpx 20rpx;border-radius:13rpx;background:transparent;color:#6f8983; }
.rent-type-tabs>view.active { background:#0e8177;color:#fff;box-shadow:0 8rpx 16rpx rgba(14,129,119,.18); }
.rent-type-tabs strong,.rent-type-tabs small { display:block; }
.rent-type-tabs strong { font-size:27rpx;line-height:1.25; }
.rent-type-tabs small { margin-top:5rpx;font-size:19rpx;opacity:.78; }
.layout-guidance { margin:14rpx 2rpx 18rpx;color:#76918b;font-size:20rpx;line-height:1.5; }
.auto-floor strong { display:block;color:#315f5a;font-size:26rpx;line-height:1.4; }
.option-field checkbox-group { display:block; }
.option-field label { display:flex;align-items:center;gap:8rpx;color:#315f5a;font-size:25rpx;font-weight:700; }
.option-field checkbox { transform:scale(.88);margin-right:2rpx; }
.utility-summary strong { display:block;color:#315f5a;font-size:25rpx;line-height:1.4; }
.term-block { margin-top:18rpx;padding:20rpx 22rpx;border:1rpx solid #e3eeea;border-radius:16rpx;background:#fbfdfc; }
.term-block>text { display:block;margin-bottom:14rpx;color:#6d8e89;font-size:21rpx;font-weight:600; }
.payment-options { display:grid;grid-template-columns:repeat(5,1fr);gap:10rpx; }
.payment-options>view { display:flex;min-height:58rpx;align-items:center;justify-content:center;border:1rpx solid #dce9e4;border-radius:10rpx;background:#fff;color:#5d7973;font-size:21rpx;font-weight:700; }
.payment-options .payment-fixed { border-color:#cfe6df;background:#edf8f4;color:#0e8177; }
.payment-options>view.active { border-color:#0e8177;background:#0e8177;color:#fff; }
.utility-options { display:grid;grid-template-columns:1fr 1fr;gap:10rpx; }
.utility-options label { display:flex;align-items:center;min-height:64rpx;box-sizing:border-box;padding:0 12rpx;border:1rpx solid #dce9e4;border-radius:11rpx;background:#fff;color:#5d7973;font-size:20rpx;line-height:1.25; }
.utility-options label.active { border-color:#0e8177;background:#edf8f4;color:#0e8177;font-weight:700; }
.utility-options radio { transform:scale(.78);margin-right:0; }
.rent-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14rpx;
}
.unit-selector-card { display:flex;align-items:center;justify-content:space-between;gap:18rpx;margin-top:18rpx;padding:16rpx 20rpx;border:1rpx solid #dce9e4;border-radius:16rpx;background:#fbfdfc; }
.unit-selector-card.selected { border-color:#b8dfd4;background:#edf8f4;box-shadow:0 8rpx 18rpx rgba(14,129,119,.08); }
.unit-selector-card strong,.unit-selector-card text { display:block; }
.unit-selector-card strong { color:#63817a;font-size:21rpx;font-weight:600; }
.unit-selector-card strong small { margin-left:8rpx;color:#8aa39d;font-size:19rpx;font-weight:500; }
.unit-selector-card text { margin-top:5rpx;color:#315f5a;font-size:24rpx;font-weight:700; }
.unit-selector-card b { flex:none;color:#0e8177;font-size:22rpx;font-weight:700; }
.structure-modal { position:fixed;z-index:1000;top:0;right:0;bottom:0;left:0;display:flex;align-items:flex-end;background:rgba(8,32,29,.46); }
.structure-sheet { width:100%;max-height:78vh;border-radius:30rpx 30rpx 0 0;background:#fff;box-shadow:0 -16rpx 40rpx rgba(5,35,31,.16); }
.structure-sheet-head { display:flex;align-items:flex-start;justify-content:space-between;padding:30rpx 30rpx 20rpx; }
.structure-sheet-head strong,.structure-sheet-head small { display:block; }
.structure-sheet-head strong { color:#173f3c;font-size:32rpx; }
.structure-sheet-head small { margin-top:7rpx;color:#81948f;font-size:20rpx; }
.structure-head-actions { display:flex;align-items:center;gap:12rpx; }
.structure-back { padding:0 16rpx;border-radius:24rpx;background:#edf6f3;color:#0e8177;font-size:21rpx;font-weight:700;line-height:48rpx; }
.structure-close { width:48rpx;height:48rpx;line-height:44rpx;text-align:center;border-radius:50%;background:#f0f5f2;color:#527069;font-size:38rpx; }
.structure-progress { display:grid;grid-template-columns:repeat(4,1fr);gap:8rpx;margin:0 30rpx;padding:12rpx;border-radius:14rpx;background:#f3f7f5; }
.structure-progress-three { grid-template-columns:repeat(3,1fr); }
.structure-progress text { text-align:center;color:#91a59f;font-size:20rpx; }
.structure-progress text.available { color:#55766f;text-decoration:underline; }
.structure-progress text.active { color:#0e8177;font-weight:800; }
.structure-grid-scroll { height:500rpx;padding:24rpx 30rpx;box-sizing:border-box; }
.structure-grid { display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16rpx; }
.structure-grid-item { display:flex;min-height:66rpx;align-items:center;justify-content:center;padding:0 8rpx;border-radius:10rpx;background:#f3f5f5;color:#294d47;font-size:22rpx;font-weight:700; }
.structure-grid-item.active { background:#0e8177;color:#fff;box-shadow:0 8rpx 16rpx rgba(14,129,119,.2); }
.structure-sheet-footer { display:flex;align-items:center;justify-content:space-between;gap:16rpx;padding:20rpx 30rpx calc(20rpx + env(safe-area-inset-bottom));border-top:1rpx solid #edf2f0; }
.structure-sheet-footer text { color:#829691;font-size:19rpx; }
.structure-sheet-footer button { min-width:150rpx;margin:0;border:0;border-radius:12rpx;background:#edf4f1;color:#37675f;font-size:22rpx;line-height:68rpx; }
.rule-setup { margin-top:14rpx;padding:16rpx;border-radius:12rpx;background:#eef5f2; }
.rule-setup-head text,.rule-setup-head small { display:block; }
.rule-setup-head text { color:#315b54;font-size:21rpx;font-weight:700; }
.rule-setup-head small { margin-top:5rpx;color:#78938e;font-size:18rpx; }
.rule-inputs { display:grid;grid-template-columns:repeat(4,1fr);gap:8rpx;margin-top:14rpx; }
.rule-inputs>view { padding:10rpx;border-radius:10rpx;background:#fff; }
.rule-inputs small { display:block;color:#78938e;font-size:17rpx; }
.rule-inputs input { height:38rpx;margin-top:4rpx;color:#173f3c;font-size:22rpx; }
.rule-setup button { margin:14rpx 0 0;border:0;border-radius:10rpx;background:#0e8177;color:#fff;font-size:21rpx;line-height:62rpx; }
.structure-setup-trigger { display:flex;align-items:center;justify-content:space-between;gap:14rpx;margin-top:14rpx;padding:16rpx 2rpx;color:#6e8c86;font-size:20rpx; }
.structure-setup-trigger b { color:#0e8177;font-size:20rpx;font-weight:700; }
.detail-row{margin-top:14rpx}.detail-grid{display:grid;grid-template-columns:1fr 1fr;gap:14rpx;margin-top:16rpx}.detail-grid .field{margin-top:0}.detail-grid .metro-field{grid-column:span 2}
.channel-scope {
  margin-top: 16rpx;
}
.scope-heading { display:flex;align-items:baseline;justify-content:space-between;gap:14rpx;margin-bottom:12rpx; }
.scope-heading text { color:#315f5a;font-size:24rpx;font-weight:700; }
.scope-heading small { color:#819b95;font-size:19rpx; }
.auto-channel-picker { border-color:#cfe6df;background:#edf8f4; }
.map-toggle { display:flex;align-items:center;justify-content:space-between;gap:14rpx;margin-top:12rpx;padding:0 6rpx;color:#0e8177;font-size:21rpx;font-weight:700; }
.map-toggle small { color:#72918a;font-size:19rpx;font-weight:500; }
.map-section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20rpx;
  margin-top: 34rpx;
}
.map-section-heading strong,
.map-section-heading small {
  display: block;
}
.map-section-heading strong {
  color: #173f3c;
  font-size: 30rpx;
}
.map-section-heading small {
  margin-top: 7rpx;
  color: #86a19c;
  font-size: 20rpx;
}
.map-section-heading > text {
  flex: none;
  padding-top: 4rpx;
  color: #0e8177;
  font-size: 23rpx;
  font-weight: 700;
}
.community-heading { margin-top: 4rpx; }
.community-search { margin-top: 14rpx; }
.selected-community {
  margin-top: 10rpx;
  padding: 14rpx 16rpx;
  border-radius: 12rpx;
  background: #edf6f3;
  color: #0e8177;
  font-size: 22rpx;
  font-weight: 700;
}
.title-composer {
  margin-top: 18rpx;
  padding: 20rpx;
  border-radius: 16rpx;
  background: #f7faf8;
}
.title-composer > view strong,
.title-composer > view small { display: block; }
.title-composer > view strong { color: #173f3c; font-size: 24rpx; }
.title-composer > view small { margin-top: 6rpx; color: #78938e; font-size: 19rpx; }
.title-composer checkbox-group { display: flex; flex-wrap: wrap; gap: 12rpx; margin-top: 16rpx; }
.title-composer label {
  display: flex;
  align-items: center;
  gap: 4rpx;
  padding: 8rpx 12rpx;
  border-radius: 10rpx;
  background: #fff;
  color: #637b76;
  font-size: 20rpx;
}
.title-composer label.active { background: #e0f0eb; color: #0b766d; font-weight: 700; }
.title-composer checkbox { transform: scale(.78); margin-right: -4rpx; }
.location-search {
  position: relative;
  z-index: 3;
  margin-top: 18rpx;
}
.location-search-box {
  display: flex;
  height: 82rpx;
  box-sizing: border-box;
  align-items: center;
  gap: 12rpx;
  padding: 0 20rpx;
  border: 1rpx solid #dbe9e5;
  border-radius: 16rpx;
  background: #fbfdfc;
}
.location-search-box > text {
  color: #0e8177;
  font-size: 32rpx;
}
.location-search-box input {
  min-width: 0;
  flex: 1;
  color: #173f3c;
  font-size: 25rpx;
}
.location-search-box small {
  flex: none;
  color: #78a39d;
  font-size: 19rpx;
}
.location-results {
  margin-top: 8rpx;
  overflow: hidden;
  border: 1rpx solid #dbe9e5;
  border-radius: 16rpx;
  background: #fff;
  box-shadow: 0 16rpx 36rpx rgba(23, 63, 60, 0.12);
}
.location-results > view {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18rpx;
  padding: 20rpx;
  border-bottom: 1rpx solid #edf3f1;
}
.location-results > view:last-child {
  border-bottom: 0;
}
.location-results strong,
.location-results small {
  display: block;
}
.location-results strong {
  color: #173f3c;
  font-size: 24rpx;
}
.location-results small {
  margin-top: 6rpx;
  color: #7f9b96;
  font-size: 20rpx;
}
.location-results > view > text {
  flex: none;
  color: #0e8177;
  font-size: 19rpx;
}
.location-search-tip {
  padding: 12rpx 4rpx 0;
  color: #8ba39f;
  font-size: 19rpx;
}
.channel-scope > text {
  display: block;
  margin-bottom: 12rpx;
  color: #4f7974;
  font-size: 24rpx;
  font-weight: 700;
}
.channel-picker {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx;
  border: 0;
  border-radius: 16rpx;
  background: #edf6f3;
}
.channel-picker strong,
.channel-picker small {
  display: block;
}
.channel-picker strong {
  color: #173f3c;
  font-size: 25rpx;
}
.channel-picker small {
  margin-top: 6rpx;
  color: #78a39d;
  font-size: 20rpx;
}
.channel-picker > text {
  color: #0e8177;
  font-size: 22rpx;
  font-weight: 700;
}
.map-picker {
  margin: 14rpx 0 8rpx;
  overflow: hidden;
  border: 1rpx solid #dfeae6;
  border-radius: 20rpx;
  background: #f4fbf8;
}
.property-map {
  width: 100%;
  height: 330rpx;
}
.map-picker-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18rpx;
  padding: 20rpx;
}
.map-picker-footer > view {
  min-width: 0;
  flex: 1;
}
.map-picker-footer strong,
.map-picker-footer small {
  display: block;
}
.map-picker-footer strong {
  color: #173f3c;
  font-size: 24rpx;
}
.map-picker-footer strong.warning {
  color: #c36b2b;
}
.map-picker-footer small {
  margin-top: 6rpx;
  color: #78a39d;
  font-size: 20rpx;
  line-height: 1.45;
}
.map-picker-footer button {
  flex: none;
  margin: 0;
  border: 0;
  border-radius: 12rpx;
  background: #0e8177;
  color: #fff;
  font-size: 22rpx;
}
.map-hint {
  flex: none;
  color: #0e8177;
  font-size: 20rpx;
}
.delete-draft { color: #bd5a45 !important; background: #fff4f1 !important; }
.rent-input {
  display: flex;
  align-items: center;
}
.rent-input input {
  min-width: 0;
  flex: 1;
}
.rent-input small {
  flex: none;
  color: #8aa7a0;
  font-size: 21rpx;
}
.rent-summary { display:flex;align-items:baseline;gap:12rpx;margin-top:18rpx;padding:20rpx 22rpx;border-radius:16rpx;background:#f8f3ea; }
.rent-summary text { color:#8a7b62;font-size:20rpx; }
.rent-summary strong { color:#d95c38;font-size:38rpx;line-height:1; }
.rent-summary strong small { margin-left:4rpx;font-size:21rpx;font-weight:600; }
.rent-summary span { color:#667d76;font-size:21rpx; }
.picker-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 14rpx;
}
.picker-item {
  position: relative;
  padding: 24rpx 22rpx;
  text-align: left;
  border: 1rpx solid #e1ece8;
  border-radius: 16rpx;
  background: #fbfdfc;
}
.picker-item strong,
.picker-item small {
  display: block;
}
.picker-item strong {
  color: #173f3c;
  font-size: 28rpx;
}
.picker-item small {
  margin-top: 6rpx;
  color: #86a19c;
  font-size: 20rpx;
}
.layout-preview {
  margin-top: 22rpx;
  padding: 22rpx;
  border-left: 6rpx solid #d9b876;
  border-radius: 4rpx 16rpx 16rpx 4rpx;
  background: #f7f4ed;
}
.layout-preview text,
.layout-preview strong {
  display: block;
}
.layout-preview text {
  color: #78a39d;
  font-size: 21rpx;
}
.layout-preview strong {
  margin-top: 7rpx;
  color: #315f5a;
  font-size: 24rpx;
}
.amenity-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12rpx;
}
.amenity-grid label {
  display: flex;
  min-height: 132rpx;
  box-sizing: border-box;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
  padding: 14rpx 8rpx 12rpx;
  border: 1rpx solid #e2eeea;
  border-radius: 16rpx;
  color: #5f8580;
  font-size: 23rpx;
  text-align: center;
}
.amenity-grid label.active {
  border-color: #9dd8cc;
  background: #edfaf6;
  color: #0e8177;
}
.amenity-grid checkbox {
  position: absolute;
  width: 1rpx;
  height: 1rpx;
  opacity: 0;
}
.amenity-option-icon {
  width: 48rpx;
  height: 48rpx;
  padding: 10rpx;
  border-radius: 18rpx;
  background: #f0f8f5;
}
.amenity-grid label.active .amenity-option-icon {
  background: #d9f2ea;
}
.description {
  position: relative;
  margin-top: 22rpx;
  padding: 18rpx;
  border: 1rpx solid #e2ece9;
  border-radius: 16rpx;
  background: #fbfdfc;
}
.description textarea {
  width: 100%;
  height: 190rpx;
  font-size: 26rpx;
}
.description > text {
  position: absolute;
  right: 16rpx;
  bottom: 12rpx;
  color: #9ab2ae;
  font-size: 19rpx;
}
.media-guidance {
  display: flex;
  align-items: center;
  gap: 14rpx;
  margin-bottom: 20rpx;
  padding: 16rpx 20rpx;
  border-radius: 16rpx;
  background: #f1f5ea;
}
.media-guidance strong {
  color: #40543f;
  font-size: 24rpx;
}
.media-guidance text {
  color: #61705d;
  font-size: 21rpx;
}
.uploaded-gallery,
.pending-gallery,
.video-preview-list {
  margin-bottom: 24rpx;
}
.media-group-title {
  display: block;
  margin: 0 0 12rpx 2rpx;
  color: #61705d;
  font-size: 22rpx;
  font-weight: 600;
}
.photo-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14rpx;
}
.photo-card {
  position: relative;
  height: 224rpx;
  overflow: hidden;
  border-radius: 18rpx;
  background: #e6f3ef;
}
.photo-card.cover {
  box-shadow: 0 0 0 3rpx #536a45;
}
.photo-card image {
  width: 100%;
  height: 100%;
}
.photo-card > text,
.video-card > text {
  position: absolute;
  left: 8rpx;
  bottom: 8rpx;
  padding: 4rpx 8rpx;
  border-radius: 8rpx;
  background: rgba(0, 0, 0, 0.5);
  color: #fff;
  font-size: 18rpx;
}
.photo-card > .cover-mark {
  background: rgba(54, 77, 48, 0.8);
}
.uploaded-photo { opacity: 0.9; }
.media-tools {
  position: absolute;
  top: 8rpx;
  right: 8rpx;
  display: flex;
  gap: 6rpx;
}
.media-tools text {
  min-width: 42rpx;
  height: 34rpx;
  padding: 0 8rpx;
  border-radius: 9rpx;
  background: rgba(25, 33, 25, 0.66);
  color: #fff;
  font-size: 18rpx;
  line-height: 34rpx;
  text-align: center;
}
.media-tools .remove {
  min-width: 34rpx;
  padding: 0;
  background: rgba(160, 62, 52, 0.82);
  font-size: 28rpx;
}
.video-card {
  position: relative;
  height: 360rpx;
  overflow: hidden;
  border-radius: 18rpx;
  background: #1e2825;
}
.video-entry + .video-entry { margin-top: 14rpx; }
.video-card video {
  display: block;
  width: 100%;
  height: 100%;
}
.video-toggle {
  width: 100%;
  height: 72rpx;
  margin: 12rpx 0 0;
  border: 0;
  border-radius: 14rpx;
  background: #eef3eb;
  color: #40543f;
  font-size: 23rpx;
  font-weight: 600;
  line-height: 72rpx;
}
.media-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14rpx;
}
.media-actions > view {
  padding: 26rpx 12rpx;
  border: 2rpx dashed #b7dcd4;
  border-radius: 16rpx;
  text-align: center;
}
.media-actions strong,
.media-actions small {
  display: block;
}
.media-actions strong {
  color: #0e8177;
  font-size: 24rpx;
}
.media-actions small {
  margin-top: 8rpx;
  color: #78a39d;
  font-size: 20rpx;
}
.media-limit-hint {
  padding: 24rpx 24rpx 0;
  color: #91a19b;
  font-size: 20rpx;
  line-height: 1.45;
  text-align: center;
}
.save-space {
  height: 150rpx;
}
.save-bar {
  position: fixed;
  z-index: 5;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 16rpx 32rpx calc(16rpx + env(safe-area-inset-bottom));
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 -8rpx 28rpx rgba(15, 70, 65, 0.09);
}
.progress {
  margin-bottom: 10rpx;
  color: #0e8177;
  font-size: 21rpx;
  text-align: center;
}
.save-actions {
  display: grid;
  grid-template-columns: 0.8fr 1.2fr;
  gap: 14rpx;
}
.step-actions {
  display: grid;
  grid-template-columns: minmax(150rpx, 0.7fr) minmax(0, 1.5fr);
  align-items: center;
  gap: 14rpx;
}
.step-actions button,
.save-actions button {
  height: 88rpx;
  line-height: 88rpx;
}
.draft-link {
  color: #5f807b;
  font-size: 25rpx;
  font-weight: 600;
  text-align: center;
}
.back-button {
  margin: 0;
  border: 0;
  border-radius: 16rpx;
  background: #edf4f1;
  color: #315f5a;
  font-size: 25rpx;
}
.next-button {
  margin: 0;
  border: 0;
  border-radius: 16rpx;
  background: #173f3c;
  color: #fff;
  font-size: 26rpx;
  font-weight: 700;
}
.next-button:active,
.back-button:active,
.media-actions > view:active,
.picker-item:active {
  transform: scale(0.985);
}
.save-actions button {
  margin: 0;
  border: 0;
  border-radius: 16rpx;
  background: #edf6f3;
  color: #0e8177;
  font-size: 27rpx;
}
.save-actions button::after {
  border: 0;
}
.save-actions .primary {
  background: #0e8177;
  color: #fff;
}
.save-actions button[disabled] {
  opacity: 0.5;
}
.editor-page .page-presentation-head{margin-bottom:26rpx}.editor-hero{margin:8rpx -32rpx 0;padding-top:34rpx;padding-bottom:58rpx}.step-nav{margin-top:-30rpx;margin-bottom:28rpx}.section-card{margin-bottom:24rpx;padding:36rpx 30rpx 42rpx}.section-head{margin-bottom:22rpx}.field{margin-top:20rpx;padding:22rpx}.map-section-heading{margin-top:38rpx;margin-bottom:18rpx}
.editor-hero view:first-child>text{font-size:40rpx;line-height:1.25}.step-item small{font-size:21rpx;line-height:1.25}.section-head strong{font-size:33rpx;line-height:1.3}.field>text{font-size:25rpx;line-height:1.25}.field input{font-size:28rpx;line-height:1.4}.map-section-heading strong{font-size:32rpx;line-height:1.3}.location-search-box input{font-size:27rpx}.channel-scope>text{font-size:25rpx}.channel-picker{padding:22rpx}.channel-picker strong{font-size:28rpx;line-height:1.3}.channel-picker small{margin-top:7rpx;line-height:1.45}.map-picker-footer{padding:20rpx 22rpx}.map-picker-footer small{margin-top:8rpx;line-height:1.5}.picker-item{min-height:116rpx}.picker-item strong{font-size:29rpx;line-height:1.25}.picker-item small{margin-top:7rpx;line-height:1.2}
</style>
