import test from 'node:test'
import assert from 'node:assert/strict'
import { auditSource } from '../../scripts/audit-network-source.mjs'

test('recognizes guarded request inside TS assertion', () => {
  const result = auditSource('client.ts', 'uni.request({ ...opts, url: safeRequestUrl(input, base, true) } as UniApp.RequestOptions)')
  assert.equal(result.length, 1)
  assert.equal(result[0].directGuard, true)
  assert.equal(result[0].status, 'BLOCKED')
})
test('later spread can override guarded URL', () => {
  const result = auditSource('client.ts', 'uni.request({ url: safeRequestUrl(input, base, true), ...opts })')
  assert.equal(result[0].directGuard, false)
})
test('raw upload URL is not treated as guarded', () => {
  const result = auditSource('page.vue', '<script setup lang="ts">uni.uploadFile({ url: userInput })</script>')
  assert.equal(result[0].directGuard, false)
})
test('same-name guard remains blocked without proven binding', () => {
  const result = auditSource('page.ts', 'function safeUploadUrl(x: string) { return x }; uni.uploadFile({url:safeUploadUrl(userInput)})')
  assert.equal(result[0].status, 'BLOCKED')
})

test('real project request and uploads resolve their expected declarations', async () => {
  const { auditTree } = await import('../../scripts/audit-network-source.mjs')
  const calls = auditTree('src')
  assert.equal(calls.length, 6)
  const request = calls.find(c => c.callee === 'uni.request')
  assert.equal(request.directGuard, true)
  assert.equal(request.importAlias, true)
  assert.equal(request.originVerified, true)
  assert.match(request.resolvedDeclarations[0].file, /src\/api\/request-url\.(?:d\.mts|mjs)$/)
  const uploads = calls.filter(c => c.callee === 'uni.uploadFile')
  assert.equal(uploads.length, 4)
  for (const upload of uploads) {
    assert.equal(upload.directGuard, true)
    assert.equal(upload.importAlias, true)
    assert.equal(upload.originVerified, true)
    assert.match(upload.resolvedDeclarations[0].file, /src\/api\/client\.ts$/)
    assert.equal(upload.wrapperBinding.directReturn, true)
    assert.equal(upload.wrapperBinding.importAlias, true)
    assert.match(upload.wrapperBinding.declarations[0].file, /src\/api\/upload-url\.(?:d\.mts|mjs)$/)
  }
  const [download] = calls.filter(c => c.callee === 'uni.downloadFile')
  assert.ok(download)
  assert.equal(download.directGuard, true)
  assert.equal(download.originVerified, true)
  assert.match(download.wrapperBinding.declarations[0].file, /src\/api\/download-url\.(?:d\.mts|mjs)$/)
})

test('guard reassignment is detected by symbol identity', () => {
  const [result] = auditSource('page.ts', 'let safeUploadUrl=(x: string)=>x; safeUploadUrl = value => value; uni.uploadFile({url:safeUploadUrl(input)})')
  assert.equal(result.reassigned, true)
  assert.equal(result.status, 'BLOCKED')
})
test('shadowed variable does not count as a write to the called symbol', () => {
  const [result] = auditSource('page.ts', 'const safeUploadUrl=(x: string)=>x; { let safeUploadUrl = 1; safeUploadUrl = 2 }; uni.uploadFile({url:safeUploadUrl(input)})')
  assert.equal(result.reassigned, false)
  assert.equal(result.status, 'BLOCKED')
})
