<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { listProperties, type Property } from '../../api/properties'
import { cacheMapProperties, getMapPropertyCache } from './mapPropertyCache'

const cached = getMapPropertyCache()
const properties = ref<Property[]>(cached ?? [])
const loading = ref(false)
const failed = ref(false)
const mapUnavailable = ref(false)
const mapRenderKey = ref(0)
const selected = ref<number>()
let mapWatchdog: ReturnType<typeof setTimeout> | undefined
const communityGroups = computed(() => {
  const groups = new Map<string, { name: string; items: Property[]; position?: { latitude: number; longitude: number } }>()
  for (const item of properties.value) {
    if (item.status !== 'available') continue
    const name = item.community?.trim()
    const key = name?.toLowerCase().replace(/[^\p{L}\p{N}]/gu, '')
    if (!key) continue
    const group = groups.get(key) ?? { name, items: [] }
    group.items.push(item)
    const latitude = Number(item.latitude)
    const longitude = Number(item.longitude)
    if (!group.position && latitude && longitude && Math.abs(latitude) <= 90 && Math.abs(longitude) <= 180) {
      group.position = { latitude, longitude }
    }
    groups.set(key, group)
  }
  return [...groups.values()]
})
const communities = computed(() => communityGroups.value.flatMap(({ name, items, position }) => position ? [{ name, items, ...position }] : []))
const markers = computed(() => communities.value.map((group, index) => ({
  id: index + 1,
  latitude: group.latitude,
  longitude: group.longitude,
  iconPath: '/static/icons/pin.svg',
  width: 18,
  height: 18,
  callout: { content: group.name + ' · ' + group.items.length + ' 套', display: 'ALWAYS', color: '#ffffff', bgColor: '#375644', padding: 7, borderRadius: 12, fontSize: 12 },
})))
const active = computed(() => communities.value[(selected.value ?? 0) - 1])
const viewport = ref<{ center: { latitude: number; longitude: number }; points: { latitude: number; longitude: number }[] }>()
function initializeViewport() {
  if (viewport.value) return
  const points = communities.value.map(({ latitude, longitude }) => ({ latitude, longitude }))
  viewport.value = { center: points[0] ?? { latitude: 30.475, longitude: 114.421 }, points }
  startMapWatchdog()
}
function clearMapWatchdog() {
  if (mapWatchdog) clearTimeout(mapWatchdog)
  mapWatchdog = undefined
}
function startMapWatchdog() {
  clearMapWatchdog()
  mapUnavailable.value = false
  mapWatchdog = setTimeout(() => {
    mapUnavailable.value = true
    selected.value = undefined
  }, 8000)
}
function onMapUpdated() {
  clearMapWatchdog()
}
function retryMap() {
  mapRenderKey.value += 1
  startMapWatchdog()
}
function selectMarker(event: { detail?: { markerId?: number }; markerId?: number }) {
  selected.value = Number(event.detail?.markerId ?? event.markerId) || undefined
}
async function load() {
  loading.value = true
  failed.value = false
  try {
    properties.value = await listProperties()
    cacheMapProperties(properties.value)
    selected.value = undefined
  } catch {
    failed.value = true
  } finally {
    initializeViewport()
    loading.value = false
  }
}
function openProperty(id: string) {
  uni.navigateTo({ url: '/pages/properties/detail?id=' + encodeURIComponent(id) })
}
onLoad(() => {
  if (cached?.length) initializeViewport()
  void load()
})
onBeforeUnmount(clearMapWatchdog)
</script>

