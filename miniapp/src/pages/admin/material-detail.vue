<script setup lang="ts">
import DetailMediaGallery from "../../components/DetailMediaGallery.vue";
import { onLoad, onPageScroll, onShow, onHide } from "@dcloudio/uni-app";
import { computed, onBeforeUnmount, ref } from "vue";
import { getMaterialCategories, listMaterialLibrary, listProperties, updateProperty, type Property, type PropertyMedia } from "../../api/properties";
import { ApiError, request, resolveAssetUrl, safeDownloadUrl } from "../../api/client";
import { getSession, hasRole, sessionAccount } from "../../auth/session";
import { refreshCurrentSession, SessionPermissionChanged } from "../../auth/refresh-session";

const item = ref<Property | null>(null);
let pageID = "";
let ownerAccount = "";
let permissionSequence = 0;
let loadSequence = 0;
const invalidatePage = () => {
  permissionSequence++;
  loadSequence++;
  pauseVideoAt(activeMediaIndex.value);
  item.value = null;
  editing.value = false;
  failed.value = true;
  loading.value = false;
};

const loading = ref(true);
const failed = ref(false);
const permissionError = ref("");
const converted = ref(false);
const busyMedia = ref(false);
const media = computed(() => item.value?.media || []);
const mediaItems = computed(() => [...media.value.filter(entry => entry.type === "video"), ...media.value.filter(entry => entry.type === "image")]);
const images = computed(() => mediaItems.value.filter(entry => entry.type === "image"));
const activeMediaIndex = ref(0);
const playingVideoIndex = ref(-1);
const videoDimensions = ref<Record<number, { width: number; height: number }>>({});
const statusBarHeight = ref(0);
const showScrolledTitle = ref(false);
const showMediaFullscreenHint = ref(true);
let mediaHintTimer: ReturnType<typeof setTimeout> | undefined;
const showFullscreenHint = () => {
  showMediaFullscreenHint.value = true;
  if (mediaHintTimer) clearTimeout(mediaHintTimer);
  mediaHintTimer = setTimeout(() => { showMediaFullscreenHint.value = false; }, 4000);
};
onBeforeUnmount(() => { if (mediaHintTimer) clearTimeout(mediaHintTimer); });
const mediaViewportHeight = 720;
const imageMediaIndices = computed(() => mediaItems.value.map((entry, index) => entry.type === "image" ? index : -1).filter(index => index >= 0));
const videoMediaIndex = computed(() => mediaItems.value.findIndex(entry => entry.type === "video"));
const activeImageNumber = computed(() => Math.max(1, imageMediaIndices.value.indexOf(activeMediaIndex.value) + 1));
const activeVideoIsPortrait = computed(() => {
  const size = videoDimensions.value[activeMediaIndex.value];
  return mediaItems.value[activeMediaIndex.value]?.type === "video" && Boolean(size?.height && size.height > size.width);
});
const videoPoster = computed(() => {
  const image = images.value[0];
  return image ? mediaUrl(image.url) : "";
});
const facilities = computed(() => {
  const match = String(item.value?.description || "").match(/配套设施：([^\n]*)/u);
  return match?.[1]?.split("、").map(name => name.trim()).filter(Boolean) || [];
});
const mediaUrl = (url: string) => resolveAssetUrl(url);
const back = () => uni.navigateBack({ fail: () => uni.redirectTo({ url: "/pages/admin/material-library" }) });
const pauseVideoAt = (index: number) => {
  if (mediaItems.value[index]?.type === "video") uni.createVideoContext(`detail-video-${index}`).pause();
};
const handleMediaChange = (event: { detail: { current: number } }) => {
  if (event.detail.current !== activeMediaIndex.value) pauseVideoAt(activeMediaIndex.value);
  activeMediaIndex.value = event.detail.current;
  playingVideoIndex.value = -1;
  showMediaFullscreenHint.value = mediaItems.value[event.detail.current]?.type !== "video";
  if (mediaHintTimer) clearTimeout(mediaHintTimer);
  if (showMediaFullscreenHint.value) showFullscreenHint();
};
const selectMedia = (index: number) => {
  if (index !== activeMediaIndex.value) pauseVideoAt(activeMediaIndex.value);
  activeMediaIndex.value = index;
  playingVideoIndex.value = -1;
  showMediaFullscreenHint.value = mediaItems.value[index]?.type !== "video";
  if (mediaHintTimer) clearTimeout(mediaHintTimer);
  if (showMediaFullscreenHint.value) showFullscreenHint();
};
const readVideoMetadata = (index: number, event: { detail?: { width?: number; height?: number } }) => {
  const width = Number(event.detail?.width);
  const height = Number(event.detail?.height);
  if (width && height) videoDimensions.value = { ...videoDimensions.value, [index]: { width, height } };
};
const load = async (id: string) => {
  if (!hasRole("admin")) {
    uni.showToast({ title: "仅管理员可查看", icon: "none" });
    setTimeout(back, 300);
    return;
  }
  loading.value = true;
  failed.value = false;
  try {
    const account = sessionAccount();
    const sequence = ++loadSequence;
    const items = await listMaterialLibrary();
    if (!hasRole("admin") || sessionAccount() !== account || sequence !== loadSequence) return;
    item.value = items.find(entry => entry.id === id) || null;
    failed.value = !item.value;
    const linkedID = item.value?.convertedPropertyId;
    converted.value = Boolean(linkedID);
    if (linkedID) {
      void listProperties().then(properties => {
        if (item.value?.convertedPropertyId === linkedID) converted.value = properties.some(property => property.id === linkedID);
      }).catch(() => undefined);
    }
    if (item.value) void prepareCategories();
    if (item.value && mediaItems.value[0]?.type === "image") showFullscreenHint();
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
};
const editing = ref(false);
const saving = ref(false);
const categories = ref<string[]>([]);
const draft = ref({ title: "", layout: "", wholeLayout: "", roomSequence: "", floor: "", rent: "", area: "", facilities: "" });

const openEdit = () => {
  const source = item.value;
  if (!source || saving.value) return;
  draft.value = {
    title: source.title || source.community,
    layout: source.layout || "",
    wholeLayout: source.wholeLayout || "",
    roomSequence: source.roomSequence ? String(source.roomSequence) : "",
    floor: source.floorInfo || "",
    rent: String(source.rentMonthly ?? ""),
    area: source.areaSqm ? String(source.areaSqm) : "",
    facilities: facilities.value.join("、"),
  };
  editing.value = true;
  uni.nextTick(() => uni.pageScrollTo({ selector: ".material-edit", duration: 200 }));
};

const prepareCategories = async () => {
  try {
    categories.value = await getMaterialCategories();
  } catch {
    uni.showToast({ title: "分类暂时无法读取", icon: "none" });
  }
};

const saveChanges = async (changes: Partial<Property>, message: string) => {
  const source = item.value;
  if (!source || saving.value) return;
  saving.value = true;
  try {
    if (!ownerAccount || sessionAccount() !== ownerAccount) throw new Error("账号已变化");
    await refreshCurrentSession("admin");
    const updated = await updateProperty({ ...source, ...changes, isMaterial: true });
    item.value = { ...source, ...updated, media: source.media };
    editing.value = false;
    uni.showToast({ title: message });
  } catch (error) {
    if ((error as { statusCode?: number })?.statusCode === 409) {
      uni.showModal({
        title: "素材已更新",
        content: "重新加载会放弃当前未保存的更改。",
        confirmText: "重新加载",
        success: ({ confirm }) => {
          if (!confirm) return;
          editing.value = false;
          void load(source.id);
        },
      });
    } else uni.showToast({ title: error instanceof Error ? error.message : "保存失败", icon: "none" });
  } finally {
    saving.value = false;
  }
};

const saveEdit = () => {
  const source = item.value;
  if (!source) return;
  const fields = draft.value;
  const roomSequence = Number(fields.roomSequence || 0);
  if (source.rentType === "合租" && roomSequence !== 0) {
    const match = fields.wholeLayout.match(/([0-9]+|[一二两三四五六七八九])\s*室/u);
    const chineseRooms: Record<string, number> = { 一: 1, 二: 2, 两: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9 };
    const roomCount = match ? Number(match[1]) || chineseRooms[match[1]] || 0 : 0;
    if (!Number.isInteger(roomSequence) || roomSequence < 1 || roomSequence > Math.min(roomCount, 9)) {
      uni.showToast({ title: "房间序号不能超过户型室数", icon: "none" });
      return;
    }
  }
  const facilityLine = /^配套设施[ \t]*[：:][ \t]*[^\n]*(?:\n|$)/m;
  const notes = (source.description || "").replace(facilityLine, "").trim();
  const description = [fields.facilities.trim() ? `配套设施：${fields.facilities.trim()}` : "", notes].filter(Boolean).join("\n");
  void saveChanges({
    title: fields.title.trim() || source.community,
    layout: fields.layout.trim(),
    ...(source.rentType === "合租" ? { wholeLayout: fields.wholeLayout.trim(), roomSequence } : {}),
    floorInfo: fields.floor.trim(),
    rentMonthly: Math.max(0, Number(fields.rent) || 0),
    areaSqm: Math.max(0, Number(fields.area) || 0),
    description,
  }, "素材已更新");
};

const moveCategory = async (index: number) => {
  const source = item.value;
  const community = categories.value[index];
  if (!source || saving.value || !community || community === (source.materialCategory || source.community)) return;
  saving.value = true;
  let target: { address: string; latitude: number; longitude: number } | undefined;
  try {
    const locations = await request<Record<string, { address: string; latitude: number; longitude: number }>>("/v1/admin/community-locations");
    target = locations[community];
  } catch {
    uni.showToast({ title: "读取小区位置失败，请重试", icon: "none" });
    return;
  } finally {
    saving.value = false;
  }
  await saveChanges({
    community, materialCategory: community,
    title: source.title === source.community ? community : source.title,
    location: target?.address || community,
    latitude: target?.latitude || 0, longitude: target?.longitude || 0,
    areaLabel: "", buildingNo: "", unitNo: "", roomNo: "",
  }, "已移动分类");
};

const previewImages = (current: PropertyMedia) => {
  uni.previewImage({ current: mediaUrl(current.url), urls: images.value.map(entry => mediaUrl(entry.url)) });
};
const saveMedia = async (entry: PropertyMedia) => {
  if (busyMedia.value) return;
  busyMedia.value = true;
  try {
    if (!hasRole("admin") || !ownerAccount || sessionAccount() !== ownerAccount) throw new Error("账号已变化");
    await refreshCurrentSession("admin");
  } catch (error) {
    busyMedia.value = false;
    if (!hasRole("admin") || sessionAccount() !== ownerAccount || (error instanceof ApiError && [401,403].includes(error.statusCode))) invalidatePage();
    else permissionError.value = "权限暂时无法核验，请重试";
    uni.showToast({title:"暂时无法下载，请重试",icon:"none"});
    return;
  }
  busyMedia.value = true;
  uni.showLoading({ title: "正在准备保存", mask: true });
  try {
    uni.downloadFile({
      url: safeDownloadUrl(entry.url),
      success: result => {
        if (!hasRole("admin") || sessionAccount() !== ownerAccount) { invalidatePage(); return; }
        if (result.statusCode !== 200 || !result.tempFilePath) {
          uni.showToast({ title: "下载失败，请检查网络", icon: "none" });
          return;
        }
        const save = entry.type === "video" ? uni.saveVideoToPhotosAlbum : uni.saveImageToPhotosAlbum;
        save({
          filePath: result.tempFilePath,
          success: () => uni.showToast({ title: "已保存到相册" }),
          fail: () => uni.showModal({ title: "保存未完成", content: "请检查微信的相册访问权限后重试。", showCancel: false }),
        });
      },
      fail: () => uni.showToast({ title: "下载失败，请检查网络", icon: "none" }),
      complete: () => { busyMedia.value = false; uni.hideLoading(); },
    });
  } catch {
    busyMedia.value = false;
    uni.hideLoading();
    uni.showToast({ title: "此媒体地址无法下载", icon: "none" });
  }
};
const showMediaActions = (entry: PropertyMedia) => {
  uni.showActionSheet({ itemList: [entry.type === "video" ? "保存视频到相册" : "保存图片到相册"], success: () => saveMedia(entry) });
};
onLoad(query => {
  statusBarHeight.value = Number(uni.getSystemInfoSync().statusBarHeight) || 0;
  pageID = String(query?.id || "");
});
onHide(() => { permissionSequence++; });
const checkPermission = async () => {
  permissionError.value = "";
  const sequence = ++permissionSequence;
  if (!hasRole("admin") || (ownerAccount && sessionAccount() !== ownerAccount)) {
    invalidatePage();
    return;
  }
  try {
    await refreshCurrentSession("admin");
    if (sequence !== permissionSequence) return;
    ownerAccount = sessionAccount();
    if (!item.value) await load(pageID);
  } catch (error) {
    if (sequence === permissionSequence) {
      if (error instanceof SessionPermissionChanged || (error instanceof ApiError && [401,403].includes(error.statusCode))) invalidatePage();
      else permissionError.value = "权限暂时无法核验，请重试";
    }
  }
};
onShow(checkPermission);
onPageScroll(({ scrollTop }) => { showScrolledTitle.value = scrollTop > Math.max(80, statusBarHeight.value + 38); });
</script>
<template>
  <view class="material-detail-page">
    <view v-if="permissionError"><text>{{ permissionError }}</text><button @click="checkPermission">重试</button></view>
    <view v-if="loading" class="detail-state">正在加载素材…</view>
    <view v-else-if="failed || !item" class="detail-state"><strong>素材暂时无法打开</strong><button @click="back">返回素材库</button></view>
    <template v-else>
      <cover-view v-if="showScrolledTitle" class="detail-scrolled-header" :style="{ height: `${statusBarHeight + 44}px` }">
      <cover-view class="detail-scrolled-back" @tap.stop="back">‹</cover-view>
      <cover-view class="detail-scrolled-title">素材详情</cover-view>
    </cover-view>
    <cover-view v-if="!showScrolledTitle" class="detail-media-back" @tap.stop="back"><cover-view class="detail-back-glyph">‹</cover-view></cover-view>
      <DetailMediaGallery v-if="mediaItems.length" :media-items="mediaItems" :active-media-index="activeMediaIndex" :playing-video-index="playingVideoIndex" :active-video-is-portrait="activeVideoIsPortrait" :video-poster="videoPoster" :show-media-fullscreen-hint="showMediaFullscreenHint" :image-media-indices="imageMediaIndices" :video-media-index="videoMediaIndex" :active-image-number="activeImageNumber" :is-admin="true" @image-click="url => previewImages(images.find(image => image.url === url)!)" @image-longpress="index => showMediaActions(mediaItems[index])" @video-action="index => showMediaActions(mediaItems[index])" @change="handleMediaChange" @select="selectMedia" @metadata="readVideoMetadata" @play="index => { playingVideoIndex = index; showFullscreenHint(); }" @stop="playingVideoIndex = -1" />
      <view v-else class="detail-cover-empty"><text>暂无实拍图</text><small>素材暂无图片或视频</small></view>
      <view class="detail-card overview-card">
        <view v-if="converted" class="admin-preview-tag">已转房源 · 素材保留</view>
        <view class="price-line"><strong>{{ Number(item.rentMonthly) > 0 ? `¥${item.rentMonthly}` : "价格待补" }}</strong><small v-if="Number(item.rentMonthly) > 0">元/月</small></view>
        <view class="overview-title">{{ item.title || item.community }}</view>
        <view class="overview-address">{{ item.location && item.location !== item.community ? `${item.location} · ${item.community}` : item.community }}</view>
        <view class="key-facts"><view><small>面积</small><strong>{{ item.areaSqm ? `${item.areaSqm}㎡` : "待补充" }}</strong></view><view><small>{{ item.rentType === '合租' ? '整套户型' : '户型' }}</small><strong>{{ item.rentType === '合租' ? item.wholeLayout || '待补充' : item.layout || '待补充' }}</strong></view><view><small>楼层</small><strong>{{ item.floorInfo || '待补充' }}</strong></view><view><small>租赁方式</small><strong>{{ item.rentType || '待补充' }}</strong></view><view v-if="item.rentType === '合租' && item.roomSequence"><small>房间序号</small><strong>{{ item.roomSequence }}号房</strong></view></view>
      </view>
      <view v-if="facilities.length" class="detail-card"><strong>房屋设施</strong><view class="tag-list facilities"><text v-for="name in facilities" :key="name">{{ name }}</text></view></view>
      <view class="detail-card"><strong>素材信息</strong><view class="info-line"><text>分类</text><text>{{ item.materialCategory || item.community }}</text></view><view class="info-line"><text>媒体</text><text>{{ images.length }} 张图片 · {{ media.length - images.length }} 个视频</text></view><view v-if="converted" class="converted-message">该素材已转换为房源，素材仍保留在素材库中。</view></view>
      <view class="detail-card material-actions">
        <button :disabled="saving" @click="openEdit">编辑素材</button>
        <picker v-if="categories.length > 1" :range="categories" :disabled="saving || editing" @change="moveCategory(Number($event.detail.value))"><button :disabled="saving || editing">移动分类</button></picker>
      </view>
      <view v-if="editing" class="detail-card material-edit">
        <strong>编辑素材</strong>
        <label>素材名称<input v-model="draft.title" maxlength="60" /></label>
        <label v-if="item.rentType === '合租'">整套户型<input v-model="draft.wholeLayout" placeholder="如 2室1厅" /></label>
        <label>{{ item.rentType === "合租" ? "出租房型" : "户型" }}<input v-model="draft.layout" /></label>
        <label v-if="item.rentType === '合租'">房间序号<input v-model="draft.roomSequence" type="number" maxlength="1" placeholder="选填，1–9" /></label>
        <label>楼层<input v-model="draft.floor" /></label>
        <label>月租<input v-model="draft.rent" type="number" /></label>
        <label>面积（㎡）<input v-model="draft.area" type="digit" /></label>
        <label>房屋设施<input v-model="draft.facilities" placeholder="用顿号分隔" /></label>
        <view class="material-actions"><button :disabled="saving" @click="editing = false">取消</button><button :loading="saving" :disabled="saving" @click="saveEdit">保存素材</button></view>
      </view>
    </template>
  </view>
</template>
<style scoped>
.material-actions{display:flex;gap:16rpx;align-items:center}
.material-actions button{margin:0;padding:0 24rpx;font-size:26rpx;color:#40543f;background:#edf2e8}
.material-edit{display:grid;gap:18rpx}
.material-edit label{display:grid;gap:8rpx;font-size:24rpx}
.material-edit input{min-height:64rpx;padding:0 16rpx;border:1rpx solid #dce5d9;border-radius:12rpx;background:#fbfcfa}

.overview-title{margin-top:18rpx;color:#263227;font-size:37rpx;font-weight:750;line-height:1.34}.overview-address{margin-top:10rpx;color:#7a8378;font-size:23rpx;line-height:1.5}.admin-preview-tag{display:inline-block;margin-bottom:14rpx;padding:7rpx 12rpx;border-radius:8rpx;background:#f8eadf;color:#b55b37;font-size:20rpx}.key-facts{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:0 34rpx;margin-top:22rpx}.key-facts>view{display:flex;align-items:baseline;gap:16rpx;min-width:0;min-height:56rpx;padding:9rpx 0}.key-facts small{flex:none;color:#939b93;font-size:20rpx}.key-facts strong{color:#334033;font-size:24rpx;font-weight:400;line-height:1.45}
.material-detail-page{position:relative;min-height:100vh;padding:0 24rpx calc(250rpx + env(safe-area-inset-bottom));box-sizing:border-box;background:#f6f4ee;color:#29382d}
.media-gallery{position:relative;overflow:hidden;margin:0 -24rpx;border-radius:0;background:#dce4d5}
.media-swiper{width:100%;height:720rpx}
.media-gallery .media-image,.media-gallery .media-video,.video-stage{display:block;width:100%;height:100%;border-radius:0}
.media-image,.media-video{background:#e4e7de}.video-stage{position:relative;background:#40543f}
.media-fullscreen-hint{position:absolute;z-index:4;right:76rpx;bottom:calc(3px + env(safe-area-inset-bottom));display:flex;align-items:center;gap:7rpx;padding:10rpx 14rpx;border:1rpx solid rgba(255,255,255,.34);border-radius:999rpx;background:rgba(18,25,19,.56);box-shadow:0 3rpx 12rpx rgba(0,0,0,.14);color:rgba(255,255,255,.94);font-size:20rpx;white-space:nowrap;pointer-events:none}
.media-fullscreen-hint.image-hint{top:50%;right:auto;bottom:auto;left:50%;padding:13rpx 18rpx;transform:translate(-50%,-50%);font-size:22rpx}.hint-arrow{font-size:22rpx}
.media-strip{position:absolute;z-index:3;right:18rpx;bottom:calc(16px + 1px);left:18rpx;width:auto;margin:0;white-space:nowrap;transition:opacity .18s ease}.media-strip-row{display:inline-flex;gap:8rpx;padding:0}
.media-thumb{position:relative;display:flex;width:auto;min-width:66rpx;height:48rpx;align-items:center;justify-content:center;padding:0 15rpx;overflow:hidden;border:1rpx solid rgba(255,255,255,.58);border-radius:8rpx;background:rgba(27,38,29,.48);box-shadow:0 3rpx 10rpx rgba(0,0,0,.2);box-sizing:border-box}.media-thumb.video{min-width:66rpx;background:rgba(27,38,29,.62)}.media-thumb .media-thumb-label{display:block;padding:0;background:transparent;color:rgba(255,255,255,.96);font-size:17rpx;font-weight:600;line-height:1;white-space:nowrap}.media-thumb.active{border-color:rgba(255,255,255,.96);background:rgba(55,77,56,.82);box-shadow:0 0 0 2rpx rgba(255,255,255,.35),0 3rpx 10rpx rgba(0,0,0,.2)}
.media-gallery.video-active .media-strip{bottom:calc(104rpx - 20px + 1px)}.media-gallery.video-playing .media-strip{opacity:0;pointer-events:none}
.video-actions-button{position:absolute;z-index:5;right:20rpx;top:calc(env(safe-area-inset-top) + 90rpx);padding:10rpx 18rpx;border:1rpx solid rgba(255,255,255,.6);border-radius:999rpx;background:rgba(24,34,25,.68);color:#fff;font-size:22rpx;line-height:1.4}
.detail-media-back{position:absolute;z-index:6;top:calc(env(safe-area-inset-top) + 16rpx);left:22rpx;display:grid;width:88rpx;height:88rpx;place-items:center;border:1rpx solid rgba(255,255,255,.56);border-radius:50%;background:rgba(23,31,24,.38);box-shadow:0 3rpx 12rpx rgba(0,0,0,.16)}.detail-media-back .detail-back-glyph{color:#fff;font-size:54rpx;font-weight:300;line-height:88rpx;text-align:center}
.detail-scrolled-header{position:fixed;z-index:24;top:0;right:0;left:0;box-sizing:content-box;height:36px;background:rgba(255,253,249,.98);box-shadow:0 1px 8px rgba(40,52,39,.08)}.detail-scrolled-back{position:absolute;bottom:0;left:14px;display:block;width:44px;height:44px;text-align:center;font-size:34px;line-height:44px;color:#40543f}.detail-scrolled-back text{font-size:34px;font-weight:300;line-height:1}.detail-scrolled-title{position:absolute;right:98px;bottom:0;left:98px;overflow:hidden;height:44px;color:#29382d;font-size:15px;font-weight:700;line-height:44px;text-align:center;text-overflow:ellipsis;white-space:nowrap}
.detail-cover-empty{display:flex;height:720rpx;align-items:center;justify-content:center;flex-direction:column;gap:9rpx;margin:0 -24rpx;background:linear-gradient(145deg,#e8e2d5,#d7dfcd);color:#465440;font-size:31rpx;font-weight:650}.detail-cover-empty small{color:#788071;font-size:23rpx;font-weight:400}
.detail-card{margin-top:20rpx;margin-bottom:20rpx;padding:26rpx;border:1rpx solid #e9e6df;border-radius:22rpx;background:#fffdf9}.overview-card{padding:30rpx 28rpx}.detail-heading{display:flex;align-items:center;justify-content:space-between;gap:14rpx;margin-bottom:18rpx}.detail-heading>view{display:grid;gap:5rpx;min-width:0}.detail-heading strong{font-size:30rpx}.detail-heading small{color:#7b8778;font-size:20rpx}.detail-heading>text{flex:none;padding:8rpx 12rpx;border-radius:18rpx;background:#eaf3eb;color:#28734c;font-size:18rpx}.detail-heading>text.archived{background:#edf1f6;color:#61728a}
.price-line{display:flex;align-items:baseline;color:#d76440}.price-line strong{font-size:48rpx;letter-spacing:-1rpx}.price-line small{margin-left:4rpx;color:#8b8177;font-size:22rpx}.tag-list{display:flex;flex-wrap:wrap;gap:10rpx;margin-top:16rpx}.tag-list text{max-width:100%;overflow:hidden;padding:8rpx 13rpx;border-radius:8rpx;background:#edf0dc;color:#536a45;font-size:21rpx;text-overflow:ellipsis;white-space:nowrap}
.detail-card>strong{font-size:31rpx}.facilities{margin-top:14rpx}.info-line{display:flex;justify-content:space-between;gap:12rpx;margin-top:12rpx;font-size:20rpx}.info-line text:first-child{color:#7b8778}.info-line text:last-child{color:#344536}.converted-message{margin-top:14rpx;padding:12rpx;border-radius:10rpx;background:#f1f4ed;color:#6d7b69;font-size:19rpx}.no-media-state,.detail-state{display:grid;min-height:260rpx;place-content:center;justify-items:center;gap:16rpx;color:#778273;font-size:23rpx}.detail-state button{margin:0;padding:0 20rpx;border:0;border-radius:12rpx;background:#40543f;color:#fff;font-size:21rpx;line-height:62rpx}
</style>
