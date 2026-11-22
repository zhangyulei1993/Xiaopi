import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createHash } from 'node:crypto'
import { spawnSync } from 'node:child_process'
import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import { fileURLToPath } from 'node:url'
import { classify } from '../../scripts/classify-smoke-network.mjs'
import { classifyIPhone17Pixel, deviceFrameMasks } from './device-frame.mjs'
import { readPng, comparePixels, exceedsVisualThreshold, checkRect, overlaps } from './pixels.mjs'

const root = fileURLToPath(new URL('../../', import.meta.url))
const project = path.join(root, 'dist/mp-weixin-test')
const reportRoot = path.join(root, 'tests/reports/ui')
const runId = new Date().toISOString()
const suiteLock = enterSuiteLock(runId)
const out = path.join(reportRoot, runId)
const baselineId = process.env.UI_BASELINE || (process.argv.includes('--extended') ? 'extended-v1' : 'core-v1')
assert.match(baselineId, /^[a-z0-9-]+$/)
const baseline = path.join(root, 'tests/ui/baselines', baselineId)
const capture = process.argv.includes('--capture')
const homeOnly = process.argv.includes('--home-only')
const extended = process.argv.includes('--extended')
const listingDataMode = process.env.XIAOPI_UI_LISTING_DATA || ''
assert.ok(!listingDataMode || listingDataMode === 'imported', 'Invalid UI listing data mode')
const expectedDevice = { model:'iPhone 17', screenWidth:402, screenHeight:874, pixelRatio:3, SDKVersion:'3.17.6' }
const masks = [{ x:14, y:35, width:65, height:19, reason:'Simulator status-bar clock only' }]
const report = { runId, execution_id:suiteLock.executionId, project, baselineId, capture, status:'FAIL', device:null, masks, cases:[], commands:[] }
const started = Date.now()
fs.mkdirSync(out, { recursive:true })
let label = 'preflight'
let sequence = 0
let metadata

function run(command, args, retry = false) {
  const result = spawnSync(command, args, { cwd:root, ...suiteChildOptions(suiteLock), encoding:'utf8', timeout:30000, maxBuffer:8*1024*1024 })
  const output = (result.stdout || '') + '\n' + (result.stderr || '')
  const log = path.join(out, label + '-' + (++sequence) + '.log')
  fs.writeFileSync(log, output.replace(/eyJ[\w-]+\.[\w-]+\.[\w-]+/g, '[REDACTED JWT]'))
  report.commands.push({ command, args, exitCode:result.status, log })
  if (result.status !== 0 && retry && /timeout waiting for automator response/.test(output)) return run(command,args)
  assert.equal(result.status,0, log)
  return result.stdout
}
function cli(tool, args = [], retry = false) {
  const reply = JSON.parse(run('wechatide',['-c','Codex',tool,'--project',project,...args],retry))
  assert.ok(reply.ok && reply.result?.success !== false, tool)
  return reply.result
}
function el(action, selector, args = []) {
  return cli('automation_element_action',['--action',action,'--selector',selector,'--wait-for-selector',selector,...args],
    ['size','offset','text','outerWxml','value'].includes(action))
}
function evaluate(source, retry = false) {
  return cli('automation_evaluate',['--fn-source',source],retry).result.result
}
function open(route, query) {
  cli('simulator_open_page',['--page',route,...(query ? ['--query',query] : [])])
}
async function waitForPage(route, selector) {
  const deadline = Date.now() + 12000
  let observed
  do {
    observed = evaluate('function(){return getCurrentPages().at(-1)?.route}',true)
    if (observed === route) {
      try {
        const rect = el('size',selector)
        if (rect.width > 0 && rect.height > 0) return
      } catch (error) {
        const log = error.message.split('\n')[0]
        if (!fs.existsSync(log) || !/page is not on top of page stack|waitForSelector timeout/.test(fs.readFileSync(log,'utf8'))) throw error
      }
    }
    await new Promise(resolve=>setTimeout(resolve,200))
  } while(Date.now()<deadline)
  throw new Error(`Page not ready: ${route} ${selector}; observed ${observed}`)
}

