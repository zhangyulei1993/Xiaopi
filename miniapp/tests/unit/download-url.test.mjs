import test from 'node:test'
import assert from 'node:assert/strict'
import { validateDownloadUrl } from '../../src/api/download-url.mjs'

const api = 'http://127.0.0.1:18080'

test('allows media served by the configured API origin', () => {
  assert.equal(validateDownloadUrl(`${api}/uploads/video.mp4`, api), `${api}/uploads/video.mp4`)
})

test('allows the production COS media host only over HTTPS', () => {
  const url = 'https://cos.xiaopizufang.cn/media/video.mp4'
  assert.equal(validateDownloadUrl(url, api), url)
  assert.throws(() => validateDownloadUrl('http://cos.xiaopizufang.cn/media/video.mp4', api), /not allowed/)
})

test('rejects unrelated and protocol-relative media URLs', () => {
  assert.throws(() => validateDownloadUrl('https://attacker.example/video.mp4', api), /not allowed/)
  assert.throws(() => validateDownloadUrl('//attacker.example/video.mp4', api), /invalid/)
})
