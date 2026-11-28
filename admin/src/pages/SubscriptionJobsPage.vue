<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'

type Job = { id: number; kind: string; subjectId: string; result: string; attempts: number; lastError: string }
const props = defineProps<{ api: <T>(path: string, options?: RequestInit) => Promise<T> }>()
const items = ref<Job[]>([])
const loading = ref(false)
const failure = ref('')
const retrying = ref<number>()
const labels: Record<string, string> = { pending: '待发送', retrying: '等待自动重试', failed: '已耗尽重试', completed: '已处理', uncertain: '发送结果待核实', unknown: '历史结果未记录' }
async function load() {
  if (loading.value) return
  loading.value = true
  failure.value = ''
  try {
    items.value = await props.api<Job[]>('/v1/admin/subscription-jobs')
  } catch (error) {
    failure.value = error instanceof Error ? error.message : '通知任务读取失败'
  } finally { loading.value = false }
}
async function retry(job: Job) {
  if (retrying.value) return
  retrying.value = job.id
  try {
    await props.api(`/v1/admin/subscription-jobs/${job.id}/retry`, { method: 'POST' })
    ElMessage.success('已加入重试队列')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加入重试队列失败')
  } finally {
    retrying.value = undefined
    await load()
  }
}
onMounted(load)
</script>

<template>
  <section class="page-header"><div><span>内容与运营</span><h1>通知任务</h1><p>显示最近任务，优先列出发送失败的记录。</p></div><el-button :loading="loading" @click="load">刷新数据</el-button></section>
  <section class="surface table-panel" v-loading="loading">
    <el-alert v-if="failure" :title="failure" type="error" :closable="false" />
    <el-table :data="items" empty-text="暂无通知任务">
      <el-table-column prop="id" label="任务" width="90" />
      <el-table-column label="类型" width="140"><template #default="{ row }">{{ row.kind === 'request' ? '客户需求提醒' : '片区上新提醒' }}</template></el-table-column>
      <el-table-column prop="subjectId" label="关联记录" min-width="190" />
      <el-table-column label="状态" min-width="145"><template #default="{ row }">{{ labels[row.result] || row.result }}</template></el-table-column>
      <el-table-column prop="attempts" label="失败次数" width="100" />
      <el-table-column prop="lastError" label="最近失败原因" min-width="210" />
      <el-table-column label="操作" width="110"><template #default="{ row }"><el-button v-if="row.result === 'failed'" link type="primary" :disabled="!!retrying || loading" @click="retry(row)">重新发送</el-button></template></el-table-column>
    </el-table>
  </section>
</template>
