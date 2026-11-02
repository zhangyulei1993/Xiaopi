<script setup lang="ts">
import { ref } from 'vue'
import { saveSession } from '../../auth/session'
import { temporaryLogin, wechatLogin } from '../../api/properties'

const isLoggingIn = ref(false)
const phone = ref('')
const accessCode = ref('')

const showLoginError = (message = '登录失败，请稍后重试') => {
  uni.showToast({ title: message, icon: 'none' })
}

const completeLogin = (accessToken: string, role?: 'user' | 'admin', boundPhone = '') => {
  saveSession(accessToken, role === 'admin' ? 'admin' : 'user')
  if (boundPhone) uni.setStorageSync('xiaopi_bound_phone', boundPhone)
  uni.showToast({ title: '登录成功' })
  setTimeout(() => {
    uni.navigateBack({ fail: () => uni.switchTab({ url: '/pages/home/index' }) })
  }, 500)
}

const loginWithWechat = () => {
  if (isLoggingIn.value) return

  isLoggingIn.value = true
  uni.login({
    provider: 'weixin',
    success: async (res) => {
      try {
        const result = await wechatLogin(res.code)
        completeLogin(result.accessToken, result.role)
      } catch (error) {
        showLoginError(error instanceof Error ? error.message : undefined)
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

const loginTemporarily = async () => {
  if (isLoggingIn.value) return
  if (!/^\+?\d{6,20}$/.test(phone.value.trim()) || accessCode.value.trim().length !== 8) {
    showLoginError('请输入手机号和 8 位访问码')
    return
  }
  isLoggingIn.value = true
  try {
    const result = await temporaryLogin(phone.value.trim(), accessCode.value.trim().toUpperCase())
    completeLogin(result.accessToken, result.role, result.phone)
  } catch (error) {
    showLoginError(error instanceof Error ? error.message : undefined)
  } finally {
    isLoggingIn.value = false
  }
}
</script>

<template>
  <view class="page">
    <view class="card login-card">
      <view class="logo">小皮</view>
      <view class="title">登录小皮租房</view>
      <view class="muted">微信能力接入完成前，可使用管理员发放的临时访问码登录</view>
      <view class="temporary-form">
        <input v-model="phone" type="number" maxlength="20" placeholder="手机号" />
        <input v-model="accessCode" maxlength="8" placeholder="8 位访问码" />
      </view>
      <button class="button" :loading="isLoggingIn" :disabled="isLoggingIn" @click="loginTemporarily">
        使用访问码登录
      </button>
      <view class="divider"><text>或</text></view>
      <button class="wechat-button" :loading="isLoggingIn" :disabled="isLoggingIn" @click="loginWithWechat">微信一键登录</button>
    </view>
  </view>
</template>

<style scoped>
.login-card { text-align: center; margin-top: 180rpx; }
.logo { width: 140rpx; height: 140rpx; line-height: 140rpx; margin: 0 auto 32rpx; border-radius: 42rpx; background: #dff6ef; color: #0f8177; font-weight: 700; font-size: 44rpx; }
.button { margin-top: 60rpx; background: #0f8177; color: white; border-radius: 999rpx; }
.temporary-form { margin-top: 40rpx; }
.temporary-form input { height: 88rpx; margin-top: 20rpx; padding: 0 28rpx; border: 1rpx solid #d7eae5; border-radius: 18rpx; background: #fff; text-align: left; }
.divider { position: relative; margin: 34rpx 0 24rpx; color: #9bbab4; font-size: 24rpx; }
.divider::before { content: ''; position: absolute; top: 50%; left: 0; right: 0; height: 1rpx; background: #e3efeb; }
.divider text { position: relative; padding: 0 20rpx; background: #fff; }
.wechat-button { background: #edf7f3; color: #0f8177; border-radius: 999rpx; }
.wechat-button::after { border: 0; }
</style>
