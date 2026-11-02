<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { ref } from "vue";
import { API_ORIGIN } from "../../api/client";
import {
  getPublicConfig,
  listFavorites,
  listProperties,
  setFavorite,
  type Property,
} from "../../api/properties";
import { getSession } from "../../auth/session";
const item = ref<Property | null>(null),
  contactPhone = ref(""),
  loading = ref(true),
  loadFailed = ref(false),
  favorite = ref(false);
const load = async (id = "") => {
  loading.value = true;
  loadFailed.value = false;
  try {
    const [items, config] = await Promise.all([
      listProperties(),
      getPublicConfig().catch(() => ({ contactPhone: "" })),
    ]);
    item.value = items.find((property) => property.id === id) || null;
    contactPhone.value = config.contactPhone;
    if (getSession()?.role === "user")
      favorite.value = (await listFavorites().catch(() => [])).includes(id);
  } catch (error) {
    loadFailed.value = true;
    uni.showToast({
      title: error instanceof Error ? error.message : "房源加载失败",
      icon: "none",
    });
  } finally {
    loading.value = false;
  }
};
onLoad((query) => load(String(query?.id || "")));
const mediaUrl = (url: string) =>
  url.startsWith("http") ? url : `${API_ORIGIN}${url}`;
const requireUser = () => {
  if (getSession()?.role === "user") return true;
  if (getSession()?.role === "admin") {
    uni.showToast({ title: "管理员无需使用找房互动", icon: "none" });
    return false;
  }
  uni.navigateTo({ url: "/pages/auth/login" });
  return false;
};
const toggle = async () => {
  if (!item.value || !requireUser()) return;
  const next = !favorite.value;
  try {
    await setFavorite(item.value.id, next);
    favorite.value = next;
    uni.showToast({ title: next ? "已关注" : "已取消关注" });
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "操作失败",
      icon: "none",
    });
  }
};
const reserve = () => {
  if (!item.value || !requireUser()) return;
  uni.navigateTo({
    url: `/pages/requests/index?propertyId=${item.value.id}&title=${encodeURIComponent(item.value.title || item.value.community)}`,
  });
};
const contact = () => {
  if (!getSession()) {
    uni.navigateTo({ url: "/pages/auth/login" });
    return;
  }
  if (!contactPhone.value) {
    uni.showToast({ title: "管理员电话尚未配置", icon: "none" });
    return;
  }
  uni.makePhoneCall({ phoneNumber: contactPhone.value });
};
</script>
<template>
  <view v-if="loading" class="page state">正在加载房源...</view
  ><view v-else-if="loadFailed" class="page state"
    >房源加载失败，请返回重试</view
  ><view v-else-if="item" class="page detail-page"
    ><view v-if="!item.media?.length" class="detail-cover">🏠</view
    ><view v-else class="media-list"
      ><block
        v-for="(media, index) in item.media"
        :key="`${media.url}-${index}`"
        ><image
          v-if="media.type === 'image'"
          class="media-image"
          :src="mediaUrl(media.url)"
          mode="aspectFill" /><video
          v-else
          class="media-video"
          :src="mediaUrl(media.url)"
          controls
          object-fit="contain" /></block></view
    ><view class="card overview"
      ><view class="overview-top"
        ><view class="price">{{ item.rentMonthly }} <small>元/月</small></view
        ><view class="heart" :class="{ active: favorite }" @click="toggle"
          >{{ favorite ? "♥" : "♡"
          }}<small>{{ favorite ? "已关注" : "关注" }}</small></view
        ></view
      ><view class="title">{{ item.title || item.community }}</view
      ><view class="muted">{{ item.layout }} · {{ item.location }}</view
      ><view class="tags"
        ><text>{{ item.community }}</text
        ><text>{{ item.feeText || "租房信息" }}</text></view
      ></view
    ><view class="card"
      ><view class="title small">房源信息</view
      ><view class="row"
        ><text>小区</text><text>{{ item.community }}</text></view
      ><view class="row"
        ><text>位置</text><text>{{ item.location }}</text></view
      ><view class="row"
        ><text>户型</text><text>{{ item.layout }}</text></view
      ></view
    ><view class="card"
      ><view class="title small">房源描述</view
      ><view class="desc">{{ item.description || "暂无描述" }}</view></view
    ><view class="bottom-space" /><view class="action-bar"
      ><view class="contact-action" @click="contact"
        ><text>☎</text><small>联系小皮</small></view
      ><view class="reserve" @click="reserve">预约房源</view></view
    ></view
  ><view v-else class="page state">房源不存在或已下架</view>
</template>
<style scoped>
.state {
  text-align: center;
  padding-top: 180rpx;
  color: #78a39d;
}
.detail-page {
  padding-bottom: 150rpx;
}
.detail-cover {
  height: 420rpx;
  border-radius: 28rpx;
  background: #dff6ef;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 160rpx;
}
.media-list {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
}
.media-image,
.media-video {
  width: 100%;
  height: 420rpx;
  border-radius: 28rpx;
  background: #dff6ef;
}
.overview-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}
.price small {
  font-size: 22rpx;
}
.heart {
  display: flex;
  align-items: center;
  gap: 8rpx;
  color: #638b85;
  font-size: 40rpx;
}
.heart small {
  font-size: 22rpx;
}
.heart.active {
  color: #e44d66;
}
.small {
  font-size: 34rpx;
  margin-bottom: 20rpx;
}
.tags {
  display: flex;
  gap: 16rpx;
  margin-top: 24rpx;
}
.tags text {
  padding: 12rpx 22rpx;
  border-radius: 999rpx;
  background: #e9faf5;
  color: #0f8177;
}
.row {
  display: flex;
  justify-content: space-between;
  padding: 22rpx 0;
  border-bottom: 1rpx solid #eef5f2;
  color: #6e9a93;
}
.row text:last-child {
  color: #073b3a;
}
.desc {
  line-height: 1.8;
  color: #57857e;
}
.action-bar {
  position: fixed;
  z-index: 5;
  left: 0;
  right: 0;
  bottom: 0;
  display: grid;
  grid-template-columns: 180rpx 1fr;
  gap: 18rpx;
  padding: 20rpx 32rpx calc(20rpx + env(safe-area-inset-bottom));
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 -8rpx 30rpx rgba(15, 70, 65, 0.1);
}
.contact-action {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  color: #315f5a;
}
.contact-action text {
  font-size: 31rpx;
}
.contact-action small {
  font-size: 21rpx;
}
.reserve {
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 999rpx;
  background: #0e8177;
  color: #fff;
  font-size: 30rpx;
  font-weight: 700;
}
</style>
