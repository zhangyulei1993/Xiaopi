<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Picture } from '@element-plus/icons-vue'
import type { Property, PublicConfig, PropertyDetailFacility } from '../types/domain'
import homeTabIcon from '../../../miniapp/src/static/tabbar/home.png'
import propertyTabActiveIcon from '../../../miniapp/src/static/tabbar/house-active.png'
import meTabIcon from '../../../miniapp/src/static/tabbar/me.png'

import { normalizeCommunityIdentity } from '../utils/communityName'

type Material = Property & { materialCategory?: string; convertedPropertyId?: string; isMaterial?: boolean }
const props = defineProps<{ api: <T>(path: string, options?: RequestInit) => Promise<T>; apiBase: string }>()
const emit = defineEmits<{ 'continue-edit': [item: Property] }>()
async function continueEditing(item: Material) {
  if (!item.convertedPropertyId || busyID.value) return
  busyID.value = item.id
  try {
    const properties = await props.api<Property[]>('/v1/properties')
    const property = properties.find(entry => entry.id === item.convertedPropertyId)
    if (property) emit('continue-edit', property)
    else {
      busyID.value = ''
      await convert(item, true)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '读取关联房源失败')
  } finally {
    busyID.value = ''
  }
}
const items = ref<Material[]>([])
const categories = ref<string[]>([])
const properties = ref<Property[]>([])
function linkedActionLabel(item: Material) {
  const linked = properties.value.find(property => property.id === item.convertedPropertyId)
  if (!linked) return '打开关联房源'
  return ['draft', 'pending_review'].includes(linked.status) ? '继续编辑草稿' : '编辑关联房源'
}
const publicConfig = ref<PublicConfig | null>(null)
const selectedCategory = ref('全部')
const showConverted = ref(false)
const loading = ref(false)
const busyID = ref('')
const categoryDraft = ref('')
const categoryQuery = ref('')
const categoryManageOpen = ref(false)
const error = ref('')
const detailItem = ref<Material | null>(null)
const detailVisible = computed({ get: () => Boolean(detailItem.value), set: (value: boolean) => { if (!value) detailItem.value = null } })
const detailScrolled = ref(false)
const detailMapExpanded = ref(false)
const editOpen = ref(false)
const editDraft = ref<Material | null>(null)
const editAmenities = ref('')
const editNotes = ref('')
const savingEdit = ref(false)
const failedMedia = ref(new Set<string>())
const categoryMatches = (item: Material, name: string) =>
  normalizeCommunityIdentity(item.materialCategory || item.community) === normalizeCommunityIdentity(name)
const categoryCounts = computed(() => categories.value.map(name => ({ name, count: items.value.filter(item => categoryMatches(item, name)).length })))
const categoryManagementItems = computed(() => categoryCounts.value
  .map(category => {
    const sample = items.value.find(item => categoryMatches(item, category.name) && item.location && item.location !== category.name)
    return { ...category, address: sample?.location || '小区地址待补充' }
  })
  .filter(category => !categoryQuery.value.trim() || normalizeCommunityIdentity(category.name).includes(normalizeCommunityIdentity(categoryQuery.value)) || category.address.includes(categoryQuery.value.trim())))
