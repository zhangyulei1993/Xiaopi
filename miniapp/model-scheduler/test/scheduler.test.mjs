import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import {execFileSync} from 'node:child_process'
import {test} from 'node:test'
import {analyze,recommend,summarize,validateCatalog} from '../engine.mjs'
import {archiveCatalog,codexUsage,readRecords,saveRecord,usageRecord,rolloutRecord} from '../storage.mjs'

const model=(id,level,cost,efforts)=>({model_id:id,model_name:id,provider:'fixture',capability_level:level,
  coding_score:level*18,reasoning_score:level*18,speed_score:100-cost,cost_score:cost,
  supported_reasoning_efforts:efforts,availability:'active',last_verified_at:null})
const low=model('fixture-low',2,10,['Low','Medium'])
const high=model('fixture-high',5,90,['Medium','High','Extra High'])
const catalog={version:'1',default_model:low.model_id,models:[low,high]}
const simple=analyze({description:'修改按钮文案'})
const security=analyze({description:'修复安全漏洞'})
const temp=t=>{
  const dir=fs.mkdtempSync(path.join(os.tmpdir(),'model-scheduler-'))
  t.after(()=>fs.rmSync(dir,{recursive:true,force:true}))
  return dir
}

test('five levels distinguish intent, dependency and change scope',()=>{
  assert.equal(simple.task_level,'L1')
  assert.equal(analyze({description:'增加普通页面功能'}).task_level,'L2')
  assert.equal(analyze({changes:[{file:'x.vue',lines:100},{file:'y.ts',lines:3}]}).task_level,'L3')
  assert.equal(analyze({description:'调整数据库字段'}).task_level,'L4')
  assert.equal(security.task_level,'L5')
  assert.equal(security.risk_level,'critical')
  assert.equal(analyze({description:'改颜色',changes:[{file:'miniapp/src/api/client.ts'}]}).task_level,'L4')
})

test('economical default for simple work and capability floor for risky work',()=>{
  assert.equal(recommend(simple,catalog).recommendation.model_id,low.model_id)
  const plan=recommend(security,catalog,{current:low.model_id})
  assert.equal(plan.recommendation.model_id,high.model_id)
  assert.equal(plan.upgrade_suggested,true)
  assert.equal(plan.recommendation.reasoning_effort,'High')
  assert.equal(recommend(security,catalog,{extraHigh:true}).recommendation.reasoning_effort,'Extra High')
  assert.equal(recommend(security,{...catalog,models:[low]}).status,'NO_ELIGIBLE_MODEL')
})

test('manual override takes precedence and rejects unknown or unsupported combinations',()=>{
  assert.equal(recommend(simple,catalog,{override:high.model_id}).recommendation.model_id,high.model_id)
  assert.equal(recommend(simple,catalog,{override:'unknown'}).status,'UNKNOWN_OVERRIDE')
  assert.equal(recommend(simple,catalog,{override:low.model_id,effort:'High'}).status,'OVERRIDE_UNAVAILABLE')
  assert.equal(recommend(simple,{...catalog,models:[{...low,availability:'retired'},high]}).recommendation.model_id,high.model_id)
  assert.equal(recommend(simple,catalog).recommendation.evaluation_status,'COLD_START')
  assert.throws(()=>validateCatalog({...catalog,models:[low,low]}))
})

test('same task level history needs at least three completed samples',()=>{
  const row={actual_model:low.model_id,task_level:'L1',final_result:'PASS',test_result:'PASS',retry_count:0,usage_source:'codex_exec_jsonl',actual_effort:'Low'}
  assert.equal(recommend(simple,catalog,{history:[row]}).recommendation.evaluation_status,'INSUFFICIENT_SAMPLES')
  assert.equal(recommend(simple,catalog,{history:[row,row,row]}).recommendation.evaluation_status,'EVALUATED')
  assert.equal(summarize([row,{...row,final_result:'FAIL',retry_count:1}]).rework_rate,.5)
})

test('persisted records whitelist metrics and preserve missing usage',t=>{
  const dir=temp(t)
  const empty=usageRecord({execution_id:'run',task_id:'task',prompt:'discard'})
  assert.equal(empty.actual_model,'UNAVAILABLE')
  assert.equal(empty.input_tokens,'UNAVAILABLE')
  assert.equal(empty.actual_usage_cost,'UNAVAILABLE')
  assert.equal('prompt' in empty,false)
  const metrics=codexUsage('{"type":"item.completed","item":{"text":"discard"}}\n{"type":"turn.completed","usage":{"input_tokens":12,"output_tokens":7,"cached_input_tokens":0}}')
  const record=usageRecord(empty,metrics)
  assert.equal(record.cached_tokens,0)
  saveRecord(dir,record)
  assert.deepEqual(readRecords(dir),[record])
  assert.throws(()=>saveRecord(dir,record),{code:'EEXIST'})
  assert.throws(()=>codexUsage('{}'))
})

