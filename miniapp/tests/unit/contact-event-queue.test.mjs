import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

const source = readFileSync(new URL('../../src/utils/contact-event-queue.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText

function fixture() {
  const storage = new Map()
  const reports = []
  let session
  let report = async () => {}
  class ApiError extends Error {
    constructor(statusCode) { super(); this.statusCode = statusCode }
  }
  const exports = {}
  const modules = {
    '../api/client': { ApiError },
    '../api/properties': { recordContactEvent: event => { reports.push([session.accessToken, event.propertyId]); return report() } },
    '../auth/session': { getSession: () => session },
  }
  vm.runInNewContext(compiled, {
    exports, require: path => modules[path],
    uni: {
      getStorageSync: key => storage.get(key),
      setStorageSync: (key, value) => storage.set(key, value),
      removeStorageSync: key => storage.delete(key),
      base64ToArrayBuffer: value => Uint8Array.from(Buffer.from(value, 'base64')).buffer,
    },
  })
  const logIn = (sub, signature = 'signature') => {
    const accessToken = Buffer.from(JSON.stringify({ sub })).toString('base64url') + '.' + signature
    session = { accessToken, role: 'user' }
    return accessToken
  }
  return { ...exports, storage, reports, logIn, setReport: handler => { report = handler }, ApiError }
}

test('failed clicks stay with the original account across account and token changes', async () => {
  const f = fixture()
  f.logIn('account-a')
  let rejectReport
  f.setReport(() => new Promise((_, reject) => { rejectReport = reject }))
  f.recordOrQueueContact({ propertyId: 'property-a', type: 'consult_entry_click' })
  f.logIn('account-b')
  rejectReport(new f.ApiError(0))
  await Promise.resolve()
  f.setReport(async () => {})
  await f.flushPendingContactEvents()
  assert.equal(f.reports.length, 1)
  const refreshedToken = f.logIn('account-a', 'refreshed')
  await f.flushPendingContactEvents()
  assert.deepEqual(f.reports[1], [refreshedToken, 'property-a'])
  assert.equal(f.storage.size, 0)
})

test('unowned legacy clicks are discarded and account changes stop a running flush', async () => {
  const f = fixture()
  const baseKey = 'xiaopi_pending_contact_events'
  const event = propertyId => ({ propertyId, type: 'phone_entry_click' })
  f.storage.set(baseKey, [event('legacy')])
  f.storage.set(baseKey + ':account-a', [event('first'), event('second')])
  f.logIn('account-a')
  f.setReport(async () => { f.logIn('account-b') })
  await f.flushPendingContactEvents()
  assert.deepEqual(f.reports.map(([, propertyId]) => propertyId), ['first'])
  assert.equal(f.storage.has(baseKey), false)
  assert.equal(f.storage.get(baseKey + ':account-a')[0].propertyId, 'second')
})
