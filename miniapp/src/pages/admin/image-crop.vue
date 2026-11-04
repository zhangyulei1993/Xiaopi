<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { computed, ref } from "vue";

type CropSource = { index: number; path: string; width: number; height: number; size: number };
type TouchPoint = { x: number; y: number };
const CANVAS_WIDTH = 1200;
const CANVAS_HEIGHT = 900;
const FRAME_WIDTH = 900;
const FRAME_HEIGHT = 675;
const FRAME_X = (CANVAS_WIDTH - FRAME_WIDTH) / 2;
const FRAME_Y = (CANVAS_HEIGHT - FRAME_HEIGHT) / 2;
const source = ref<CropSource | null>(null);
const zoom = ref(130);
const offsetX = ref(0);
const offsetY = ref(0);
const saving = ref(false);
let drag: { point: TouchPoint; offsetX: number; offsetY: number } | null = null;
let pinch: { distance: number; zoom: number } | null = null;

const baseScale = computed(() => source.value ? Math.max(CANVAS_WIDTH / source.value.width, CANVAS_HEIGHT / source.value.height) : 1);
const minZoom = computed(() => {
  if (!source.value) return 70;
  const frameScale = Math.max(FRAME_WIDTH / source.value.width, FRAME_HEIGHT / source.value.height);
  return Math.max(55, Math.min(100, Math.ceil(frameScale / baseScale.value * 100)));
});
const scale = computed(() => baseScale.value * zoom.value / 100);
const renderedWidth = computed(() => source.value ? source.value.width * scale.value : 0);
const renderedHeight = computed(() => source.value ? source.value.height * scale.value : 0);
const imageLeft = computed(() => (CANVAS_WIDTH - renderedWidth.value) / 2 + offsetX.value);
const imageTop = computed(() => (CANVAS_HEIGHT - renderedHeight.value) / 2 + offsetY.value);
const zoomLabel = computed(() => `${zoom.value}%`);

const clampImage = () => {
  const centeredLeft = (CANVAS_WIDTH - renderedWidth.value) / 2;
  const centeredTop = (CANVAS_HEIGHT - renderedHeight.value) / 2;
  const minX = FRAME_X + FRAME_WIDTH - centeredLeft - renderedWidth.value;
  const maxX = FRAME_X - centeredLeft;
  const minY = FRAME_Y + FRAME_HEIGHT - centeredTop - renderedHeight.value;
  const maxY = FRAME_Y - centeredTop;
  offsetX.value = Math.max(minX, Math.min(maxX, offsetX.value));
  offsetY.value = Math.max(minY, Math.min(maxY, offsetY.value));
};
const drawPreview = () => {
  if (!source.value) return;
  clampImage();
  const context = uni.createCanvasContext("property-crop-canvas");
  context.setFillStyle("#17211d");
  context.fillRect(0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);
  context.drawImage(source.value.path, imageLeft.value, imageTop.value, renderedWidth.value, renderedHeight.value);
  context.draw();
};
const resetCrop = () => { zoom.value = 100; offsetX.value = 0; offsetY.value = 0; drawPreview(); };
const updateZoom = (event: { detail: { value: number } }) => {
  zoom.value = Math.max(minZoom.value, Math.min(220, Number(event.detail.value) || minZoom.value));
  drawPreview();
};
const touchPoint = (touch: any): TouchPoint => ({ x: Number(touch?.clientX || touch?.pageX || 0), y: Number(touch?.clientY || touch?.pageY || 0) });
const touchDistance = (first: any, second: any) => {
  const a = touchPoint(first); const b = touchPoint(second);
  return Math.hypot(a.x - b.x, a.y - b.y);
};
const touchStart = (event: any) => {
  const touches = event.touches || [];
  if (touches.length >= 2) {
    pinch = { distance: touchDistance(touches[0], touches[1]), zoom: zoom.value };
    drag = null;
    return;
  }
  if (!touches.length) return;
  drag = { point: touchPoint(touches[0]), offsetX: offsetX.value, offsetY: offsetY.value };
  pinch = null;
};
const touchMove = (event: any) => {
  const touches = event.touches || [];
  if (touches.length >= 2) {
    if (!pinch) { pinch = { distance: touchDistance(touches[0], touches[1]), zoom: zoom.value }; return; }
    zoom.value = Math.max(minZoom.value, Math.min(220, Math.round(pinch.zoom * touchDistance(touches[0], touches[1]) / Math.max(1, pinch.distance))));
    drawPreview();
    return;
  }
  if (!drag || !touches.length) return;
  const point = touchPoint(touches[0]);
  const factor = CANVAS_WIDTH / Math.max(1, (uni.getSystemInfoSync().windowWidth || 375) - 48);
  offsetX.value = drag.offsetX + (point.x - drag.point.x) * factor;
  offsetY.value = drag.offsetY + (point.y - drag.point.y) * factor;
  drawPreview();
};
const touchEnd = () => { drag = null; pinch = null; };
const complete = (path: string, size: number) => {
  uni.setStorageSync("xiaopi-property-image-crop-result", { index: source.value?.index, path, width: CANVAS_WIDTH, height: CANVAS_HEIGHT, size });
  uni.navigateBack();
};
const finish = () => {
  if (!source.value || saving.value) return;
  saving.value = true;
  clampImage();
  const sx = (FRAME_X - imageLeft.value) / scale.value;
  const sy = (FRAME_Y - imageTop.value) / scale.value;
  const context = uni.createCanvasContext("property-crop-canvas");
  context.drawImage(source.value.path, sx, sy, FRAME_WIDTH / scale.value, FRAME_HEIGHT / scale.value, 0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);
  context.draw(false, () => uni.canvasToTempFilePath({
    canvasId: "property-crop-canvas", destWidth: CANVAS_WIDTH, destHeight: CANVAS_HEIGHT, quality: 0.92,
    success: (result) => uni.getFileInfo({ filePath: result.tempFilePath, success: (file) => complete(result.tempFilePath, file.size), fail: () => complete(result.tempFilePath, source.value?.size || 0) }),
    fail: () => { saving.value = false; uni.showToast({ title: "图片处理失败，请重试", icon: "none" }); },
  }));
};
const cancel = () => uni.navigateBack();
onLoad(() => {
  const cached = uni.getStorageSync("xiaopi-property-image-crop-source") as CropSource;
  uni.removeStorageSync("xiaopi-property-image-crop-source");
  if (!cached?.path || !cached.width || !cached.height) { uni.showToast({ title: "没有可处理的图片", icon: "none" }); setTimeout(() => uni.navigateBack(), 300); return; }
  source.value = cached;
  setTimeout(drawPreview, 120);
});
</script>

