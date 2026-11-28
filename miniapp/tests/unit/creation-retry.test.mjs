import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

function compile(source, context) {
  const exports = {}
  const code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  vm.runInNewContext(code, { exports, ...context })
  return exports
}
const ref = value => ({ value })

test('lost new-listing response retries the original task then applies edited draft fields', async () => {
  const page = readFileSync(new URL('../../src/pages/admin/property-edit.vue', import.meta.url), 'utf8')
  const source = page.slice(page.indexOf('let createTaskKey ='), page.indexOf('type SaveMode =')).replace('const createDraftOnce =', 'export const createDraftOnce =')
  const calls = [], updates = [], id = ref('')
  const { createDraftOnce } = compile(source, {
    id,
    createProperty: async payload => {
      calls.push(JSON.parse(JSON.stringify(payload)))
      if (calls.length === 1) throw new Error('response lost after commit')
      return { id: 'one-draft', ...payload }
    },
    updateProperty: async payload => { updates.push(payload); return payload },
  })
  await assert.rejects(createDraftOnce({ title: '小区甲', rentMonthly: 1000 }))
  const saved = await createDraftOnce({ title: '小区甲', rentMonthly: 1200 })
  assert.deepEqual(calls[0], calls[1])
  assert.equal(saved.id, 'one-draft')
  assert.equal(saved.rentMonthly, 1200)
  assert.equal(id.value, 'one-draft')
  assert.equal(updates.length, 1)
})

test('material retry keeps the submitted snapshot and resumes media without creating another item', async () => {
  const page = readFileSync(new URL('../../src/pages/admin/material-library.vue', import.meta.url), 'utf8')
  const source = 'let createKey = ""; export ' + page.slice(page.indexOf('const saveMaterial = async'), page.indexOf('\nconst ', page.indexOf('const saveMaterial = async') + 10))
  const calls = [], uploadCalls = [], snapshot = ref(null), draftId = ref(''), saving = ref(false)
  const refs = {
    saving, draftId, createSnapshot: snapshot, category: ref('小区甲'),
    pendingMedia: ref([{ path: '/image.jpg', type: 'image' }]),
    rentType: ref('整租'), materialLayoutText: ref('2室1厅'), sharedRoomType: ref('主卧'),
    rentMonthly: ref('1000'), materialFloor: ref('2'), materialRoomCount: ref(2),
    materialRoomSequence: ref(0), materialAmenities: ref(['空调']),
    selectedCommunityLocation: ref({ address: '甲地址', latitude: 30, longitude: 114 }),
    uploadedMedia: ref([]), uploadProgress: ref(''), activeCommunity: ref('全部'), formOpen: ref(true),
  }
  const { saveMaterial } = compile(source, {
    ...refs, mediaFileMatches: () => true, refreshCurrentSession: async () => {},
    createMaterial: async payload => {
      calls.push(JSON.parse(JSON.stringify(payload)))
      if (calls.length === 1) throw new Error('response lost')
      return { id: 'material-one' }
    },
    upload: async (...args) => { uploadCalls.push(args); return { id: 'media-one' } },
    replacePropertyMedia: async () => {}, refresh: async () => {}, resetForm: () => {},
    uni: { showToast() {}, showModal() {} },
  })
  await saveMaterial()
  assert.equal(draftId.value, '')
  await saveMaterial()
  assert.deepEqual(calls[0], calls[1])
  assert.equal(draftId.value, 'material-one')
  assert.equal(uploadCalls.length, 1)
  assert.equal(saving.value, false)
  // Incomplete submissions show the original fields but prevent edits until resolved.
  assert.match(page, /class="material-basic" :class="\{ 'entry-locked': saving \|\| !!createSnapshot \}"/)
  assert.match(page, /<input :disabled="saving \|\| !!createSnapshot" v-model="rentMonthly"/)
})
