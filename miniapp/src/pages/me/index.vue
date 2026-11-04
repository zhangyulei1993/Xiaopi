<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { API_ORIGIN, resolveAssetUrl } from "../../api/client";
import {
  clearSession,
  getSession,
  saveSession,
  type SessionRole,
} from "../../auth/session";
import {
  bindWechatPhone,
  getWechatUserProfile,
  refreshAuth,
  updateWechatUserProfile,
} from "../../api/properties";
import { usePagePresentation } from "../../utils/presentation";

const role = ref<SessionRole | null>(null);
const phone = ref("");
const isBindingPhone = ref(false);
const nickname = ref("");
const avatarUrl = ref("");
const isUploadingAvatar = ref(false);
const isSavingProfile = ref(false);
const isLoggedIn = computed(() => role.value !== null);
const isAdmin = computed(() => role.value === "admin");
const avatarDisplay = computed(() =>
  avatarUrl.value ? resolveAssetUrl(avatarUrl.value) : "",
);
const { presentation, loadPresentation, pageStyle } = usePagePresentation("me", "我的", "管理关注与找房需求");

onShow(async () => {
  await loadPresentation();
  const session = getSession();
  role.value = session?.role || null;
  phone.value = String(uni.getStorageSync("xiaopi_bound_phone") || "");
  if (!session) return;
  try {
    const refreshed = await refreshAuth();
    saveSession(refreshed.accessToken, refreshed.role);
    role.value = refreshed.role;
    const profile = await getWechatUserProfile();
    nickname.value = profile.nickname || "";
    avatarUrl.value = profile.avatarUrl || "";
    phone.value = profile.phone || phone.value;
  } catch {
    role.value = getSession()?.role || null;
  }
});

const login = () => uni.navigateTo({ url: "/pages/auth/login" });
const publish = () => uni.navigateTo({ url: "/pages/publish/index" });
const browse = () => uni.switchTab({ url: "/pages/properties/index" });
const requestHome = () => uni.navigateTo({ url: "/pages/requests/index" });
const favorites = () => {
  uni.setStorageSync("xiaopi_show_favorites", true);
  browse();
};
const footprints = () => uni.navigateTo({ url: "/pages/footprints/index" });
const workbench = () => uni.navigateTo({ url: "/pages/admin/index" });
const editProperty = () =>
  uni.navigateTo({ url: "/pages/admin/property-edit" });

const chooseAvatar = (event: { detail: { avatarUrl?: string } }) => {
  const path = event.detail.avatarUrl;
  if (!path || isUploadingAvatar.value) return;
  isUploadingAvatar.value = true;
  uni.uploadFile({
    url: `${API_ORIGIN}/api/v1/auth/avatar`,
    filePath: path,
    name: "file",
    header: { Authorization: `Bearer ${getSession()?.accessToken || ""}` },
    success: (response) => {
      try {
        const body = JSON.parse(String(response.data || "{}"));
        if (response.statusCode >= 200 && response.statusCode < 300 && body?.code === 0) {
          avatarUrl.value = String(body.data?.url || "");
          uni.showToast({ title: "头像已选择" });
          return;
        }
        uni.showToast({ title: body?.message || "头像上传失败", icon: "none" });
      } catch {
        uni.showToast({ title: "头像上传失败", icon: "none" });
      }
    },
    fail: () => uni.showToast({ title: "头像上传失败", icon: "none" }),
    complete: () => (isUploadingAvatar.value = false),
  });
};

const saveProfile = async () => {
  if (isSavingProfile.value || (!nickname.value.trim() && !avatarUrl.value)) return;
  isSavingProfile.value = true;
  try {
    const profile = await updateWechatUserProfile(
      nickname.value.trim(),
      avatarUrl.value,
    );
    nickname.value = profile.nickname || "";
    avatarUrl.value = profile.avatarUrl || "";
    uni.showToast({ title: "资料已保存" });
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "保存失败",
      icon: "none",
    });
  } finally {
    isSavingProfile.value = false;
  }
};

