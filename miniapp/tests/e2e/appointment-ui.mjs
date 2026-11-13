import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { classify } from '../../scripts/classify-smoke-network.mjs'

const root = fileURLToPath(new URL('../../', import.meta.url))
const project = path.join(root, 'dist/mp-weixin-test')
const out = path.join(root, 'tests/reports/appointment-ui', new Date().toISOString().replaceAll(':', '-'))
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
  // An inspector event after the UI settles flushes the throttled Network cache.
  const probe = call(name + '-network-flush', 'automation_evaluate', '--fn-source', 'function(){return new Promise((resolve,reject)=>wx.request({url:"http://127.0.0.1:18083/api/health",method:"GET",success:r=>resolve(r.statusCode),fail:reject}))}')
  assert.equal(probe.result.result, 200)
  const raw = call(name + '-network', 'get_simulator_network', '--command', 'grep -n .')
  assert.equal(errors, '', name + ' console errors')
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
  const recent = "SELECT COUNT(*) FROM rental_requests WHERE user_id='qa-user-a' AND property_id='qa-property-1' AND request_type='appointment' AND created_at >= UTC_TIMESTAMP() - INTERVAL 30 MINUTE;"
  const resumeId = process.argv[2]
  if (resumeId) assert.match(resumeId, /^[0-9a-f-]{36}$/)
  const idSQL = "SELECT id FROM rental_requests WHERE user_id='qa-user-a' AND property_id='qa-property-1' AND request_type='appointment' AND created_at >= UTC_TIMESTAMP()-INTERVAL 30 MINUTE;"
  if (!resumeId) {
    const cooldownSQL = "SELECT GREATEST(0, COALESCE(MAX(TIMESTAMPDIFF(SECOND,UTC_TIMESTAMP(),created_at+INTERVAL 30 MINUTE)+1),0)) FROM rental_requests WHERE user_id='qa-user-a' AND property_id='qa-property-1' AND request_type='appointment';"
    const cooldown = Number(mysql('recent-cooldown', cooldownSQL).trim())
    assert.ok(Number.isInteger(cooldown) && cooldown >= 0 && cooldown <= 120, 'Unexpected recent appointment; deduplication window is still active')
    if (cooldown) await new Promise(resolve => setTimeout(resolve, cooldown * 1000))
  }
  assert.equal(mysql('recent-before', idSQL).trim(), resumeId || '', 'Unexpected recent appointment')
  report.resumeId = resumeId
  const contact = resumeId ? 'QA UI Appointment' : 'QA UI ' + Date.now()

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
  function fill(prefix, checkBlank) {
  element(prefix+'open-appointment','tap','.floating-appointment')
  const input = '.appointment-field input'
  const submit = '.appointment-submit'
  element(prefix+'input-ready','size',input)
  if (checkBlank) {
  element(prefix+'blank-name','trigger',input,'--type','input','--detail','{"value":""}')
  element(prefix+'blank-submit','trigger',submit,'--type','tap')
  element(prefix+'invalid-sheet','size','.appointment-sheet')
  assert.equal(mysql('recent-after-invalid', idSQL).trim(), resumeId || '')
  }
  const time = '.appointment-times text'
  const choice = element(prefix+'time-option','text',time)
  assert.match(choice,/今天方便/)
  element(prefix+'time-click','trigger',time,'--type','tap')
  assert.match(element(prefix+'active-time','text','.appointment-times .active'), /今天方便/)
  element(prefix+'name-input','input',input,'--value',contact)
  call(prefix+'form-screenshot','simulator_screenshot','--path',path.join(out,prefix+'form.png'),'--optimize=false')
  element(prefix+'submit','trigger',submit,'--type','tap')
  }
  const posts = entries => entries.filter(e=>e.type==='HTTP_RESPONSE' && e.detail.method==='POST' && e.detail.url==='http://127.0.0.1:18083/api/v1/rental-requests')
  fill('first-', true)
  const first = posts(capture('submitted','.overview'))
  assert.deepEqual(first.map(e=>e.detail.status), [resumeId ? 200 : 201])
  const id = mysql('created-id', idSQL).trim()
  assert.match(id,/^[0-9a-f-]{36}$/)
  assert.ok(JSON.stringify(first[0].detail.response).includes(id))
  if (resumeId) assert.equal(id, resumeId)
  else {
    call('reopen-detail','simulator_open_page','--page','pages/properties/detail','--query','id=qa-property-1')
    element('detail-ready','size','.overview')
    fill('duplicate-', false)
    const repeated = posts(capture('duplicate','.overview'))
    assert.equal(repeated.at(-1)?.detail.status, 200)
    assert.ok(JSON.stringify(repeated.at(-1).detail.response).includes(id))
  }
  assert.equal(mysql('recent-after',recent).trim(),'1')
  assert.equal(mysql('same-appointment',idSQL).trim(), id)
  report.record=mysql('appointment-row',"SELECT id,user_id,property_id,contact_name,preferred_time FROM rental_requests WHERE id='" + id + "';").trim()
  assert.deepEqual(report.record.split('\t'), [id, 'qa-user-a', 'qa-property-1', contact, '今天方便'])
  if (!resumeId) {
    // Only remove the row proven to have been created by this exact invocation.
    const owned = "id='" + id + "' AND user_id='qa-user-a' AND property_id='qa-property-1' AND request_type='appointment' AND contact_name='" + contact + "'"
    assert.match(contact, /^QA UI [0-9]{13}$/)
    assert.equal(mysql('cleanup-created-appointment', 'DELETE FROM rental_requests WHERE ' + owned + '; SELECT ROW_COUNT();').trim(), '1')
    assert.equal(mysql('cleanup-verify', idSQL).trim(), '')
  }
  report.status = 'PASS'
} catch (error) {
  report.error = error.message
  process.exitCode = 1
} finally {
  fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ status: report.status, report: path.join(out, 'result.json'), error: report.error }))
}
