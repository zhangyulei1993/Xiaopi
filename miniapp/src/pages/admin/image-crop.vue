<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { computed, ref } from "vue";

type CropSource = { index: number; path: string; width: number; height: number; size: number };
const CANVAS_WIDTH = 1200;
const CANVAS_HEIGHT = 900;
const FRAME_WIDTH = 900;
const FRAME_HEIGHT = 675;
const source = ref<CropSource | null>(null);
const zoom = ref(130);
const frameX = ref((CANVAS_WIDTH - FRAME_WIDTH) / 2);
const frameY = ref((CANVAS_HEIGHT - FRAME_HEIGHT) / 2);
const saving = ref(false);
let dragStart: { x: number; y: number; frameX: number; frameY: number } | null = null;

const baseScale = computed(() => source.value ? Math.max(CANVAS_WIDTH / source.value.width, CANVAS_HEIGHT / source.value.height) : 1);
const minZoom = computed(() => {
  if (!source.value) return 70;
  const required = Math.max(FRAME_WIDTH / source.value.width, FRAME_HEIGHT / source.value.height);
  return Math.max(55, Math.min(100, Math.ceil((required / baseScale.value) * 100)));
});
const scale = computed(() => baseScale.value * (zoom.value / 100));
const renderedWidth = computed(() => source.value ? source.value.width * scale.value : 0);
const renderedHeight = computed(() => source.value ? source.value.height * scale.value : 0);
const imageLeft = computed(() => (CANVAS_WIDTH - renderedWidth.value) / 2);
const imageTop = computed(() => (CANVAS_HEIGHT - renderedHeight.value) / 2);
const zoomLabel = computed(() => `${zoom.value}%`);
const frameStyle = computed(() => ({ width: `${FRAME_WIDTH / CANVAS_WIDTH * 100}%`, height: `${FRAME_HEIGHT / CANVAS_HEIGHT * 100}%`, left: `${frameX.value / CANVAS_WIDTH * 100}%`, top: `${frameY.value / CANVAS_HEIGHT * 100}%` }));

const clampFrame = () => {
  const left = Math.max(0, imageLeft.value);
  const top = Math.max(0, imageTop.value);
  const right = Math.min(CANVAS_WIDTH - FRAME_WIDTH, imageLeft.value + renderedWidth.value - FRAME_WIDTH);
  const bottom = Math.min(CANVAS_HEIGHT - FRAME_HEIGHT, imageTop.value + renderedHeight.value - FRAME_HEIGHT);
  frameX.value = Math.max(left, Math.min(Math.max(left, right), frameX.value));
  frameY.value = Math.max(top, Math.min(Math.max(top, bottom), frameY.value));
};
const drawPreview = () => {
  if (!source.value) return;
  clampFrame();
  const context = uni.createCanvasContext("property-crop-canvas");
  context.setFillStyle("#17211d");
  context.fillRect(0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);
  context.drawImage(source.value.path, imageLeft.value, imageTop.value, renderedWidth.value, renderedHeight.value);
  context.draw();
};
const resetCrop = () => { zoom.value = 100; frameX.value = (CANVAS_WIDTH - FRAME_WIDTH) / 2; frameY.value = (CANVAS_HEIGHT - FRAME_HEIGHT) / 2; drawPreview(); };
const nudge = (x: number, y: number) => { frameX.value += x; frameY.value += y; drawPreview(); };
const updateZoom = (event: { detail: { value: number } }) => { zoom.value = Math.max(minZoom.value, Number(event.detail.value) || minZoom.value); drawPreview(); };
const point = (event: any) => event.touches?.[0] || event.changedTouches?.[0];
const dragStartHandler = (event: any) => {
  const touch = point(event); if (!touch) return;
  dragStart = { x: Number(touch.clientX || touch.pageX || 0), y: Number(touch.clientY || touch.pageY || 0), frameX: frameX.value, frameY: frameY.value };
};
const dragMoveHandler = (event: any) => {
  const touch = point(event); if (!dragStart || !touch) return;
  const factor = CANVAS_WIDTH / Math.max(1, (uni.getSystemInfoSync().windowWidth || 375) - 48);
  frameX.value = dragStart.frameX + (Number(touch.clientX || touch.pageX || 0) - dragStart.x) * factor;
  frameY.value = dragStart.frameY + (Number(touch.clientY || touch.pageY || 0) - dragStart.y) * factor;
  drawPreview();
};
const complete = (path: string, size: number) => {
  uni.setStorageSync("xiaopi-property-image-crop-result", { index: source.value?.index, path, width: CANVAS_WIDTH, height: CANVAS_HEIGHT, size });
  uni.navigateBack();
};
const finish = () => {
  if (!source.value || saving.value) return;
  saving.value = true; clampFrame();
  const sourceX = (frameX.value - imageLeft.value) / scale.value;
  const sourceY = (frameY.value - imageTop.value) / scale.value;
  const context = uni.createCanvasContext("property-crop-canvas");
  context.drawImage(source.value.path, sourceX, sourceY, FRAME_WIDTH / scale.value, FRAME_HEIGHT / scale.value, 0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);
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
  source.value = cached; setTimeout(drawPreview, 120);
});
</script>

