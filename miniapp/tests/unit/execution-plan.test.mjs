import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { evidenceIsCurrent, executePlan, executionPlan, permissionRows, requiredExit, testRecord } from '../execution-plan.mjs'

test('shared dependency delegates both visual suites exactly once to the full runner', () => {
  const script = fileURLToPath(new URL('../impact.mjs', import.meta.url))
  const plan = JSON.parse(execFileSync(process.execPath,[script,'--plan','--file=miniapp/src/api/client.ts','--example=+request(url)']))
  assert.equal(plan.scope,'full')
  assert.deepEqual(plan.commands.filter(command => command.at(-1) !== 'test:ui:device'), [['npm','run','test']])
  const suites = executionPlan({full:true})
  for (const name of ['core-ui','extended-ui']) assert.equal(suites.filter(item=>item===name).length,1)
  assert.deepEqual(executionPlan({full:false,visual:'home'}),['build-and-security','business-e2e','core-ui'])
})

test('API followup completes before other writes and an injected UI failure does not stop independent suites', async () => {
  const plan=executionPlan({full:true})
  const events=[]
  const suites={}
  await executePlan(plan, async name=>{
    events.push(name)
    if(name==='role-api') {
      await new Promise(resolve=>setImmediate(resolve))
      events.push('api-supplement-cleanup')
    }
    if(name==='role-ui') throw new Error('Injected bridge failure')
    suites[name]={status:'PASS'}
  },name=>{suites[name]={status:'FAIL'}})
  assert.deepEqual(events,[...plan.slice(0,2),'api-supplement-cleanup',...plan.slice(2)])
  assert.equal(requiredExit(plan,suites),1)
})

test('preflight failure prevents any dependent suite from running', async () => {
  const plan=executionPlan({full:true})
  const events=[]
  await executePlan(plan,name=>{events.push(name);throw new Error('Injected preflight failure')},()=>{})
  assert.deepEqual(events,['build-and-security'])
})

test('a current retry replaces only its matching permission result', () => {
  const base=[{id:'route',status:'FAIL'},{id:'untested',status:'NOT_RUN'},{id:'denied',status:'FAIL'}]
  const retry=[{id:'route',status:'PASS'},{id:'untested',status:'PASS',carried_forward:true}]
  assert.deepEqual(permissionRows(base,retry,row=>row.id).map(row=>row.status),['PASS','NOT_RUN','FAIL'])
})

test('untested and historical passes cannot become current passes', () => {
  const now=Date.now()
  assert.equal(evidenceIsCurrent({isFile:()=>true,birthtimeMs:now-5000,mtimeMs:now},now),false)
  assert.equal(evidenceIsCurrent({isFile:()=>true,birthtimeMs:now,mtimeMs:now},now),true)
  const result={startedAt:new Date(now).toISOString(),evidence:'/evidence'}
  for (const row of [{status:'NOT_RUN'},{status:'PASS',carried_forward:true}]) {
    const record=testRecord(row,result,'test','run')
    assert.equal(record.status,'NOT_RUN')
    assert.equal(record.from_current_execution,false)
    assert.equal(record.assertion_status,'NOT_RUN')
  }
  assert.equal(requiredExit(['api'],{api:{status:'INCOMPLETE'}}),1)
})
