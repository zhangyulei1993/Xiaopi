<script setup lang="ts">
import { House } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, ref, watch } from 'vue'
import type { Property } from '../types/domain'
import homeTabIcon from '../../../miniapp/src/static/tabbar/home.png'
import propertyTabActiveIcon from '../../../miniapp/src/static/tabbar/house-active.png'
import meTabIcon from '../../../miniapp/src/static/tabbar/me.png'

type Status = { key: string; label: string; icon: string }
const props = defineProps<{ api: <T>(path: string, options?: RequestInit) => Promise<T>; apiBase: string; requestedEdit?: Property | null; channels: { title: string; latitude?: number; longitude?: number; radiusKm?: number }[]; properties: Property[]; filteredProperties: Property[]; filter: string; statuses: Status[]; loading: boolean; normalizedStatus: (value: string) => string; statusLabel: (value: string) => string; cover: (value: Property) => string; nearbyRadius?: number; nearbyLimit?: number }>()
const emit = defineEmits<{ 'edit-opened': []; refresh: []; filter: [value: string]; status: [item: Property, value: string]; saved: [item: Property]; deleted: [id: string] }>()
const failedCovers = ref(new Set<string>())
const selectedProperty = ref<Property | null>(null)
const detailDialogVisible = computed({ get: () => Boolean(selectedProperty.value), set: (visible: boolean) => { if (!visible) selectedProperty.value = null } })
const detailMedia = computed(() => selectedProperty.value?.media || [])
const detailMediaIndex = ref(0)
const failedDetailMedia = ref(new Set<number>())
const mapExpanded = ref(false)
const detailScrolled = ref(false)
const coverAvailable = (item: Property) => Boolean(props.cover(item)) && !failedCovers.value.has(props.cover(item))
const markCoverFailed = (item: Property) => { failedCovers.value = new Set(failedCovers.value).add(props.cover(item)) }
const cardStatusLabel = (item: Property) => ({ draft: '草稿', published: '已发布', available: '可租', rented: '已租', offline: '已下架' }[props.normalizedStatus(item.status)] || props.statusLabel(item.status))
const openDetail = (item: Property) => { selectedProperty.value = item; detailMediaIndex.value = 0; mapExpanded.value = false; detailScrolled.value = false; failedDetailMedia.value = new Set() }
const detailImages = computed(() => detailMedia.value.filter(media => media.type === 'image'))
const detailVideos = computed(() => detailMedia.value.filter(media => media.type === 'video'))
const nearbyProperties = computed(() => {
  const current = selectedProperty.value
  if (!current || !Number(current.latitude) || !Number(current.longitude)) return []
  const radians = (value: number) => value * Math.PI / 180
  const distanceKm = (item: Property) => {
    const lat = radians(Number(item.latitude) - Number(current.latitude))
    const lng = radians(Number(item.longitude) - Number(current.longitude))
    const a = Math.sin(lat / 2) ** 2 + Math.cos(radians(Number(current.latitude))) * Math.cos(radians(Number(item.latitude))) * Math.sin(lng / 2) ** 2
    return 6371 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))
  }
  return props.properties.filter(item => item.id !== current.id && Number(item.latitude) && Number(item.longitude) && props.normalizedStatus(item.status) === 'available')
    .map(item => ({ item, distance: distanceKm(item) })).filter(entry => entry.distance <= (props.nearbyRadius || 3)).sort((a, b) => a.distance - b.distance).slice(0, props.nearbyLimit || 4)
})
const mediaIsVideo = computed(() => detailMedia.value[detailMediaIndex.value]?.type === 'video')
const selectDetailMedia = (type: 'image' | 'video') => {
  const index = detailMedia.value.findIndex(media => media.type === type)
  if (index >= 0) detailMediaIndex.value = index
}
const stepDetailMedia = (direction: number) => {
  if (!detailMedia.value.length) return
  detailMediaIndex.value = (detailMediaIndex.value + direction + detailMedia.value.length) % detailMedia.value.length
}
const editDraft = ref<Property | null>(null)
const editVisible = ref(false)
const savingEdit = ref(false)
const deleting = ref(false)
const editError = ref('')
let editBatch = ''
let submittedBody = ''
const submitted = ref(false)
const canReloadEdit = ref(false)
const reloadingEdit = ref(false)
let editBase: Property | null = null
let locationLookup = 0
const locatingCommunity = ref(false)
type PendingFile = { file: File; key: string; id?: string; type: string; url?: string }
type EditMedia = NonNullable<Property['media']>[number] | PendingFile
const editMedia = ref<EditMedia[]>([])
function chooseEditMedia(event: Event) {
  const input = event.target as HTMLInputElement
  for (const file of Array.from(input.files || [])) {
    const video = file.type === 'video/mp4'
    if ((!video && !['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) || file.size > (video ? 40 : 10) * 1024 * 1024) {
      ElMessage.warning(`无法添加 ${file.name}，请检查文件格式和大小`)
      continue
    }
    editMedia.value.push({ file, key: newEditBatch(), type: video ? 'video' : 'image' })
  }
  input.value = ''
}
function moveEditMedia(index: number, offset: number) {
  const media = editMedia.value
  if (!media[index + offset]) return
  const [item] = media.splice(index, 1)
  media.splice(index + offset, 0, item)
}
const editFields = [
  ['title', '房源名称'], ['community', '小区'], ['location', '地址'],
  ['layout', '户型 / 出租房型'], ['wholeLayout', '整套户型'],
  ['buildingNo', '楼栋'], ['unitNo', '单元'], ['roomNo', '室号'],
  ['floorInfo', '楼层'], ['orientation', '朝向'], ['elevator', '电梯'],
  ['depositPayment', '押付方式'], ['feeText', '中介费'], ['utilitiesText', '水电杂费'],
  ['availableDate', '可入住时间'],
] as const
const numericFields = [
  ['rentMonthly', '月租'], ['areaSqm', '面积'], ['roomSequence', '房间序号'],
  ['latitude', '纬度'], ['longitude', '经度'],
] as const
function newEditBatch() {
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), value => value.toString(16).padStart(2, '0')).join('')
}
const locationKeys = ['community', 'location', 'latitude', 'longitude'] as const
const normalizeCommunity = (name: string) => name.normalize('NFKC').replace(/[\p{P}\p{Z}]/gu, '').toLowerCase()
async function changeLocationText(key: 'community' | 'location', value: string) {
  const item = editDraft.value
  if (!item || item[key] === value) return
  item[key] = value
  item.latitude = 0
  item.longitude = 0
  const lookup = ++locationLookup
  locatingCommunity.value = false
  if (key !== 'community') return
  item.location = ''
  item.buildingNo = ''
  item.unitNo = ''
  item.roomNo = ''
  const name = normalizeCommunity(value)
  if (!name) return
  locatingCommunity.value = true
  try {
    const entries = await props.api<Record<string, { address: string; latitude: number; longitude: number }>>('/v1/admin/community-locations')
    const match = Object.entries(entries).find(([community]) => normalizeCommunity(community) === name)?.[1]
    if (match && lookup === locationLookup && editDraft.value === item && editVisible.value && !savingEdit.value && !submitted.value) {
      item.location = match.address
      item.latitude = match.latitude
      item.longitude = match.longitude
    }
  } catch {
    if (lookup === locationLookup && editVisible.value) editError.value = '小区位置读取失败，请手动补齐地址和坐标。'
  } finally {
    if (lookup === locationLookup) locatingCommunity.value = false
  }
}
function resolveAreaLabel(item: Property) {
  if (!editBase || locationKeys.every(key => item[key] === editBase?.[key])) return item.areaLabel
  const latitude = Number(item.latitude)
  const longitude = Number(item.longitude)
  if (!latitude || !longitude) return ''
  const radians = (degrees: number) => degrees * Math.PI / 180
  let nearest = Infinity
  let label = ''
  for (const channel of props.channels) {
    if (!channel.latitude || !channel.longitude) continue
    const a = Math.sin(radians(channel.latitude - latitude) / 2) ** 2
      + Math.cos(radians(latitude)) * Math.cos(radians(channel.latitude)) * Math.sin(radians(channel.longitude - longitude) / 2) ** 2
    const distance = 12742 * Math.asin(Math.min(1, Math.sqrt(a)))
    if (distance <= (channel.radiusKm || 3) && distance < nearest) {
      nearest = distance
      label = channel.title
    }
  }
  return label
}
async function reloadEdit() {
  const draft = editDraft.value
  const base = editBase
  if (!draft || !base || reloadingEdit.value || savingEdit.value) return
  reloadingEdit.value = true
  try {
    const latest = (await props.api<Property[]>('/v1/properties')).find(item => item.id === draft.id)
    if (!latest) { editError.value = '这套房源已不存在。'; return }
    const keys = [...editFields.map(([key]) => key), ...numericFields.map(([key]) => key), 'rentType', 'description', 'rulesText'] as const
    const changed = new Set<keyof Property>(keys.filter(key => draft[key] !== base[key]))
    if (locationKeys.some(key => changed.has(key))) {
      for (const key of [...locationKeys, 'buildingNo', 'unitNo', 'roomNo'] as const) changed.add(key)
    }
    if (['rentType', 'layout', 'wholeLayout', 'roomSequence'].some(key => changed.has(key as keyof Property))) {
      for (const key of ['rentType', 'layout', 'wholeLayout', 'roomSequence'] as const) changed.add(key)
    }
    const conflicts = [...changed].filter(key => latest[key] !== base[key] && latest[key] !== draft[key])
    if (conflicts.length) {
      const labels = new Map<keyof Property, string>([...editFields, ...numericFields, ['rentType', '出租方式'], ['description', '房源描述'], ['rulesText', '居住规则']])
      const summary = conflicts.map(key => `${labels.get(key)}：最新「${latest[key] ?? '空'}」，本次「${draft[key] ?? '空'}」`).join('；')
      try {
        await ElMessageBox.confirm(summary, '核对冲突字段', { confirmButtonText: '保留本次修改', cancelButtonText: '返回表单', type: 'warning' })
      } catch { return }
    }
    const order = (item: Property) => (item.media || []).map(media => media.id).join(',')
    const existingOrder = editMedia.value.filter(media => !('file' in media)).map(media => media.id).join(',')
    const changedMedia = existingOrder !== order(base) || editMedia.value.some(media => 'file' in media)
    const latestMediaChanged = order(latest) !== order(base)
    if (changedMedia && latestMediaChanged) {
      try {
        await ElMessageBox.confirm('媒体列表已更新。请基于最新媒体重新调整顺序，尚未保存的文件会保留。', '媒体列表已更新', { confirmButtonText: '使用最新媒体', cancelButtonText: '返回表单' })
      } catch { return }
    }
    locationLookup++
    editBase = { ...latest }
    editDraft.value = { ...latest, ...Object.fromEntries([...changed].map(key => [key, draft[key]])) }
    editBatch = newEditBatch()
    if (latestMediaChanged) {
      const pending = editMedia.value.filter(media => 'file' in media && !latest.media?.some(saved => saved.id === media.id))
      editMedia.value = [...(latest.media || []), ...pending]
    }
    for (const media of editMedia.value) {
      if ('file' in media) {
        media.id = undefined
        media.url = undefined
      }
    }
    submittedBody = ''
    submitted.value = false
    canReloadEdit.value = false
    editError.value = ''
    ElMessage.info('已读取最新版本并保留本次修改，请核对后保存')
  } catch (error) {
    editError.value = error instanceof Error ? error.message : '读取最新房源失败'
  } finally {
    reloadingEdit.value = false
  }
}
function openEdit(item: Property) {
  editMedia.value = [...(item.media || [])]
  editDraft.value = { ...item, media: [...(item.media || [])] }
  editBase = { ...item }
  locationLookup++
  locatingCommunity.value = false
  editBatch = newEditBatch()
  submittedBody = ''
  submitted.value = false
  canReloadEdit.value = false
  editError.value = ''
  detailDialogVisible.value = false
  editVisible.value = true
}
watch(() => props.requestedEdit, item => {
  if (!item) return
  openEdit(item)
  emit('edit-opened')
}, { immediate: true })
async function saveEdit() {
  const item = editDraft.value
  if (!item || savingEdit.value || locatingCommunity.value) return
  if (item.editRevision == null || editMedia.value.some(media => !('file' in media) && !media.id)) {
    editError.value = '当前服务端缺少统一保存所需的数据，请更新服务端并刷新房源列表。'
    return
  }
  if (reloadingEdit.value) return
  savingEdit.value = true
  editError.value = ''
  try {
    if (!submitted.value) {
      for (const file of editMedia.value) {
        if (!('file' in file) || file.id) continue
        const body = new FormData()
        body.append('file', file.file)
        body.append('propertyId', item.id)
        body.append('editBatch', editBatch)
        body.append('uploadKey', file.key)
        const uploaded = await props.api<{ id: string; url: string }>('/v1/media/upload', { method: 'POST', body })
        file.id = uploaded.id
        file.url = uploaded.url
      }
      const mediaIds = editMedia.value.map(media => media.id)
      submittedBody = JSON.stringify({ ...item, areaLabel: resolveAreaLabel(item), editBatch, mediaIds })
      submitted.value = true
    }
    const saved = await props.api<Property>('/v1/properties/update', {
      method: 'PATCH',
      body: submittedBody,
    })
    const updated = { ...item, ...saved, media: editMedia.value.map(media => ({ id: media.id, type: media.type, url: media.url || '' })) }
    emit('saved', updated)
    editVisible.value = false
    openDetail(updated)
    ElMessage.success('房源已保存')
  } catch (error) {
    // A definite rejection permits adjustments. A lost response must retry the same payload.
    const status = (error as { status?: number })?.status
    if (status && status >= 400 && status < 500) submitted.value = false
    canReloadEdit.value = !status || status === 409 || status >= 500
    editError.value = error instanceof Error ? error.message : '房源保存失败'
  } finally {
    savingEdit.value = false
  }
}
async function confirmDelete(item: Property) {
  if (deleting.value) return
  try {
    await ElMessageBox.confirm(`确定删除「${item.title || item.community}」吗？此操作无法撤销。`, '删除房源', { type: 'warning', confirmButtonText: '删除房源', cancelButtonText: '取消' })
  } catch { return }
  deleting.value = true
  try {
    await props.api(`/v1/properties/${encodeURIComponent(item.id)}`, { method: 'DELETE' })
    emit('deleted', item.id)
    detailDialogVisible.value = false
    ElMessage.success('房源已删除')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '房源删除失败')
  } finally {
    deleting.value = false
  }
}
const mediaURL = (url: string) => /^https?:\/\//i.test(url) ? url : props.apiBase.replace(/\/api\/?$/, '') + (url.startsWith('/') ? url : '/' + url)
const selectedMediaBackground = computed(() => {
  const selected = detailMedia.value[detailMediaIndex.value]
  const media = selected?.type === 'image' ? selected : detailImages.value[0]
  const item = selectedProperty.value
  return media ? { backgroundImage: `url(${mediaURL(media.url)})` } : item && coverAvailable(item) ? { backgroundImage: `url(${props.cover(item)})` } : {}
})
const detailFacts = computed(() => {
  const item = selectedProperty.value
  if (!item) return []
  return [
    ['面积', item.rentType === '合租' && item.wholeAreaSqm ? `${item.wholeAreaSqm}㎡` : item.areaSqm ? `${item.areaSqm}㎡` : '待补充'],
    ['户型', item.rentType === '合租' && item.wholeLayout ? item.wholeLayout : item.layout || item.actualLayout || '待补充'],
    ['楼层', item.floorInfo || '待补充'],
    ...(item.orientation ? [['朝向', item.orientation] as [string, string]] : []),
    ...(item.elevator ? [['电梯', item.elevator] as [string, string]] : []),
    ['租赁方式', item.rentType || '待补充'],
  ] as [string, string][]
})
const detailAmenities = computed(() => {
  const value = String(selectedProperty.value?.description || '')
  const match = value.match(/配套设施[ \t]*[：:][ \t]*([^\n]*)/)
  return [...new Set((match?.[1] || '').split(/[、,，]/).map(name => name.trim()).filter(Boolean))]
})
const detailDescription = computed(() => String(selectedProperty.value?.description || '').replace(/配套设施[ \t]*[：:][ \t]*[^\n]*\n?/, '').trim())
const detailDoorplate = computed(() => {
  const item = selectedProperty.value
  if (!item) return ''
  return [item.buildingNo, item.unitNo && `${item.unitNo}单元`, item.roomNo || (item.roomSequence ? `${item.roomSequence}号房` : '')].filter(Boolean).join(' ')
})
const detailHasLocation = computed(() => Boolean(Number(selectedProperty.value?.latitude) && Number(selectedProperty.value?.longitude)))
const detailStatusPreview = computed(() => {
  const status = selectedProperty.value?.status
  if (status === 'offline') return '已下架预览'
  if (status === 'draft' || status === 'pending_review') return '待发布预览'
  return ''
})
const statusActions = (value: string) => {
  const status = props.normalizedStatus(value)
  if (status === 'draft') return [{ label: '发布房源', value: 'available' }]
  if (status === 'published') return [{ label: '设为可租', value: 'available' }, { label: '标记已租', value: 'rented' }, { label: '下架', value: 'offline' }]
  if (status === 'available') return [{ label: '标记已租', value: 'rented' }, { label: '下架', value: 'offline' }]
  if (status === 'rented') return [{ label: '设为可租', value: 'available' }]
  if (status === 'offline') return [{ label: '上架可租', value: 'available' }, { label: '设为已租', value: 'rented' }]
  return []
}
</script>

