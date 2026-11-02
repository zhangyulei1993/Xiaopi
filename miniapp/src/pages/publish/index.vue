<script setup lang="ts">
import { onLoad } from '@dcloudio/uni-app'
import { reactive, ref } from 'vue'
import { createPropertyLead, refreshAuth } from '../../api/properties'
import { getSession, saveSession } from '../../auth/session'

const form = reactive({
  sourceName: '',
  sourcePhone: '',
  community: '',
  location: '',
  layout: '',
  expectedRentMonthly: 0,
  note: '',
})
const submitting = ref(false)
const checkingRole = ref(true)

onLoad(async () => {
  const session = getSession()
  if (!session) {
    checkingRole.value = false
    return
  }
  try {
    const refreshed = await refreshAuth()
    saveSession(refreshed.accessToken, refreshed.role)
    if (refreshed.role === 'admin') {
      uni.showToast({ title: '管理员无需提交房源线索', icon: 'none' })
      setTimeout(() => uni.switchTab({ url: '/pages/me/index' }), 300)
      return
    }
  } catch {
    // 请求提交时仍会由服务端再次校验管理员身份。
  }
  checkingRole.value = false
})

const submit = async () => {
  if (submitting.value) return
  if (!form.sourceName.trim()) {
    uni.showToast({ title: '请填写联系人', icon: 'none' })
    return
  }
  if (!/^1\d{10}$/.test(form.sourcePhone)) {
    uni.showToast({ title: '请填写正确的手机号', icon: 'none' })
    return
  }
  if (!form.community.trim()) {
    uni.showToast({ title: '请填写小区名称', icon: 'none' })
    return
  }

  submitting.value = true
  try {
    await createPropertyLead({ ...form })
    uni.showToast({ title: '线索已提交' })
    setTimeout(() => uni.switchTab({ url: '/pages/me/index' }), 500)
  } catch (error) {
    uni.showToast({ title: error instanceof Error ? error.message : '提交失败', icon: 'none' })
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <view v-if="!checkingRole" class="page">
    <view class="tip card">
      <view class="title small">提供房源线索</view>
      <view class="muted">提交基础信息后，小皮会电话联系并实地核验。正式房源由运营人员审核发布。</view>
    </view>
    <view class="card">
      <view class="field"><text>联系人</text><input v-model.trim="form.sourceName" placeholder="怎么称呼你" /></view>
      <view class="field"><text>联系电话</text><input v-model="form.sourcePhone" type="number" maxlength="11" placeholder="方便小皮联系" /></view>
      <view class="field"><text>小区</text><input v-model.trim="form.community" placeholder="请输入小区名称" /></view>
      <view class="field"><text>位置</text><input v-model.trim="form.location" placeholder="区域 / 地铁 / 商圈" /></view>
      <view class="field"><text>户型</text><input v-model.trim="form.layout" placeholder="例如：一室一厅" /></view>
      <view class="field"><text>预期租金</text><input v-model.number="form.expectedRentMonthly" type="number" placeholder="请输入租金" /></view>
      <view class="field column"><text>补充说明</text><textarea v-model.trim="form.note" maxlength="500" placeholder="入住时间、房屋情况等"></textarea></view>
    </view>
    <button class="button" :loading="submitting" :disabled="submitting" @click="submit">
      {{ submitting ? '提交中...' : '提交房源线索' }}
    </button>
  </view>
  <view v-else class="page checking muted">正在确认用户身份...</view>
</template>

<style scoped>
.tip { color: #4c8880; background: #e5faf5; line-height: 1.7; }
.small { font-size: 34rpx; margin-bottom: 10rpx; }
.field { display: flex; align-items: center; padding: 24rpx 0; border-bottom: 1rpx solid #edf4f1; }
.field text { width: 150rpx; color: #57857e; }
.field input { flex: 1; }
.column { display: block; }
.column textarea { width: 100%; height: 170rpx; margin-top: 18rpx; }
.button { width: 100%; border: 0; font-size: 30rpx; line-height: 1.4; }
.button::after { border: 0; }
.checking { padding-top: 180rpx; text-align: center; }
</style>
