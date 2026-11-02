<script setup lang="ts">
import { ref } from 'vue'
import { wechatLogin } from '../../api/properties'

const TOKEN_STORAGE_KEY = 'xiaopi_access_token'
const isLoggingIn = ref(false)

const showLoginError = () => {
  uni.showToast({ title: '登录失败，请稍后重试', icon: 'none' })
}

const login = () => {
  if (isLoggingIn.value) return

  isLoggingIn.value = true
  uni.login({
    provider: 'weixin',
    success: async (res) => {
      try {
        const result = await wechatLogin(res.code)
        uni.setStorageSync(TOKEN_STORAGE_KEY, result.accessToken)
        uni.showToast({ title: '登录成功' })
        setTimeout(() => {
          uni.navigateBack({ fail: () => uni.switchTab({ url: '/pages/home/index' }) })
        }, 500)
      } catch {
        showLoginError()
      } finally {
        isLoggingIn.value = false
      }
    },
    fail: () => {
      isLoggingIn.value = false
      showLoginError()
    },
  })
}
</script>

<template>
  <view class="page">
    <view class="card login-card">
      <view class="logo">小皮</view>
      <view class="title">微信登录</view>
      <view class="muted">登录后可以联系房源、提交房源线索</view>
      <button class="button" :loading="isLoggingIn" :disabled="isLoggingIn" @click="login">
        微信一键登录
      </button>
    </view>
  </view>
</template>

<style scoped>
.login-card { text-align: center; margin-top: 180rpx; }
.logo { width: 140rpx; height: 140rpx; line-height: 140rpx; margin: 0 auto 32rpx; border-radius: 42rpx; background: #dff6ef; color: #0f8177; font-weight: 700; font-size: 44rpx; }
.button { margin-top: 60rpx; background: #0f8177; color: white; border-radius: 999rpx; }
</style>
