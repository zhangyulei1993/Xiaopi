<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { API_ORIGIN, resolveAssetUrl, safeUploadUrl } from "../../api/client";
import {
  convertMaterial,
  createMaterial,
  deleteMaterial,
  getMaterialCategories,
  getPublicConfig,
  listMaterialLibrary,
  replacePropertyMedia,
  saveMaterialCategories,
  type Property,
  type PropertyMedia,
} from "../../api/properties";
import { getSession, hasRole } from "../../auth/session";

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

type PendingMedia = { path: string; type: "image" | "video" };
const loading = ref(false);
const saving = ref(false);
const maxImageBytes = 10 * 1024 * 1024;
const maxVideoBytes = 40 * 1024 * 1024;
const materials = ref<Property[]>([]);
const categories = ref<string[]>([]);
const activeCategory = ref("全部");
const formOpen = ref(false);
const rentType = ref<"整租" | "合租">("整租");
const area = ref("");
const draftId = ref("");
const uploadedMedia = ref<PropertyMedia[]>([]);
const uploadProgress = ref("");
const categoryManagerOpen = ref(false);
const newCategory = ref("");
const title = ref("");
const layout = ref("");
const rentMonthly = ref("");
const description = ref("");
const category = ref("");
const pendingMedia = ref<PendingMedia[]>([]);

const filteredMaterials = computed(() => activeCategory.value === "全部"
  ? materials.value
  : materials.value.filter(item => (item.materialCategory || "待整理") === activeCategory.value));
const imageCount = computed(() => pendingMedia.value.filter(item => item.type === "image").length);
const hasVideo = computed(() => pendingMedia.value.some(item => item.type === "video"));
const mediaUrl = (url: string) => resolveAssetUrl(url);
const previewMaterialImages = (item: Property, current: PropertyMedia) => {
  const images = (item.media || []).filter(media => media.type === "image");
  uni.previewImage({ current: mediaUrl(current.url), urls: images.map(media => mediaUrl(media.url)) });
};
const refresh = async () => {
  loading.value = true;
  try {
    const [kinds, items, config] = await Promise.all([
      getMaterialCategories(), listMaterialLibrary(), getPublicConfig().catch(() => null),
    ]);
    if (config) tagConfig.value = config.listingTitle;
    materials.value = items;
    categories.value = kinds;
    if (!kinds.length) categoryManagerOpen.value = true;
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "素材库加载失败", icon: "none" });
  } finally {
    loading.value = false;
  }
};
onShow(async () => {
  if (!hasRole("admin")) {
    uni.showToast({ title: "仅管理员可使用素材库", icon: "none" });
    setTimeout(() => uni.navigateBack(), 250);
    return;
  }
  await refresh();
});

