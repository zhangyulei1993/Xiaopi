import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'

const suiteLock = enterSuiteLock(new Date().toISOString())
const source = process.argv[2]
const matrix = JSON.parse(fs.readFileSync(source))
const out = path.join('tests/reports/role-permissions', new Date().toISOString().replaceAll(':','-') + '-api')
fs.mkdirSync(out,{recursive:true})
const report = { source, status:'INCOMPLETE', results:[], cleanup:[] }
const db='xiaopi-miniapp-test-mysql-test-1', server='xiaopi-miniapp-test-api-test-1'
const docker=(...args)=>execFileSync('docker',args,{...suiteChildOptions(suiteLock),encoding:'utf8',timeout:10000}).trim()
const sql=query=>docker('exec',db,'sh','-ec',
  '[ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -BN "$MYSQL_DATABASE" -e "$1"','sh',query)
const count=(table,where)=>Number(sql(`SELECT COUNT(*) FROM ${table} WHERE ${where}`))
const sessions={}
async function api(role,method,route,body) {
  const response=await fetch('http://127.0.0.1:18083'+route,{method,redirect:'error',signal:AbortSignal.timeout(5000),
    headers:{'Content-Type':'application/json',...(sessions[role]?{Authorization:'Bearer '+sessions[role]}:{})},
    body:body===undefined?undefined:JSON.stringify(body)})
  return {status:response.status, body:await response.json()}
}
const pending=matrix.api.filter(row=>row.status==='NOT_RUN')
const key=(method,route,role)=>[method,route,role].join(' ')
const checks=new Map()
function test(method,route,role,status,body,verify,target=route) {
  checks.set(key(method,route,role),async row=>{
    const res=await api(role,method,target,body)
    row.httpStatus=res.status
    assert.equal(res.status,status,res.body.message)
    if(status<300) assert.equal(res.body.code,0)
    await verify?.(res.body)
  })
}
const id=randomUUID(), marker='qa-api-'+id, live=randomUUID()
let draft
const cleanups=[]
function cleanup(table,where) {
  cleanups.push({table,where})
}
try {
  for(const container of [server,db]) {
    const info=JSON.parse(docker('inspect',container))[0]
    assert.equal(info.Config.Labels['com.docker.compose.project'],'xiaopi-miniapp-test')
    assert.deepEqual(Object.keys(info.NetworkSettings.Networks),['xiaopi_miniapp_e2e_test_bridge'])
    assert.deepEqual(Object.values(info.NetworkSettings.Ports).flat().filter(Boolean),
      container===server?[{HostIp:'127.0.0.1',HostPort:'18083'}]:[])
  }
  docker('exec',server,'sh','-ec','[ "$APP_ENV" = test ] && [ "$MYSQL_HOST" = mysql-test ] && [ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && test -z "$WECHAT_MINI_APP_ID$WECHAT_MINI_APP_SECRET"')
  assert.equal((await api('guest','GET','/api/health')).body.data.status,'ok')
  assert.equal(count('users',"id IN ('qa-admin','qa-user-a') AND open_id=CONCAT('test:',id) AND status='active'"),2)
  for(const [role,code] of [['user','qa-user-a'],['admin','qa-admin']]) {
    const res=await api('guest','POST','/api/v1/auth/wechat/login',{code})
    assert.equal(res.status,200)
    assert.equal(res.body.data.role,role)
    assert.equal(res.body.data.testOnly,true)
    sessions[role]=res.body.data.accessToken
  }
  for(const role of ['guest','user','admin']) test('POST','/api/v1/auth/wechat/login',role,200,{code:'qa-user-a'},body=>{
    assert.equal(body.data.testOnly,true)
    assert.equal(body.data.role,'user')
  })
  cleanup('properties',`community IN ('${marker}','${marker}-live')`)
  sql(`INSERT INTO properties (id,title,community,location,layout,rent_monthly,description,status,owner_id)
       VALUES ('${live}','${marker}','${marker}-live','QA','QA',1800,'','available','qa-admin')`)
  cleanup('property_favorites',`property_id='${live}'`)
  cleanup('user_footprints',`property_id='${live}'`)
  cleanup('property_contact_events',`property_id='${live}'`)
  const property={title:marker,community:marker,rentMonthly:1800}
  test('POST','/api/v1/properties','admin',201,property,body=>{
    draft=body.data.id
    assert.match(draft,/^[0-9a-f-]{36}$/i)
    assert.equal(count('properties',`id='${draft}' AND community='${marker}' AND status='draft'`),1)
    cleanup('operation_logs',`target_id='${draft}'`)
  })
  checks.set(key('PATCH','/api/v1/properties/update','admin'),async row=>{
    assert.ok(draft,'Draft creation prerequisite')
    const res=await api('admin','PATCH','/api/v1/properties/update',{...property,id:draft,rentMonthly:1801})
    row.httpStatus=res.status
    assert.equal(res.status,200)
    assert.equal(count('properties',`id='${draft}' AND community='${marker}' AND rent_monthly=1801`),1)
  })
  checks.set(key('PATCH','/api/v1/properties/media','admin'),async row=>{
    assert.ok(draft,'Draft creation prerequisite')
    const res=await api('admin','PATCH','/api/v1/properties/media',{propertyId:draft,mediaIds:[]})
    row.httpStatus=res.status
    assert.equal(res.status,200)
    assert.equal(count('property_media',`property_id='${draft}'`),0)
  })
  checks.set(key('DELETE','/api/v1/properties/{propertyID}','admin'),async row=>{
    assert.ok(draft,'Draft creation prerequisite')
    assert.equal(count('properties',`id='${draft}' AND community='${marker}' AND status='draft'`),1)
    const res=await api('admin','DELETE','/api/v1/properties/'+draft)
    row.httpStatus=res.status
    assert.equal(res.status,200)
    assert.equal(count('properties',`id='${draft}'`),0)
  })
  for(const role of ['guest','user']) {
    const community=marker+'-'+role
    cleanup('property_leads',`community='${community}'`)
    test('POST','/api/v1/property-leads',role,201,{community,note:marker},body=>{
      assert.equal(body.data.community,community)
      assert.equal(count('property_leads',`community='${community}' AND note='${marker}'`),1)
    })
  }
  test('PUT','/api/v1/favorites/{propertyID}','user',200,undefined,()=>{
    assert.equal(count('property_favorites',`property_id='${live}' AND user_id='qa-user-a'`),1)
  },'/api/v1/favorites/'+live)
  test('DELETE','/api/v1/favorites/{propertyID}','user',200,undefined,()=>{
    assert.equal(count('property_favorites',`property_id='${live}'`),0)
  },'/api/v1/favorites/'+live)
  test('POST','/api/v1/footprints/{propertyID}','user',200,undefined,()=>{
    assert.equal(count('user_footprints',`property_id='${live}' AND user_id='qa-user-a'`),1)
  },'/api/v1/footprints/'+live)
  test('POST','/api/v1/contact-events','user',201,{propertyId:live,type:'consult_entry_click',source:'qa-permission-test'},()=>{
    assert.equal(count('property_contact_events',`property_id='${live}' AND user_id='qa-user-a'`),1)
  })
  cleanup('building_rules',`community='${marker}' AND building_no='QA1'`)
  cleanup('operation_logs',`target_id='${marker}/QA1'`)
  test('PUT','/api/v1/admin/building-rules','admin',200,{community:marker,buildingNo:'QA1',unitCount:1,floorCount:2,householdsPerFloor:2},()=>{
    assert.equal(count('building_rules',`community='${marker}' AND building_no='QA1' AND floor_count=2`),1)
  })
  const lead=randomUUID(), request=randomUUID()
  cleanup('property_leads',`id='${lead}' AND community='${marker}'`)
  cleanup('rental_requests',`id='${request}' AND note='${marker}'`)
  cleanup('operation_logs',`target_id IN ('${lead}','${request}')`)
  cleanup('property_status_history',`property_id='${live}'`)
  cleanup('operation_logs',`target_id='${live}'`)
  sql(`INSERT INTO property_leads (id,community,note) VALUES ('${lead}','${marker}','${marker}');
       INSERT INTO rental_requests (id,user_id,request_type,phone,note) VALUES ('${request}','qa-user-a','general','13800000001','${marker}')`)
  test('PATCH','/api/v1/property-leads/follow-up','admin',200,{id:lead,status:'contacted',followUpNote:marker},()=>{
    assert.equal(count('property_leads',`id='${lead}' AND status='contacted' AND follow_up_note='${marker}'`),1)
  })
  // Use a different source lead so conversion and follow-up are independently testable.
  const conversion=randomUUID()
  cleanup('property_leads',`id='${conversion}' AND community='${marker}'`)
  sql(`INSERT INTO property_leads (id,community,note) VALUES ('${conversion}','${marker}','${marker}')`)
  test('POST','/api/v1/property-leads/convert','admin',201,{leadId:conversion,...property},body=>{
    assert.match(body.data.id,/^[0-9a-f-]{36}$/i)
    assert.equal(count('property_leads',`id='${conversion}' AND status='converted' AND converted_property_id='${body.data.id}'`),1)
    assert.equal(count('properties',`id='${body.data.id}' AND community='${marker}' AND status='draft'`),1)
  })
  test('PATCH','/api/v1/admin/rental-requests/follow-up','admin',200,{id:request,status:'contacted',followUpNote:marker},()=>{
    assert.equal(count('rental_requests',`id='${request}' AND status='contacted' AND follow_up_note='${marker}'`),1)
  })
  test('PATCH','/api/v1/properties/status','admin',200,{propertyId:live,status:'offline',note:marker},()=>{
    assert.equal(count('properties',`id='${live}' AND status='offline'`),1)
    assert.equal(count('property_status_history',`property_id='${live}' AND to_status='offline' AND note='${marker}'`),1)
  })
  if (pending.some(row => row.path === '/api/v1/admin/temporary-access' && row.method === 'POST')) {
    const userId = randomUUID()
    const phone = '000' + BigInt('0x' + userId.replaceAll('-', '').slice(0, 14)).toString().padStart(17, '0')
    const userWhere = `id='${userId}' AND open_id='test:${userId}'`
    assert.equal(count('users', `phone='${phone}'`), 0)
    assert.equal(count('temporary_access_codes', `phone='${phone}'`), 0)
    cleanup('users', userWhere)
    cleanup('admin_users', `user_id='${userId}'`)
    cleanup('temporary_access_codes', `phone='${phone}'`)
    cleanup('user_footprints', `user_id='${userId}'`)
    sql(`INSERT INTO users (id,open_id,phone,avatar_url) VALUES ('${userId}','test:${userId}','${phone}','')`)
    let accessCode
    let userToken
    test('POST','/api/v1/admin/temporary-access','admin',201,{phone,expiresInHours:1},body=>{
      accessCode=body.data.accessCode
      assert.ok(typeof accessCode === 'string' && /^\d{6}$/.test(accessCode), 'Expected a temporary access code')
      assert.equal(count('temporary_access_codes', `phone='${phone}' AND status='active'`),1)
    })
    for(const role of ['guest','user','admin']) checks.set(key('POST','/api/v1/auth/temporary/login',role),async row=>{
      if(!accessCode) throw new Error('Temporary access code was not created')
      const res=await api(role,'POST','/api/v1/auth/temporary/login',{phone,accessCode})
      row.httpStatus=res.status
      assert.equal(res.status,200)
      assert.equal(res.body.data.role,'user')
      assert.equal(res.body.data.temporary,true)
      sessions.disposable=res.body.data.accessToken
      const profile=await api('disposable','GET','/api/v1/auth/profile')
      assert.equal(profile.status,200)
      assert.equal(profile.body.data.id,userId,'Temporary login did not resolve to the disposable fixture')
      userToken=sessions.disposable
    })
    test('PATCH','/api/v1/admin/temporary-access/status','admin',200,{phone,status:'revoked'},async ()=>{
      assert.equal(count('temporary_access_codes', `phone='${phone}' AND status='revoked'`),1)
      if(!accessCode) throw new Error('Temporary access code was not created')
      assert.equal((await api('guest','POST','/api/v1/auth/temporary/login',{phone,accessCode})).status,401)
    })
    checks.set(key('PATCH','/api/v1/auth/profile','user'),async row=>{
      if(!userToken) throw new Error('Disposable user login was not verified')
      sessions.disposable=userToken
      const res=await api('disposable','PATCH','/api/v1/auth/profile',{nickname:'QA permission fixture'})
      row.httpStatus=res.status
      assert.equal(res.status,200)
      assert.equal(res.body.data.id,userId)
      assert.equal(count('users', `${userWhere} AND nickname='QA permission fixture'`),1)
    })
    checks.set(key('DELETE','/api/v1/footprints','user'),async row=>{
      if(!userToken) throw new Error('Disposable user login was not verified')
      sessions.disposable=userToken
      sql(`INSERT INTO user_footprints (user_id,property_id) VALUES ('${userId}','${live}')`)
      const res=await api('disposable','DELETE','/api/v1/footprints')
      row.httpStatus=res.status
      assert.equal(res.status,200)
      assert.equal(count('user_footprints', `user_id='${userId}'`),0)
    })
    test('POST','/api/v1/admin/admin-users','admin',201,{phone},()=>{
      assert.equal(count('admin_users', `user_id='${userId}' AND status='active'`),1)
    })
    checks.set(key('PATCH','/api/v1/auth/profile','admin'),async row=>{
      if(!accessCode) throw new Error('Temporary access code was not created')
      const login=await api('guest','POST','/api/v1/auth/temporary/login',{phone,accessCode})
      assert.equal(login.status,200)
      assert.equal(login.body.data.role,'admin')
      sessions.disposableAdmin=login.body.data.accessToken
      const profile=await api('disposableAdmin','GET','/api/v1/auth/profile')
      assert.equal(profile.body.data.id,userId)
      const res=await api('disposableAdmin','PATCH','/api/v1/auth/profile',{nickname:'QA admin fixture'})
      row.httpStatus=res.status
      assert.equal(res.status,200)
      assert.equal(res.body.data.id,userId)
      assert.equal(count('users', `${userWhere} AND nickname='QA admin fixture'`),1)
    })
    test('PATCH','/api/v1/admin/admin-users/status','admin',200,{userId,status:'disabled'},()=>{
      assert.equal(count('admin_users', `user_id='${userId}' AND status='disabled'`),1)
    })
    test('DELETE','/api/v1/admin/admin-users/{userID}','admin',200,undefined,()=>{
      assert.equal(count('admin_users', `user_id='${userId}'`),0)
    },'/api/v1/admin/admin-users/'+userId)

  }
  const rank = row => {
    if(row.path.endsWith('/admin/temporary-access') && row.method==='POST') return -2
    if(row.path.endsWith('/auth/temporary/login')) return -1
    if(row.path.endsWith('/admin/admin-users') && row.method==='POST') return 3
    if(row.path.endsWith('/auth/profile') && row.role==='admin') return 4
    if(row.path.endsWith('/admin/admin-users/status')) return 5
    if(row.path.endsWith('/admin/admin-users/{userID}')) return 6
    if(row.path.endsWith('/admin/temporary-access/status')) return 7
    if(row.path.endsWith('/properties/status')) return 2
    return row.method === 'DELETE' ? 1 : 0
  }
  // Draft deletion is last so media and update can both use the same isolated fixture.
  for(const old of [...pending].sort((a,b)=>rank(a)-rank(b))) {
    const probe=checks.get(key(old.method,old.path,old.role))
    if(!probe) continue
    const row={...old, category:old.path.endsWith('/wechat/login')?'可安全执行':'需要合成数据', status:'INCOMPLETE'}
    delete row.reason
    report.results.push(row)
    try { await probe(row); row.status='PASS' }
    catch(error) { row.status=error instanceof assert.AssertionError?'FAIL':'INCOMPLETE';row.reason=error.message }
  }
} catch(error) {
  report.error=error.message
} finally {
  for(const {table,where} of cleanups.reverse()) {
    try {
      sql(`DELETE FROM ${table} WHERE ${where}`)
      assert.equal(count(table,where),0)
      report.cleanup.push({table,status:'PASS',remaining:0})
    } catch(error) { report.cleanup.push({table,status:'FAIL',reason:error.message}) }
  }
}
const classify = row => {
  if(row.path.endsWith('/wechat/phone')) return ['需要真实微信环境','需要真实手机号授权或合规的平台替身；本轮不调用微信。']
  if(row.path.endsWith('/rental-requests') && row.method==='POST') return ['需要专用依赖','创建需求会启动管理员通知；本轮未配置隔离通知验收。']
  if(/location|media\/upload|auth\/avatar|message-subscriptions/.test(row.path)) return ['需要专用依赖','需要隔离地图、上传存储或订阅授权替身。']
  if(/\/config\/public$/.test(row.path)) return ['需要专用依赖','修改共享配置；缺少独占数据库快照及恢复校验。']
  if(row.path.endsWith('/footprints') && row.method==='DELETE') return ['需要合成数据','批量清理需要专用空白会话，避免删除已有 QA 足迹。']
  return ['需要合成数据','需要专用一次性账号、凭证或业务对象及可验证的级联清理。']
}
if (report.cleanup.some(row => row.status === 'FAIL')) {
  for (const row of report.results.filter(row => row.status === 'PASS')) {
    row.status = 'INCOMPLETE'
    row.reason = 'Fixture cleanup could not be verified'
  }
}
const observed=new Map(report.results.map(row=>[key(row.method,row.path,row.role),row]))
matrix.api=matrix.api.map(row=>{
  if(row.status!=='NOT_RUN') return {...row,carried_forward:true}
  const tested=observed.get(key(row.method,row.path,row.role))
  if(tested) return tested
  const [category,reason]=classify(row)
  return {...row,category,reason}
})
const states=['PASS','FAIL','INCOMPLETE','NOT_RUN','NOT_APPLICABLE']
const counts=rows=>Object.fromEntries(states.map(state=>[state,rows.filter(r=>r.status===state).length]))
report.status=report.error||report.cleanup.some(r=>r.status!=='PASS')||report.results.some(r=>r.status!=='PASS')?'FAIL':'PASS'
report.coverage=counts(matrix.api)
matrix.counts ??= {}
matrix.counts.API=Object.fromEntries(['guest','user','admin'].map(role=>[role,counts(matrix.api.filter(r=>r.role===role))]))
matrix.apiEvidence=path.resolve(out,'api-result.json')
fs.writeFileSync(path.join(out,'role-permission-matrix.json'),JSON.stringify(matrix,null,2)+'\n')
fs.writeFileSync(path.join(out,'api-result.json'),JSON.stringify(report,null,2)+'\n')
fs.writeFileSync(path.join(out,'api-permission-report.md'),['# API 权限缺口复验','',report.status,report.error||'',
  '全量 API 覆盖：'+JSON.stringify(report.coverage),'新增执行：'+JSON.stringify(counts(report.results)),'',
  ...matrix.api.filter(r=>!r.carried_forward).map(r=>'- '+[r.role,r.method,r.path,r.category,r.status,r.httpStatus,r.reason].filter(x=>x!==undefined).join(' · ')),
  '', '## 清理', ...report.cleanup.map(r=>'- '+JSON.stringify(r))].join('\n')+'\n')
console.log(JSON.stringify({status:report.status,coverage:report.coverage,report:path.resolve(out,'api-result.json')}))
if(report.status!=='PASS')process.exitCode=1
