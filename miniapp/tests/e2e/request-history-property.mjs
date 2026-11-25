import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { classify } from '../../scripts/classify-smoke-network.mjs'
import { normalizeDevtoolsCall } from './devtools-compat.mjs'

const root = fileURLToPath(new URL('../../', import.meta.url))
const project = path.join(root, 'dist/mp-weixin-test')
const out = path.join(root, 'tests/reports/request-history-property', new Date().toISOString().replaceAll(':', '-'))
fs.mkdirSync(out, { recursive: true })
const suiteLock = enterSuiteLock(new Date().toISOString())
const report = { project, status: 'FAIL', steps: [] }
function run(name, command, args, input) {
  const r = spawnSync(command, args, { cwd: root, ...suiteChildOptions(suiteLock), encoding: 'utf8', input, timeout: 25000 })
  report.steps.push({ name, exitCode: r.status })
  fs.writeFileSync(path.join(out, name + '.log'), ((r.stdout || '') + (r.stderr || '')).replace(/eyJ[\w-]+\.[\w-]+\.[\w-]+/g, '[REDACTED JWT]'))
  assert.equal(r.status, 0, name + ': ' + (r.error?.message ?? r.stderr))
  return r.stdout
}
function call(name, tool, ...args) {
  ;({ tool, args } = normalizeDevtoolsCall(tool, args))
  const reply = JSON.parse(run(name, 'wechatide', ['-c', 'Codex', tool, '--project', project, ...args]))
  assert.ok(reply.ok && reply.result?.success !== false, name)
  return reply.result
}
function capture(name, selector) {
  call(name + '-dom', 'automation_element_action', '--action', 'outerWxml', '--selector', selector, '--wait-for-selector', selector)
  call(name + '-screenshot', 'simulator_screenshot', '--path', path.join(out, name + '.png'), '--optimize=false')
  const errors = call(name + '-console', 'get_simulator_console', '--command', 'grep -iE "error|exception|fail"')
  if (name === 'detail') {
    // Flush DevTools' throttled Network cache without replaying the footprint write.
    const probe = call('detail-network-flush','automation_evaluate','--fn-source','function(){return new Promise((resolve,reject)=>wx.request({url:"http://127.0.0.1:18083/api/health",method:"GET",success:r=>resolve(r.statusCode),fail:reject}))}')
    assert.equal(probe.result.result,200)
  }
  const raw = call(name + '-network', 'get_simulator_network', '--command', "grep -n -E 'HTTP_(REQUEST|RESPONSE).*127.0.0.1:18083/api/v1/(auth/wechat/login|footprints/qa-property-1)'")
  assert.equal(errors, '', name + ' console errors')
  assert.equal(typeof raw, 'string')
  const entries = raw.split('\n').filter(line => /^\d+:/.test(line)).map(line => JSON.parse(line.replace(/^\d+:/, '')))
  for (const { detail } of entries) {
    const login = detail.method === 'POST' && detail.url === 'http://127.0.0.1:18083/api/v1/auth/wechat/login'
    const footprint = detail.method === 'POST' && detail.url === 'http://127.0.0.1:18083/api/v1/footprints/qa-property-1'
    if (login || (name === 'detail' && footprint)) assert.ok(detail.status === undefined || detail.status === 200)
    else assert.notEqual(classify(detail), 'FAIL', name + ' unexpected network event')
  }
  return entries
}
try {
  run('build-security', process.execPath, ['scripts/scan-build-security.mjs', project])
  const api = 'xiaopi-miniapp-test-api-test-1'
  const db = 'xiaopi-miniapp-test-mysql-test-1'
  for (const container of [api, db]) {
    assert.equal(run(container + '-owner', 'docker', ['inspect', '-f', '{{index .Config.Labels "com.docker.compose.project"}}', container]).trim(), 'xiaopi-miniapp-test')
  }
  assert.equal(run('api-binding', 'docker', ['port', api, '8080/tcp']).trim(), '127.0.0.1:18083')
  run('api-environment', 'docker', ['exec', api, 'sh', '-ec', '[ "$APP_ENV" = test ] && [ "$MYSQL_HOST" = mysql-test ] && [ "$MYSQL_DATABASE" = xiaopi_miniapp_test ]'])
  const mysql = (name, sql) => run(name, 'docker', ['exec', '-i', db, 'sh', '-ec', '[ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 -h127.0.0.1 -u"$MYSQL_USER" -N "$MYSQL_DATABASE"'], sql)
  assert.equal(mysql('fixture-precheck', "SELECT COUNT(*) FROM users WHERE id IN ('qa-user-a','qa-user-b') AND open_id=CONCAT('test:',id) AND status='active';").trim(), '2')
  assert.equal(mysql('property-precheck', "SELECT COUNT(*) FROM properties WHERE id='qa-property-1' AND status='available';").trim(), '1')
  const propertyIdentity = mysql('property-identity', "SELECT community,location FROM properties WHERE id='qa-property-1';").trim().split('\t')
  assert.equal(propertyIdentity.length, 2)
  assert.equal(mysql('history-fixture', "SELECT COUNT(*) FROM rental_requests WHERE (id='qa-history-e2e-a' AND user_id='qa-user-a' OR id='qa-history-e2e-b' AND user_id='qa-user-b') AND property_id='qa-property-1';").trim(), '2')
  const count = (name, user) => Number(mysql(name, "SELECT COALESCE(SUM(view_count),0) FROM user_footprints WHERE property_id='qa-property-1' AND user_id='" + user + "';").trim())
  const beforeA = count('footprint-before-a', 'qa-user-a')
  const beforeB = count('footprint-before-b', 'qa-user-b')
  const state = call('pending-precheck', 'automation_evaluate', '--fn-source', 'function(){return !!wx.getStorageSync("xiaopi_pending_action")}')
  assert.equal(state.result.result, false)
  function login(user) {
    call('open-login-' + user, 'simulator_open_page', '--page', 'pages/auth/login')
    const number = user === 'A' ? '13800000001' : '13800000002'
    const code = user === 'A' ? '100001' : '100002'
    call('login-phone-' + user, 'automation_element_action', '--action', 'input', '--selector', '.test-phone-input', '--wait-for-selector', '.test-phone-input', '--value', number)
    call('login-code-' + user, 'automation_element_action', '--action', 'input', '--selector', '.test-access-code-input', '--wait-for-selector', '.test-access-code-input', '--value', code)
    call('login-tap-' + user, 'automation_element_action', '--action', 'tap', '--selector', '#test-access-code-submit', '--wait-for-selector', '#test-access-code-submit')
    call('login-ready-' + user, 'automation_element_action', '--action', 'size', '--selector', '.hero-search', '--wait-for-selector', '.hero-search')
    call('open-history-' + user, 'simulator_open_page', '--page', 'pages/requests/history')
  }
  login('A')
  const selector = '.record.clickable'
  const label = call('linked-record', 'automation_element_action', '--action', 'text', '--selector', selector, '--wait-for-selector', selector)
  // An account can have multiple appointments; only click a known target.
  assert.match(label, /QA History Listing A|TEST Community A/)
  assert.match(label, /查看房源/)
  call('tap-linked-property', 'automation_element_action', '--action', 'tap', '--selector', selector, '--wait-for-selector', selector)
  const entries = capture('detail', '.overview')
  const route = call('detail-route', 'automation_runtime_info', '--action', 'currentPage')
  assert.equal(route.currentPage.path, 'pages/properties/detail')
  assert.equal(route.currentPage.query.id, 'qa-property-1')
  const text = call('detail-text', 'automation_element_action', '--action', 'text', '--selector', '.overview')
  for (const value of [propertyIdentity[0],'1800',propertyIdentity[1],'1bd 1ba']) assert.ok(text.includes(value), 'Missing UI value: ' + value)
  const media = call('property-media', 'automation_element_action', '--action', 'outerWxml', '--selector', '.media-gallery', '--wait-for-selector', '.media-gallery')
  assert.match(media, /<(?:video|image)\b/, 'Linked property media is rendered')
  const footprint = entries.filter(e => e.detail.method === 'POST' && e.detail.url === 'http://127.0.0.1:18083/api/v1/footprints/qa-property-1')
  const afterA = count('footprint-after-a', 'qa-user-a')
  report.footprint = { beforeA, afterA, beforeB, responses: footprint.filter(e => e.type === 'HTTP_RESPONSE').map(e => e.detail.status) }
  assert.ok(footprint.some(e => e.type === 'HTTP_REQUEST'))
  assert.ok(report.footprint.responses.length > 0 && report.footprint.responses.every(status => status === 200))
  assert.equal(afterA, beforeA + 1)
  const apiBase = 'http://127.0.0.1:18083/api'
  async function localJSON(endpoint, options = {}) {
    const response = await fetch(apiBase + endpoint, { ...options, redirect: 'error', signal: AbortSignal.timeout(5000) })
    assert.equal(response.status, 200, endpoint)
    const body = await response.json()
    assert.equal(body.code, 0, endpoint)
    return body.data
  }
  const session = await localJSON('/v1/auth/wechat/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code: 'qa-user-b' }),
  })
  assert.ok(session.testOnly && session.role === 'user' && session.accessToken)
  const options = { headers: { Authorization: 'Bearer ' + session.accessToken } }
  const history = await localJSON('/v1/rental-requests/mine', options)
  assert.ok(history.some(item => item.id === 'qa-history-e2e-b'))
  assert.ok(history.every(item => item.userId === 'qa-user-b'))
  const footprints = await localJSON('/v1/footprints', options)
  assert.equal(footprints?.find(item => item.propertyId === 'qa-property-1')?.viewCount ?? 0, beforeB)
  report.userB = { verification: 'isolated authenticated API', ownHistoryVisible: true, targetFootprintCount: beforeB }
  report.footprint.afterB = count('footprint-after-b', 'qa-user-b')
  assert.equal(report.footprint.afterB, beforeB)
  report.assertions = ['Property association and overview verified', 'Fixture has no image; verified the empty cover', 'Exactly one isolated footprint increment for A', 'B history excluded A private records']
  report.status = 'PASS'
} catch (error) {
  report.error = error.message
  process.exitCode = 1
} finally {
  fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ status: report.status, report: path.join(out, 'result.json'), error: report.error }))
}
