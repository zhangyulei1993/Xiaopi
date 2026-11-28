import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'

const compose = ['compose', '--env-file', 'tests/.env.test', '-f', 'tests/docker-compose.test.yml']
const config = JSON.parse(execFileSync('docker', [...compose, 'config', '--format', 'json'], { encoding: 'utf8' }))
const env = config.services['api-test'].environment
assert.equal(env.APP_ENV, 'test')
assert.equal(env.MYSQL_DATABASE, 'xiaopi_miniapp_test')
assert.equal(env.MYSQL_HOST, 'mysql-test')
const base = 'http://127.0.0.1:18083/api'
let token = ''
async function api(method, path, body) {
  const response = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json', Authorization: token ? `Bearer ${token}` : '' }, body: body === undefined ? undefined : JSON.stringify(body) })
  return { status: response.status, body: await response.json() }
}
for (const pair of env.TEST_WECHAT_CODE_MAP.split(',')) {
  const code = pair.slice(0, pair.indexOf('='))
  const login = await api('POST', '/v1/auth/wechat/login', { code })
  if (login.body.data?.testOnly && login.body.data.role === 'admin') { token = login.body.data.accessToken; break }
}
assert.ok(token, 'isolated administrator required')
const properties = await api('GET', '/v1/properties')
assert.equal(properties.status, 200)
const sample = properties.body.data.find(p => p.community)
assert.ok(sample, 'existing community required; do not create catalogue fixtures')
const body = { title: '创建重试专项回归', community: sample.community, location: sample.location, rentMonthly: 1000, createKey: `audit-${Date.now()}` }
let createdID
try {
  const first = await api('POST', '/v1/properties', body)
  assert.equal(first.status, 201)
  createdID = first.body.data.id
  const retry = await api('POST', '/v1/properties', body)
  assert.equal(retry.status, 201)
  assert.equal(retry.body.data.id, createdID)
  const changed = await api('POST', '/v1/properties', { ...body, rentMonthly: 1200 })
  assert.equal(changed.status, 409)
  const list = await api('GET', '/v1/properties')
  assert.equal(list.body.data.filter(p => p.title === body.title).length, 1)
  console.log('PASS: 18083 creation replay returns one draft; changed retry body is rejected (409)')
} finally {
  if (createdID) {
    const removed = await api('DELETE', `/v1/properties/${encodeURIComponent(createdID)}`)
    assert.equal(removed.status, 200)
    console.log('PASS: removed only this run’s temporary draft; imported community data preserved')
  }
}
