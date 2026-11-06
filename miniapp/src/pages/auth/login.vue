<script setup lang="ts">
import { onLoad, onShow } from '@dcloudio/uni-app'
import { ref } from 'vue'
import { clearSession, getSession, saveSession } from '../../auth/session'
import { API_ORIGIN } from '../../api/client'
import { getWechatUserProfile, refreshAuth, temporaryLogin, updateWechatUserProfile, wechatLogin } from '../../api/properties'
import { usePagePresentation } from '../../utils/presentation'

const isLoggingIn = ref(false)
const isUploadingAvatar = ref(false)
const isSavingProfile = ref(false)
const phone = ref('')
const accessCode = ref('')
const nickname = ref('')
const avatarUrl = ref('')
const showProfileSetup = ref(false)
const loginReason = ref('登录后可同步关注、足迹与找房进度')
const brandName = ref('小皮租房')
const { presentation, loadPresentation, pageStyle } = usePagePresentation('authLogin', '登录小皮', '登录后管理你的找房进度')

onLoad(async (query) => {
  const config = await loadPresentation()
  brandName.value = config?.heroTitle || brandName.value
  const reason = String(query?.reason || '')
  loginReason.value = reason === 'favorites' ? '登录后查看并同步你的关注房源' : reason === 'footprints' ? '登录后查看最近浏览过的房源' : loginReason.value
})
onShow(() => {
  const croppedAvatar = String(uni.getStorageSync('xiaopi-avatar-crop-result') || '')
  if (!croppedAvatar) return
  uni.removeStorageSync('xiaopi-avatar-crop-result')
  uploadAvatar(croppedAvatar)
})

const showLoginError = (message = '登录失败，请稍后重试') => {
  uni.showToast({ title: message, icon: 'none' })
}

const enterApp = () => uni.navigateBack({ fail: () => uni.switchTab({ url: '/pages/home/index' }) })

const completeLogin = async (accessToken: string, role?: 'user' | 'admin', boundPhone = '', askProfile = false) => {
  saveSession(accessToken, role === 'admin' ? 'admin' : 'user')
  if (boundPhone) uni.setStorageSync('xiaopi_bound_phone', boundPhone)
  if (askProfile) {
    try {
      const profile = await getWechatUserProfile()
      nickname.value = profile.nickname || ''
      avatarUrl.value = profile.avatarUrl || ''
      if (!profile.nickname || !profile.avatarUrl) {
        showProfileSetup.value = true
        return
      }
    } catch {
      // 身份已建立，不因资料读取失败阻断登录。
    }
  }
  uni.showToast({ title: '登录成功' })
  setTimeout(enterApp, 350)
}

