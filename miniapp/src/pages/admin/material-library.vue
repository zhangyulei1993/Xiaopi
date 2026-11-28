<script setup lang="ts">
import { returnToMyPage } from "../../utils/permission-redirect";
import { useForegroundUpload } from "../../utils/foreground-upload";
import { onHide, onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { API_ORIGIN, request, resolveAssetUrl, safeUploadUrl } from "../../api/client";
import {
  convertMaterial,
  listProperties,
  createMaterial,
  deleteMaterial,
  getMaterialCategories,
  getPublicConfig,
  listMaterialLibrary,
  renameMaterialCategory,
  replacePropertyMedia,
  searchLocations,
  saveMaterialCategories,
  type LocationSearchItem,
  type Property,
  type PropertyMedia,
} from "../../api/properties";
import { getSession, hasRole } from "../../auth/session";
import { refreshCurrentSession, startSessionKeepAlive } from "../../auth/refresh-session";

import { defaultListingTitleFields, listingTagValue, uniqueListingTags } from "../../utils/listing-title";

import { normalizeCommunityName } from "../../utils/community-name";

const pricePreferenceKey = "xiaopi_material_show_rent";
const showPrice = ref(uni.getStorageSync(pricePreferenceKey) === true);
const tagConfig = ref<Awaited<ReturnType<typeof getPublicConfig>>["listingTitle"]>(undefined);
const togglePrice = (values: string[]) => {
  showPrice.value = values.includes("price");
  uni.setStorageSync(pricePreferenceKey, showPrice.value);
};
const materialTags = (item: Property) => {
  const config = tagConfig.value;
  const fields = [...(config?.enabledFields ?? defaultListingTitleFields),
    ...(config?.adminEnabledFields ?? [])];
  const order = [...(config?.listTagOrder ?? []), ...fields];
  const fieldsInOrder = [...new Set(order)]
    .filter(field => field !== "community" && fields.includes(field));
  return uniqueListingTags(fieldsInOrder.map(field => {
      if (field === "listingSource" && !item.listingSource) return "";
      const value = field === "layout" && item.rentType === "合租" && item.wholeLayout
        ? item.wholeLayout : listingTagValue(item, field);
      return field === "layout" ? value.match(/\d+室(?:\d+厅)?/)?.[0] || value : value;
    }));
};
const extraMaterialTags = (item: Property) => materialTags(item).filter(tag => tag !== materialLayout(item) && tag !== item.rentType);

type PendingMedia = { path: string; type: "image" | "video" };
type ChosenMediaFile = { tempFilePath: string; fileType?: string; size: number; duration?: number };
const loading = ref(false);
const saving = ref(false);
const maxImageBytes = 10 * 1024 * 1024;
const maxVideoBytes = 40 * 1024 * 1024;
const materials = ref<Property[]>([]);
const linkedProperties = ref<Property[] | null>(null);
const linkedProperty = (item: Property) => linkedProperties.value?.find(property => property.id === item.convertedPropertyId);
const isConverted = (item: Property) => Boolean(item.convertedPropertyId && (linkedProperties.value === null || linkedProperty(item)));
const isDraft = (property: Property) => ["draft", "pending_review"].includes(property.status);
const linkedActionLabel = (item: Property) => {
  const property = linkedProperty(item);
  return property && isDraft(property) ? "继续编辑草稿" : "查看房源";
};
const categories = ref<string[]>([]);
const activeCommunity = ref("全部");
const activeLayout = ref("全部");
const showConvertedArchive = ref(false);
const formOpen = ref(false);
const rentType = ref<"整租" | "合租">("整租");
const sharedRoomType = ref("主卧");
const materialRoomCount = ref(1);
const materialHallCount = ref(1);
const materialHasKitchen = ref(true);
const materialBathroomCount = ref(1);
const materialBalconyCount = ref(0);
const materialRoomSequence = ref(0);
const materialFloor = ref("");
const materialAmenities = ref(["冰箱", "洗衣机", "空调", "热水器"]);
const materialAmenityOptions = ["冰箱", "洗衣机", "空调", "热水器", "电视", "宽带", "燃气", "油烟机", "沙发", "衣柜", "书桌", "床", "电梯", "阳台"];
const draftId = ref("");
let createKey = "";
const createSnapshot = ref<Parameters<typeof createMaterial>[0] | null>(null);
const uploadedMedia = ref<PropertyMedia[]>([]);
const uploadProgress = ref("");
const categoryManagerOpen = ref(false);
const categoryActionOpen = ref(false);
const categoryActionName = ref("");
const renameCategoryOpen = ref(false);
const renameCategoryFrom = ref("");
const renameCategoryValue = ref("");
const communityFilterOpen = ref(false);
const communityFilterQuery = ref("");
const communityPickerOpen = ref(false);
const communityPickerTargetCategory = ref("");
const communityPickerReturnTo = ref<"entry" | "manager" | "filter">("entry");
const communityPickerQuery = ref("");
const communityMapSearching = ref(false);
const communityMapSearched = ref(false);
const communityMapResults = ref<LocationSearchItem[]>([]);
const communityDirectory = ref<Record<string, number>>({});
const communityLocations = ref<Record<string, { address: string; latitude: number; longitude: number }>>({});
const communityPlans = ref<Record<string, { community?: string }>>({});
const selectedMapLocation = ref<{ address: string; latitude: number; longitude: number } | null>(null);
const rentMonthly = ref("");
const category = ref("");
// When adding from a community category, that category is already the
// destination. Keep it visible as context and skip the redundant picker.
const entryCategoryFixed = ref(false);
const pendingMedia = ref<PendingMedia[]>([]);
const materialLayoutText = computed(() => [
  `${materialRoomCount.value}室`,
  `${materialHallCount.value}厅`,
  materialHasKitchen.value ? "1厨" : "",
  materialBathroomCount.value ? `${materialBathroomCount.value}卫` : "",
  materialBalconyCount.value ? `${materialBalconyCount.value}阳台` : "",
].filter(Boolean).join(""));
const toggleMaterialAmenity = (name: string) => {
  materialAmenities.value = materialAmenities.value.includes(name)
    ? materialAmenities.value.filter(item => item !== name)
    : [...materialAmenities.value, name];
};

const mediaPathExtension = (path: string) => {
  const cleanPath = path.split(/[?#]/u, 1)[0];
  const dot = cleanPath.lastIndexOf(".");
  return dot > cleanPath.lastIndexOf("/") ? cleanPath.slice(dot + 1).toLowerCase() : "";
};
const mediaFileMatches = (file: ChosenMediaFile, expected: PendingMedia["type"]) => {
  const extension = mediaPathExtension(file.tempFilePath);
  const imageExtensions = new Set(["jpg", "jpeg", "png", "webp", "gif", "bmp"]);
  const videoExtensions = new Set(["mp4", "mov", "m4v", "3gp", "avi", "mkv"]);
  const allowedExtensions = expected === "image" ? imageExtensions : videoExtensions;
  return file.fileType === expected && (!extension || allowedExtensions.has(extension));
};

const stripBuildingSuffix = (value: string) => value.trim()
  .replace(/(?:[-—·\s]*[0-9０-９零〇一二两三四五六七八九十百廿]+(?:[-—·][0-9０-９零〇一二两三四五六七八九十百廿]+)?\s*(?:单元|室))$/iu, "")
  .replace(/(?:[-—·\s]*[0-9０-９零〇一二两三四五六七八九十百廿]+\s*(?:栋|幢|号楼|号栋|座))$/iu, "")
  .replace(/[-—·\s]+$/u, "").trim();
const materialCommunity = (item: Property) => String(item.materialCategory || item.community || "待整理").trim();
const materialLayout = (item: Property) => {
  const source = String((item.rentType === "合租" ? item.wholeLayout || item.layout : item.layout || item.wholeLayout) || "").trim();
  const roomPlan = source.match(/[1-9]\d*\s*室(?:\s*[1-9]\d*\s*厅)?/u)?.[0]?.replace(/\s+/g, "") || "";
  if (roomPlan) return roomPlan;
  if (/主卧/u.test(source)) return "合租主卧";
  if (/次卧/u.test(source)) return "合租次卧";
  if (/隔断/u.test(source)) return "合租隔断间";
  return source ? source.slice(0, 18) : "户型待补";
};
const layoutFilterOptions = ["一室", "两室", "三室", "四室", "五室", "六室+"];
const materialLayoutGroup = (item: Property) => {
  const source = String((item.rentType === "合租" ? item.wholeLayout || item.layout : item.layout || item.wholeLayout) || materialLayout(item));
  const roomLabel = source.match(/(\d+|一|二|两|三|四|五|六|七|八|九|十)\s*室/u)?.[1] || "";
  const roomCount = Number(roomLabel) || ({ 一: 1, 二: 2, 两: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9, 十: 10 } as Record<string, number>)[roomLabel] || 0;
  if (!roomCount) return "";
  if (roomCount >= 6) return "六室+";
  return layoutFilterOptions[roomCount - 1] || "";
};
const communityOptions = computed(() => {
  const names = new Map<string, string>();
  for (const name of [...categories.value, ...materials.value.map(materialCommunity)]) {
    const trimmed = name.trim();
    const key = normalizeCommunityName(trimmed);
    if (key && !names.has(key)) names.set(key, trimmed);
  }
  return [...names.values()].sort((a, b) => a.localeCompare(b, "zh-CN"));
});
const filteredCommunityOptions = computed(() => {
  const query = normalizeCommunityName(communityFilterQuery.value);
  const options = communityOptions.value
    .filter(name => !query || normalizeCommunityName(name).includes(query))
    .sort((a, b) => categoryMaterials(b).length - categoryMaterials(a).length || a.localeCompare(b, "zh-CN"));
  return ["全部", ...options].filter(name => !query || name === "全部" || normalizeCommunityName(name).includes(query));
});
const communityLibraryOptions = computed(() => {
  const names = new Map<string, string>();
  for (const name of [...Object.keys(communityDirectory.value), ...Object.keys(communityLocations.value), ...Object.keys(communityPlans.value), ...communityOptions.value]) {
    const trimmed = stripBuildingSuffix(String(communityPlans.value[name]?.community || name).trim());
    const key = normalizeCommunityName(trimmed);
    if (key && !names.has(key)) names.set(key, trimmed);
  }
  return [...names.values()].sort((a, b) => a.localeCompare(b, "zh-CN"));
});
const filteredCommunityLibraryOptions = computed(() => {
  const query = normalizeCommunityName(communityPickerQuery.value);
  const options = query ? communityLibraryOptions.value.filter(name => normalizeCommunityName(name).includes(query)) : communityLibraryOptions.value;
  return options.slice(0, query ? 80 : 32);
});
const canCreateCommunityCategory = computed(() => {
  const name = stripBuildingSuffix(communityPickerQuery.value);
  return !communityPickerTargetCategory.value && name.length >= 2 &&
    !categories.value.some(item => normalizeCommunityName(item) === normalizeCommunityName(name));
});
const selectedCommunityLocation = computed(() => selectedMapLocation.value || Object.entries(communityLocations.value).find(([name]) => normalizeCommunityName(name) === normalizeCommunityName(category.value))?.[1] || null);
const activeMaterials = computed(() => materials.value.filter(item => !showConvertedArchive.value || isConverted(item)));
const filteredMaterials = computed(() => activeMaterials.value.filter(item =>
  (activeCommunity.value === "全部" || normalizeCommunityName(materialCommunity(item)) === normalizeCommunityName(activeCommunity.value)) &&
  (activeLayout.value === "全部" || materialLayoutGroup(item) === activeLayout.value)));
const convertedCount = computed(() => materials.value.filter(isConverted).length);
const countForCommunity = (name: string) => activeMaterials.value.filter(item =>
  (name === "全部" || normalizeCommunityName(materialCommunity(item)) === normalizeCommunityName(name)) &&
  (activeLayout.value === "全部" || materialLayoutGroup(item) === activeLayout.value)).length;
const countForLayout = (name: string) => activeMaterials.value.filter(item =>
  (activeCommunity.value === "全部" || normalizeCommunityName(materialCommunity(item)) === normalizeCommunityName(activeCommunity.value)) &&
  (name === "全部" || materialLayoutGroup(item) === name)).length;
const categoryMaterials = (name: string) => materials.value.filter(item => normalizeCommunityName(materialCommunity(item)) === normalizeCommunityName(name));
const categoryLocation = (name: string) => {
  const fromLibrary = Object.entries(communityLocations.value).find(([community]) => normalizeCommunityName(community) === normalizeCommunityName(name))?.[1];
  if (fromLibrary) return fromLibrary;
  const fromMaterial = categoryMaterials(name).find(item => item.location && normalizeCommunityName(item.location) !== normalizeCommunityName(name) && Number(item.latitude) && Number(item.longitude));
  return fromMaterial ? { address: fromMaterial.location, latitude: Number(fromMaterial.latitude), longitude: Number(fromMaterial.longitude) } : null;
};
const imageCount = computed(() => pendingMedia.value.filter(item => item.type === "image").length);
const hasVideo = computed(() => pendingMedia.value.some(item => item.type === "video"));
const mediaUrl = (url: string) => resolveAssetUrl(url);
const openMaterialDetail = (item: Property) => {
  uni.navigateTo({ url: `/pages/admin/material-detail?id=${encodeURIComponent(item.id)}` });
};
const refresh = async () => {
  loading.value = true;
  try {
    const [kinds, items, config, directory, locations, plans, properties] = await Promise.all([
      getMaterialCategories(), listMaterialLibrary(), getPublicConfig().catch(() => null),
      request<Record<string, number>>("/v1/admin/doorplates").catch(() => ({})),
      request<Record<string, { address: string; latitude: number; longitude: number }>>("/v1/admin/community-locations").catch(() => ({})),
      request<Record<string, { community?: string }>>("/v1/admin/doorplate-plans").catch(() => ({})),
      listProperties().catch(() => null),
    ]);
    if (config) tagConfig.value = config.listingTitle;
    materials.value = items;
    linkedProperties.value = properties;
    categories.value = kinds;
    communityDirectory.value = directory;
    communityLocations.value = locations;
    communityPlans.value = plans;
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "素材库加载失败", icon: "none" });
  } finally {
    loading.value = false;
  }
};
let stopSessionKeepAlive: (() => void) | null = null;
onShow(async () => {
  if (!hasRole("admin")) {
    uni.showToast({ title: "仅管理员可使用素材库", icon: "none" });
    returnToMyPage();
    return;
  }
  stopSessionKeepAlive?.();
  stopSessionKeepAlive = startSessionKeepAlive("admin");
  await refresh();
});
onHide(() => {
  stopSessionKeepAlive?.();
  stopSessionKeepAlive = null;
});

const openCommunityPicker = (targetCategory = "", returnTo: "entry" | "manager" | "filter" = "entry") => {
  communityPickerTargetCategory.value = targetCategory;
  communityPickerReturnTo.value = targetCategory ? "manager" : returnTo;
  communityPickerQuery.value = targetCategory;
  communityMapResults.value = [];
  communityMapSearched.value = false;
  communityPickerOpen.value = true;
  if (targetCategory) setTimeout(() => void searchCommunityMap(), 50);
};
const dismissCommunityPicker = () => {
  communityPickerOpen.value = false;
  if (communityPickerReturnTo.value === "manager") categoryManagerOpen.value = true;
  if (communityPickerReturnTo.value === "filter") communityFilterOpen.value = true;
  communityPickerTargetCategory.value = "";
  communityPickerQuery.value = "";
  communityMapResults.value = [];
};
const startAddCommunityCategory = (returnTo: "manager" | "filter") => {
  communityFilterOpen.value = false;
  categoryManagerOpen.value = returnTo === "manager";
  openCommunityPicker("", returnTo);
};
const openCommunityFilter = () => {
  categoryManagerOpen.value = false;
  communityFilterOpen.value = true;
};
const toggleCategoryManager = () => {
  communityFilterOpen.value = false;
  categoryManagerOpen.value = !categoryManagerOpen.value;
};
const chooseCommunity = async (rawName: string, location?: LocationSearchItem) => {
  const value = stripBuildingSuffix(rawName);
  if (value.length < 2 || saving.value) return;
  const targetCategory = communityPickerTargetCategory.value;
  const existing = categories.value.find(item => normalizeCommunityName(item) === normalizeCommunityName(value));
  const libraryName = communityLibraryOptions.value.find(item => normalizeCommunityName(item) === normalizeCommunityName(value));
  const canonical = targetCategory || existing || libraryName || value;
  try {
    const libraryLocation = Object.entries(communityLocations.value).find(([name]) => normalizeCommunityName(name) === normalizeCommunityName(value))?.[1];
    let locationSaved = false;
    if (location && Number(location.latitude) && Number(location.longitude)) {
      const address = [location.district, location.address].filter(Boolean).join(" · ") || location.title;
      const alreadySaved = libraryLocation &&
        normalizeCommunityName(libraryLocation.address) === normalizeCommunityName(address) &&
        Number(libraryLocation.latitude) === Number(location.latitude) &&
        Number(libraryLocation.longitude) === Number(location.longitude);
      if (!alreadySaved) {
        communityLocations.value = await request<Record<string, { address: string; latitude: number; longitude: number }>>(`/v1/admin/community-locations?community=${encodeURIComponent(canonical)}`, {
          method: "PUT",
          data: { address, latitude: Number(location.latitude), longitude: Number(location.longitude) },
        });
        locationSaved = true;
      }
    }
    selectedMapLocation.value = null;
    if (!targetCategory && !existing) categories.value = await saveMaterialCategories({ add: canonical });
    if (communityPickerReturnTo.value === "entry") category.value = canonical;
    if (communityPickerReturnTo.value === "entry") activeCommunity.value = canonical;
    communityPickerOpen.value = false;
    if (communityPickerReturnTo.value === "manager") categoryManagerOpen.value = true;
    if (communityPickerReturnTo.value === "filter") communityFilterOpen.value = true;
    communityPickerTargetCategory.value = "";
    if (communityPickerReturnTo.value !== "filter") communityPickerQuery.value = "";
    communityMapResults.value = [];
    uni.showToast({ title: locationSaved ? "小区位置已保存" : existing ? "已选择小区" : "小区分类已添加" });
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "小区选择失败", icon: "none" });
  }
};
const searchCommunityMap = async () => {
  const query = communityPickerQuery.value.trim();
  if (query.length < 2 || communityMapSearching.value) return;
  communityMapSearching.value = true;
  communityMapSearched.value = true;
  communityMapResults.value = [];
  try {
    const results = await searchLocations(query, 30.460402, 114.422247);
    const deduped = new Map<string, LocationSearchItem>();
    for (const result of results) {
      const title = stripBuildingSuffix(result.title);
      if (/(?:[\[【(（])?(?:公交站|公交车站|地铁站|地铁口|学校|医院|商场|超市|酒店|银行|餐厅|停车场|便利店|营业厅|售楼部|门诊部|写字楼)(?:[\]】)）])?$/u.test(title)) continue;
      const identity = normalizeCommunityName(title);
      if (identity.length < 2 || !Number(result.latitude) || !Number(result.longitude)) continue;
      const previous = deduped.get(identity);
      if (!previous || result.title.trim() === title) deduped.set(identity, { ...result, title });
    }
    communityMapResults.value = [...deduped.values()];
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "地图搜索失败", icon: "none" });
  } finally {
    communityMapSearching.value = false;
  }
};
const removeCategory = (value: string) => {
  if (formOpen.value || saving.value || draftId.value) return;
  const usageCount = categoryMaterials(value).length;
  if (usageCount) {
    uni.showToast({ title: `该小区还有 ${usageCount} 份素材，先处理素材再移除分类`, icon: "none" });
    return;
  }
  uni.showModal({
    title: "移除这个小区分类？",
    content: "只会移除素材库里的小区分类，不会删除小区库记录或房源数据。",
    success: async result => {
      if (!result.confirm) return;
      try {
        categories.value = await saveMaterialCategories({ remove: value });
        if (activeCommunity.value === value) activeCommunity.value = "全部";
        if (category.value === value) selectCategory(categories.value[0] || "");
        await refresh();
      } catch (error) {
        uni.showToast({ title: error instanceof Error ? error.message : "分类删除失败", icon: "none" });
      }
    },
  });
};
const openCategoryActions = (name: string) => {
  categoryActionName.value = name;
  categoryActionOpen.value = true;
};
const performCategoryAction = (action: "view" | "add" | "rename" | "location" | "remove") => {
  const name = categoryActionName.value;
  categoryActionOpen.value = false;
  if (!name) return;
  if (action === "view") viewCategory(name);
  if (action === "add") {
    viewCategory(name);
    showConvertedArchive.value = false;
    activeLayout.value = "全部";
    openEntry();
  }
  if (action === "rename") {
    renameCategoryFrom.value = name;
    renameCategoryValue.value = name;
    renameCategoryOpen.value = true;
  }
  if (action === "location") openCommunityPicker(name, "manager");
  if (action === "remove") removeCategory(name);
};
const saveCategoryRename = async () => {
  const from = renameCategoryFrom.value;
  const to = renameCategoryValue.value.trim();
  if (!from || !to || saving.value) return;
  if (categories.value.some(item => item !== from && normalizeCommunityName(item) === normalizeCommunityName(to))) {
    uni.showToast({ title: "已有名称相同的小区分类", icon: "none" });
    return;
  }
  saving.value = true;
  try {
    await renameMaterialCategory(from, to);
    categories.value = categories.value.map(item => item === from ? to : item);
    materials.value = materials.value.map(item => normalizeCommunityName(materialCommunity(item)) === normalizeCommunityName(from)
      ? { ...item, community: normalizeCommunityName(String(item.materialCategory || "")) === normalizeCommunityName(from) ? to : item.community, materialCategory: to, location: item.location === from ? to : item.location, title: item.title === from ? to : item.title }
      : item);
    if (activeCommunity.value === from) activeCommunity.value = to;
    if (category.value === from) category.value = to;
    renameCategoryOpen.value = false;
    await refresh();
    uni.showToast({ title: "小区分类已修改" });
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "分类修改失败", icon: "none" });
  } finally {
    saving.value = false;
  }
};
const selectCategory = (value: string) => {
  category.value = value;
  selectedMapLocation.value = null;
};
const chooseImages = () => {
  const remaining = Math.max(0, 9 - imageCount.value);
  if (!remaining) return;
  uni.chooseMedia({
    count: remaining,
    mediaType: ["image"],
    sourceType: ["album", "camera"],
    success: result => {
      const typedFiles = result.tempFiles as ChosenMediaFile[];
      const supported = typedFiles.filter(file => mediaFileMatches(file, "image"));
      const accepted = supported.filter(file => file.size <= maxImageBytes);
      pendingMedia.value.push(...accepted.map(file => ({ path: file.tempFilePath, type: "image" as const })));
      if (supported.length < typedFiles.length) uni.showToast({ title: "只能选择 JPG、PNG、WEBP 等图片", icon: "none" });
      else if (accepted.length < supported.length) uni.showToast({ title: "单张图片不能超过 10MB", icon: "none" });
    },
    fail: error => { if (!String(error.errMsg || "").includes("cancel")) uni.showToast({ title: "选择图片失败", icon: "none" }); },
  });
};
const chooseVideo = () => {
  if (hasVideo.value) return;
  uni.chooseMedia({
    count: 1,
    mediaType: ["video"],
    sourceType: ["album", "camera"],
    maxDuration: 60,
    success: result => {
      const file = result.tempFiles[0] as ChosenMediaFile | undefined;
      if (!file) return;
      if (!mediaFileMatches(file, "video")) {
        uni.showToast({ title: "请选择 MP4、MOV 等视频文件", icon: "none" });
        return;
      }
      if (file.size > maxVideoBytes) {
        uni.showToast({ title: "视频不能超过 40MB", icon: "none" });
        return;
      }
      if (Number(file.duration || 0) > 60) {
        uni.showToast({ title: "视频最长 60 秒", icon: "none" });
        return;
      }
      pendingMedia.value.push({ path: file.tempFilePath, type: "video" });
    },
    fail: error => { if (!String(error.errMsg || "").includes("cancel")) uni.showToast({ title: "选择视频失败", icon: "none" }); },
  });
};
const uploadInForeground = useForegroundUpload(
  () => { uploadProgress.value = "上传已暂停，返回后继续"; },
  () => { uploadProgress.value = "正在恢复上传…"; },
);
const upload = (file: PendingMedia, propertyId: string, sort: number) => {
  const uploadKey = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  return uploadInForeground<PropertyMedia>((resolve, reject) => {
  return uni.uploadFile({
    url: safeUploadUrl(`${API_ORIGIN}/api/v1/media/upload`),
    filePath: file.path,
    name: "file",
    header: { Authorization: `Bearer ${getSession()?.accessToken || ""}` },
    formData: { propertyId, uploadKey, mediaType: file.type, sort: String(sort) },
    success: response => {
      try {
        const body = JSON.parse(String(response.data || "{}"));
        if (response.statusCode >= 200 && response.statusCode < 300 && body?.code === 0 && body?.data?.id) resolve(body.data as PropertyMedia);
        else reject(new Error(body?.message || "素材上传失败"));
      } catch {
        reject(new Error("素材上传失败"));
      }
    },
    fail: error => reject(new Error(error.errMsg || "素材上传失败")),
  });
});
};
const resetForm = () => {
  createKey = "";
  createSnapshot.value = null;
  rentMonthly.value = "";
  pendingMedia.value = [];
  rentType.value = "整租";
  sharedRoomType.value = "主卧";
  materialRoomCount.value = 1;
  materialHallCount.value = 1;
  materialHasKitchen.value = true;
  materialBathroomCount.value = 1;
  materialBalconyCount.value = 0;
  materialRoomSequence.value = 0;
  materialFloor.value = "";
  materialAmenities.value = ["冰箱", "洗衣机", "空调", "热水器"];
  draftId.value = "";
  uploadedMedia.value = [];
  selectedMapLocation.value = null;
  category.value = "";
  entryCategoryFixed.value = false;
};
const hasUnsavedEntry = () => !!(
  pendingMedia.value.length || rentMonthly.value || materialFloor.value || materialRoomSequence.value ||
  rentType.value !== "整租" || sharedRoomType.value !== "主卧" || materialRoomCount.value !== 1 ||
  materialHallCount.value !== 1 || !materialHasKitchen.value || materialBathroomCount.value !== 1 ||
  materialBalconyCount.value !== 0 || materialAmenities.value.join("|") !== "冰箱|洗衣机|空调|热水器"
);
const closeEntry = () => {
  if (saving.value) return;
  if (draftId.value) {
    uni.showActionSheet({
      itemList: uploadedMedia.value.length ? ["继续上传", "保留已上传素材并退出", "删除本次素材"] : ["继续上传", "删除本次素材"],
      success: async ({ tapIndex }) => {
        if (tapIndex === 0) return;
        const keepUploaded = uploadedMedia.value.length > 0 && tapIndex === 1;
        const id = draftId.value;
        saving.value = true;
        try {
          if (keepUploaded) await replacePropertyMedia(id, uploadedMedia.value.map(item => item.id));
          else await deleteMaterial(id);
          resetForm();
          formOpen.value = false;
          await refresh();
          uni.showToast({ title: keepUploaded ? "已保留上传素材" : "已删除本次素材", icon: "none" });
        } catch (error) {
          uni.showModal({ title: "操作未完成", content: error instanceof Error ? error.message : "请检查网络后重试", showCancel: false });
        } finally { saving.value = false; }
      },
    });
    return;
  }
  if (!hasUnsavedEntry()) {
    resetForm();
    formOpen.value = false;
    return;
  }
  uni.showModal({
    title: "放弃本次录入？",
    confirmText: "放弃",
    success: ({ confirm }) => {
      if (!confirm) return;
      resetForm();
      formOpen.value = false;
    },
  });
};
const openEntry = () => {
  const selectedCategory = activeCommunity.value !== "全部" ? activeCommunity.value : "";
  if (!draftId.value && !pendingMedia.value.length) {
    selectCategory(selectedCategory);
    entryCategoryFixed.value = Boolean(selectedCategory);
  }
  formOpen.value = true;
};
const viewCategory = (value: string) => {
  if (formOpen.value) return;
  activeCommunity.value = value;
  categoryManagerOpen.value = false;
  communityFilterOpen.value = false;
  communityFilterQuery.value = "";
};
const viewLayout = (value: string) => {
  if (formOpen.value) return;
  activeLayout.value = value;
};
const saveMaterial = async () => {
  if (saving.value) return;
  const community = category.value;
  if (!community) {
    uni.showToast({ title: "先选择分类", icon: "none" });
    openCommunityPicker();
    return;
  }
  if (!pendingMedia.value.length) return;
  const invalidMedia = pendingMedia.value.find(file => {
    const chosenFile = { tempFilePath: file.path, fileType: file.type, size: 0 };
    return !mediaFileMatches(chosenFile, file.type);
  });
  if (invalidMedia) {
    uni.showToast({ title: invalidMedia.type === "image" ? "请移除非图片文件后重新选择" : "请移除非视频文件后重新选择", icon: "none" });
    return;
  }
  saving.value = true;
  try {
    if (!draftId.value) {
      if (!createKey) createKey = Date.now().toString(36) + "-" + Math.random().toString(36).slice(2);
      if (!createSnapshot.value) createSnapshot.value = {
        createKey,
        title: community,
        community,
        location: selectedCommunityLocation.value?.address || community,
        latitude: Number(selectedCommunityLocation.value?.latitude) || 0,
        longitude: Number(selectedCommunityLocation.value?.longitude) || 0,
        layout: rentType.value === "合租" ? sharedRoomType.value : materialLayoutText.value,
        rentMonthly: Math.max(0, Number(rentMonthly.value) || 0),
        rentType: rentType.value,
        floorInfo: materialFloor.value ? `${Number(materialFloor.value)}楼` : "",
        roomSequence: rentType.value === "合租" ? Math.min(materialRoomCount.value, Math.max(0, Number(materialRoomSequence.value) || 0)) : 0,
        ...(rentType.value === "合租" ? { wholeLayout: materialLayoutText.value } : {}),
        description: materialAmenities.value.length ? `配套设施：${materialAmenities.value.join("、")}` : "",
        materialCategory: community,
      };
    }
    await refreshCurrentSession("admin");
    if (!draftId.value) {

      const created = await createMaterial(createSnapshot.value!);
      draftId.value = created.id;
    }
    for (let index = uploadedMedia.value.length; index < pendingMedia.value.length; index++) {
      uploadProgress.value = `上传 ${index + 1}/${pendingMedia.value.length}`;
      uploadedMedia.value.push(await upload(pendingMedia.value[index], draftId.value, index));
    }
    await replacePropertyMedia(draftId.value, uploadedMedia.value.map(item => item.id));
    activeCommunity.value = community;
    resetForm();
    formOpen.value = false;
    await refresh();
    uni.showToast({ title: "素材已保存" });
  } catch (error) {
    uni.showModal({
      title: "保存未完成",
      content: `${error instanceof Error ? error.message : "请重试保存"}。可继续上传，或点击退出录入选择保留已上传素材。`,
      showCancel: false,
    });
  } finally {
    uploadProgress.value = "";
    saving.value = false;
  }
};
const removeMaterial = (item: Property) => {
  if (saving.value || item.id === draftId.value) return;
  uni.showModal({
    title: "删除素材？",
    content: "素材记录及已上传的图片、视频会一起删除，无法恢复。",
    success: async result => {
      if (!result.confirm) return;
      try {
        await deleteMaterial(item.id);
        materials.value = materials.value.filter(entry => entry.id !== item.id);
        uni.showToast({ title: "素材已删除" });
      } catch (error) {
        uni.showToast({ title: error instanceof Error ? error.message : "删除失败", icon: "none" });
      }
    },
  });
};
const openConvertedProperty = async (item: Property) => {
  if (!item.convertedPropertyId || saving.value) return;
  saving.value = true;
  let missing = false;
  try {
    await refreshCurrentSession("admin");
    linkedProperties.value = await listProperties();
    const property = linkedProperty(item);
    missing = !property;
    if (property) {
      const page = isDraft(property) ? "/pages/admin/property-edit" : "/pages/properties/detail";
      uni.navigateTo({ url: `${page}?id=${encodeURIComponent(property.id)}` });
    }
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "打开房源失败", icon: "none" });
  } finally {
    saving.value = false;
  }
  if (missing) convert(item);
};
const convert = (item: Property) => {
  if (saving.value || item.id === draftId.value) return;
  uni.showModal({
    title: "转为房源草稿",
    content: "将复制为房源草稿，原素材会保留在原分类并标记已转换。补齐信息后可发布。",
    confirmText: "转为草稿",
    success: async result => {
      if (!result.confirm) return;
      saving.value = true;
      try {
        await refreshCurrentSession("admin");
        const converted = await convertMaterial(item.id);
        await refresh();
        uni.navigateTo({ url: `/pages/admin/property-edit?id=${encodeURIComponent(converted.id)}` });
      } catch (error) {
        uni.showToast({ title: error instanceof Error ? error.message : "转换失败", icon: "none" });
      } finally {
        saving.value = false;
      }
    },
  });
};
</script>

