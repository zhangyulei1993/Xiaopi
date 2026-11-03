<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { computed, ref } from "vue";

type CropSource = {
  index: number;
  path: string;
  width: number;
  height: number;
  size: number;
};

const CANVAS_WIDTH = 1200;
const CANVAS_HEIGHT = 900;
const source = ref<CropSource | null>(null);
const zoom = ref(100);
const offsetX = ref(0);
const offsetY = ref(0);
const saving = ref(false);
let dragStart: { x: number; y: number; offsetX: number; offsetY: number } | null = null;

const scale = computed(() => {
  if (!source.value) return 1;
  return Math.max(
    CANVAS_WIDTH / source.value.width,
    CANVAS_HEIGHT / source.value.height,
  ) * (zoom.value / 100);
});
const renderedWidth = computed(() => (source.value ? source.value.width * scale.value : 0));
const renderedHeight = computed(() => (source.value ? source.value.height * scale.value : 0));
const maxOffsetX = computed(() => Math.max(0, (renderedWidth.value - CANVAS_WIDTH) / 2));
const maxOffsetY = computed(() => Math.max(0, (renderedHeight.value - CANVAS_HEIGHT) / 2));
const zoomLabel = computed(() => `${zoom.value}%`);

const clampOffset = () => {
  offsetX.value = Math.max(-maxOffsetX.value, Math.min(maxOffsetX.value, offsetX.value));
  offsetY.value = Math.max(-maxOffsetY.value, Math.min(maxOffsetY.value, offsetY.value));
};
const drawPreview = () => {
  if (!source.value) return;
  clampOffset();
  const context = uni.createCanvasContext("property-crop-canvas");
  context.setFillStyle("#17211d");
  context.fillRect(0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);
  context.drawImage(
    source.value.path,
    (CANVAS_WIDTH - renderedWidth.value) / 2 + offsetX.value,
    (CANVAS_HEIGHT - renderedHeight.value) / 2 + offsetY.value,
    renderedWidth.value,
    renderedHeight.value,
  );
  context.draw();
};
const resetCrop = () => {
  zoom.value = 100;
  offsetX.value = 0;
  offsetY.value = 0;
  drawPreview();
};
const nudge = (x: number, y: number) => {
  offsetX.value += x;
  offsetY.value += y;
  drawPreview();
};
const updateZoom = (event: { detail: { value: number } }) => {
  zoom.value = Number(event.detail.value) || 100;
  drawPreview();
};
const point = (event: any) => event.touches?.[0] || event.changedTouches?.[0];
const dragStartHandler = (event: any) => {
  const touch = point(event);
  if (!touch) return;
  dragStart = {
    x: Number(touch.clientX || touch.pageX || 0),
    y: Number(touch.clientY || touch.pageY || 0),
    offsetX: offsetX.value,
    offsetY: offsetY.value,
  };
};
const dragMoveHandler = (event: any) => {
  const touch = point(event);
  if (!dragStart || !touch) return;
  const currentX = Number(touch.clientX || touch.pageX || 0);
  const currentY = Number(touch.clientY || touch.pageY || 0);
  const screenWidth = uni.getSystemInfoSync().windowWidth || 375;
  const factor = CANVAS_WIDTH / Math.max(1, screenWidth - 48);
  offsetX.value = dragStart.offsetX + (currentX - dragStart.x) * factor;
  offsetY.value = dragStart.offsetY + (currentY - dragStart.y) * factor;
  drawPreview();
};
const finish = () => {
  if (!source.value || saving.value) return;
  saving.value = true;
  const context = uni.createCanvasContext("property-crop-canvas");
  context.setFillStyle("#17211d");
  context.fillRect(0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);
  context.drawImage(
    source.value.path,
    (CANVAS_WIDTH - renderedWidth.value) / 2 + offsetX.value,
    (CANVAS_HEIGHT - renderedHeight.value) / 2 + offsetY.value,
    renderedWidth.value,
    renderedHeight.value,
  );
  context.draw(false, () => {
    uni.canvasToTempFilePath({
      canvasId: "property-crop-canvas",
      destWidth: CANVAS_WIDTH,
      destHeight: CANVAS_HEIGHT,
      quality: 0.92,
      success: (result) => {
        const path = result.tempFilePath;
        uni.getFileInfo({
          filePath: path,
          success: (file) => {
            uni.setStorageSync("xiaopi-property-image-crop-result", {
              index: source.value?.index,
              path,
              width: CANVAS_WIDTH,
              height: CANVAS_HEIGHT,
              size: file.size,
            });
            uni.navigateBack();
          },
          fail: () => {
            uni.setStorageSync("xiaopi-property-image-crop-result", {
              index: source.value?.index,
              path,
              width: CANVAS_WIDTH,
              height: CANVAS_HEIGHT,
              size: source.value?.size,
            });
            uni.navigateBack();
          },
        });
      },
      fail: () => {
        saving.value = false;
        uni.showToast({ title: "图片处理失败，请重试", icon: "none" });
      },
    });
  });
};
const cancel = () => uni.navigateBack();

