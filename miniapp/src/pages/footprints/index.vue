<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { clearFootprints, listFootprints, listProperties, type Footprint, type Property } from "../../api/properties";
import { resolveAssetUrl } from "../../api/client";

const footprints = ref<Footprint[]>([]), properties = ref<Property[]>([]), loading = ref(true);
const items = computed(() => footprints.value.map((footprint) => ({ footprint, property: properties.value.find((item) => item.id === footprint.propertyId) })).filter((item) => item.property));
const load = async () => { loading.value = true; try { [footprints.value, properties.value] = await Promise.all([listFootprints(), listProperties()]); } catch (error) { uni.showToast({ title: error instanceof Error ? error.message : "足迹加载失败", icon: "none" }); } finally { loading.value = false; } };
onShow(load);
const open = (id: string) => uni.navigateTo({ url: `/pages/properties/detail?id=${encodeURIComponent(id)}` });
const clearAll = () => uni.showModal({ title: "清空浏览足迹", content: "清空后无法恢复，确认继续？", success: async (result) => { if (!result.confirm) return; await clearFootprints(); footprints.value = []; uni.showToast({ title: "已清空" }); } });
</script>

<template><view class="page"><view class="head"><view><view class="title">浏览足迹</view><text>最近看过的房源</text></view><button v-if="items.length" @click="clearAll">清空</button></view><view v-if="!loading && !items.length" class="empty">还没有浏览记录</view><view class="list"><view v-for="entry in items" :key="entry.property!.id" class="card item" @click="open(entry.property!.id)"><image :src="resolveAssetUrl(entry.property!.media?.find(media => media.type === 'image')?.url || '')" mode="aspectFill"/><view><strong>{{ entry.property!.community || entry.property!.title }}</strong><text>{{ entry.property!.layout }} · {{ entry.property!.location }}</text><view><b>{{ entry.property!.rentMonthly }}</b> 元/月</view><small>{{ entry.footprint.viewCount }} 次浏览 · {{ new Date(entry.footprint.viewedAt).toLocaleDateString() }}</small></view></view></view></view></template>

<style scoped>.page{padding:32rpx}.head{display:flex;align-items:center;justify-content:space-between;margin:16rpx 0 30rpx}.head text{display:block;margin-top:8rpx;color:#72817a;font-size:24rpx}.head button{margin:0;padding:12rpx 24rpx;border:0;background:#eef3ea;color:#526a3c;font-size:24rpx}.head button::after{border:0}.list{display:grid;gap:22rpx}.item{display:grid;grid-template-columns:220rpx 1fr;gap:24rpx;padding:18rpx}.item image{width:220rpx;height:166rpx;border-radius:18rpx;background:#eef1eb}.item>view{min-width:0;display:flex;flex-direction:column}.item strong{font-size:30rpx;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.item text{margin-top:10rpx;color:#718077;font-size:24rpx;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.item b{font-size:34rpx;color:#dc6543}.item small{margin-top:auto;color:#99a39d;font-size:21rpx}.empty{padding:160rpx 0;text-align:center;color:#8b9891}</style>
