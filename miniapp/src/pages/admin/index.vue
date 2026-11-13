<script setup lang="ts">
import { watchPostEffect } from "vue";
import { permissionAudit } from "../../utils/permission-audit";
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { request } from "../../api/client";
import { getSubscriptionTemplates, listContactEvents, registerMessageSubscription, type Property, type PropertyLead, type RentalRequest, type OperationLog, type FavoriteContact, type ContactEvent } from "../../api/properties";
import { hasRole } from "../../auth/session";
import { usePagePresentation } from "../../utils/presentation";
import { callPhone } from "../../utils/phone";
import { requestMessageSubscription } from "../../utils/subscribe";
const tab = ref<"leads" | "properties" | "requests" | "interests">("properties"),
  leads = ref<PropertyLead[]>([]),
  properties = ref<Property[]>([]),
  requests = ref<RentalRequest[]>([]),
  activities = ref<OperationLog[]>([]),
  interests = ref<FavoriteContact[]>([]),
	contactEvents = ref<ContactEvent[]>([]),
  loading = ref(false),
  loadFailed = ref(false);
const appointmentTemplateID = ref("");
const { presentation, loadPresentation, pageStyle } = usePagePresentation("adminWorkbench", "小皮工作台", "房源运营与数据处理");
const summary = computed(() => ({
  draft: properties.value.filter((item) => item.status === "draft").length,
  pending:
    leads.value.filter((item) => item.status !== "converted" && item.status !== "closed").length +
    requests.value.filter((item) => !item.status || item.status === "new").length,
  incomplete: properties.value.filter((item) => !item.community || !item.location || !item.latitude || !item.longitude || !item.media?.length).length,
}));
const load = async () => {
  loading.value = true;
  loadFailed.value = false;
  try {
    [leads.value, properties.value, requests.value, activities.value, interests.value, contactEvents.value] = await Promise.all([
      request<PropertyLead[]>("/v1/property-leads"),
      request<Property[]>("/v1/properties"),
      request<RentalRequest[]>("/v1/admin/rental-requests"),
      request<OperationLog[]>("/v1/admin/operation-logs").catch(() => []),
	  request<FavoriteContact[]>("/v1/admin/favorite-contacts").catch(() => []),
	  listContactEvents().catch(() => []),
    ]);
	appointmentTemplateID.value = (await getSubscriptionTemplates().catch(() => null))?.appointment || "";
  } catch (error) {
    loadFailed.value = true;
    uni.showToast({
      title: error instanceof Error ? error.message : "加载失败",
      icon: "none",
    });
  } finally {
    loading.value = false;
  }
};
onShow(async () => {
  permissionAudit("onShow", "admin-workbench");
  await loadPresentation();
  const allowed = hasRole("admin");
  permissionAudit("guard", "admin-workbench", allowed);
  if (!allowed) {
    uni.showToast({ title: "暂无运营权限", icon: "none" });
    permissionAudit("redirect", "admin-workbench", "pages/me/index");
    uni.switchTab({ url: "/pages/me/index" });
    return;
  }
  const requestedTab = String(uni.getStorageSync("xiaopi_admin_tab") || "");
  if (["properties", "leads", "requests", "interests"].includes(requestedTab)) {
    tab.value = requestedTab as typeof tab.value;
    uni.removeStorageSync("xiaopi_admin_tab");
  }
  load();
});
const edit = (id = "") =>
  uni.navigateTo({ url: `/pages/admin/property-edit${id ? `?id=${id}` : ""}` });
