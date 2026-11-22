import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../../', import.meta.url))
const project = path.join(root, 'dist/mp-weixin-test')
const base = 'http://127.0.0.1:18083'
const out = path.join(root, 'tests/reports/role-permissions', new Date().toISOString().replaceAll(':', '-'))
const matrix = JSON.parse(fs.readFileSync(new URL('../fixtures/role-permission-matrix.json', import.meta.url)))
const report = { status: 'FAIL', api: [], ui: [], isolation: [] }
const sessions = {}
const suiteLock = enterSuiteLock(new Date().toISOString())
fs.mkdirSync(out, { recursive: true })

function run(command, args) {
  const result = spawnSync(command, args, { cwd: root, ...suiteChildOptions(suiteLock), encoding: 'utf8', timeout: 25000, maxBuffer: 12 * 1024 * 1024 })
  if (result.status !== 0) {
    let message
    try { message = JSON.parse(result.stdout).message } catch {}
    throw new Error(message || result.error?.message || result.stderr || command + ' exited ' + result.status)
  }
  return result.stdout
}
function cli(tool, ...args) {
  const reply = JSON.parse(run('wechatide', ['-c', 'Codex', tool, '--project', project, ...args]))
  assert.ok(reply.ok && reply.result?.success !== false, reply.message || tool)
  return reply.result
}
async function api(method, route, session, body) {
  const response = await fetch(base + route, {
    method, redirect: 'error', signal: AbortSignal.timeout(5000),
    headers: { 'Content-Type': 'application/json', ...(session && { Authorization: 'Bearer ' + session.accessToken }) },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  return { status: response.status, body: await response.json() }
}
async function check(rows, item, fn) {
  try { Object.assign(item, await fn(), { status: 'PASS' }) }
  catch (error) {
    Object.assign(item, { status: 'FAIL', reason: error.message,
      failureType: error instanceof assert.AssertionError ? 'ASSERTION' : /timeout|page stack|runtimeid/i.test(error.message) ? 'AUTOMATOR' : 'EXECUTION' })
  }
  rows.push(item)
}
function mysql(sql) {
  return run('docker', ['exec', 'xiaopi-miniapp-test-mysql-test-1', 'sh', '-ec',
    '[ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -BN "$MYSQL_DATABASE" -e "$1"', 'sh', sql]).trim().split('\n').filter(Boolean)
}
const ui = {
  'pages/requests/history': { guest: '.empty.state button', user: '.hero', admin: '.page' },
  'pages/footprints/index': { guest: '.empty.state button', user: '.head', admin: '.page' },
  'pages/admin/index': { guest: '.page', user: '.page', admin: '.tabs' },
  'pages/admin/property-edit': { guest: '.page', user: '.page', admin: '.page' },
  'pages/home/index': { guest: '.home-page', user: '.home-page', admin: '.home-page' },
  'pages/map/index': { guest: '.map-page', user: '.map-page', admin: '.map-page' },
  'pages/properties/index': { guest: '.page', user: '.page', admin: '.page' },
  'pages/me/index': { guest: '.page', user: '.page', admin: '.page' },
  'pages/publish/index': { guest: '.page', user: '.page', admin: '.page' },
  'pages/requests/index': { guest: '.page', user: '.page', admin: '.page' },
  'pages/auth/login': { guest: '.test-login-panel', user: '.test-login-panel', admin: '.test-login-panel' },
  'pages/properties/detail': { guest: '.detail-page', admin: '.detail-page' },
}
const cacheAudit = process.argv.includes('--cache-audit')
const routeAudit = process.argv.includes('--route-audit')
const auditRoutes = {
  'pages/admin/index': { roles: ['guest','user'], endpoints: ['/api/v1/admin/rental-requests','/api/v1/admin/contact-events'] },
  'pages/admin/property-edit': { roles: ['guest','user'], endpoints: ['/api/v1/admin/building-rules?community=TEST%20Community%20A'] },
  'pages/footprints/index': { roles: ['admin'], endpoints: ['/api/v1/footprints'] },
  'pages/requests/index': { roles: ['admin'], endpoints: ['/api/v1/rental-requests/mine'] },
  'pages/requests/history': { roles: ['admin'], endpoints: ['/api/v1/rental-requests/mine'] },
}
const retryArg = process.argv.find(arg => arg.startsWith('--role-retry='))
const previous = retryArg && JSON.parse(fs.readFileSync(path.resolve(root, retryArg.slice('--role-retry='.length))))
const retryCases = previous ? previous.ui.filter(row => row.status === 'FAIL' && !row.scenario) : []
if (previous) assert.ok(retryCases.length && retryCases.every(row => matrix.roles.includes(row.role) &&
  matrix.pages.some(page => page.route === row.route && page.expected[row.role] === row.expected)), 'Invalid retry matrix')
const adminSaveOnly = process.argv.includes('--admin-save-only')
const apiOnly = process.argv.includes('--api-only')
const uiOnly = adminSaveOnly || cacheAudit || routeAudit || process.argv.includes('--ui-only')
try {
  const routes = [...fs.readFileSync(path.join(root, '../backend/internal/httpapi/server.go'), 'utf8').matchAll(/mux\.HandleFunc\("(\w+) ([^"]+)"/g)]
    .map(([, method, route]) => method + ' ' + route)
  assert.deepEqual(matrix.apis.map(row => row.method + ' ' + row.path), routes)
  assert.deepEqual(matrix.pages.map(row => row.route), JSON.parse(fs.readFileSync(path.join(root, 'src/pages.json'))).pages.map(row => row.path))
  run(process.execPath, ['scripts/scan-build-security.mjs', project])
  const apiContainer = 'xiaopi-miniapp-test-api-test-1'
  for (const container of [apiContainer, 'xiaopi-miniapp-test-mysql-test-1']) {
    assert.equal(run('docker', ['inspect', '-f', '{{index .Config.Labels "com.docker.compose.project"}}', container]).trim(), 'xiaopi-miniapp-test')
    assert.deepEqual(Object.keys(JSON.parse(run('docker', ['inspect', '-f', '{{json .NetworkSettings.Networks}}', container]))), ['xiaopi_miniapp_e2e_test_bridge'])
  }
  assert.equal(run('docker', ['port', apiContainer, '8080/tcp']).trim(), '127.0.0.1:18083')
  run('docker', ['exec', apiContainer, 'sh', '-ec', '[ "$APP_ENV" = test ] && [ "$MYSQL_HOST" = mysql-test ] && [ "$MYSQL_DATABASE" = xiaopi_miniapp_test ]'])
  assert.equal((await api('GET', '/api/health')).body.data.status, 'ok')
  for (const [identity, code] of Object.entries({ user: 'qa-user-a', userB: 'qa-user-b', admin: 'qa-admin' })) {
    const { status, body } = await api('POST', '/api/v1/auth/wechat/login', null, { code })
    assert.equal(status, 200)
    assert.ok(body.data.testOnly && body.data.accessToken)
    assert.equal(body.data.role, identity === 'admin' ? 'admin' : 'user')
    sessions[identity] = body.data
  }
  if (!uiOnly) {
    for (const route of matrix.apis) for (const role of matrix.roles) {
      const expected = route.expected[role]
      const item = { method: route.method, path: route.path, role, expected }
      const refresh = route.path === '/api/v1/auth/refresh'
      const dependsOnService = /(?:login|\/phone|\/avatar|\/media\/upload|\/location\/|message-subscriptions)/.test(route.path)
      if ((!refresh && route.probe !== 'GET' && expected === 'ALLOW') ||
          (route.probe === 'NOT_RUN') || route.method === 'DELETE' && !route.path.includes('{') && expected === 'ALLOW') {
        report.api.push({ ...item, status: 'NOT_RUN', reason: dependsOnService ? '需要专用认证或外部服务测试条件' : '需要一次性合成数据及验证清理' })
        continue
      }
      await check(report.api, item, async () => {
        const query = route.method === 'GET' && route.path === '/api/v1/admin/building-rules' ? '?community=TEST%20Community%20A' : ''
        const target = route.path.replace(/\{\w+\}/g, 'qa-role-test-nonexistent') + query
        const response = await api(route.method, target, sessions[role], route.method === 'GET' || route.method === 'DELETE' ? undefined : {})
        item.actual = response.status
        if (refresh && expected === 'ALLOW') assert.ok(response.body.data?.accessToken)
        assert.equal(response.status, expected === 'ALLOW' ? 200 : expected)
        if (expected !== 'ALLOW') assert.equal(response.body.data ?? null, null, '拒绝响应携带业务数据')
        if (route.method === 'GET' && route.path === '/api/v1/properties') {
          const properties = response.body.data
          const fixture = properties.find(property => property.id === 'qa-property-1')
          assert.ok(fixture, 'Missing QA property')
          for (const field of ['buildingNo', 'unitNo', 'roomNo']) {
            assert.equal(Object.hasOwn(fixture, field), role === 'admin', field)
          }
          if (role !== 'admin') assert.ok(properties.every(property => property.status === 'available'))
        }
      })
    }
    for (const spec of [
      { name: 'appointments', route: 'rental-requests/mine', table: 'rental_requests', key: 'id', filter: " AND request_type='appointment'", pick: row => row.id, include: row => row.requestType === 'appointment' },
      { name: 'rental-history', route: 'rental-requests/mine', table: 'rental_requests', key: 'id', filter: " AND request_type<>'appointment'", pick: row => row.id, include: row => row.requestType !== 'appointment' },
      { name: 'favorites', route: 'favorites', table: 'property_favorites', key: 'property_id', pick: row => row },
      { name: 'footprints', route: 'footprints', table: 'user_footprints', key: 'property_id', pick: row => row.propertyId },
    ]) {
      const getExpected = () => Object.fromEntries(['qa-user-a', 'qa-user-b'].map(user => [user,
        mysql(`SELECT ${spec.key} FROM ${spec.table} WHERE user_id='${user}'${spec.filter || ''} ORDER BY 1`)]))
      const item = { scenario: spec.name }
      const fixtures = {
        appointments: ["INSERT IGNORE INTO rental_requests (id,user_id,request_type,property_id,phone,note) VALUES ('qa-role-appointment','qa-user-a','appointment','qa-property-1','13800000001','role-isolation fixture')", "DELETE FROM rental_requests WHERE id='qa-role-appointment' AND user_id='qa-user-a' AND note='role-isolation fixture'"],
        'rental-history': ["INSERT IGNORE INTO rental_requests (id,user_id,request_type,phone,note) VALUES ('qa-role-general','qa-user-a','general','13800000001','role-isolation fixture')", "DELETE FROM rental_requests WHERE id='qa-role-general' AND user_id='qa-user-a' AND note='role-isolation fixture'"],
        favorites: ["INSERT IGNORE INTO property_favorites (user_id,property_id,created_at) VALUES ('qa-user-a','qa-property-1','2025-06-01 00:01:23')", "DELETE FROM property_favorites WHERE user_id='qa-user-a' AND property_id='qa-property-1' AND created_at='2025-06-01 00:01:23'"],
        footprints: ["INSERT IGNORE INTO user_footprints (user_id,property_id,viewed_at,view_count) VALUES ('qa-user-a','qa-property-1','2025-06-01 00:01:23',1)", "DELETE FROM user_footprints WHERE user_id='qa-user-a' AND property_id='qa-property-1' AND viewed_at='2025-06-01 00:01:23' AND view_count=1"],
      }
      await check(report.isolation, item, async () => {
        let expected = getExpected()
        let inserted = false
        const [insert, remove] = fixtures[spec.name]
        try {
          if (JSON.stringify(expected['qa-user-a']) === JSON.stringify(expected['qa-user-b'])) {
            assert.deepEqual(mysql("SELECT COUNT(*) FROM users WHERE id IN ('qa-user-a','qa-user-b') AND open_id=CONCAT('test:',id)"), ['2'])
            assert.deepEqual(mysql("SELECT COUNT(*) FROM properties WHERE id='qa-property-1' AND status='available'"), ['1'])
            inserted = mysql(insert + '; SELECT ROW_COUNT()')[0] === '1'
            expected = getExpected()
          }
          assert.notDeepEqual(expected['qa-user-a'], expected['qa-user-b'], '缺少可区分的 QA 数据')
          item.fixtureCounts = Object.fromEntries(Object.entries(expected).map(([user, ids]) => [user, ids.length]))
          for (const [identity, user] of [['user', 'qa-user-a'], ['userB', 'qa-user-b']]) {
            const { status, body } = await api('GET', '/api/v1/' + spec.route, sessions[identity])
            assert.equal(status, 200)
            assert.deepEqual((body.data || []).filter(spec.include || (() => true)).map(spec.pick).sort(), expected[user])
          }
        } finally {
          if (inserted) assert.deepEqual(mysql(remove + '; SELECT ROW_COUNT()'), ['1'], '未能清理本次插入的 QA 数据')
        }
      })
    }
  }
  if (previous) {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'wechat-role-retry-'))
    const evaluate = (fn, ...args) => cli('automation_evaluate', '--fn-source', fn, ...args).result.result
    const dom = selector => cli('automation_element_action', '--action', 'outerWxml', '--selector', selector, '--wait-for-selector', selector)
    const stack = () => evaluate('function(){return getCurrentPages().map(p=>p.route)}')
    const waitRoute = async (target, row) => {
      const deadline = Date.now() + 10000
      do {
        const routes = stack()
        if (JSON.stringify(routes) !== JSON.stringify(row.stacks.at(-1))) row.stacks.push(routes)
        if (routes.at(-1) === target) return
        await new Promise(resolve => setTimeout(resolve, 200))
      } while (Date.now() < deadline)
      throw new Error('Expected route not observed: ' + target)
    }
    try {
      for (const prior of retryCases) {
        const {role, route, expected} = prior
        const row = {role, route, expected, priorFailure: prior.reason, stacks: [], status: 'INCOMPLETE',
          execution_status: 'INCOMPLETE', assertion_status: 'NOT_RUN', stage: 'reset-window'}
        report.ui.push(row)
        const label = role + '-' + route.replaceAll('/', '-')
        try {
          cli('simulator_open_page', '--page', 'pages/home/index')
          row.stage = 'home-ready'
          dom('.home-page')
          row.stage = 'set-session'
          const argsFile = path.join(dir, 'session.json')
          fs.writeFileSync(argsFile, JSON.stringify([sessions[role]?.role || '', sessions[role]?.accessToken || '']), {mode: 0o600})
          const configured = evaluate(`function(role,token){
            if(wx.getStorageSync("xiaopi_pending_action") || wx.getStorageSync("xiaopi-avatar-crop-result") ||
               wx.getStorageSync("xiaopi-property-image-crop-result")) return false
            for(const [k,v] of [["xiaopi_access_token",token],["xiaopi_session_role",role]]) {
              if(v) wx.setStorageSync(k,v); else wx.removeStorageSync(k)
            }
            wx.setStorageSync("xiaopi_qa_permission_audit",[])
            return true
          }`, '--args-file', argsFile)
          if (!configured) throw new Error('Pending simulator action or crop; refusing navigation')
          row.stage = 'navigate'
          const login = route === 'pages/auth/login'
          const entry = login ? 'pages/requests/history' : route
          const redirect = expected === '返回我的'
          const target = redirect ? 'pages/me/index' : route
          try {
            cli('automation_navigate','--action','navigateTo','--url','/'+entry,'--wait-for-selector','.home-page')
          } catch (error) {
            row.navigationError = error.message
          }
          if (login) {
            row.stage = 'guest-login-prompt'
            await waitRoute(entry, row)
            const button = '.empty.state button'
            const prompt = dom(button)
            fs.writeFileSync(path.join(out,label+'-prompt.json'),JSON.stringify(prompt,null,2))
            if (!/登录/.test(JSON.stringify(prompt))) {
              row.execution_status = 'PASS'
              assert.fail('Guest prompt does not show a login action')
            }
            cli('automation_element_action','--action','tap','--selector',button,'--wait-for-selector',button)
          }
          row.stage = 'final-route'
          await waitRoute(target, row)
          row.stage = 'visible-content'
          const content = dom(login ? '.page' : role === 'admin' ? '.admin-identity' : '.profile')
          row.actual = stack().at(-1)
          fs.writeFileSync(path.join(out,label+'-dom.json'),JSON.stringify(content,null,2))
          cli('simulator_screenshot','--path',path.join(out,label+'.png'),'--optimize=false')
          row.stage = 'permission-evidence'
          if (redirect && auditRoutes[route]) {
            const timeline = evaluate('function(){return wx.getStorageSync("xiaopi_qa_permission_audit")}')
            row.timeline = Array.isArray(timeline) ? timeline.filter(e => e.route === route) : []
            if (!row.timeline.some(e => e.event === 'guard') ||
                !row.timeline.some(e => e.event === 'render-state')) throw new Error('Missing guard or render evidence')
          }
          row.execution_status = 'PASS'
          assert.equal(row.actual, target)
          if (login) {
            assert.match(JSON.stringify(content), /微信登录/)
            assert.match(JSON.stringify(content), /隔离测试身份/)
          }
          if (redirect && auditRoutes[route]) {
            assert.ok(row.timeline.some(e => e.event === 'guard' && e.state === false), 'Denied guard not observed')
            assert.ok(!row.timeline.some(e => ['private-read','api-call'].includes(e.event) ||
              (['render-state','private-binding'].includes(e.event) && e.state === true)), 'Denied page touched or bound private data')
          }
          row.status = row.assertion_status = 'PASS'
        } catch (error) {
          row.reason = error.message
          if (row.execution_status === 'PASS' && error instanceof assert.AssertionError) {
            row.status = row.assertion_status = 'FAIL'
          } else {
            row.execution_status = 'INCOMPLETE'
          }
          try { cli('simulator_screenshot','--path',path.join(out,label+'-failure.png'),'--optimize=false') } catch {}
        }
      }
    } finally { fs.rmSync(dir,{recursive:true,force:true}) }
  } else if (!apiOnly) {
  cli('simulator_open_page', '--page', 'pages/home/index')
  const pendingProbe = () => cli('automation_evaluate', '--fn-source', 'function(){return !!wx.getStorageSync("xiaopi_pending_action")}')
  let pending
  try { pending = pendingProbe() }
  catch (error) {
    if (!error.message.includes('timeout waiting for automator response')) throw error
    report.readinessRetry = error.message
    pending = pendingProbe()
  }
  assert.equal(pending.result.result, false, 'Pending business action')
  const secretDir = fs.mkdtempSync(path.join(os.tmpdir(), 'wechat-role-'))
  try {
    if (cacheAudit) {
      const crop = 'xiaopi-property-image-crop-result'
      const phone = 'xiaopi_request_bound_phone'
      for (const scenario of [
        {name:'admin-request-denied',role:'admin',route:'pages/requests/index',key:phone,owner:'user',denied:true},
        {name:'guest-editor-denied',role:'guest',route:'pages/admin/property-edit',key:crop,owner:'admin',denied:true},
        {name:'user-editor-denied',role:'user',route:'pages/admin/property-edit',key:crop,owner:'admin',denied:true},
        {name:'admin-editor',role:'admin',route:'pages/admin/property-edit',key:crop,owner:'admin',edit:true},
        {name:'owner-phone',role:'user',route:'pages/requests/index',key:phone,owner:'user',expectRead:true},
        {name:'switch-to-user-b',role:'userB',route:'pages/requests/index',key:phone,owner:'user'},
      ]) {
        const {name,role,route,key,owner,denied,edit,expectRead} = scenario
        const item = {name,role,route,stage:"open-home"}
        await check(report.ui,item,async()=>{
          cli('simulator_open_page','--page','pages/home/index')
          item.stage = 'home-ready'
          cli('automation_element_action','--action','size','--selector','.home-page','--wait-for-selector','.home-page')
          const argsFile = path.join(secretDir,'cache-args.json')
          fs.writeFileSync(argsFile,JSON.stringify([sessions[role]?.role || '',sessions[role]?.accessToken || '',key,sessions[owner].accessToken,key===crop?{index:999,path:'qa-synthetic-crop'}:'13800009999']),{mode:0o600})
          let timeline
          let seeded = false
          try {
            item.stage = "seed-cache"
            cli('automation_evaluate','--fn-source',`function(role,token,key,owner,marker){
              const keys=['xiaopi_access_token','xiaopi_session_role','xiaopi_qa_permission_audit',key,key+':owner','xiaopi_bound_phone']
              wx.setStorageSync('xiaopi_cache_audit_backup',keys.map(key=>[key,wx.getStorageSync(key)]))
              if(token){wx.setStorageSync(keys[0],token);wx.setStorageSync(keys[1],role)}
              else{wx.removeStorageSync(keys[0]);wx.removeStorageSync(keys[1])}
              wx.setStorageSync(keys[2],[])
              wx.setStorageSync(key,marker)
              wx.setStorageSync(key+':owner',owner)
              return true
            }`,'--args-file',argsFile)
            seeded = true
            item.stage = 'navigate'
            if (denied) {
              cli('automation_evaluate','--fn-source',`function(){setTimeout(()=>wx.navigateTo({url:${JSON.stringify('/'+route)}}),0);return true}`)
            } else {
              cli('simulator_open_page','--page',route,...(edit?['--query','id=qa-property-1']:[]))
            }
            item.stage = 'capture'
            const screenshot=path.join(out,name+'.png')
            cli('simulator_screenshot','--path',screenshot,'--optimize=false')
            item.stage = 'timeline'
            timeline=cli('automation_evaluate','--fn-source','function(){return wx.getStorageSync("xiaopi_qa_permission_audit")}').result.result
            fs.writeFileSync(path.join(out,name+'-timeline.json'),JSON.stringify(timeline,null,2))
            const events=timeline.filter(event=>event.route===route)
            assert.ok(events.some(event=>event.event==='guard' && event.state===!denied),'Missing expected role guard')
            const reads=events.filter(event=>event.event==='private-read')
            if (denied) {
              assert.equal(reads.length,0)
              assert.ok(!events.some(event=>['private-binding','render-state'].includes(event.event)&&event.state===true))
              assert.ok(events.some(event=>event.event==='redirect'&&event.state==='pages/me/index'))
            } else if (edit) {
              assert.ok(reads.some(event=>event.source==='property-crop-cache'&&event.state===true))
              const selector='.rent-input input'
              cli('automation_element_action','--action','input','--selector',selector,'--value','1801','--wait-for-selector',selector)
              const observed=cli('automation_element_action','--action','value','--selector',selector)
              assert.match(JSON.stringify(observed),/1801/)
            } else if (expectRead) {
              assert.ok(reads.some(event=>event.source==='bound-phone-cache'&&event.state===true))
            } else {
              assert.equal(reads.length,0,'Other account cache payload was read')
            }
            return {screenshot,timeline:path.join(out,name+'-timeline.json'),saved:false}
          } finally {
            if (seeded) try { cli('automation_evaluate','--fn-source',`function(){
              const backup=wx.getStorageSync('xiaopi_cache_audit_backup')||[]
              for(const [key,value] of backup){if(value===''||value==null)wx.removeStorageSync(key);else wx.setStorageSync(key,value)}
              wx.removeStorageSync('xiaopi_cache_audit_backup')
              return true
            }`) } catch (error) { item.cleanupError = error.message; throw error }
          }
        })
      }
    } else if (!adminSaveOnly) for (const role of matrix.roles) {
      const argsFile = path.join(secretDir, 'session.json')
      fs.writeFileSync(argsFile, JSON.stringify([role, sessions[role]?.accessToken || '']), { mode: 0o600 })
      cli('automation_evaluate', '--fn-source', 'function(role,token){if(token){wx.setStorageSync("xiaopi_access_token",token);wx.setStorageSync("xiaopi_session_role",role)}else{wx.removeStorageSync("xiaopi_access_token");wx.removeStorageSync("xiaopi_session_role")}return true}', '--args-file', argsFile)
      for (const { route, expected } of matrix.pages) {
        const item = { route, role, expected: expected[role] }
        if (routeAudit && !auditRoutes[route]?.roles.includes(role)) continue
        const selector = ui[route]?.[role]
        if (!selector) {
          report.ui.push({ ...item, status: 'NOT_RUN', reason: route.endsWith('image-crop') ? '缺少一次性房源图片和管理员编辑器传入的裁剪上下文' : route.endsWith('avatar-crop') ? '缺少一次性头像图片和头像选择器传入的裁剪上下文' : '缺少普通用户访问详情时自动写入足迹的一次性房源及清理条件' })
          continue
        }
        await check(report.ui, item, async () => {
          const label = role + '-' + route.replaceAll('/', '-')
          const url = '/' + route + (route === 'pages/properties/detail' ? '?id=qa-property-1' : '')
          const redirect = expected[role] === '返回我的'
          const target = redirect ? 'pages/me/index' : route
          const ready = redirect ? (role === 'admin' ? '.admin-identity' : '.profile') : selector
          const listing = route === 'pages/properties/index'
          if (redirect || listing) {
            if (redirect) {
              cli('simulator_open_page', '--page', 'pages/home/index')
              if (routeAudit) cli('automation_element_action', '--action', 'size', '--selector', '.home-page', '--wait-for-selector', '.home-page')
            }
            const readNetwork = () => cli('get_simulator_network','--command','grep -n -E "HTTP_REQUEST|HTTP_RESPONSE"')
            const baselineNetwork = routeAudit ? readNetwork() : ''
            if (routeAudit) {
              cli('automation_evaluate','--fn-source',`function(){
                wx.setStorageSync("xiaopi_qa_permission_audit",[])
                if (${route === 'pages/requests/index'}) {
                  globalThis.__qaAuditPreviousPhone=wx.getStorageSync("xiaopi_bound_phone")
                  wx.setStorageSync("xiaopi_bound_phone","13800000001")
                }
                return true
              }`)
            }
            const navigation = routeAudit ? cli('automation_evaluate','--fn-source',`function(){
              const stacks=[getCurrentPages().map(page=>page.route)]
              setTimeout(()=>wx.navigateTo({url:${JSON.stringify(url)}}),0)
              return {stacks,actual:stacks[0].at(-1)}
            }`).result.result : { stacks: [], actual: null }
            if (!routeAudit) {
              const start = cli('automation_evaluate', '--fn-source', `function(){
                const stack=getCurrentPages().map(page=>page.route)
                setTimeout(()=>wx.${listing?'switchTab':'navigateTo'}({url:${JSON.stringify(url)}}),0)
                return stack
              }`).result.result
              navigation.stacks.push(start)
              const deadline = Date.now() + 10000
              do {
                const stack = cli('automation_evaluate', '--fn-source', 'function(){return getCurrentPages().map(page=>page.route)}').result.result
                if (JSON.stringify(stack) !== JSON.stringify(navigation.stacks.at(-1))) navigation.stacks.push(stack)
                navigation.actual = stack.at(-1)
                if (navigation.actual === target && (start.at(-1) !== target || navigation.stacks.length > 1)) break
                await new Promise(resolve => setTimeout(resolve, 200))
              } while (Date.now() < deadline)
              assert.ok(navigation.stacks.length > 1 || start.at(-1) !== target, 'Navigation transition was not observed')
            }
            item.navigation = navigation
            item.actual = navigation.actual
            if (routeAudit) {
              item.evidence = {}
              const collect = async (name, read) => {
                try { item.evidence[name] = await read() }
                catch (error) { item.evidence[name] = { error: error.message } }
              }
              await collect('screenshot', () => {
                const file = path.join(out,label+'-audit.png')
                cli('simulator_screenshot','--path',file,'--optimize=false')
                return file
              })
              await collect('finalStack', () => cli('automation_evaluate','--fn-source','function(){return getCurrentPages().map(page=>page.route)}').result.result)
              await collect('timeline', () => cli('automation_evaluate','--fn-source',`function(){
                const trace=wx.getStorageSync("xiaopi_qa_permission_audit")
                wx.removeStorageSync("xiaopi_qa_permission_audit")
                if (${route === 'pages/requests/index'}) {
                  const old=globalThis.__qaAuditPreviousPhone
                  if(old)wx.setStorageSync("xiaopi_bound_phone",old)
                  else wx.removeStorageSync("xiaopi_bound_phone")
                  delete globalThis.__qaAuditPreviousPhone
                }
                return trace
              }`).result.result)
              await collect('network', () => {
                const raw = readNetwork()
                const previous = baselineNetwork.trim().split('\n').filter(Boolean)
                const current = raw.trim().split('\n').filter(Boolean)
                const continuous = previous.every((line,index)=>line===current[index])
                const events = continuous ? current.slice(previous.length) : []
                const privateEvents = events.map(line=>JSON.parse(line.replace(/^\d+:/,'')))
                  .filter(event=>auditRoutes[route].endpoints.some(endpoint=>event.detail?.url?.startsWith(base+endpoint.split('?')[0])))
                                    .map(event=>({type:event.type,url:event.detail.url,method:event.detail.method,status:event.detail.status,
                    response:event.type==='HTTP_RESPONSE' ? event.detail.response ?? event.detail.body ?? null : undefined}))
                return {continuous,observedEvents:events.length,privateEvents}

              })
              if (route === 'pages/requests/index') await collect('submitProbe', async () => {
                const query = "SELECT COUNT(*) FROM rental_requests"
                const before = mysql(query)
                const guest = await api('POST','/api/v1/rental-requests',undefined,{requestType:'invalid'})
                const user = await api('POST','/api/v1/rental-requests',sessions.user,{requestType:'invalid'})
                return {before,after:mysql(query),guest,user}
              })
              await collect('api', async () => {
                const results=[]
                for (const endpoint of auditRoutes[route].endpoints) {
                  const response = await api('GET',endpoint,sessions[role])
                  results.push({endpoint,status:response.status,body:response.body})
                }
                return results
              })
            }
            if (routeAudit) {
              try {
                item.evidence.nativeRoute = cli('automation_runtime_info','--action','currentPage').currentPage.path
              } catch (error) { item.evidence.nativeRoute = { error:error.message } }
            }
            assert.equal(routeAudit ? item.evidence.nativeRoute : navigation.actual, target)
          } else {
            cli('simulator_open_page', '--page', route, ...(route === 'pages/properties/detail' ? ['--query', 'id=qa-property-1'] : []))
          }
          if (routeAudit) {
            const timeline = item.evidence.timeline
            assert.ok(Array.isArray(timeline) && timeline.some(event=>event.route===route && event.event==='guard' && event.state===false), 'Missing denied guard lifecycle evidence')
            const reads = timeline.filter(event=>event.route===route && event.event==='private-read')
            const rendered = timeline.filter(event=>event.route===route && event.event==='render-state')
            const privateCalls = timeline.filter(event=>event.route===route && event.event==='api-call')
            assert.ok(!rendered.some(event=>event.state===true), 'Private data bound to rendered state')
            assert.ok(!reads.length && !privateCalls.length && rendered.length, 'Incomplete private data lifecycle evidence')
            return { actual: target }
          }
          const dom = cli('automation_element_action', '--action', 'outerWxml', '--selector', ready, '--wait-for-selector', ready)
          const actual = cli('automation_runtime_info', '--action', 'currentPage').currentPage.path
          item.actual = actual
          assert.equal(actual, target)
          fs.writeFileSync(path.join(out, label + '-dom.json'), JSON.stringify(dom, null, 2))
          if (expected[role] === '登录引导') assert.match(JSON.stringify(dom), /微信登录/)
          if (role === 'guest' && route === 'pages/requests/history') {
            cli('automation_element_action', '--action', 'tap', '--selector', selector, '--wait-for-selector', selector)
            cli('automation_element_action', '--action', 'size', '--selector', '.test-login-panel', '--wait-for-selector', '.test-login-panel')
          }
          if (redirect || expected[role] === '登录引导' || route === 'pages/admin/index') {
            cli('simulator_screenshot', '--path', path.join(out, label + '.png'), '--optimize=false')
          }
          return { actual, dom: label + '-dom.json' }
        })
      }
    }
    if (!cacheAudit && !routeAudit) {
      await check(report.ui, { role: 'admin', route: 'pages/admin/property-edit', scenario: 'save-isolated-draft' }, async () => {
        const id = randomUUID()
        const where = `id='${id}' AND owner_id='qa-admin' AND status='draft'`
        const fixture = mysql(`INSERT INTO properties (id,title,community,location,layout,rent_monthly,description,status,owner_id)
          VALUES ('${id}','QA permission save','QA permission save','QA Address','1bd',1800,'','draft','qa-admin'); SELECT ROW_COUNT()`)
        assert.deepEqual(fixture, ['1'])
        try {
          const argsFile = path.join(secretDir, 'admin-save.json')
          fs.writeFileSync(argsFile, JSON.stringify([sessions.admin.accessToken]), { mode: 0o600 })
          cli('automation_evaluate', '--fn-source', 'function(token){wx.setStorageSync("xiaopi_access_token",token);wx.setStorageSync("xiaopi_session_role","admin");return true}', '--args-file', argsFile)
          const crop = cli('automation_evaluate', '--fn-source', 'function(){return !!wx.getStorageSync("xiaopi-property-image-crop-result")}').result.result
          assert.equal(crop, false, 'Unexpected pending crop')
          cli('simulator_open_page', '--page', 'pages/admin/property-edit', '--query', 'id=' + id)
          const el = (action, selector, ...args) => cli('automation_element_action', '--action', action, '--selector', selector, '--wait-for-selector', selector, ...args)
          const hero = el('outerWxml', '.editor-hero')
          assert.match(JSON.stringify(hero), /完善房源信息/)
          const loaded = cli('automation_evaluate', '--fn-source', 'function(){const p=getCurrentPages().at(-1);return p?.options?.id ?? p?.$page?.options?.id}').result.result
          assert.equal(loaded, id, 'Unexpected editor target')
          const address = 'input[placeholder="选择小区后自动回填，可补充门牌信息"]'
          assert.match(JSON.stringify(el('value', address)), /QA Address/, 'QA draft address did not load')
          el('input', address, '--value', 'QA Address updated')
          assert.match(JSON.stringify(el('value', address)), /QA Address updated/)
          assert.deepEqual(mysql(`SELECT location FROM properties WHERE ${where}`), ['QA Address'])
          el('tap', '.step-save-button')
          const deadline = Date.now() + 10000
          let observed
          do {
            observed = mysql(`SELECT location FROM properties WHERE ${where}`)
            if (observed[0] === 'QA Address updated') break
            await new Promise(resolve => setTimeout(resolve, 250))
          } while (Date.now() < deadline)
          assert.deepEqual(observed, ['QA Address updated'])
          const screenshot = path.join(out, 'admin-save-draft.png')
          cli('simulator_screenshot', '--path', screenshot, '--optimize=false')
          return { database: { before: 'QA Address', after: 'QA Address updated', status: 'draft' }, fixture: id, screenshot }
        } catch (error) {
          try { cli('simulator_screenshot', '--path', path.join(out, 'admin-save-draft-failure.png'), '--optimize=false') } catch {}
          throw error
        } finally {
          assert.deepEqual(mysql(`DELETE FROM properties WHERE ${where}; SELECT ROW_COUNT()`), ['1'], 'QA draft cleanup failed')
        }
      })
    }
  } finally { fs.rmSync(secretDir, { recursive: true, force: true }) }
  }
  const rows = [...report.api, ...report.ui, ...report.isolation]
  report.status = rows.some(row => row.status === 'FAIL') ? 'FAIL' : rows.some(row => row.status === 'INCOMPLETE') ? 'INCOMPLETE' : rows.some(row => row.status === 'NOT_RUN') ? 'PASS_WITH_GAPS' : 'PASS'
} catch (error) {
  report.error = error.message
} finally {
  for (const page of previous || apiOnly || cacheAudit || adminSaveOnly ? [] : matrix.pages) for (const role of matrix.roles) {
    if (routeAudit && !auditRoutes[page.route]?.roles.includes(role)) continue
    if (!report.ui.some(row => row.route === page.route && row.role === role)) {
      report.ui.push({ route: page.route, role, expected: page.expected[role], status: 'NOT_RUN', reason: report.error || '未执行' })
    }
  }
  const result = path.join(out, 'result.json')
  report.counts = Object.fromEntries(['api','ui','isolation'].map(key => [key, Object.fromEntries(
    ['PASS','FAIL','INCOMPLETE','NOT_RUN'].map(status => [status, report[key].filter(row => row.status === status).length]))]))
  fs.writeFileSync(result, JSON.stringify(report, null, 2) + '\n')
  const rows = [...report.api, ...report.ui, ...report.isolation]
  fs.writeFileSync(path.join(out, 'report.md'), ['# 三身份权限验收', '', report.status, report.error || '', '',
    ...rows.map(row => '- ' + [row.role, row.method, row.path || row.route || row.scenario, row.status, row.reason].filter(Boolean).join(' · '))].join('\n') + '\n')
  if (previous) {
    const byCase = new Map(report.ui.map(row => [row.role + ':' + row.route, row]))
    const merged = previous.ui.map(row => row.status === 'FAIL' && !row.scenario
      ? byCase.get(row.role + ':' + row.route) || {...row,status:'INCOMPLETE',execution_status:'NOT_RUN',assertion_status:'NOT_RUN',reason:report.error || 'Retry did not execute'}
      : {...row, carried_forward:true})
    const counts = Object.fromEntries([['API',previous.api],['页面',merged]].map(([scope, rows]) => [scope,
      Object.fromEntries(matrix.roles.map(role => [role, Object.fromEntries(['PASS','FAIL','INCOMPLETE','NOT_RUN'].map(status =>
        [status, rows.filter(row => row.role === role && row.status === status).length]))]))]))
    const {suites: previousSuites, ...coverage} = previous
    fs.writeFileSync(path.join(out,'role-permission-matrix.json'),JSON.stringify({...coverage,counts,ui:merged,retryStatus:report.status,
      retryEvidence:result,previousEvidence:retryArg.slice('--role-retry='.length)},null,2)+'\n')
    fs.writeFileSync(path.join(out,'role-permission-report.md'), ['# 页面权限失败复验','', ...merged.map(row =>
      '- '+[row.role,row.route,row.status,row.execution_status,row.assertion_status,row.stage,row.carried_forward?'沿用原结果，未复跑':row.reason].filter(Boolean).join(' · '))].join('\n')+'\n')
  }
  console.log(JSON.stringify({ status: report.status, report: result, error: report.error }))
  process.exitCode = ['FAIL','INCOMPLETE'].includes(report.status) ? 1 : 0
}
