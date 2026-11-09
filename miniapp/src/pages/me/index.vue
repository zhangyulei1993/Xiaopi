<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { API_ORIGIN, request, resolveAssetUrl } from "../../api/client";
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
  wechatLogin,
} from "../../api/properties";
import type { Property, PropertyLead, RentalRequest, FavoriteContact } from "../../api/properties";
import { usePagePresentation } from "../../utils/presentation";

const role = ref<SessionRole | null>(null);
const phone = ref("");
const isBindingPhone = ref(false);
const nickname = ref("");
const savedNickname = ref("");
const avatarUrl = ref("");
const isUploadingAvatar = ref(false);
const isSavingProfile = ref(false);
const isLoggingIn = ref(false);
const adminOverview = ref({ draft: 0, pending: 0, incomplete: 0, interests: 0 });
const isLoggedIn = computed(() => role.value !== null);
const isAdmin = computed(() => role.value === "admin");
const canUseAccessCode = computed(() => role.value !== null && role.value !== "user");
const avatarDisplay = computed(() =>
  avatarUrl.value ? resolveAssetUrl(avatarUrl.value) : "",
);
const { presentation, loadPresentation, pageStyle } = usePagePresentation("me", "我的", "管理关注与找房需求");

onShow(async () => {
	uni.$emit("xiaopi-tab-active", "pages/me/index");
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
    savedNickname.value = profile.nickname || "";
    avatarUrl.value = profile.avatarUrl || "";
    phone.value = profile.phone || phone.value;
    const croppedAvatar = String(uni.getStorageSync("xiaopi-avatar-crop-result") || "");
    if (croppedAvatar) {
      uni.removeStorageSync("xiaopi-avatar-crop-result");
      uploadAvatar(croppedAvatar);
    }
    if (refreshed.role === "admin") {
      const [properties, leads, requests, interests] = await Promise.all([
        request<Property[]>("/v1/properties").catch(() => []),
        request<PropertyLead[]>("/v1/property-leads").catch(() => []),
        request<RentalRequest[]>("/v1/admin/rental-requests").catch(() => []),
        request<FavoriteContact[]>("/v1/admin/favorite-contacts").catch(() => []),
      ]);
      adminOverview.value = {
        draft: properties.filter((item) => item.status === "draft").length,
        pending: leads.filter((item) => item.status !== "converted" && item.status !== "closed").length + requests.filter((item) => !item.status || item.status === "new").length,
        incomplete: properties.filter((item) => !item.community || !item.location || !item.latitude || !item.longitude || !item.media?.length).length,
        interests: interests.length,
      };
    }
  } catch {
	role.value = getSession()?.role || null;
	if (!role.value) {
		phone.value = "";
		nickname.value = "";
		avatarUrl.value = "";
		uni.removeStorageSync("xiaopi_bound_phone");
	}
  }
});

