import { test } from 'node:test'
import assert from 'node:assert/strict'
import { safeRequestUrl } from '../../src/api/request-url.mjs'
const base='http://127.0.0.1:18083/api'
for (const [name,input] of [['relative','/v1/properties'],['absolute',`${base}/v1/properties`]]) test(name,()=>assert.equal(safeRequestUrl(input,base,true),`${base}/v1/properties`))
for (const input of ['https://api.xiaopizufang.cn/api/v1/x','https://unknown.invalid/api/v1/x','http://127.0.0.1:18084/api/v1/x','//evil/x','not-a-url','/../../outside','ftp://example.com/x','/v1/%2e%2e/x']) test(`reject ${input}`,()=>assert.throws(()=>safeRequestUrl(input,base,true)))
test('production preserves existing concatenation',()=>assert.equal(safeRequestUrl('/v1/x','https://prod/api',false),'https://prod/api/v1/x'))