const openPropertyList = (filter = "") => {
  uni.setStorageSync("xiaopi_admin_property_status", filter || "all");
  uni.switchTab({ url: "/pages/properties/index" });
};
const openRequestProperty = (item: RentalRequest) => {
  if (item.requestType !== "appointment" || !item.propertyAvailable || !item.propertyId) return;
  uni.navigateTo({ url: `/pages/properties/detail?id=${encodeURIComponent(item.propertyId)}` });
};
const convert = async (lead: PropertyLead) => {
  try {
    const result = await request<Property>("/v1/property-leads/convert", {
      method: "POST",
      data: {
        leadId: lead.id,
        title: lead.community,
        community: lead.community,
        location: lead.location,
        layout: lead.layout,
        rentMonthly: lead.expectedRentMonthly,
        description: lead.note,
        ownerId: "xiaopi-admin",
      },
    });
    uni.showToast({ title: "已生成房源草稿" });
    edit(result.id);
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "转换失败",
      icon: "none",
    });
  }
};
const contactLead = (phoneNumber: string) => callPhone(phoneNumber, "客户手机号缺失");
const enableAppointmentAlerts = async () => {
	const accepted = await requestMessageSubscription(appointmentTemplateID.value);
	if (!accepted) { uni.showToast({ title: "未开启预约提醒", icon: "none" }); return; }
	await registerMessageSubscription(appointmentTemplateID.value);
	uni.showToast({ title: "已开启一次预约提醒" });
};
const followUp = async (type: "lead" | "request", id: string) => {
  try {
    await request(type === "lead" ? "/v1/property-leads/follow-up" : "/v1/admin/rental-requests/follow-up", { method:"PATCH", data:{ id, status:"contacted", followUpNote:"已在工作台联系客户" } });
    uni.showToast({ title:"已标记为已联系" }); await load();
  } catch (error) { uni.showToast({ title:error instanceof Error ? error.message : "更新失败", icon:"none" }); }
};
if (import.meta.env.MODE === "test") {
  watchPostEffect(() => permissionAudit("render-state", "admin-workbench", leads.value.length + properties.value.length + requests.value.length + activities.value.length + interests.value.length + contactEvents.value.length > 0));
}
</script>
<template>
  <view class="page" :style="pageStyle"
    ><view class="page-presentation-head"><view class="page-presentation-title">{{ presentation.title }}</view><text>{{ presentation.subtitle }}</text></view
    ><view class="work-head"
      ><view
        ><view class="title">房源总览</view
        ></view
      ><view class="head-actions"><view class="alert-switch" @click="enableAppointmentAlerts">开启预约提醒</view><view class="refresh" @click="load">刷新</view></view></view
    ><view class="summary"
      ><view
        ><strong>{{ summary.draft }}</strong
        ><text>草稿</text></view
      ><view
        ><strong>{{ summary.pending }}</strong
        ><text>待联系</text></view
      ><view
        ><strong>{{ summary.incomplete }}</strong
        ><text>待补资料</text></view
      ></view
    ><view class="tabs"
      ><text
        :class="{ active: tab === 'properties' }"
        @click="tab = 'properties'"
        >房源管理 {{ properties.length }}</text
      ><text :class="{ active: tab === 'leads' }" @click="tab = 'leads'"
        >房源线索 {{ leads.length }}</text
      ><text :class="{ active: tab === 'requests' }" @click="tab = 'requests'"
        >客户需求 {{ requests.length }}</text
      ><text :class="{ active: tab === 'interests' }" @click="tab = 'interests'"
        >用户意向 {{ interests.length + contactEvents.length }}</text
      ></view
    ><view v-if="tab === 'properties'" class="new-property" @click="edit()"
      ><text>＋</text
      ><view
        ><strong>录入新房源</strong
        ></view
      ><text>›</text></view
    ><view v-if="loading" class="muted empty">加载中...</view
    ><view v-else-if="loadFailed" class="empty load-error"><strong>工作台数据加载失败</strong><text>当前显示的待办和统计可能不完整。</text><button @click="load">重新加载</button></view
    ><template v-else-if="tab === 'leads'"
      ><view v-for="lead in leads" :key="lead.id" class="card item"
        ><view class="row"
          ><view class="title small">{{ lead.community }}</view
          ><text class="status">{{ lead.status }}</text></view
        ><view class="muted"
          >{{ lead.sourceName || "未填写姓名" }} · {{ lead.sourcePhone }}</view
        ><view class="muted"
          >{{ lead.location }} · {{ lead.layout }} ·
          {{ lead.expectedRentMonthly }} 元/月</view
        ><view class="note">{{ lead.note || "暂无补充说明" }}</view
        ><view class="actions"
          ><button @click="contactLead(lead.sourcePhone)">联系客户</button
          ><button @click="followUp('lead', lead.id)">标记已联系</button
          ><button class="primary" @click="convert(lead)">
            转为房源草稿
          </button></view
        ></view
      ><view v-if="!leads.length" class="empty muted"
        >暂无房源线索</view
      ></template
    ><template v-else-if="tab === 'requests'"
      ><view v-for="item in requests" :key="item.id" class="card item"
        ><view class="row"><view class="title small">{{ item.requestType === 'appointment' ? '预约看房' : item.requestType === 'area_subscription' ? '片区新房预约' : '找房需求' }}</view><text class="status">{{ item.status || 'new' }}</text></view
		><view v-if="item.requestType === 'appointment'" class="request-property" :class="{ unavailable: !item.propertyAvailable }" @click="openRequestProperty(item)"><text>预约房源</text><strong>{{ item.propertyTitle || item.location || '房源信息待确认' }}</strong><small>{{ item.propertyAvailable ? '查看房源 ›' : '房源已下架或无法查看' }}</small></view
		><view v-if="item.requestType === 'appointment'" class="muted info">{{ item.contactName || '称谓待补充' }} · {{ item.preferredTime || '时间待确认' }}</view
        ><view class="muted info">{{ item.location || '地区待确认' }} · {{ item.layout || '户型不限' }}</view
        ><view class="property-meta"><text>¥ {{ item.budgetMin || 0 }}–{{ item.budgetMax || '不限' }} / 月</text><text>{{ item.moveInDate || '入住时间待定' }}</text></view
        ><view v-if="item.note" class="note">{{ item.note }}</view
        ><view class="actions"><button @click="contactLead(item.phone)">联系客户</button><button @click="followUp('request', String(item.id))">标记已联系</button><button class="primary" @click="tab = 'properties'">匹配房源</button></view
        ></view
      ><view v-if="!requests.length" class="empty muted">暂无客户需求</view
      ></template
    ><template v-else-if="tab === 'interests'"
      ><view v-for="item in contactEvents" :key="item.id" class="card item"><view class="row"><view class="title small">{{ item.propertyName || '房源' }}</view><text class="status">{{ item.type === 'phone_entry_click' ? '拨号入口点击' : '咨询入口点击' }}</text></view><view class="muted">{{ item.nickname || '微信用户' }} · {{ item.phone || '手机号待绑定' }}</view><view class="property-meta"><text>{{ item.source === 'detail' ? '详情页触发' : item.source || '页面触发' }}</text><text>{{ item.createdAt ? new Date(item.createdAt).toLocaleString() : '' }}</text></view><view class="actions"><button :disabled="!item.phone" @click="contactLead(item.phone || '')">{{ item.phone ? '联系用户' : '等待绑定手机号' }}</button></view></view
      ><view v-for="item in interests" :key="`${item.userId}-${item.propertyId}`" class="card item"><view class="row"><view class="title small">{{ item.propertyName }}</view><text class="status">已关注</text></view><view class="muted">{{ item.nickname || '微信用户' }}</view><view class="property-meta"><text>{{ item.phone || '手机号待绑定' }}</text><text>{{ new Date(item.createdAt).toLocaleDateString() }}</text></view><view class="actions"><button :disabled="!item.phone" @click="contactLead(item.phone)">{{ item.phone ? '联系用户' : '等待绑定手机号' }}</button></view></view><view v-if="!interests.length && !contactEvents.length" class="empty muted">暂无用户意向记录</view></template
    ><template v-else
      ><view class="card property-management-entry"><strong>房源列表与管理操作已统一到「房源」页</strong><text>可在统一列表中搜索、筛选、编辑、复制、更新状态或删除房源。</text><button class="primary" @click="openPropertyList">打开房源管理</button></view></template
    ><view v-if="activities.length" class="activity card"><view class="title small">最近操作</view><view v-for="item in activities.slice(0, 5)" :key="item.id" class="muted">{{ item.action }} · {{ item.detail }}</view></view
    ></view
  >
