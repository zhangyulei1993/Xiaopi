import fs from 'node:fs'
import path from 'node:path'
import {createHash} from 'node:crypto'
import {UNAVAILABLE} from './engine.mjs'

export function archiveCatalog(directory,catalog) {
  const fields=['model_id','model_name','provider','capability_level','coding_score','reasoning_score',
    'speed_score','cost_score','supported_reasoning_efforts','availability','last_verified_at']
  const snapshot={version:catalog.version,default_model:catalog.default_model ?? null,
    models:catalog.models.map(model=>Object.fromEntries(fields.map(field=>[field,model[field]])))}
  const bytes = JSON.stringify(snapshot,null,2)+'\n'
  const hash = createHash('sha256').update(bytes).digest('hex')
  const target = path.join(directory,'catalog-history',hash+'.json')
  fs.mkdirSync(path.dirname(target),{recursive:true,mode:0o700})
  try { fs.writeFileSync(target,bytes,{flag:'wx',mode:0o600}) }
  catch(error) { if(error.code!=='EEXIST') throw error }
  return hash
}
export function codexUsage(text) {
  const events=text.split('\n').filter(Boolean).map(line=>JSON.parse(line))
  const completed=events.filter(event=>event.type==='turn.completed')
  if(completed.length!==1) throw new Error('Provide JSONL for one completed Codex turn')
  const usage=completed[0].usage ?? {}
  return {input_tokens:usage.input_tokens,cached_tokens:usage.cached_input_tokens,output_tokens:usage.output_tokens}
}
const number = value => typeof value==='number' && Number.isFinite(value) && value>=0 ? value : UNAVAILABLE
const id = value => typeof value==='string' && /^[a-zA-Z0-9_./:-]{1,100}$/.test(value) ? value : UNAVAILABLE
const status = value => ['PASS','FAIL','INCOMPLETE','NOT_RUN'].includes(value)?value:UNAVAILABLE
export function usageRecord(meta,usage={}) {
  const cost=meta.cost_evidence
  const reported=cost?.source==='provider_report' && /^[A-Z]{3}$/.test(cost.currency) &&
    id(cost.reference)!==UNAVAILABLE && number(cost.amount)!==UNAVAILABLE
  return {
    execution_id:id(meta.execution_id),task_id:id(meta.task_id),
    task_level:/^L[1-5]$/.test(meta.task_level)?meta.task_level:UNAVAILABLE,
    task_type:['安全','架构或复杂故障','认证与权限','数据库','公共 API','公共组件','文案与样式','多文件联动','常规开发'].includes(meta.task_type)?meta.task_type:UNAVAILABLE,
    recommended_model:id(meta.recommended_model),actual_model:id(meta.actual_model),
    recommended_effort:['Low','Medium','High','Extra High'].includes(meta.recommended_effort)?meta.recommended_effort:UNAVAILABLE,
    actual_effort:['Low','Medium','High','Extra High'].includes(meta.actual_effort)?meta.actual_effort:UNAVAILABLE,
    input_tokens:number(usage.input_tokens),output_tokens:number(usage.output_tokens),cached_tokens:number(usage.cached_tokens),
    code_quality:['PASS','FAIL','NEEDS_REWORK'].includes(meta.code_quality)?meta.code_quality:UNAVAILABLE,
    actual_usage_cost:reported?cost.amount:UNAVAILABLE,
    cost_currency:reported?cost.currency:UNAVAILABLE,
    cost_source:reported?'operator_supplied_provider_report':'UNAVAILABLE',
    cost_reference:reported?cost.reference:UNAVAILABLE, execution_duration:number(meta.execution_duration),retry_count:number(meta.retry_count),
    test_result:status(meta.test_result),final_result:status(meta.final_result),
    model_source:id(meta.actual_model)===UNAVAILABLE?'UNAVAILABLE':'operator_declared',
    usage_source:Object.values(usage).some(Number.isFinite)?'codex_exec_jsonl':'UNAVAILABLE',
    timestamp:new Date().toISOString(),
  }
}
// Rollouts contain repeated cumulative counters. Import one historical snapshot.
export function rolloutRecord(text) {
  let session, latest
  for(const line of text.split('\n').filter(Boolean)) {
    const event=JSON.parse(line)
    if(event.type==='session_meta' && !session) session=event.payload?.id
    if(event.type==='event_msg' && event.payload?.type==='token_count' && event.payload.info?.total_token_usage) {
      latest=event.payload.info.total_token_usage
    }
  }
  if(id(session)===UNAVAILABLE || !latest || !['input_tokens','output_tokens','cached_input_tokens'].every(key=>
    Number.isSafeInteger(latest[key]) && latest[key]>=0) || latest.cached_input_tokens>latest.input_tokens) {
    throw new Error('Historical rollout lacks a valid session ID or cumulative usage')
  }
  const execution_id='rollout:'+createHash('sha256').update(session).digest('hex')
  return {...usageRecord({execution_id,task_id:'historical-session'},{
    input_tokens:latest.input_tokens,output_tokens:latest.output_tokens,cached_tokens:latest.cached_input_tokens,
  }),usage_source:'codex_rollout_session_total'}
}
export function readRecords(dir) {
  if(!fs.existsSync(dir)) return []
  return fs.readdirSync(dir).filter(name=>/^[a-f0-9]{64}\.json$/.test(name)).map(name=>JSON.parse(fs.readFileSync(path.join(dir,name),'utf8')))
}
export function saveRecord(dir,row) {
  if([row.execution_id,row.task_id].includes(UNAVAILABLE)) throw new Error('Valid execution_id and task_id required')
  fs.mkdirSync(dir,{recursive:true,mode:0o700})
  const hash=createHash('sha256').update(JSON.stringify([row.execution_id,row.task_id])).digest('hex')
  fs.writeFileSync(path.join(dir,hash+'.json'),JSON.stringify(row,null,2)+'\n',{flag:'wx',mode:0o600})
}
