import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { classify } from '../../scripts/classify-smoke-network.mjs'

const root = fileURLToPath(new URL('../../', import.meta.url))
const project = path.join(root, 'dist/mp-weixin-test')
const out = path.join(root, 'tests/reports/favorites-toggle', new Date().toISOString().replaceAll(':', '-'))
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
    const login = detail.method === 'POST' && ['http://127.0.0.1:18083/api/v1/auth/wechat/login', 'http://127.0.0.1:18083/api/v1/auth/refresh'].includes(detail.url)
    const target = detail.url === 'http://127.0.0.1:18083/api/v1/favorites/qa-property-1'
    const toggle = target && (name === 'added' && detail.method === 'PUT' || name === 'removed' && detail.method === 'DELETE')
    if (login || toggle) assert.ok(detail.status === undefined || detail.status === 200)
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
  const fixtureSQL = "SELECT id,title,rent_monthly FROM properties WHERE id='qa-property-1' AND status='available';"
  assert.equal(mysql('listing-fixture', fixtureSQL).trim(), 'qa-property-1\tTEST Listing A\t1800')
  const counts = "SELECT COUNT(*) FROM property_favorites WHERE user_id IN ('qa-user-a','qa-user-b') AND property_id='qa-property-1';"
  const footprints = "SELECT user_id,property_id,view_count FROM user_footprints WHERE user_id IN ('qa-user-a','qa-user-b') ORDER BY user_id,property_id;"
  const beforeFootprints = mysql('footprints-before', footprints)
  const resume = process.argv.includes('--resume-after-add')
  if (resume) {
    const prior = path.join(root, 'tests/reports/favorites-toggle/2026-11-10T19-30-00.473Z')
    assert.equal(fs.readFileSync(path.join(prior, 'favorites-added.log'), 'utf8').trim(), 'qa-user-a\tqa-property-1')
    assert.equal(mysql('favorites-before', counts).trim(), '1')
    report.resumedFrom = prior
  } else {
    assert.equal(mysql('favorites-before', counts).trim(), '0')
  }
  const pending = call('pending-precheck', 'automation_evaluate', '--fn-source', 'function(){return !!wx.getStorageSync("xiaopi_pending_action")}')
  assert.equal(pending.result.result, false)
  const element = (name, action, selector, ...args) => call(name,'automation_element_action','--action',action,'--selector',selector,'--wait-for-selector',selector,...args)
  call('open-qa-login','simulator_open_page','--page','pages/auth/login')
  const qa = '.test-login-panel button'
  assert.match(element('qa-label','text',qa), /普通用户 A/)
  element('qa-tap','tap',qa)
  element('qa-ready','size','.hero-search')
  call('open-property-card','simulator_open_page','--page','pages/properties/index')
  assert.match(element('target-card','text','.property-listing-title'), /TEST Community A/)
  const heart = '.masonry-property .favorite'

  function verifyToggle(events, method) {
    const endpoint = 'http://127.0.0.1:18083/api/v1/favorites/qa-property-1'
    assert.equal(events.filter(e => e.type === 'HTTP_REQUEST' && e.detail.method === method && e.detail.url === endpoint).length,1)
    assert.ok(events.some(e => e.type === 'HTTP_RESPONSE' && e.detail.method === method && e.detail.url === endpoint && e.detail.status === 200))
  }
  if (!resume) {
    element('add-favorite','trigger',heart,'--type','tap')
    verifyToggle(capture('added','.favorite.active'),'PUT')
  }
  assert.equal(mysql('favorites-added', "SELECT user_id,property_id FROM property_favorites WHERE user_id IN ('qa-user-a','qa-user-b') AND property_id='qa-property-1';").trim(), 'qa-user-a\tqa-property-1')
  const base = 'http://127.0.0.1:18083/api'
  const login = await fetch(base+'/v1/auth/wechat/login',{method:'POST',redirect:'error',signal:AbortSignal.timeout(5000),headers:{'Content-Type':'application/json'},body:JSON.stringify({code:'qa-user-b'})})
  assert.equal(login.status,200)
  const {data:session} = await login.json()
  assert.ok(session.testOnly && session.role==='user' && session.accessToken)
  const response = await fetch(base+'/v1/favorites',{redirect:'error',signal:AbortSignal.timeout(5000),headers:{Authorization:'Bearer '+session.accessToken}})
  assert.equal(response.status,200)
  const b = await response.json()
  assert.equal(b.code,0)
  assert.ok(Array.isArray(b.data) && !b.data.includes('qa-property-1'))
  report.userB = { status:200, targetVisible:false }
  call('open-me','simulator_open_page','--page','pages/me/index')
  element('my-favorites-entry','tap','.shortcut-icon.favorite')
  assert.match(element('favorites-filter','text','.favorite-filter.active'), /只看已关注/)
  assert.match(element('favorite-listing','text','.property-listing-title'), /TEST Community A/)
  capture('my-favorites','.grid')
  element('remove-favorite','trigger',heart,'--type','tap')
  const removed = capture('removed','.favorites-empty')
  verifyToggle(removed,'DELETE')
  assert.match(element('empty-favorites','text','.favorites-empty'), /还没有找到关注的房源/)
  assert.equal(mysql('favorites-after',counts).trim(),'0')
  assert.equal(mysql('footprints-after',footprints), beforeFootprints)
  report.database = { initial:resume ? 1 : 0, afterAdd:1, afterCancel:0, footprintsUnchanged:true }
  report.status = 'PASS'
} catch (error) {
  report.error = error.message
  process.exitCode = 1
} finally {
  fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ status: report.status, report: path.join(out, 'result.json'), error: report.error }))
}
