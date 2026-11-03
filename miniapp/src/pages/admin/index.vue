<script setup lang="ts">
import { onShow } from "@dcloudio/uni-app";
import { computed, ref } from "vue";
import { request } from "../../api/client";
import { type Property, type PropertyLead } from "../../api/properties";
import { hasRole } from "../../auth/session";
import { usePagePresentation } from "../../utils/presentation";
const tab = ref<"leads" | "properties">("properties"),
  leads = ref<PropertyLead[]>([]),
  properties = ref<Property[]>([]),
  loading = ref(false),
  workingID = ref("");
const statusLabel = (status: string) =>
  ({
    draft: "草稿",
    pending_review: "待审核",
    published: "已发布",
    offline: "已下架",
  })[status] || status;
const { presentation, loadPresentation, pageStyle } = usePagePresentation("adminWorkbench", "小皮工作台", "房源运营与数据处理");
const summary = computed(() => ({
  draft: properties.value.filter((item) => item.status === "draft").length,
  pending: properties.value.filter((item) => item.status === "pending_review")
    .length,
  published: properties.value.filter((item) => item.status === "published")
    .length,
}));
const load = async () => {
  loading.value = true;
  try {
    [leads.value, properties.value] = await Promise.all([
      request<PropertyLead[]>("/v1/property-leads"),
      request<Property[]>("/v1/properties"),
    ]);
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "加载失败",
      icon: "none",
    });
  } finally {
    loading.value = false;
  }
};
onShow(async () => {
  await loadPresentation();
  if (!hasRole("admin")) {
    uni.showToast({ title: "暂无运营权限", icon: "none" });
    uni.switchTab({ url: "/pages/me/index" });
    return;
  }
  load();
});
const edit = (id = "") =>
  uni.navigateTo({ url: `/pages/admin/property-edit${id ? `?id=${id}` : ""}` });
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
const changeStatus = async (property: Property, status: string) => {
  if (workingID.value) return;
  workingID.value = property.id;
  try {
    await request("/v1/properties/status", {
      method: "PATCH",
      data: { propertyId: property.id, status, note: "小程序工作台操作" },
    });
    uni.showToast({
      title:
        status === "published"
          ? "已发布"
          : status === "offline"
            ? "已下架"
            : "已提交",
    });
    await load();
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "状态更新失败",
      icon: "none",
    });
  } finally {
    workingID.value = "";
  }
};
const publish = (property: Property) => {
  uni.showModal({
    title: "确认发布房源",
    content: "确认后立即上架。",
    confirmText: "确认发布",
    success: (result) => result.confirm && changeStatus(property, "published"),
  });
};
const contactLead = (phoneNumber: string) => uni.makePhoneCall({ phoneNumber });
</script>
<template>
  <view class="page" :style="pageStyle"
    ><view class="page-presentation-head"><view class="page-presentation-title">{{ presentation.title }}</view><text>{{ presentation.subtitle }}</text></view
    ><view class="work-head"
      ><view
        ><view class="title">房源运营工作台</view
        ></view
      ><view class="refresh" @click="load">刷新</view></view
    ><view class="summary"
      ><view
        ><strong>{{ summary.draft }}</strong
        ><text>草稿</text></view
      ><view
        ><strong>{{ summary.pending }}</strong
        ><text>历史待审</text></view
      ><view
        ><strong>{{ summary.published }}</strong
        ><text>已发布</text></view
      ></view
    ><view class="tabs"
      ><text
        :class="{ active: tab === 'properties' }"
        @click="tab = 'properties'"
        >正式房源 {{ properties.length }}</text
      ><text :class="{ active: tab === 'leads' }" @click="tab = 'leads'"
        >房源线索 {{ leads.length }}</text
      ></view
    ><view v-if="tab === 'properties'" class="new-property" @click="edit()"
      ><text>＋</text
      ><view
        ><strong>录入新房源</strong
        ></view
      ><text>›</text></view
    ><view v-if="loading" class="muted empty">加载中...</view
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
          ><button class="primary" @click="convert(lead)">
            转为房源草稿
          </button></view
        ></view
      ><view v-if="!leads.length" class="empty muted"
        >暂无房源线索</view
      ></template
    ><template v-else
      ><view v-for="property in properties" :key="property.id" class="card item"
        ><view class="row"
          ><view
            ><view class="title small">{{
              property.title || property.community
            }}</view
            ><view class="muted info"
              >{{ property.community }} · {{ property.layout }}</view
            ></view
          ><text class="status" :class="property.status">{{
            statusLabel(property.status)
          }}</text></view
        ><view class="property-meta"
          ><text>{{ property.location || "位置待补充" }}</text
          ><strong>{{ property.rentMonthly }} 元/月</strong></view
        ><view class="actions"
          ><button :disabled="property.status === 'published'" @click="edit(property.id)">{{ property.status === 'published' ? '下架后编辑' : '编辑' }}</button
          ><button
            v-if="property.status === 'draft' || property.status === 'offline'"
            class="primary"
            :loading="workingID === property.id"
            @click="publish(property)"
          >
            确认发布</button
          ><button
            v-else-if="property.status === 'pending_review'"
            class="primary"
            :loading="workingID === property.id"
            @click="publish(property)"
          >
            确认发布</button
          ><button
            v-else-if="property.status === 'published'"
            class="danger"
            :loading="workingID === property.id"
            @click="changeStatus(property, 'offline')"
          >
            下架
          </button></view
        ></view
      ><view v-if="!properties.length" class="empty muted"
        >暂无正式房源</view
      ></template
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
  color: #0e8177;
  background: #e1f7f1;
  padding: 12rpx 22rpx;
  border-radius: 999rpx;
}
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
  color: #173f3c;
  font-size: 38rpx;
}
.summary text {
  margin-top: 5rpx;
  color: #78a39d;
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
  color: #0e8177;
  font-weight: 700;
}
.new-property {
  display: flex;
  align-items: center;
  gap: 18rpx;
  margin-bottom: 24rpx;
  padding: 24rpx 28rpx;
  border-radius: 24rpx;
  background: linear-gradient(135deg, #173f3c, #287168);
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
  background: #e8f8f3;
  color: #0e8177;
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
  color: #57857e;
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
  background: #eff8f5;
  color: #0e8177;
  border-radius: 999rpx;
  font-size: 24rpx;
}
.actions button::after {
  border: 0;
}
.actions .primary {
  background: #0e8177;
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
.page-presentation-head{margin-bottom:28rpx}.work-head{margin-bottom:20rpx}.work-head .title{font-size:36rpx;line-height:1.3}.summary{margin-bottom:28rpx}.tabs{margin-bottom:28rpx}.new-property{margin-bottom:28rpx}.item{margin-bottom:22rpx}.item .title{line-height:1.35}.info{line-height:1.55}.actions{margin-top:28rpx}
</style>
