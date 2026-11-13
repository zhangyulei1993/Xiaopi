import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { executionPlan, executePlan, requiredExit, permissionRows, evidenceIsCurrent, testRecord } from './execution-plan.mjs'
import { enterSuiteLock, suiteChildOptions } from './suite-lock.mjs'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const project = path.join(root, 'dist/mp-weixin-test')
const suiteLock = enterSuiteLock(new Date().toISOString().replaceAll(':', '-'))
const out = path.join(root, 'tests/reports/business-suite', suiteLock.executionId)
fs.mkdirSync(out, { recursive: true })
const matrix = JSON.parse(fs.readFileSync(path.join(root, 'tests/fixtures/business-coverage.json')))
const steps = [
  { names: ['my-appointments-guest-gate'], script: 'request-history-guest', guest: true, replaySafe: true, dependency: 'No pending business action. Local guest session.' },
  { names: ['home-load'], replaySafe: true, args: ['scripts/test-runner.mjs', 'smoke', 'home.json'], dependency: 'QA A and synthetic listing.' },
  { names: ['property-list'], replaySafe: true, args: ['scripts/test-runner.mjs', 'smoke', 'properties.json'], dependency: 'QA A and synthetic listing.' },
  { names: ['property-detail'], args: ['scripts/test-runner.mjs', 'e2e', 'property-detail-test-backend.json'], dependency: 'QA A and qa-property-1. Expected QA footprint write.' },
  { names: ['appointment-form', 'appointment-time-validation'], args: ['scripts/test-runner.mjs', 'e2e', 'appointment-required-fields.json'], dependency: 'QA A. Synthetic invalid form, no submit request.' },
  { names: ['general-form-validation'], script: 'general-form-validation', dependency: 'QA A bound test phone. Blank region.' },
  { names: ['my-appointments'], script: 'request-history-user', dependency: 'QA A and B. Upserts dedicated synthetic history rows.' },
  { names: ['my-appointments-linked-property'], script: 'request-history-property', dependency: 'History fixtures from prior step. Expected QA footprint write.' },
  { names: ['my-footprints-read'], script: 'footprints-read', replaySafe: true, dependency: 'QA A footprint created in prior step.' },
  { names: ['favorite-add-remove'], script: 'favorites-toggle', dependency: 'A and B start without target favorite. Test removes its own favorite.' },
  { names: ['appointment-property-association'], script: 'admin-appointment-association', dependency: 'QA admin and synthetic history fixtures.' },
  { names: ['appointment-submit'], script: 'appointment-ui', dependency: 'QA A and qa-property-1. No recent appointment. Test creates one record and verifies duplicate submission.' },
]
const selection = process.argv.find(arg => arg.startsWith('--cases='))
const requested = selection?.slice(8).split(',')
const selected = requested ? steps.filter(step => step.names.some(name => requested.includes(name))) : steps
if (requested) assert.ok(requested.length && requested.every(name => selected.some(step => step.names.includes(name))), 'Unknown case selection')
const roleRetry = process.argv.find(arg => arg.startsWith('--role-retry='))
if (roleRetry) assert.ok(!selection, 'Role retry cannot be combined with business selection')
const rolesOnly = Boolean(roleRetry) || process.argv.includes('--roles-only')
const report = { scope: selection ? 'selected' : rolesOnly ? 'permissions' : 'full-business', project, status: 'FAIL', startedAt: new Date().toISOString(), preflight: [], cases: [], recovery: [] }
const full = !selection && !rolesOnly
const visual = process.argv.find(arg => arg.startsWith('--visual='))?.slice(9)
const listingDataMode = process.argv.find(arg => arg.startsWith('--listing-data='))?.slice(15)
assert.ok(!listingDataMode || listingDataMode === 'imported', 'Invalid listing data mode')
assert.ok(!listingDataMode || requested?.length === 1 && requested[0] === 'home-load', 'Imported listing data mode is limited to home-load')
assert.ok(!visual || selection && ['home','core'].includes(visual), 'Invalid selected visual scope')
const plan = executionPlan({full, rolesOnly, roleRetry, visual})
const executionId = path.basename(out)
Object.assign(report, { execution_id: executionId, plan })
const suites = report.suites = Object.fromEntries([
  'build-and-security', 'role-api', 'role-ui', 'business-e2e', 'core-ui', 'extended-ui',
].map(name => [name, { status:'NOT RUN', exitCode:null,
  required: plan.includes(name) }]))
