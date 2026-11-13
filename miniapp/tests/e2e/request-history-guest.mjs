import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { classify } from '../../scripts/classify-smoke-network.mjs'

const root = fileURLToPath(new URL('../../', import.meta.url))
const project = path.join(root, 'dist/mp-weixin-test')
const out = path.join(root, 'tests/reports/request-history-guest', new Date().toISOString().replaceAll(':', '-'))
fs.mkdirSync(out, { recursive: true })
const suiteLock = enterSuiteLock(new Date().toISOString())
const report = { project, status: 'FAIL', steps: [] }
function run(name, command, args) {
  const r = spawnSync(command, args, { cwd: root, ...suiteChildOptions(suiteLock), encoding: 'utf8', timeout: 25000 })
  report.steps.push({ name, exitCode: r.status })
  fs.writeFileSync(path.join(out, name + '.log'), (r.stdout || '') + (r.stderr || ''))
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
  for (const line of raw.split('\n').filter(Boolean)) {
    const { detail } = JSON.parse(line.replace(/^\d+:/, ''))
    assert.notEqual(classify(detail), 'FAIL', name + ' unexpected network event')
    assert.ok(!new URL(detail.url).pathname.startsWith('/api/v1/rental-requests'), 'Guest requested private history')
  }
}
try {
  run('build-security', process.execPath, ['scripts/scan-build-security.mjs', project])
  const state = call('guest-precheck', 'automation_evaluate', '--fn-source', 'function(){return {guest:!wx.getStorageSync("xiaopi_access_token"),pending:!!wx.getStorageSync("xiaopi_pending_action")}}')
  assert.deepEqual(state.result.result, { guest: true, pending: false })
  call('open-history', 'simulator_open_page', '--page', 'pages/requests/history')
  const button = '.empty.state button'
  const label = call('guest-label', 'automation_element_action', '--action', 'text', '--selector', '.empty.state', '--wait-for-selector', button)
  assert.match(label, /登录后查看找房记录/)
  capture('history', '.empty.state')
  call('tap-login-entry', 'automation_element_action', '--action', 'tap', '--selector', button, '--wait-for-selector', button)
  const login = call('login-route', 'automation_runtime_info', '--action', 'currentPage')
  assert.equal(login.currentPage.path, 'pages/auth/login')
  assert.equal(login.currentPage.query.reason, 'requests')
  capture('login', '.login-title')
  report.status = 'PASS'
} catch (error) {
  report.error = error.message
  process.exitCode = 1
} finally {
  fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ status: report.status, report: path.join(out, 'result.json'), error: report.error }))
}
