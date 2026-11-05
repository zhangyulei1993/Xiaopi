<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { reactive, ref } from "vue";
import { resolveAssetUrl } from "../../api/client";
import { bindWechatPhone, createRentalRequest, getPublicConfig, getSubscriptionTemplates, getWechatUserProfile, refreshAuth, registerMessageSubscription } from "../../api/properties";
import { getSession, saveSession } from "../../auth/session";
import { callPhone } from "../../utils/phone";
import { requestMessageSubscription } from "../../utils/subscribe";

const propertyID = ref(""),
  propertyTitle = ref(""),
  areaMode = ref(false),
  contactPhone = ref(""),
  pagePresentation = ref({ title: "小皮帮你找房", subtitle: "告诉我们你的找房需求", backgroundImage: "" }),
  submitting = ref(false), appointmentCreated = ref(false), bindingPhone = ref(false);
const areaTemplateID = ref("");
const form = reactive({
  phone: String(uni.getStorageSync("xiaopi_bound_phone") || ""),
  location: "",
  layout: "",
  budgetMin: 0,
  budgetMax: 2500,
  moveInDate: "",
  note: "",
});
onLoad(async (query) => {
  if (getSession()?.role === "admin") {
    uni.switchTab({ url: "/pages/me/index" });
    return;
  }
  propertyID.value = String(query?.propertyId || "");
  propertyTitle.value = decodeURIComponent(String(query?.title || ""));
	areaMode.value = String(query?.mode || "") === "area";
	if (areaMode.value) form.location = decodeURIComponent(String(query?.area || ""));
  const config = await getPublicConfig().catch(() => null);
	areaTemplateID.value = (await getSubscriptionTemplates().catch(() => null))?.areaListing || "";
  contactPhone.value = config?.contactPhone || "";
  pagePresentation.value = { ...pagePresentation.value, ...(config?.pagePresentations?.requests || {}) };
	if (getSession()?.role === "user") {
		const profile = await getWechatUserProfile().catch(() => null);
		if (profile?.phone) {
			form.phone = profile.phone;
			uni.setStorageSync("xiaopi_bound_phone", profile.phone);
		}
	}
});
const ensureLogin = () => {
  if (getSession()?.role === "user") return true;
  uni.showModal({
    title: "登录后提交",
    content: "登录后才能保存并跟进你的找房需求",
    confirmText: "去登录",
    success: (res) =>
      res.confirm && uni.navigateTo({ url: "/pages/auth/login" }),
  });
  return false;
};
const submit = async () => {
  if (!ensureLogin() || submitting.value) return;
  if (
    (!propertyID.value && !form.location.trim()) ||
    form.budgetMax < form.budgetMin
  ) {
    uni.showToast({ title: "请完善手机号、地区和预算", icon: "none" });
    return;
  }
  submitting.value = true;
  try {
	const messageAccepted = areaMode.value ? await requestMessageSubscription(areaTemplateID.value) : false;
    await createRentalRequest({
      requestType: propertyID.value ? "appointment" : areaMode.value ? "area_subscription" : "general",
      propertyId: propertyID.value,
      phone: form.phone,
      location: form.location,
      layout: form.layout,
      budgetMin: Number(form.budgetMin),
      budgetMax: Number(form.budgetMax),
      moveInDate: form.moveInDate,
      note: form.note,
    });
	if (messageAccepted) await registerMessageSubscription(areaTemplateID.value, form.location).catch(() => null);
	if (!form.phone) { appointmentCreated.value = true; return; }
    uni.showModal({
      title: propertyID.value ? "预约已提交" : areaMode.value ? "片区预约已提交" : "需求已提交",
      content: areaMode.value ? "管理员会补充该片区房源；微信新房通知需完成订阅授权。" : "小皮会根据你的条件匹配房源并联系你。",
      showCancel: false,
      success: () => uni.navigateBack(),
    });
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "提交失败",
      icon: "none",
    });
  } finally {
    submitting.value = false;
  }
};
const finishAppointment = () => uni.showModal({ title: propertyID.value ? "预约已提交" : areaMode.value ? "片区预约已提交" : "需求已提交", content: areaMode.value ? "该片区有新房源时会提醒你。" : "小皮会联系你确认需求。", showCancel: false, success: () => uni.navigateBack() });
const bindPhoneAfterAppointment = async (event: { detail: { code?: string } }) => {
	const code = event.detail.code;
	if (!code || bindingPhone.value) return;
	bindingPhone.value = true;
	try {
		const result = await bindWechatPhone(code);
		form.phone = result.phone;
		uni.setStorageSync("xiaopi_bound_phone", result.phone);
		const refreshed = await refreshAuth(); saveSession(refreshed.accessToken, refreshed.role);
		appointmentCreated.value = false; finishAppointment();
	} catch (error) { uni.showToast({ title: error instanceof Error ? error.message : "绑定失败", icon: "none" }); }
	finally { bindingPhone.value = false; }
};
const call = () => callPhone(contactPhone.value);
</script>
<template>
  <view class="page" :style="pagePresentation.backgroundImage ? { backgroundImage: `linear-gradient(rgba(247,246,241,.94), rgba(247,246,241,.98)), url(${resolveAssetUrl(pagePresentation.backgroundImage)})`, backgroundSize: 'cover' } : {}"
    ><view class="request-hero"
      ><text>{{ propertyID ? "预约房源" : areaMode ? "片区新房预约" : pagePresentation.title }}</text
      ><view>{{
        propertyID ? propertyTitle : areaMode ? form.location : "说清需求，比反复筛选更高效"
      }}</view
      ><small>{{
        propertyID
          ? "留下手机号和时间，小皮会回拨确认"
          : areaMode ? "该片区发布新房源后通知你"
          : pagePresentation.subtitle
      }}</small></view
    ><view class="card form"
      ><view v-if="!areaMode" class="field"
        ><text>联系电话 *</text
        ><input
          v-model.trim="form.phone"
          type="number"
          placeholder="用于接收匹配和预约通知" /></view
      ><view v-if="!propertyID" class="field"
        ><text>意向地区 *</text
        ><input
          v-model.trim="form.location"
          placeholder="例如：佛祖岭、金融港" /></view
      ><view v-if="!propertyID && !areaMode" class="field"
        ><text>期望户型</text
        ><input
          v-model.trim="form.layout"
          placeholder="例如：一室一厅 / 合租主卧" /></view
      ><view v-if="!propertyID" class="field"
        ><text>月租预算</text
        ><view class="budget"
          ><input
            v-model.number="form.budgetMin"
            type="number"
            placeholder="最低"
          /><text>至</text
          ><input
            v-model.number="form.budgetMax"
            type="number"
            placeholder="最高"
          /><text>元</text></view
        ></view
      ><view v-if="!areaMode" class="field"
        ><text>预计入住</text
        ><picker mode="date" @change="form.moveInDate = $event.detail.value"
          ><view class="picker"
            >{{ form.moveInDate || "选择日期" }} <text>›</text></view
          ></picker
        ></view
      ><view class="field"
        ><text>补充要求</text
        ><textarea
          v-model.trim="form.note"
          maxlength="300"
          :placeholder="propertyID ? '例如：工作日晚上、周末上午，或想先了解的问题' : '通勤地点、整租/合租、楼层、宠物、采光等'"
        ></textarea></view></view
    ><view class="privacy"
      >提交即表示同意小皮根据以上信息联系你，仅用于本次找房服务。</view
    ><view class="button" @click="submit">{{
      submitting ? "提交中..." : propertyID ? "登记看房时间" : areaMode ? "预约该片区新房" : "提交找房需求"
    }}</view
    ><view class="contact" @click="call">想马上确认房源？联系小皮 ›</view></view
    ><view v-if="appointmentCreated" class="phone-bind-mask"><view class="phone-bind-sheet"><view class="phone-bind-mark">✓</view><strong>{{ propertyID ? '预约已登记' : areaMode ? '片区预约已登记' : '找房需求已提交' }}</strong><text>{{ areaMode ? '绑定手机号后，管理员也可以在有合适新房时主动联系你。' : '绑定手机号后，小皮可以主动联系你确认需求。' }}</text><button open-type="getPhoneNumber" :loading="bindingPhone" @getphonenumber="bindPhoneAfterAppointment">绑定手机号</button><view @click="appointmentCreated = false; finishAppointment()">暂不绑定</view></view></view
  >