let safe = false
const env = { ...process.env, WECHAT_PROJECT: root, WECHAT_BUILD_OUTPUT: project, WECHATIDE_CLI: 'wechatide', WECHATIDE_CLIENT: 'Codex' }
function run(name, command, args, input, timeout = 240000, extraEnv = {}) {
  const started = Date.now()
  const r = spawnSync(command, args, { cwd: root, ...suiteChildOptions(suiteLock, {...env,...extraEnv}), encoding: 'utf8', input, timeout, maxBuffer: 12 * 1024 * 1024 })
  const log = path.join(out, name + '.log')
  fs.writeFileSync(log, (r.stdout + '\n' + r.stderr + '\n' + (r.error?.message || '')).replace(/eyJ[\w-]+\.[\w-]+\.[\w-]+/g, '[REDACTED JWT]'))
  return { name, command: [command, ...args], exitCode: r.status, startedAt: new Date(started).toISOString(), finishedAt: new Date().toISOString(), durationMs: Date.now() - started, stdout: r.stdout || '', log }
}
function freshEvidence(file, result) {
  if (!file || !fs.existsSync(file)) return null
  const actual = fs.realpathSync(file)
  const reports = fs.realpathSync(path.join(root, 'tests/reports'))
  if (!actual.startsWith(reports + path.sep) || !evidenceIsCurrent(fs.statSync(actual), Date.parse(result.startedAt))) return null
  return actual
}
function suite(name, command, args, extraEnv = {}) {
  const {stdout, ...result} = run(name, command, args, undefined, 600000, extraEnv)
  let evidence
  let details
  try {
    evidence = freshEvidence(JSON.parse(stdout.trim().split('\n').at(-1)).report, result)
    if (evidence) details = JSON.parse(fs.readFileSync(evidence))
  } catch {}
  const status = details?.status === 'INCOMPLETE' || details?.status === 'PASS_WITH_GAPS' ? 'INCOMPLETE' :
    result.exitCode === 0 && details?.status === 'PASS' ? 'PASS' : 'FAIL'
  return {...result, evidence, status, details}
}

const permissionKey = row => [row.method, row.path ?? row.route, row.role, row.scenario].join(':')
function runPermissions(name) {
  if (name === 'role-ui' && process.argv.includes('--simulate-role-ui-failure')) {
    const {details,...injected} = suite(name, process.execPath, ['-e','console.error("Injected Automator failure");process.exitCode=1'])
    Object.assign(suites[name],injected)
    return
  }
  const apiMode = name === 'role-api'
  const first = suite(name, process.execPath, ['tests/e2e/role-permissions.mjs',
    apiMode ? '--api-only' : '--ui-only', ...(!apiMode && roleRetry ? [roleRetry] : [])])
  const runs = [first]
  const field = apiMode ? 'api' : 'ui'
  const firstRows = first.details?.[field] ?? []
  let rows = firstRows.map(row => ({row, result:first}))
  const followupNeeded = apiMode ? firstRows.some(row => row.status === 'NOT_RUN') :
    !roleRetry && firstRows.some(row => row.status === 'FAIL' && row.failureType === 'AUTOMATOR' && !row.scenario)
  if (followupNeeded && first.evidence) {
    const input = path.join(out, name + '-followup-input.json')
    fs.writeFileSync(input, JSON.stringify(apiMode ? first.details : {...first.details, ui:firstRows.map(row =>
      row.status === 'FAIL' && row.failureType !== 'AUTOMATOR' ? {...row,status:'INCOMPLETE'} : row)}))
    const followup = apiMode ? suite('role-api-supplement', process.execPath, ['tests/e2e/role-api-coverage.mjs',input]) :
      suite('role-ui-retry', process.execPath, ['tests/e2e/role-permissions.mjs','--ui-only','--role-retry='+input])
    runs.push(followup)
    const updated = followup.details?.[apiMode ? 'results' : 'ui'] ?? []
    const merged = permissionRows(firstRows, updated, permissionKey)
    rows = merged.map((row,index) => ({row,result:row === firstRows[index] ? first : followup}))
  }
  for (const row of first.details?.isolation ?? []) rows.push({row,result:first})
  const status = rows.some(({row})=>row.status==='FAIL') || runs.some(run=>!run.details) ||
    (first.status==='FAIL' && !firstRows.some(row=>row.status==='FAIL')) ||
    (apiMode && runs[1]?.status==='FAIL') ? 'FAIL' :
    !rows.length || rows.some(({row})=>row.status!=='PASS') ? 'INCOMPLETE' : 'PASS'
  const evidence = path.join(out,name+'-combined.json')
  const attempts = runs.map(({details,...run})=>run)
  const records = rows.map(({row,result})=>testRecord(row,result,name+':'+permissionKey(row),executionId))
  fs.writeFileSync(evidence,JSON.stringify({execution_id:executionId,status,attempts,tests:records},null,2)+'\n')
  Object.assign(suites[name],{status,exitCode:status==='PASS'?0:1,evidence,attempts,records,
    startedAt:first.startedAt,finishedAt:runs.at(-1).finishedAt})
}

