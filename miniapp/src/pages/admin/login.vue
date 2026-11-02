<script setup lang="ts">
import { reactive, ref } from 'vue'
import { adminLogin } from '../../api/properties'
import { saveSession } from '../../auth/session'

const form = reactive({ username: '', password: '' })
const submitting = ref(false)

const submit = async () => {
  if (!form.username.trim() || !form.password) {
    uni.showToast({ title: '请输入账号和密码', icon: 'none' })
    return
  }

  submitting.value = true
  try {
    const result = await adminLogin(form.username.trim(), form.password)
    saveSession(result.accessToken, 'admin')
    uni.showToast({ title: '管理员登录成功' })
    setTimeout(() => uni.redirectTo({ url: '/pages/admin/index' }), 400)
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : '登录失败', icon: 'none' })
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <view class="page">
    <view class="card login-card">
      <view class="logo">管</view>
      <view class="title">管理员登录</view>
      <view class="muted intro">仅供小皮房源运营人员使用</view>
      <input v-model="form.username" class="input" placeholder="管理员账号" />
      <input v-model="form.password" class="input" password placeholder="管理员密码" />
      <button class="button" :loading="submitting" :disabled="submitting" @click="submit">
        登录工作台
      </button>
    </view>
  </view>
</template>

<style scoped>
.login-card { margin-top: 100rpx; text-align: center; }
.logo { width: 120rpx; height: 120rpx; line-height: 120rpx; margin: 0 auto 24rpx; border-radius: 36rpx; background: #dff6ef; color: #0f8177; font-size: 40rpx; font-weight: 700; }
.intro { margin: 12rpx 0 40rpx; }
.input { height: 92rpx; padding: 0 28rpx; margin-top: 20rpx; border: 1rpx solid #d8ebe7; border-radius: 20rpx; background: #f8fffd; text-align: left; }
.button { margin-top: 42rpx; }
</style>
