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
const out = path.join(root, 'tests/reports/general-form-validation', new Date().toISOString().replaceAll(':', '-'))
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
  const raw = call(name + '-network', 'get_simulator_network', '--command', "grep -n -E 'HTTP_(REQUEST|RESPONSE).*127.0.0.1:18083/api/v1/rental-requests")
  assert.equal(errors, '', name + ' console errors')
  assert.equal(typeof raw, 'string')
  const entries = raw.split('\n').filter(line => /^\d+:/.test(line)).map(line => JSON.parse(line.replace(/^\d+:/, '')))
  for (const { detail } of entries) {
    const login = detail.method === 'POST' && detail.url === 'http://127.0.0.1:18083/api/v1/auth/wechat/login'
    if (login) assert.ok(detail.status === undefined || detail.status === 200)
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
  const mysql = (name, sql) => run(name, 'docker', ['exec', '-i', db, 'sh', '-ec', '[ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -N "$MYSQL_DATABASE"'], sql)
  assert.equal(mysql('fixture-precheck', "SELECT COUNT(*) FROM users WHERE id IN ('qa-user-a','qa-user-b') AND open_id=CONCAT('test:',id) AND status='active';").trim(), '2')
  assert.equal(mysql('property-precheck', "SELECT COUNT(*) FROM properties WHERE id='qa-property-1' AND status='available';").trim(), '1')
  const records = "SELECT id,updated_at FROM rental_requests WHERE user_id='qa-user-a' ORDER BY id;"
  const before = mysql('records-before', records)
  const state = call('pending-precheck', 'automation_evaluate', '--fn-source', 'function(){return !!wx.getStorageSync("xiaopi_pending_action")}')
  assert.equal(state.result.result, false)
  call('open-qa-login', 'simulator_open_page', '--page', 'pages/auth/login')
  call('test-phone-input', 'automation_element_action', '--action', 'input', '--selector', '.test-phone-input', '--wait-for-selector', '.test-phone-input', '--value', '13800000001')
  call('test-access-code-input', 'automation_element_action', '--action', 'input', '--selector', '.test-access-code-input', '--wait-for-selector', '.test-access-code-input', '--value', '100001')
  call('qa-tap', 'automation_element_action', '--action', 'tap', '--selector', '#test-access-code-submit', '--wait-for-selector', '#test-access-code-submit')
  call('qa-ready', 'automation_element_action', '--action', 'size', '--selector', '.hero-search', '--wait-for-selector', '.hero-search')
  call('open-general-request', 'simulator_open_page', '--page', 'pages/requests/index')
  const element = (name, action, selector, ...args) => call(name, 'automation_element_action', '--action', action, '--selector', selector, '--wait-for-selector', selector, ...args)
  assert.match(element('bound-phone','text','.verified-phone'), /已绑定/)
  assert.match(element('submit-label','text','.button'), /提交找房需求/)
  const fields = element('form-before','outerWxml','.page')
  assert.match(JSON.stringify(fields), /地区/)
  element('submit-invalid','trigger','.button','--type','tap')
  call('validation-feedback','simulator_screenshot','--path',path.join(out,'validation.png'),'--optimize=false')
  const entries = capture('general-form', '.page')
  const route = call('form-route', 'automation_runtime_info', '--action', 'currentPage')
  assert.equal(route.currentPage.path, 'pages/requests/index')
  // DevTools may omit an already completed profile request from its bounded
  // network buffer. The bound-phone UI is asserted above; this case is about
  // preventing an invalid submit and ensuring the persisted request set stays
  // unchanged, so do not make the profile transport log a hard dependency.
  assert.ok(entries.every(e => !(e.detail.method === 'POST' && e.detail.url.endsWith('/v1/rental-requests'))))
  assert.equal(mysql('records-after', records), before)
  report.databaseUnchanged = true
  report.status = 'PASS'
} catch (error) {
  report.error = error.message
  process.exitCode = 1
} finally {
  fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ status: report.status, report: path.join(out, 'result.json'), error: report.error }))
}
