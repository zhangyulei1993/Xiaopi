<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

type Doorplate = { community: string; building: string; unit: string; room: string }
const props = defineProps<{ api: <T>(path: string, options?: RequestInit) => Promise<T> }>()
const path = '/v1/admin/doorplates'
const counts = ref<Record<string, number>>({})
const selected = ref('')
const rows = ref<Doorplate[]>([])
const pending = ref<Doorplate[]>([])
const importNames = ref<Record<string, string>>({})
const sourceNames = computed(() => [...new Set(pending.value.map(row => row.community))])
const normalizeCommunityName = (value: string) => value.trim().toLocaleLowerCase().replace(/[^\p{L}\p{N}]/gu, '')
const suggestedImportName = (source: string) => {
  const key = normalizeCommunityName(source)
  const known = [...Object.keys(counts.value), ...Object.keys(locations.value), ...Object.keys(plans.value)]
  return known.find(name => normalizeCommunityName(name) === key) || source.trim()
}
const error = ref('')
const busy = ref(false)

type Status = 'pending' | 'active' | 'done'
type Plan = { community: string; status: Status; due: string; note: string }
const plansPath = '/v1/admin/doorplate-plans'
const plans = ref<Record<string, Plan>>({})
const stages: { key: Status; label: string }[] = [
  { key: 'pending', label: '待采集' },
  { key: 'active', label: '正在采集' },
  { key: 'done', label: '已完成' },
]
const tab = ref<Status | 'catalog'>('catalog')
const editing = ref<Plan | null>(null)
const existingPlan = ref(false)
const saving = ref(false)
const planError = ref('')
const search = ref('')
const coverage = computed(() => Object.entries(counts.value).sort((a, b) => b[1] - a[1]))
const total = computed(() => coverage.value.reduce((sum, [, count]) => sum + count, 0))
const stageCounts = computed(() => Object.fromEntries(stages.map(stage => [
  stage.key, Object.values(plans.value).filter(plan => plan.status === stage.key).length,
])))
const visiblePlans = computed(() => Object.values(plans.value)
  .filter(plan => plan.status === tab.value && plan.community.includes(search.value.trim()))
  .sort((a, b) => (a.due || '9999').localeCompare(b.due || '9999')))
const visibleCoverage = computed(() => coverage.value.filter(([name]) => name.includes(search.value.trim())))
const roomSearch = ref('')
const building = ref('')
const unit = ref('')
const buildings = computed(() => {
  const groups = new Map<string, Map<string, string[]>>()
  for (const row of rows.value) {
    if (!groups.has(row.building)) groups.set(row.building, new Map())
    const units = groups.get(row.building)!
    if (!units.has(row.unit)) units.set(row.unit, [])
    units.get(row.unit)!.push(row.room)
  }
  return groups
})
const units = computed(() => buildings.value.get(building.value) ?? new Map<string, string[]>())
const originalRooms = computed(() => units.value.get(unit.value) ?? [])
const rooms = computed(() => originalRooms.value.filter(room => room.includes(roomSearch.value.trim())))
const floors = computed(() => {
  const groups = new Map<number, string[]>()
  for (const room of rooms.value) {
    if (!/^[1-9]\d{2,3}$/.test(room) || room.endsWith('00')) continue
    const floor = Number(room.slice(0, -2))
    if (!groups.has(floor)) groups.set(floor, [])
    groups.get(floor)!.push(room)
  }
  return [...groups.entries()].sort(([a], [b]) => b - a)
})
function selectBuilding(name: string) {
  building.value = name
  unit.value = units.value.keys().next().value ?? ''
  roomSearch.value = ''
}

function selectTab(value: typeof tab.value) {
  search.value = ''
  tab.value = value
}
function edit(plan?: Plan, community = '') {
  existingPlan.value = Boolean(plan)
  planError.value = ''
  editing.value = plan ? { ...plan } : { community, status: 'pending', due: '', note: '' }
}
async function savePlan() {
  if (!editing.value || saving.value) return
  const plan = { ...editing.value, community: editing.value.community.trim() }
  if (!plan.community) { planError.value = '请输入小区名称'; return }
  if (!existingPlan.value && Object.prototype.hasOwnProperty.call(plans.value, plan.community)) {
    planError.value = '该小区已有采集计划，请在对应列表编辑'
    return
  }
  saving.value = true
  planError.value = ''
  try {
    plans.value = await props.api(plansPath, { method: 'PUT', body: JSON.stringify({ ...plan, due: plan.due || '' }) })
    editing.value = null
    selectTab(plan.status)
  } catch (e) {
    planError.value = (e as Error).message
  } finally {
    saving.value = false
  }
}