</template>
<style scoped>
.work-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24rpx;
}
.refresh {
  color: #40543f;
  background: #e9eedc;
  padding: 12rpx 22rpx;
  border-radius: 999rpx;
}
.head-actions{display:flex;align-items:center;gap:10rpx}.alert-switch{padding:12rpx 18rpx;border-radius:999rpx;background:#40543f;color:#fff;font-size:22rpx}
.summary {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14rpx;
  margin-bottom: 24rpx;
}
.summary > view {
  padding: 22rpx;
  border-radius: 22rpx;
  background: #fff;
  text-align: center;
  box-shadow: 0 8rpx 24rpx rgba(15, 129, 119, 0.05);
}
.summary strong,
.summary text {
  display: block;
}
.summary strong {
  color: #29382d;
  font-size: 38rpx;
}
.summary text {
  margin-top: 5rpx;
  color: #7e887d;
  font-size: 21rpx;
}
.tabs {
  display: flex;
  background: #e3f6f1;
  border-radius: 999rpx;
  padding: 6rpx;
  margin-bottom: 24rpx;
}
.tabs text {
  flex: 1;
  text-align: center;
  padding: 18rpx;
  color: #5d938b;
}
.tabs .active {
  background: #fff;
  border-radius: 999rpx;
  color: #40543f;
  font-weight: 700;
}
.new-property {
  display: flex;
  align-items: center;
  gap: 18rpx;
  margin-bottom: 24rpx;
  padding: 24rpx 28rpx;
  border-radius: 24rpx;
  background: linear-gradient(135deg, #29382d, #536a45);
  color: #fff;
}
.new-property > text:first-child {
  display: flex;
  width: 52rpx;
  height: 52rpx;
  align-items: center;
  justify-content: center;
  border-radius: 16rpx;
  background: rgba(255, 255, 255, 0.12);
}
.new-property > view {
  flex: 1;
}
.new-property strong,
.new-property small {
  display: block;
}
.new-property small {
  margin-top: 6rpx;
  color: rgba(255, 255, 255, 0.65);
  font-size: 21rpx;
}
.small {
  font-size: 31rpx;
}
.item {
  padding: 28rpx;
}
.row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 18rpx;
}
.status {
  flex: none;
  background: #eef2e7;
  color: #40543f;
  padding: 8rpx 14rpx;
  border-radius: 999rpx;
  font-size: 21rpx;
}
.status.draft {
  background: #f0f3f5;
  color: #73818e;
}
.status.pending_review {
  background: #fff4dd;
  color: #a97822;
}
.status.offline {
  background: #fff0f1;
  color: #b94d5e;
}
.info {
  margin-top: 7rpx;
  font-size: 22rpx;
}
.property-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  margin-top: 22rpx;
  padding: 18rpx;
  border-radius: 16rpx;
  background: #f6faf9;
  color: #628681;
  font-size: 23rpx;
}
.property-meta strong {
  color: #e44d66;
  font-size: 27rpx;
}
.note {
  margin-top: 18rpx;
  padding: 18rpx;
  background: #f6fbf9;
  color: #64705f;
  border-radius: 16rpx;
}
.actions {
  display: flex;
  gap: 14rpx;
  margin-top: 24rpx;
}
.actions button {
  flex: 1;
  margin: 0;
  background: #f2f4ed;
  color: #40543f;
  border-radius: 999rpx;
  font-size: 24rpx;
}
.actions button::after {
  border: 0;
}
.actions .primary {
  background: #40543f;
  color: #fff;
}
.actions .danger {
  background: #fff0f1;
  color: #bd4056;
}
.empty {
  text-align: center;
  padding: 100rpx 0;
}
.load-error{display:flex;align-items:center;flex-direction:column;gap:12rpx;padding:60rpx 28rpx}.load-error strong{color:#40543f;font-size:29rpx}.load-error text{color:#828c82;font-size:22rpx}.load-error button{margin-top:8rpx;padding:0 28rpx;border:0;border-radius:12rpx;background:#40543f;color:#fff;font-size:22rpx;line-height:68rpx}.load-error button::after{border:0}.request-property{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:10rpx;margin-top:14rpx;padding:13rpx 15rpx;border:1rpx solid #dce6d9;border-radius:12rpx;background:#f3f7ef}.request-property>text{color:#7c8979;font-size:19rpx}.request-property strong{overflow:hidden;color:#40543f;font-size:22rpx;text-overflow:ellipsis;white-space:nowrap}.request-property small{color:#637b58;font-size:18rpx}.request-property.unavailable{grid-template-columns:auto minmax(0,1fr)}.request-property.unavailable small{grid-column:2;color:#969b93}.request-property:active{opacity:.78}
.page-presentation-head{margin-bottom:28rpx}.work-head{margin-bottom:20rpx}.work-head .title{font-size:36rpx;line-height:1.3}.summary{margin-bottom:28rpx}.tabs{margin-bottom:28rpx}.new-property{margin-bottom:28rpx}.item{margin-bottom:22rpx}.item .title{line-height:1.35}.info{line-height:1.55}.actions{margin-top:28rpx}
.refresh{font-size:23rpx;line-height:1.2}.summary>view{padding:25rpx 16rpx}.summary strong{font-size:40rpx;line-height:1.1}.summary text{font-size:22rpx;line-height:1.3}.tabs text{padding:19rpx 12rpx;font-size:25rpx;line-height:1.25}.new-property strong{font-size:30rpx;line-height:1.3}.item{padding:30rpx}.item .muted{margin-top:9rpx;font-size:23rpx;line-height:1.55}.note{font-size:23rpx;line-height:1.55}.actions button{min-height:76rpx;font-size:24rpx;line-height:76rpx}
.property-management-entry{display:flex;align-items:center;flex-direction:column;gap:14rpx;padding:42rpx 26rpx;text-align:center}.property-management-entry strong{color:#40543f;font-size:28rpx}.property-management-entry text{color:#7e887d;font-size:22rpx;line-height:1.5}.property-management-entry button{min-height:72rpx;margin-top:6rpx;padding:0 30rpx;border:0;border-radius:12rpx;background:#40543f;color:#fff;font-size:23rpx;line-height:72rpx}.property-management-entry button::after{border:0}
</style>
