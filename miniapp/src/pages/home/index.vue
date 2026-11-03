<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { ref } from "vue";
import {
  getPublicConfig,
  listProperties,
  type Property,
  type PublicConfig,
} from "../../api/properties";

const defaultConfig: PublicConfig = {
  heroTitle: "小皮租房",
  heroSubtitle: "在武汉，住得更省心",
  heroDescription: "真实房源 · 可靠服务",
  notice: "签约前请核对合同条款与费用明细，保障自身权益。",
  contactPhone: "",
  locations: [],
  featuredChannels: [
    {
      icon: "🏢",
      title: "金融港",
      subtitle: "通勤优选",
      keyword: "金融港",
      radiusKm: 3,
    },
    {
      icon: "🌳",
      title: "光谷",
      subtitle: "品质房源",
      keyword: "光谷",
      radiusKm: 3,
    },
    {
      icon: "🌆",
      title: "街道口",
      subtitle: "生活便利",
      keyword: "街道口",
      radiusKm: 3,
    },
  ],
};

const properties = ref<Property[]>([]);
const config = ref<PublicConfig>(defaultConfig);

onShow(async () => {
  const [propertyResult, configResult] = await Promise.allSettled([
    listProperties(),
    getPublicConfig(),
  ]);
  if (propertyResult.status === "fulfilled")
    properties.value = propertyResult.value;
  if (configResult.status === "fulfilled")
    config.value = { ...defaultConfig, ...configResult.value };
});

const open = (id: string) =>
  uni.navigateTo({ url: `/pages/properties/detail?id=${id}` });
const browse = () => uni.switchTab({ url: "/pages/properties/index" });
const openChannel = (channel: PublicConfig["featuredChannels"][number]) => {
  uni.setStorageSync(
    "xiaopi_property_filter",
    JSON.stringify({
      keyword: channel.keyword,
      latitude: channel.latitude || 0,
      longitude: channel.longitude || 0,
      radiusKm: channel.radiusKm || 3,
      title: channel.title,
    }),
  );
  browse();
};
</script>

<template>
  <view class="page">
    <view class="hero">
      <view class="hero-copy">
        <view class="brand">{{ config.heroTitle }}</view>
        <view class="slogan">{{ config.heroSubtitle }}</view>
        <view class="hero-muted">{{ config.heroDescription }}</view>
        <view class="map-button" @click="browse">查找房源 ›</view>
      </view>
      <view class="hero-figure">🏠</view>
    </view>

    <view v-if="config.notice" class="notice"
      ><text class="notice-label">公告</text
      ><text>{{ config.notice }}</text></view
    >

    <view class="section-title">精彩瞬间</view>
    <scroll-view scroll-x class="moments">
      <view class="moment">真实房源<br />现场实拍</view>
      <view class="moment second">安心入住<br />精选房源</view>
      <view class="moment third">周边生活<br />一站了解</view>
    </scroll-view>

    <view class="section-head"
      ><view class="section-title">房源专区</view
      ><text @click="browse">全部 {{ properties.length }}</text></view
    >
    <view
      v-if="properties.length"
      class="feature card"
      @click="open(properties[0].id)"
    >
      <view class="feature-image">🏙️</view>
      <view class="feature-overlay"
        ><text>热门专区</text
        ><view>{{ properties[0].community || "精选房源" }}</view
        ><small
          >{{ properties[0].location }} · {{ properties[0].layout }}</small
        ></view
      >
    </view>
    <view v-else class="feature card">
      <view class="feature-image">🏙️</view>
      <view class="feature-overlay"
        ><text>热门专区</text><view>武汉精选房源</view
        ><small>真实房源，轻松找房</small></view
      >
    </view>

    <view class="section-head"
      ><view class="section-title">精选频道</view><text>运营精选</text></view
    >
    <scroll-view scroll-x class="channels-scroll">
      <view class="channels">
        <view
          v-for="channel in config.featuredChannels"
          :key="`${channel.title}-${channel.keyword}`"
          class="channel"
          @click="openChannel(channel)"
        >
          <text class="channel-icon">{{ channel.icon || "🏠" }}</text>
          <text class="channel-title">{{ channel.title }}</text>
          <small>{{ channel.subtitle }}</small>
        </view>
      </view>
    </scroll-view>
  </view>