type CommunityLocation = { address: string; latitude: number; longitude: number }
const locationPath = '/v1/admin/community-locations'
const locations = ref<Record<string, CommunityLocation>>({})
const locationEditor = ref<{ community: string; address: string; latitude: string; longitude: string } | null>(null)
const locationSaving = ref(false)
const locationError = ref('')

function editLocation() {
  const location = locations.value[selected.value]
  locationError.value = ''
  locationEditor.value = {
    community: selected.value,
    address: location?.address ?? '',
    latitude: location ? String(location.latitude) : '',
    longitude: location ? String(location.longitude) : '',
  }
}
async function saveLocation() {
  if (!locationEditor.value || locationSaving.value) return
  const { community, address, latitude, longitude } = locationEditor.value
  locationSaving.value = true
  locationError.value = ''
  try {
    locations.value = await props.api(locationPath + '?community=' + encodeURIComponent(community), {
      method: 'PUT',
      body: JSON.stringify({ address, latitude: Number(latitude), longitude: Number(longitude) }),
    })
    locationEditor.value = null
  } catch (e) {
    locationError.value = (e as Error).message
  } finally {
    locationSaving.value = false
  }
}

async function load() {
  try {
    [counts.value, plans.value, locations.value] = await Promise.all([props.api<Record<string, number>>(path), props.api<Record<string, Plan>>(plansPath), props.api<Record<string, CommunityLocation>>(locationPath)])
    const first = coverage.value[0]?.[0]
    if (first && !selected.value) await show(first)
  } catch (e) {
    error.value = (e as Error).message
  }
}
async function show(community: string) {
  roomSearch.value = ''
  selected.value = community
  rows.value = []
  selectBuilding('')
  try {
    const result = await props.api<Doorplate[]>(path + '?community=' + encodeURIComponent(community))
    if (selected.value === community) {
      rows.value = result
      selectBuilding(buildings.value.keys().next().value ?? '')
    }
  } catch (e) {
    error.value = (e as Error).message
  }
}
async function read(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  error.value = ''
  pending.value = []
  importNames.value = Object.create(null)
  try {
    const data = JSON.parse(await file.text())
    if (!Array.isArray(data.records) || !data.records.length) throw new Error('请选择包含 records 的门牌导出 JSON')
    pending.value = data.records.map(({ community, building, unit, room }: Doorplate) => {
      if (![community, building, room].every(value => typeof value === 'string' && value.trim()) || typeof unit !== 'string') {
        throw new Error('存在格式错误的小区或门牌')
      }
      return { community, building, unit, room }
    })
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    input.value = ''
  }
}
async function importRows() {
  busy.value = true
  error.value = ''
  try {
    counts.value = await props.api(path, { method: 'POST', body: JSON.stringify(pending.value.map(row => ({ ...row, community: importNames.value[row.community]?.trim() || suggestedImportName(row.community) }))) })
    pending.value = []
    if (selected.value) await show(selected.value)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>

<template>
  <section class="page-header">
    <div><span>业务管理</span><h1>小区门牌数据</h1><p>查看已收录的门牌，安排下一批采集。</p></div>
    <el-button type="primary" @click="edit()">新增采集计划</el-button>
  </section>
  <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />
  <div class="stats-grid dashboard-grid doorplate-stats">
    <button class="surface stat-card" @click="selectTab('catalog')"><span>已收录小区</span><strong>{{ coverage.length }}</strong><small>{{ total.toLocaleString() }} 条门牌</small></button>
    <button v-for="stage in stages" :key="stage.key" class="surface stat-card" @click="selectTab(stage.key)"><span>{{ stage.label }}</span><strong>{{ stageCounts[stage.key] }}</strong><small>采集计划</small></button>
  </div>
  <section class="surface form-section doorplate-workspace">
    <div class="doorplate-toolbar">
      <div class="doorplate-tabs" aria-label="门牌数据视图">
        <el-button :type="tab === 'catalog' ? 'primary' : 'default'" :aria-pressed="tab === 'catalog'" @click="selectTab('catalog')">门牌库</el-button>
        <el-button v-for="stage in stages" :key="stage.key" :type="tab === stage.key ? 'primary' : 'default'" :aria-pressed="tab === stage.key" @click="selectTab(stage.key)">{{ stage.label }}</el-button>
      </div>
      <el-input v-model="search" clearable aria-label="搜索小区" placeholder="搜索小区" />
    </div>
    <template v-if="tab === 'catalog'">
      <div class="doorplate-toolbar">
        <h2>门牌分布</h2>
        <div class="doorplate-import">
          <label class="el-button"><input type="file" accept=".json" :disabled="busy" @change="read">选择 JSON 文件</label>
          <el-button v-if="pending.length" type="primary" :loading="busy" @click="importRows">导入 {{ pending.length }} 条门牌</el-button>
        </div>
      </div>
      <div v-if="pending.length" class="doorplate-import-review">
        <p>系统会忽略标点、空格等名称差异并匹配已有小区；请核对映射，必要时可手动指定标准名称。</p>
        <label v-for="name in sourceNames" :key="name"><span>{{ name }}</span><el-input v-model="importNames[name]" :aria-label="name + '的录入名称'" :placeholder="suggestedImportName(name)" :disabled="busy" /><small>将导入到：{{ importNames[name]?.trim() || suggestedImportName(name) }}</small></label>
      </div>
      <div class="doorplate-split">
      <aside class="doorplate-catalog"><h3>小区列表</h3>
      <div class="doorplate-bars">
        <button v-for="[name, count] in visibleCoverage" :key="name" type="button" :class="{ selected: selected === name }" @click="show(name)">
          <span>{{ name }}</span><span class="bar-track"><i :style="{ width: count / coverage[0][1] * 100 + '%' }"></i></span><strong>{{ count.toLocaleString() }}</strong>
        </button>
      </div>
      <el-empty v-if="!visibleCoverage.length" description="暂无匹配的门牌记录" />
      </aside>
      <div class="doorplate-detail">
      <template v-if="selected">
        <div class="doorplate-toolbar">
          <h2>{{ selected }}</h2>
          <el-button @click="editLocation">{{ locations[selected] ? '编辑位置' : '补齐位置' }}</el-button>
          <span class="detail-count">{{ buildings.size }} 栋 · {{ rows.length.toLocaleString() }} 条门牌</span>
          <el-input v-model="roomSearch" clearable aria-label="筛选当前单元房号" placeholder="筛选当前单元房号" />
          <el-button @click="edit(plans[selected], selected)">{{ plans[selected] ? '编辑采集计划' : '安排补采' }}</el-button>
        </div>
        <div class="doorplate-columns">
          <section class="doorplate-column">
            <header><div><h3>楼栋</h3><small>选择楼栋查看单元</small></div><b>{{ buildings.size }}</b></header>
            <div class="column-scroll">
              <button v-for="[name, entries] in buildings" :key="name" type="button" :aria-pressed="building === name" @click="selectBuilding(name)">
                <strong>{{ name }}</strong><span>{{ entries.has('') ? (entries.size > 1 ? (entries.size - 1) + ' 单元 · 含无单元' : '无单元') : entries.size + ' 单元' }} ›</span>
              </button>
            </div>
          </section>
          <section class="doorplate-column">
            <header><div><h3>单元</h3><small>{{ building || '请先选择楼栋' }}</small></div><b>{{ units.size }}</b></header>
            <div class="column-scroll">
              <button v-for="[name, entries] in units" :key="name" type="button" :aria-pressed="unit === name" @click="unit = name; roomSearch = ''">
                <strong>{{ name || '无单元' }}</strong><span>{{ entries.length }} 门牌 ›</span>
              </button>
            </div>
          </section>
          <section class="doorplate-column">
            <header><div><h3>门牌与楼层</h3><small>{{ building ? [building, unit || '无单元'].join(' · ') : '请先选择楼栋' }}</small></div><b>{{ originalRooms.length }}</b></header>
            <div class="column-scroll room-column">
              <p v-if="floors.length" class="floor-note">楼层按房号推算，待核实。原始房号见下方。</p>
              <p v-else class="floor-note">暂无可按编号归组的楼层。</p>
              <div v-for="[floor, entries] in floors" :key="floor" class="floor-row">
                <strong>{{ floor }} 层</strong><div class="room-chips"><span v-for="room in entries" :key="room">{{ room }}</span></div>
              </div>
              <details open class="raw-rooms"><summary>{{ roomSearch.trim() ? '匹配的原始门牌' : '全部原始门牌' }} · {{ rooms.length }} 项</summary>
                <div class="room-chips"><span v-for="room in rooms" :key="room">{{ room }}</span></div>
                <p v-if="!rooms.length">暂无匹配的门牌</p>
              </details>
            </div>
          </section>
        </div>
      </template>
      <el-empty v-else description="选择左侧小区查看门牌" />
      </div>
      </div>
    </template>
    <el-table v-else :data="visiblePlans" empty-text="暂无匹配的采集计划">
      <el-table-column prop="community" label="小区" min-width="135" />
      <el-table-column label="已收录门牌" width="120"><template #default="{ row }">{{ counts[row.community] || 0 }} 条</template></el-table-column>
      <el-table-column label="计划日期" width="125"><template #default="{ row }">{{ row.due || '待定' }}</template></el-table-column>
      <el-table-column prop="note" label="采集范围或问题" min-width="160" show-overflow-tooltip />
      <el-table-column label="操作" width="145"><template #default="{ row }"><el-button link type="primary" @click="edit(row)">编辑</el-button><el-button v-if="counts[row.community]" link type="primary" @click="selectTab('catalog'); show(row.community)">查看门牌</el-button></template></el-table-column>
    </el-table>
  </section>
  <el-dialog :model-value="!!locationEditor" title="小区位置" width="460px" :show-close="!locationSaving" :close-on-click-modal="!locationSaving" :close-on-press-escape="!locationSaving" @close="locationEditor = null">
    <el-form v-if="locationEditor" label-position="top" :disabled="locationSaving">
      <p>{{ locationEditor.community }} · 后续录入该小区时自动带入</p>
      <el-alert v-if="locationError" :title="locationError" type="error" :closable="false" />
      <el-form-item label="小区地址"><el-input v-model="locationEditor.address" maxlength="60" /></el-form-item>
      <el-form-item label="纬度（GCJ-02）"><el-input v-model="locationEditor.latitude" /></el-form-item>
      <el-form-item label="经度（GCJ-02）"><el-input v-model="locationEditor.longitude" /></el-form-item>
    </el-form>
    <template #footer><el-button :disabled="locationSaving" @click="locationEditor = null">取消</el-button><el-button type="primary" :loading="locationSaving" @click="saveLocation">保存位置</el-button></template>
  </el-dialog>
  <el-dialog :model-value="!!editing" :title="existingPlan ? '编辑采集计划' : '新增采集计划'" width="460px" :show-close="!saving" :close-on-click-modal="!saving" :close-on-press-escape="!saving" @close="editing = null">
    <el-form v-if="editing" label-position="top" @submit.prevent="savePlan">
      <el-alert v-if="planError" :title="planError" type="error" :closable="false" />
      <el-form-item label="小区名称"><el-input v-model="editing.community" maxlength="100" :disabled="existingPlan || saving" /></el-form-item>
      <el-form-item label="采集状态"><el-select v-model="editing.status" :disabled="saving"><el-option v-for="stage in stages" :key="stage.key" :label="stage.label" :value="stage.key" /></el-select></el-form-item>
      <el-form-item label="计划日期"><el-date-picker v-model="editing.due" type="date" value-format="YYYY-MM-DD" :value-on-clear="''" :disabled="saving" /></el-form-item>
      <el-form-item label="采集范围或问题"><el-input v-model="editing.note" type="textarea" :rows="3" maxlength="500" :disabled="saving" placeholder="如：补采三期单元，核对 5 栋房号" /></el-form-item>
    </el-form>
    <template #footer><el-button :disabled="saving" @click="editing = null">取消</el-button><el-button type="primary" :loading="saving" @click="savePlan">保存计划</el-button></template>
  </el-dialog>
</template>

<style scoped>
.doorplate-stats { grid-template-columns: repeat(auto-fit, minmax(145px, 1fr)); margin-bottom: 16px; }
.doorplate-workspace { display: grid; gap: 16px; }
.doorplate-toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; }
.doorplate-toolbar h2 { font-size: 17px; margin: 0 auto 0 0; }
.doorplate-toolbar > .el-input { width: min(220px, 100%); }
.doorplate-tabs { display:flex; flex-wrap:wrap; gap:6px; margin-right:auto; }
.doorplate-tabs .el-button { margin:0; }
.doorplate-workspace button, .doorplate-import label { font-family:inherit; }
.doorplate-bars button, .column-scroll button { font-size:14px; }
.column-scroll button span, .doorplate-column small { font-size:12px; }
.doorplate-column h3, .doorplate-catalog h3, .doorplate-toolbar h2 { color:var(--color-primary); }
.doorplate-import { display: flex; align-items: center; gap: 8px; }
.doorplate-import label { position: relative; overflow: hidden; }
.doorplate-import input { position: absolute; inset: 0; opacity: 0; cursor: pointer; }
.doorplate-import-review { background: var(--el-fill-color-light); border-radius: 12px; padding: 12px; }
.doorplate-import-review p { margin: 0 0 10px; }
.doorplate-import-review label { display: grid; grid-template-columns: minmax(100px, 1fr) 2fr; align-items: center; gap: 8px; margin-top: 6px; }
.doorplate-bars { display: grid; gap: 4px; }
.doorplate-bars button { display: grid; grid-template-columns: minmax(104px, 1fr) minmax(50px, 4fr) 55px; gap: 12px; align-items: center; padding: 8px 10px; background: transparent; border: 1px solid transparent; border-radius: 8px; text-align: left; color: inherit; cursor: pointer; }
.doorplate-bars button:hover, .doorplate-bars button.selected { background: var(--el-color-primary-light-9); border-color: var(--el-color-primary-light-8); }
.bar-track { height: 8px; background: var(--el-border-color-light); border-radius: 8px; overflow: hidden; }
.bar-track i { display: block; height: 100%; background: var(--color-primary); border-radius: 8px; }
.doorplate-bars strong { text-align: right; font-variant-numeric: tabular-nums; }
 .doorplate-columns { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 2fr); gap: 12px; }