const filteredItems = computed(() => items.value.filter(item => {
  return (selectedCategory.value === '全部' || categoryMatches(item, selectedCategory.value)) && (!showConverted.value || Boolean(item.convertedPropertyId))
}))
const mediaURL = (url: string) => {
  if (/^https?:/i.test(url)) return url
  const origin = props.apiBase.replace(/\/api\/?$/, '')
  return `${origin}${url.startsWith('/') ? '' : '/'}${url}`
}
const coverMedia = (item: Material) => item.media?.find(media => media.type === 'image' && !failedMedia.value.has(media.url))
const detailMedia = computed(() => detailItem.value?.media || [])
const detailMediaIndex = ref(0)
const detailCurrentMedia = computed(() => detailMedia.value[detailMediaIndex.value])
const detailIsVideo = computed(() => detailCurrentMedia.value?.type === 'video')
function openDetail(item: Material) { detailItem.value = item; detailMediaIndex.value = 0; detailScrolled.value = false; detailMapExpanded.value = false }
function stepDetailMedia(direction: number) {
  if (!detailMedia.value.length) return
  detailMediaIndex.value = (detailMediaIndex.value + direction + detailMedia.value.length) % detailMedia.value.length
}
function selectDetailMedia(type: 'image' | 'video') {
  const index = detailMedia.value.findIndex(media => media.type === type)
  if (index >= 0) detailMediaIndex.value = index
}
const detailModule = (key: string) => publicConfig.value?.propertyDetail.modules.find(module => module.key === key)
const detailModuleEnabled = (key: string) => key === 'overview' || (detailModule(key)?.enabled ?? true)
const detailModuleTitle = (key: string, fallback: string) => detailModule(key)?.title || fallback
const detailHasLocation = computed(() => Boolean(Number(detailItem.value?.latitude) && Number(detailItem.value?.longitude)))
const detailDisplayAddress = computed(() => {
  const item = detailItem.value
  const community = String(item?.community || '').trim()
  const raw = String(item?.location || '').trim()
  const address = /^(位置待补充|地址待补充)$/.test(raw) ? '' : raw
  if (!address) return community || raw || '位置待补充'
  const normalize = (value: string) => value.replace(/[\s·・,，。\-—]/g, '').toLowerCase()
  return !community || normalize(address).includes(normalize(community)) ? address : `${address} · ${community}`
})
const detailDoorplate = computed(() => {
  const item = detailItem.value
  if (!item || !publicConfig.value?.listingTitle.adminEnabledFields.includes('unit')) return ''
  const base = [item.buildingNo, item.unitNo, item.roomNo].filter(Boolean).join('') || '待补充'
  return item.rentType === '合租' && item.roomSequence ? `${base} · ${item.roomSequence}号房` : base
})
const detailStatusPreview = computed(() => {
  const status = detailItem.value?.status
  if (status === 'available') return ''
  if (status === 'offline') return '已下架预览'
  if (status === 'pending_review' || status === 'draft') return '待发布预览'
  if (status === 'rented') return '已租预览'
  return '待设可租预览'
})
const detailFacts = computed(() => {
  const item = detailItem.value
  if (!item) return [] as [string, string][]
  return [
    [item.rentType === '合租' && item.wholeAreaSqm ? '整套面积' : '面积', item.rentType === '合租' && item.wholeAreaSqm ? `${item.wholeAreaSqm}㎡` : item.areaSqm ? `${item.areaSqm}㎡` : '待补充'],
    [item.rentType === '合租' && item.wholeLayout ? '整套户型' : '户型', item.rentType === '合租' && item.wholeLayout ? item.wholeLayout : item.layout || item.actualLayout || '待补充'],
    ['楼层', item.floorInfo || '待补充'],
    ...(item.orientation ? [['朝向', item.orientation] as [string, string]] : []),
    ...(item.elevator ? [['电梯', item.elevator] as [string, string]] : []),
    ['租赁方式', item.rentType || '待补充'],
  ] as [string, string][]
})
const detailRentalInfo = computed(() => {
  const item = detailItem.value
  return Boolean(item && (item.depositPayment || item.feeText || item.utilitiesText || item.listingSource === '转租' || (item.rentType === '合租' && (item.layout || item.areaSqm))))
})
const detailDescription = computed(() => String(detailItem.value?.description || '').replace(/配套设施[ \t]*[：:][ \t]*[^\n]*\n?/, '').trim())
const detailAmenities = computed(() => {
  const item = detailItem.value
  if (!item) return [] as PropertyDetailFacility[]
  const listed = new Set([...(item.displayTags || []), ...(String(item.description || '').match(/配套设施[ \t]*[：:][ \t]*([^\n]*)/)?.[1] || '').split(/[、,，]/)].map(value => value.trim()).filter(Boolean))
  const configured = publicConfig.value?.propertyDetail.facilities.filter(facility => facility.enabled) || []
  return configured.length
    ? configured.filter(facility => listed.has(facility.label))
    : [...listed].map((label, index) => ({ key: `material-${index}`, label, group: '家具家电', enabled: true }))
})
const detailFacilityGroups = computed(() => {
  const groups = new Map<string, PropertyDetailFacility[]>()
  detailAmenities.value.forEach(facility => groups.set(facility.group, [...(groups.get(facility.group) || []), facility]))
  return Array.from(groups, ([title, facilities]) => ({ title, facilities }))
})
function distanceKm(from: Pick<Property, 'latitude' | 'longitude'>, to: Pick<Property, 'latitude' | 'longitude'>) {
  const radians = (value: number) => value * Math.PI / 180
  const lat1 = radians(Number(from.latitude)), lat2 = radians(Number(to.latitude))
  const dLat = lat2 - lat1, dLng = radians(Number(to.longitude) - Number(from.longitude))
  const a = Math.sin(dLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) ** 2
  return 6371 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))
}
const detailNearby = computed(() => {
  const current = detailItem.value
  if (!current || !detailHasLocation.value) return [] as { item: Property; distance: number }[]
  const settings = publicConfig.value?.propertyDetail.nearby
  const radius = Number(settings?.radiusKm) || 3
  const channels = publicConfig.value?.featuredChannels || []
  const inSameChannel = (candidate: Property) => channels.some(channel => Number(channel.latitude) && Number(channel.longitude) &&
    distanceKm(current, { latitude: Number(channel.latitude), longitude: Number(channel.longitude) }) <= Number(channel.radiusKm || 3) &&
    distanceKm(candidate, { latitude: Number(channel.latitude), longitude: Number(channel.longitude) }) <= Number(channel.radiusKm || 3))
  return properties.value.filter(item => item.id !== current.id && ['published', 'available'].includes(item.status) && Number(item.latitude) && Number(item.longitude))
    .map(item => ({ item, distance: distanceKm(current, item) })).filter(entry => entry.distance <= radius)
    .filter(entry => settings?.relation !== 'channel' || inSameChannel(entry.item))
    .sort((a, b) => settings?.sort === 'newest' ? String(b.item.id).localeCompare(String(a.item.id)) : a.distance - b.distance)
    .slice(0, Number(settings?.limit) || 4)
})
const facilityIcons: Record<string, string> = {
  refrigerator: new URL('../../../miniapp/src/static/amenities/fridge.svg', import.meta.url).href,
  washer: new URL('../../../miniapp/src/static/amenities/washer.svg', import.meta.url).href,
  air_conditioner: new URL('../../../miniapp/src/static/amenities/air-conditioner.svg', import.meta.url).href,
  water_heater: new URL('../../../miniapp/src/static/amenities/water-heater.svg', import.meta.url).href,
  television: new URL('../../../miniapp/src/static/amenities/tv.svg', import.meta.url).href,
  broadband: new URL('../../../miniapp/src/static/amenities/wifi.svg', import.meta.url).href,
  gas: new URL('../../../miniapp/src/static/amenities/gas.svg', import.meta.url).href,
  hood: new URL('../../../miniapp/src/static/amenities/hood.svg', import.meta.url).href,
  sofa: new URL('../../../miniapp/src/static/amenities/sofa.svg', import.meta.url).href,
  wardrobe: new URL('../../../miniapp/src/static/amenities/wardrobe.svg', import.meta.url).href,
  desk: new URL('../../../miniapp/src/static/amenities/desk.svg', import.meta.url).href,
  bed: new URL('../../../miniapp/src/static/amenities/bed.svg', import.meta.url).href,
  microwave: new URL('../../../miniapp/src/static/amenities/microwave.svg', import.meta.url).href,
  dishwasher: new URL('../../../miniapp/src/static/amenities/dishwasher.svg', import.meta.url).href,
  dryer: new URL('../../../miniapp/src/static/amenities/dryer.svg', import.meta.url).href,
  curtains: new URL('../../../miniapp/src/static/amenities/curtains.svg', import.meta.url).href,
  dining_table: new URL('../../../miniapp/src/static/amenities/dining_table.svg', import.meta.url).href,
  range: new URL('../../../miniapp/src/static/amenities/range.svg', import.meta.url).href,
}
const detailFacilityIcon = (key: string) => facilityIcons[key] || ''
function markMediaFailed(url: string) { failedMedia.value = new Set(failedMedia.value).add(url) }
async function load() {
  loading.value = true; error.value = ''
  try {
    const [materialData, categoryData, propertyData, configData] = await Promise.all([
      props.api<Material[]>('/v1/admin/material-library'),
      props.api<string[]>('/v1/admin/material-library/categories'),
      props.api<Property[]>('/v1/properties').catch(() => []),
      props.api<PublicConfig>('/v1/config/public').catch(() => null),
    ])
    items.value = Array.isArray(materialData) ? materialData : []; categories.value = Array.isArray(categoryData) ? categoryData : []
    properties.value = Array.isArray(propertyData) ? propertyData : []; publicConfig.value = configData

  } catch (e) { error.value = e instanceof Error ? e.message : '素材库加载失败' }
  finally { loading.value = false }
}
async function addCategory() {
  const name = categoryDraft.value.trim()
  if (!name) return
  if (categories.value.includes(name)) { ElMessage.warning('该小区分类已存在'); return }
  try {
    categories.value = await props.api<string[]>('/v1/admin/material-library/categories', { method: 'PUT', body: JSON.stringify({ add: name }) })
    selectedCategory.value = name; categoryDraft.value = ''; ElMessage.success('分类已添加')
  } catch (e) { ElMessage.error(e instanceof Error ? e.message : '分类保存失败') }
}
async function renameCategory(name: string) {
  try {
    const { value } = await ElMessageBox.prompt('重命名会同步更新该分类下素材的小区归属。', '修改小区分类', { inputValue: name, inputPattern: /^.{1,60}$/, inputErrorMessage: '请输入 1 到 60 个字符', confirmButtonText: '保存名称', cancelButtonText: '取消' })
    const nextName = String(value || '').trim()
    if (!nextName || nextName === name) return
    await props.api('/v1/admin/material-library/categories/rename', { method: 'PATCH', body: JSON.stringify({ from: name, to: nextName }) })
    if (selectedCategory.value === name) selectedCategory.value = nextName
    await load()
    if (detailItem.value) detailItem.value = items.value.find(item => item.id === detailItem.value?.id) || null
    ElMessage.success('分类名称已更新')
  } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error(e instanceof Error ? e.message : '分类重命名失败') }
}
async function removeCategory(name: string) {
  if ((categoryCounts.value.find(category => category.name === name)?.count || 0) > 0) {
    ElMessage.warning('请先移动或删除分类下的素材，再移除分类')
    return
  }
  try {
    await ElMessageBox.confirm(`移除「${name}」分类？分类下仍有素材时无法移除。`, '移除分类', { type: 'warning', confirmButtonText: '移除', cancelButtonText: '取消' })
    categories.value = await props.api<string[]>('/v1/admin/material-library/categories', { method: 'PUT', body: JSON.stringify({ remove: name }) })
    if (selectedCategory.value === name) selectedCategory.value = '全部'
    ElMessage.success('分类已移除')
  } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error(e instanceof Error ? e.message : '移除失败') }
}
async function convert(item: Material, missing = false) {
  try {
    await ElMessageBox.confirm(missing ? '关联房源已删除，是否重新转换？原素材会保留。' : '将素材复制为房源草稿，原素材会保留。', '转为房源草稿', { confirmButtonText: '创建草稿', cancelButtonText: '取消' })
    busyID.value = item.id
    const property = await props.api<Property>(`/v1/admin/material-library/${encodeURIComponent(item.id)}/convert`, { method: 'PATCH' })
    ElMessage.success('房源草稿已就绪，原素材保留')
    emit('continue-edit', property)
  } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error(e instanceof Error ? e.message : '转换失败') }
  finally { busyID.value = '' }
}
function openEdit(item: Material) {
  editDraft.value = { ...item }
  const description = String(item.description || '')
  editAmenities.value = description.match(/配套设施[ \t]*[：:][ \t]*([^\n]*)/)?.[1] || ''
  editNotes.value = description.replace(/配套设施[ \t]*[：:][ \t]*[^\n]*\n?/, '').trim()
  editOpen.value = true
}
async function saveEdit() {
  const draft = editDraft.value
  if (!draft || savingEdit.value) return
  savingEdit.value = true
  try {
    await props.api('/v1/properties/update', { method: 'PATCH', body: JSON.stringify({ ...draft, description: [editAmenities.value.trim() ? `配套设施：${editAmenities.value.trim()}` : '', editNotes.value.trim()].filter(Boolean).join('\n'), isMaterial: true, materialCategory: draft.materialCategory || draft.community }) })
    ElMessage.success('素材信息已保存')
    editOpen.value = false
    await load()
    detailItem.value = items.value.find(item => item.id === draft.id) || null
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '素材保存失败')
    if ((e as { status?: number })?.status === 409) {
      try {
        await ElMessageBox.confirm('重新加载会放弃当前未保存的修改。', '素材版本已变化', { confirmButtonText: '重新加载', cancelButtonText: '返回表单' })
        editOpen.value = false
        await load()
        detailItem.value = items.value.find(item => item.id === draft.id) || null
      } catch { /* Keep the form open when the user cancels. */ }
    }
  }
  finally { savingEdit.value = false }
}
async function moveCategory(item: Material, category: string) {
  if (!category || categoryMatches(item, category) || busyID.value) return
  busyID.value = item.id
  try {
    const locations = await props.api<Record<string, { address: string; latitude: number; longitude: number }>>('/v1/admin/community-locations')
    const target = locations[category]
    await props.api('/v1/properties/update', { method: 'PATCH', body: JSON.stringify({ ...item, title: item.title === item.community ? category : item.title, community: category, location: target?.address || category, latitude: Number(target?.latitude) || 0, longitude: Number(target?.longitude) || 0, areaLabel: '', buildingNo: '', unitNo: '', roomNo: '', materialCategory: category, isMaterial: true }) })
    ElMessage.success('已移动到新分类')
    await load()
    detailItem.value = items.value.find(value => value.id === item.id) || null
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '移动分类失败')
    if ((e as { status?: number })?.status === 409) {
      await load()
      detailItem.value = items.value.find(value => value.id === item.id) || null
    }
  } finally { busyID.value = '' }
}
async function remove(item: Material) {
  try {
    await ElMessageBox.confirm('删除后素材及其媒体将不可恢复。', '删除素材', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    busyID.value = item.id
    await props.api(`/v1/admin/material-library/${encodeURIComponent(item.id)}`, { method: 'DELETE' })
    ElMessage.success('素材已删除'); detailItem.value = null; await load()
  } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error(e instanceof Error ? e.message : '删除失败') }
  finally { busyID.value = '' }
}
onMounted(load)
</script>

