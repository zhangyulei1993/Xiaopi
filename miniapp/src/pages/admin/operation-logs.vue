<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { queryOperationLogs, type OperationLog } from "../../api/properties";
import { hasRole } from "../../auth/session";

const categories = [
  { value: "all", label: "全部" },
  { value: "property", label: "房源、素材与门牌" },
  { value: "customer", label: "客户与需求" },
  { value: "message", label: "消息通知" },
];
const category = ref("all");
const startDate = ref("");
const endDate = ref("");
const page = ref(1);
const pageSize = 20;
const items = ref<OperationLog[]>([]);
const total = ref(0);
const pageCount = ref(0);
const loading = ref(false);
const failed = ref(false);
const currentCategoryLabel = computed(() => categories.find((item) => item.value === category.value)?.label || "操作");
let loadSequence = 0;

const load = async () => {
  const sequence = ++loadSequence;
  loading.value = true;
  failed.value = false;
  try {
    const result = await queryOperationLogs({ category: category.value, startDate: startDate.value, endDate: endDate.value, page: page.value, pageSize });
    if (sequence !== loadSequence) return;
    items.value = result.items || [];
    total.value = result.total || 0;
    pageCount.value = result.pageCount || 0;
  } catch (error) {
    if (sequence !== loadSequence) return;
    failed.value = true;
    uni.showToast({ title: error instanceof Error ? error.message : "操作记录加载失败", icon: "none" });
  } finally {
    if (sequence === loadSequence) loading.value = false;
  }
};
const selectCategory = (value: string) => {
  if (category.value === value) return;
  category.value = value;
  page.value = 1;
  void load();
};
const chooseStartDate = (event: { detail: { value: string } }) => {
  if (endDate.value && event.detail.value > endDate.value) {
    uni.showToast({ title: "开始日期不能晚于结束日期", icon: "none" });
    return;
  }
  startDate.value = event.detail.value;
  page.value = 1;
  void load();
};
const chooseEndDate = (event: { detail: { value: string } }) => {
  if (startDate.value && event.detail.value < startDate.value) {
    uni.showToast({ title: "结束日期不能早于开始日期", icon: "none" });
    return;
  }
  endDate.value = event.detail.value;
  page.value = 1;
  void load();
};
const clearDates = () => {
  if (!startDate.value && !endDate.value) return;
  startDate.value = "";
  endDate.value = "";
  page.value = 1;
  void load();
};
const turnPage = (delta: number) => {
  const next = page.value + delta;
  if (next < 1 || next > pageCount.value || loading.value) return;
  page.value = next;
  void load();
};
const categoryOf = (type: string) => ({
  property: "房源", material: "素材库", building_rule: "门牌数据",
  lead: "房源线索", request: "客户需求", subscription: "消息通知",
} as Record<string, string>)[type] || "其他";
const dateText = (value: string) => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (num: number) => String(num).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
};
onShow(() => {
  if (!hasRole("admin")) {
    uni.showToast({ title: "仅管理员可查看", icon: "none" });
    uni.switchTab({ url: "/pages/me/index" });
    return;
  }
  void load();
});
</script>