function allowed({detail}) {
  if (extended && detail.url === 'http://127.0.0.1:18083/api/v1/favorites/qa-property-1' &&
      ['PUT','DELETE'].includes(detail.method) && [undefined,200,204].includes(detail.status)) return true
  if (extended && detail.url === 'http://127.0.0.1:18083/uploads/qa-ui-visual.png' &&
      detail.method === 'GET' && [undefined,200,304].includes(detail.status)) return true
  if (classify(detail) !== 'FAIL') return true
  return detail.method === 'POST' &&
    ['http://127.0.0.1:18083/api/v1/auth/wechat/login','http://127.0.0.1:18083/api/v1/footprints/qa-property-1'].includes(detail.url) &&
    [undefined,200,201].includes(detail.status)
}
async function state(name, route, ready, nodes, options = {}) {
  label = name
  const result = { name, route, source: 'src/'+route+'.vue', ready, nodes:{}, findings:[], status:'FAIL',
    masks:[...masks,...(extended ? deviceFrameMasks : []),...(options.masks || [])], actual:path.join(out,name+'.png'), expected:path.join(baseline,name+'.png'), diff:path.join(out,name+'-diff.png') }
  report.cases.push(result)
  try {
    await options.setup?.()
    if (extended) await waitForPage(route,ready)
    el('size',ready)
    const current = cli('automation_runtime_info',['--action','currentPage'],true)
    assert.equal(current.currentPage.path,route)
    result.window = evaluate('function(){const w=wx.getWindowInfo();return {width:w.windowWidth,height:w.windowHeight}}',true)
    const approved = metadata?.images.find(image=>image.name===name)
    if (approved) assert.deepEqual(result.window,approved.window,'Route viewport dimensions changed')
    result.dom = path.join(out,name+'-dom.json')
    fs.writeFileSync(result.dom, JSON.stringify(el('outerWxml',ready),null,2))
    for (const [selector, pattern] of options.text || []) assert.match(el('text',selector),pattern)
    if (options.absent) assert.equal(evaluate(`function(){return new Promise(resolve=>wx.createSelectorQuery().select(${JSON.stringify(options.absent)}).boundingClientRect(resolve).exec())}`,true),null)
    // The detail route uses custom navigation. Native offsets use the full screen
    // there, even when getWindowInfo retains the preceding tab's viewport.
    result.layoutViewport = route === 'pages/properties/detail'
      ? {width:report.device.screenWidth,height:report.device.screenHeight} : result.window
    for (const [selector, visible] of nodes) {
      const {width,height} = el('size',selector)
      const {left,top} = el('offset',selector)
      result.nodes[selector] = {left,top,width,height}
      if (!checkRect(result.nodes[selector],result.layoutViewport.width,result.layoutViewport.height,visible)) result.findings.push({ category:'LAYOUT_CANDIDATE', message:selector+' outside its expected bounds or has abnormal size' })
    }
    options.verify?.(result)
    if (options.pair && overlaps(...options.pair.map(selector=>result.nodes[selector]))) result.findings.push({category:'LAYOUT_CANDIDATE',message:'Critical controls overlap'})
    cli('simulator_screenshot',['--path',result.actual,'--optimize=false'],true)
  } catch (error) {
    result.findings.push({category:/dimensions|ENOENT/.test(error.message)?'ENVIRONMENT_OR_BASELINE':'AUTOMATOR_OR_INTERACTION',message:error.message})
    if (!fs.existsSync(result.actual)) {
      try { cli('simulator_screenshot',['--path',result.actual,'--optimize=false'],true) } catch {}
    }
  } finally {
    try {
      const actual = readPng(result.actual)
      assert.deepEqual([actual.width,actual.height],[636,1382],'Simulator screenshot dimensions changed')
      options.verifyImage?.(actual,result)
      if (!capture) {
        const hash = createHash('sha256').update(fs.readFileSync(result.expected)).digest('hex')
        const entry = metadata.images.find(image=>image.name===name)
        assert.equal(hash,entry?.sha256,'Baseline image integrity check failed')
        assert.deepEqual(result.masks,entry.masks,'Approved visual masks changed')
        // Preserve the approved manifest while using the measured device contour
        // instead of the legacy rectangular frame masks in extended-v1.
        result.appliedMasks = result.masks.filter(mask => !deviceFrameMasks.some(frame =>
          ['x','y','width','height','reason'].every(key => frame[key] === mask[key])))
        const comparison = comparePixels(readPng(result.expected),actual,result.appliedMasks,classifyIPhone17Pixel)
        const { image, ...metrics } = comparison
        result.visual = metrics
        if (Object.values(metrics.categories).some(area=>area.changed) || result.findings.length) fs.writeFileSync(result.diff,image)
        if (exceedsVisualThreshold(metrics)) result.findings.push({category:'VISUAL_DIFFERENCE_REVIEW',message:'Pixel/region threshold exceeded'})
      }
    } catch (error) { result.findings.push({category:'ENVIRONMENT_OR_BASELINE',message:error.message}) }
    const probes = [
      ['console','get_simulator_console','grep -iE "error|exception|fail"'],
      ['network','get_simulator_network','grep -n HTTP_REQUEST'],
      ['network-errors','get_simulator_network','grep -n -E "HTTP_RESPONSE.*status[^0-9]*[45][0-9]{2}"'],
    ]
    for (const [kind,tool,pattern] of probes) {
      try {
        const raw = cli(tool,['--command',pattern],true)
        result[kind] = path.join(out,name+'-'+kind+'.json')
        fs.writeFileSync(result[kind],JSON.stringify(raw.replace(/eyJ[\w-]+\.[\w-]+\.[\w-]+/g,'[REDACTED JWT]'),null,2))
        if (kind === 'console') assert.equal(raw,'','Console errors')
        else {
          const events = raw.split('\n').filter(line=>line && line!=='--').map(line=>JSON.parse(line.replace(/^\d+:/,'')))
          const relevant = events.filter(({type,detail})=>kind==='network' ? type==='HTTP_REQUEST' : type==='HTTP_RESPONSE' && Number(detail.status)>=400)
          assert.ok(relevant.every(allowed),'Unexpected simulator traffic or HTTP failure')
        }
      } catch (error) { result.findings.push({category:'ENVIRONMENT_OR_RUNTIME',message:error.message}) }
    }
    if (!result.findings.length) result.status = capture ? 'CANDIDATE' : 'PASS'
  }
}