function requireStep(name, command, args) {
  const { stdout, ...result } = run(name, command, args)
  report.preflight.push(result)
  assert.equal(result.exitCode, 0, name + ' failed')
  return stdout
}
function cli(name, tool, args = []) {
  const raw = requireStep(name, 'wechatide', ['-c', 'Codex', tool, '--project', project, ...args])
  const reply = JSON.parse(raw)
  assert.ok(reply.ok && reply.result?.success !== false, name)
  return reply.result
}
const api = 'xiaopi-miniapp-test-api-test-1'
const db = 'xiaopi-miniapp-test-mysql-test-1'
function mysql(name, sql) {
  return requireStep(name, 'docker', ['exec', db, 'sh', '-ec', '[ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 -h127.0.0.1 -u"$MYSQL_USER" -N "$MYSQL_DATABASE" -e "$1"', 'sh', sql]).trim()
}
function diagnosis(log) {
  const text = fs.readFileSync(log, 'utf8')
  if (/timeout waiting for automator response|ETIMEDOUT|ECONNREFUSED|Simulator lock exists|security scan failed/i.test(text)) return 'ENVIRONMENT'
  if (/Missing admin appointments response/.test(text)) return 'TEST_EVIDENCE'
  if (/Unexpected recent appointment|Existing QA footprint required|fixture/i.test(text)) return 'FIXTURE_OR_ASSERTION'
  return 'UNCLASSIFIED_ASSERTION'
}
try {
  await executePlan(plan, async name => {
    if (name === 'build-and-security') {
      const applicable = matrix.scenarios.filter(s => s.status !== 'NOT_APPLICABLE')
      assert.deepEqual(steps.flatMap(step => step.names).sort(), applicable.map(s => s.name).sort())
      requireStep('build-and-security', 'npm', ['run', 'test:build-scan'])
      for (const container of [api, db]) {
        assert.equal(requireStep(container, 'docker', ['inspect', '-f', '{{index .Config.Labels "com.docker.compose.project"}}', container]).trim(), 'xiaopi-miniapp-test')
        const networks = JSON.parse(requireStep(container+'-networks', 'docker', ['inspect', '-f', '{{json .NetworkSettings.Networks}}', container]))
        assert.deepEqual(Object.keys(networks), ['xiaopi_miniapp_e2e_test_bridge'])
        const ports = JSON.parse(requireStep(container+'-ports', 'docker', ['inspect', '-f', '{{json .NetworkSettings.Ports}}', container]))
        const published = Object.values(ports).flatMap(binding => binding || [])
        assert.deepEqual(published, container === api ? [{HostIp:'127.0.0.1',HostPort:'18083'}] : [])

      }
      assert.equal(requireStep('api-port','docker',['port',api,'8080/tcp']).trim(), '127.0.0.1:18083')
      requireStep('api-environment','docker',['exec',api,'sh','-ec','[ "$APP_ENV" = test ] && [ "$MYSQL_HOST" = mysql-test ] && [ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && test -z "$WECHAT_MINI_APP_ID$WECHAT_MINI_APP_SECRET"'])
      assert.equal(mysql('qa-users', "SELECT COUNT(*) FROM users WHERE id IN ('qa-user-a','qa-user-b','qa-admin') AND open_id=CONCAT('test:',id) AND status='active';"), '3')
      if (listingDataMode === 'imported') {
        const imported = mysql('imported-properties', "SELECT COUNT(*) FROM properties WHERE id<>'qa-property-1' AND status='available'; SELECT COALESCE(JSON_UNQUOTE(JSON_EXTRACT(config_value,'$.contactPhone')),'') FROM app_configs WHERE config_key='miniapp_public';")
        assert.equal(imported, '4', 'Expected four imported listings and a redacted public contact')
      } else {
        assert.equal(mysql('qa-property', "SELECT COUNT(*) FROM properties WHERE id='qa-property-1' AND status='available';"), '1')
      }
      const health = await fetch('http://127.0.0.1:18083/api/health', {redirect:'error',signal:AbortSignal.timeout(5000)})
      assert.equal(health.status,200)
      assert.equal((await health.json()).data.status,'ok')
      report.preflight.push({name:'api-health',exitCode:0})
      safe = true
      Object.assign(suites['build-and-security'], { status:'PASS', exitCode:0 })
      return
    }
    if (name === 'role-api' || name === 'role-ui') return runPermissions(name)
    if (name === 'core-ui' || name === 'extended-ui') {
      const args = ['tests/ui/core.mjs', ...(name === 'extended-ui' ? ['--extended'] : [])]
      if (name === 'core-ui' && visual === 'home') args.push('--home-only')
      const uiEnv = listingDataMode === 'imported' ? {XIAOPI_UI_LISTING_DATA:'imported'} : {}
      const {details,...result} = suite(name,process.execPath,args,uiEnv)
      Object.assign(suites[name],result,{records:(details?.cases ?? []).map(row=>testRecord(row,result,name+':'+row.name,executionId))})
      return
    }
    try {
      report.window = cli('open-test-project','open_project_window')
      let ready = false
      for (let attempt = 1; attempt <= 3; attempt++) {
        const args = ['--fn-source','function(){return !!(wx.getStorageSync("xiaopi_pending_action") || wx.getStorageSync("xiaopi-avatar-crop-result"))}']
        const { stdout, ...probe } = run('automator-ready-' + attempt, 'wechatide', ['-c','Codex','automation_evaluate','--project',project,...args])
        report.preflight.push(probe)
        const reply = JSON.parse(stdout)
        if (probe.exitCode === 0 && reply.ok) {
          assert.equal(reply.result.result.result, false, 'Pending business action in simulator')
          ready = true
          break
        }
        if (!reply.message?.includes('timeout waiting for automator response')) break
        report.recovery.push({scenario:'automator-ready',category:'ENVIRONMENT',action:'Retried a read-only readiness probe after a bridge timeout.'})
      }
      assert.ok(ready, 'Automator did not become ready')
      for (const [i, step] of selected.entries()) {
        const name = step.names[0]
        console.log('Running ' + step.names.join(', '))
        if (step.guest) cli('prepare-guest','automation_evaluate',['--fn-source','function(){wx.removeStorageSync("xiaopi_access_token");wx.removeStorageSync("xiaopi_session_role");return !wx.getStorageSync("xiaopi_access_token")}'])
        const args = step.args || ['tests/e2e/' + step.script + '.mjs']
        const attempts = []
        for (let attempt = 0; attempt <= 3; attempt++) {
          const label = attempt === 0 ? name : name + '-retry-' + attempt
          const legacyDir = path.join(root,'tests/reports/miniapp')
          const previous = new Set(fs.existsSync(legacyDir) ? fs.readdirSync(legacyDir) : [])
          const runCase = run(label, process.execPath, args)
          let evidence
          if (step.script) {
            try { evidence = JSON.parse(runCase.stdout.trim().split('\n').at(-1)).report } catch {}
          } else if (fs.existsSync(legacyDir)) {
            const created = fs.readdirSync(legacyDir).filter(file => !previous.has(file) && fs.existsSync(path.join(legacyDir,file,'result.json')))
            if (created.length === 1) evidence = path.join(legacyDir,created[0],'result.json')
          }
          const { stdout, ...result } = runCase
          evidence = freshEvidence(evidence, result)
          let assertion
          try { if (evidence) assertion = JSON.parse(fs.readFileSync(evidence)).status } catch {}
          const status = result.exitCode === 0 && ['PASS','passed'].includes(assertion) ? 'PASS' : 'FAIL'
          attempts.push({ ...result, status, evidence })
          if (status === 'PASS') break
          const category = diagnosis(result.log)
          const diagnostics = []
          for (const [tool,args] of [
            ['simulator_screenshot',['--path',path.join(out,label+'-failure.png'),'--optimize=false']],
            ['automation_page_action',['--action','querySelectorAll','--selector','view']],
            ['get_simulator_console',['--command','grep -iE "error|exception|fail"']],
            ['get_simulator_network',['--command','grep -n .']],
          ]) {
            const { stdout, ...probe } = run(label+'-failure-'+tool,'wechatide',['-c','Codex',tool,'--project',project,...args])
            diagnostics.push(probe)
          }
          attempts.at(-1).category = category
          attempts.at(-1).diagnostics = diagnostics
          const transientBridge = /timeout waiting for automator response/i.test(fs.readFileSync(result.log,'utf8'))
          if (!step.replaySafe || !transientBridge || report.recovery.filter(item => item.rebuild).length >= 3 || attempt === 3) {
            report.recovery.push({ scenario:name, category, action:'Evidence captured. No safe automatic replay or source edit.' })
            break
          }
          const recovery = { scenario:name, category, action:'Rebuild and replay a read-only business case after a confirmed bridge timeout.', rebuild:true }
          report.recovery.push(recovery)
          requireStep(label+'-rebuild', 'npm', ['run', 'test:build-scan'])
          recovery.outcome = 'rebuild passed'
        }
        const latest = attempts.at(-1)
        for (const scenario of step.names) report.cases.push({ scenario, dependency:step.dependency, command:args, ...latest, attempts })
        if (latest.status !== 'PASS') {
          for (const remaining of selected.slice(i+1)) for (const scenario of remaining.names) report.cases.push({scenario,status:'NOT RUN',dependency:remaining.dependency})
          break
        }
      }
      const passed = report.cases.length === selected.flatMap(step => step.names).length && report.cases.every(c=>c.status==='PASS')
      Object.assign(suites['business-e2e'], { status:passed?'PASS':'FAIL', exitCode:passed?0:1 })
    } catch (error) {
      Object.assign(suites['business-e2e'], { status:'FAIL', exitCode:1, error:error.message })
    }
  }, (name,error) => {
    Object.assign(suites[name],{status:'FAIL',exitCode:1,error:error.message})
  })
} catch (error) {
  report.error = error.message
  if (!safe) Object.assign(suites['build-and-security'], {status:'FAIL', exitCode:1})
} finally {
  if (suites['business-e2e'].status !== 'NOT RUN') {
    const evidence = path.join(out, 'business-e2e.json')
    fs.writeFileSync(evidence,JSON.stringify({ ...suites['business-e2e'], cases:report.cases },null,2)+'\n')
    suites['business-e2e'].evidence = evidence
  }
  report.status = requiredExit(plan,suites) === 0 ? 'PASS' : 'FAIL'
  for (const item of matrix.scenarios.filter(s => !report.cases.some(c=>c.scenario===s.name))) report.cases.push({scenario:item.name,status:item.status==='NOT_APPLICABLE'?'NOT_APPLICABLE':'NOT RUN'})
  const build = report.preflight.find(step=>step.name==='build-and-security')
  report.tests = [
    testRecord(suites['build-and-security'],build,'build-and-security',executionId),
    ...Object.entries(suites).filter(([name])=>name !== 'build-and-security' && name !== 'business-e2e').flatMap(([name,suite])=>
      [testRecord(suite,suite.startedAt ? suite : suite.attempts?.[0],name,executionId), ...(suite.records ?? [])]),
    ...report.cases.map(row=>testRecord(row,row,'business-e2e:'+row.scenario,executionId)),
  ]
  report.finishedAt = new Date().toISOString()
  report.counts = Object.fromEntries(['PASS', 'FAIL', 'NOT RUN', 'NOT_APPLICABLE'].map(status => [status, report.cases.filter(item => item.status === status).length]))
  fs.writeFileSync(path.join(out,'result.json'),JSON.stringify(report,null,2)+'\n')
  const lines = ['# 微信小程序业务 E2E 汇总', '', report.status + ' · ' + report.scope, report.error || '', '## 环境检查', '',
    ...report.preflight.map(step => '- ' + step.name + ' · exit ' + step.exitCode + (step.log ? ' · [日志](' + path.relative(out,step.log) + ')' : '')),
    '', '## 独立套件', '',
    ...Object.entries(suites).map(([name,item])=>'- '+name+' · '+item.status+' · exit '+(item.exitCode ?? '—')+(item.evidence ? ' · [独立报告]('+path.relative(out,item.evidence)+')' : item.log ? ' · [日志]('+path.relative(out,item.log)+')' : '')),
    '', '## 业务用例', '',
    ...report.cases.map(c => '- ' + c.scenario + ' · ' + c.status + (c.evidence ? ' · [独立结果及同目录证据](' + path.relative(out,c.evidence) + ')' : '')),
    '', '## 故障与恢复', '',
    ...report.recovery.map(r => '- ' + r.scenario + ' · ' + r.category + ' · ' + r.action)]
  fs.writeFileSync(path.join(out,'report.md'),lines.join('\n')+'\n')
  console.log(JSON.stringify({status:report.status,report:path.join(out,'result.json'),error:report.error}))
  if (report.status!=='PASS') process.exitCode = 1
}