<template>
  <section class="page-header"><div><span>内容与运营</span><h1>房源素材库</h1><p>按小区整理待发布素材，可转为房源草稿继续完善。</p></div><el-button :loading="loading" @click="load">刷新数据</el-button></section>
  <div class="material-stats"><article class="surface"><strong>{{ items.length }}</strong><span>素材总数</span></article><article class="surface"><strong>{{ categories.length }}</strong><span>小区分类</span></article><article class="surface"><strong>{{ items.reduce((n, item) => n + (item.media?.length || 0), 0) }}</strong><span>图片与视频</span></article></div>
  <section class="surface material-panel">
    <div class="material-toolbar"><div class="material-tabs"><el-button :type="!showConverted ? 'primary' : 'default'" @click="showConverted = false">全部素材 <el-tag size="small" effect="plain">{{ items.length }}</el-tag></el-button><el-button :type="showConverted ? 'primary' : 'default'" @click="showConverted = true">已转房源 <el-tag size="small" effect="plain">{{ items.filter(item => item.convertedPropertyId).length }}</el-tag></el-button></div><el-button @click="categoryManageOpen = true">分类管理 <el-tag size="small" effect="plain">{{ categories.length }}</el-tag></el-button></div>
    <nav class="material-categories"><el-button :type="selectedCategory === '全部' ? 'primary' : 'default'" @click="selectedCategory = '全部'">全部 {{ items.length }}</el-button><el-button v-for="category in categoryCounts" :key="category.name" :type="selectedCategory === category.name ? 'primary' : 'default'" @click="selectedCategory = category.name">{{ category.name }} <span class="category-count">{{ category.count }}</span></el-button></nav>
    <el-alert v-if="error" type="error" :title="error" show-icon :closable="false" />
    <div v-loading="loading" class="material-grid">
      <article v-for="item in filteredItems" :key="item.id" class="material-card" :class="{ converted: item.convertedPropertyId }" role="button" tabindex="0" :aria-label="`查看素材：${item.title || item.community}`" @click="openDetail(item)" @keydown.enter="openDetail(item)" @keydown.space.prevent="openDetail(item)">
        <div class="material-cover"><img v-if="coverMedia(item)" :src="mediaURL(coverMedia(item)!.url)" :alt="`${item.community}素材封面`" @error="markMediaFailed(coverMedia(item)!.url)"/><div v-else class="media-empty"><el-icon><Picture /></el-icon><span>封面暂不可用</span></div><el-tag class="material-state" size="small" :type="item.convertedPropertyId ? 'success' : 'info'">{{ item.convertedPropertyId ? '已转房源' : '待转房源' }}</el-tag><span class="media-count">{{ item.media?.filter(media => media.type === 'image').length || 0 }} 图 · {{ item.media?.filter(media => media.type === 'video').length || 0 }} 视频</span></div>
        <div class="material-copy"><div class="material-title"><strong :title="item.title || item.community">{{ item.title || item.community }}</strong></div><div class="material-community" :title="item.materialCategory || item.community">{{ item.materialCategory || item.community }}</div><p :title="item.location || item.community">{{ item.location || item.community }}</p><div class="material-meta"><span>{{ item.layout || item.wholeLayout || '户型待补充' }}</span><span v-if="item.areaSqm">{{ item.areaSqm }}㎡</span><span v-if="item.floorInfo">{{ item.floorInfo }}</span><span v-if="item.orientation">{{ item.orientation }}</span><span v-if="item.elevator">{{ item.elevator === '有' ? '有电梯' : item.elevator }}</span><span v-for="tag in (item.displayTags || []).slice(0,2)" :key="tag">{{ tag }}</span></div><div class="material-card-footer"><b v-if="item.rentMonthly > 0" class="material-rent">{{ item.rentMonthly }} 元/月</b><small v-else>打开查看详情</small><span class="open-detail">查看详情 ›</span></div></div>
      </article>
      <el-empty v-if="!loading && !error && !filteredItems.length" :description="showConverted ? '当前筛选下暂无已转房源的素材' : '当前筛选下暂无素材'" />
    </div>
  </section>
  <el-dialog v-model="detailVisible" class="admin-material-detail-dialog" width="min(420px, 94vw)" :show-close="false" destroy-on-close align-center>
    <div v-if="detailItem" class="admin-material-phone-frame">
      <div class="phone-preview page-phone-preview preview-propertyDetail canvas-identity-admin canvas-operator admin-material-phone">
        <div class="phone-status"><span>22:18</span><span>4G ▰</span></div>
        <div class="page-mini-content">
          <div class="detail-canvas operator">
            <div v-if="detailScrolled" class="canvas-detail-sticky"><i class="canvas-detail-sticky-capsule" aria-hidden="true">•••　⊙</i><button type="button" aria-label="返回素材列表" @click="detailVisible = false">‹</button><span>房源详情</span></div>
            <div class="detail-canvas-scroll admin-material-detail-scroll" @scroll.passive="detailScrolled = ($event.currentTarget as HTMLElement).scrollTop > 65">
              <div class="detail-canvas-cover" :class="{ 'is-placeholder': !detailCurrentMedia || failedMedia.has(detailCurrentMedia.url) }" :style="detailCurrentMedia && !failedMedia.has(detailCurrentMedia.url) && !detailIsVideo ? { backgroundImage: `url(${mediaURL(detailCurrentMedia.url)})` } : {}">
                <video v-if="detailIsVideo && detailCurrentMedia && !failedMedia.has(detailCurrentMedia.url)" :key="detailCurrentMedia.url" :src="mediaURL(detailCurrentMedia.url)" :poster="detailItem.media?.find(media => media.type === 'image')?.url ? mediaURL(detailItem.media.find(media => media.type === 'image')!.url) : undefined" controls playsinline preload="metadata" @error="markMediaFailed(detailCurrentMedia.url)"/>
                <div class="detail-canvas-media-nav"><button type="button" aria-label="关闭素材详情" @click="detailVisible = false">‹</button><i>•••　⊙</i></div>
                <div v-if="!detailCurrentMedia || failedMedia.has(detailCurrentMedia.url)" class="detail-canvas-empty-media"><el-icon><Picture /></el-icon><b>{{ detailCurrentMedia ? '媒体暂不可用' : '暂无图片或视频' }}</b><small>{{ detailCurrentMedia ? '请检查媒体地址' : '这个素材还没有媒体文件' }}</small></div>
                <div v-if="detailMedia.length" class="detail-canvas-media-tabs"><button v-if="detailMedia.some(media => media.type === 'video')" type="button" :class="{ active: detailIsVideo }" @click="selectDetailMedia('video')">视频</button><button v-if="detailMedia.some(media => media.type === 'image')" type="button" :class="{ active: !detailIsVideo }" @click="selectDetailMedia('image')">照片 {{ detailIsVideo ? 1 : detailMedia.slice(0, detailMediaIndex + 1).filter(media => media.type === 'image').length }}/{{ detailMedia.filter(media => media.type === 'image').length }}</button><span class="detail-canvas-media-pager"><button type="button" aria-label="上一张" @click="stepDetailMedia(-1)">‹</button>{{ detailMediaIndex + 1 }}/{{ detailMedia.length }}<button type="button" aria-label="下一张" @click="stepDetailMedia(1)">›</button></span></div>
              </div>
              <section v-if="detailModuleEnabled('overview')" class="detail-canvas-card overview-card admin-material-overview">
                <em v-if="detailItem.convertedPropertyId || detailStatusPreview" class="detail-canvas-status-preview">{{ detailItem.convertedPropertyId ? '已转为房源 · 素材仍保留' : detailStatusPreview }}</em>
                <div class="detail-canvas-price">{{ detailItem.rentMonthly || 0 }}<small>元/月</small></div>
                <strong class="detail-canvas-title">{{ detailItem.title || detailItem.community }}</strong>
                <span class="detail-canvas-address">{{ detailDisplayAddress }}</span>
                <small v-if="detailDoorplate" class="preview-operator-doorplate">门牌定位 · {{ detailDoorplate }}</small>
                <div v-if="detailItem.listingSource === '转租'" class="detail-canvas-meta"><em>转租 · 剩余 {{ detailItem.remainingLeaseMonths || 0 }} 个月</em></div>
                <div class="detail-canvas-facts"><i v-for="([label, value]) in detailFacts" :key="label"><small>{{ label }}</small>{{ value }}</i></div>
              </section>
              <section v-if="detailRentalInfo" class="detail-canvas-card detail-canvas-info"><b>租赁信息</b><small class="detail-canvas-section-copy">费用与租期说明</small><div><i v-if="detailItem.depositPayment"><small>押付方式</small>{{ detailItem.depositPayment }}</i><i v-if="detailItem.feeText"><small>中介费</small>{{ detailItem.feeText }}</i><i v-if="detailItem.utilitiesText" class="wide"><small>水电杂费</small>{{ detailItem.utilitiesText }}</i><i v-if="detailItem.rentType === '合租' && detailItem.layout"><small>使用空间</small>{{ detailItem.layout }}</i><i v-if="detailItem.rentType === '合租' && detailItem.areaSqm"><small>使用面积</small>{{ detailItem.areaSqm }}㎡</i><i v-if="detailItem.listingSource === '转租'"><small>剩余租期</small>{{ detailItem.remainingLeaseMonths || 0 }} 个月</i><i v-if="detailItem.listingSource === '转租' && detailItem.transferDeposit"><small>承接押金</small>{{ detailItem.transferDeposit }} 元</i></div></section>
              <section v-if="detailModuleEnabled('location') && detailHasLocation" class="detail-canvas-card detail-canvas-location"><header><span><b>{{ detailModuleTitle('location', '位置与周边') }}</b><small>{{ detailItem.location || '已保存房源定位' }}</small></span><i>已定位</i></header><button type="button" class="canvas-detail-map-toggle" @click="detailMapExpanded = !detailMapExpanded"><span><strong>{{ detailMapExpanded ? '收起地图' : '查看地图位置' }}</strong><small>已保存房源定位</small></span><i>{{ detailMapExpanded ? '⌃' : '›' }}</i></button><div v-if="detailMapExpanded" class="canvas-detail-map"><div class="canvas-detail-map-grid"><span>⌖</span><b>{{ detailItem.community }}</b><small>{{ detailItem.latitude }}，{{ detailItem.longitude }}</small></div></div></section>
              <section v-if="detailModuleEnabled('facilities')" class="detail-canvas-card detail-canvas-facilities"><header><span><b>{{ detailModuleTitle('facilities', '房屋设施') }}</b><small>家具家电等可勾选配置</small></span><i>{{ detailAmenities.length }} 项</i></header><template v-if="detailFacilityGroups.length"><div v-for="group in detailFacilityGroups" :key="group.title" class="detail-canvas-facility-group"><small>{{ group.title }}</small><div><i v-for="facility in group.facilities" :key="facility.key"><b><img v-if="detailFacilityIcon(facility.key)" :src="detailFacilityIcon(facility.key)" alt=""><span v-else>{{ facility.label.slice(0,1) }}</span></b>{{ facility.label }}</i></div></div></template><p v-else>家具家电信息待补充，可联系小皮确认</p></section>
              <section v-if="detailModuleEnabled('description')" class="detail-canvas-card detail-canvas-description"><b>{{ detailModuleTitle('description', '房源简介') }}</b><small class="detail-canvas-section-copy">房东补充说明与居住规则</small><p>{{ [detailDescription, detailItem.rulesText].filter(Boolean).join(' · ') || '暂无描述' }}</p></section>
              <section v-if="detailModuleEnabled('nearby') && detailHasLocation" class="detail-canvas-card detail-canvas-nearby"><header><span><b>{{ detailModuleTitle('nearby', '附近房源') }}</b></span><i>{{ detailNearby.length ? `${detailNearby.length} 套` : '继续找房' }}</i></header><article v-for="entry in detailNearby" :key="entry.item.id"><div :style="coverMedia(entry.item) ? { backgroundImage: `url(${mediaURL(coverMedia(entry.item)!.url)})` } : {}"></div><span><b>{{ entry.item.title || entry.item.community }}</b><small>{{ entry.item.layout || entry.item.community }}</small><em>{{ entry.item.rentType || '' }}{{ entry.item.areaSqm ? ` · ${entry.item.areaSqm}㎡` : '' }}</em><strong>¥{{ entry.item.rentMonthly }}<i>/月</i><small>{{ entry.distance < 1 ? `${Math.round(entry.distance * 1000)}m` : `${entry.distance.toFixed(1)}km` }}</small></strong></span></article><div v-if="!detailNearby.length" class="canvas-nearby-empty-state"><strong>附近暂时没有其他可租房源</strong><small>可以继续查看其他地区的新房源</small></div></section>
            </div>
            <div class="detail-canvas-actions operator-actions admin-material-detail-actions">
              <button type="button" @click="openEdit(detailItem)">编辑素材</button>
              <el-select :model-value="categories.find(name => categoryMatches(detailItem!, name))" placeholder="移动分类" :disabled="!!busyID" @click.stop @change="moveCategory(detailItem, $event)"><el-option v-for="category in categories" :key="category" :label="category" :value="category"/></el-select>
              <button v-if="!detailItem.convertedPropertyId" type="button" :disabled="busyID === detailItem.id" @click="convert(detailItem)">转为房源草稿</button><button v-else type="button" :disabled="busyID === detailItem.id" @click="continueEditing(detailItem)">{{ linkedActionLabel(detailItem) }}</button>
              <button type="button" class="detail-canvas-delete" :disabled="busyID === detailItem.id" @click="remove(detailItem)">删除素材</button>
            </div>
          </div>
        </div>
        <div class="mini-tabbar"><button><img :src="homeTabIcon" alt=""/><small>首页</small></button><button class="active"><img :src="propertyTabActiveIcon" alt=""/><small>房源</small></button><button><img :src="meTabIcon" alt=""/><small>我的</small></button></div>
      </div>
    </div>
  </el-dialog>
  <el-dialog v-model="categoryManageOpen" title="小区分类管理" width="min(760px, 94vw)" destroy-on-close>
    <div class="category-manager-intro"><div><strong>按小区归类素材</strong><p>分类名称可修改；移除前需要先处理分类下的素材。</p></div><el-tag type="info" effect="plain">{{ categories.length }} 个分类</el-tag></div>
    <div class="category-manager-toolbar"><el-input v-model="categoryDraft" placeholder="输入小区名称，新建分类" maxlength="60" @keyup.enter="addCategory"/><el-button type="primary" @click="addCategory">新增分类</el-button></div>
    <el-input v-model="categoryQuery" class="category-search" placeholder="搜索分类或地址" clearable />
    <div class="category-manager-grid">
      <article v-for="category in categoryManagementItems" :key="category.name" class="category-manager-card">
        <div class="category-card-top"><span class="community-mark">{{ category.name.slice(0,1) }}</span><el-tag :type="category.count ? 'success' : 'info'" size="small">{{ category.count ? `${category.count} 份素材` : '暂无素材' }}</el-tag></div>
        <strong :title="category.name">{{ category.name }}</strong><p :title="category.address">{{ category.address }}</p>
        <div class="category-card-actions"><el-button size="small" @click="renameCategory(category.name)">修改名称</el-button><el-button size="small" type="danger" plain :disabled="category.count > 0" @click="removeCategory(category.name)">移除</el-button></div>
      </article>
      <el-empty v-if="!categoryManagementItems.length" description="没有匹配的小区分类" />
    </div>
  </el-dialog>
  <el-dialog v-model="editOpen" title="编辑素材" width="min(620px, 94vw)" destroy-on-close><el-form v-if="editDraft" label-position="top"><el-form-item label="素材名称"><el-input v-model="editDraft.title" maxlength="60"/></el-form-item><template v-if="editDraft.rentType === '合租'"><el-form-item label="整套户型"><el-input v-model="editDraft.wholeLayout" placeholder="如 2室1厅"/></el-form-item><el-form-item label="出租房型"><el-input v-model="editDraft.layout" placeholder="如 主卧"/></el-form-item><el-form-item label="房间序号（0 表示未指定）"><el-input-number v-model="editDraft.roomSequence" :min="0" :max="9" :precision="0"/></el-form-item></template><el-form-item v-else label="户型"><el-input v-model="editDraft.layout" placeholder="如 2室1厅"/></el-form-item><el-form-item label="面积（㎡）"><el-input-number v-model="editDraft.areaSqm" :min="0" :max="999"/></el-form-item><el-form-item label="楼层"><el-input v-model="editDraft.floorInfo" placeholder="如 12/32层"/></el-form-item><el-form-item label="朝向"><el-input v-model="editDraft.orientation"/></el-form-item><el-form-item label="电梯"><el-input v-model="editDraft.elevator" placeholder="如 有"/></el-form-item><el-form-item label="价格（可选）"><el-input-number v-model="editDraft.rentMonthly" :min="0"/></el-form-item><el-form-item label="房屋设施"><el-input v-model="editAmenities" placeholder="多个设施用顿号分隔"/></el-form-item><el-form-item label="备注"><el-input v-model="editNotes" type="textarea" :rows="3"/></el-form-item></el-form><template #footer><el-button @click="editOpen = false">取消</el-button><el-button type="primary" :loading="savingEdit" @click="saveEdit">保存</el-button></template></el-dialog>