try {
  const build = spawnSync('npm',['run','test:build-scan'],{cwd:root,...suiteChildOptions(suiteLock),encoding:'utf8',timeout:180000})
  fs.writeFileSync(path.join(out,'build-and-security.log'),(build.stdout||'')+(build.stderr||''))
  assert.equal(build.status,0,'Test build/security gate failed')
  const api = 'xiaopi-miniapp-test-api-test-1'
  const db = 'xiaopi-miniapp-test-mysql-test-1'
  for (const container of [api,db]) {
    const identity = JSON.parse(run('docker',['inspect','-f','{"owner":{{json (index .Config.Labels "com.docker.compose.project")}},"networks":{{json .NetworkSettings.Networks}},"ports":{{json .NetworkSettings.Ports}}}',container]))
    assert.equal(identity.owner,'xiaopi-miniapp-test')
    assert.deepEqual(Object.keys(identity.networks),['xiaopi_miniapp_e2e_test_bridge'])
    assert.deepEqual(Object.values(identity.ports).flatMap(value=>value||[]),container === api ? [{HostIp:'127.0.0.1',HostPort:'18083'}] : [])
  }
  run('docker',['exec',api,'sh','-ec','[ "$APP_ENV" = test ] && [ "$MYSQL_HOST" = mysql-test ] && [ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && test -z "$WECHAT_MINI_APP_ID$WECHAT_MINI_APP_SECRET"'])
  const sql = listingDataMode === 'imported'
    ? "SELECT COUNT(*) FROM properties WHERE id<>'qa-property-1' AND status='available'; SELECT id FROM users WHERE id='qa-user-a' AND open_id='test:qa-user-a' AND status='active'; SELECT COUNT(*) FROM property_favorites WHERE user_id='qa-user-a'; SELECT JSON_UNQUOTE(JSON_EXTRACT(config_value,'$.contactPhone')) FROM app_configs WHERE config_key='miniapp_public';"
    : "SELECT id,title,rent_monthly FROM properties WHERE id='qa-property-1' AND status='available'; SELECT id FROM users WHERE id='qa-user-a' AND open_id='test:qa-user-a' AND status='active'; SELECT COUNT(*) FROM property_favorites WHERE user_id='qa-user-a';"
  const expected = listingDataMode === 'imported' ? '4\nqa-user-a\n0\n' : 'qa-property-1\tTEST Listing A\t1800\nqa-user-a\n0'
  assert.equal(run('docker',['exec',db,'sh','-ec','[ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -N "$MYSQL_DATABASE" -e "$1"','sh',sql]).trim(),expected.trim())
  const health = await fetch('http://127.0.0.1:18083/api/health',{redirect:'error',signal:AbortSignal.timeout(5000)})
  assert.equal(health.status,200)
  assert.equal((await health.json()).data.status,'ok')
  if (!capture) {
    metadata = JSON.parse(fs.readFileSync(path.join(baseline,'manifest.json')))
    assert.deepEqual(metadata.device,expectedDevice)
    assert.deepEqual(metadata.masks,masks)
  }

  cli('open_project_window')
  assert.equal(evaluate('function(){return !!(wx.getStorageSync("xiaopi_pending_action") || wx.getStorageSync("xiaopi-avatar-crop-result"))}',true),false)
  const device = cli('automation_runtime_info',['--action','systemInfo'],true).systemInfo.result
  report.device = Object.fromEntries(Object.keys(expectedDevice).map(key=>[key,device[key]]))
  assert.deepEqual(report.device,expectedDevice)
  const preflightPath = path.join(out,'screenshot-preflight.png')
  cli('simulator_screenshot',['--path',preflightPath,'--optimize=false'],true)
  const {width,height} = readPng(preflightPath)
  report.screenshotPreflight = {width,height,expectedWidth:636,expectedHeight:1382,file:preflightPath}
  if (width !== 636 || height !== 1382) {
    report.status = 'ENVIRONMENT_BLOCKED'
    throw new Error(`原始截图为 ${width}×${height}，需要 636×1382。请恢复隔离测试窗口全屏及自适应比例。`)
  }
  if (extended) {
    open('pages/auth/login')
    await waitForPage('pages/auth/login','.test-login-panel button')
  } else open('pages/auth/login')
  assert.match(el('text','.test-login-panel button'),/普通用户 A/)
  el('tap','.test-login-panel button')
  el('size','.hero-search')
  if (extended) {
    const { runExtended } = await import('./extended.mjs')
    const retryOnly = process.argv.includes('--failed-seven')
    const selection = new Set(['horizontal-form','detail-with-image','favorite-added','favorite-list','favorite-removed','admin-appointment-list','admin-appointment-scroll'])
    await runExtended({ state:retryOnly ? (...args)=>selection.has(args[0]) && state(...args) : state,
      cli, el, open, waitForPage, evaluate, run, out, db, retryOnly })
  } else {
  await state('home','pages/home/index','.home-masonry-card',[['.hero-search',true]],{absent:'.home-state'})
  if (!homeOnly) {
  await state('list-data','pages/properties/index','.masonry-property',[['.property-listing-title',true],['.masonry-property .favorite',true],['.masonry-property .price',true]],{setup:()=>open('pages/properties/index'),text:[['.property-listing-title',/TEST Community A/],['.masonry-property .price',/1800/]]})
  await state('list-empty','pages/properties/index','.favorites-empty',[['.favorites-empty',true]],{setup:()=>{
    el('tap','.favorite-filter')
    el('size','.favorite-filter.active')
  },text:[['.favorites-empty',/还没有找到关注的房源/]]})
  await state('detail-placeholder','pages/properties/detail','.overview',[['.detail-cover-empty',true],['.overview .title',true],['.overview .price',true],['.overview .muted',true],['.overview .heart',true]],{setup:()=>{ el('tap','.favorite-filter.active'); el('tap','.masonry-property') },text:[['.overview .title',/TEST Community A/],['.overview .price',/1800/],['.overview .muted',/TEST Address/]],pair:['.overview .price','.overview .heart']})
  await state('appointment-open','pages/properties/detail','.appointment-sheet',[['.appointment-sheet',true],['.appointment-field input',true],['.appointment-submit',true]],{setup:()=>{ el('tap','.floating-appointment'); assert.equal(el('value','.appointment-field input'),'测试用户 A') },pair:['.appointment-field input','.appointment-submit']})
  await state('appointment-filled','pages/properties/detail','.appointment-sheet',[['.appointment-times .active',true],['.appointment-submit',true]],{setup:()=>{
    el('trigger','.appointment-times text',['--type','tap'])
    el('input','.appointment-field input',['--value','QA UI'])
    assert.equal(el('value','.appointment-field input'),'QA UI')
  },text:[['.appointment-times .active',/今天方便/]]})
  await state('appointment-closed','pages/properties/detail','.overview',[['.floating-appointment',true]],{
    setup:()=>el('trigger','.appointment-sheet-head > text',['--type','tap']),absent:'.appointment-sheet'
  })
  await state('detail-scrolled','pages/properties/detail','.overview',[['.floating-appointment',true]],{
    setup:()=>{
      const before = el('offset','.overview').top
      cli('automation_viewport_action',['--action','pageScrollTo','--scroll-top','360','--wait-for-selector','.overview'])
      let shifted = false
      for (let attempt=0; attempt<3 && !shifted; attempt++) shifted = el('offset','.overview').top < before-80
      assert.ok(shifted,'Detail did not scroll')
    }
  })
  await state('list-return','pages/properties/index','.masonry-property',[['.property-listing-title',true]],{
    setup:()=>el('tap','.detail-media-back'),absent:'.favorite-filter.active',text:[['.property-listing-title',/TEST Community A/]]
  })
  }
  }
  report.scope = homeOnly ? 'home' : extended ? 'extended' : 'core'
  report.status = report.cases.length > 0 && report.cases.every(item=>item.status === (capture?'CANDIDATE':'PASS')) ? (capture?'CANDIDATE':'PASS')
    : 'FAIL'
} catch (error) { report.error = error.message }
finally {
  report.durationMs = Date.now()-started
  fs.writeFileSync(path.join(out,'result.json'),JSON.stringify(report,null,2)+'\n')
  const lines = ['# UI 质量检测','',report.status+' · '+report.durationMs+' ms',
    report.device ? report.device.model+' · '+report.device.screenWidth+'×'+report.device.screenHeight+' · SDK '+report.device.SDKVersion : '',
    report.screenshotPreflight ? '截图预检 '+report.screenshotPreflight.width+'×'+report.screenshotPreflight.height+' · [原始截图](screenshot-preflight.png)' : '',
    report.error||'', '']
  for (const item of report.cases) {
    const evidence = ['actual','expected','diff','dom','console','network','network-errors']
      .filter(key=>item[key] && fs.existsSync(item[key]))
      .map(key=>'['+key+']('+path.relative(out,item[key])+')')
    lines.push('## '+item.name+' · '+item.status,'',item.route+' · '+item.source,
      '视口 '+(item.window ? item.window.width+'×'+item.window.height : '未读取'),evidence.join(' · '),'',
      ...(item.visual ? ['| 区域 | 差异像素 | 排除像素 |','| --- | --- | --- |',...Object.entries(item.visual.categories).map(([area,counts])=>'| '+area+' | '+counts.changed+' | '+counts.excluded+' |'),''] : []),
      ...item.findings.map(f=>'- '+f.category+' · '+f.message),'')
  }
  fs.writeFileSync(path.join(out,'report.md'),lines.join('\n')+'\n')
  console.log(JSON.stringify({status:report.status,report:path.join(out,'result.json'),error:report.error}))
  if (!['PASS','CANDIDATE'].includes(report.status)) process.exitCode = 1
}
