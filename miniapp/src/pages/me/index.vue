<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import {
  clearSession,
  getSession,
  saveSession,
  type SessionRole,
} from "../../auth/session";
import { bindWechatPhone, refreshAuth } from "../../api/properties";

const role = ref<SessionRole | null>(null);
const phone = ref("");
const isBindingPhone = ref(false);
const isLoggedIn = computed(() => role.value !== null);
const isAdmin = computed(() => role.value === "admin");

onShow(async () => {
  const session = getSession();
  role.value = session?.role || null;
  phone.value = String(uni.getStorageSync("xiaopi_bound_phone") || "");
  if (!session) return;
  try {
    const refreshed = await refreshAuth();
    saveSession(refreshed.accessToken, refreshed.role);
    role.value = refreshed.role;
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
const workbench = () => uni.navigateTo({ url: "/pages/admin/index" });
const editProperty = () =>
  uni.navigateTo({ url: "/pages/admin/property-edit" });

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
    uni.showToast({ title: "手机号绑定成功" });
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
  <view class="page">
    <view class="profile card" @click="!isLoggedIn && login()">
      <view class="avatar">{{ isAdmin ? "管" : "小" }}</view>
      <view class="profile-copy">
        <view class="profile-title"
          ><view class="title">{{
            isAdmin ? "小皮管理员" : isLoggedIn ? "微信用户" : "登录小皮租房"
          }}</view
          ><text
            v-if="isLoggedIn"
            class="role-badge"
            :class="{ admin: isAdmin }"
            >{{ isAdmin ? "运营管理员" : "普通用户" }}</text
          ></view
        >
        <view class="muted">
          {{
            isAdmin
              ? "房源运营工作台已授权"
              : isLoggedIn
                ? "已登录，可联系房源"
                : "登录后联系房源并管理身份服务"
          }}
        </view>
      </view>
      <text v-if="!isLoggedIn" class="arrow">›</text>
    </view>

    <view class="card menu">
      <view @click="browse">浏览全部房源 <text>›</text></view>
      <view v-if="!isAdmin" @click="favorites">我的关注 <text>›</text></view>
      <view v-if="!isAdmin" @click="requestHome"
        >提交找房需求 <text>›</text></view
      >
      <view v-if="!isAdmin" @click="publish">提交房源线索 <text>›</text></view>
    </view>

    <view v-if="isLoggedIn" class="card phone-card">
      <view>
        <view class="phone-title">微信手机号</view>
        <view class="muted">{{
          phone || "尚未绑定，管理员权限需要通过手机号指定"
        }}</view>
      </view>
      <button
        v-if="!phone"
        open-type="getPhoneNumber"
        :loading="isBindingPhone"
        :disabled="isBindingPhone"
        @getphonenumber="bindPhone"
      >
        绑定手机号
      </button>
      <text v-else class="bound">已绑定</text>
    </view>

    <view v-if="isAdmin" class="card menu">
      <view class="group-title">运营管理</view>
      <view @click="workbench">管理员工作台 <text>›</text></view>
      <view @click="editProperty">录入正式房源 <text>›</text></view>
    </view>

    <button v-if="isLoggedIn" class="logout" @click="logout">退出登录</button>
  </view>
</template>

<style scoped>
.profile {
  display: flex;
  align-items: center;
  gap: 24rpx;
  padding: 40rpx;
}
.profile-copy {
  flex: 1;
}
.profile-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12rpx;
}
.role-badge {
  padding: 6rpx 12rpx;
  border-radius: 999rpx;
  background: #edf4f2;
  color: #6b8c86;
  font-size: 20rpx;
  font-weight: 600;
}
.role-badge.admin {
  background: #e1f5ee;
  color: #0f8177;
}
.avatar {
  width: 110rpx;
  height: 110rpx;
  border-radius: 50%;
  background: #0f8177;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 48rpx;
  font-weight: 700;
}
.arrow {
  color: #9bbab4;
  font-size: 54rpx;
}
.menu view {
  padding: 30rpx 0;
  border-bottom: 1rpx solid #edf4f1;
  font-size: 30rpx;
}
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

.profile { margin-top:18rpx; padding:32rpx; border:0; border-radius:24rpx; background:#e8eedc; box-shadow:none; }.avatar { width:96rpx; height:96rpx; border-radius:26rpx 26rpx 10rpx; background:#40543f; font-size:40rpx; }.profile .title { font-size:34rpx; }.role-badge { border-radius:8rpx; background:#f8f7f1; color:#63705e; }.role-badge.admin { background:#d9e5bc; color:#44563d; }.menu { border:0; border-radius:24rpx; box-shadow:none; }.menu view { padding:27rpx 0; border-color:#eeece7; color:#354235; }.menu text { color:#a0a79e; }.phone-card { border:0; border-radius:24rpx; box-shadow:none; }.phone-card button { border-radius:12rpx; background:#40543f; }.bound { color:#607856; }.logout { color:#bd725f; }
</style>
