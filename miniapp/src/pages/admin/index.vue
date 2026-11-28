<script setup lang="ts">
import { returnToMyPage } from "../../utils/permission-redirect";
import { watchPostEffect } from "vue";
import { permissionAudit } from "../../utils/permission-audit";
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { ApiError, request } from "../../api/client";
import { getSubscriptionTemplates, listContactEvents, registerMessageSubscription, type Property, type PropertyLead, type RentalRequest, type OperationLog, type FavoriteContact, type ContactEvent } from "../../api/properties";
import { hasRole } from "../../auth/session";
import { usePagePresentation } from "../../utils/presentation";
import { callPhone } from "../../utils/phone";
import { requestMessageSubscription } from "../../utils/subscribe";
const tab = ref<"requests" | "interests" | "leads">("requests"),
  leads = ref<PropertyLead[]>([]),
  properties = ref<Property[]>([]),
  requests = ref<RentalRequest[]>([]),
  activities = ref<OperationLog[]>([]),
  interests = ref<FavoriteContact[]>([]),
	contactEvents = ref<ContactEvent[]>([]),

  subscribingAppointmentReminders = ref(false);
const { presentation, loadPresentation, pageStyle } = usePagePresentation("adminWorkbench", "小皮工作台", "房源运营与数据处理");
const summary = computed(() => ({
  draft: properties.value.filter((item) => item.status === "draft").length,
  pending:
    leads.value.filter((item) => item.status !== "converted" && item.status !== "closed").length +
    requests.value.filter((item) => !item.status || item.status === "new").length,
  incomplete: properties.value.filter((item) => !item.community || !item.location || !item.latitude || !item.longitude || !item.media?.length).length,
}));
const pending = ref<string[]>([]);
const failedSections = ref<string[]>([]);
const tabSections = { requests: ["客户需求"], interests: ["关注记录", "联系记录"], leads: ["房源线索"] };
const loading = computed(() => tabSections[tab.value].some(section => pending.value.includes(section)));
const tabFailed = computed(() => tabSections[tab.value].some(section => failedSections.value.includes(section)));
let loadSequence = 0;
const load = async () => {
  const sequence = ++loadSequence;
  failedSections.value = [];
  pending.value = ["房源线索", "房源", "客户需求", "操作记录", "关注记录", "联系记录"];
  async function read<T>(section: string, requestData: () => Promise<T>, apply: (data: T) => void) {
    try {
      const data = await requestData();
      if (sequence === loadSequence) apply(data);
    } catch {
      if (sequence === loadSequence) failedSections.value.push(section);
    } finally {
      if (sequence === loadSequence) pending.value = pending.value.filter(name => name !== section);
    }
  }
  await Promise.all([
    read("房源线索", () => request<PropertyLead[]>("/v1/property-leads"), data => { leads.value = data; }),
    read("房源", () => request<Property[]>("/v1/properties"), data => { properties.value = data; }),
    read("客户需求", () => request<RentalRequest[]>("/v1/admin/rental-requests"), data => { requests.value = data; }),
    read("操作记录", () => request<OperationLog[]>("/v1/admin/operation-logs"), data => { activities.value = data; }),
    read("关注记录", () => request<FavoriteContact[]>("/v1/admin/favorite-contacts"), data => { interests.value = data; }),
    read("联系记录", listContactEvents, data => { contactEvents.value = data; }),
  ]);
};
onShow(async () => {
  permissionAudit("onShow", "admin-workbench");
  await loadPresentation();
  const allowed = hasRole("admin");
  permissionAudit("guard", "admin-workbench", allowed);
  if (!allowed) {
    uni.showToast({ title: "暂无运营权限", icon: "none" });
    permissionAudit("redirect", "admin-workbench", "pages/me/index");
    returnToMyPage();
    return;
  }
  const requestedTab = String(uni.getStorageSync("xiaopi_admin_tab") || "");
  if (["requests", "interests", "leads"].includes(requestedTab)) {
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
const openOperationLogs = () => uni.navigateTo({ url: "/pages/admin/operation-logs" });
const subscribeAppointmentReminders = async () => {
  if (!hasRole("admin") || subscribingAppointmentReminders.value) return;
  subscribingAppointmentReminders.value = true;
  try {
    const templates = await getSubscriptionTemplates();
    if (!templates.appointment) throw new Error("预约提醒模板未配置");
    const accepted = await requestMessageSubscription(templates.appointment);
    if (!accepted) return;
    await registerMessageSubscription(templates.appointment);
    uni.showToast({ title: "已获得一次预约提醒授权" });
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "预约提醒授权登记失败", icon: "none" });
  } finally {
    subscribingAppointmentReminders.value = false;
  }
};
const openRequestProperty = (item: RentalRequest) => {
  if (item.requestType !== "appointment" || !item.propertyAvailable || !item.propertyId) return;
  uni.navigateTo({ url: `/pages/properties/detail?id=${encodeURIComponent(item.propertyId)}` });
};
const statusLabels: Record<string, string> = {
  new: "待联系", contacted: "已联系", inspection_pending: "待看房",
  matched: "已匹配", completed: "已完成", converted: "已转房源",
  closed: "已关闭", invalid: "无效",
};
const statusLabel = (status?: string) => statusLabels[String(status || "new")] || status || "待联系";
const canFollowUp = (status?: string) => !["contacted", "converted", "closed", "completed", "invalid"].includes(String(status || "new"));
const canConvertLead = (status?: string) => !["converted", "closed", "invalid"].includes(String(status || "new"));
const convert = async (lead: PropertyLead) => {
  if (!canConvertLead(lead.status)) {
    uni.showToast({ title: lead.status === "converted" ? "这条线索已转为房源" : "这条线索当前不能转为房源", icon: "none" });
    return;
  }
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
const followUp = async (type: "lead" | "request", id: string, status?: string, note?: string) => {
  if (!canFollowUp(status)) return;
  try {
    await request(type === "lead" ? "/v1/property-leads/follow-up" : "/v1/admin/rental-requests/follow-up", { method:"PATCH", data:{ id, status:"contacted", followUpNote: note || "已在工作台联系客户" } });
    uni.showToast({ title:"已标记为已联系" }); await load();
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : "更新失败", icon: "none" });
    if (error instanceof ApiError && (error.statusCode === 404 || error.statusCode === 409)) await load();
  }
};
if (import.meta.env.MODE === "test") {
  watchPostEffect(() => permissionAudit("render-state", "admin-workbench", leads.value.length + properties.value.length + requests.value.length + activities.value.length + interests.value.length + contactEvents.value.length > 0));
}

