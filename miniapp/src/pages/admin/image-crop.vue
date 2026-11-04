<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { computed, ref } from "vue";

type CropSource = { index: number; path: string; width: number; height: number; size: number };
type Point = { x: number; y: number };
const OUTPUT_WIDTH = 1200;
const OUTPUT_HEIGHT = 900;
const source = ref<CropSource | null>(null);
const zoom = ref(130);
const offsetX = ref(0);
const offsetY = ref(0);
const saving = ref(false);
let drag: { point: Point; x: number; y: number } | null = null;
let pinch: { distance: number; zoom: number } | null = null;

const baseScale = computed(() => source.value ? Math.max(OUTPUT_WIDTH / source.value.width, OUTPUT_HEIGHT / source.value.height) : 1);
const scale = computed(() => baseScale.value * zoom.value / 100);
const renderWidth = computed(() => source.value ? source.value.width * scale.value : 0);
const renderHeight = computed(() => source.value ? source.value.height * scale.value : 0);
const imageLeft = computed(() => (OUTPUT_WIDTH - renderWidth.value) / 2 + offsetX.value);
const imageTop = computed(() => (OUTPUT_HEIGHT - renderHeight.value) / 2 + offsetY.value);
const imageStyle = computed(() => ({
  width: `${renderWidth.value / OUTPUT_WIDTH * 100}%`,
  height: `${renderHeight.value / OUTPUT_HEIGHT * 100}%`,
  left: `${imageLeft.value / OUTPUT_WIDTH * 100}%`,
  top: `${imageTop.value / OUTPUT_HEIGHT * 100}%`,
}));
const zoomLabel = computed(() => `${zoom.value}%`);