<template>
  <view class="crop-page">
    <view class="intro"><strong>调整展示位置</strong><text>拖动白色裁切框选择保留区域；滑杆可放大或缩小图片。</text></view>
    <view class="canvas-wrap" @touchstart="dragStartHandler" @touchmove.stop.prevent="dragMoveHandler"><canvas canvas-id="property-crop-canvas" class="crop-canvas" :width="CANVAS_WIDTH" :height="CANVAS_HEIGHT" /><view class="crop-shade"></view><view class="crop-frame" :style="frameStyle"><view class="frame-label">拖动裁切框</view></view></view>
    <view class="crop-controls"><text>图片缩放 {{ zoomLabel }}</text><slider :value="zoom" :min="minZoom" :max="220" :step="1" activeColor="#536a45" backgroundColor="#dfe9e2" @change="updateZoom" @changing="updateZoom" /><view class="nudge-row"><button @click="nudge(0, -60)">↑</button><button @click="nudge(-60, 0)">←</button><button @click="resetCrop">居中</button><button @click="nudge(60, 0)">→</button><button @click="nudge(0, 60)">↓</button></view></view>
    <view class="bottom-actions"><button class="secondary" @click="cancel">取消</button><button class="primary" :loading="saving" @click="finish">使用此裁切</button></view>
  </view>
</template>

<style scoped>
.crop-page{min-height:100vh;padding:32rpx 24rpx 48rpx;background:#f7f5ef;box-sizing:border-box}.intro strong,.intro text{display:block}.intro strong{color:#304530;font-size:34rpx;letter-spacing:-1rpx}.intro text{margin-top:12rpx;color:#71806e;font-size:24rpx;line-height:1.55}.canvas-wrap{position:relative;width:100%;margin-top:28rpx;overflow:hidden;border-radius:22rpx;background:#17211d;box-shadow:0 16rpx 36rpx rgba(44,65,48,.18)}.crop-canvas{display:block;width:100%;height:513rpx}.crop-shade{position:absolute;inset:0;pointer-events:none;background:rgba(0,0,0,.22)}.crop-frame{position:absolute;box-sizing:border-box;border:3rpx solid #fff;box-shadow:0 0 0 2000rpx rgba(0,0,0,.18)}.frame-label{position:absolute;left:50%;top:50%;padding:8rpx 14rpx;border-radius:12rpx;transform:translate(-50%,-50%);background:rgba(21,32,26,.62);color:#fff;font-size:21rpx;white-space:nowrap}.crop-controls{margin-top:26rpx;padding:24rpx;border-radius:20rpx;background:#fff}.crop-controls>text{display:block;color:#52634f;font-size:24rpx;font-weight:600}.crop-controls slider{margin:12rpx -12rpx 8rpx}.nudge-row{display:grid;grid-template-columns:repeat(5,1fr);gap:10rpx}.nudge-row button,.bottom-actions button{margin:0;border:0}.nudge-row button{padding:0;height:64rpx;border-radius:14rpx;background:#eef3eb;color:#445741;font-size:23rpx;line-height:64rpx}.bottom-actions{display:grid;grid-template-columns:1fr 1.7fr;gap:16rpx;margin-top:30rpx}.bottom-actions button{height:88rpx;border-radius:18rpx;font-size:27rpx;line-height:88rpx}.secondary{background:#e8eee5;color:#4d6149}.primary{background:#536a45;color:#fff}
</style>