onLoad(() => {
  const cached = uni.getStorageSync("xiaopi-property-image-crop-source") as CropSource;
  uni.removeStorageSync("xiaopi-property-image-crop-source");
  if (!cached?.path || !cached.width || !cached.height) {
    uni.showToast({ title: "没有可处理的图片", icon: "none" });
    setTimeout(() => uni.navigateBack(), 300);
    return;
  }
  source.value = cached;
  setTimeout(drawPreview, 120);
});
</script>

<template>
  <view class="crop-page">
    <view class="intro">
      <strong>调整展示位置</strong>
      <text>拖动图片选择保留区域，列表会按 4:3 展示。</text>
    </view>
    <view class="canvas-wrap" @touchstart="dragStartHandler" @touchmove.stop.prevent="dragMoveHandler">
      <canvas canvas-id="property-crop-canvas" class="crop-canvas" :width="CANVAS_WIDTH" :height="CANVAS_HEIGHT" />
      <view class="crop-grid"></view>
    </view>
    <view class="crop-controls">
      <text>缩放 {{ zoomLabel }}</text>
      <slider :value="zoom" :min="100" :max="220" :step="1" activeColor="#536a45" backgroundColor="#dfe9e2" @change="updateZoom" @changing="updateZoom" />
      <view class="nudge-row">
        <button @click="nudge(0, 60)">↑</button>
        <button @click="nudge(-60, 0)">←</button>
        <button @click="resetCrop">重置</button>
        <button @click="nudge(60, 0)">→</button>
        <button @click="nudge(0, -60)">↓</button>
      </view>
    </view>
    <view class="bottom-actions">
      <button class="secondary" @click="cancel">取消</button>
      <button class="primary" :loading="saving" @click="finish">使用此裁切</button>
    </view>
  </view>
</template>

<style scoped>
.crop-page { min-height: 100vh; padding: 32rpx 24rpx 48rpx; background: #f7f5ef; box-sizing: border-box; }
.intro strong, .intro text { display: block; }
.intro strong { color: #304530; font-size: 34rpx; letter-spacing: -1rpx; }
.intro text { margin-top: 12rpx; color: #71806e; font-size: 24rpx; line-height: 1.55; }
.canvas-wrap { position: relative; width: 100%; margin-top: 28rpx; overflow: hidden; border-radius: 22rpx; background: #17211d; box-shadow: 0 16rpx 36rpx rgba(44, 65, 48, 0.18); }
.crop-canvas { display: block; width: 100%; height: 513rpx; }
.crop-grid { position: absolute; inset: 0; pointer-events: none; background: linear-gradient(to right, transparent 33.1%, rgba(255,255,255,.35) 33.3%, rgba(255,255,255,.35) 33.7%, transparent 33.9%, transparent 66.1%, rgba(255,255,255,.35) 66.3%, rgba(255,255,255,.35) 66.7%, transparent 66.9%), linear-gradient(to bottom, transparent 33.1%, rgba(255,255,255,.35) 33.3%, rgba(255,255,255,.35) 33.7%, transparent 33.9%, transparent 66.1%, rgba(255,255,255,.35) 66.3%, rgba(255,255,255,.35) 66.7%, transparent 66.9%); }
.crop-controls { margin-top: 26rpx; padding: 24rpx; border-radius: 20rpx; background: #fff; }
.crop-controls > text { display: block; color: #52634f; font-size: 24rpx; font-weight: 600; }
.crop-controls slider { margin: 12rpx -12rpx 8rpx; }
.nudge-row { display: grid; grid-template-columns: repeat(5, 1fr); gap: 10rpx; }
.nudge-row button, .bottom-actions button { margin: 0; border: 0; }
.nudge-row button { padding: 0; height: 64rpx; border-radius: 14rpx; background: #eef3eb; color: #445741; font-size: 23rpx; line-height: 64rpx; }
.bottom-actions { display: grid; grid-template-columns: 1fr 1.7fr; gap: 16rpx; margin-top: 30rpx; }
.bottom-actions button { height: 88rpx; border-radius: 18rpx; font-size: 27rpx; line-height: 88rpx; }
.secondary { background: #e8eee5; color: #4d6149; }
.primary { background: #536a45; color: #fff; }
</style>
