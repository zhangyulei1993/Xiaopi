<script setup lang="ts">
import { resolveAssetUrl } from "../api/client";
import type { PropertyMedia } from "../api/properties";
import { getCurrentInstance, watch } from "vue";
const props = defineProps<{ mediaItems: PropertyMedia[]; activeMediaIndex: number; playingVideoIndex: number; activeVideoIsPortrait: boolean; videoPoster: string; showMediaFullscreenHint: boolean; imageMediaIndices: number[]; videoMediaIndex: number; activeImageNumber: number; isAdmin: boolean }>();
const emit = defineEmits(["imageClick", "imageLongpress", "videoAction", "change", "select", "metadata", "play", "stop"]);
const mediaViewportHeight = 720;
const instance = getCurrentInstance();
watch(() => props.activeMediaIndex, (_next, previous) => {
  if (props.mediaItems[previous]?.type === "video") uni.createVideoContext(`detail-video-${previous}`, instance?.proxy as any).pause();
});
const shouldLoad = (index: number) => { const count = props.mediaItems.length; const current = props.activeMediaIndex; return count < 4 || Math.abs(index - current) <= 1 || (index === 0 && current === count - 1) || (index === count - 1 && current === 0); };
</script>
<template><view class="media-gallery" :class="{ 'portrait-video': activeVideoIsPortrait, 'video-active': mediaItems[activeMediaIndex]?.type === 'video', 'video-playing': playingVideoIndex === activeMediaIndex }"
      ><swiper class="media-swiper" :style="{ height: `${mediaViewportHeight}rpx` }" circular :current="activeMediaIndex" @change="event => emit('change', event)">
        <swiper-item v-for="(media, index) in mediaItems" :key="`${media.url}-${index}`">
          <image
            v-if="media.type === 'image' && shouldLoad(index)"
            class="media-image"
            :src="resolveAssetUrl(media.url)"
            lazy-load
            mode="aspectFill"
            :show-menu-by-longpress="false"
            @click="emit('imageClick', media.url)" @longpress="emit('imageLongpress', index)" />
          <view v-else-if="media.type === 'video' && shouldLoad(index)" class="video-stage">
            <video
              :id="`detail-video-${index}`"
              class="media-video"
              :src="resolveAssetUrl(media.url)"
              :poster="videoPoster"
              controls
              show-play-btn
              :show-center-play-btn="true"
              show-progress
              show-fullscreen-btn
              enable-progress-gesture
              enable-play-gesture
              preload="metadata"
              object-fit="cover"
              @longpress="emit('videoAction', index)"
              @loadedmetadata="emit('metadata', index, $event)"
              @play="emit('play', index)"
              @pause="emit('stop')"
              @ended="emit('stop')"
            ></video>
          </view>
          <view v-else class="media-loading">加载媒体中</view>
        </swiper-item>
      </swiper
      ><view v-if="showMediaFullscreenHint && (mediaItems[activeMediaIndex]?.type !== 'video' || playingVideoIndex === activeMediaIndex)" class="media-fullscreen-hint" :class="{ 'video-hint': mediaItems[activeMediaIndex]?.type === 'video', 'image-hint': mediaItems[activeMediaIndex]?.type !== 'video' }"><text>{{ mediaItems[activeMediaIndex]?.type === 'video' ? '全屏观看' : '点击图片查看大图' }}</text><text class="hint-arrow">{{ mediaItems[activeMediaIndex]?.type === 'video' ? '↘' : '↗' }}</text></view>
      <view v-if="isAdmin && mediaItems[activeMediaIndex]?.type === 'video'" class="video-actions-button" @click.stop="emit('videoAction', activeMediaIndex)">视频操作</view>
      <view v-if="imageMediaIndices.length || videoMediaIndex >= 0" class="media-strip">
        <view class="media-strip-row">
          <view v-if="videoMediaIndex >= 0" class="media-thumb video" :class="{ active: activeMediaIndex === videoMediaIndex }" @click="emit('select', videoMediaIndex)"><text class="media-thumb-label">视频</text></view>
          <view v-if="imageMediaIndices.length" class="media-thumb" :class="{ active: mediaItems[activeMediaIndex]?.type === 'image' }" @click="emit('select', mediaItems[activeMediaIndex]?.type === 'image' ? activeMediaIndex : imageMediaIndices[0])"><text class="media-thumb-label">照片 {{ mediaItems[activeMediaIndex]?.type === 'image' ? activeImageNumber : 1 }}/{{ imageMediaIndices.length }}</text></view>
        </view>
      </view>
      </view></template><style scoped>
.media-gallery{position:relative;overflow:hidden;margin:0 -24rpx;background:#dce4d5}.media-swiper{width:100%;height:720rpx}.media-gallery .media-image,.media-gallery .media-video,.video-stage{display:block;width:100%;height:100%}.video-stage{position:relative;background:#40543f}.media-loading{display:flex;width:100%;height:100%;align-items:center;justify-content:center;background:#edf1e9;color:#7c8878;font-size:24rpx}.media-strip{position:absolute;z-index:3;left:18rpx;bottom:17px;max-width:calc(100% - 36rpx);white-space:nowrap}.media-strip-row{display:inline-flex;gap:8rpx}.media-thumb{display:flex;min-width:66rpx;height:48rpx;align-items:center;justify-content:center;padding:0 15rpx;border:1rpx solid #ffffff94;border-radius:8rpx;background:#1b261d7a;box-sizing:border-box}.media-thumb-label{color:#fff;font-size:17rpx;font-weight:600;white-space:nowrap}.media-thumb.active{border-color:#fff;background:#374d38d1}.media-gallery.video-active .media-strip{bottom:calc(104rpx - 20px + 1px)}.media-gallery.video-playing .media-strip,.media-gallery.video-playing .video-actions-button{opacity:0;pointer-events:none}.media-fullscreen-hint{position:absolute;z-index:4;right:76rpx;bottom:3px;padding:10rpx 14rpx;border-radius:999rpx;background:#1219138f;color:#fff;font-size:20rpx;pointer-events:none}.media-fullscreen-hint.image-hint{top:50%;right:auto;bottom:auto;left:50%;transform:translate(-50%,-50%);font-size:22rpx}.video-actions-button{position:absolute;z-index:5;right:20rpx;top:calc(var(--status-bar-height, 0px) + 90rpx);padding:10rpx 18rpx;border:1rpx solid #ffffff99;border-radius:999rpx;background:#182219ad;color:#fff;font-size:22rpx}
</style>
