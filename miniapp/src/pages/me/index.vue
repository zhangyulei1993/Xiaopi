<script setup lang="ts">
import { onShow } from '@dcloudio/uni-app'
import { computed, ref } from 'vue'
import { clearSession, getSession, type SessionRole } from '../../auth/session'

const role = ref<SessionRole | null>(null)
const isLoggedIn = computed(() => role.value !== null)
const isAdmin = computed(() => role.value === 'admin')

onShow(() => {
  role.value = getSession()?.role || null
})

const login = () => uni.navigateTo({ url: '/pages/auth/login' })
const publish = () => uni.navigateTo({ url: '/pages/publish/index' })
const browse = () => uni.switchTab({ url: '/pages/properties/index' })
const adminLogin = () => uni.navigateTo({ url: '/pages/admin/login' })
const workbench = () => uni.navigateTo({ url: '/pages/admin/index' })
const editProperty = () => uni.navigateTo({ url: '/pages/admin/property-edit' })

const logout = () => {
  uni.showModal({
    title: '退出登录',
    content: '退出后需要重新登录才能使用身份服务。',
    success: (res) => {
      if (!res.confirm) return
      clearSession()
      role.value = null
      uni.showToast({ title: '已退出登录' })
    },
  })
}
</script>

<template>
  <view class="page">
    <view class="profile card" @click="!isLoggedIn && login()">
      <view class="avatar">{{ isAdmin ? '管' : '小' }}</view>
      <view class="profile-copy">
        <view class="title">{{ isAdmin ? '小皮管理员' : isLoggedIn ? '微信用户' : '登录小皮租房' }}</view>
        <view class="muted">
          {{ isAdmin ? '房源运营工作台已授权' : isLoggedIn ? '已登录，可联系房源' : '登录后联系房源并管理身份服务' }}
        </view>
      </view>
      <text v-if="!isLoggedIn" class="arrow">›</text>
    </view>

    <view class="card menu">
      <view @click="browse">浏览全部房源 <text>›</text></view>
      <view @click="publish">提交房源线索 <text>›</text></view>
    </view>

    <view v-if="isAdmin" class="card menu">
      <view class="group-title">运营管理</view>
      <view @click="workbench">管理员工作台 <text>›</text></view>
      <view @click="editProperty">录入正式房源 <text>›</text></view>
    </view>

    <view v-else class="card menu">
      <view @click="adminLogin">运营人员入口 <text>›</text></view>
    </view>

    <button v-if="isLoggedIn" class="logout" @click="logout">退出登录</button>
  </view>
</template>

<style scoped>
.profile { display: flex; align-items: center; gap: 24rpx; padding: 40rpx; }
.profile-copy { flex: 1; }
.avatar { width: 110rpx; height: 110rpx; border-radius: 50%; background: #0f8177; color: #fff; display: flex; align-items: center; justify-content: center; font-size: 48rpx; font-weight: 700; }
.arrow { color: #9bbab4; font-size: 54rpx; }
.menu view { padding: 30rpx 0; border-bottom: 1rpx solid #edf4f1; font-size: 30rpx; }
.menu view:last-child { border-bottom: 0; }
.menu text { float: right; color: #9bbab4; font-size: 40rpx; }
.menu .group-title { padding-top: 4rpx; color: #78a39d; font-size: 24rpx; }
.logout { margin-top: 36rpx; border: 0; background: transparent; color: #d24b61; font-size: 28rpx; }
.logout::after { border: 0; }
</style>