const loginWithWechat = () => {
  if (isLoggingIn.value) return

  isLoggingIn.value = true
  uni.login({
    provider: 'weixin',
    success: async (res) => {
      try {
        const result = await wechatLogin(res.code)
        await completeLogin(result.accessToken, result.role, '', true)
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
    clearSession()
    uni.removeStorageSync('xiaopi_bound_phone')
    saveSession(result.accessToken, result.role)
    try {
      const refreshed = await refreshAuth()
      saveSession(refreshed.accessToken, refreshed.role)
      uni.setStorageSync('xiaopi_bound_phone', result.phone)
      uni.removeStorageSync('xiaopi_show_favorites')
      uni.showToast({ title: '身份已切换' })
      setTimeout(() => uni.reLaunch({ url: '/pages/me/index' }), 300)
    } catch (error) {
      clearSession()
      uni.removeStorageSync('xiaopi_bound_phone')
      throw error
    }
  } catch (error) {
    showLoginError(error instanceof Error ? error.message : undefined)
  } finally {
    isLoggingIn.value = false
  }
}

const uploadAvatar = (path: string) => {
  if (!path || isUploadingAvatar.value) return
  isUploadingAvatar.value = true
  uni.uploadFile({
    url: `${API_ORIGIN}/api/v1/auth/avatar`,
    filePath: path,
    name: 'file',
    header: { Authorization: `Bearer ${getSession()?.accessToken || ''}` },
    success: (response) => {
      try {
        const body = JSON.parse(String(response.data || '{}'))
        if (response.statusCode >= 200 && response.statusCode < 300 && body?.code === 0) {
          avatarUrl.value = String(body.data?.url || '')
          return
        }
        uni.showToast({ title: body?.message || '头像上传失败', icon: 'none' })
      } catch {
        uni.showToast({ title: '头像上传失败', icon: 'none' })
      }
    },
    fail: () => uni.showToast({ title: '头像上传失败', icon: 'none' }),
    complete: () => { isUploadingAvatar.value = false },
  })
}
const chooseAvatar = (event: { detail: { avatarUrl?: string } }) => {
  const path = event.detail.avatarUrl
  if (!path || isUploadingAvatar.value) return
  uni.getImageInfo({
    src: path,
    success: (image) => {
      if (image.width === image.height) {
        uploadAvatar(path)
        return
      }
      uni.setStorageSync('xiaopi-avatar-crop-source', { path, width: image.width, height: image.height })
      uni.navigateTo({ url: '/pages/auth/avatar-crop' })
    },
    fail: () => uni.showToast({ title: '头像读取失败', icon: 'none' }),
  })
}

const saveProfileAndEnter = async () => {
  if (isSavingProfile.value) return
  if (!nickname.value.trim() && !avatarUrl.value) {
    uni.showToast({ title: '可直接跳过，或补充一项资料', icon: 'none' })
    return
  }
  isSavingProfile.value = true
  try {
    await updateWechatUserProfile(nickname.value.trim(), avatarUrl.value)
    uni.showToast({ title: '资料已保存' })
    setTimeout(enterApp, 300)
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : '保存失败', icon: 'none' })
  } finally {
    isSavingProfile.value = false
  }
}
</script>

<template>
  <view class="page" :style="pageStyle">
    <view class="login-hero"><view class="brand-mark"><text>小</text><i></i></view><view class="hero-kicker">{{ brandName }}</view><view class="hero-title">{{ presentation.title }}</view><text>{{ presentation.subtitle }}</text></view>
    <view class="card login-card">
      <view class="login-title"><view><strong>微信登录</strong><text>{{ loginReason }}</text></view><i>可信房源</i></view>
      <button class="wechat-button" :loading="isLoggingIn" :disabled="isLoggingIn" @click="loginWithWechat"><text>微</text><view><strong>微信一键登录</strong><small>推荐使用</small></view><b>›</b></button>
      <view class="divider"><text>测试访问</text></view>
      <view class="temporary-form">
        <view><text>手机号</text><input v-model="phone" type="number" maxlength="20" placeholder="请输入手机号" /></view>
        <view><text>访问码</text><input v-model="accessCode" type="number" maxlength="6" placeholder="6 位数字" /></view>
      </view>
      <button class="code-button" :loading="isLoggingIn" :disabled="isLoggingIn" @click="loginTemporarily">使用访问码登录</button>
    </view>
    <view class="login-note">登录即表示同意小程序隐私保护说明</view>
    <view v-if="showProfileSetup" class="profile-mask">
      <view class="profile-sheet">
        <view class="setup-kicker">WELCOME TO XIAOPI</view>
        <strong>让小皮认识你</strong>
        <text>头像和昵称仅用于展示，可稍后在“我的”中完善。</text>
        <button class="setup-avatar" open-type="chooseAvatar" :loading="isUploadingAvatar" @chooseavatar="chooseAvatar">
          <image v-if="avatarUrl" :src="avatarUrl" mode="aspectFill" /><text v-else>选头像</text>
        </button>
        <input v-model="nickname" type="nickname" maxlength="32" placeholder="填写昵称（可选）" />
        <button class="setup-confirm" :loading="isSavingProfile" @click="saveProfileAndEnter">保存并进入</button>
        <view class="setup-skip" @click="enterApp">暂时跳过</view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:54rpx 32rpx 40rpx;background:radial-gradient(circle at 82% 4%,#e2e8cd 0,transparent 34%),#f6f3eb}.login-hero{padding:28rpx 14rpx 48rpx}.brand-mark{position:relative;display:grid;place-items:center;width:90rpx;height:90rpx;border-radius:28rpx 28rpx 10rpx;background:#40543f;color:#fff;font-size:38rpx;font-weight:850;box-shadow:0 16rpx 34rpx rgba(64,84,63,.18)}.brand-mark i{position:absolute;right:-7rpx;bottom:10rpx;width:18rpx;height:18rpx;border:5rpx solid #f6f3eb;border-radius:50%;background:#d96b4b}.hero-kicker{margin-top:44rpx;color:#7f8b76;font-size:18rpx;font-weight:700;letter-spacing:4rpx}.hero-title{margin-top:10rpx;color:#29382d;font-size:52rpx;font-weight:850;letter-spacing:-4rpx}.login-hero>text{display:block;margin-top:16rpx;color:#7d877d;font-size:24rpx}.login-card{margin:0;padding:34rpx 30rpx 30rpx;border:0;border-radius:32rpx;background:rgba(255,255,255,.94);box-shadow:0 24rpx 70rpx rgba(64,76,59,.1)}.login-title{display:flex;align-items:flex-start;justify-content:space-between}.login-title strong,.login-title text{display:block}.login-title strong{font-size:35rpx}.login-title text{margin-top:7rpx;color:#8a9389;font-size:22rpx}.login-title>i{padding:7rpx 12rpx;border-radius:8rpx;background:#eef1e4;color:#647259;font-size:18rpx;font-style:normal;font-weight:700}.wechat-button{display:grid;grid-template-columns:54rpx 1fr auto;align-items:center;gap:16rpx;width:100%;margin:30rpx 0 0;padding:20rpx 24rpx;border:0;border-radius:18rpx;background:#40543f;color:#fff;text-align:left;line-height:1.2}.wechat-button::after,.code-button::after{border:0}.wechat-button>text{display:grid;place-items:center;width:54rpx;height:54rpx;border-radius:16rpx;background:#e7edcf;color:#40543f;font-size:24rpx;font-weight:800}.wechat-button strong,.wechat-button small{display:block}.wechat-button strong{font-size:27rpx}.wechat-button small{margin-top:7rpx;color:rgba(255,255,255,.6);font-size:19rpx}.wechat-button b{font-size:38rpx;font-weight:400}.divider{position:relative;margin:34rpx 0 18rpx;color:#a0a69f;font-size:20rpx;text-align:center}
.divider::before { content: ''; position: absolute; top: 50%; left: 0; right: 0; height: 1rpx; background: #e3efeb; }
.divider text{position:relative;padding:0 18rpx;background:#fff}.temporary-form{display:grid;grid-template-columns:1.15fr .85fr;gap:12rpx}.temporary-form>view{padding:16rpx 18rpx;border:1rpx solid #e5e7df;border-radius:16rpx;background:#fafbf8}.temporary-form text{display:block;color:#727d70;font-size:19rpx}.temporary-form input{height:48rpx;margin-top:5rpx;font-size:25rpx;line-height:48rpx}.code-button{width:100%;margin:18rpx 0 0;border:0;border-radius:16rpx;background:#ecefe7;color:#40543f;font-size:25rpx;line-height:78rpx}.login-note{margin-top:28rpx;color:#9ca39a;font-size:19rpx;text-align:center}
.profile-mask{position:fixed;z-index:20;inset:0;display:flex;align-items:flex-end;padding:32rpx;background:rgba(28,39,29,.46);box-sizing:border-box}.profile-sheet{width:100%;padding:42rpx 34rpx 28rpx;border-radius:34rpx 34rpx 18rpx;background:#fff;box-sizing:border-box;text-align:center}.setup-kicker{color:#819073;font-size:18rpx;font-weight:700;letter-spacing:3rpx}.profile-sheet>strong{display:block;margin-top:12rpx;color:#304530;font-size:40rpx;letter-spacing:-2rpx}.profile-sheet>text{display:block;margin:14rpx auto 26rpx;color:#879084;font-size:23rpx;line-height:1.55}.setup-avatar{display:grid;place-items:center;width:132rpx;height:132rpx;margin:0 auto 24rpx;padding:0;overflow:hidden;border:0;border-radius:38rpx;background:#e8eedf;color:#56694e;font-size:23rpx}.setup-avatar::after,.setup-confirm::after{border:0}.setup-avatar image{width:100%;height:100%}.profile-sheet input{height:84rpx;padding:0 24rpx;border:1rpx solid #dbe5d7;border-radius:16rpx;background:#fafcf8;font-size:27rpx;text-align:left}.setup-confirm{width:100%;margin-top:18rpx;border:0;border-radius:16rpx;background:#40543f;color:#fff;font-size:28rpx;line-height:88rpx}.setup-skip{margin-top:24rpx;color:#84917e;font-size:25rpx;line-height:48rpx}
.page{padding-bottom:calc(40rpx + env(safe-area-inset-bottom))}.profile-mask{padding-bottom:calc(32rpx + env(safe-area-inset-bottom))}
</style>