const openAccessCodeLogin = () => uni.navigateTo({ url: "/pages/auth/login?mode=access-code" });
const login = (afterLogin?: () => void) => {
  if (isLoggingIn.value) return;
  isLoggingIn.value = true;
  uni.login({
    provider: "weixin",
    success: async (result) => {
      try {
        const session = await wechatLogin(result.code);
        saveSession(session.accessToken, session.role);
        role.value = session.role;
        uni.showToast({ title: "登录成功" });
        if (session.role === "user" && afterLogin) afterLogin();
        else setTimeout(() => uni.reLaunch({ url: "/pages/me/index" }), 250);
      } catch (error) {
        uni.showToast({ title: error instanceof Error ? error.message : "登录失败，请重试", icon: "none" });
      } finally {
        isLoggingIn.value = false;
      }
    },
    fail: () => {
      isLoggingIn.value = false;
      uni.showToast({ title: "微信登录未完成", icon: "none" });
    },
  });
};
const requireLogin = (title: string, content: string, afterLogin: () => void) => {
  if (isLoggedIn.value) { afterLogin(); return; }
  uni.showModal({
    title,
    content,
    confirmText: "微信登录",
    success: (result) => result.confirm && login(afterLogin),
  });
};
const saveNickname = async () => {
  if (!isLoggedIn.value || isSavingProfile.value) return;
  const nextNickname = nickname.value.trim();
  if (!nextNickname || nextNickname === savedNickname.value) return;
  isSavingProfile.value = true;
  try {
    const profile = await updateWechatUserProfile(nextNickname, avatarUrl.value);
    nickname.value = profile.nickname || "";
    savedNickname.value = profile.nickname || "";
    avatarUrl.value = profile.avatarUrl || "";
    uni.showToast({ title: "昵称已更新" });
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "保存失败", icon: "none" });
  } finally {
    isSavingProfile.value = false;
  }
};
const publish = () => uni.navigateTo({ url: "/pages/publish/index" });
const browse = () => uni.switchTab({ url: "/pages/properties/index" });
const requestHome = () => uni.navigateTo({ url: "/pages/requests/index" });
const openRequestHistory = () => requireLogin("登录后查看找房记录", "预约、订阅和找房需求会同步到你的微信账号", () => uni.navigateTo({ url: "/pages/requests/history" }));
const openFavorites = () => {
  uni.setStorageSync("xiaopi_show_favorites", true);
  browse();
};
const favorites = () => requireLogin("登录后查看关注", "关注记录会同步到你的微信账号", openFavorites);
const openFootprints = () => uni.navigateTo({ url: "/pages/footprints/index" });
const footprints = () => requireLogin("登录后查看足迹", "登录后可查看最近浏览过的房源", openFootprints);
const workbench = () => uni.navigateTo({ url: "/pages/admin/index" });
const openAdminTab = (tab: "properties" | "leads" | "requests" | "interests") => {
  uni.setStorageSync("xiaopi_admin_tab", tab);
  workbench();
};
const editProperty = () =>
  uni.navigateTo({ url: "/pages/admin/property-edit" });