<template>
  <section class="page-header"><div><span>房源管理</span><h1>房源列表</h1></div><el-button :loading="loading" @click="emit('refresh')">刷新数据</el-button></section>
  <div class="property-status-grid"><button class="surface property-status-card" :class="{ active: filter === 'all' }" @click="emit('filter', 'all')"><i>全</i><span>全部</span><strong>{{ properties.length }}</strong></button><button v-for="status in statuses" :key="status.key" class="surface property-status-card" :class="{ active: filter === status.key }" @click="emit('filter', status.key)"><i>{{ status.icon }}</i><span>{{ status.label }}</span><strong>{{ properties.filter(item => normalizedStatus(item.status) === status.key).length }}</strong></button></div>
  <section class="surface property-list-panel">
    <div class="table-heading"><div><h2>{{ filter === 'all' ? '全部房源' : statusLabel(filter) }}</h2><p>共 {{ filteredProperties.length }} 套</p></div><span class="occupancy-key"><i class="key-available"/>可租 <i class="key-rented"/>已租 <i class="key-offline"/>下架</span></div>
    <div v-if="filteredProperties.length" class="property-card-grid">
      <article v-for="item in filteredProperties" :key="item.id" class="property-card" :class="`status-${normalizedStatus(item.status)}`" role="button" tabindex="0" :aria-label="`查看${item.title || item.community}详情`" @click="openDetail(item)" @keydown.enter.prevent="openDetail(item)" @keydown.space.prevent="openDetail(item)">
        <div class="property-cover"><img v-if="coverAvailable(item)" :src="cover(item)" :alt="`${item.community}房源照片`" @error="markCoverFailed(item)"/><div v-else class="cover-fallback"><el-icon><House /></el-icon><span>{{ item.community }}</span></div><el-tag class="cover-status" size="small">{{ cardStatusLabel(item) }}</el-tag><span v-if="item.roomSequence || item.roomNo" class="property-room">{{ item.roomNo || `${item.roomSequence}号房` }}</span></div>
        <div class="property-copy">
          <strong class="property-name" :title="item.title || item.community">{{ item.title || item.community }}</strong>
          <span class="property-community" :title="item.community">{{ item.community }}</span>
          <div class="property-tags"><span>{{ item.layout || '户型待补充' }}</span><span v-if="item.areaSqm">{{ item.areaSqm }}㎡</span><span v-for="tag in (item.displayTags || []).slice(0,2)" :key="tag">{{ tag }}</span></div>
          <small :title="item.location">{{ item.location || '地址待补充' }}</small>
          <div class="property-state-line"><span class="state-readout"><i/>{{ cardStatusLabel(item) }}</span><b class="property-rent">{{ item.rentMonthly > 0 ? `${item.rentMonthly} 元/月` : '租金待补充' }}</b></div>
          <el-select :model-value="normalizedStatus(item.status)" size="small" :aria-label="`修改${item.community}出租状态`" @click.stop @change="emit('status', item, $event)"><el-option :label="cardStatusLabel(item)" :value="normalizedStatus(item.status)" /><el-option v-for="action in statusActions(item.status)" :key="action.value" :label="action.label" :value="action.value" /></el-select>
        </div>
      </article>
    </div>
    <el-empty v-else description="该状态下暂无房源"/>
  </section>
  <el-dialog v-model="detailDialogVisible" class="admin-property-detail-dialog" width="min(420px, 94vw)" :show-close="true" :destroy-on-close="true" align-center>
    <template #header><span class="detail-dialog-accessible-title">房源详情</span></template>
    <div v-if="selectedProperty" class="admin-property-canvas-frame">
      <div class="phone-preview page-phone-preview preview-propertyDetail canvas-identity-admin canvas-operator">
        <div class="phone-status"><span>22:18</span><span>4G ▰</span></div>
        <div class="page-mini-content">
          <div class="detail-canvas" :class="{ operator: true }">
            <div v-if="detailScrolled" class="canvas-detail-sticky"><i class="canvas-detail-sticky-capsule" aria-hidden="true">•••　⊙</i><button type="button" aria-label="返回" @click="detailDialogVisible = false">‹</button><span>房源详情</span></div>
            <div class="detail-canvas-scroll" @scroll.passive="detailScrolled = ($event.currentTarget as HTMLElement).scrollTop > 65">
              <div class="detail-canvas-cover" :class="{ 'is-placeholder': !detailMedia.length || failedDetailMedia.has(detailMediaIndex) }" :style="selectedMediaBackground">
                <template v-if="mediaIsVideo && !failedDetailMedia.has(detailMediaIndex)"><video :key="detailMedia[detailMediaIndex].url" :src="mediaURL(detailMedia[detailMediaIndex].url)" controls playsinline preload="metadata" @error="failedDetailMedia.add(detailMediaIndex)" /></template>
                <div class="detail-canvas-media-nav"><button type="button" aria-label="返回房源列表" @click="detailDialogVisible = false">‹</button><i>•••　⊙</i></div>
                <div v-if="!detailMedia.length || failedDetailMedia.has(detailMediaIndex)" class="detail-canvas-empty-media"><el-icon><House /></el-icon><b>{{ failedDetailMedia.has(detailMediaIndex) ? '媒体暂不可用' : '暂无实拍图' }}</b><small>{{ failedDetailMedia.has(detailMediaIndex) ? '请检查媒体地址' : '联系小皮了解房源详情' }}</small></div>
                <div v-if="detailMedia.length" class="detail-canvas-media-tabs"><button v-if="detailVideos.length" type="button" :class="{ active: mediaIsVideo }" @click="selectDetailMedia('video')">视频</button><button v-if="detailImages.length" type="button" :class="{ active: !mediaIsVideo }" @click="selectDetailMedia('image')">照片 {{ mediaIsVideo ? 1 : detailMedia.slice(0, detailMediaIndex + 1).filter(media => media.type === 'image').length }}/{{ detailImages.length }}</button><span class="detail-canvas-media-pager"><button type="button" aria-label="上一张" @click="stepDetailMedia(-1)">‹</button>{{ detailMediaIndex + 1 }}/{{ detailMedia.length }}<button type="button" aria-label="下一张" @click="stepDetailMedia(1)">›</button></span></div>
              </div>
              <section class="detail-canvas-card overview-card">
                <em v-if="detailStatusPreview" class="detail-canvas-status-preview">{{ detailStatusPreview }}</em>
                <div class="detail-canvas-price">{{ selectedProperty.rentMonthly || 0 }}<small>元/月</small></div>
                <strong class="detail-canvas-title">{{ selectedProperty.title || selectedProperty.community }}</strong>
                <span class="detail-canvas-address">{{ selectedProperty.location || '位置待补充' }}</span>
                <small v-if="detailDoorplate" class="preview-operator-doorplate">门牌定位 · {{ detailDoorplate }}</small>
                <div v-if="selectedProperty.listingSource === '转租'" class="detail-canvas-meta"><em>转租 · 剩余 {{ selectedProperty.remainingLeaseMonths || 0 }} 个月</em></div>
                <div class="detail-canvas-facts"><i v-for="([label, value]) in detailFacts" :key="label"><small>{{ label }}</small>{{ value }}</i></div>
              </section>
              <section v-if="selectedProperty.depositPayment || selectedProperty.feeText || selectedProperty.utilitiesText || selectedProperty.listingSource === '转租' || (selectedProperty.rentType === '合租' && (selectedProperty.layout || selectedProperty.areaSqm))" class="detail-canvas-card detail-canvas-info"><b>租赁信息</b><small class="detail-canvas-section-copy">费用与租期说明</small><div><i v-if="selectedProperty.depositPayment"><small>押付方式</small>{{ selectedProperty.depositPayment }}</i><i v-if="selectedProperty.feeText"><small>中介费</small>{{ selectedProperty.feeText }}</i><i v-if="selectedProperty.utilitiesText" class="wide"><small>水电杂费</small>{{ selectedProperty.utilitiesText }}</i><i v-if="selectedProperty.rentType === '合租' && selectedProperty.layout"><small>使用空间</small>{{ selectedProperty.layout }}</i><i v-if="selectedProperty.rentType === '合租' && selectedProperty.areaSqm"><small>使用面积</small>{{ selectedProperty.areaSqm }}㎡</i><i v-if="selectedProperty.listingSource === '转租'"><small>剩余租期</small>{{ selectedProperty.remainingLeaseMonths || 0 }} 个月</i></div></section>
              <section v-if="detailHasLocation" class="detail-canvas-card detail-canvas-location"><header><span><b>位置与周边</b><small>{{ selectedProperty.location || '已保存房源定位' }}</small></span><i>已定位</i></header><button type="button" class="canvas-detail-map-toggle" @click="mapExpanded = !mapExpanded"><span><strong>{{ mapExpanded ? '收起地图' : '查看地图位置' }}</strong><small>已保存房源定位</small></span><i>{{ mapExpanded ? '⌃' : '›' }}</i></button><div v-if="mapExpanded" class="canvas-detail-map"><div class="canvas-detail-map-grid"><span>⌖</span><b>{{ selectedProperty.community }}</b><small>{{ selectedProperty.latitude }}，{{ selectedProperty.longitude }}</small></div></div></section>
              <section v-if="detailAmenities.length" class="detail-canvas-card detail-canvas-facilities"><header><span><b>房屋设施</b><small>家具家电等可勾选配置</small></span><i>{{ detailAmenities.length }} 项</i></header><div class="admin-detail-facility-tags"><i v-for="tag in detailAmenities" :key="tag"><b>{{ tag.slice(0, 1) }}</b>{{ tag }}</i></div></section>
              <section v-if="detailDescription || selectedProperty.rulesText" class="detail-canvas-card detail-canvas-description"><b>房源描述</b><small class="detail-canvas-section-copy">房东补充说明与居住规则</small><p>{{ [detailDescription, selectedProperty.rulesText].filter(Boolean).join(' · ') }}</p></section>
              <section v-if="detailHasLocation" class="detail-canvas-card detail-canvas-nearby"><header><span><b>附近房源</b></span><i>{{ nearbyProperties.length ? `${nearbyProperties.length} 套` : '继续找房' }}</i></header><article v-for="entry in nearbyProperties" :key="entry.item.id"><div :style="coverAvailable(entry.item) ? { backgroundImage: `url(${cover(entry.item)})` } : {}"/><span><b>{{ entry.item.title || entry.item.community }}</b><small>{{ entry.item.layout || entry.item.community }}</small><strong>¥{{ entry.item.rentMonthly }}<i>/月</i></strong></span></article><div v-if="!nearbyProperties.length" class="canvas-nearby-empty-state"><strong>附近暂时没有其他可租房源</strong><small>可以继续查看其他地区的新房源</small><button type="button" @click="detailDialogVisible = false">返回房源列表</button></div></section>
            </div>
            <div class="detail-canvas-actions operator-actions"><div class="detail-canvas-admin-lead"><button type="button" :disabled="deleting" @click="openEdit(selectedProperty!)">编辑房源</button><button type="button" class="detail-canvas-delete" :disabled="deleting" @click="confirmDelete(selectedProperty!)">{{ deleting ? '删除中…' : '删除房源' }}</button></div></div>
          </div>
        </div>
        <div class="mini-tabbar"><button><img :src="homeTabIcon" alt=""/><small>首页</small></button><button class="active"><img :src="propertyTabActiveIcon" alt=""/><small>房源</small></button><button><img :src="meTabIcon" alt=""/><small>我的</small></button></div>
      </div>
    </div>
  </el-dialog>
  <el-dialog v-model="editVisible" title="编辑房源" width="min(740px, 94vw)" :close-on-click-modal="false" :show-close="!savingEdit && !reloadingEdit" :close-on-press-escape="!savingEdit && !reloadingEdit">
    <el-alert v-if="editError" :title="editError" type="error" show-icon :closable="false" />
    <el-form v-if="editDraft" label-position="top" :disabled="savingEdit || reloadingEdit || submitted" class="property-edit-form">
      <el-form-item v-for="[key, label] in editFields" :key="key" :label="label"><el-input v-if="key === 'community' || key === 'location'" :model-value="editDraft[key]" @change="changeLocationText(key, $event)" /><el-input v-else v-model="editDraft[key]" /></el-form-item>
      <el-form-item v-for="[key, label] in numericFields" :key="key" :label="label"><el-input-number v-model="editDraft[key]" @change="(key === 'latitude' || key === 'longitude') && (locationLookup++, locatingCommunity = false)" :precision="key === 'latitude' || key === 'longitude' ? 6 : key === 'areaSqm' ? 2 : 0" /></el-form-item>
      <el-form-item label="出租方式"><el-select v-model="editDraft.rentType"><el-option label="整租" value="整租" /><el-option label="合租" value="合租" /></el-select></el-form-item>
      <el-form-item label="房源描述" class="property-edit-wide"><el-input v-model="editDraft.description" type="textarea" :rows="4" /></el-form-item>
      <el-form-item label="图片与视频" class="property-edit-wide">
        <div class="property-edit-media">
          <div v-for="(media, index) in editMedia" :key="'file' in media ? media.key : media.id || media.url">
            <span v-if="'file' in media">{{ media.file.name }}</span>
            <a v-else :href="mediaURL(media.url)" target="_blank" rel="noopener noreferrer">{{ media.type === 'video' ? '视频' : '图片' }} {{ index + 1 }}</a>
            <el-button size="small" :disabled="index === 0" @click="moveEditMedia(index, -1)">上移</el-button>
            <el-button size="small" :disabled="index === editMedia.length - 1" @click="moveEditMedia(index, 1)">下移</el-button>
            <el-button size="small" @click="editMedia.splice(index, 1)">移除</el-button>
          </div>
          <input type="file" accept="image/jpeg,image/png,image/webp,video/mp4" multiple :disabled="savingEdit || reloadingEdit || submitted" @change="chooseEditMedia" />
        </div>
      </el-form-item>
      <el-form-item label="居住规则" class="property-edit-wide"><el-input v-model="editDraft.rulesText" type="textarea" :rows="2" /></el-form-item>
    </el-form>
    <template #footer>
      <el-button :disabled="savingEdit || reloadingEdit" @click="editVisible = false">取消</el-button>
      <el-button v-if="canReloadEdit" :loading="reloadingEdit" :disabled="savingEdit" @click="reloadEdit">读取最新版本</el-button>
      <el-button type="primary" :loading="savingEdit" :disabled="reloadingEdit || locatingCommunity" @click="saveEdit">{{ submitted && editError ? '重试保存' : '保存修改' }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.property-edit-form{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:0 16px;margin-top:16px}
.property-edit-wide{grid-column:1/-1}
.property-edit-media{display:grid;gap:8px;width:100%}.property-edit-media>div{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
.property-edit-form :deep(.el-input-number){width:100%}

.property-card-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:14px}.property-card{--card-accent:#8793a2;display:flex;aspect-ratio:1;min-width:0;flex-direction:column;overflow:hidden;border:1px solid #dfe7dc;border-top:4px solid var(--card-accent);border-radius:14px;background:#fff;box-shadow:0 2px 8px #203b6008;cursor:pointer;transition:transform .16s ease,box-shadow .16s ease}.property-card:hover{transform:translateY(-2px);box-shadow:0 8px 20px #203b6014}.property-card:focus-visible{outline:3px solid #7b9f73;outline-offset:2px}.property-card.status-available{--card-accent:#358567}.property-card.status-published{--card-accent:#315d82}.property-card.status-draft{--card-accent:#8793a2}.property-card.status-rented{--card-accent:#c86d48}.property-card.status-offline{--card-accent:#8e989e}.property-cover{position:relative;display:grid;flex:0 0 37%;place-items:center;width:100%;min-height:0;overflow:hidden;background:#edf1eb;color:#8290a0}.property-cover img{width:100%;height:100%;object-fit:cover}.cover-fallback{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:5px;background:linear-gradient(145deg,#eff3eb,#e5ebdf);color:#71836e;font-size:12px}.cover-fallback span{max-width:90%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.cover-status{position:absolute;top:9px;left:9px;border:0;background:var(--card-accent);color:#fff;font-size:13px;font-weight:750}.property-room{position:absolute;right:9px;top:9px;padding:4px 7px;border-radius:7px;background:#17243bd9;color:#fff;font-size:11px}.property-copy{display:flex;min-width:0;min-height:0;flex:1;flex-direction:column;align-items:flex-start;gap:5px;padding:9px 11px}.property-name{overflow:hidden;width:100%;color:#25344a;font-size:15px;font-weight:750;text-overflow:ellipsis;white-space:nowrap}.property-community{overflow:hidden;max-width:100%;color:#607166;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.property-tags{display:flex;flex-wrap:wrap;gap:4px;max-height:22px;overflow:hidden}.property-tags span{padding:3px 6px;border-radius:6px;background:#f1f5ef;color:#586a57;font-size:10px;line-height:1.4}.property-copy small{overflow:hidden;width:100%;color:#7b8794;font-size:11px;text-overflow:ellipsis;white-space:nowrap}.property-state-line{display:flex;width:100%;align-items:center;justify-content:space-between;gap:6px;margin-top:auto}.state-readout{display:flex;align-items:center;gap:6px;color:var(--card-accent);font-size:14px;font-weight:800;white-space:nowrap}.state-readout i{width:9px;height:9px;border-radius:50%;background:var(--card-accent);box-shadow:0 0 0 3px color-mix(in srgb,var(--card-accent) 15%,transparent)}.property-rent{overflow:hidden;color:#d76440;font-size:14px;font-weight:800;text-overflow:ellipsis;white-space:nowrap}.property-copy>.el-select{width:100%}.property-copy>.el-select .el-select__wrapper{min-height:28px}.occupancy-key{display:flex;align-items:center;gap:7px;color:#6e7d75;font-size:12px}.occupancy-key i{width:8px;height:8px;margin-left:8px;border-radius:50%;background:#358567}.occupancy-key i:first-child{margin-left:0}.key-rented{background:#c86d48!important}.key-offline{background:#8e989e!important}
.detail-dialog-accessible-title{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0)}:global(.admin-property-detail-dialog .el-dialog__header){position:absolute;z-index:5;top:4px;right:5px;margin:0;padding:0}:global(.admin-property-detail-dialog .el-dialog__headerbtn){width:34px;height:34px;border-radius:50%;background:#fffdf9d9}:global(.admin-property-detail-dialog .el-dialog__body){padding:0!important}.admin-property-canvas{height:min(82vh,850px);min-height:520px;overflow:hidden;border-radius:12px;background:#f6f4ee;color:#29382d}.admin-property-canvas .detail-canvas{height:100%}.admin-property-canvas .detail-canvas-scroll{padding:0 14px 16px}.admin-property-canvas .detail-canvas-cover{width:calc(100% + 28px);height:260px;margin:0 -14px;aspect-ratio:auto;background-position:center;background-size:cover}.admin-property-canvas .detail-canvas-cover video{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}.admin-property-canvas .detail-canvas-media-nav{top:18px;right:16px;left:16px}.admin-property-canvas .detail-canvas-media-nav span{display:grid;width:34px;height:34px;place-items:center;border-radius:50%;background:#29382d88;color:#fff;font-size:28px;line-height:1}.admin-property-canvas .detail-canvas-media-nav i{padding:7px 10px;border-radius:10px;background:#29382d88;font-size:12px}.admin-property-canvas .detail-canvas-empty-media{gap:7px;padding-top:0}.admin-property-canvas .detail-canvas-empty-media b{font-size:15px}.admin-property-canvas .detail-canvas-empty-media small{font-size:11px}.admin-property-canvas .detail-canvas-media-tabs{right:14px;bottom:12px;left:14px;gap:6px;overflow:auto}.admin-property-canvas .detail-canvas-media-tabs button{flex:none;padding:7px 10px;border:1px solid #ffffffc7;border-radius:7px;background:#263629a8;color:#fff;font-size:11px;cursor:pointer}.admin-property-canvas .detail-canvas-media-tabs button.active{background:#40543f;font-weight:800}.admin-property-canvas .detail-canvas-card{margin-top:12px;padding:16px;border-radius:14px}.admin-property-canvas .detail-canvas-price{font-size:29px}.admin-property-canvas .detail-canvas-price small{font-size:13px}.admin-property-canvas .detail-canvas-title{margin-top:9px;font-size:19px}.admin-property-canvas .detail-canvas-address{font-size:12px}.admin-property-canvas .preview-operator-doorplate{display:block;margin-top:7px;color:#647260;font-size:12px}.admin-property-canvas .detail-canvas-status-preview{margin-bottom:8px;font-size:11px}.admin-property-canvas .detail-canvas-facts,.admin-property-canvas .detail-canvas-info>div{gap:8px 18px;margin-top:12px}.admin-property-canvas .detail-canvas-facts i,.admin-property-canvas .detail-canvas-info i{min-height:36px;font-size:13px}.admin-property-canvas .detail-canvas-facts small,.admin-property-canvas .detail-canvas-info small{font-size:10px}.admin-property-canvas .detail-canvas-info>b,.admin-property-canvas .detail-canvas-facilities header b,.admin-property-canvas .detail-canvas-description>b{font-size:15px}.admin-property-canvas .detail-canvas-section-copy,.admin-property-canvas .detail-canvas-facilities header small{font-size:11px!important}.admin-property-canvas .detail-canvas-location header b{font-size:15px}.admin-property-canvas .detail-canvas-location header small{font-size:11px}.admin-property-canvas .detail-canvas-location header>i{font-size:10px}.admin-property-canvas .canvas-detail-map-grid{height:150px}.admin-property-canvas .canvas-detail-map-grid>b{font-size:14px}.admin-property-canvas .canvas-detail-map-grid>small{font-size:10px}.admin-property-canvas .admin-detail-facility-tags{display:flex;flex-wrap:wrap;gap:7px;margin-top:14px}.admin-property-canvas .admin-detail-facility-tags i{padding:7px 10px;border-radius:8px;background:#edf0dc;color:#536a45;font-size:12px;font-style:normal}.admin-property-canvas .detail-canvas-description p{font-size:13px;line-height:1.65;white-space:pre-wrap}.admin-property-canvas .detail-canvas-actions.operator-actions{gap:8px;padding:10px 14px calc(12px + env(safe-area-inset-bottom));background:#fffdf9}.admin-property-canvas .detail-canvas-admin-status{grid-template-columns:repeat(auto-fit,minmax(90px,1fr));grid-auto-flow:initial;gap:7px}.admin-property-canvas .detail-canvas-admin-status button{min-height:38px!important;font-size:12px!important}.admin-property-canvas .admin-detail-return{min-height:34px;border:1px solid #dce5d9;background:#f5f7f2;color:#526452;font-size:12px}.admin-property-canvas .detail-canvas-actions.operator-actions button:first-child{background:#40543f;color:#fff}.admin-property-canvas .detail-canvas-actions.operator-actions .admin-detail-return{background:#f5f7f2;color:#526452}@media(max-width:600px){.property-card-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:9px}.property-copy{padding:7px;gap:4px}.property-name{font-size:12px}.property-community,.property-copy small{font-size:10px}.property-tags span{font-size:9px;padding:2px 4px}.state-readout{font-size:12px}.property-rent{font-size:11px}.cover-status{top:6px;left:6px;font-size:11px}.property-room{top:6px;right:6px;font-size:9px}.occupancy-key{display:none}.admin-property-canvas{min-height:460px;height:78vh}.admin-property-canvas .detail-canvas-cover{height:220px}}
.admin-property-canvas-frame{display:grid;place-items:center;min-height:min(82vh,850px);overflow:hidden;border-radius:12px;background:#e9e8e1}.admin-property-canvas-frame .phone-preview{width:min(402px,37.74vh);height:min(82vh,850px);aspect-ratio:auto;flex:none}.admin-property-canvas-frame .page-mini-content{padding:0;overflow:hidden}.admin-property-canvas-frame .detail-canvas{display:flex;height:100%;flex-direction:column}.admin-property-canvas-frame .detail-canvas-scroll{flex:1;min-height:0;overflow:auto;padding:0 6px 4px}.admin-property-canvas-frame .detail-canvas-cover video{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}.admin-property-canvas-frame .admin-detail-facility-tags{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:7px;margin-top:10px}.admin-property-canvas-frame .admin-detail-facility-tags i{display:grid;justify-items:center;gap:3px;color:#40543f;font-size:7px;font-style:normal;text-align:center}.admin-property-canvas-frame .admin-detail-facility-tags i b{display:grid;width:23px;height:23px;place-items:center;border-radius:7px;background:#edf1e8;color:#40543f;font-size:10px}.admin-property-canvas-frame .detail-canvas-nearby article>div{background-position:center;background-size:cover}.admin-property-canvas-frame .detail-canvas-actions.operator-actions{flex:none}.material-empty-hint{display:grid;justify-items:center;gap:8px;color:#758274;font-size:13px}
</style>

<style scoped>
:global(.admin-property-detail-dialog){padding:0!important;border:0!important;border-radius:0!important;background:transparent!important;box-shadow:none!important}
:global(.admin-property-detail-dialog .el-dialog__body){padding:0!important}
.admin-property-canvas-frame{min-height:0!important;overflow:visible!important;border-radius:0!important;background:transparent!important}
.admin-property-canvas-frame .detail-canvas-cover{display:block;width:calc(100% + 12px);max-width:none;height:auto;min-height:0;aspect-ratio:750 / 720;margin:0 -6px;flex:none;background-position:center;background-size:cover}
.admin-property-canvas-frame .detail-canvas-cover>video{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}
.admin-property-canvas-frame .detail-canvas-media-tabs{flex-wrap:nowrap;white-space:nowrap}
.admin-property-canvas-frame .detail-canvas-media-tabs button{flex:none;white-space:nowrap}
</style>
