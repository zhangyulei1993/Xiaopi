<script setup lang="ts">
import { onLoad } from '@dcloudio/uni-app'
import { ref } from 'vue'
import { API_ORIGIN } from '../../api/client'
import { getPublicConfig, listProperties, type Property } from '../../api/properties'

const TOKEN_STORAGE_KEY = 'xiaopi_access_token'
const item = ref<Property | null>(null)
const contactPhone = ref('')

onLoad(async (query) => {
  try {
    const [items, config] = await Promise.all([
      listProperties(),
      getPublicConfig().catch(() => ({ contactPhone: '' })),
    ])
    item.value = items.find((property) => property.id === query?.id) || items[0] || null
    contactPhone.value = config.contactPhone
  } catch {
    uni.showToast({ title: '房源加载失败', icon: 'none' })
  }
})

const mediaUrl = (url: string) => url.startsWith('http') ? url : `${API_ORIGIN}${url}`

const contact = () => {
  if (!uni.getStorageSync(TOKEN_STORAGE_KEY)) {
    uni.showModal({
      title: '登录后联系',
      content: '请先微信登录，再拨打小皮咨询电话',
      confirmText: '去登录',
      success: (res) => {
        if (res.confirm) uni.navigateTo({ url: '/pages/auth/login' })
      },
    })
    return
  }

  if (!contactPhone.value) {
    uni.showToast({ title: '管理员电话尚未配置', icon: 'none' })
    return
  }

  uni.makePhoneCall({ phoneNumber: contactPhone.value })
}
</script>
<template><view v-if="item" class="page"><view v-if="!item.media?.length" class="detail-cover">🏠</view><view v-else class="media-list"><block v-for="(media, index) in item.media" :key="`${media.url}-${index}`"><image v-if="media.type === 'image'" class="media-image" :src="mediaUrl(media.url)" mode="aspectFill"/><video v-else class="media-video" :src="mediaUrl(media.url)" controls object-fit="contain"/></block></view><view class="card"><view class="price">{{ item.rentMonthly }} 元/月</view><view class="title">{{ item.title || item.community }}</view><view class="muted">{{ item.layout }} · {{ item.location }}</view><view class="tags"><text>{{ item.community }}</text><text>{{ item.feeText || '租房信息' }}</text></view></view><view class="card"><view class="title small">房源信息</view><view class="row"><text>小区</text><text>{{ item.community }}</text></view><view class="row"><text>位置</text><text>{{ item.location }}</text></view><view class="row"><text>户型</text><text>{{ item.layout }}</text></view></view><view class="card"><view class="title small">房源描述</view><view class="desc">{{ item.description || '暂无描述' }}</view></view><view class="button" @click="contact">电话咨询</view></view><view v-else class="page muted">房源不存在</view></template>
<style scoped>.detail-cover{height:420rpx;border-radius:28rpx;background:#dff6ef;display:flex;align-items:center;justify-content:center;font-size:160rpx}.media-list{display:flex;flex-direction:column;gap:18rpx}.media-image,.media-video{width:100%;height:420rpx;border-radius:28rpx;background:#dff6ef}.small{font-size:34rpx;margin-bottom:20rpx}.tags{display:flex;gap:16rpx;margin-top:24rpx}.tags text{padding:12rpx 22rpx;border-radius:999rpx;background:#e9faf5;color:#0f8177}.row{display:flex;justify-content:space-between;padding:22rpx 0;border-bottom:1rpx solid #eef5f2;color:#6e9a93}.row text:last-child{color:#073b3a}.desc{line-height:1.8;color:#57857e}</style>