</template>

<style scoped>
.hero {
  display: flex;
  min-height: 330rpx;
  border-radius: 34rpx;
  padding: 42rpx 34rpx;
  background: linear-gradient(135deg, #d0f8ef, #fbfffe);
  overflow: hidden;
}
.hero-copy {
  flex: 1;
}
.brand {
  font-size: 58rpx;
  font-weight: 900;
  letter-spacing: 2rpx;
}
.slogan {
  font-size: 34rpx;
  font-weight: 700;
  margin-top: 18rpx;
}
.hero-muted {
  color: #68a198;
  font-size: 26rpx;
  margin-top: 14rpx;
}
.map-button {
  display: inline-block;
  background: #0e8177;
  color: #fff;
  border-radius: 999rpx;
  padding: 18rpx 30rpx;
  margin-top: 32rpx;
  font-size: 30rpx;
}
.hero-figure {
  font-size: 130rpx;
  align-self: center;
}
.notice {
  display: flex;
  align-items: center;
  gap: 16rpx;
  padding: 20rpx 18rpx;
  margin-top: 22rpx;
  background: #e5faf5;
  border: 1rpx solid #d2f1e9;
  border-radius: 24rpx;
  color: #407e77;
  font-size: 25rpx;
}
.notice-label {
  flex: none;
  background: #16a394;
  color: #fff;
  border-radius: 14rpx;
  padding: 10rpx 16rpx;
}
.moments {
  white-space: nowrap;
  width: 100%;
}
.moment {
  display: inline-flex;
  width: 260rpx;
  height: 160rpx;
  margin-right: 16rpx;
  border-radius: 26rpx;
  background: linear-gradient(145deg, #9fd3c5, #3d716c);
  color: #fff;
  align-items: flex-end;
  padding: 18rpx;
  font-size: 28rpx;
  font-weight: 700;
  box-sizing: border-box;
}
.second {
  background: linear-gradient(145deg, #eac7a4, #7e765f);
}
.third {
  background: linear-gradient(145deg, #9bc9e5, #527c88);
}
.section-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.section-head > .section-title {
  margin-bottom: 20rpx;
}
.section-head > text {
  color: #0e8177;
  font-size: 26rpx;
}
.feature {
  position: relative;
  padding: 0;
  overflow: hidden;
  height: 320rpx;
}
.feature-image {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(145deg, #327c9a, #b9e8e1);
  font-size: 160rpx;
}
.feature-overlay {
  position: absolute;
  inset: 0;
  padding: 38rpx 30rpx;
  color: #fff;
  background: linear-gradient(
    90deg,
    rgba(3, 52, 57, 0.72),
    rgba(3, 52, 57, 0) 80%
  );
}
.feature-overlay > text {
  background: #3b8a82aa;
  padding: 10rpx 18rpx;
  border-radius: 16rpx;
}
.feature-overlay > view {
  font-size: 42rpx;
  font-weight: 800;
  margin-top: 28rpx;
}
.feature-overlay small {
  font-size: 26rpx;
}
.channels-scroll {
  width: 100%;
  white-space: nowrap;
}
.channels {
  display: inline-flex;
  gap: 18rpx;
  padding-bottom: 10rpx;
}
.channel {
  width: 210rpx;
  height: 170rpx;
  border-radius: 24rpx;
  background: #d9f3ee;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
}
.channel-icon {
  font-size: 48rpx;
}
.channel-title {
  font-size: 27rpx;
  font-weight: 700;
  margin-top: 8rpx;
}
.channel small {
  font-size: 21rpx;
  color: #78a39d;
  margin-top: 4rpx;
}
</style>