const uploadAvatar = (path: string) => {
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
          const nextAvatar = String(body.data?.url || "");
          updateWechatUserProfile(nickname.value.trim(), nextAvatar)
            .then((profile) => {
              avatarUrl.value = profile.avatarUrl || nextAvatar;
              nickname.value = profile.nickname || nickname.value;
              savedNickname.value = profile.nickname || savedNickname.value;
              uni.showToast({ title: "头像已更新" });
            })
            .catch((error) => uni.showToast({ title: error instanceof Error ? error.message : "头像保存失败", icon: "none" }));
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
const chooseAvatar = (event: { detail: { avatarUrl?: string } }) => {
  const path = event.detail.avatarUrl;
  if (!path || isUploadingAvatar.value) return;
  uni.getImageInfo({
    src: path,
    success: (image) => {
      if (image.width === image.height) {
        uploadAvatar(path);
        return;
      }
      uni.setStorageSync("xiaopi-avatar-crop-source", { path, width: image.width, height: image.height });
      uni.navigateTo({ url: "/pages/auth/avatar-crop" });
    },
    fail: () => uni.showToast({ title: "头像读取失败", icon: "none" }),
  });
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
    <view v-if="isAdmin" class="me-head"><view><text>个人中心</text><view>{{ presentation.title }}</view></view><text class="me-head-note">运营管理与待办事项</text></view>
    <view v-else class="me-head"><view><text>个人中心</text><view>{{ presentation.title }}</view></view><text class="me-head-note">{{ presentation.subtitle }}</text></view>

    <view v-if="!isAdmin" class="profile" :class="{ guest: !isLoggedIn }" @click="!isLoggedIn && login()">
      <view class="profile-glow"></view><view class="profile-top"><button v-if="isLoggedIn" class="avatar avatar-button" open-type="chooseAvatar" :loading="isUploadingAvatar" :disabled="isUploadingAvatar" @chooseavatar="chooseAvatar"><image v-if="avatarDisplay" :src="avatarDisplay" mode="aspectFill" /><text v-else>{{ isAdmin ? "管" : "小" }}</text></button><view v-else class="avatar"><text>{{ isAdmin ? "管" : "小" }}</text></view><view class="profile-copy"><view class="profile-title"><input v-if="isLoggedIn" v-model="nickname" class="nickname-input" type="nickname" maxlength="32" placeholder="点击填写微信昵称" confirm-type="done" @confirm="saveNickname" @blur="saveNickname" /><view v-else class="title">登录后开始找房</view><text v-if="isLoggedIn" class="role-badge">{{ isAdmin ? "管理员" : "普通用户" }}</text></view><view class="profile-sub">{{ isAdmin ? "房源运营权限已启用" : isLoggedIn ? (phone ? `手机号 ${phone.slice(0, 3)}****${phone.slice(-4)}` : "微信账号已登录") : "同步收藏、足迹和找房需求" }}</view></view><text class="profile-arrow">{{ isLoggedIn ? "" : "›" }}</text></view>
      <view class="profile-bottom"><text>{{ isLoggedIn ? (phone ? "点头像或昵称即可修改资料" : "绑定手机号，方便联系与身份识别") : (isLoggingIn ? "正在登录…" : "点击直接微信登录") }}</text><button v-if="isLoggedIn && !phone" open-type="getPhoneNumber" :loading="isBindingPhone" :disabled="isBindingPhone" @getphonenumber.stop="bindPhone">绑定手机号</button><text v-else class="profile-action">{{ isLoggedIn ? "已登录" : "进入" }}</text></view>
    </view>

    <view v-if="!isAdmin" class="shortcut-section"><view class="section-heading"><strong>我的找房</strong><text>常用服务</text></view><view class="shortcut-grid"><view @click="browse"><i class="shortcut-icon home"></i><strong>全部房源</strong><text>无需登录</text></view><view @click="favorites"><i class="shortcut-icon favorite"></i><strong>我的关注</strong><text>{{ isLoggedIn ? "收藏的房源" : "登录后查看" }}</text></view><view @click="footprints"><i class="shortcut-icon footprint"></i><strong>浏览足迹</strong><text>{{ isLoggedIn ? "最近看过" : "登录后查看" }}</text></view><view @click="requestHome"><i class="shortcut-icon request"></i><strong>找房需求</strong><text>提交时登录</text></view></view>
    </view>
    <view v-if="!isAdmin" class="card secondary-entry request-history-entry" @click="openRequestHistory"><view><strong>我的预约与找房记录</strong><text>{{ isLoggedIn ? "查看预约、订阅和跟进进度" : "登录后可查看" }}</text></view><text>›</text></view>
    <view v-if="!isAdmin" class="card secondary-entry" @click="publish"><view><strong>有房源想推荐？</strong><text>无需登录，提交后电话核验</text></view><text>›</text></view>

    <view v-if="isAdmin" class="admin-identity">
      <button class="admin-avatar avatar-button" open-type="chooseAvatar" :loading="isUploadingAvatar" :disabled="isUploadingAvatar" @chooseavatar="chooseAvatar"><image v-if="avatarDisplay" :src="avatarDisplay" mode="aspectFill" /><text v-else>管</text></button>
      <view class="admin-identity-copy"><view><input v-model="nickname" class="admin-name" type="nickname" maxlength="32" placeholder="填写昵称" confirm-type="done" @confirm="saveNickname" @blur="saveNickname" /><text class="admin-role">管理员</text></view><small>{{ phone ? `已绑定 ${phone.slice(0, 3)}****${phone.slice(-4)}` : "运营身份已启用" }}</small></view>
      <text class="admin-edit">可编辑</text>
    </view>
    <view v-if="isAdmin" class="admin-overview">
      <view class="section-heading"><strong>运营工作台</strong><text @click="workbench">查看全部 ›</text></view>
      <view class="admin-summary"><view @click="openAdminTab('properties')"><strong>{{ adminOverview.draft }}</strong><text>房源草稿</text></view><view @click="openAdminTab('requests')"><strong>{{ adminOverview.pending }}</strong><text>待联系</text></view><view @click="openAdminTab('properties')"><strong>{{ adminOverview.incomplete }}</strong><text>待补资料</text></view></view>
    </view>
    <view v-if="isAdmin" class="admin-actions">
      <view @click="editProperty"><view class="action-icon publish-icon">＋</view><strong>录入房源</strong><text>发布可租房源</text></view>
      <view @click="openAdminTab('properties')"><view class="action-icon workspace-icon">▦</view><strong>房源管理</strong><text>查看和编辑房源</text></view>
      <view @click="openAdminTab('requests')"><view class="action-icon request-icon">◌</view><strong>客户需求</strong><text>预约与找房需求</text></view>
      <view @click="openAdminTab('interests')"><view class="action-icon interest-icon">♡</view><strong>用户意向</strong><text>{{ adminOverview.interests }} 条关注记录</text></view>
    </view>

    <view v-if="isLoggedIn" class="session-actions"><text v-if="canUseAccessCode" class="role-access-code" @click="openAccessCodeLogin">访问码登录</text><button class="logout" @click="logout">退出登录</button></view>
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
.avatar-button{margin:0;padding:0;overflow:hidden;border:0;box-sizing:border-box}.avatar-button::after{border:0}.avatar-button image{display:block;width:100%;height:100%;border-radius:inherit}.nickname-input{width:100%;height:48rpx;padding:0;border:0;background:transparent;color:#fff;font-size:34rpx;font-weight:700;line-height:48rpx;letter-spacing:-1rpx}.nickname-input::placeholder{color:rgba(255,255,255,.8);font-weight:500}
.avatar image { width:100%;height:100%;border-radius:inherit }.profile .title{color:#fff;font-size:34rpx;line-height:1.25}.profile-sub{margin-top:8rpx;color:rgba(255,255,255,.68);font-size:23rpx}.profile-arrow{font-size:48rpx}
.arrow {
  color: #9bbab4;
  font-size: 54rpx;
}
.shortcut-section{margin:34rpx 0 24rpx}.section-heading{display:flex;align-items:baseline;justify-content:space-between;margin:0 4rpx 18rpx}.section-heading strong{font-size:32rpx}.section-heading text{color:#929b91;font-size:21rpx}.shortcut-grid{display:grid;grid-template-columns:1fr 1fr;gap:14rpx}.shortcut-grid>view{display:grid;grid-template-columns:64rpx 1fr;grid-template-rows:auto auto;column-gap:18rpx;padding:24rpx;border-radius:22rpx;background:#fff}.shortcut-grid>view:active{transform:scale(.98);background:#f0f2e9}.shortcut-grid i{grid-row:1/3;position:relative;display:grid;place-items:center;width:64rpx;height:64rpx;border-radius:18rpx 18rpx 8rpx;background:#e9eedc;color:#4d623e;font-style:normal}.shortcut-icon::before{font-size:30rpx;font-weight:500;line-height:1}.shortcut-icon.home::before{content:"⌂"}.shortcut-icon.favorite::before{content:"♡";font-size:38rpx}.shortcut-icon.footprint::before{content:"◴"}.shortcut-icon.request::before{content:"✦"}.shortcut-grid strong{font-size:27rpx;line-height:1.25}.shortcut-grid text{margin-top:6rpx;color:#899287;font-size:21rpx;line-height:1.35}.secondary-entry{display:flex;align-items:center;justify-content:space-between;border:0;background:#efeee7}.secondary-entry view{display:flex;flex-direction:column;gap:7rpx}.secondary-entry strong{font-size:27rpx}.secondary-entry view text{color:#899287;font-size:21rpx}.secondary-entry>text{color:#74806f;font-size:40rpx}
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
  color: #7e887d;
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
  background: #40543f;
  color: #fff;
  font-size: 24rpx;
  line-height: 68rpx;
}
.phone-card button::after {
  border: 0;
}
.bound {
  flex: 0 0 auto;
  color: #40543f;
  font-size: 24rpx;
}

.logout{margin-top:24rpx;padding-bottom:14rpx;color:#a2695b}
.profile-copy{min-width:0}.profile .title{line-height:1.3}.role-badge{line-height:1.35}.menu .group-title{padding-bottom:10rpx;font-size:22rpx;letter-spacing:.5rpx}.menu view:not(.group-title){font-size:29rpx;line-height:1.35}.phone-title{font-size:29rpx;line-height:1.3}.phone-card .muted{margin-top:7rpx;line-height:1.5}.logout{font-size:27rpx}.session-actions{display:flex;align-items:center;justify-content:center;gap:28rpx;margin-top:24rpx}.session-actions .logout{width:auto;margin:0;padding:0 12rpx}.role-access-code{padding:8rpx 12rpx;color:#8b9488;font-size:20rpx;line-height:44rpx}

/* The admin state deliberately keeps the same "我的" rhythm as a tenant,
   while surfacing the workspace queues instead of hiding them behind a hero. */
.admin-identity{display:flex;align-items:center;gap:18rpx;margin-bottom:26rpx;padding:24rpx;border-radius:24rpx;background:#40543f;color:#fff}.admin-avatar{width:88rpx;height:88rpx;min-width:88rpx;flex:none;border-radius:24rpx;background:#e7edcf;color:#40543f;font-size:38rpx;line-height:88rpx}.admin-identity-copy{min-width:0;flex:1}.admin-identity-copy>view{display:flex;align-items:center;gap:10rpx}.admin-name{width:220rpx;height:42rpx;padding:0;border:0;background:transparent;color:#fff;font-size:30rpx;font-weight:750;line-height:42rpx}.admin-name::placeholder{color:rgba(255,255,255,.7)}.admin-role{padding:5rpx 10rpx;border-radius:8rpx;background:rgba(231,237,207,.16);color:#e7edcf;font-size:18rpx;font-weight:700}.admin-identity-copy small{display:block;overflow:hidden;margin-top:7rpx;color:rgba(255,255,255,.66);font-size:20rpx;text-overflow:ellipsis;white-space:nowrap}.admin-edit{flex:none;color:#e7edcf;font-size:19rpx}
.admin-overview{margin:0 0 22rpx}.admin-summary{display:grid;grid-template-columns:repeat(3,1fr);gap:12rpx}.admin-summary>view{padding:22rpx 14rpx;border-radius:18rpx;background:#fff;text-align:center;box-shadow:0 8rpx 22rpx rgba(55,72,51,.05)}.admin-summary strong,.admin-summary text{display:block}.admin-summary strong{color:#40543f;font-size:38rpx;line-height:1.1}.admin-summary text{margin-top:8rpx;color:#7c887a;font-size:20rpx;line-height:1.2}
.admin-actions{display:grid;grid-template-columns:1fr 1fr;gap:14rpx}.admin-actions>view{position:relative;min-height:136rpx;padding:22rpx;box-sizing:border-box;border-radius:20rpx;background:#fff}.admin-actions>view:active,.admin-summary>view:active{transform:scale(.98);background:#f1f4e9}.action-icon{display:grid;width:48rpx;height:48rpx;place-items:center;border-radius:14rpx;background:#e9eedc;color:#40543f;font-size:28rpx;font-weight:700}.workspace-icon{background:#e7efe9;color:#4f715e}.request-icon{background:#f8ede2;color:#b86b4e}.interest-icon{background:#f7e9eb;color:#bd6772}.admin-actions strong,.admin-actions text{display:block}.admin-actions strong{margin-top:14rpx;color:#354535;font-size:26rpx;line-height:1.25}.admin-actions text{margin-top:6rpx;color:#899287;font-size:19rpx;line-height:1.35}
.avatar{color:#354b35}.page{padding-bottom:calc(156rpx + env(safe-area-inset-bottom))}.logout:active{opacity:.72}.admin-actions>view,.shortcut-grid>view,.secondary-entry{border:1rpx solid #ebe8e1}
</style>
