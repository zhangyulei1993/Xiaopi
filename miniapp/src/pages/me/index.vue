<script setup lang="ts">
import { refreshCurrentSession } from "../../auth/refresh-session";
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { API_ORIGIN, safeUploadUrl, resolveAssetUrl } from "../../api/client";
import {
  clearSession,
  getSession,
  saveSession,
  type SessionRole,
} from "../../auth/session";
import {
  bindWechatPhone,
  getWechatUserProfile,
  updateWechatUserProfile,
  wechatLogin,
} from "../../api/properties";
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
const isLoggedIn = computed(() => role.value !== null);
const isAdmin = computed(() => role.value === "admin");
const canUseAccessCode = computed(() => role.value !== null && role.value !== "user");
const avatarDisplay = computed(() =>
  avatarUrl.value ? resolveAssetUrl(avatarUrl.value) : "",
);
const { presentation, loadPresentation, pageStyle } = usePagePresentation("me", "我的", "管理关注与找房需求");

onShow(async () => {
  void loadPresentation();
  const session = getSession();
  role.value = session?.role || null;
  phone.value = "";
  nickname.value = "";
  savedNickname.value = "";
  avatarUrl.value = "";
  if (!session) return;
  try {
    const refreshed = await refreshCurrentSession();
    role.value = refreshed.role;
    const profile = await getWechatUserProfile();
    if (getSession()?.accessToken !== refreshed.accessToken) return;
    nickname.value = profile.nickname || "";
    savedNickname.value = profile.nickname || "";
    avatarUrl.value = profile.avatarUrl || "";
    phone.value = profile.phone || "";
    const croppedAvatar = String(uni.getStorageSync("xiaopi-avatar-crop-result") || "");
    if (croppedAvatar) {
      uni.removeStorageSync("xiaopi-avatar-crop-result");
      uploadAvatar(croppedAvatar);
    }
  } catch {
	role.value = getSession()?.role || null;
	if (!role.value) {
		phone.value = "";
		nickname.value = "";
		avatarUrl.value = "";
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
        const sessionRole = session.role === "admin" ? "admin" : "user";
        saveSession(session.accessToken, sessionRole);
        role.value = sessionRole;
        uni.showToast({ title: "登录成功" });
        if (sessionRole === "user" && afterLogin) afterLogin();
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
const openRequestHistory = () => requireLogin(copyText("登录后查看找房记录"), copyText("预约、订阅和找房需求会同步到你的微信账号"), () => uni.navigateTo({ url: "/pages/requests/history" }));
const openFavorites = () => {
  uni.setStorageSync("xiaopi_show_favorites", true);
  browse();
};
const favorites = () => requireLogin(copyText("登录后查看关注"), copyText("关注记录会同步到你的微信账号"), openFavorites);
const openFootprints = () => uni.navigateTo({ url: "/pages/footprints/index" });
const footprints = () => requireLogin(copyText("登录后查看足迹"), copyText("登录后可查看最近浏览过的房源"), openFootprints);
const workbench = () => uni.navigateTo({ url: "/pages/admin/index" });
const openPropertyManagement = (filter = "") => {
  uni.setStorageSync("xiaopi_admin_property_status", filter || "all");
  uni.switchTab({ url: "/pages/properties/index" });
};
const openMaterialLibrary = () =>
  uni.navigateTo({ url: "/pages/admin/material-library" });

const uploadAvatar = (path: string) => {
  if (!path || isUploadingAvatar.value) return;
  isUploadingAvatar.value = true;
  uni.uploadFile({
    url: safeUploadUrl(`${API_ORIGIN}/api/v1/auth/avatar`),
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
    const token = getSession()?.accessToken;
    const result = await bindWechatPhone(code);
    if (getSession()?.accessToken !== token) throw new Error("登录账号已变化，请重试");
    phone.value = result.phone;
    const refreshed = await refreshCurrentSession();
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
      role.value = null;
      phone.value = "";
      uni.showToast({ title: "已退出登录" });
    },
  });
};

function copyText(fallback: string) { return presentation.value?.copy?.[fallback] || fallback }
</script>

<template>
  <view class="page" :style="pageStyle">
    <view v-if="isAdmin" class="me-head"><view><text>{{ presentation.eyebrow || "个人中心" }}</text><view>{{ presentation.title }}</view></view><text class="me-head-note">{{ presentation.operatorSubtitle || "运营管理与待办事项" }}</text></view>
    <view v-else class="me-head"><view><text>{{ presentation.eyebrow || "个人中心" }}</text><view>{{ presentation.title }}</view></view><text class="me-head-note">{{ presentation.subtitle }}</text></view>

    <view v-if="!isAdmin" class="profile" :class="{ guest: !isLoggedIn }">
      <view class="profile-glow"></view>
      <view class="profile-top">
        <button v-if="isLoggedIn" class="avatar avatar-button" open-type="chooseAvatar" :loading="isUploadingAvatar" :disabled="isUploadingAvatar" @chooseavatar="chooseAvatar"><image v-if="avatarDisplay" :src="avatarDisplay" mode="aspectFill" /><text v-else>小</text></button>
        <view v-else class="avatar"><text>访</text></view>
        <view class="profile-copy">
          <view class="profile-title"><input v-if="isLoggedIn" v-model="nickname" class="nickname-input" type="nickname" maxlength="32" placeholder="点击填写微信昵称" confirm-type="done" @confirm="saveNickname" @blur="saveNickname" /><view v-else class="title">{{ copyText("登录后开始找房") }}</view><text v-if="isLoggedIn" class="role-badge">普通用户</text><text v-else class="guest-badge">访客模式</text></view>
          <view class="profile-sub">{{ isLoggedIn ? (phone ? `手机号 ${phone.slice(0, 3)}****${phone.slice(-4)}` : "微信账号已登录") : copyText("同步收藏、足迹和找房需求") }}</view>
        </view>
      </view>
      <view v-if="isLoggedIn" class="profile-bottom"><text>{{ phone ? "点头像或昵称即可修改资料" : "绑定手机号，方便联系与身份识别" }}</text><button v-if="!phone" open-type="getPhoneNumber" :loading="isBindingPhone" :disabled="isBindingPhone" @getphonenumber.stop="bindPhone">绑定手机号</button><text v-else class="profile-action">已登录</text></view>
      <view v-else class="guest-actions"><text>先浏览房源；登录后可同步关注、足迹和需求</text><view><button class="guest-browse" @click.stop="browse">先看房源</button><button class="guest-login" :loading="isLoggingIn" :disabled="isLoggingIn" @click.stop="login()">微信登录</button></view><text class="test-access-entry" @click.stop="openAccessCodeLogin">访问码登录</text></view>
    </view>

    <view v-if="!isAdmin" class="shortcut-section"><view class="section-heading"><strong>{{ copyText("我的找房") }}</strong><text>关注、记录与需求</text></view><view class="shortcut-grid"><view @click="favorites"><i class="shortcut-icon favorite"></i><view class="shortcut-copy"><strong>我的关注</strong><text>{{ isLoggedIn ? "收藏的房源" : copyText("登录后查看") }}</text></view></view><view @click="footprints"><i class="shortcut-icon footprint"></i><view class="shortcut-copy"><strong>浏览足迹</strong><text>{{ isLoggedIn ? "最近看过" : copyText("登录后查看") }}</text></view></view><view @click="requestHome"><i class="shortcut-icon request"></i><view class="shortcut-copy"><strong>找房需求</strong><text>{{ copyText("提交时登录") }}</text></view></view><view @click="openRequestHistory"><i class="shortcut-icon history"></i><view class="shortcut-copy"><strong>预约与需求记录</strong><text>{{ isLoggedIn ? "查看预约和跟进进度" : copyText("登录后查看") }}</text></view></view></view>
    </view>
    <view v-if="!isAdmin" class="card secondary-entry" @click="publish"><view><strong>{{ copyText("有房源想推荐？") }}</strong><text>{{ copyText("无需登录") }}，提交后电话核验</text></view><text>›</text></view>

    <view v-if="isAdmin" class="admin-identity">
      <button class="admin-avatar avatar-button" open-type="chooseAvatar" :loading="isUploadingAvatar" :disabled="isUploadingAvatar" @chooseavatar="chooseAvatar"><image v-if="avatarDisplay" :src="avatarDisplay" mode="aspectFill" /><text v-else>管</text></button>
      <view class="admin-identity-copy"><view><input v-model="nickname" class="admin-name" type="nickname" maxlength="32" placeholder="填写昵称" confirm-type="done" @confirm="saveNickname" @blur="saveNickname" /><text class="admin-role">管理员</text></view><small>{{ phone ? `已绑定 ${phone.slice(0, 3)}****${phone.slice(-4)}` : "运营身份已启用" }}</small></view>
      <text class="admin-edit">可编辑</text>
    </view>
    <view v-if="isAdmin" class="admin-actions">
      <view class="admin-entry-group property-entry"><view class="admin-entry-heading"><view class="action-icon workspace-icon">▦</view><view><strong>房源管理</strong><text>浏览、筛选和编辑房源</text></view></view><view class="admin-entry-links single"><button @click="openPropertyManagement()"><text>▤</text>房源列表<text class="entry-arrow">›</text></button></view></view>
      <view class="admin-entry-group"><view class="admin-entry-heading"><view class="action-icon request-icon">◌</view><view><strong>运营工作台</strong><text>素材、客户与操作记录</text></view></view><view class="admin-entry-links"><button @click="openMaterialLibrary"><text>▧</text>素材管理</button><button @click="workbench"><text>▤</text>运营管理</button></view></view>
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
.shortcut-section{margin:34rpx 0 24rpx}.section-heading{display:flex;align-items:baseline;justify-content:space-between;margin:0 4rpx 18rpx}.section-heading strong{font-size:32rpx}.section-heading text{color:#929b91;font-size:21rpx}.shortcut-grid{display:grid;grid-template-columns:1fr 1fr;gap:14rpx}.shortcut-grid>view{display:grid;min-width:0;grid-template-columns:64rpx minmax(0,1fr);align-items:center;column-gap:18rpx;padding:24rpx;border-radius:22rpx;background:#fff}.shortcut-grid>view:active{transform:scale(.98);background:#f0f2e9}.shortcut-grid i{position:relative;display:grid;place-items:center;width:64rpx;height:64rpx;border-radius:18rpx 18rpx 8rpx;background:#e9eedc;color:#4d623e;font-style:normal}.shortcut-copy{display:flex;min-width:0;flex-direction:column;gap:6rpx}.shortcut-icon::before{font-size:30rpx;font-weight:500;line-height:1}.shortcut-icon.home::before{content:"⌂"}.shortcut-icon.favorite::before{content:"♡";font-size:38rpx}.shortcut-icon.footprint::before{content:"◴"}.shortcut-icon.request::before{content:"✦"}.shortcut-icon.history::before{content:"☷"}.shortcut-grid strong{overflow:hidden;font-size:27rpx;line-height:1.25;text-overflow:ellipsis;white-space:nowrap}.shortcut-grid text{overflow:hidden;color:#899287;font-size:21rpx;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.secondary-entry{display:flex;align-items:center;justify-content:space-between;border:0;background:#efeee7}.secondary-entry view{display:flex;flex-direction:column;gap:7rpx}.secondary-entry strong{font-size:27rpx}.secondary-entry view text{color:#899287;font-size:21rpx}.secondary-entry>text{color:#74806f;font-size:40rpx}
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

/* Admin shortcuts are grouped by task to keep the page clear and scannable. */
.admin-identity{display:flex;align-items:center;gap:18rpx;margin-bottom:26rpx;padding:24rpx;border-radius:24rpx;background:#40543f;color:#fff}.admin-avatar{width:88rpx;height:88rpx;min-width:88rpx;flex:none;border-radius:24rpx;background:#e7edcf;color:#40543f;font-size:38rpx;line-height:88rpx}.admin-identity-copy{min-width:0;flex:1}.admin-identity-copy>view{display:flex;align-items:center;gap:10rpx}.admin-name{width:220rpx;height:42rpx;padding:0;border:0;background:transparent;color:#fff;font-size:30rpx;font-weight:750;line-height:42rpx}.admin-name::placeholder{color:rgba(255,255,255,.7)}.admin-role{padding:5rpx 10rpx;border-radius:8rpx;background:rgba(231,237,207,.16);color:#e7edcf;font-size:18rpx;font-weight:700}.admin-identity-copy small{display:block;overflow:hidden;margin-top:7rpx;color:rgba(255,255,255,.66);font-size:20rpx;text-overflow:ellipsis;white-space:nowrap}.admin-edit{flex:none;color:#e7edcf;font-size:19rpx}
.admin-actions{display:grid;grid-template-columns:1fr;gap:16rpx}.admin-entry-group{position:relative;min-height:0;padding:24rpx;box-sizing:border-box;border:1rpx solid #e7e8dd;border-radius:24rpx;background:linear-gradient(145deg,#fff 0%,#fdfdf9 100%);box-shadow:0 8rpx 24rpx rgba(52,75,53,.045)}.admin-entry-heading{display:flex;align-items:center;gap:16rpx}.admin-entry-heading>view:last-child{min-width:0}.admin-entry-heading strong,.admin-entry-heading text{display:block}.admin-entry-heading strong{color:#354535;font-size:28rpx;font-weight:700;line-height:1.25}.admin-entry-heading text{margin-top:6rpx;color:#899287;font-size:21rpx;line-height:1.35}.admin-entry-links{display:grid;grid-template-columns:1fr 1fr;gap:12rpx;margin-top:20rpx}.admin-entry-links.single{grid-template-columns:1fr}.admin-entry-links button{display:flex;align-items:center;justify-content:center;gap:10rpx;min-width:0;height:72rpx;margin:0;padding:0 16rpx;border:1rpx solid #e0e6da;border-radius:14rpx;background:#f7f9f4;color:#4b6048;font-size:24rpx;font-weight:700;line-height:1;transition:transform .15s ease,background .15s ease}.admin-entry-links button:active{transform:scale(.98);background:#edf2e8}.admin-entry-links button::after{border:0}.admin-entry-links button text{margin:0;color:inherit;font-size:25rpx;line-height:1}.admin-entry-links .entry-arrow{margin-left:auto;color:#82907c;font-size:32rpx}.property-entry .admin-entry-links button{justify-content:flex-start;border-color:#40543f;background:#40543f;color:#fff}.property-entry .admin-entry-links button:active{background:#354b35}.property-entry .entry-arrow{color:rgba(255,255,255,.8)}.action-icon{display:grid;width:56rpx;height:56rpx;flex:none;place-items:center;border-radius:17rpx 17rpx 8rpx;background:#e9eedc;color:#40543f;font-size:29rpx;font-weight:700}.workspace-icon{background:#e7efe9;color:#4f715e}.request-icon{background:#f5eee1;color:#92764b}.interest-icon{background:#f7e9eb;color:#bd6772}
.avatar{color:#354b35}.page{padding-bottom:calc(96rpx + env(safe-area-inset-bottom))}.logout:active{opacity:.72}.admin-actions>view,.shortcut-grid>view,.secondary-entry{border:1rpx solid #ebe8e1}
.profile .avatar{display:grid;place-items:center;width:88rpx;height:88rpx;flex:none;border-radius:26rpx;background:#e7edcf;color:#354b35;font-size:36rpx;font-weight:800;line-height:1}.profile-title{justify-content:flex-start;font-size:34rpx}.guest-badge{padding:5rpx 10rpx;border-radius:9rpx;background:rgba(231,237,207,.18);color:#edf2db;font-size:18rpx;font-weight:700}.guest-actions{position:relative;display:grid;gap:13rpx;margin-top:22rpx;padding-top:19rpx;border-top:1rpx solid rgba(255,255,255,.16)}.guest-actions>text:first-child{color:rgba(255,255,255,.76);font-size:20rpx;line-height:1.45}.guest-actions>view{display:grid;grid-template-columns:1fr 1.2fr;gap:10rpx}.guest-actions button{height:68rpx;margin:0;padding:0 12rpx;border:1rpx solid rgba(255,255,255,.24);border-radius:13rpx;font-size:22rpx;font-weight:700;line-height:68rpx}.guest-actions button::after{border:0}.guest-actions .guest-browse{background:rgba(255,255,255,.12);color:#fff}.guest-actions .guest-login{border-color:#e7edcf;background:#e7edcf;color:#354b35}.test-access-entry{justify-self:center;padding:2rpx 12rpx;color:rgba(255,255,255,.68)!important;font-size:18rpx!important;line-height:1.4}
.nickname-input{height:40rpx;font-size:30rpx;font-weight:650;line-height:40rpx}.role-badge,.admin-role{padding:3rpx 8rpx;border-radius:7rpx;font-size:16rpx;font-weight:650;line-height:1.2}.role-badge{background:rgba(231,237,207,.16);color:#e7edcf}.admin-name{font-size:28rpx;font-weight:650}
</style>