const clampImage = () => {
  const centeredLeft = (OUTPUT_WIDTH - renderWidth.value) / 2;
  const centeredTop = (OUTPUT_HEIGHT - renderHeight.value) / 2;
  const minX = OUTPUT_WIDTH - centeredLeft - renderWidth.value;
  const maxX = -centeredLeft;
  const minY = OUTPUT_HEIGHT - centeredTop - renderHeight.value;
  const maxY = -centeredTop;
  offsetX.value = Math.max(minX, Math.min(maxX, offsetX.value));
  offsetY.value = Math.max(minY, Math.min(maxY, offsetY.value));
};
const setZoom = (value: number) => {
  zoom.value = Math.max(100, Math.min(220, value));
  clampImage();
};
const reset = () => { zoom.value = 100; offsetX.value = 0; offsetY.value = 0; };
const point = (touch: any): Point => ({ x: Number(touch?.clientX || touch?.pageX || 0), y: Number(touch?.clientY || touch?.pageY || 0) });
const distance = (first: any, second: any) => { const a = point(first); const b = point(second); return Math.hypot(a.x - b.x, a.y - b.y); };
const onTouchStart = (event: any) => {
  const touches = event.touches || [];
  if (touches.length >= 2) { pinch = { distance: distance(touches[0], touches[1]), zoom: zoom.value }; drag = null; return; }
  if (touches.length) { drag = { point: point(touches[0]), x: offsetX.value, y: offsetY.value }; pinch = null; }
};
const onTouchMove = (event: any) => {
  const touches = event.touches || [];
  if (touches.length >= 2) {
    if (!pinch) { pinch = { distance: distance(touches[0], touches[1]), zoom: zoom.value }; return; }
    setZoom(Math.round(pinch.zoom * distance(touches[0], touches[1]) / Math.max(1, pinch.distance)));
    return;
  }
  if (!drag || !touches.length) return;
  const current = point(touches[0]);
  const factor = OUTPUT_WIDTH / Math.max(1, (uni.getSystemInfoSync().windowWidth || 375) - 48);
  offsetX.value = drag.x + (current.x - drag.point.x) * factor;
  offsetY.value = drag.y + (current.y - drag.point.y) * factor;
  clampImage();
};
const onTouchEnd = () => { drag = null; pinch = null; };
const updateSlider = (event: { detail: { value: number } }) => setZoom(Number(event.detail.value) || 100);
const complete = (path: string, size: number) => {
  uni.setStorageSync("xiaopi-property-image-crop-result", { index: source.value?.index, path, width: OUTPUT_WIDTH, height: OUTPUT_HEIGHT, size });
  uni.navigateBack();
};
const finish = () => {
  if (!source.value || saving.value) return;
  saving.value = true;
  clampImage();
  const sourceX = (0 - imageLeft.value) / scale.value;
  const sourceY = (0 - imageTop.value) / scale.value;
  const context = uni.createCanvasContext("crop-export-canvas");
  context.drawImage(source.value.path, sourceX, sourceY, OUTPUT_WIDTH / scale.value, OUTPUT_HEIGHT / scale.value, 0, 0, OUTPUT_WIDTH, OUTPUT_HEIGHT);
  context.draw(false, () => uni.canvasToTempFilePath({
    canvasId: "crop-export-canvas", destWidth: OUTPUT_WIDTH, destHeight: OUTPUT_HEIGHT, quality: 0.92,
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
  clampImage();
});
</script>

<template>
  <view class="crop-page">
    <view class="intro"><strong>调整展示位置</strong><text>单指拖动图片调整位置，双指捏合缩放。当前画面就是最终房源图。</text></view>
    <view class="crop-stage"><image v-if="source" class="crop-image" :src="source.path" :style="imageStyle" mode="scaleToFill" /><view class="crop-tip">拖动图片 · 双指缩放</view><view class="touch-layer" @touchstart="onTouchStart" @touchmove.stop.prevent="onTouchMove" @touchend="onTouchEnd" @touchcancel="onTouchEnd"></view></view>
    <canvas canvas-id="crop-export-canvas" class="export-canvas" :width="OUTPUT_WIDTH" :height="OUTPUT_HEIGHT" />
    <view class="crop-controls"><text>图片缩放 {{ zoomLabel }}</text><slider :value="zoom" :min="100" :max="220" :step="1" activeColor="#536a45" backgroundColor="#dfe9e2" @change="updateSlider" @changing="updateSlider" /><button class="reset-button" @click="reset">恢复全图居中</button></view>
    <view class="bottom-actions"><button class="secondary" @click="cancel">取消</button><button class="primary" :loading="saving" @click="finish">使用此裁切</button></view>
  </view>
</template>

<style scoped>
.crop-page{min-height:100vh;padding:32rpx 24rpx 48rpx;background:#f7f5ef;box-sizing:border-box}.intro strong,.intro text{display:block}.intro strong{color:#304530;font-size:34rpx;letter-spacing:-1rpx}.intro text{margin-top:12rpx;color:#71806e;font-size:24rpx;line-height:1.55}.crop-stage{position:relative;width:100%;height:513rpx;margin-top:28rpx;overflow:hidden;border-radius:22rpx;background:#17211d;box-shadow:0 16rpx 36rpx rgba(44,65,48,.18)}.crop-image{position:absolute;display:block}.crop-tip{position:absolute;z-index:1;left:50%;top:50%;padding:8rpx 14rpx;border-radius:12rpx;transform:translate(-50%,-50%);background:rgba(21,32,26,.62);color:#fff;font-size:21rpx;white-space:nowrap;pointer-events:none}.touch-layer{position:absolute;z-index:2;inset:0}.export-canvas{position:fixed;left:-2000px;top:-2000px;width:1200px;height:900px;opacity:0}.crop-controls{margin-top:26rpx;padding:24rpx;border-radius:20rpx;background:#fff}.crop-controls>text{display:block;color:#52634f;font-size:24rpx;font-weight:600}.crop-controls slider{margin:12rpx -12rpx}.reset-button{height:64rpx;margin:0;border:0;border-radius:14rpx;background:#eef3eb;color:#445741;font-size:23rpx;line-height:64rpx}.bottom-actions{display:grid;grid-template-columns:1fr 1.7fr;gap:16rpx;margin-top:30rpx}.bottom-actions button{height:88rpx;margin:0;border:0;border-radius:18rpx;font-size:27rpx;line-height:88rpx}.secondary{background:#e8eee5;color:#4d6149}.primary{background:#536a45;color:#fff}
</style>
