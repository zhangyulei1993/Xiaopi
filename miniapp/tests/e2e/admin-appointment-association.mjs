import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { classify } from '../../scripts/classify-smoke-network.mjs'

const root = fileURLToPath(new URL('../../', import.meta.url))
const project = path.join(root, 'dist/mp-weixin-test')
const out = path.join(root, 'tests/reports/admin-appointment-association', new Date().toISOString().replaceAll(':', '-'))
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
  assert.equal(errors, '', name + ' console errors')
  assert.equal(typeof raw, 'string')
  const entries = raw.split('\n').filter(Boolean).map(line => JSON.parse(line.replace(/^\d+:/, '')))
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
  const mysql = (name, sql) => run(name, 'docker', ['exec', '-i', db, 'sh', '-ec', '[ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 -h127.0.0.1 -u"$MYSQL_USER" -N "$MYSQL_DATABASE"'], sql)
  // The local test database may contain manually created smoke rows. This case
  // must inspect only the two deterministic synthetic appointments it owns.
  const sql = "SELECT JSON_OBJECT('id',r.id,'userId',r.user_id,'propertyId',r.property_id,'propertyTitle',r.property_title,'contactName',r.contact_name,'phone',r.phone,'preferredTime',r.preferred_time,'status',r.status,'livePropertyTitle',p.title) FROM rental_requests r LEFT JOIN properties p ON p.id=r.property_id WHERE r.request_type='appointment' AND r.id IN ('qa-history-e2e-a','qa-history-e2e-b') ORDER BY r.id;"
  const before = mysql('appointments-before', sql)
  const records = before.trim().split('\n').filter(Boolean).map(JSON.parse)
  assert.ok(records.some(r => r.id === 'qa-history-e2e-a') && records.some(r => r.id === 'qa-history-e2e-b'))
  assert.ok(records.every(r => ['qa-user-a','qa-user-b'].includes(r.userId) && r.propertyId.startsWith('qa-')), 'Expected QA-only appointment fixtures')
  const pending = call('pending-precheck','automation_evaluate','--fn-source','function(){return !!wx.getStorageSync("xiaopi_pending_action")}')
  assert.equal(pending.result.result, false)
  const response = await fetch('http://127.0.0.1:18083/api/v1/auth/wechat/login', {
    method: 'POST', redirect: 'error', signal: AbortSignal.timeout(5000),
    headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code: 'qa-admin' }),
  })
  assert.equal(response.status, 200)
  const { data: session } = await response.json()
  assert.ok(session.testOnly && session.role === 'admin' && session.accessToken)
  const secretDir = fs.mkdtempSync(path.join(os.tmpdir(), 'wechat-qa-session-'))
  try {
    const argsFile = path.join(secretDir, 'args.json')
    fs.writeFileSync(argsFile, JSON.stringify([session.accessToken]), { mode: 0o600 })
    call('set-qa-admin-session','automation_evaluate','--fn-source','function(token){wx.setStorageSync("xiaopi_access_token",token);wx.setStorageSync("xiaopi_session_role","admin");wx.setStorageSync("xiaopi_admin_tab","requests");return true}','--args-file',argsFile)
  } finally { fs.rmSync(secretDir, { recursive: true, force: true }) }
  const isAppointmentsResponse = e => e.type==='HTTP_RESPONSE' && e.detail.url==='http://127.0.0.1:18083/api/v1/admin/rental-requests' && e.detail.method==='GET' && e.detail.status===200
  call('open-admin-requests','simulator_open_page','--page','pages/admin/index')
  call('wait-for-cards','automation_element_action','--action','size','--selector','.request-property','--wait-for-selector','.request-property')
  let network = capture('appointments','.page').find(isAppointmentsResponse)
  report.networkEvidence = 'appointments-network.log'
  for (let attempt = 1; !network && attempt <= 2; attempt++) {
    // DevTools batches inspector events. A local read can flush the pending responses.
    const probe = call('network-flush-'+attempt,'automation_evaluate','--fn-source','function(){return new Promise((resolve,reject)=>wx.request({url:"http://127.0.0.1:18083/api/health",method:"GET",success:r=>resolve(r.statusCode),fail:reject}))}')
    assert.equal(probe.result.result, 200)
    const name = 'appointments-network-after-flush-'+attempt
    const raw = call(name, 'get_simulator_network', '--command', 'grep -n .')
    const events = raw.split('\n').filter(Boolean).map(line => JSON.parse(line.replace(/^\d+:/, '')))
    assert.ok(events.every(({detail}) => classify(detail) !== 'FAIL'), 'Unexpected network event after cache flush')
    network = events.find(isAppointmentsResponse)
    report.networkEvidence = name+'.log'
  }
  const route = call('admin-route','automation_runtime_info','--action','currentPage')
  assert.equal(route.currentPage.path,'pages/admin/index')
  const dom = fs.readFileSync(path.join(out,'appointments-dom.log'),'utf8')
  assert.ok(network,'Missing admin appointments response')
  const payload = typeof network.detail.response==='string' ? JSON.parse(network.detail.response) : network.detail.response
  const appointments = payload.data.filter(r=>r.requestType==='appointment' && ['qa-history-e2e-a','qa-history-e2e-b'].includes(r.id))
  assert.equal(appointments.length,records.length)
  const cards = dom.split(/class=\?"card item\b/)
  for (const record of records) {
    const returned = appointments.find(r=>r.id===record.id)
    assert.ok(returned,record.id)
    for (const field of ['userId','propertyId','propertyTitle','contactName','phone','preferredTime','status']) assert.equal(returned[field],record[field],record.id+' '+field)
    assert.equal(returned.propertyAvailable,Boolean(record.livePropertyTitle))
    const card = cards.find(text=>text.includes(record.contactName) && text.includes(record.preferredTime) && text.includes(record.propertyTitle))
    assert.ok(card && card.includes(record.status) && card.includes('联系客户'), record.id+' card association')
  }
  assert.equal(mysql('appointments-after',sql),before)
  report.appointments = records.map(({ id, userId, propertyId, propertyTitle, status }) => ({ id, userId, propertyId, propertyTitle, status }))
  report.databaseUnchanged = true
  report.status = 'PASS'
} catch (error) {
  report.error = error.message
  process.exitCode = 1
} finally {
  fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ status: report.status, report: path.join(out, 'result.json'), error: report.error }))
}