const bindPhone = async (event: {
  detail: { code?: string; errMsg?: string };
}) => {
  const code = event.detail.code;
  if (!code || isBindingPhone.value) {
    if (event.detail.errMsg?.includes("deny"))
      uni.showToast({ title: "需要授权手机号才能绑定", icon: "none" });
    return;
  }
  isBindingPhone.value = true;
  try {
    const result = await bindWechatPhone(code);
    phone.value = result.phone;
    uni.setStorageSync("xiaopi_bound_phone", result.phone);
    const refreshed = await refreshAuth();
    saveSession(refreshed.accessToken, refreshed.role);
    role.value = refreshed.role;
    uni.showToast({
      title: refreshed.role === "admin" ? "已进入管理员身份" : "手机号绑定成功",
    });
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "手机号绑定失败",
      icon: "none",
    });
  } finally {
    isBindingPhone.value = false;
  }
};

const logout = () => {
  uni.showModal({
    title: "退出登录",
    content: "退出后需要重新登录才能使用身份服务。",
    success: (res) => {
      if (!res.confirm) return;
      clearSession();
      uni.removeStorageSync("xiaopi_bound_phone");
      role.value = null;
      phone.value = "";
      uni.showToast({ title: "已退出登录" });
    },
  });
};
</script>

<template>
  <view class="page" :style="pageStyle">
    <view class="me-head"><view><text>PERSONAL SPACE</text><view>{{ presentation.title }}</view></view><text class="me-head-note">{{ isAdmin ? "运营工作台" : "收藏、足迹与找房进度" }}</text></view>
    <view class="profile" :class="{ guest: !isLoggedIn }" @click="!isLoggedIn && login()">
      <view class="profile-glow"></view><view class="profile-top"><view class="avatar"><image v-if="avatarDisplay" :src="avatarDisplay" mode="aspectFill" /><text v-else>{{ isAdmin ? "管" : "小" }}</text></view><view class="profile-copy"><view class="profile-title"><view class="title">{{ isAdmin ? nickname || "小皮管理员" : isLoggedIn ? nickname || "微信用户" : "登录后开始找房" }}</view><text v-if="isLoggedIn" class="role-badge">{{ isAdmin ? "管理员" : "普通用户" }}</text></view><view class="profile-sub">{{ isAdmin ? "房源运营权限已启用" : isLoggedIn ? (phone ? `手机号 ${phone.slice(0, 3)}****${phone.slice(-4)}` : "微信账号已登录") : "同步收藏、足迹和找房需求" }}</view></view><text class="profile-arrow">{{ isLoggedIn ? "" : "›" }}</text></view>
      <view class="profile-bottom"><text>{{ isLoggedIn ? (phone ? "账号资料已完善" : "绑定手机号，方便联系与身份识别") : "微信一键登录" }}</text><button v-if="isLoggedIn && !phone" open-type="getPhoneNumber" :loading="isBindingPhone" :disabled="isBindingPhone" @getphonenumber.stop="bindPhone">绑定手机号</button><text v-else class="profile-action">{{ isLoggedIn ? "已登录" : "进入" }}</text></view>
    </view>

    <view v-if="isLoggedIn" class="card profile-editor">
      <view class="profile-editor-title"><text>个人资料</text><small>头像和昵称</small></view>
      <view class="profile-fields">
        <button class="avatar-picker" open-type="chooseAvatar" :loading="isUploadingAvatar" @chooseavatar="chooseAvatar">
          <image v-if="avatarDisplay" :src="avatarDisplay" mode="aspectFill" /><text v-else>选择头像</text>
        </button>
        <input v-model="nickname" type="nickname" maxlength="32" placeholder="填写微信昵称" />
        <button class="profile-save" :loading="isSavingProfile" :disabled="isSavingProfile || (!nickname.trim() && !avatarUrl)" @click="saveProfile">保存</button>
      </view>
    </view>

    <view v-if="!isAdmin" class="shortcut-section"><view class="section-heading"><strong>我的找房</strong><text>常用服务</text></view><view class="shortcut-grid"><view @click="browse"><i>房</i><strong>全部房源</strong><text>继续看看好房</text></view><view @click="favorites"><i>心</i><strong>我的关注</strong><text>收藏的房源</text></view><view v-if="isLoggedIn" @click="footprints"><i>迹</i><strong>浏览足迹</strong><text>最近看过</text></view><view @click="requestHome"><i>需</i><strong>找房需求</strong><text>告诉我们想法</text></view></view>
    </view>
    <view v-if="!isAdmin" class="card secondary-entry" @click="publish"><view><strong>有房源想推荐？</strong><text>提交房源线索</text></view><text>›</text></view>

    <view v-if="isAdmin" class="card menu">
      <view class="group-title">运营管理</view>
      <view @click="workbench">管理员工作台 <text>›</text></view>
      <view @click="editProperty">录入正式房源 <text>›</text></view>
    </view>

    <button v-if="isLoggedIn" class="logout" @click="logout">退出登录</button>
  </view>