.doorplate-column { border: 1px solid var(--el-border-color); border-radius: 12px; min-width: 0; overflow: hidden; }
.doorplate-column header { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-height: 62px; padding: 10px 14px; background: var(--el-fill-color-light); border-bottom: 1px solid var(--el-border-color-light); }
.doorplate-column h3 { margin: 0 0 4px; font-size: 15px; }
.doorplate-column small, .column-scroll button span { color: var(--color-text-secondary); }
.doorplate-column header b { background: var(--el-color-primary-light-9); color: var(--color-primary); border-radius: 8px; padding: 4px 8px; }
.column-scroll { height: min(48vh, 440px); overflow: auto; padding: 6px; }
.column-scroll button { display: flex; width: 100%; justify-content: space-between; align-items: center; gap: 6px; padding: 10px 8px; background: transparent; border: 1px solid transparent; border-radius: 8px; color: inherit; text-align: left; cursor: pointer; }
.column-scroll button:hover, .column-scroll button[aria-pressed="true"] { background: var(--el-color-primary-light-9); border-color: var(--el-color-primary-light-8); color: var(--color-primary); }
.room-column { padding: 10px 14px; }
.floor-note { margin: 0 0 8px; padding: 8px; border-radius: 8px; background: var(--color-heritage-green-soft); color: var(--color-heritage-green); }
.floor-row { display: grid; grid-template-columns: 48px 1fr; align-items: start; gap: 8px; padding: 6px 0; }
.room-chips { display: flex; flex-wrap: wrap; gap: 4px; }
.room-chips span { background: var(--el-fill-color-light); border: 1px solid var(--el-border-color); border-radius: 6px; padding: 3px 7px; }
.raw-rooms { border-top: 1px solid var(--el-border-color-light); margin-top: 10px; padding-top: 10px; }
.raw-rooms summary { cursor: pointer; margin-bottom: 8px; }
@media (max-width: 780px) {
  .doorplate-columns { grid-template-columns: 1fr 1fr; }
  .doorplate-column:last-child { grid-column: 1 / -1; }
  .column-scroll { max-height: 260px; }
}
.doorplate-split { display:grid; grid-template-columns: minmax(180px, 21%) minmax(0, 1fr); gap:16px; align-items:start; }
.doorplate-catalog { min-width:0; padding-right:12px; border-right:1px solid var(--el-border-color-light); }
.doorplate-catalog h3 { margin:0 0 8px; font-size:15px; }
.doorplate-catalog .doorplate-bars { max-height: min(57vh, 500px); overflow:auto; }
.doorplate-catalog .doorplate-bars button { grid-template-columns:minmax(0,1fr) auto; gap:4px 8px; }
.doorplate-catalog .bar-track { grid-row:2; grid-column:1 / -1; height:4px; }
.doorplate-catalog .doorplate-bars strong { grid-row:1; grid-column:2; }
.doorplate-detail { min-width:0; }
.doorplate-detail > .doorplate-toolbar { margin-bottom:10px; }
.detail-count { color:var(--color-text-secondary); font-size:12px; }
.doorplate-stats .stat-card { min-height:0; padding-block:10px; }
@media (max-width: 960px) {
  .doorplate-split { grid-template-columns: 1fr; }
  .doorplate-catalog { padding-right:0; border-right:0; }
  .doorplate-catalog .doorplate-bars { max-height:130px; }
}
</style>