</template>
<style scoped>
.request-hero {
  padding: 38rpx 34rpx;
  margin-bottom: 24rpx;
  border-radius: 34rpx;
  background: radial-gradient(
      circle at 90% 10%,
      rgba(244, 217, 161, 0.4),
      transparent 180rpx
    ),
    linear-gradient(140deg, #173f3c, #287168);
  color: #fff;
}
.request-hero > text {
  color: #f4d9a1;
  font-size: 23rpx;
  font-weight: 700;
}
.request-hero > view {
  margin-top: 16rpx;
  font-size: 40rpx;
  font-weight: 800;
}
.request-hero small {
  display: block;
  margin-top: 12rpx;
  color: rgba(255, 255, 255, 0.68);
  font-size: 24rpx;
  line-height: 1.6;
}
.form {
  padding: 8rpx 28rpx;
}
.field {
  padding: 25rpx 0;
  border-bottom: 1rpx solid #edf4f1;
}
.field > text {
  display: block;
  margin-bottom: 14rpx;
  color: #315f5a;
  font-size: 25rpx;
  font-weight: 700;
}
.field input,
.field textarea,
.picker {
  width: 100%;
  color: #073b3a;
  font-size: 28rpx;
  box-sizing: border-box;
}
.field textarea {
  height: 150rpx;
}
.budget {
  display: grid;
  grid-template-columns: 1fr auto 1fr auto;
  align-items: center;
  gap: 12rpx;
}
.budget input {
  padding: 16rpx;
  border-radius: 16rpx;
  background: #f4f9f7;
  text-align: center;
}
.budget text,
.picker text {
  color: #8aa7a0;
}
.picker {
  display: flex;
  justify-content: space-between;
}
.privacy {
  padding: 0 12rpx;
  color: #8aa7a0;
  font-size: 21rpx;
  line-height: 1.6;
}
.contact {
  text-align: center;
  margin-top: 28rpx;
  color: #0e8177;
  font-size: 26rpx;
}
.phone-bind-mask{position:fixed;z-index:20;inset:0;display:flex;align-items:flex-end;background:rgba(27,34,27,.42)}.phone-bind-sheet{width:100%;padding:42rpx 36rpx calc(30rpx + env(safe-area-inset-bottom));border-radius:34rpx 34rpx 0 0;background:#fff;text-align:center}.phone-bind-mark{display:grid;place-items:center;width:66rpx;height:66rpx;margin:0 auto 18rpx;border-radius:22rpx;background:#e6edd7;color:#40543f;font-size:32rpx;font-weight:800}.phone-bind-sheet strong,.phone-bind-sheet text{display:block}.phone-bind-sheet strong{font-size:34rpx}.phone-bind-sheet text{margin:14rpx auto 24rpx;color:#7b8579;font-size:24rpx;line-height:1.6}.phone-bind-sheet button{border:0;border-radius:16rpx;background:#40543f;color:#fff;font-size:28rpx}.phone-bind-sheet button::after{border:0}.phone-bind-sheet>view:last-child{margin-top:20rpx;color:#929a91;font-size:23rpx}
.request-hero{margin-bottom:28rpx;padding:42rpx 34rpx}.request-hero>view{line-height:1.28}.form{padding:12rpx 28rpx}.field{padding:28rpx 0}.privacy{margin-top:8rpx;line-height:1.7}.contact{margin-top:32rpx;padding-bottom:12rpx}
.request-hero>text{font-size:24rpx;letter-spacing:.5rpx}.request-hero>view{font-size:42rpx;letter-spacing:-1rpx}.request-hero small{line-height:1.65}.field>text{margin-bottom:16rpx;font-size:26rpx;line-height:1.25}.field input,.field textarea,.picker{font-size:29rpx;line-height:1.45}.budget input{min-height:72rpx;padding:0 16rpx}.picker{min-height:50rpx;align-items:center}.privacy{font-size:22rpx}.button{margin-top:24rpx;min-height:96rpx;line-height:96rpx}
</style>