function copyText(fallback: string) { return presentation.value?.copy?.[fallback] || fallback }
</script>
<template>
  <view class="page" :style="pageStyle"
    ><view class="page-presentation-head"><view class="page-presentation-title">{{ presentation.title }}</view><text>{{ presentation.subtitle }}</text></view
    ><view class="work-head"
      ><view
        ><view class="title">房源总览</view
        ></view
      ><view class="head-actions"><button class="subscribe-reminder" :disabled="subscribingAppointmentReminders" @click="subscribeAppointmentReminders">{{ subscribingAppointmentReminders ? "处理中…" : "订阅预约提醒" }}</button><view class="refresh" @click="!pending.length && load()">刷新</view></view></view
    ><view class="subscribe-reminder-note">每次授权对应一条预约通知；可再次授权后续提醒</view
    ><view v-if="failedSections.length" class="load-error"><text>{{ failedSections.join("、") }}加载失败，统计可能不完整。</text><button size="mini" @click="load" :disabled="!!pending.length">重试</button></view
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
    ><view class="activity-entry card" @click="openOperationLogs"><view class="activity-entry-head"><view><strong>操作记录</strong><text>分类查看并按时间回溯管理员操作</text></view><text class="activity-entry-link">查看全部 ›</text></view><view v-for="item in activities.slice(0, 3)" :key="item.id" class="activity-entry-row"><strong>{{ item.action }}</strong><text>{{ item.detail || item.targetType }}</text></view><view v-if="!activities.length && !pending.includes('操作记录') && !failedSections.includes('操作记录')" class="activity-entry-empty">暂无操作记录</view></view
    ><view class="tabs"
      ><text id="workbench-tab-requests" :class="{ active: tab === 'requests' }" @click="tab = 'requests'"
        >客户需求 {{ requests.length }}</text
      ><text id="workbench-tab-interests" :class="{ active: tab === 'interests' }" @click="tab = 'interests'"
        >用户意向 {{ interests.length + contactEvents.length }}</text
      ><text id="workbench-tab-leads" :class="{ active: tab === 'leads' }" @click="tab = 'leads'"
        >房源线索 {{ leads.length }}</text
      ></view
    ><view v-if="loading" class="muted empty">加载中...</view
    ><template v-else-if="tab === 'leads'"
      ><view v-for="lead in leads" :key="lead.id" class="card item"
        ><view class="row"
          ><view class="title small">{{ lead.community }}</view
          ><text class="status">{{ statusLabel(lead.status) }}</text></view
        ><view class="muted"
          >{{ lead.sourceName || "未填写姓名" }} · {{ lead.sourcePhone }}</view
        ><view class="muted"
          >{{ lead.location }} · {{ lead.layout }} ·
          {{ lead.expectedRentMonthly }} 元/月</view
        ><view class="note">{{ lead.note || copyText("暂无补充说明") }}</view
        ><view class="actions"
          ><button :disabled="!lead.sourcePhone" @click="contactLead(lead.sourcePhone)">{{ lead.sourcePhone ? "联系客户" : "手机号待补" }}</button
          ><button :disabled="!canFollowUp(lead.status)" @click="followUp('lead', lead.id, lead.status, lead.followUpNote)">{{ canFollowUp(lead.status) ? "标记已联系" : statusLabel(lead.status) }}</button
          ><button class="primary" :disabled="!canConvertLead(lead.status)" @click="convert(lead)">
            {{ lead.status === "converted" ? "已转房源" : lead.status === "closed" || lead.status === "invalid" ? statusLabel(lead.status) : "转为房源草稿" }}
          </button></view
        ></view
      ><view v-if="!tabFailed && !leads.length" class="empty muted"
        >{{ copyText("暂无房源线索") }}</view
      ></template
    ><template v-else-if="tab === 'requests'"
      ><view v-for="item in requests" :key="item.id" class="card item"
        ><view class="row"><view class="title small">{{ item.requestType === 'appointment' ? '预约看房' : item.requestType === 'area_subscription' ? '片区新房预约' : '找房需求' }}</view><text class="status">{{ statusLabel(item.status) }}</text></view
		><view v-if="item.requestType === 'appointment'" class="request-property" :class="{ unavailable: !item.propertyAvailable }" @click="openRequestProperty(item)"><text>预约房源</text><strong>{{ item.propertyTitle || item.location || '房源信息待确认' }}</strong><small>{{ item.propertyAvailable ? '查看房源 ›' : '房源已下架或无法查看' }}</small></view
		><view v-if="item.requestType === 'appointment'" class="muted info">{{ item.contactName || '称谓待补充' }} · {{ item.preferredTime || '时间待确认' }}</view
        ><view class="muted info">{{ item.location || '地区待确认' }} · {{ item.layout || '户型不限' }}</view
        ><view class="property-meta"><text>¥ {{ item.budgetMin || 0 }}–{{ item.budgetMax || '不限' }} / 月</text><text>{{ item.moveInDate || '入住时间待定' }}</text></view
        ><view v-if="item.note" class="note">{{ item.note }}</view
        ><view class="actions"><button :disabled="!item.phone" @click="contactLead(item.phone)">{{ item.phone ? "联系客户" : "手机号待补" }}</button><button :disabled="!canFollowUp(item.status)" @click="followUp('request', String(item.id), item.status, item.followUpNote)">{{ canFollowUp(item.status) ? "标记已联系" : statusLabel(item.status) }}</button><button class="primary" @click="openPropertyList()">匹配房源</button></view
        ></view
      ><view v-if="!tabFailed && !requests.length" class="empty muted">{{ copyText("暂无客户需求") }}</view
      ></template
    ><template v-else-if="tab === 'interests'"
      ><view v-for="item in contactEvents" :key="item.id" class="card item"><view class="row"><view class="title small">{{ item.propertyName || '房源' }}</view><text class="status">{{ item.type === 'phone_entry_click' ? '拨号入口点击' : '咨询入口点击' }}</text></view><view class="muted">{{ item.nickname || '微信用户' }} · {{ item.phone || '手机号待绑定' }}</view><view class="property-meta"><text>{{ item.source === 'detail' ? '详情页触发' : item.source || '页面触发' }}</text><text>{{ item.createdAt ? new Date(item.createdAt).toLocaleString() : '' }}</text></view><view class="actions"><button :disabled="!item.phone" @click="contactLead(item.phone || '')">{{ item.phone ? '联系用户' : '等待绑定手机号' }}</button></view></view
      ><view v-for="item in interests" :key="`${item.userId}-${item.propertyId}`" class="card item"><view class="row"><view class="title small">{{ item.propertyName }}</view><text class="status">已关注</text></view><view class="muted">{{ item.nickname || '微信用户' }}</view><view class="property-meta"><text>{{ item.phone || '手机号待绑定' }}</text><text>{{ new Date(item.createdAt).toLocaleDateString() }}</text></view><view class="actions"><button :disabled="!item.phone" @click="contactLead(item.phone)">{{ item.phone ? '联系用户' : '等待绑定手机号' }}</button></view></view><view v-if="!tabFailed && !interests.length && !contactEvents.length" class="empty muted">{{ copyText("暂无用户意向") }}记录</view></template
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
.head-actions{display:flex;align-items:center;gap:10rpx}
.subscribe-reminder{height:62rpx;margin:0;padding:0 20rpx;border:1rpx solid #d9e3d4;border-radius:999rpx;background:#fff;color:#40543f;font-size:21rpx;font-weight:700;line-height:60rpx}.subscribe-reminder::after{border:0}.subscribe-reminder[disabled]{opacity:.6}.subscribe-reminder-note{margin:-12rpx 0 22rpx;color:#818b7e;font-size:19rpx;line-height:1.4;text-align:right}
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
.activity-entry{margin:-4rpx 0 24rpx;padding:20rpx 24rpx}.activity-entry-head{display:flex;align-items:center;justify-content:space-between;gap:14rpx}.activity-entry-head>view:first-child{min-width:0}.activity-entry-head strong,.activity-entry-head text{display:block}.activity-entry-head strong{color:#344633;font-size:27rpx}.activity-entry-head>view:first-child text{margin-top:5rpx;color:#899287;font-size:20rpx}.activity-entry-link{flex:none;color:#506747;font-size:21rpx;font-weight:700}.activity-entry-row{display:flex;align-items:baseline;gap:12rpx;margin-top:13rpx;padding-top:12rpx;border-top:1rpx solid #eff0e9}.activity-entry-row strong{flex:none;color:#52614d;font-size:20rpx}.activity-entry-row text{overflow:hidden;color:#8a9288;font-size:19rpx;text-overflow:ellipsis;white-space:nowrap}.activity-entry-empty{margin-top:13rpx;padding-top:12rpx;border-top:1rpx solid #eff0e9;color:#92998f;font-size:20rpx}
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
.page-presentation-head{margin-bottom:28rpx}.work-head{margin-bottom:20rpx}.work-head .title{font-size:36rpx;line-height:1.3}.summary{margin-bottom:28rpx}.tabs{margin-bottom:28rpx}.item{margin-bottom:22rpx}.item .title{line-height:1.35}.info{line-height:1.55}.actions{margin-top:28rpx}
.refresh{font-size:23rpx;line-height:1.2}.summary>view{padding:25rpx 16rpx}.summary strong{font-size:40rpx;line-height:1.1}.summary text{font-size:22rpx;line-height:1.3}.tabs text{padding:19rpx 12rpx;font-size:25rpx;line-height:1.25}.item{padding:30rpx}.item .muted{margin-top:9rpx;font-size:23rpx;line-height:1.55}.note{font-size:23rpx;line-height:1.55}.actions button{min-height:76rpx;font-size:24rpx;line-height:76rpx}
.property-management-entry{display:flex;align-items:center;flex-direction:column;gap:14rpx;padding:42rpx 26rpx;text-align:center}.property-management-entry strong{color:#40543f;font-size:28rpx}.property-management-entry text{color:#7e887d;font-size:22rpx;line-height:1.5}.property-management-entry button{min-height:72rpx;margin-top:6rpx;padding:0 30rpx;border:0;border-radius:12rpx;background:#40543f;color:#fff;font-size:23rpx;line-height:72rpx}.property-management-entry button::after{border:0}
</style>
