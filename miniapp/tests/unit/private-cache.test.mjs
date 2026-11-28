import assert from 'node:assert/strict'
import test from 'node:test'
import { readPrivateCache, writePrivateCache } from '../../src/auth/private-cache.ts'

for (const [name, allowedRole, value] of [
  ['phone', 'user', 'qa-synthetic-phone'],
  ['crop', 'admin', { index: 0, path: 'qa-synthetic-crop' }],
]) {
  test(name + ' cache reads require the authorized session and owner', () => {
    const owner = { role: allowedRole, accessToken: 'qa-owner' }
    const storage = new Map()
    const reads = []
    const events = []
    const read = key => { reads.push(key); return storage.get(key) }
    const write = (key, data) => storage.set(key, data)
    const onRead = present => events.push(present)
    writePrivateCache(owner, allowedRole, name, value, write)
    const otherRole = allowedRole === 'admin' ? 'user' : 'admin'
    assert.equal(readPrivateCache(null, allowedRole, name, read, onRead), undefined)
    assert.equal(readPrivateCache({ role: otherRole, accessToken: owner.accessToken }, allowedRole, name, read, onRead), undefined)
    assert.deepEqual(reads, [])
    assert.equal(readPrivateCache({ ...owner, accessToken: 'qa-other' }, allowedRole, name, read, onRead), undefined)
    assert.deepEqual(reads, [name + ':owner'])
    assert.deepEqual(events, [])
    assert.deepEqual(readPrivateCache(owner, allowedRole, name, read, onRead), value)
    assert.deepEqual(events, [true])
    storage.delete(name + ':owner')
    assert.equal(readPrivateCache(owner, allowedRole, name, read, onRead), undefined)
    assert.deepEqual(events, [true])
  })
}

import fs from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'

function runLifecycle(page, hook, context) {
  const file = new URL('../../src/pages/' + page + '.vue', import.meta.url)
  const script = fs.readFileSync(file,'utf8').match(/<script setup[^>]*>([\s\S]*?)<\/script>/)[1]
  const source = ts.createSourceFile('page.ts',script,ts.ScriptTarget.Latest,true)
  const registration = source.statements.find(node =>
    ts.isExpressionStatement(node) && ts.isCallExpression(node.expression) &&
    node.expression.expression.getText(source) === hook)
  assert.ok(registration, hook)
  const callback = registration.expression.arguments[0].getText(source)
  const js = ts.transpileModule('(' + callback + ')({})',{compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText
  const redirect = fs.readFileSync(new URL('../../src/utils/permission-redirect.ts', import.meta.url), 'utf8').replace('export function', 'function')
  return vm.runInNewContext(redirect + '\n' + js, { ...context, setTimeout: callback => callback() })
}

for (const [page, hook, roles] of [
  ['requests/index','onLoad',['admin']],
  ['admin/property-edit','onShow',[null,'user']],
]) {
  for (const role of roles) test(page + ' rejects ' + (role || 'guest') + ' before the nonempty cache is read', async () => {
    const events = []
    const reads = []
    const storage = new Map([
      ['xiaopi_request_bound_phone','qa-synthetic-phone'],
      ['xiaopi-property-image-crop-result',{ index:0,path:'qa-synthetic-crop' }],
    ])
    await runLifecycle(page,hook,{
      getSession:()=>role ? {role,accessToken:'qa-session'} : null,
      permissionAudit:(...event)=>events.push(event),
      readPrivateCache,
      uni:{
        getStorageSync:key=>{reads.push(key);return storage.get(key)},
        switchTab:({url})=>events.push(['navigation',url]),
      },
    })
    assert.deepEqual(reads,[])
    assert.ok(events.some(([event,,allowed])=>event==='guard' && allowed===false))
    assert.ok(!events.some(([event])=>event==='private-read' || event==='private-binding'))
    if (hook==='onLoad') assert.ok(events.some(([event,url])=>event==='navigation' && url==='/pages/me/index'))
  })
}