</template>

<style scoped>
.material-stats{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px;margin:0 0 20px}.material-stats article{display:flex;align-items:center;gap:12px;padding:18px 22px}.material-stats strong{font-size:28px;color:#203b60}.material-stats span{color:#718096}.material-panel{padding:22px}.material-toolbar{display:flex;justify-content:space-between;align-items:center;gap:16px}.material-tabs{display:flex;gap:8px}.material-categories{display:flex;flex-wrap:wrap;gap:8px;margin:18px 0 22px}.material-categories .el-button{margin:0}.category-count{margin-left:4px;color:#748276}.category-manager-intro{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:16px}.category-manager-intro strong{color:#263b32;font-size:16px}.category-manager-intro p{margin:5px 0 0;color:#748178;font-size:13px}.category-manager-toolbar{display:flex;gap:10px}.category-manager-toolbar .el-input{flex:1}.category-search{margin:12px 0 14px}.category-manager-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;max-height:min(54vh,480px);overflow-y:auto;padding:1px}.category-manager-card{min-width:0;padding:13px;border:1px solid #dfe7dc;border-radius:12px;background:#fbfcfa}.category-card-top{display:flex;align-items:center;justify-content:space-between;margin-bottom:10px}.community-mark{display:grid;place-items:center;width:30px;height:30px;border-radius:9px;background:#e8efe3;color:#476447;font-weight:800}.category-manager-card>strong{display:block;overflow:hidden;color:#263b32;text-overflow:ellipsis;white-space:nowrap}.category-manager-card>p{overflow:hidden;margin:6px 0 12px;color:#78837b;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.category-card-actions{display:flex;justify-content:flex-end;gap:6px}.material-categories .el-button{max-width:100%}.material-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:14px;min-height:180px}.material-card{--material-accent:#8793a2;display:flex;aspect-ratio:1;min-width:0;flex-direction:column;overflow:hidden;border:1px solid #dfe7dc;border-top:4px solid var(--material-accent);border-radius:14px;background:#fff;box-shadow:0 2px 8px #203b6008;cursor:pointer;transition:transform .16s ease,box-shadow .16s ease}.material-card:not(.converted){--material-accent:#bb8d47}.material-card.converted{--material-accent:#3c7c68}.material-card:hover{transform:translateY(-2px);box-shadow:0 8px 20px #203b6014}.material-card:focus-visible{outline:3px solid #7b9f73;outline-offset:2px}.material-cover{position:relative;display:grid;flex:0 0 39%;place-items:center;width:100%;min-height:0;overflow:hidden;background:#edf1eb}.material-cover img{width:100%;height:100%;object-fit:cover}.media-empty{display:flex;width:100%;height:100%;flex-direction:column;align-items:center;justify-content:center;gap:6px;color:#8793a2;font-size:12px}.material-state{position:absolute;top:9px;left:9px;border:0;background:#fffF;font-weight:700}.media-count{position:absolute;right:8px;bottom:8px;padding:4px 7px;border-radius:7px;background:#17243bd9;color:#fff;font-size:11px}.material-copy{display:flex;min-width:0;min-height:0;flex:1;flex-direction:column;align-items:flex-start;gap:5px;padding:9px 11px}.material-title{width:100%}.material-title strong{display:block;overflow:hidden;color:#25344a;font-size:15px;text-overflow:ellipsis;white-space:nowrap}.material-community{overflow:hidden;max-width:100%;color:#607166;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.material-copy p{overflow:hidden;width:100%;margin:0;color:#7b8794;font-size:11px;text-overflow:ellipsis;white-space:nowrap}.material-meta{display:flex;max-height:38px;flex-wrap:wrap;gap:4px;overflow:hidden}.material-meta span{padding:3px 5px;border-radius:6px;background:#f1f5ef;color:#586a57;font-size:10px;line-height:1.3}.material-card-footer{display:flex;width:100%;align-items:center;justify-content:space-between;gap:8px;margin-top:auto;padding-top:3px}.material-copy small{overflow:hidden;color:#8793a2;font-size:11px;text-overflow:ellipsis;white-space:nowrap}.material-rent{color:#d76440;font-size:14px;white-space:nowrap}.open-detail{color:#71896b;font-size:11px;white-space:nowrap}.detail-gallery{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.detail-gallery img,.detail-gallery video{width:100%;height:210px;border-radius:12px;background:#f2f4f6;object-fit:cover}.detail-heading{display:flex;align-items:flex-start;justify-content:space-between;margin-top:20px}.detail-heading h2{margin:0;font-size:22px}.detail-heading p,.detail-description{color:#718096}.detail-tags{display:flex;flex-wrap:wrap;gap:8px;margin:14px 0}.detail-actions{display:flex;flex-wrap:wrap;gap:10px;margin-top:18px}.detail-actions .el-select{width:220px}@media(max-width:900px){.material-toolbar{align-items:center}.material-stats{gap:8px}.material-stats article{padding:12px;flex-direction:column;align-items:flex-start}}@media(max-width:600px){.material-panel{padding:14px}.material-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:9px}.material-copy{padding:7px;gap:4px}.material-title strong{font-size:12px}.material-community,.material-copy p{font-size:10px}.material-meta span{font-size:9px;padding:2px 4px}.material-rent{font-size:11px}.open-detail{font-size:10px}.category-manager-grid{grid-template-columns:1fr}.category-manager-toolbar .el-button{padding-inline:12px}.detail-gallery img,.detail-gallery video{height:150px}}
</style>

<style scoped>
:global(.admin-material-detail-dialog){padding:0!important;border:0!important;border-radius:0!important;background:transparent!important;box-shadow:none!important}
:global(.admin-material-detail-dialog .el-dialog__header){display:none!important}
:global(.admin-material-detail-dialog .el-dialog__body){padding:0!important}
.admin-material-phone-frame{display:grid;place-items:center;background:transparent}
.admin-material-phone{width:min(402px,37.74vh)!important;height:min(82vh,850px)!important;aspect-ratio:auto!important;flex:none!important;overflow:hidden}
.admin-material-phone .page-mini-content{min-height:0;padding:0;overflow:hidden}
.admin-material-phone .detail-canvas{display:flex;height:100%;min-height:0;flex-direction:column}
.admin-material-phone .admin-material-detail-scroll{flex:1;min-height:0;overflow:auto;padding:0 6px 8px}
.admin-material-phone .detail-canvas-cover{display:block;width:calc(100% + 12px);max-width:none;height:auto;min-height:0;aspect-ratio:750 / 720;margin:0 -6px;flex:none;background-position:center;background-size:cover}
.admin-material-phone .detail-canvas-cover>video{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}
.admin-material-phone .detail-canvas-media-tabs{right:10px;bottom:10px;left:10px;display:flex;width:auto;flex-wrap:nowrap;gap:5px}
.admin-material-phone .detail-canvas-media-tabs>button{display:flex;width:auto;min-width:0;height:22px;flex:none;align-items:center;justify-content:center;padding:0 9px;white-space:nowrap;font-size:8px}
.admin-material-phone .detail-canvas-media-pager{height:22px}
.admin-material-phone .admin-material-overview{margin-top:8px;padding:11px}
.admin-material-phone .detail-canvas-price{font-size:19px}
.admin-material-phone .detail-canvas-title{margin-top:6px;font-size:13px}
.admin-material-phone .detail-canvas-address{font-size:9px}
.admin-material-phone .detail-canvas-tags{display:flex;flex-wrap:wrap;gap:4px;margin-top:8px}
.admin-material-phone .detail-canvas-tags i{padding:3px 5px;border-radius:5px;background:#edf0dc;color:#536a45;font-size:7px;font-style:normal}
.admin-material-phone .detail-canvas-description{padding:11px}
.admin-material-phone .admin-material-detail-actions{display:grid;grid-template-columns:1fr 1fr;align-items:center;gap:6px;padding:8px 10px calc(10px + env(safe-area-inset-bottom));background:#fffdf9}
.admin-material-phone .admin-material-detail-actions>button{min-width:0;min-height:30px;padding:0 5px;border:0;border-radius:7px;background:#edf1e8;color:#40543f;font-size:8px;font-weight:750;white-space:nowrap}
.admin-material-phone .admin-material-detail-actions>button:nth-of-type(2){background:#40543f;color:#fff}
.admin-material-phone .admin-material-detail-actions>button.detail-canvas-delete{background:#faeeeb;color:#a24d40}
.admin-material-phone .admin-material-detail-actions .el-select{width:100%;min-width:0}
.admin-material-phone .admin-material-detail-actions .el-select__wrapper{min-height:30px;padding:0 7px;font-size:8px}
.admin-material-phone .converted-label{justify-self:center;color:#3c7c68;font-size:8px;font-weight:750}
.material-grid{grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:11px}
.material-card{aspect-ratio:1;}
.material-cover{flex-basis:34%}
.material-copy{gap:4px;padding:7px 9px}
.material-title strong{font-size:13px}
.material-community{font-size:10px}
.material-copy p{font-size:9px}
.material-meta{max-height:32px;gap:3px}
.material-meta span{padding:2px 4px;font-size:9px}
.material-state{top:6px;left:6px;font-size:9px}
.media-count{right:6px;bottom:6px;padding:3px 5px;font-size:9px}
.material-rent{font-size:12px}
.open-detail{font-size:9px}
@media(max-width:600px){.material-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:9px}.material-copy{padding:6px 7px}.material-title strong{font-size:11px}.material-community,.material-copy p{font-size:9px}.material-meta span{font-size:8px}.material-rent{font-size:10px}.open-detail{font-size:8px}}
</style>