test('CLI reloads catalog versions and reuses the unchanged local test command',t=>{
  const dir=temp(t),file=path.join(dir,'catalog.json')
  const run=(command,...args)=>JSON.parse(execFileSync(process.execPath,[
    'model-scheduler/cli.mjs',command,'--json','--catalog='+file,'--state='+dir,...args],{encoding:'utf8'}))
  fs.writeFileSync(file,JSON.stringify(catalog))
  const first=run('plan','--planned','--task=修改首页文案','--file=miniapp/src/pages/home/index.vue')
  assert.equal(first.recommendation.model_id,low.model_id)
  assert.ok(first.dependency_count>=0)
  assert.equal(first.test_scope,'local')
  assert.ok(first.test_plan[0].some(arg=>arg.includes('home-load')))
  fs.writeFileSync(file,JSON.stringify({...catalog,version:'2',models:[{...low,availability:'disabled'},high]}))
  assert.equal(run('plan','--planned','--task=修改文案','--file=miniapp/src/pages/home/index.vue').recommendation.model_id,high.model_id)
  assert.equal(fs.readdirSync(path.join(dir,'catalog-history')).length,2)
  assert.equal(run('benchmark').results.length,14)
  assert.ok(run('benchmark').results.every(row=>row.status==='未评估'))
  assert.equal(run('stats').total_records,0)
  assert.equal(archiveCatalog(dir,catalog),first.revision)
})

test('rollout imports the last cumulative count once and discards other fields',t=>{
  const dir=temp(t)
  const snapshot=n=>({type:'event_msg',payload:{type:'token_count',info:{total_token_usage:
    {input_tokens:n,output_tokens:7,cached_input_tokens:5}}}})
  const text=[{type:'session_meta',payload:{id:'test-id',base_instructions:'discard'}},snapshot(10),snapshot(20)]
    .map(row=>JSON.stringify(row)).join('\n')
  const row=rolloutRecord(text)
  assert.equal(row.input_tokens,20)
  assert.equal(row.output_tokens,7)
  assert.equal(row.cached_tokens,5)
  assert.equal(row.actual_model,'UNAVAILABLE')
  assert.equal(row.final_result,'UNAVAILABLE')
  assert.equal(JSON.stringify(row).includes('discard'),false)
  saveRecord(dir,row)
  assert.throws(()=>saveRecord(dir,rolloutRecord(text)),{code:'EEXIST'})
})

test('picker-visible candidates require confirmation before recommendation',()=>{
  const candidates=validateCatalog(JSON.parse(fs.readFileSync(new URL('../catalog/candidate.json',import.meta.url))))
  assert.ok(candidates.models.length>0)
  assert.ok(candidates.models.every(entry=>entry.availability==='UNVERIFIED'))
  assert.equal(recommend(simple,candidates).status,'NO_ELIGIBLE_MODEL')
  assert.equal(recommend(simple,candidates,{override:candidates.models[0].model_id}).status,'OVERRIDE_UNAVAILABLE')
})

test('external Go repository gets a Go-only plan without running its commands',t=>{
  const dir=temp(t)
  execFileSync('git',['init','-q',dir])
  const result=JSON.parse(execFileSync(process.execPath,['model-scheduler/cli.mjs','plan','--json',
    '--project='+dir,'--state='+dir,'--planned','--file=internal/app/app.go','--task=实现常规功能']))
  assert.deepEqual(result.test_plan,[['go','test','./...']])
  assert.deepEqual(result.affected_modules,[])
  assert.equal(result.dependency_graph,'UNAVAILABLE')
})

test('efficiency uses completed metered tasks and never treats cached input as extra tokens',()=>{
  const row={final_result:'PASS',input_tokens:100,output_tokens:20,cached_tokens:80}
  assert.equal(summarize([row]).successful_tasks_per_million_tokens,1e6/120)
  assert.equal(summarize([{final_result:'PASS',input_tokens:100}]).successful_tasks_per_million_tokens,'UNAVAILABLE')
})

test('actual cost accepts a referenced provider report and excludes estimates',()=>{
  const meta={final_result:'PASS'}
  const evidence={source:'provider_report',currency:'USD',amount:0.2,reference:'billing-run-1'}
  assert.equal(usageRecord({...meta,cost_evidence:{...evidence,source:'estimate'}}).actual_usage_cost,'UNAVAILABLE')
  const row=usageRecord({...meta,cost_evidence:evidence})
  assert.equal(summarize([row]).average_cost,0.2)
  assert.equal(summarize([row,{...row,cost_currency:'EUR'}]).average_cost,'UNAVAILABLE')
})

test('picker-listed model can be pinned without unverified scores entering automatic ranking',()=>{
  const model={
    model_id:'picker-model',model_name:'Picker model',provider:'OpenAI',availability:'active',
    capability_level:'UNVERIFIED',coding_score:'UNVERIFIED',reasoning_score:'UNVERIFIED',
    speed_score:'UNVERIFIED',cost_score:'UNVERIFIED',supported_reasoning_efforts:['Low','High'],
    last_verified_at:null,
  }
  const catalog=validateCatalog({version:'picker',models:[model]})
  const task=analyze({description:'修改首页文案'})
  assert.equal(recommend(task,catalog).status,'NO_ELIGIBLE_MODEL')
  const pinned=recommend(task,catalog,{override:model.model_id,effort:'Low'})
  assert.equal(pinned.recommendation.model_id,model.model_id)
  assert.equal(pinned.recommendation.estimated_consumption,'UNAVAILABLE')
})