<template>
  <view class="crop-page">
    <view class="intro"><strong>调整展示位置</strong><text>单指拖动图片调整位置，双指捏合缩放；白框内的区域会作为房源图片。</text></view>
    <view class="canvas-wrap"><canvas canvas-id="property-crop-canvas" class="crop-canvas" :width="CANVAS_WIDTH" :height="CANVAS_HEIGHT" /><view class="crop-frame"><view class="frame-label">拖动图片</view></view><view class="touch-layer" @touchstart="touchStart" @touchmove.stop.prevent="touchMove" @touchend="touchEnd" @touchcancel="touchEnd"></view></view>
    <view class="crop-controls"><text>图片缩放 {{ zoomLabel }}</text><slider :value="zoom" :min="minZoom" :max="220" :step="1" activeColor="#536a45" backgroundColor="#dfe9e2" @change="updateZoom" @changing="updateZoom" /><button class="reset-button" @click="resetCrop">恢复居中</button></view>
    <view class="bottom-actions"><button class="secondary" @click="cancel">取消</button><button class="primary" :loading="saving" @click="finish">使用此裁切</button></view>
  </view>
</template>

<style scoped>
.crop-page{min-height:100vh;padding:32rpx 24rpx 48rpx;background:#f7f5ef;box-sizing:border-box}.intro strong,.intro text{display:block}.intro strong{color:#304530;font-size:34rpx;letter-spacing:-1rpx}.intro text{margin-top:12rpx;color:#71806e;font-size:24rpx;line-height:1.55}.canvas-wrap{position:relative;width:100%;margin-top:28rpx;overflow:hidden;border-radius:22rpx;background:#17211d;box-shadow:0 16rpx 36rpx rgba(44,65,48,.18)}.crop-canvas{display:block;width:100%;height:513rpx}.crop-frame{position:absolute;left:12.5%;top:12.5%;width:75%;height:75%;box-sizing:border-box;border:3rpx solid #fff;box-shadow:0 0 0 2000rpx rgba(0,0,0,.38);pointer-events:none}.frame-label{position:absolute;left:50%;top:50%;padding:8rpx 14rpx;border-radius:12rpx;transform:translate(-50%,-50%);background:rgba(21,32,26,.62);color:#fff;font-size:21rpx;white-space:nowrap}.touch-layer{position:absolute;z-index:2;inset:0}.crop-controls{margin-top:26rpx;padding:24rpx;border-radius:20rpx;background:#fff}.crop-controls>text{display:block;color:#52634f;font-size:24rpx;font-weight:600}.crop-controls slider{margin:12rpx -12rpx 12rpx}.reset-button{height:64rpx;margin:0;border:0;border-radius:14rpx;background:#eef3eb;color:#445741;font-size:23rpx;line-height:64rpx}.bottom-actions{display:grid;grid-template-columns:1fr 1.7fr;gap:16rpx;margin-top:30rpx}.bottom-actions button{height:88rpx;margin:0;border:0;border-radius:18rpx;font-size:27rpx;line-height:88rpx}.secondary{background:#e8eee5;color:#4d6149}.primary{background:#536a45;color:#fff}
</style>