</template>

<style scoped>
.page{padding-top:24rpx;padding-bottom:80rpx}.me-head{display:flex;align-items:flex-end;justify-content:space-between;margin:8rpx 4rpx 26rpx}.me-head>view>text{display:block;color:#8e988b;font-size:18rpx;font-weight:700;letter-spacing:3rpx}.me-head>view>view{margin-top:6rpx;font-size:48rpx;font-weight:850;letter-spacing:-3rpx}.me-head-note{padding-bottom:6rpx;color:#7b877a;font-size:22rpx}.profile{position:relative;overflow:hidden;margin-bottom:24rpx;padding:34rpx 32rpx 24rpx;border-radius:32rpx 32rpx 16rpx;background:#354b35;color:#fff;box-shadow:0 18rpx 42rpx rgba(52,75,53,.18)}.profile-glow{position:absolute;right:-80rpx;top:-110rpx;width:300rpx;height:300rpx;border-radius:50%;background:rgba(222,232,193,.14)}.profile-top{position:relative;display:flex;align-items:center;gap:24rpx}.profile-bottom{position:relative;display:flex;align-items:center;justify-content:space-between;margin-top:26rpx;padding-top:22rpx;border-top:1rpx solid rgba(255,255,255,.14);color:rgba(255,255,255,.72);font-size:22rpx}.profile-bottom button{margin:0;padding:0 22rpx;border:0;border-radius:12rpx;background:#e7edcf;color:#354b35;font-size:22rpx;line-height:58rpx}.profile-bottom button::after{border:0}.profile-action{color:#e7edcf;font-weight:700}.profile.guest{background:#4d5e3e}.profile.guest:active{transform:scale(.985)}
.profile-copy {
  flex: 1;
  min-width:0;
}
.profile-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12rpx;
}
.role-badge {
  padding: 6rpx 12rpx;border-radius:8rpx;background:rgba(231,237,207,.14);color:#e7edcf;
  font-size: 20rpx;
  font-weight: 600;
}
.avatar {
  width: 104rpx;height:104rpx;border-radius:28rpx 28rpx 12rpx;background:#e7edcf;color:#354b35;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 48rpx;
  font-weight: 700;
}
.avatar image { width:100%;height:100%;border-radius:inherit }.profile .title{color:#fff;font-size:34rpx;line-height:1.25}.profile-sub{margin-top:8rpx;color:rgba(255,255,255,.62);font-size:23rpx}.profile-arrow{font-size:48rpx}.profile-editor{padding:26rpx;border:0}.profile-editor-title{display:flex;align-items:baseline;justify-content:space-between;color:#304530;font-size:28rpx;font-weight:700}.profile-editor-title small{color:#929b91;font-size:21rpx;font-weight:400}
.profile-fields { display: grid; grid-template-columns: 112rpx 1fr 112rpx; align-items: center; gap: 18rpx; margin-top: 24rpx; }
.avatar-picker { width: 112rpx; height: 112rpx; margin: 0; padding: 0; border: 0; border-radius: 50%; background: #edf3e9; color: #52634f; font-size: 21rpx; line-height: 112rpx; overflow: hidden; }
.avatar-picker::after,.profile-save::after { border: 0; }
.avatar-picker image { width: 100%; height: 100%; display: block; }
.profile-fields input { height: 80rpx; padding: 0 22rpx; border: 1rpx solid #dfe8dc; border-radius: 16rpx; background: #f8faf6; font-size: 27rpx; }
.profile-save { width: 112rpx; height: 72rpx; margin: 0; padding: 0; border: 0; border-radius: 16rpx; background: #536a45; color: #fff; font-size: 25rpx; line-height: 72rpx; }
.arrow {
  color: #9bbab4;
  font-size: 54rpx;
}
.shortcut-section{margin:34rpx 0 24rpx}.section-heading{display:flex;align-items:baseline;justify-content:space-between;margin:0 4rpx 18rpx}.section-heading strong{font-size:32rpx}.section-heading text{color:#929b91;font-size:21rpx}.shortcut-grid{display:grid;grid-template-columns:1fr 1fr;gap:14rpx}.shortcut-grid>view{display:grid;grid-template-columns:64rpx 1fr;grid-template-rows:auto auto;column-gap:18rpx;padding:24rpx;border-radius:22rpx;background:#fff}.shortcut-grid>view:active{transform:scale(.98);background:#f0f2e9}.shortcut-grid i{grid-row:1/3;display:grid;place-items:center;width:64rpx;height:64rpx;border-radius:18rpx 18rpx 8rpx;background:#e9eedc;color:#4d623e;font-style:normal;font-size:24rpx;font-weight:800}.shortcut-grid strong{font-size:27rpx;line-height:1.25}.shortcut-grid text{margin-top:6rpx;color:#929b91;font-size:20rpx}.secondary-entry{display:flex;align-items:center;justify-content:space-between;border:0;background:#efeee7}.secondary-entry view{display:flex;flex-direction:column;gap:7rpx}.secondary-entry strong{font-size:27rpx}.secondary-entry view text{color:#899287;font-size:21rpx}.secondary-entry>text{color:#74806f;font-size:40rpx}
.menu view:last-child {
  border-bottom: 0;
}
.menu text {
  float: right;
  color: #9bbab4;
  font-size: 40rpx;
}
.menu .group-title {
  padding-top: 4rpx;
  color: #78a39d;
  font-size: 24rpx;
}
.logout {
  margin-top: 36rpx;
  border: 0;
  background: transparent;
  color: #d24b61;
  font-size: 28rpx;
}
.logout::after {
  border: 0;
}
.phone-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24rpx;
}
.phone-title {
  margin-bottom: 8rpx;
  color: #123e3c;
  font-size: 28rpx;
  font-weight: 600;
}
.phone-card button {
  flex: 0 0 auto;
  margin: 0;
  padding: 0 24rpx;
  background: #0f8177;
  color: #fff;
  font-size: 24rpx;
  line-height: 68rpx;
}
.phone-card button::after {
  border: 0;
}
.bound {
  flex: 0 0 auto;
  color: #0f8177;
  font-size: 24rpx;
}

.logout{margin-top:24rpx;padding-bottom:14rpx;color:#a2695b}
.profile-copy{min-width:0}.profile .title{line-height:1.3}.role-badge{line-height:1.35}.menu .group-title{padding-bottom:10rpx;font-size:22rpx;letter-spacing:.5rpx}.menu view:not(.group-title){font-size:29rpx;line-height:1.35}.phone-title{font-size:29rpx;line-height:1.3}.phone-card .muted{margin-top:7rpx;line-height:1.5}.logout{font-size:27rpx}
</style>