<template>
  <view class="page">
    <view class="page-heading"><text class="eyebrow">运营留痕</text><view>操作记录</view><text class="subtitle">查看房源、客户跟进和消息通知等操作</text></view>
    <view class="filter-card">
      <scroll-view class="category-scroll" scroll-x :show-scrollbar="false"><view class="category-row"><button v-for="item in categories" :id="`operation-category-${item.value}`" :key="item.value" class="category-chip" :class="{ active: category === item.value }" @click="selectCategory(item.value)">{{ item.label }}</button></view></scroll-view>
      <view class="date-filter"><text class="filter-caption">时间区间</text><picker id="operation-start-date" mode="date" :value="startDate" @change="chooseStartDate"><view class="date-button" :class="{ placeholder: !startDate }">{{ startDate || "开始日期" }}</view></picker><text class="date-separator">至</text><picker id="operation-end-date" mode="date" :value="endDate" @change="chooseEndDate"><view class="date-button" :class="{ placeholder: !endDate }">{{ endDate || "结束日期" }}</view></picker><text v-if="startDate || endDate" class="clear-dates" @click="clearDates">清除</text></view>
    </view>
    <view class="results-heading"><view><strong>{{ currentCategoryLabel }}</strong><text>{{ total }} 条记录</text></view><text v-if="startDate || endDate">{{ startDate || "不限" }} — {{ endDate || "不限" }}</text></view>
    <view v-if="loading && !items.length" class="state-card">正在加载操作记录…</view>
    <view v-else-if="failed && !items.length" class="state-card"><text>操作记录暂时无法加载</text><button @click="load">重新加载</button></view>
    <view v-else-if="!items.length" class="state-card"><strong>暂无操作记录</strong><text>调整分类或时间范围后再查看</text></view>
    <view v-else class="log-list"><view v-for="item in items" :key="item.id" class="log-card"><view class="log-top"><text class="log-category">{{ categoryOf(item.targetType) }}</text><text class="log-time">{{ dateText(item.createdAt) }}</text></view><strong class="log-action">{{ item.action }}</strong><text v-if="item.detail" class="log-detail">{{ item.detail }}</text><text v-if="item.targetId" class="log-target">记录编号 {{ item.targetId }}</text></view></view>
    <view v-if="pageCount > 1" class="pagination"><button id="operation-page-previous" :disabled="page <= 1 || loading" @click="turnPage(-1)">上一页</button><text>第 {{ page }} / {{ pageCount }} 页</text><button id="operation-page-next" :disabled="page >= pageCount || loading" @click="turnPage(1)">下一页</button></view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;box-sizing:border-box;padding:30rpx 28rpx calc(50rpx + env(safe-area-inset-bottom));color:#26342b;background:#f7f6f1}.page-heading{margin:6rpx 4rpx 24rpx}.eyebrow{color:#879180;font-size:20rpx;font-weight:700;letter-spacing:2rpx}.page-heading>view{margin-top:7rpx;color:#293b2d;font-size:42rpx;font-weight:800;line-height:1.25}.subtitle{display:block;margin-top:9rpx;color:#7c8679;font-size:23rpx}.filter-card,.log-card,.state-card{border:1rpx solid #e8e7df;border-radius:20rpx;background:#fff}.filter-card{overflow:hidden;padding:18rpx 18rpx 20rpx}.category-scroll{white-space:nowrap}.category-row{display:inline-flex;gap:10rpx}.category-chip{flex:none;margin:0;padding:0 22rpx;border:1rpx solid #dfe5d9;border-radius:999rpx;background:#fff;color:#62705f;font-size:22rpx;font-weight:650;line-height:58rpx}.category-chip::after{border:0}.category-chip.active{border-color:#40543f;background:#40543f;color:#fff}.date-filter{display:flex;align-items:center;gap:10rpx;margin-top:18rpx}.filter-caption{flex:none;color:#677366;font-size:21rpx;font-weight:700}.date-button{min-width:0;padding:12rpx 14rpx;border:1rpx solid #e3e7df;border-radius:10rpx;color:#40543f;font-size:20rpx;line-height:1.25}.date-button.placeholder{color:#9ba398}.date-separator{color:#90998f;font-size:20rpx}.clear-dates{flex:none;padding:8rpx;color:#71816e;font-size:20rpx}.results-heading{display:flex;align-items:center;justify-content:space-between;margin:25rpx 4rpx 14rpx;color:#879083;font-size:20rpx}.results-heading>view{display:flex;align-items:baseline;gap:13rpx}.results-heading strong{color:#344633;font-size:27rpx}.log-list{display:flex;flex-direction:column;gap:12rpx}.log-card{padding:21rpx 22rpx}.log-top{display:flex;align-items:center;justify-content:space-between;gap:14rpx}.log-category{padding:6rpx 11rpx;border-radius:999rpx;background:#eef2e9;color:#52664c;font-size:19rpx;font-weight:700}.log-time{color:#929a91;font-size:19rpx}.log-action{display:block;margin-top:13rpx;color:#293b2d;font-size:26rpx;line-height:1.4}.log-detail,.log-target{display:block;margin-top:8rpx;color:#6f796d;font-size:22rpx;line-height:1.5;word-break:break-all}.log-target{color:#a0a69e;font-size:18rpx}.state-card{display:flex;min-height:250rpx;flex-direction:column;align-items:center;justify-content:center;gap:14rpx;color:#778173;font-size:22rpx}.state-card strong{color:#3c4c3b;font-size:27rpx}.state-card button{margin:8rpx 0 0;padding:0 24rpx;border-radius:10rpx;background:#40543f;color:#fff;font-size:22rpx;line-height:62rpx}.state-card button::after{border:0}.pagination{display:flex;align-items:center;justify-content:center;gap:24rpx;margin:22rpx 0}.pagination button{margin:0;padding:0 20rpx;border:1rpx solid #dfe5d9;border-radius:10rpx;background:#fff;color:#40543f;font-size:21rpx;line-height:58rpx}.pagination button[disabled]{color:#aab0a7;opacity:.6}.pagination button::after{border:0}.pagination text{color:#6c7769;font-size:21rpx}
</style>
