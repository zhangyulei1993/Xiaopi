<script setup lang="ts">
import { onShow } from '@dcloudio/uni-app'
import { computed, ref } from 'vue'
import { API_ORIGIN } from '../../api/client'
import { listProperties, type Property } from '../../api/properties'

const properties = ref<Property[]>([])
const keyword = ref('')
const selectedArea = ref('全部')
const loading = ref(false)
const loadFailed = ref(false)

const areas = computed(() => {
  const values = properties.value.map((item) => item.location?.split(/[·/]/)[0]?.trim()).filter(Boolean)
  return ['全部', ...Array.from(new Set(values))]
})

const filteredProperties = computed(() => {
  const query = keyword.value.trim().toLowerCase()
  return properties.value.filter((item) => {
    const matchesArea = selectedArea.value === '全部' || item.location?.includes(selectedArea.value)
    const searchable = `${item.title} ${item.community} ${item.location} ${item.layout}`.toLowerCase()
    return matchesArea && (!query || searchable.includes(query))
  })
})

const load = async () => {
  loading.value = true
  loadFailed.value = false
  try {
    properties.value = await listProperties()
  } catch (error) {
    loadFailed.value = true
    uni.showToast({ title: error instanceof Error ? error.message : '房源加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

onShow(load)

const open = (id: string) => uni.navigateTo({ url: `/pages/properties/detail?id=${id}` })
const coverUrl = (item: Property) => {
  const cover = item.media?.find((media) => media.type === 'image')?.url
  if (!cover) return ''
  return cover.startsWith('http') ? cover : `${API_ORIGIN}${cover}`
}
</script>

<template>
  <view class="page">
    <view class="location">发现合适住处 <text>{{ filteredProperties.length }} 套可租</text></view>
    <view class="search card">
      <text>⌕</text>
      <input v-model="keyword" placeholder="搜索小区、位置或户型" confirm-type="search" />
      <text v-if="keyword" class="clear" @click="keyword = ''">×</text>
    </view>

    <scroll-view scroll-x class="chips">
      <text
        v-for="area in areas"
        :key="area"
        :class="{ active: selectedArea === area }"
        @click="selectedArea = area"
      >{{ area }}</text>
    </scroll-view>

    <view v-if="loading" class="state card">正在加载房源...</view>
    <view v-else-if="loadFailed" class="state card">
      <view>房源加载失败</view>
      <button size="mini" @click="load">重新加载</button>
    </view>
    <view v-else-if="filteredProperties.length" class="grid">
      <view v-for="item in filteredProperties" :key="item.id" class="property card" @click="open(item.id)">
        <view class="cover">
          <image v-if="coverUrl(item)" :src="coverUrl(item)" mode="aspectFill" />
          <text v-else>🏠</text>
          <view class="badge">可租</view>
          <view class="fee">{{ item.feeText || '租房信息' }}</view>
        </view>
        <view class="title small">{{ item.title || item.community }}</view>
        <view class="muted summary">{{ item.layout }} · {{ item.location }}</view>
        <view class="price">{{ item.rentMonthly }} <small>元/月</small></view>
      </view>
    </view>
    <view v-else class="state card">没有找到符合条件的房源</view>
  </view>
</template>

<style scoped>
.location { font-size: 32rpx; color: #315f5a; margin: 28rpx 0; font-weight: 700; }
.location text { float: right; color: #0e8177; font-size: 26rpx; font-weight: 400; }
.search { display: flex; align-items: center; gap: 16rpx; color: #8aa7a0; }
.search input { flex: 1; color: #073b3a; }
.clear { padding: 6rpx 14rpx; font-size: 34rpx; }
.chips { white-space: nowrap; margin: 10rpx 0 24rpx; }
.chips text { display: inline-block; padding: 14rpx 26rpx; background: #fff; border-radius: 999rpx; color: #6b9891; margin-right: 14rpx; }
.chips .active { background: #d9f7ef; color: #0f8177; font-weight: 700; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 18rpx; }
.property { padding: 0; overflow: hidden; margin: 0; }
.cover { height: 230rpx; background: linear-gradient(145deg,#cdebe5,#86b8ae); display: flex; align-items: center; justify-content: center; font-size: 90rpx; position: relative; overflow: hidden; }
.cover image { width: 100%; height: 100%; }
.badge,.fee { position: absolute; color: #fff; font-size: 22rpx; padding: 8rpx 14rpx; }
.badge { top: 16rpx; left: 16rpx; background: #0e8177; border-radius: 999rpx; }
.fee { bottom: 0; left: 0; max-width: 80%; background: #ff8610; border-radius: 0 16rpx 0 0; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.small { font-size: 30rpx; margin: 18rpx 18rpx 8rpx; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.summary,.property>.price { margin-left: 18rpx; margin-right: 18rpx; }
.summary { min-height: 68rpx; }
.property>.price { font-size: 34rpx; margin-top: 12rpx; margin-bottom: 20rpx; }
.property>.price small { font-size: 22rpx; }
.state { padding: 90rpx 30rpx; text-align: center; color: #78a39d; }
.state button { margin-top: 24rpx; background: #e1f7f1; color: #0e8177; }
</style>
