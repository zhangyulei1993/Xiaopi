<script setup lang="ts">
import { onLoad, onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { listMyRentalRequests, type RentalRequest } from "../../api/properties";
import { getSession } from "../../auth/session";

const items = ref<RentalRequest[]>([]);
const loading = ref(true);
const typeLabel = (type: RentalRequest["requestType"]) => ({ appointment: "预约看房", area_subscription: "片区订阅", general: "找房需求" }[type] || "找房需求");
const statusLabel = (status?: string) => ({ new: "待联系", contacted: "已联系", closed: "已完成" }[String(status || "new")] || "处理中");
const statusClass = (status?: string) => `status-${String(status || "new")}`;
const titleOf = (item: RentalRequest) => item.requestType === "appointment"
  ? item.location || "预约房源"
  : item.requestType === "area_subscription"
    ? `${item.location || "所选片区"} 新房提醒`
    : item.location || "找房需求";
const detailOf = (item: RentalRequest) => {
  if (item.requestType === "appointment") return `${item.contactName || "您"} · ${item.preferredTime || "时间待确认"}`;
  const budget = item.budgetMin || item.budgetMax ? `${item.budgetMin || 0}–${item.budgetMax || "不限"} 元/月` : "预算待补充";
  return [item.layout, budget].filter(Boolean).join(" · ");
};
const dateOf = (item: RentalRequest) => item.createdAt ? new Date(item.createdAt).toLocaleDateString() : "刚刚提交";
const activeCount = computed(() => items.value.filter((item) => !["closed", "completed"].includes(item.status || "new")).length);

const load = async () => {
  if (getSession()?.role !== "user") return;
  loading.value = true;
  try { items.value = await listMyRentalRequests(); }
  catch (error) { uni.showToast({ title: error instanceof Error ? error.message : "记录加载失败", icon: "none" }); }
  finally { loading.value = false; }
};
const goFind = () => uni.switchTab({ url: "/pages/properties/index" });
onLoad(() => {
  if (getSession()?.role === "user") return;
  uni.navigateTo({ url: "/pages/auth/login?reason=requests" });
});
onShow(() => { void load(); });
</script>

<template>
  <view class="page">
    <view class="hero"><view><text>我的找房记录</text><small>预约、订阅和找房需求都会保留在这里</small></view><view class="count"><strong>{{ activeCount }}</strong><small>进行中</small></view></view>
    <view v-if="!loading && !items.length" class="empty"><strong>暂时没有找房记录</strong><text>提交需求或预约看房后，会在这里查看进度。</text><button @click="goFind">去找房</button></view>
    <view v-else class="list"><view v-for="item in items" :key="item.id" class="record"><view class="record-head"><view><strong>{{ typeLabel(item.requestType) }}</strong><text>{{ titleOf(item) }}</text></view><text class="status" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</text></view><view class="record-detail">{{ detailOf(item) }}</view><view v-if="item.followUpNote" class="follow-up"><text>小皮跟进</text><strong>{{ item.followUpNote }}</strong></view><view class="record-foot"><text>{{ dateOf(item) }}</text><text v-if="item.requestType === 'appointment'">房源预约</text></view></view></view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:28rpx 32rpx calc(60rpx + env(safe-area-inset-bottom));background:#f7f6f1}.hero{display:flex;align-items:flex-end;justify-content:space-between;margin:8rpx 4rpx 30rpx}.hero>view:first-child{min-width:0}.hero text{display:block;color:#29382d;font-size:42rpx;font-weight:850;letter-spacing:-1rpx}.hero small{display:block;margin-top:10rpx;color:#7e887d;font-size:22rpx;line-height:1.45}.count{min-width:92rpx;padding:12rpx;border-radius:16rpx;background:#e9eedc;text-align:center}.count strong{display:block;color:#40543f;font-size:32rpx;line-height:1}.count small{margin-top:5rpx;font-size:18rpx}.list{display:grid;gap:16rpx}.record{padding:24rpx;border:1rpx solid #e8e4db;border-radius:22rpx;background:#fffdf9}.record-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16rpx}.record-head>view{min-width:0}.record-head strong{display:block;color:#2d3d30;font-size:28rpx;line-height:1.3}.record-head>view>text{display:block;overflow:hidden;margin-top:8rpx;color:#728070;font-size:23rpx;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.status{flex:none;padding:7rpx 11rpx;border-radius:9rpx;background:#edf0dc;color:#526a45;font-size:19rpx;font-weight:700}.status-contacted{background:#e8f0ec;color:#39705a}.status-closed{background:#f0efea;color:#858a81}.record-detail{margin-top:18rpx;color:#52604f;font-size:23rpx;line-height:1.55}.follow-up{margin-top:18rpx;padding:15rpx 16rpx;border-radius:13rpx;background:#f2f4ed}.follow-up text{display:block;color:#8b9488;font-size:19rpx}.follow-up strong{display:block;margin-top:5rpx;color:#465544;font-size:22rpx;font-weight:500;line-height:1.45}.record-foot{display:flex;justify-content:space-between;margin-top:20rpx;padding-top:16rpx;border-top:1rpx solid #efede7;color:#929990;font-size:19rpx}.empty{display:flex;align-items:center;flex-direction:column;margin-top:70rpx;padding:90rpx 34rpx;border:1rpx dashed #dce2d5;border-radius:24rpx;background:#fbfaf6;text-align:center}.empty strong{color:#53634e;font-size:29rpx}.empty text{margin-top:12rpx;color:#8b9488;font-size:22rpx;line-height:1.6}.empty button{margin-top:30rpx;padding:0 30rpx;border:0;border-radius:14rpx;background:#40543f;color:#fff;font-size:24rpx;line-height:72rpx}.empty button::after{border:0}
</style>