const addCategory = async () => {
  const value = newCategory.value.trim();
  if (!value || formOpen.value || saving.value || draftId.value) return;
  if (value === "全部") {
    uni.showToast({ title: "请填写小区名称", icon: "none" });
    return;
  }
  if (categories.value.some(item => normalizeCommunityName(item) === normalizeCommunityName(value))) {
    uni.showToast({ title: "分类已存在", icon: "none" });
    return;
  }
  try {
    categories.value = await saveMaterialCategories([...categories.value, value]);
    selectCategory(value);
    activeCategory.value = value;
    newCategory.value = "";
    uni.showToast({ title: "分类已添加" });
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "分类保存失败", icon: "none" });
  }
};
const removeCategory = (value: string) => {
  if (formOpen.value || saving.value || draftId.value) return;
  uni.showModal({
    title: "移除这个小区分类？",
    content: "为避免素材地址错配，请先删除该分类下的素材或将它们转为房源草稿。",
    success: async result => {
      if (!result.confirm) return;
      try {
        categories.value = await saveMaterialCategories(categories.value.filter(item => item !== value));
        if (activeCategory.value === value) activeCategory.value = "全部";
        if (category.value === value) selectCategory(categories.value[0] || "");
        await refresh();
      } catch (error) {
        uni.showToast({ title: error instanceof Error ? error.message : "分类删除失败", icon: "none" });
      }
    },
  });
};
const selectCategory = (value: string) => {
  category.value = value;
};
const chooseImages = () => {
  const remaining = Math.max(0, 9 - imageCount.value);
  if (!remaining) return;
  uni.chooseMedia({
    count: remaining,
    mediaType: ["image"],
    sourceType: ["album", "camera"],
    success: result => {
      const accepted = result.tempFiles.filter(file => file.size <= maxImageBytes);
      pendingMedia.value.push(...accepted.map(file => ({ path: file.tempFilePath, type: "image" as const })));
      if (accepted.length < result.tempFiles.length) uni.showToast({ title: "单张图片不能超过 10MB", icon: "none" });
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
      const file = result.tempFiles[0];
      if (!file) return;
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
const upload = (file: PendingMedia, propertyId: string, sort: number) => new Promise<PropertyMedia>((resolve, reject) => {
  uni.uploadFile({
    url: safeUploadUrl(`${API_ORIGIN}/api/v1/media/upload`),
    filePath: file.path,
    name: "file",
    header: { Authorization: `Bearer ${getSession()?.accessToken || ""}` },
    formData: { propertyId, mediaType: file.type, sort: String(sort) },
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
const resetForm = () => {
  title.value = "";
  layout.value = "";
  rentMonthly.value = "";
  description.value = "";
  pendingMedia.value = [];
  rentType.value = "整租";
  area.value = "";
  draftId.value = "";
  uploadedMedia.value = [];
};
const closeEntry = () => {
  if (saving.value || draftId.value) return;
  if (!pendingMedia.value.length && !title.value && !layout.value && !rentMonthly.value && !area.value && !description.value) {
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
  if (!categories.value.length) {
    categoryManagerOpen.value = true;
    return;
  }
  if (activeCategory.value === "全部") return;
  if (!draftId.value && !pendingMedia.value.length) selectCategory(activeCategory.value);
  formOpen.value = true;
};
const viewCategory = (value: string) => {
  if (formOpen.value) return;
  activeCategory.value = value;
};
const saveMaterial = async () => {
  if (saving.value) return;
  const community = category.value;
  if (!categories.value.includes(community) || !pendingMedia.value.length) return;
  saving.value = true;
  try {
    if (!draftId.value) {
      const created = await createMaterial({
        title: title.value.trim() || [community, layout.value.trim()].filter(Boolean).join(" · ").slice(0, 60),
        community,
        location: community,
        layout: rentType.value === "合租" ? "" : layout.value.trim(),
        rentMonthly: Math.max(0, Number(rentMonthly.value) || 0),
        rentType: rentType.value,
        areaSqm: Math.max(0, Number(area.value) || 0),
        ...(rentType.value === "合租" ? { wholeLayout: layout.value.trim() } : {}),
        description: description.value.trim(),
        materialCategory: community,
      });
      draftId.value = created.id;
    }
    for (let index = uploadedMedia.value.length; index < pendingMedia.value.length; index++) {
      uploadProgress.value = `上传 ${index + 1}/${pendingMedia.value.length}`;
      uploadedMedia.value.push(await upload(pendingMedia.value[index], draftId.value, index));
    }
    await replacePropertyMedia(draftId.value, uploadedMedia.value.map(item => item.id));
    activeCategory.value = community;
    resetForm();
    formOpen.value = false;
    await refresh();
    uni.showToast({ title: "素材已保存" });
  } catch (error) {
    uni.showModal({
      title: "保存未完成",
      content: error instanceof Error ? error.message : "请重试保存",
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
const convert = (item: Property) => {
  if (saving.value || item.id === draftId.value) return;
  uni.showModal({
    title: "转为房源草稿",
    content: "转换后移入房源管理的草稿，图片与视频随草稿保留。补齐发布信息后可发布。",
    confirmText: "转为草稿",
    success: async result => {
      if (!result.confirm) return;
      try {
        await convertMaterial(item.id);
        uni.navigateTo({ url: `/pages/admin/property-edit?id=${encodeURIComponent(item.id)}` });
      } catch (error) {
        uni.showToast({ title: error instanceof Error ? error.message : "转换失败", icon: "none" });
      }
    },
  });
};
</script>

<template>
  <view class="material-page">
    <view class="page-head"><view><text>仅管理员可见</text><strong>房源素材库</strong><small>按小区保存素材，需要发布时转为房源草稿。</small></view><button v-if="activeCategory !== '全部'" :disabled="saving || !!draftId" @click="formOpen ? closeEntry() : openEntry()">{{ formOpen ? "收起" : "＋ 新增素材" }}</button></view>

    <view class="summary-row"><view><strong>{{ materials.length }}</strong><text>素材</text></view><view><strong>{{ categories.length }}</strong><text>分类</text></view><view><strong>{{ materials.reduce((sum, item) => sum + (item.media?.length || 0), 0) }}</strong><text>媒体</text></view></view>

    <view v-if="formOpen" class="entry-card">
      <view class="card-heading"><strong>{{ category || "选择小区分类" }}</strong><text>新增素材</text></view>
      <view class="media-picker-row"><button :disabled="saving || !!draftId || imageCount >= 9" @click="chooseImages">＋ 图片 <small>{{ imageCount }}/9</small></button><button :disabled="saving || !!draftId || hasVideo" @click="chooseVideo">＋ 视频 <small>{{ hasVideo ? 1 : 0 }}/1</small></button><text>图片 ≤10MB，视频 ≤40MB / 60秒</text></view>
      <view v-if="pendingMedia.length" class="pending-media"><view v-for="(media, index) in pendingMedia" :key="media.path" class="pending-item"><image v-if="media.type === 'image'" :src="media.path" mode="aspectFill"/><view v-else class="video-pending">视频</view><button v-if="!saving && !draftId" @click="pendingMedia.splice(index, 1)">×</button></view></view>
      <view v-if="!draftId" class="material-basic">
        <view class="form-row">
          <text>出租方式</text>
          <view class="material-rent-types">
            <button :class="{ active: rentType === '整租' }" @click="rentType = '整租'">整租</button>
            <button :class="{ active: rentType === '合租' }" @click="rentType = '合租'">合租</button>
          </view>
        </view>
        <view class="form-pair"><label>{{ rentType === "合租" ? "整套户型" : "户型" }}<input v-model="layout" maxlength="32" placeholder="如：2室1厅" /></label><label>月租<input v-model="rentMonthly" type="number" placeholder="元/月" /></label></view>
        <view class="form-pair"><label>面积<input v-model="area" type="digit" placeholder="㎡" /></label><label>素材名称<input v-model="title" maxlength="60" placeholder="默认小区名" /></label></view>
        <view class="form-row"><text>备注（选填）</text><textarea v-model="description" maxlength="500" placeholder="记录房源特点" /></view>
      </view>
      <button class="save-button" :loading="saving" :disabled="saving || !pendingMedia.length" @click="saveMaterial">{{ uploadProgress || (saving ? "保存中…" : draftId ? "继续保存" : "保存素材") }}</button>
    </view>

    <view class="category-head"><scroll-view scroll-x class="category-scroll"><view class="category-row"><button :disabled="formOpen" :class="{ active: activeCategory === '全部' }" @click="viewCategory('全部')">全部 {{ materials.length }}</button><button v-for="kind in categories" :key="kind" :disabled="formOpen" :class="{ active: activeCategory === kind }" @click="viewCategory(kind)">{{ kind }} {{ materials.filter(item => (item.materialCategory || '待整理') === kind).length }}</button></view></scroll-view><button class="manage-categories" :disabled="formOpen" @click="categoryManagerOpen = !categoryManagerOpen">分类管理</button></view>
    <view v-if="categoryManagerOpen && !formOpen" class="category-manager"><view class="category-manager-title"><strong>按小区建立分类</strong><small>一个分类对应一个小区，分类内素材共用该小区地址。</small></view><view v-for="kind in categories" :key="kind" class="category-chip"><text>{{ kind }}</text><button @click="removeCategory(kind)">×</button></view><view class="add-category"><input v-model="newCategory" maxlength="60" placeholder="输入小区名称" @confirm="addCategory"/><button @click="addCategory">添加小区</button></view></view>

    <checkbox-group class="material-price-switch" @change="togglePrice($event.detail.value)">
      <label><checkbox value="price" :checked="showPrice" color="#40543f" />显示价格</label>
    </checkbox-group>
    <view v-if="loading" class="empty-state">正在加载素材…</view>
    <view v-else-if="!filteredMaterials.length" class="empty-state"><strong>这里还没有素材</strong><text>看到值得留存的房源时，先把图片和视频快速存进来。</text><button v-if="activeCategory !== '全部'" @click="openEntry">＋ 新增素材</button><text v-else>选择小区分类后即可收存。</text></view>
    <view v-else class="material-list">
      <view v-for="item in filteredMaterials" :key="item.id" class="material-card">
        <view class="material-media-strip"><template v-for="media in (item.media || []).slice(0, 4)" :key="media.id"><image v-if="media.type === 'image'" :src="mediaUrl(media.url)" mode="aspectFill" @click="previewMaterialImages(item, media)"/><view v-else class="video-tile"><video :src="mediaUrl(media.url)" controls object-fit="cover"/><text>视频</text></view></template><view v-if="!item.media?.length" class="no-media">暂无图片或视频</view><view class="material-count">{{ item.media?.filter(media => media.type === 'image').length || 0 }}图 · {{ item.media?.filter(media => media.type === 'video').length || 0 }}视频</view></view>
        <view class="material-copy"><view class="title-line"><strong>{{ item.title || item.community }}</strong><text>{{ item.materialCategory || item.community || "小区待补" }}</text></view><view class="material-tags"><text v-for="tag in materialTags(item)" :key="tag">{{ tag }}</text><text v-if="showPrice" class="material-price-tag">{{ item.rentMonthly }} 元/月</text></view><view v-if="item.description" class="optional-info"><text>{{ item.description }}</text></view><view class="card-actions"><button class="convert-button" @click="convert(item)">转为房源草稿</button><button class="delete-button" @click="removeMaterial(item)">删除</button></view></view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.material-page{min-height:100vh;padding:28rpx 24rpx 80rpx;background:#f7f6f1;color:#26372c;box-sizing:border-box}.page-head{display:flex;align-items:center;justify-content:space-between;gap:16rpx;margin:8rpx 2rpx 24rpx}.page-head>view{display:flex;flex-direction:column;gap:8rpx}.page-head text{color:#72816e;font-size:21rpx;font-weight:700}.page-head strong{font-size:42rpx;font-weight:800}.page-head small{color:#7c877a;font-size:23rpx}.page-head button,.manage-categories,.add-category button{margin:0;padding:0 22rpx;border:0;border-radius:14rpx;background:#40543f;color:#fff;font-size:23rpx;font-weight:700;line-height:72rpx;white-space:nowrap}.page-head button::after,.manage-categories::after,.add-category button::after{border:0}.summary-row{display:grid;grid-template-columns:repeat(3,1fr);gap:12rpx;margin-bottom:20rpx}.summary-row>view{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:4rpx;padding:20rpx;border-radius:18rpx;background:#fff;text-align:center}.summary-row strong{font-size:32rpx;color:#213a5e;text-align:center}.summary-row text{font-size:21rpx;color:#7d877c;text-align:center}.entry-card,.category-manager,.material-card{margin-bottom:20rpx;padding:22rpx;border:1rpx solid #e1e8dd;border-radius:20rpx;background:#fff}.card-heading{display:flex;align-items:baseline;justify-content:space-between;margin-bottom:18rpx}.card-heading strong{font-size:30rpx}.card-heading text{font-size:20rpx;color:#7e897c}.form-row{display:grid;gap:10rpx;margin:14rpx 0}.form-row>text,.form-pair label{color:#54634f;font-size:23rpx;font-weight:650}.form-row input,.form-row textarea,.form-pair input,.form-pair picker view,.add-category input{width:100%;min-height:76rpx;padding:0 18rpx;border:1rpx solid #dce5d9;border-radius:13rpx;box-sizing:border-box;background:#fbfcfa;color:#28382b;font-size:24rpx}.form-row textarea{height:120rpx;padding-top:14rpx}.form-pair{display:grid;grid-template-columns:1fr 1fr;gap:14rpx;margin:14rpx 0}.form-pair label{display:grid;gap:9rpx}.form-pair picker view{display:flex;align-items:center}.media-picker-row{display:flex;align-items:center;gap:10rpx;flex-wrap:wrap;margin:18rpx 0}.media-picker-row button{margin:0;padding:0 18rpx;border:1rpx solid #d7e4d5;border-radius:12rpx;background:#f3f7ef;color:#40543f;font-size:22rpx;line-height:64rpx}.media-picker-row button::after{border:0}.media-picker-row small{margin-left:5rpx}.media-picker-row>text{color:#858f82;font-size:20rpx}.pending-media{display:flex;gap:10rpx;flex-wrap:wrap;margin:14rpx 0}.pending-item{position:relative;width:126rpx;height:126rpx}.pending-item image,.video-pending{width:100%;height:100%;border-radius:12rpx}.video-pending{display:grid;place-items:center;background:#e9eee4;color:#40543f}.pending-item button{position:absolute;right:-8rpx;top:-8rpx;width:38rpx;height:38rpx;padding:0;border-radius:50%;background:#26372c;color:white;line-height:38rpx}.pending-item button::after{border:0}.save-button{margin:18rpx 0 0;border:0;border-radius:14rpx;background:#40543f;color:#fff;font-weight:700;line-height:78rpx}.save-button::after{border:0}.category-head{display:flex;align-items:center;gap:8rpx;margin:22rpx 0 14rpx}.category-scroll{flex:1;min-width:0;white-space:nowrap}.category-row{display:flex;gap:10rpx}.category-row button{flex:none;margin:0;padding:0 18rpx;border:1rpx solid #dce5d9;border-radius:30rpx;background:#fff;color:#596856;font-size:21rpx;line-height:62rpx}.category-row button.active{border-color:#40543f;background:#40543f;color:#fff}.category-row button::after{border:0}.manage-categories{padding:0 14rpx;background:#edf2e8;color:#53644e;font-size:19rpx;line-height:58rpx}.category-manager{display:flex;align-items:center;gap:10rpx;flex-wrap:wrap}.category-chip{display:flex;align-items:center;gap:4rpx;padding:4rpx 8rpx 4rpx 14rpx;border-radius:24rpx;background:#f1f5ed;color:#40543f;font-size:21rpx}.category-chip button{width:36rpx;height:36rpx;margin:0;padding:0;border:0;border-radius:50%;background:#dfe9d9;color:#40543f;font-size:26rpx;line-height:36rpx}.category-chip button::after{border:0}.add-category{display:flex;gap:8rpx;flex:1;min-width:250rpx}.add-category input{min-width:0;min-height:58rpx;font-size:21rpx}.add-category button{line-height:58rpx;font-size:20rpx}.material-list{display:grid;gap:16rpx}.material-card{padding:0;overflow:hidden}.material-media-strip{position:relative;display:flex;height:220rpx;overflow:hidden;background:#edf0e9}.material-media-strip>image,.video-tile{flex:1;min-width:0;height:100%;object-fit:cover}.video-tile{position:relative}.video-tile video{width:100%;height:100%}.video-tile text{position:absolute;left:10rpx;top:10rpx;padding:4rpx 10rpx;border-radius:10rpx;background:#26372ccc;color:#fff;font-size:18rpx}.no-media{display:grid;place-items:center;width:100%;color:#8b9587;font-size:22rpx}.material-count{position:absolute;right:12rpx;bottom:12rpx;padding:7rpx 12rpx;border-radius:20rpx;background:#26372ccc;color:#fff;font-size:18rpx}.material-copy{padding:18rpx}.title-line{display:flex;align-items:center;justify-content:space-between;gap:12rpx}.title-line strong{font-size:28rpx}.title-line text{flex:none;padding:6rpx 12rpx;border-radius:18rpx;background:#eef3e9;color:#52634c;font-size:19rpx}.material-copy>small{display:block;margin-top:8rpx;color:#737f71;font-size:21rpx;line-height:1.5}.optional-info{display:flex;gap:10rpx;flex-wrap:wrap;margin-top:10rpx;color:#667363;font-size:20rpx}.card-actions{display:flex;gap:10rpx;margin-top:16rpx}.card-actions button{margin:0;padding:0 18rpx;border:0;border-radius:11rpx;font-size:21rpx;font-weight:650;line-height:62rpx}.card-actions button::after{border:0}.convert-button{background:#40543f;color:#fff}.delete-button{background:#fbefed;color:#a24d40}.empty-state{display:grid;justify-items:center;gap:12rpx;padding:90rpx 32rpx;color:#778273;text-align:center}.empty-state strong{font-size:28rpx;color:#40543f}.empty-state text{font-size:22rpx;line-height:1.5}.empty-state button{margin:8rpx 0 0;padding:0 22rpx;border:0;border-radius:12rpx;background:#40543f;color:#fff;font-size:22rpx;line-height:68rpx}.empty-state button::after{border:0}
.community-picker{display:flex;align-items:center;width:100%;min-height:76rpx;padding:0 18rpx;border:1rpx solid #dce5d9;border-radius:13rpx;box-sizing:border-box;background:#fbfcfa;color:#28382b;font-size:24rpx}.empty-picker{color:#9a7250;background:#fcf8ef}.field-hint{color:#7f8b7b;font-size:19rpx;line-height:1.4}.category-manager-title{display:grid;gap:5rpx;width:100%}.category-manager-title strong{font-size:24rpx;color:#40543f}.category-manager-title small{font-size:19rpx;color:#7f8b7b;line-height:1.4}
.material-basic { display:grid;gap:14rpx; }
.material-rent-types,.material-tags { display:flex;flex-wrap:wrap;gap:8rpx; }
.material-rent-types button { margin:0;padding:0 22rpx;line-height:60rpx;font-size:23rpx;background:#f0f4ec;color:#40543f; }
.material-rent-types button.active { background:#40543f;color:#fff; }
.material-tags text { padding:4rpx 9rpx;border-radius:8rpx;background:#eef3e9;color:#40543f;font-size:20rpx; }
.material-tags .material-price-tag { background:#fff0e9;color:#a64d32; }
.material-price-switch { display:flex;justify-content:flex-end;margin:12rpx 0;color:#40543f;font-size:22rpx; }
</style>