<template>
  <view class="map-page">
    <map v-if="viewport && !mapUnavailable" :key="mapRenderKey" class="property-map" :latitude="viewport.center.latitude" :longitude="viewport.center.longitude" :scale="12"
      :markers="markers" :include-points="viewport.points" @updated="onMapUpdated" @markertap="selectMarker" @callouttap="selectMarker" />
    <view v-if="!mapUnavailable" class="map-summary">
      <text v-if="loading && !viewport">正在加载房源…</text>
      <text v-else-if="failed">{{ communities.length ? "刷新失败，正在显示最近房源" : "房源加载失败" }}</text>
      <text v-else-if="communities.length">{{ communities.length }} 个小区 · 点击标记看房源</text>
      <text v-else>暂时没有可在地图上显示的房源</text>
      <button v-if="failed" size="mini" @click="load">重试</button>
    </view>
    <view v-else class="map-fallback">
      <view class="fallback-heading"><strong>地图暂不可用</strong><text>可能是当前微信版本或设备地图组件未能加载</text></view>
      <button size="mini" @click="retryMap">重试地图</button>
      <button size="mini" @click="load">刷新房源</button>
    </view>
    <scroll-view v-if="mapUnavailable" scroll-y class="fallback-list">
      <view v-if="loading && !communities.length" class="fallback-empty">正在加载房源…</view>
      <view v-else-if="!communities.length" class="fallback-empty">{{ failed ? "房源加载失败，请点击刷新房源" : "暂时没有可在地图上显示的房源" }}</view>
      <view v-for="group in communityGroups" :key="group.name" class="fallback-community">
        <view class="fallback-community-heading"><strong>{{ group.name }}</strong><text>{{ group.items.length }} 套</text></view>
        <view v-for="item in group.items" :key="item.id" class="map-row" @click="openProperty(item.id)">
          <text>{{ item.layout || item.community }}</text><strong>¥{{ item.rentMonthly }}/月 ›</strong>
        </view>
      </view>
    </scroll-view>
    <view v-if="active" class="map-sheet">
      <view class="sheet-heading"><strong>{{ active.name }} · {{ active.items.length }} 套</strong><button size="mini" @click="selected = undefined">关闭</button></view>
      <scroll-view scroll-y class="map-results">
        <view v-for="item in active.items" :key="item.id" class="map-row" @click="openProperty(item.id)">
          <text>{{ item.layout || item.community }}</text><strong>¥{{ item.rentMonthly }}/月 ›</strong>
        </view>
      </scroll-view>
    </view>
  </view>
</template>

<style scoped>
.map-page{position:relative;height:100vh;background:#f7f6f1;color:#283d31}
.property-map{width:100%;height:100%}
.map-summary{position:absolute;top:20rpx;left:24rpx;display:flex;align-items:center;gap:14rpx;background:#fffdf9;padding:16rpx 24rpx;border-radius:20rpx;font-size:25rpx}
.map-fallback{position:absolute;z-index:2;top:16rpx;left:20rpx;right:20rpx;display:flex;align-items:center;gap:12rpx;padding:16rpx 18rpx;border:1rpx solid #e8e4db;border-radius:18rpx;background:#fffdf9;box-shadow:0 6rpx 24rpx rgba(32,51,39,.08)}
.fallback-heading{min-width:0;flex:1}.fallback-heading strong,.fallback-heading text{display:block}.fallback-heading strong{font-size:24rpx}.fallback-heading text{margin-top:4rpx;color:#7b857a;font-size:18rpx;line-height:1.35}.map-fallback button{flex:none;margin:0;padding:0 14rpx;border-radius:10rpx;background:#40543f;color:#fff;font-size:19rpx;line-height:58rpx}.map-fallback button:last-child{background:#edf2e8;color:#40543f}
.fallback-list{position:absolute;top:132rpx;right:20rpx;bottom:24rpx;left:20rpx;overflow-y:auto;padding:0 20rpx 18rpx;border:1rpx solid #ebe7df;border-radius:22rpx;background:#fffdf9;box-shadow:0 8rpx 28rpx rgba(32,51,39,.08)}
.fallback-empty{padding:48rpx 10rpx;color:#7b857a;text-align:center;font-size:24rpx}
.fallback-community{padding:18rpx 0;border-bottom:1rpx solid #eeeae3}.fallback-community-heading{display:flex;align-items:center;justify-content:space-between;gap:14rpx;padding:0 0 6rpx}.fallback-community-heading strong{color:#304636;font-size:25rpx}.fallback-community-heading text{flex:none;color:#71806f;font-size:20rpx}
.map-sheet{position:absolute;bottom:0;left:0;right:0;background:#fffdf9;border-radius:28rpx 28rpx 0 0;padding:24rpx 28rpx calc(24rpx + env(safe-area-inset-bottom))}
.sheet-heading,.map-row{display:flex;align-items:center;justify-content:space-between;gap:16rpx}
.map-results{max-height:36vh}
.map-row{border-bottom:1px solid #eeeae3;padding:20rpx 0;font-size:24rpx}.fallback-community .map-row:last-child{border-bottom:0}
</style>