<template>
  <view class="material-page">
    <view class="page-head"><view><strong>房源素材库</strong><small>按小区和户型整理，随时转为房源草稿。</small></view><button :disabled="saving" @click="formOpen ? closeEntry() : openEntry()">{{ formOpen ? (draftId ? "退出录入" : "收起") : "＋ 新增素材" }}</button></view>

    <view class="summary-row"><view><strong>{{ materials.length }}</strong><text>素材</text></view><view><strong>{{ categories.length }}</strong><text>分类</text></view><view><strong>{{ materials.reduce((sum, item) => sum + (item.media?.length || 0), 0) }}</strong><text>媒体</text></view></view>

    <view v-if="formOpen" class="entry-card">
      <view class="card-heading"><strong>新增素材</strong><text>发布时再补全信息</text></view>
      <view v-if="entryCategoryFixed && category" class="community-select-control fixed-category"><view><strong>{{ category }}</strong><small>当前分类 · 素材已归入该小区分类</small></view><text>已选分类</text></view>
      <view v-else class="community-select-control" @click="!saving && !createSnapshot && !draftId && openCommunityPicker()"><view><strong>{{ category || "选择分类" }}</strong><small>{{ category ? (selectedCommunityLocation?.address || "位置待补") : "分类按小区归属 · 可从已有分类中选择" }}</small></view><text>{{ draftId ? "已关联" : category ? "更换 ›" : "选择 ›" }}</text></view>
      <view class="media-picker-row"><button class="choose-image-button" :disabled="saving || !!createSnapshot || !!draftId || imageCount >= 9" @click="chooseImages">＋ 图片 <small>{{ imageCount }}/9</small></button><button class="choose-video-button" :disabled="saving || !!createSnapshot || !!draftId || hasVideo" @click="chooseVideo">＋ 视频 <small>{{ hasVideo ? 1 : 0 }}/1</small></button><text>图片 ≤10MB，视频 ≤40MB / 60秒</text></view>
      <view v-if="pendingMedia.length" class="pending-media"><view v-for="(media, index) in pendingMedia" :key="media.path" class="pending-item"><image v-if="media.type === 'image'" :src="media.path" mode="aspectFill"/><view v-else class="video-pending"><video :src="media.path" controls object-fit="cover" :show-fullscreen-btn="false"/></view><button v-if="!saving && !createSnapshot && !draftId" @click="pendingMedia.splice(index, 1)">×</button></view></view>
      <view class="material-basic" :class="{ 'entry-locked': saving || !!createSnapshot }">
        <view class="form-row">
          <text>出租方式</text>
          <view class="material-rent-types">
            <button :disabled="saving || !!createSnapshot" :class="{ active: rentType === '整租' }" @click="rentType = '整租'">整租</button>
            <button :disabled="saving || !!createSnapshot" :class="{ active: rentType === '合租' }" @click="rentType = '合租'">合租</button>
          </view>
        </view>
        <view class="material-layout-settings">
          <view class="material-field-title">{{ rentType === "合租" ? "整套户型" : "房屋户型" }}</view>
          <view class="material-choice-row"><button :disabled="saving || !!createSnapshot" v-for="count in [1,2,3,4,5,6,7,8,9]" :key="count" :class="{ active: materialRoomCount === count }" @click="materialRoomCount = count; materialRoomSequence = Math.min(materialRoomSequence, count)">{{ `${count}室` }}</button></view>
          <view class="material-choice-grid">
            <view><text>客厅</text><view class="material-choice-row"><button :disabled="saving || !!createSnapshot" v-for="count in [0,1,2,3]" :key="count" :class="{ active: materialHallCount === count }" @click="materialHallCount = count">{{ count ? `${count}厅` : "无" }}</button></view></view>
            <view><text>厨房</text><view class="material-choice-row"><button :disabled="saving || !!createSnapshot" :class="{ active: materialHasKitchen }" @click="materialHasKitchen = true">有</button><button :disabled="saving || !!createSnapshot" :class="{ active: !materialHasKitchen }" @click="materialHasKitchen = false">无</button></view></view>
            <view><text>卫生间</text><view class="material-choice-row"><button :disabled="saving || !!createSnapshot" v-for="count in [1,2,3,4]" :key="count" :class="{ active: materialBathroomCount === count }" @click="materialBathroomCount = count">{{ count }}个</button></view></view>
            <view><text>阳台</text><view class="material-choice-row"><button :disabled="saving || !!createSnapshot" v-for="count in [0,1,2,3,4]" :key="count" :class="{ active: materialBalconyCount === count }" @click="materialBalconyCount = count">{{ count ? `${count}个` : "无" }}</button></view></view>
          </view>
        </view>
        <view v-if="rentType === '合租'" class="material-layout-settings">
          <view class="material-field-title">出租房型</view>
          <view class="material-choice-row"><button :disabled="saving || !!createSnapshot" v-for="room in ['主卧','次卧','单间','床位']" :key="room" :class="{ active: sharedRoomType === room }" @click="sharedRoomType = room">{{ room }}</button></view>
          <view class="material-field-title room-sequence-title">房间序号 <small>选填</small></view>
          <view class="material-choice-row"><button :disabled="saving || !!createSnapshot" :class="{ active: !materialRoomSequence }" @click="materialRoomSequence = 0">不指定</button><button :disabled="saving || !!createSnapshot" v-for="number in Math.min(materialRoomCount, 9)" :key="number" :class="{ active: materialRoomSequence === number }" @click="materialRoomSequence = number">{{ number }}号房</button></view>
        </view>
        <view class="material-field-pair">
          <label>楼层 <small>选填</small><view class="material-floor-input"><input :disabled="saving || !!createSnapshot" v-model="materialFloor" type="number" maxlength="2" placeholder="输入楼层" /><text>楼</text></view></label>
          <label>月租 <input :disabled="saving || !!createSnapshot" v-model="rentMonthly" type="number" placeholder="元/月" /></label>
        </view>
        <view class="material-layout-settings">
          <view class="material-field-title">房屋设施</view>
          <view class="material-choice-row"><button :disabled="saving || !!createSnapshot" v-for="name in materialAmenityOptions" :key="name" :class="{ active: materialAmenities.includes(name) }" @click="toggleMaterialAmenity(name)">{{ name }}</button></view>
        </view>
      </view>
      <button class="save-button" :loading="saving" :disabled="saving || !pendingMedia.length" @click="saveMaterial">{{ uploadProgress || (saving ? "保存中…" : draftId ? "继续保存" : "保存素材") }}</button>
    </view>

    <view v-if="!formOpen" class="filter-panel">
      <view class="community-filter-heading"><strong>{{ materials.length ? "素材分类" : "小区分类" }}</strong><button @click="toggleCategoryManager">{{ categoryManagerOpen ? "收起分类" : "分类管理" }}</button></view>
      <view v-if="materials.length" class="material-archive-row"><button :class="{ active: !showConvertedArchive }" @click="showConvertedArchive = false">全部素材 <text>{{ materials.length }}</text></button><button :class="{ active: showConvertedArchive }" @click="showConvertedArchive = true">已转房源 <text>{{ convertedCount }}</text></button></view>
      <template v-if="materials.length">
        <view class="filter-row community-filter-row"><button class="community-filter-trigger" @click="openCommunityFilter"><text class="community-filter-current">{{ activeCommunity }}</text><text class="community-filter-count">{{ countForCommunity(activeCommunity) }} 份</text><small>选择分类</small></button></view>
        <view class="filter-row layout-filter-row"><strong>户型</strong><view class="category-scroll"><view class="category-row"><button :class="{ active: activeLayout === '全部' }" @click="viewLayout('全部')">全部 {{ countForLayout('全部') }}</button><button v-for="name in layoutFilterOptions" :key="name" :class="{ active: activeLayout === name }" @click="viewLayout(name)">{{ name }} {{ countForLayout(name) }}</button></view></view></view>
      </template>
    </view>
    <view v-if="communityFilterOpen && !formOpen" class="community-filter-mask" @click="communityFilterOpen = false"><view class="community-filter-sheet" @click.stop><view class="community-filter-head"><view><strong>选择分类</strong><small>分类按小区归属 · 共 {{ communityOptions.length }} 个</small></view><button class="community-filter-add" @click="startAddCommunityCategory('filter')">＋ 添加分类</button><text @click="communityFilterOpen = false">×</text></view><input v-model.trim="communityFilterQuery" class="community-filter-search" placeholder="搜索分类名称（小区名）"/><scroll-view scroll-y class="community-filter-options"><view class="community-filter-grid"><view v-for="name in filteredCommunityOptions" :key="name" class="community-filter-option" :class="{ active: activeCommunity === name }" @click="viewCategory(name)"><text class="community-filter-name">{{ name }}</text><text class="community-filter-count">{{ countForCommunity(name) }}</text></view><view v-if="!filteredCommunityOptions.length" class="community-filter-no-result">没有匹配的分类</view></view></scroll-view></view></view>
    <view v-if="categoryManagerOpen && !formOpen" class="category-manager">
      <view class="category-manager-heading"><view><strong>小区分类</strong></view><button class="manager-add-category" @click="startAddCommunityCategory('manager')">＋ 添加分类</button></view>
      <view v-if="!categories.length" class="category-manager-empty"><strong>还没有分类</strong><text>选择小区，即可建立对应分类。</text><button @click="openCommunityPicker('', 'manager')">选择小区并建分类</button></view>
      <view class="community-category-grid"><view v-for="name in categories" :key="name" class="community-category-mini" @click="openCategoryActions(name)"><view><strong>{{ name }}</strong><text>{{ categoryMaterials(name).length }} 份素材</text></view><text class="community-category-address-compact">{{ categoryLocation(name)?.address || "位置待补" }}</text></view></view>
    </view>

    <view v-if="categoryActionOpen && !formOpen" class="category-action-mask" @click="categoryActionOpen = false"><view class="category-action-sheet" @click.stop><view class="category-action-heading"><view><strong>{{ categoryActionName }}</strong><small>{{ categoryMaterials(categoryActionName).length }} 份素材 · {{ categoryLocation(categoryActionName)?.address || "位置待补" }}</small></view><text @click="categoryActionOpen = false">×</text></view><view class="category-action-grid"><button class="category-action-view" @click="performCategoryAction('view')"><text>▤</text><view><strong>查看素材</strong><small>{{ categoryMaterials(categoryActionName).length }} 份素材</small></view></button><button class="category-action-add" @click="performCategoryAction('add')"><text>＋</text><view><strong>在此分类新增</strong><small>自动归入 {{ categoryActionName }}</small></view></button><button class="category-action-rename" @click="performCategoryAction('rename')"><text>✎</text><view><strong>修改名称</strong><small>调整小区分类名</small></view></button><button class="category-action-location" @click="performCategoryAction('location')"><text>⌖</text><view><strong>{{ categoryLocation(categoryActionName) ? "修改地址" : "补充地址" }}</strong><small>管理分类位置信息</small></view></button></view><button class="category-action-remove" @click="performCategoryAction('remove')"><text>移除分类</text><small>不会删除素材或小区库数据</small></button></view></view>

    <view v-if="renameCategoryOpen && !formOpen" class="rename-category-mask" @click="renameCategoryOpen = false"><view class="rename-category-sheet" @click.stop><strong>修改小区名称</strong><small>该分类下的素材会同步更新归属</small><input v-model.trim="renameCategoryValue" maxlength="60" placeholder="输入小区名称"/><view><button class="rename-category-cancel" @click="renameCategoryOpen = false">取消</button><button class="rename-category-save" :loading="saving" :disabled="saving || !renameCategoryValue.trim()" @click="saveCategoryRename">保存修改</button></view></view></view>

    <view v-if="communityPickerOpen" class="community-picker-mask" @click="dismissCommunityPicker">
      <view class="community-picker-sheet" @click.stop>
        <view class="community-picker-head">
          <view><strong>{{ communityPickerTargetCategory ? "补充小区位置" : communityPickerReturnTo === "entry" ? "选择分类" : "选择小区分类" }}</strong><small>{{ communityPickerTargetCategory ? "为该小区分类补充地图位置" : communityPickerReturnTo === "entry" ? "分类按小区归属；优先选择已有分类" : "小区库优先；没有时搜地图" }}</small></view>
          <text @click="dismissCommunityPicker">×</text>
        </view>
        <view class="community-picker-search">
          <input v-model.trim="communityPickerQuery" placeholder="搜索小区名称" confirm-type="search" @confirm="searchCommunityMap"/>
          <button :disabled="communityMapSearching || communityPickerQuery.trim().length < 2" @click="searchCommunityMap">{{ communityMapSearching ? "搜索中…" : communityMapSearched ? "重新搜索地图" : "搜索地图" }}</button>
        </view>
        <view class="community-picker-section">
          <view class="community-picker-section-head"><strong>{{ communityPickerReturnTo === "entry" ? "已有分类" : "已有小区" }}</strong><button v-if="canCreateCommunityCategory" @click="chooseCommunity(communityPickerQuery)">＋ 添加分类</button></view>
          <scroll-view v-if="filteredCommunityLibraryOptions.length" scroll-y class="community-picker-list">
            <view v-for="name in filteredCommunityLibraryOptions" :key="name" class="community-picker-option" @click="chooseCommunity(name)"><view><strong>{{ name }}</strong><small>{{ communityLocations[name]?.address || "小区库记录" }}</small></view><text>小区库</text></view>
          </scroll-view>
          <view v-else class="community-picker-empty compact-empty">没有匹配结果</view>
        </view>
        <view v-if="communityPickerQuery.trim().length >= 2" class="community-picker-section map-section">
          <strong>地图结果</strong>
          <scroll-view v-if="communityMapResults.length" scroll-y class="community-picker-list">
            <view v-for="item in communityMapResults" :key="item.id" class="community-picker-option map-option" @click="chooseCommunity(item.title, item)"><view><strong>{{ item.title }}</strong><small>{{ [item.district, item.address].filter(Boolean).join(" · ") || "地图地点" }}</small></view><text>地图结果</text></view>
          </scroll-view>
          <view v-else-if="communityMapSearched && !communityMapSearching" class="community-picker-empty compact-empty">没有合适地图结果</view>
        </view>
      </view>
    </view>

    <checkbox-group v-if="!formOpen && filteredMaterials.length" class="material-price-switch" @change="togglePrice($event.detail.value)">
      <label><checkbox value="price" :checked="showPrice" color="#40543f" />显示价格</label>
    </checkbox-group>
    <view v-if="!formOpen && loading" class="empty-state">正在加载素材…</view>
    <view v-else-if="!formOpen && !materials.length" class="empty-state"><strong>还没有素材</strong><button @click="openEntry">＋ 新增素材</button></view>
    <view v-else-if="!formOpen && !filteredMaterials.length" class="empty-state"><strong>{{ activeMaterials.length ? "当前筛选下没有素材" : "还没有已转房源的素材" }}</strong><button v-if="!showConvertedArchive && !activeMaterials.length" @click="openEntry">＋ 新增素材</button><button v-else-if="activeCommunity !== '全部' || activeLayout !== '全部'" class="clear-filters" @click="activeCommunity = '全部'; activeLayout = '全部'">清除筛选</button></view>
    <view v-else-if="!formOpen" class="material-list">
      <view v-for="item in filteredMaterials" :key="item.id" class="material-card" @click="openMaterialDetail(item)">
        <view class="material-media-strip"><image v-if="item.media?.some(media => media.type === 'image')" :src="mediaUrl(item.media.find(media => media.type === 'image')!.url)" mode="aspectFill" /><view v-else-if="item.media?.some(media => media.type === 'video')" class="video-cover"><text>▶</text><small>视频素材</small></view><view v-else class="no-media">暂无图片或视频</view><view class="material-count">{{ item.media?.filter(media => media.type === 'image').length || 0 }}图 · {{ item.media?.filter(media => media.type === 'video').length || 0 }}视频</view></view>
        <view class="material-copy"><view class="title-line"><strong>{{ item.title || item.community }}</strong><text>{{ materialCommunity(item) }}</text></view><view class="material-tags"><text>{{ materialLayout(item) }}</text><text>{{ item.rentType || "出租方式待补" }}</text><text v-for="tag in extraMaterialTags(item)" :key="tag">{{ tag }}</text><text v-if="showPrice" class="material-price-tag">{{ item.rentMonthly }} 元/月</text></view><view v-if="item.description" class="optional-info"><text>{{ item.description }}</text></view><view v-if="isConverted(item)" class="converted-note">已转房源</view><view class="card-actions"><button v-if="isConverted(item)" class="convert-button" @click.stop="openConvertedProperty(item)">{{ linkedActionLabel(item) }}</button><button v-else class="convert-button" @click.stop="convert(item)">转为房源草稿</button><button class="delete-button" @click.stop="removeMaterial(item)">删除</button></view></view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.material-page{min-height:100vh;padding:28rpx 24rpx 80rpx;background:#f7f6f1;color:#26372c;box-sizing:border-box}.page-head{display:flex;align-items:center;justify-content:space-between;gap:16rpx;margin:8rpx 2rpx 24rpx}.page-head>view{display:flex;flex-direction:column;gap:8rpx}.page-head strong{font-size:42rpx;font-weight:800}.page-head small{color:#7c877a;font-size:23rpx}.page-head button,.manage-categories,.add-category button{margin:0;padding:0 22rpx;border:0;border-radius:14rpx;background:#40543f;color:#fff;font-size:23rpx;font-weight:700;line-height:72rpx;white-space:nowrap}.page-head button::after,.manage-categories::after,.add-category button::after{border:0}.summary-row{display:grid;grid-template-columns:repeat(3,1fr);gap:12rpx;margin-bottom:20rpx}.summary-row>view{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:4rpx;padding:20rpx;border-radius:18rpx;background:#fff;text-align:center}.summary-row strong{font-size:32rpx;color:#213a5e;text-align:center}.summary-row text{font-size:21rpx;color:#7d877c;text-align:center}.entry-card,.category-manager,.material-card{margin-bottom:20rpx;padding:22rpx;border:1rpx solid #e1e8dd;border-radius:20rpx;background:#fff}.card-heading{display:flex;align-items:baseline;justify-content:space-between;margin-bottom:18rpx}.card-heading strong{font-size:30rpx}.card-heading text{font-size:20rpx;color:#7e897c}.form-row{display:grid;gap:10rpx;margin:14rpx 0}.form-row>text,.form-pair label{color:#54634f;font-size:23rpx;font-weight:650}.form-row input,.form-row textarea,.form-pair input,.form-pair picker view,.add-category input{width:100%;min-height:76rpx;padding:0 18rpx;border:1rpx solid #dce5d9;border-radius:13rpx;box-sizing:border-box;background:#fbfcfa;color:#28382b;font-size:24rpx}.form-row textarea{height:120rpx;padding-top:14rpx}.form-pair{display:grid;grid-template-columns:1fr 1fr;gap:14rpx;margin:14rpx 0}.form-pair label{display:grid;gap:9rpx}.form-pair picker view{display:flex;align-items:center}.media-picker-row{display:flex;align-items:center;gap:10rpx;flex-wrap:wrap;margin:18rpx 0}.media-picker-row button{margin:0;padding:0 18rpx;border:1rpx solid #d7e4d5;border-radius:12rpx;background:#f3f7ef;color:#40543f;font-size:22rpx;line-height:64rpx}.media-picker-row button::after{border:0}.media-picker-row small{margin-left:5rpx}.media-picker-row>text{color:#858f82;font-size:20rpx}.pending-media{display:flex;gap:10rpx;flex-wrap:wrap;margin:14rpx 0}.pending-item{position:relative;width:126rpx;height:126rpx}.pending-item image,.video-pending{width:100%;height:100%;border-radius:12rpx}.video-pending{overflow:hidden;background:#e9eee4}.video-pending video{width:100%;height:100%;border-radius:12rpx}.pending-item button{position:absolute;z-index:3;right:-8rpx;top:-8rpx;width:38rpx;height:38rpx;padding:0;border-radius:50%;background:#26372c;color:white;line-height:38rpx}.pending-item button::after{border:0}.save-button{margin:18rpx 0 0;border:0;border-radius:14rpx;background:#40543f;color:#fff;font-weight:700;line-height:78rpx}.save-button::after{border:0}.category-head{display:flex;align-items:center;gap:8rpx;margin:22rpx 0 14rpx}.category-scroll{flex:1;min-width:0}.category-row{display:flex;flex-wrap:wrap;gap:10rpx}.category-row button{flex:none;margin:0;padding:0 18rpx;border:1rpx solid #dce5d9;border-radius:30rpx;background:#fff;color:#596856;font-size:21rpx;line-height:62rpx}.category-row button.active{border-color:#40543f;background:#40543f;color:#fff}.category-row button::after{border:0}.manage-categories{padding:0 14rpx;background:#edf2e8;color:#53644e;font-size:19rpx;line-height:58rpx}.category-manager{display:flex;align-items:center;gap:10rpx;flex-wrap:wrap}.category-chip{display:flex;align-items:center;gap:4rpx;padding:4rpx 8rpx 4rpx 14rpx;border-radius:24rpx;background:#f1f5ed;color:#40543f;font-size:21rpx}.category-chip button{width:36rpx;height:36rpx;margin:0;padding:0;border:0;border-radius:50%;background:#dfe9d9;color:#40543f;font-size:26rpx;line-height:36rpx}.category-chip button::after{border:0}.add-category{display:flex;gap:8rpx;flex:1;min-width:250rpx}.add-category input{min-width:0;min-height:58rpx;font-size:21rpx}.add-category button{line-height:58rpx;font-size:20rpx}.material-list{display:grid;gap:16rpx}.material-card{padding:0;overflow:hidden}.material-media-strip{position:relative;display:flex;height:220rpx;overflow:hidden;background:#edf0e9}.material-media-strip>image,.video-tile{flex:1;min-width:0;height:100%;object-fit:cover}.video-tile{position:relative}.video-tile video{width:100%;height:100%}.video-tile text{position:absolute;left:10rpx;top:10rpx;padding:4rpx 10rpx;border-radius:10rpx;background:#26372ccc;color:#fff;font-size:18rpx}.no-media{display:grid;place-items:center;width:100%;color:#8b9587;font-size:22rpx}.material-count{position:absolute;right:12rpx;bottom:12rpx;padding:7rpx 12rpx;border-radius:20rpx;background:#26372ccc;color:#fff;font-size:18rpx}.material-copy{padding:18rpx}.title-line{display:flex;align-items:center;justify-content:space-between;gap:12rpx}.title-line strong{font-size:28rpx}.title-line text{flex:none;padding:6rpx 12rpx;border-radius:18rpx;background:#eef3e9;color:#52634c;font-size:19rpx}.material-copy>small{display:block;margin-top:8rpx;color:#737f71;font-size:21rpx;line-height:1.5}.optional-info{display:flex;gap:10rpx;flex-wrap:wrap;margin-top:10rpx;color:#667363;font-size:20rpx}.card-actions{display:flex;gap:10rpx;margin-top:16rpx}.card-actions button{margin:0;padding:0 18rpx;border:0;border-radius:11rpx;font-size:21rpx;font-weight:650;line-height:62rpx}.card-actions button::after{border:0}.convert-button{background:#40543f;color:#fff}.delete-button{background:#fbefed;color:#a24d40}.empty-state{display:grid;justify-items:center;gap:12rpx;padding:90rpx 32rpx;color:#778273;text-align:center}.empty-state strong{font-size:28rpx;color:#40543f}.empty-state text{font-size:22rpx;line-height:1.5}.empty-state button{margin:8rpx 0 0;padding:0 22rpx;border:0;border-radius:12rpx;background:#40543f;color:#fff;font-size:22rpx;line-height:68rpx}.empty-state button::after{border:0}
.community-picker{display:flex;align-items:center;width:100%;min-height:76rpx;padding:0 18rpx;border:1rpx solid #dce5d9;border-radius:13rpx;box-sizing:border-box;background:#fbfcfa;color:#28382b;font-size:24rpx}.empty-picker{color:#9a7250;background:#fcf8ef}.field-hint{color:#7f8b7b;font-size:19rpx;line-height:1.4}.category-manager-title{display:grid;gap:5rpx;width:100%}.category-manager-title strong{font-size:24rpx;color:#40543f}.category-manager-title small{font-size:19rpx;color:#7f8b7b;line-height:1.4}
.material-basic { display:grid;gap:14rpx; }
.material-rent-types,.material-tags { display:flex;flex-wrap:wrap;gap:8rpx; }
.material-rent-types button { margin:0;padding:0 22rpx;line-height:60rpx;font-size:23rpx;background:#f0f4ec;color:#40543f; }
.material-rent-types button.active { background:#40543f;color:#fff; }
.material-layout-settings{display:grid;gap:13rpx;padding:18rpx;border:1rpx solid #e1e8dd;border-radius:15rpx;background:#f8faf6}.material-field-title{color:#40543f;font-size:23rpx;font-weight:700}.material-field-title small,.material-field-pair label>small{margin-left:8rpx;color:#879184;font-size:18rpx;font-weight:500}.material-choice-row{display:flex;flex-wrap:wrap;gap:9rpx}.material-choice-row button{margin:0;padding:0 15rpx;border:1rpx solid #dce5d9;border-radius:11rpx;background:#fff;color:#52634f;font-size:20rpx;line-height:52rpx}.material-choice-row button.active{border-color:#40543f;background:#40543f;color:#fff}.material-choice-row button::after{border:0}.material-choice-grid{display:grid;gap:14rpx}.material-choice-grid>view{display:flex;align-items:flex-start;gap:12rpx}.material-choice-grid>view>text{flex:none;width:86rpx;padding-top:12rpx;color:#687665;font-size:20rpx}.room-sequence-title{margin-top:4rpx}.material-field-pair{display:grid;grid-template-columns:1fr 1fr;gap:14rpx}.material-field-pair label{display:grid;align-content:start;gap:9rpx;color:#54634f;font-size:22rpx;font-weight:650}.material-field-pair input{width:100%;height:72rpx;padding:0 16rpx;border:1rpx solid #dce5d9;border-radius:12rpx;box-sizing:border-box;background:#fbfcfa;color:#28382b;font-size:23rpx}.material-floor-input{display:flex;align-items:center;gap:8rpx}.material-floor-input text{color:#667363;font-size:21rpx}
.material-tags text { padding:4rpx 9rpx;border-radius:8rpx;background:#eef3e9;color:#40543f;font-size:20rpx; }
.material-tags .material-price-tag { background:#fff0e9;color:#a64d32; }
.material-price-switch { display:flex;justify-content:flex-end;margin:12rpx 0;color:#40543f;font-size:22rpx; }
.community-select-control{display:flex;align-items:center;justify-content:space-between;gap:14rpx;margin:12rpx 0 18rpx;padding:16rpx 18rpx;border:1rpx solid #dce5d9;border-radius:14rpx;background:#f7f9f4}.community-select-control>view{display:grid;gap:5rpx;min-width:0}.community-select-control strong{font-size:24rpx;color:#334b36}.community-select-control small{font-size:19rpx;color:#798676}.community-select-control>text{flex:none;color:#40543f;font-size:21rpx;font-weight:700}.filter-panel{display:grid;gap:12rpx;margin:18rpx 0;padding:14rpx;border:1rpx solid #e1e8dd;border-radius:18rpx;background:#fff}.filter-row{display:flex;align-items:center;gap:14rpx;min-width:0}.layout-filter-row{align-items:flex-start}.layout-filter-row>strong{padding-top:12rpx}.filter-row>strong{flex:none;width:54rpx;color:#40543f;font-size:21rpx}.filter-row .category-row{padding:2rpx 0}.filter-row .category-row button{font-size:20rpx;line-height:54rpx;padding:0 15rpx}.category-manager-title{flex-basis:100%}.add-community-category,.clear-filters{margin:0;padding:0 18rpx;border:1rpx solid #d7e4d5;border-radius:12rpx;background:#f3f7ef;color:#40543f;font-size:21rpx;line-height:58rpx}.add-community-category::after,.clear-filters::after{border:0}.community-picker-mask{position:fixed;z-index:999;inset:0;display:flex;align-items:flex-end;justify-content:center;background:rgba(24,34,27,.48)}.community-picker-sheet{display:flex;flex-direction:column;width:100%;max-height:82vh;padding:24rpx 22rpx calc(24rpx + env(safe-area-inset-bottom));box-sizing:border-box;border-radius:26rpx 26rpx 0 0;background:#fdfdfa}.community-picker-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:16rpx}.community-picker-head>view{display:grid;gap:6rpx}.community-picker-head strong{font-size:28rpx;color:#26372c}.community-picker-head small{font-size:20rpx;color:#788474}.community-picker-head>text{display:grid;place-items:center;width:54rpx;height:54rpx;border-radius:50%;background:#eef3e9;color:#536650;font-size:40rpx;line-height:1}.community-picker-search{display:flex;gap:10rpx;margin-bottom:14rpx}.community-picker-search input{flex:1;min-width:0;height:72rpx;padding:0 16rpx;border:1rpx solid #dce5d9;border-radius:12rpx;background:#fff;color:#28382b;font-size:22rpx}.community-picker-search button{flex:none;margin:0;padding:0 16rpx;border:0;border-radius:12rpx;background:#40543f;color:#fff;font-size:20rpx;line-height:72rpx}.community-picker-search button::after{border:0}.community-picker-section{display:flex;flex-direction:column;min-height:0;margin-top:8rpx}.community-picker-section>strong{margin:4rpx 0 8rpx;color:#52634e;font-size:21rpx}.community-picker-list{height:24vh;min-height:120rpx;border:1rpx solid #e2e9df;border-radius:12rpx;background:#fff}.community-picker-option{display:flex;align-items:center;justify-content:space-between;gap:12rpx;padding:14rpx;border-bottom:1rpx solid #edf0eb}.community-picker-option:last-child{border-bottom:0}.community-picker-option>view{display:grid;gap:4rpx;min-width:0}.community-picker-option strong{font-size:22rpx;color:#2f4032}.community-picker-option small{overflow:hidden;color:#7b8778;font-size:18rpx;text-overflow:ellipsis;white-space:nowrap}.community-picker-option>text{flex:none;padding:5rpx 9rpx;border-radius:9rpx;background:#eaf3eb;color:#26724b;font-size:17rpx}.community-picker-option.map-option>text{background:#eaf1ff;color:#3568ad}.community-picker-empty{padding:28rpx 12rpx;color:#899487;font-size:20rpx;text-align:center}.community-picker-section.map-section .community-picker-list{height:20vh}
.page-head>view{flex:1;min-width:0}.page-head button{flex:none;padding:0 15rpx;font-size:21rpx}.community-picker-empty.compact-empty{padding:17rpx 12rpx;border:1rpx solid #e2e9df;border-radius:12rpx;background:#fff}
.category-manager{display:grid;gap:14rpx;padding:18rpx}.category-manager-heading{display:flex;align-items:center;justify-content:space-between;gap:12rpx}.category-manager-heading>view{display:grid;gap:5rpx;min-width:0}.category-manager-heading strong{font-size:26rpx;color:#26372c}.category-manager-heading small{color:#788474;font-size:19rpx;line-height:1.4}.category-manager-heading button,.category-manager-empty button{flex:none;margin:0;padding:0 15rpx;border:0;border-radius:11rpx;background:#40543f;color:#fff;font-size:20rpx;font-weight:700;line-height:58rpx}.category-manager-heading button::after,.category-manager-empty button::after{border:0}.category-manager-stats{display:grid;grid-template-columns:repeat(4,1fr);gap:8rpx}.category-manager-stats>view{display:grid;justify-items:center;gap:4rpx;padding:12rpx 4rpx;border-radius:12rpx;background:#f5f7f1}.category-manager-stats strong{font-size:25rpx;color:#213a5e}.category-manager-stats text{font-size:16rpx;color:#788474;text-align:center}.community-category-card{padding:15rpx;border:1rpx solid #e1e8dd;border-radius:15rpx;background:#fbfcfa}.community-category-top{display:flex;align-items:center;gap:10rpx}.community-category-icon{display:grid;place-items:center;flex:none;width:58rpx;height:58rpx;border-radius:14rpx;background:#eaf1e5;color:#40543f;font-size:19rpx;font-weight:750}.community-category-title{display:grid;gap:4rpx;flex:1;min-width:0}.community-category-title strong{overflow:hidden;color:#293a2d;font-size:23rpx;text-overflow:ellipsis;white-space:nowrap}.community-category-title small{color:#8a9387;font-size:17rpx}.community-category-view{flex:none;margin:0;padding:0 12rpx;border:1rpx solid #d8e3d6;border-radius:10rpx;background:#fff;color:#40543f;font-size:19rpx;line-height:50rpx}.community-category-view::after,.community-category-actions button::after{border:0}.community-category-address{display:flex;align-items:center;justify-content:space-between;gap:10rpx;margin-top:12rpx;padding:10rpx 12rpx;border-radius:10rpx;background:#f3f5ef}.community-category-address>text:first-child{overflow:hidden;color:#667363;font-size:18rpx;text-overflow:ellipsis;white-space:nowrap}.community-category-address>text:last-child{flex:none;font-size:17rpx;font-weight:650}.location-ready{color:#28734c}.location-missing{color:#a77b36}.community-category-meta{display:flex;align-items:center;gap:12rpx;flex-wrap:wrap;margin:10rpx 2rpx 0}.community-category-meta text{color:#788474;font-size:17rpx}.community-category-actions{display:flex;justify-content:space-between;gap:10rpx;margin-top:12rpx;padding-top:11rpx;border-top:1rpx solid #e9ede5}.community-category-actions button{margin:0;padding:0 12rpx;border:0;border-radius:9rpx;background:#edf2e8;color:#40543f;font-size:18rpx;line-height:48rpx}.community-category-actions button:last-child{background:#fff2ef;color:#a15144}.community-category-actions button[disabled]{background:#f1f2ef;color:#9aa196}.category-manager-empty{display:grid;justify-items:center;gap:9rpx;padding:30rpx 18rpx;border:1r dashed #d8e3d6;border-radius:14rpx;text-align:center}.category-manager-empty strong{color:#40543f;font-size:23rpx}.category-manager-empty text{color:#788474;font-size:19rpx}
.community-category-actions{align-items:center}.community-category-actions>text{color:#788474;font-size:17rpx}.community-category-actions button{background:#fff2ef;color:#a15144}.community-category-actions button[disabled]{background:#f1f2ef;color:#9aa196}
.community-category-actions .community-category-fill{background:#edf2e8;color:#40543f}
.category-manager{gap:10rpx;padding:14rpx}.category-manager-heading strong{font-size:25rpx}.category-manager-heading small{font-size:18rpx}.category-manager-summary{margin-top:-4rpx;color:#788474;font-size:18rpx}.community-category-card{padding:12rpx}.community-category-top{gap:8rpx}.community-category-top>strong{flex:1;min-width:0;overflow:hidden;color:#293a2d;font-size:22rpx;text-overflow:ellipsis;white-space:nowrap}.community-category-top>text{flex:none;padding:4rpx 8rpx;border-radius:8rpx;background:#f1f5ed;color:#667363;font-size:16rpx}.community-category-address{margin-top:8rpx;padding:8rpx 10rpx}.community-category-address>text:first-child{font-size:17rpx}.community-category-address>text:last-child{font-size:16rpx}.community-category-actions{margin-top:8rpx;padding-top:8rpx}.community-category-actions>text{font-size:16rpx}.community-category-actions button{font-size:17rpx;line-height:42rpx}
.community-filter-row{align-items:center}.community-filter-trigger{display:flex;align-items:center;gap:10rpx;min-width:0;flex:1;margin:0;padding:0 14rpx;border:1rpx solid #dbe5d7;border-radius:13rpx;background:#f8faf6;text-align:left;line-height:58rpx}.community-filter-trigger::after{border:0}.community-filter-trigger>.community-filter-current{overflow:hidden;flex:1;color:#334b36;font-size:21rpx;font-weight:700;text-overflow:ellipsis;white-space:nowrap}.community-filter-count{display:inline-flex;align-items:center;justify-content:center;flex:none;min-width:42rpx;padding:2rpx 10rpx;border-radius:20rpx;background:#40543f;color:#fff;font-size:19rpx;font-weight:800;line-height:36rpx;text-align:center}.community-filter-trigger>small{flex:none;color:#697866;font-size:18rpx}.community-filter-mask{position:fixed;z-index:1000;inset:0;display:flex;align-items:flex-end;justify-content:center;background:rgba(24,34,27,.48)}.community-filter-sheet{display:flex;flex-direction:column;width:100%;max-height:74vh;padding:22rpx 20rpx calc(20rpx + env(safe-area-inset-bottom));box-sizing:border-box;border-radius:24rpx 24rpx 0 0;background:#fdfdfa}.community-filter-head{display:flex;align-items:center;justify-content:space-between;gap:12rpx;margin-bottom:14rpx}.community-filter-head>view{display:grid;gap:5rpx}.community-filter-head strong{color:#26372c;font-size:26rpx}.community-filter-head small{color:#788474;font-size:18rpx}.community-filter-head>text{display:grid;place-items:center;flex:none;width:48rpx;height:48rpx;border-radius:50%;background:#edf2e8;color:#536650;font-size:34rpx;line-height:1}.community-filter-search{flex:none;height:66rpx;margin-bottom:12rpx;padding:0 15rpx;border:1rpx solid #dce5d9;border-radius:12rpx;background:#fff;color:#28382b;font-size:21rpx}.community-filter-options{min-height:180rpx;max-height:54vh;border:1rpx solid #e2e9df;border-radius:13rpx;background:#fff}.community-filter-option{display:flex;align-items:center;justify-content:space-between;gap:12rpx;min-height:70rpx;padding:0 15rpx;border-bottom:1rpx solid #edf0eb}.community-filter-option:last-child{border-bottom:0}.community-filter-option>.community-filter-name{overflow:hidden;flex:1;color:#344536;font-size:21rpx;text-overflow:ellipsis;white-space:nowrap}.community-filter-option>.community-filter-count{min-width:42rpx;background:#e8eee3;color:#344c36;font-size:20rpx;line-height:38rpx}.community-filter-option.active{background:#f0f5ed}.community-filter-option.active>.community-filter-name{color:#2d674b;font-weight:750}.community-filter-option.active>.community-filter-count{background:#40543f;color:#fff}.community-filter-no-result{padding:24rpx;color:#899487;font-size:20rpx;text-align:center}

/* Compact material rows and scannable community filter cards */
.material-list{gap:12rpx}.material-card{display:flex;align-items:stretch;gap:12rpx;padding:10rpx;border-radius:16rpx}.material-media-strip{flex:none;width:178rpx;height:156rpx;border-radius:12rpx}.material-media-strip>image,.video-tile{border-radius:10rpx}.material-copy{display:flex;flex:1;flex-direction:column;justify-content:center;min-width:0;padding:4rpx 2rpx}.title-line{align-items:flex-start;gap:6rpx}.title-line strong{display:-webkit-box;overflow:hidden;flex:1;font-size:22rpx;line-height:1.35;-webkit-box-orient:vertical;-webkit-line-clamp:2}.title-line text{max-width:112rpx;overflow:hidden;padding:4rpx 8rpx;font-size:16rpx;text-overflow:ellipsis;white-space:nowrap}.material-tags{gap:6rpx;margin-top:7rpx}.material-tags text{padding:3rpx 7rpx;font-size:17rpx;line-height:1.25}.material-copy>small{margin-top:5rpx;font-size:17rpx;line-height:1.35}.optional-info{margin-top:5rpx;font-size:17rpx}.card-actions{gap:7rpx;margin-top:auto;padding-top:8rpx}.card-actions button{padding:0 11rpx;font-size:17rpx;line-height:46rpx}.material-count{right:6rpx;bottom:6rpx;padding:4rpx 8rpx;font-size:15rpx}
.community-filter-options{height:auto;max-height:54vh;min-height:150rpx;padding:10rpx;box-sizing:border-box}.community-filter-grid{display:flex;align-content:flex-start;align-items:stretch;flex-wrap:wrap;gap:10rpx}.community-filter-option{flex:1 1 calc(50% - 10rpx);min-width:0;min-height:68rpx;padding:0 10rpx;border:1rpx solid #e0e8dc;border-radius:12rpx;box-sizing:border-box;background:#fbfcfa}.community-filter-option>text{font-size:18rpx}.community-filter-option>.community-filter-count{min-width:30rpx;padding:1rpx 7rpx;font-size:17rpx;line-height:32rpx}.community-filter-option.active{border-color:#40543f;background:#eef4eb}.community-filter-no-result{width:100%;box-sizing:border-box}

.community-filter-heading{display:flex;align-items:center;justify-content:space-between;gap:10rpx}.community-filter-heading>strong{color:#40543f;font-size:21rpx}.community-filter-heading>button,.community-picker-section-head>button{flex:none;margin:0;padding:0 12rpx;border:1rpx solid #d7e4d5;border-radius:10rpx;background:#f3f7ef;color:#40543f;font-size:18rpx;font-weight:700;line-height:44rpx}.community-filter-heading>button::after,.community-picker-section-head>button::after{border:0}.community-filter-row{margin-top:-4rpx}.community-picker-section-head{display:flex;align-items:center;justify-content:space-between;gap:10rpx;margin:4rpx 0 8rpx}.community-picker-section-head>strong{margin:0}.community-picker-section-head>button{padding:0 13rpx;background:#40543f;color:#fff;line-height:46rpx}
.community-filter-head{gap:8rpx}.community-filter-head>view{flex:1;min-width:0}.community-filter-head>button.community-filter-add{flex:none;margin:0;padding:0 11rpx;border:0;border-radius:10rpx;background:#40543f;color:#fff;font-size:17rpx;font-weight:700;line-height:46rpx}.community-filter-head>button.community-filter-add::after{border:0}.category-manager-heading .manager-add-category{flex:none;margin:0;padding:0 13rpx;border:0;border-radius:10rpx;background:#40543f;color:#fff;font-size:18rpx;font-weight:700;line-height:48rpx}.category-manager-heading .manager-add-category::after{border:0}
.community-category-actions{justify-content:flex-end}
.community-category-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:9rpx}.community-category-mini{display:grid;gap:7rpx;min-width:0;padding:12rpx;border:1rpx solid #e0e8dc;border-radius:13rpx;background:#fbfcfa}.community-category-mini>view{display:flex;align-items:center;justify-content:space-between;gap:5rpx;min-width:0}.community-category-mini strong{overflow:hidden;flex:1;color:#293a2d;font-size:19rpx;text-overflow:ellipsis;white-space:nowrap}.community-category-mini>view>text{flex:none;padding:3rpx 6rpx;border-radius:8rpx;background:#edf3e9;color:#53664f;font-size:15rpx;font-weight:700}.community-category-mini>.community-category-address-compact{display:block;overflow:hidden;color:#778274;font-size:15rpx;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.rename-category-mask{position:fixed;z-index:1001;inset:0;display:flex;align-items:center;justify-content:center;padding:32rpx;background:rgba(24,34,27,.48)}.rename-category-sheet{display:grid;gap:12rpx;width:100%;max-width:600rpx;padding:24rpx;border-radius:20rpx;background:#fdfdfa}.rename-category-sheet>strong{color:#26372c;font-size:25rpx}.rename-category-sheet>small{color:#788474;font-size:18rpx}.rename-category-sheet>input{height:68rpx;padding:0 14rpx;border:1rpx solid #dce5d9;border-radius:12rpx;background:#fff;color:#28382b;font-size:21rpx}.rename-category-sheet>view{display:flex;justify-content:flex-end;gap:10rpx;margin-top:4rpx}.rename-category-sheet button{margin:0;padding:0 16rpx;border:0;border-radius:10rpx;background:#edf2e8;color:#40543f;font-size:19rpx;line-height:52rpx}.rename-category-sheet button:last-child{background:#40543f;color:#fff}.rename-category-sheet button::after{border:0}
.category-action-mask{position:fixed;z-index:1000;inset:0;display:flex;align-items:flex-end;justify-content:center;background:rgba(24,34,27,.48)}.category-action-sheet{display:grid;gap:8rpx;width:100%;max-height:70vh;padding:18rpx 20rpx calc(20rpx + env(safe-area-inset-bottom));box-sizing:border-box;border-radius:24rpx 24rpx 0 0;background:#fdfdfa}.category-action-heading{display:flex;align-items:center;justify-content:space-between;gap:12rpx;padding:3rpx 2rpx 10rpx}.category-action-heading>view{display:grid;gap:5rpx;min-width:0}.category-action-heading strong{overflow:hidden;color:#26372c;font-size:23rpx;text-overflow:ellipsis;white-space:nowrap}.category-action-heading small{overflow:hidden;color:#788474;font-size:17rpx;text-overflow:ellipsis;white-space:nowrap}.category-action-heading>text{display:grid;place-items:center;flex:none;width:46rpx;height:46rpx;border-radius:50%;background:#edf2e8;color:#536650;font-size:32rpx}.category-action-sheet button{margin:0;padding:0 14rpx;border:1rpx solid #e1e8dd;border-radius:11rpx;background:#fff;color:#40543f;font-size:19rpx;text-align:left;line-height:54rpx}.category-action-sheet button::after{border:0}.category-action-sheet .category-action-remove{border-color:#f0ded9;background:#fff5f2;color:#a15144}
.category-action-sheet{gap:12rpx;padding:22rpx 20rpx calc(22rpx + env(safe-area-inset-bottom))}.category-action-heading{padding-bottom:8rpx}.category-action-grid{display:grid;grid-template-columns:1fr 1fr;gap:10rpx}.category-action-grid button{display:grid;grid-template-columns:42rpx minmax(0,1fr);align-items:center;gap:10rpx;min-height:76rpx;padding:10rpx 12rpx;border-radius:13rpx;line-height:1.2}.category-action-grid button>text{display:grid;place-items:center;width:40rpx;height:40rpx;border-radius:12rpx;background:#edf3e9;color:#40543f;font-size:24rpx;font-weight:700}.category-action-grid button>view{display:grid;gap:4rpx;min-width:0}.category-action-grid button strong{color:#344b38;font-size:19rpx;font-weight:700;white-space:nowrap}.category-action-grid button small{overflow:hidden;color:#849080;font-size:15rpx;text-overflow:ellipsis;white-space:nowrap}.category-action-sheet .category-action-remove{display:flex;align-items:center;justify-content:space-between;gap:12rpx;padding:0 14rpx;background:#fff7f4;color:#a15144;font-size:18rpx;text-align:left;line-height:48rpx}.category-action-sheet .category-action-remove small{color:#b57a70;font-size:15rpx}

.material-archive-row{display:flex;gap:10rpx;margin:2rpx 0 4rpx}.material-archive-row button{display:flex;align-items:center;gap:8rpx;margin:0;padding:0 16rpx;border:1rpx solid #dce5d9;border-radius:24rpx;background:#fff;color:#647160;font-size:19rpx;line-height:52rpx}.material-archive-row button::after{border:0}.material-archive-row button.active{border-color:#40543f;background:#40543f;color:#fff}.material-archive-row button text{min-width:28rpx;padding:0 7rpx;border-radius:18rpx;background:#edf2e8;color:#40543f;font-size:16rpx;line-height:30rpx;text-align:center}.material-archive-row button.active text{background:#fff;color:#40543f}.converted-note{margin-top:7rpx;color:#6e7e63;font-size:17rpx}
.community-select-control.fixed-category{border-color:#cbdcc6;background:#eef4ea}.community-select-control.fixed-category>text{color:#557452;font-size:19rpx}
.material-media-strip>image{width:100%;height:100%;object-fit:cover}.video-cover{display:grid;width:100%;height:100%;place-content:center;justify-items:center;gap:8rpx;background:linear-gradient(145deg,#dce3d7,#aebba9);color:#fff}.video-cover text{display:grid;width:54rpx;height:54rpx;place-items:center;border:2rpx solid #ffffffc9;border-radius:50%;background:#26372c88;font-size:23rpx}.video-cover small{padding:4rpx 9rpx;border-radius:8rpx;background:#26372c88;font-size:17rpx}
.material-list{gap:9rpx}.material-card{gap:9rpx;margin-bottom:0;padding:8rpx;border-radius:14rpx}.material-media-strip{width:142rpx;height:122rpx;border-radius:10rpx}.material-media-strip>image,.video-tile{border-radius:9rpx}.material-copy{padding:2rpx 1rpx}.title-line strong{font-size:21rpx;-webkit-line-clamp:1}.title-line text{max-width:94rpx;padding:3rpx 6rpx;font-size:14rpx}.material-tags{gap:4rpx;margin-top:5rpx}.material-tags text{padding:2rpx 6rpx;font-size:15rpx;line-height:1.2}.material-copy>small{margin-top:3rpx;font-size:15rpx;line-height:1.25}.optional-info{gap:6rpx;margin-top:3rpx;font-size:14rpx}.optional-info text{display:-webkit-box;overflow:hidden;-webkit-box-orient:vertical;-webkit-line-clamp:1}.card-actions{gap:6rpx;padding-top:5rpx}.card-actions button{padding:0 9rpx;font-size:15rpx;line-height:38rpx}.material-count{right:5rpx;bottom:5rpx;padding:3rpx 6rpx;font-size:13rpx}
.category-action-add>text{background:#40543f!important;color:#fff!important}
</style>

<style scoped>
.entry-locked { opacity: .75; pointer-events: none; }
</style>
