import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

const script = fileURLToPath(new URL('../impact.mjs', import.meta.url))
const plan = (file, edit) => JSON.parse(execFileSync(process.execPath,
  [script, '--plan', '--file=miniapp/' + file, '--example=' + edit]))

test('home style schedules only the home business and visual checks', () => {
  const result = plan('src/pages/home/index.vue', '+.hero { color: red }')
  assert.deepEqual(result.business, ['home-load'])
  assert.equal(result.visual, 'home baseline')
})
test('favorite logic includes the favorite flow and identifies the API', () => {
  const result = plan('src/pages/properties/index.vue', '+setFavorite(id)')
  assert.ok(result.business.includes('favorite-add-remove'))
  assert.ok(result.reasons[0].api.setFavorite)
})
test('request wrapper propagates indirectly from API exports to pages', () => {
  const result = plan('src/api/client.ts', '+request(url)')
  assert.equal(result.scope, 'full')
  assert.ok(result.reasons[0].pages.includes('requests/history'))
})
test('appointment sheet hosted on detail reaches related appointment flows', () => {
  const result = plan('src/pages/properties/detail.vue', '+submitAppointment()')
  assert.ok(result.business.includes('appointment-submit'))
  assert.ok(result.business.includes('appointment-property-association'))
})
test('unmapped files and role changes expand coverage', () => {
  assert.equal(plan('src/components/unmapped.vue', '+unknown()').scope, 'full')
  assert.equal(plan('src/pages/home/index.vue', '+getSession()').scope, 'full')
})
