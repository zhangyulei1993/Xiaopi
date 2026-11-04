<script setup lang="ts">
import { onLoad } from '@dcloudio/uni-app'
import { ref } from 'vue'
import { saveSession } from '../../auth/session'
import { temporaryLogin, wechatLogin } from '../../api/properties'
import { usePagePresentation } from '../../utils/presentation'

const isLoggingIn = ref(false)
const phone = ref('')
const accessCode = ref('')
const { presentation, loadPresentation, pageStyle } = usePagePresentation('authLogin', '登录小皮', '登录后管理你的找房进度')

onLoad(() => loadPresentation())

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
  if (!/^\+?\d{6,20}$/.test(phone.value.trim()) || !/^\d{6}$/.test(accessCode.value.trim())) {
	showLoginError('请输入手机号和 6 位数字访问码')
    return
  }
  isLoggingIn.value = true
  try {
	const result = await temporaryLogin(phone.value.trim(), accessCode.value.trim())
    completeLogin(result.accessToken, result.role, result.phone)
  } catch (error) {
    showLoginError(error instanceof Error ? error.message : undefined)
  } finally {
    isLoggingIn.value = false
  }
}
</script>

<template>
  <view class="page" :style="pageStyle">
    <view class="login-hero"><view class="brand-mark"><text>小</text><i></i></view><view class="hero-kicker">XIAOPI RENTAL</view><view class="hero-title">住进喜欢的生活</view><text>收藏好房、保留足迹，及时收到联系</text></view>
    <view class="card login-card">
      <view class="login-title"><view><strong>登录小皮</strong><text>微信账号安全登录</text></view><i>可信房源</i></view>
      <button class="wechat-button" :loading="isLoggingIn" :disabled="isLoggingIn" @click="loginWithWechat"><text>微</text><view><strong>微信一键登录</strong><small>推荐使用</small></view><b>›</b></button>
      <view class="divider"><text>测试访问</text></view>
      <view class="temporary-form">
        <view><text>手机号</text><input v-model="phone" type="number" maxlength="20" placeholder="请输入手机号" /></view>
        <view><text>访问码</text><input v-model="accessCode" type="number" maxlength="6" placeholder="6 位数字" /></view>
      </view>
      <button class="code-button" :loading="isLoggingIn" :disabled="isLoggingIn" @click="loginTemporarily">使用访问码登录</button>
    </view>
    <view class="login-note">登录即表示同意小程序隐私保护说明</view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:54rpx 32rpx 40rpx;background:radial-gradient(circle at 82% 4%,#e2e8cd 0,transparent 34%),#f6f3eb}.login-hero{padding:28rpx 14rpx 48rpx}.brand-mark{position:relative;display:grid;place-items:center;width:90rpx;height:90rpx;border-radius:28rpx 28rpx 10rpx;background:#40543f;color:#fff;font-size:38rpx;font-weight:850;box-shadow:0 16rpx 34rpx rgba(64,84,63,.18)}.brand-mark i{position:absolute;right:-7rpx;bottom:10rpx;width:18rpx;height:18rpx;border:5rpx solid #f6f3eb;border-radius:50%;background:#d96b4b}.hero-kicker{margin-top:44rpx;color:#7f8b76;font-size:18rpx;font-weight:700;letter-spacing:4rpx}.hero-title{margin-top:10rpx;color:#29382d;font-size:52rpx;font-weight:850;letter-spacing:-4rpx}.login-hero>text{display:block;margin-top:16rpx;color:#7d877d;font-size:24rpx}.login-card{margin:0;padding:34rpx 30rpx 30rpx;border:0;border-radius:32rpx;background:rgba(255,255,255,.94);box-shadow:0 24rpx 70rpx rgba(64,76,59,.1)}.login-title{display:flex;align-items:flex-start;justify-content:space-between}.login-title strong,.login-title text{display:block}.login-title strong{font-size:35rpx}.login-title text{margin-top:7rpx;color:#8a9389;font-size:22rpx}.login-title>i{padding:7rpx 12rpx;border-radius:8rpx;background:#eef1e4;color:#647259;font-size:18rpx;font-style:normal;font-weight:700}.wechat-button{display:grid;grid-template-columns:54rpx 1fr auto;align-items:center;gap:16rpx;width:100%;margin:30rpx 0 0;padding:20rpx 24rpx;border:0;border-radius:18rpx;background:#40543f;color:#fff;text-align:left;line-height:1.2}.wechat-button::after,.code-button::after{border:0}.wechat-button>text{display:grid;place-items:center;width:54rpx;height:54rpx;border-radius:16rpx;background:#e7edcf;color:#40543f;font-size:24rpx;font-weight:800}.wechat-button strong,.wechat-button small{display:block}.wechat-button strong{font-size:27rpx}.wechat-button small{margin-top:7rpx;color:rgba(255,255,255,.6);font-size:19rpx}.wechat-button b{font-size:38rpx;font-weight:400}.divider{position:relative;margin:34rpx 0 18rpx;color:#a0a69f;font-size:20rpx;text-align:center}
.divider::before { content: ''; position: absolute; top: 50%; left: 0; right: 0; height: 1rpx; background: #e3efeb; }
.divider text{position:relative;padding:0 18rpx;background:#fff}.temporary-form{display:grid;grid-template-columns:1.15fr .85fr;gap:12rpx}.temporary-form>view{padding:16rpx 18rpx;border:1rpx solid #e5e7df;border-radius:16rpx;background:#fafbf8}.temporary-form text{display:block;color:#727d70;font-size:19rpx}.temporary-form input{height:48rpx;margin-top:5rpx;font-size:25rpx;line-height:48rpx}.code-button{width:100%;margin:18rpx 0 0;border:0;border-radius:16rpx;background:#ecefe7;color:#40543f;font-size:25rpx;line-height:78rpx}.login-note{margin-top:28rpx;color:#9ca39a;font-size:19rpx;text-align:center}
</style>
