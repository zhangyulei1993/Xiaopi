<script setup lang="ts">
import { watchPostEffect } from "vue";
import { permissionAudit } from "../../utils/permission-audit";
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { clearFootprints, listFootprints, listProperties, type Footprint, type Property } from "../../api/properties";
import { resolveAssetUrl } from "../../api/client";
import { usePagePresentation } from "../../utils/presentation";
import { listingLayout } from "../../utils/listing-title";
import { getSession } from "../../auth/session";

const footprints = ref<Footprint[]>([]), properties = ref<Property[]>([]), loading = ref(true), loadFailed = ref(false), clearing = ref(false), needsLogin = ref(false);
const { presentation, loadPresentation, pageStyle } = usePagePresentation("footprints", "浏览足迹", "最近看过的房源");
const items = computed(() => footprints.value.map((footprint) => ({ footprint, property: properties.value.find((item) => item.id === footprint.propertyId) })).filter((item) => item.property));
const leadMedia = (property: Property) => property.media?.find((media) => media.type === "video") || property.media?.find((media) => media.type === "image");
const imageFallback = (property: Property) => property.media?.find((media) => media.type === "image")?.url || "";
const load = async () => { loading.value = true; loadFailed.value = false; try { [footprints.value, properties.value] = await Promise.all([listFootprints(), listProperties()]); } catch (error) { loadFailed.value = true; uni.showToast({ title: error instanceof Error ? error.message : "足迹加载失败", icon: "none" }); } finally { loading.value = false; } };
onShow(async () => {
  permissionAudit("onShow", "footprints");
  const role = getSession()?.role;
  permissionAudit("guard", "footprints", role === "user");
  if (role !== "admin") await loadPresentation();
  if (role !== "user") {
    footprints.value = [];
    properties.value = [];
    loading.value = false;
    loadFailed.value = false;
    needsLogin.value = role !== "admin";
    if (role === "admin") {
      permissionAudit("redirect", "footprints", "pages/me/index");
    uni.switchTab({ url: "/pages/me/index" });
    }
    return;
  }
  needsLogin.value = false;
  await load();
});
const open = (id: string) => uni.navigateTo({ url: `/pages/properties/detail?id=${encodeURIComponent(id)}` });
const goLogin = () => uni.navigateTo({ url: "/pages/auth/login?reason=footprints" });
const clearAll = () => { if (clearing.value) return; uni.showModal({ title: "清空浏览足迹", content: "清空后无法恢复，确认继续？", success: async (result) => { if (!result.confirm) return; clearing.value = true; try { await clearFootprints(); footprints.value = []; uni.showToast({ title: "已清空" }); } catch (error) { uni.showToast({ title: error instanceof Error ? error.message : "清空失败，请重试", icon: "none" }); } finally { clearing.value = false; } } }); };
if (import.meta.env.MODE === "test") {
  watchPostEffect(() => permissionAudit("render-state", "footprints", items.value.length > 0));
}

function copyText(fallback: string) { return presentation.value?.copy?.[fallback] || fallback }
</script>

<template><view class="page" :style="pageStyle"><view class="head"><view><view class="title">{{ presentation.title }}</view><text>{{ presentation.subtitle }}</text></view><button v-if="items.length && !loading" :disabled="clearing" @click="clearAll">{{ clearing ? '清空中…' : '清空' }}</button></view><view v-if="needsLogin" class="empty state"><strong>{{ copyText("登录后查看浏览足迹") }}</strong><text>{{ copyText("最近看过的房源会保存在你的账号中。") }}</text><button @click="goLogin">微信登录</button></view><view v-else-if="loading" class="empty state">正在加载浏览足迹…</view><view v-else-if="loadFailed" class="empty state"><strong>足迹暂时加载失败</strong><text>网络恢复后可以重新尝试。</text><button @click="load">重新加载</button></view><view v-else-if="!items.length" class="empty">{{ copyText("还没有浏览记录") }}</view><view v-else class="list"><view v-for="entry in items" :key="entry.property!.id" class="card item" @click="open(entry.property!.id)"><view class="media"><video v-if="leadMedia(entry.property!)?.type === 'video'" :src="resolveAssetUrl(leadMedia(entry.property!)?.url || '')" :poster="resolveAssetUrl(imageFallback(entry.property!))" muted :controls="false" :show-center-play-btn="false" object-fit="cover"/><image v-else :src="resolveAssetUrl(leadMedia(entry.property!)?.url || '')" mode="aspectFill"/><text v-if="leadMedia(entry.property!)?.type === 'video'" class="play">▶</text></view><view><strong>{{ entry.property!.community || entry.property!.title }}</strong><text>{{ listingLayout(entry.property!) }} · {{ entry.property!.location }}</text><view><b>{{ entry.property!.rentMonthly }}</b> 元/月</view><small>{{ entry.footprint.viewCount }} 次浏览 · {{ new Date(entry.footprint.viewedAt).toLocaleDateString() }}</small></view></view></view></view></template>

