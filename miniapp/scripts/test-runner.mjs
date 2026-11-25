#!/usr/bin/env node
import { enterSuiteLock, suiteChildOptions } from '../tests/suite-lock.mjs'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { normalizeDevtoolsCall } from '../tests/e2e/devtools-compat.mjs'

const root = path.resolve(process.env.WECHAT_PROJECT || path.join(path.dirname(new URL(import.meta.url).pathname), '..'))
const mode = process.argv[2] || 'smoke'
const pkg = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'))
const cfg = ['project.config.json', 'project.private.config.json'].map(n => path.join(root, n)).find(fs.existsSync)
if (!cfg) throw new Error(`No WeChat project config: ${root}`)
const projectConfig = JSON.parse(fs.readFileSync(cfg, 'utf8'))
const reportBase = path.join(root, 'tests/reports', path.basename(root))
const suiteLock = mode === 'report' ? null : enterSuiteLock(new Date().toISOString())
const runId = new Date().toISOString().replaceAll(':', '-').replaceAll('.', '-')
const reportDir = path.join(reportBase, runId)
if (suiteLock) fs.mkdirSync(reportDir, { recursive: true })
const results = []
function run(name, command, args, cwd = root) {
  const startedAt = Date.now()
  const started = new Date().toISOString()
  const p = spawnSync(command, args, { cwd, encoding: 'utf8', ...suiteChildOptions(suiteLock), timeout: 30000, maxBuffer: 6 * 1024 * 1024 })
  const stdout = (p.stdout || '').replace(/eyJ[\w-]+\.[\w-]+\.[\w-]+/g, '[REDACTED JWT]')
  let payload
  try { payload = JSON.parse(stdout.slice(stdout.indexOf('{'))) } catch {}
  const item = { name, command: [command, ...args], started, durationMs: Date.now() - startedAt, exitCode: p.status, status: p.status === 0 && payload?.ok !== false ? 'passed' : 'failed', stdout, stderr: p.stderr || '', error: p.error?.message, result: payload?.result?.result ?? payload?.result }
  results.push(item)
  fs.writeFileSync(path.join(reportDir, `${name.replaceAll(/[^\w-]/g, '_')}.log`), `${item.stdout}\n${item.stderr}`)
  return item
}
function saveReport() {
  const report = { project: root, mode, appid: projectConfig.appid || null, compileType: projectConfig.compileType || null, completedAt: new Date().toISOString(), status: results.every(r => r.status === 'passed' || r.recoveredByRetry) ? 'passed' : 'failed', results }
  fs.writeFileSync(path.join(reportDir, 'result.json'), JSON.stringify(report, null, 2))
  console.log(JSON.stringify(report, null, 2))
  if (report.status !== 'passed') process.exitCode = 1
}
if (mode === 'report') {
  const dirs = fs.existsSync(reportBase) ? fs.readdirSync(reportBase, { withFileTypes: true }).filter(x => x.isDirectory() && fs.existsSync(path.join(reportBase,x.name,'result.json'))).map(x => x.name).sort() : []
  if (!dirs.length) { console.error('No reports'); process.exit(2) }
  console.log(fs.readFileSync(path.join(reportBase, dirs.at(-1), 'result.json'), 'utf8'))
} else if (mode === 'build') {
  const buildTarget = process.env.WECHAT_BUILD_TARGET
  const preferred = buildTarget === 'test'
    ? ['build:mp-weixin:test']
    : buildTarget === 'development'
      ? ['build:mp-weixin:local']
      : ['build:mp-weixin:local', 'build:mp-weixin']
  const script = preferred.find(s => pkg.scripts?.[s]) || Object.keys(pkg.scripts || {}).find(s => /build.*(mini|weixin)/i.test(s))
  if (script) run('build', 'npm', ['run', script])
  else if (pkg.dependencies?.['@dcloudio/vite-plugin-uni']) run('build', 'npx', ['uni', 'build', '-p', 'mp-weixin'])
  else throw new Error('No recognized Mini Program build command')
  saveReport()
} else {
  try {
    const level = mode === 'e2e' ? 'e2e' : 'smoke'
    const caseDir = path.join(root, 'tests', level)
    const available = fs.existsSync(caseDir) ? fs.readdirSync(caseDir).filter(n => n.endsWith('.json')).sort() : []
    const cases = process.argv[3] ? available.filter(file => file === process.argv[3]) : available
    if (!cases.length) throw new Error(`No cases in ${caseDir}`)
    const out = path.resolve(root, process.env.WECHAT_BUILD_OUTPUT || 'dist/mp-weixin-test')
    const productionOutput = path.resolve(root, 'dist/mp-weixin')
    if (out === productionOutput) {
      throw new Error('Refusing simulator automation against the production build. Select dist/mp-weixin-test.')
    }
    const scan = run('build-security-scan', process.execPath, [path.join(root, 'scripts/scan-build-security.mjs'), out])
    if (scan.status !== 'passed') throw new Error('Build security scan failed; simulator automation is blocked')
    const simulatorProject = fs.existsSync(path.join(out, 'app.json')) ? out : root
    const cli = process.env.WECHATIDE_CLI || 'wechatide'
    const client = process.env.WECHATIDE_CLIENT || 'Codex'
    const isSafeRead = (tool, args) => {
      if (['automation_runtime_info', 'simulator_screenshot', 'get_simulator_console', 'get_simulator_network'].includes(tool)) return true
      const actionIndex = args.indexOf('--action')
      const action = actionIndex >= 0 ? args[actionIndex + 1] : ''
      if (tool === 'automation_element_action') return ['text', 'size', 'offset', 'outerWxml'].includes(action)
      if (tool === 'automation_page_action') return ['querySelector', 'querySelectorAll'].includes(action)
      return false
    }
    const call = (tool, args) => {
      ;({ tool, args } = normalizeDevtoolsCall(tool, args))
      const invoke = (suffix = '') => run(`${tool}${suffix}`, cli, ['-c', client, tool, '--project', simulatorProject, ...args])
      const first = invoke()
      const errorText = `${first.stdout}\n${first.stderr}`
      const timeout = errorText.includes('timeout waiting for automator response') && !errorText.includes('waitForSelector timeout:')
      if (first.status === 'passed' || !timeout || !isSafeRead(tool, args)) return first
      const retry = invoke('-retry1')
      retry.retryOf = first.name
      if (retry.status === 'passed') first.recoveredByRetry = true
      return retry
    }
    const waitFor = (selector) => call('automation_element_action', ['--action', 'size', '--selector', selector, '--wait-for-selector', selector])
    const readText = (selector) => call('automation_element_action', ['--action', 'text', '--selector', selector, '--wait-for-selector', selector])
    if (path.resolve(out) === path.resolve(root, 'dist/mp-weixin-test')) {
      const login = call('automation_evaluate', ['--fn-source', 'function(){return new Promise((resolve,reject)=>wx.request({url:"http://127.0.0.1:18083/api/v1/auth/wechat/login",method:"POST",data:{code:"qa-user-a"},success:r=>{const d=r.data&&r.data.data;if(r.statusCode!==200||!d||!d.accessToken)return reject(new Error("isolated QA login failed"));wx.setStorageSync("xiaopi_access_token",d.accessToken);wx.setStorageSync("xiaopi_session_role",d.role);resolve({status:r.statusCode,role:d.role,testOnly:d.testOnly})},fail:reject}))}'])
      if (login.status !== 'passed' || !/"role"\s*:\s*"user"/.test(login.stdout)) throw new Error('Isolated QA login bootstrap failed')
      const home = call('automation_navigate', ['--action', 'reLaunch', '--url', '/pages/home/index'])
      if (home.status !== 'passed') throw new Error('Isolated QA user could not open home')
      const loggedIn = waitFor('.hero-search')
      if (loggedIn.status !== 'passed') throw new Error('QA user did not reach the authenticated home page')
    }
    for (const file of cases) {
      const test = JSON.parse(fs.readFileSync(path.join(caseDir, file), 'utf8'))
      if (!test.readySelector) throw new Error(`${file} must define readySelector`)
      const [testPage, testQuery = ''] = test.page.split('?')
      const navArgs = ['--page', testPage.replace(/^\//, '')]
      if (testQuery) navArgs.push('--query', testQuery)
      const nav = call('simulator_open_page', navArgs)
      if (nav.status !== 'passed') break
      const ready = waitFor(test.readySelector)
      if (ready.status !== 'passed') break
      for (const a of test.assertions || []) {
        if (a.kind === 'current-page') {
          const result = call('automation_runtime_info', ['--action', 'currentPage'])
          if (result.status !== 'passed') results.push({ name: `assert-current-page:${a.equals}`, status: 'failed', expected: a.equals, actual: result.stdout })
          else if (!JSON.stringify(result.result ?? result.stdout).includes(a.equals)) results.push({ name: `assert-current-page:${a.equals}`, status: 'failed', expected: a.equals, actual: result.result ?? result.stdout })
          continue
        }
        const result = a.kind === 'page-text'
          ? readText(a.selector)
          : call('automation_page_action', ['--action', 'querySelector', '--selector', a.selector, '--wait-for-selector', a.selector])
        if (result.status !== 'passed' || (a.equals && !JSON.stringify(result.result ?? result.stdout).includes(a.equals))) { results.push({ name: `assert:${a.selector}`, status: 'failed', expected: a.equals, actual: result.result ?? result.stdout }); break }
      }
      for (const s of test.steps || []) {
        if (s.action === 'assert-page') {
          if (!s.readySelector) throw new Error(`assert-page in ${file} requires readySelector`)
          const pageReady = waitFor(s.readySelector)
          if (pageReady.status !== 'passed') { results.push({ name: `assert-page-ready:${s.readySelector}`, status: 'failed', actual: pageReady.stdout }); break }
          const result = call('automation_runtime_info', ['--action', 'currentPage'])
          if (result.status !== 'passed' || !result.stdout.includes(s.page)) results.push({ name: `assert-page:${s.page}`, status: 'failed', actual: result.stdout })
        } else if (s.action === 'assert-text') {
          const result = readText(s.selector)
          if (result.status !== 'passed' || (s.contains && !JSON.stringify(result.result ?? result.stdout).includes(s.contains))) results.push({ name: `assert-text:${s.selector}`, status: 'failed', expected: s.contains, actual: result.result ?? result.stdout })
        } else if (s.action === 'assert-network-absent') {
          const result = call('get_simulator_network', ['--command', `grep -iF ${JSON.stringify(s.contains)}`])
          const networkText = typeof result.result === 'string' ? result.result : result.stdout
          results.push({ name: `assert-network-absent:${s.contains}`, status: result.status === 'passed' && !networkText.includes(s.contains) ? 'passed' : 'failed', evidence: networkText })
        } else {
          const args = ['--action', s.action, '--selector', s.selector]
          if (s.value !== undefined) args.push('--value', String(s.value))
          const waitForSelector = s.waitForSelector || (['tap', 'input', 'longpress', 'trigger', 'scrollTo'].includes(s.action) ? s.selector : null)
          if (waitForSelector) args.push('--wait-for-selector', waitForSelector)
          if (call('automation_element_action', args).status !== 'passed') break
        }
      }
      if (results.some(r => r.status === 'failed' && !r.recoveredByRetry)) {
        call('automation_runtime_info', ['--action', 'currentPage'])
        call('simulator_screenshot', ['--path', path.join(reportDir, `${test.name}-failure.png`), '--optimize=false'])
        call('automation_page_action', ['--action', 'querySelectorAll', '--selector', 'view'])
        call('get_simulator_console', ['--command', 'grep -iE "error|exception|fail"'])
        call('get_simulator_network', ['--command', 'grep -n -E "HTTP_REQUEST.*127.0.0.1:18083/api|HTTP_RESPONSE.*127.0.0.1:18083/api"'])
        break
      }
      const selector = (test.steps || []).findLast(step => step.action === 'assert-page')?.readySelector || test.readySelector
      call('automation_element_action', ['--action', 'outerWxml', '--selector', selector, '--wait-for-selector', selector])
      call('simulator_screenshot', ['--path', path.join(reportDir, `${test.name}.png`), '--optimize=false'])
      call('get_simulator_console', ['--command', 'grep -iE "error|exception|fail"'])
      call('get_simulator_network', ['--command', 'grep -n -E "HTTP_REQUEST.*127.0.0.1:18083/api|HTTP_RESPONSE.*127.0.0.1:18083/api"'])
    }
    saveReport()
  } catch (error) {
    results.push({ name: 'runner', status: 'failed', error: error.message })
    fs.writeFileSync(path.join(reportDir, 'result.json'), JSON.stringify({ project: root, mode, status: 'failed', results }, null, 2))
    console.error(error.message)
    process.exitCode = 1
  }
}
