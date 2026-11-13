import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { validateUploadUrl } from '../../src/api/upload-url.mjs'
const origin = 'http://127.0.0.1:18083'
test('test origin allowed', () => assert.equal(validateUploadUrl(`${origin}/api/v1/auth/avatar`, true, origin), `${origin}/api/v1/auth/avatar`))
for (const value of ['https://api.xiaopizufang.cn/api/v1/auth/avatar', 'https://unknown.invalid/api/v1/auth/avatar', 'http://127.0.0.1:9999/api/v1/auth/avatar', 'http://127.0.0.1:18083.evil.invalid/api/a']) test(`reject ${value}`, () => assert.throws(() => validateUploadUrl(value, true, origin)))
test('wrong configured origin rejected', () => assert.throws(() => validateUploadUrl('https://api.xiaopizufang.cn/api/a', true, 'https://api.xiaopizufang.cn')))
test('production unchanged', () => assert.equal(validateUploadUrl('https://api.xiaopizufang.cn/api/a', false, ''), 'https://api.xiaopizufang.cn/api/a'))
test('three upload sites use validator', () => { for (const file of ['src/pages/me/index.vue', 'src/pages/auth/login.vue', 'src/pages/admin/property-edit.vue']) { const text = fs.readFileSync(new URL(`../../${file}`, import.meta.url), 'utf8'); assert.match(text, /uni\.uploadFile\(\{\s*url:\s*safeUploadUrl\(/) } })
