<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { computed, ref } from "vue";

type Source = { path: string; width: number; height: number };
const SIZE = 600;
const source = ref<Source | null>(null);
const zoom = ref(100);
const offsetX = ref(0);
const offsetY = ref(0);
const saving = ref(false);
let drag: { x: number; y: number; offsetX: number; offsetY: number } | null = null;
let pinch: { distance: number; zoom: number } | null = null;
const baseScale = computed(() => source.value ? Math.max(SIZE / source.value.width, SIZE / source.value.height) : 1);
const scale = computed(() => baseScale.value * zoom.value / 100);
const width = computed(() => (source.value?.width || 0) * scale.value);
const height = computed(() => (source.value?.height || 0) * scale.value);
const left = computed(() => (SIZE - width.value) / 2 + offsetX.value);
const top = computed(() => (SIZE - height.value) / 2 + offsetY.value);
const imageStyle = computed(() => ({ width: `${width.value / SIZE * 100}%`, height: `${height.value / SIZE * 100}%`, left: `${left.value / SIZE * 100}%`, top: `${top.value / SIZE * 100}%` }));
const point = (touch: any) => ({ x: Number(touch?.clientX || touch?.pageX || 0), y: Number(touch?.clientY || touch?.pageY || 0) });
const distance = (a: any, b: any) => { const first = point(a); const second = point(b); return Math.hypot(first.x - second.x, first.y - second.y); };
const clamp = () => {
  const centeredLeft = (SIZE - width.value) / 2;
  const centeredTop = (SIZE - height.value) / 2;
  offsetX.value = Math.max(SIZE - centeredLeft - width.value, Math.min(-centeredLeft, offsetX.value));
  offsetY.value = Math.max(SIZE - centeredTop - height.value, Math.min(-centeredTop, offsetY.value));
};
const setZoom = (value: number) => { zoom.value = Math.max(100, Math.min(260, value)); clamp(); };
const touchStart = (event: any) => {
  const touches = event.touches || [];
  if (touches.length > 1) { pinch = { distance: distance(touches[0], touches[1]), zoom: zoom.value }; drag = null; return; }
  if (touches.length) { const current = point(touches[0]); drag = { ...current, offsetX: offsetX.value, offsetY: offsetY.value }; pinch = null; }
};
const touchMove = (event: any) => {
  const touches = event.touches || [];
  if (touches.length > 1 && pinch) { setZoom(Math.round(pinch.zoom * distance(touches[0], touches[1]) / Math.max(1, pinch.distance))); return; }
  if (!drag || !touches.length) return;
  const current = point(touches[0]);
  const factor = SIZE / Math.max(1, (uni.getSystemInfoSync().windowWidth || 375) - 64);
  offsetX.value = drag.offsetX + (current.x - drag.x) * factor;
  offsetY.value = drag.offsetY + (current.y - drag.y) * factor;
  clamp();
};
const finish = () => {
  if (!source.value || saving.value) return;
  saving.value = true; clamp();
  const context = uni.createCanvasContext("avatar-crop-canvas");
  context.drawImage(source.value.path, -left.value / scale.value, -top.value / scale.value, SIZE / scale.value, SIZE / scale.value, 0, 0, SIZE, SIZE);
  context.draw(false, () => uni.canvasToTempFilePath({ canvasId: "avatar-crop-canvas", destWidth: SIZE, destHeight: SIZE, quality: .92,
    success: (result) => { uni.setStorageSync("xiaopi-avatar-crop-result", result.tempFilePath); uni.navigateBack(); },
    fail: () => { saving.value = false; uni.showToast({ title: "头像处理失败", icon: "none" }); },
  }));
};
onLoad(() => {
  const cached = uni.getStorageSync("xiaopi-avatar-crop-source") as Source;
  uni.removeStorageSync("xiaopi-avatar-crop-source");
  if (!cached?.path || !cached.width || !cached.height) { uni.navigateBack(); return; }
  source.value = cached;
});
</script>

<template>
  <view class="page">
    <view class="heading"><strong>调整头像</strong><text>拖动选择头像区域，双指可缩放</text></view>
    <view class="stage"><image v-if="source" :src="source.path" :style="imageStyle" mode="scaleToFill" /><view class="circle"></view><view class="touch" @touchstart="touchStart" @touchmove.stop.prevent="touchMove" @touchend="drag=null;pinch=null" /></view>
    <canvas canvas-id="avatar-crop-canvas" class="canvas" :width="SIZE" :height="SIZE" />
    <view class="control"><text>缩放 {{ zoom }}%</text><slider :value="zoom" min="100" max="260" activeColor="#40543f" @changing="setZoom(Number($event.detail.value))" @change="setZoom(Number($event.detail.value))" /></view>
    <view class="actions"><button @click="uni.navigateBack()">取消</button><button class="primary" :loading="saving" @click="finish">使用此头像</button></view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:42rpx 32rpx calc(40rpx + env(safe-area-inset-bottom));box-sizing:border-box;background:#f7f5ef}.heading strong,.heading text{display:block}.heading strong{color:#304530;font-size:38rpx}.heading text{margin-top:10rpx;color:#7a8677;font-size:23rpx}.stage{position:relative;width:100%;height:686rpx;margin-top:34rpx;overflow:hidden;border-radius:28rpx;background:#18211c}.stage image{position:absolute}.circle{position:absolute;z-index:1;left:50%;top:50%;width:530rpx;height:530rpx;border:3rpx solid #fff;border-radius:50%;transform:translate(-50%,-50%);box-shadow:0 0 0 200rpx rgba(10,18,13,.52);pointer-events:none}.touch{position:absolute;z-index:2;inset:0}.canvas{position:fixed;left:-1200px;top:-1200px;width:600px;height:600px}.control{margin-top:24rpx;padding:22rpx;border-radius:20rpx;background:#fff;color:#53634f;font-size:23rpx}.actions{display:grid;grid-template-columns:1fr 1.8fr;gap:16rpx;margin-top:26rpx}.actions button{margin:0;border:0;border-radius:18rpx;background:#e8eee5;color:#4d6149;font-size:27rpx;line-height:88rpx}.actions button::after{border:0}.actions .primary{background:#40543f;color:#fff}
</style>
