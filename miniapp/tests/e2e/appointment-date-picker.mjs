import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { classify } from '../../scripts/classify-smoke-network.mjs'

const root = fileURLToPath(new URL('../../', import.meta.url))
const project = path.join(root, 'dist/mp-weixin-test')
const out = path.join(root, 'tests/reports/appointment-date-picker', new Date().toISOString().replaceAll(':', '-'))
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
  const reply = JSON.parse(run(name, 'wechatide', ['-c', 'Codex', tool, '--project', project, ...args]))
  assert.ok(reply.ok && reply.result?.success !== false, name)
  return reply.result
}
function capture(name, selector) {
  call(name + '-dom', 'automation_element_action', '--action', 'outerWxml', '--selector', selector, '--wait-for-selector', selector)
  call(name + '-screenshot', 'simulator_screenshot', '--path', path.join(out, name + '.png'), '--optimize=false')
  const errors = call(name + '-console', 'get_simulator_console', '--command', 'grep -iE "error|exception|fail"')
  const raw = call(name + '-network', 'get_simulator_network', '--command', 'grep -n .')
  report.consoleErrors = errors
  assert.equal(typeof raw, 'string')
  const entries = raw.split('\n').filter(Boolean).map(line => JSON.parse(line.replace(/^\d+:/, '')))
  for (const { detail } of entries) {
    const login = detail.method === 'POST' && detail.url === 'http://127.0.0.1:18083/api/v1/auth/wechat/login'
    const allowed = detail.method === 'POST' && ['http://127.0.0.1:18083/api/v1/footprints/qa-property-1','http://127.0.0.1:18083/api/v1/rental-requests'].includes(detail.url)
    if (login || allowed) assert.ok(detail.status === undefined || [200,201].includes(detail.status))
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
  run('notification-disabled', 'docker', ['exec', api, 'sh', '-ec', 'test -z "$WECHAT_MINI_APP_ID$WECHAT_MINI_APP_SECRET"'])
  const state = call('pending-precheck', 'automation_evaluate', '--fn-source', 'function(){return !!wx.getStorageSync("xiaopi_pending_action")}')
  assert.equal(state.result.result, false)
  const element = (name, action, selector, ...args) => call(name, 'automation_element_action', '--action', action, '--selector', selector, '--wait-for-selector', selector, ...args)
  call('open-qa-login', 'simulator_open_page', '--page', 'pages/auth/login')
  const qa = '.test-login-panel button'
  assert.match(element('qa-label','text',qa), /普通用户 A/)
  element('qa-tap','tap',qa)
  element('qa-ready','size','.hero-search')
  call('open-detail','simulator_open_page','--page','pages/properties/detail','--query','id=qa-property-1')
  assert.match(element('detail','text','.overview'), /TEST Community A/)
  element('open-appointment','tap','.floating-appointment')
  element('sheet-ready','size','.appointment-sheet')
  const entries = capture('sheet','.appointment-sheet')
  const dom = fs.readFileSync(path.join(out, 'sheet-dom.log'), 'utf8')
  assert.doesNotMatch(JSON.stringify(entries), /"method":"POST","url":"http:\/\/127\.0\.0\.1:18083\/api\/v1\/rental-requests/)
  if (!/<(?:wx-)?picker\b/.test(dom)) {
    report.status = 'BLOCKED'
    report.reason = 'No date picker in rendered property appointment sheet. The separate request form picker is for moveInDate.'
    report.tapComparison = 'Ordinary tap opened the appointment entry. Date-control tap vs trigger tap unavailable because no date control rendered.'
    process.exitCode = 2
  } else {
    throw new Error('Date control detected. This diagnostic cannot verify its appointment date semantics.')
  }
} catch (error) {
  report.error = error.message
  process.exitCode = 1
} finally {
  fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ status: report.status, report: path.join(out, 'result.json'), error: report.error }))
}