<style scoped>
.page{min-height:100vh;padding:28rpx 32rpx calc(56rpx + env(safe-area-inset-bottom));background:#f7f6f1}.head{display:flex;align-items:flex-end;justify-content:space-between;gap:24rpx;margin:10rpx 4rpx 30rpx}.head>view{min-width:0}.head .title{font-size:42rpx;line-height:1.24}.head text{display:block;overflow:hidden;margin-top:9rpx;color:#7e887d;font-size:23rpx;line-height:1.45;text-overflow:ellipsis;white-space:nowrap}.head button{flex:none;margin:0;padding:0 22rpx;border:1rpx solid #dce3d8;border-radius:13rpx;background:#f0f3eb;color:#52644c;font-size:22rpx;line-height:62rpx}.head button:active{opacity:.75}.list{display:grid;gap:18rpx}.item{display:grid;grid-template-columns:220rpx minmax(0,1fr);gap:20rpx;margin:0;padding:12rpx;border-color:#ebe6dd;border-radius:20rpx;background:#fffdf9}.media{position:relative;width:220rpx;height:166rpx;overflow:hidden;border-radius:14rpx;background:#e8ebe3}.media image,.media video{display:block;width:100%;height:100%}.play{position:absolute;top:50%;left:50%;display:grid!important;width:48rpx;height:48rpx;margin:0!important;place-items:center;border:2rpx solid rgba(255,255,255,.88);border-radius:50%;background:rgba(28,38,30,.52);color:#fff!important;font-size:22rpx!important;line-height:1!important;transform:translate(-50%,-50%);padding-left:3rpx;box-sizing:border-box}.item>view:not(.media){min-width:0;display:flex;flex-direction:column;padding:5rpx 4rpx 3rpx 0}.item strong{overflow:hidden;color:#2d3d30;font-size:28rpx;font-weight:800;line-height:1.3;text-overflow:ellipsis;white-space:nowrap}.item text{overflow:hidden;margin-top:8rpx;color:#778176;font-size:22rpx;line-height:1.4;text-overflow:ellipsis;white-space:nowrap}.item>view>view{display:flex;align-items:baseline;margin-top:10rpx;color:#8a8f85;font-size:19rpx;line-height:1}.item b{margin-right:4rpx;color:#d76440;font-size:34rpx;font-weight:800}.item small{margin-top:auto;color:#919991;font-size:20rpx;line-height:1.3}.item:active{opacity:.82;transform:scale(.995)}.empty{margin-top:28rpx;padding:150rpx 24rpx;border:1rpx dashed #dfe3d9;border-radius:24rpx;background:#fbfaf6;color:#8b958a;font-size:24rpx;text-align:center}.empty.state{display:flex;min-height:260rpx;align-items:center;justify-content:center;flex-direction:column;gap:10rpx;margin-top:28rpx;padding:32rpx;background:#fbfaf6}.empty.state>text{color:#778176;line-height:1.5}.empty.state button{margin-top:12rpx;padding:0 26rpx;border:0;border-radius:13rpx;background:#40543f;color:#fff;font-size:22rpx;line-height:66rpx}.empty.state button::after{border:0}
</style>
