import {execFileSync} from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import {gitChanges, miniappImpact, goImpact} from './adapters.mjs'
import {fileURLToPath} from 'node:url'
import {analyze, recommend, summarize, validateCatalog, UNAVAILABLE} from './engine.mjs'
import {archiveCatalog, codexUsage, rolloutRecord, readRecords, saveRecord, usageRecord} from './storage.mjs'

const root=fileURLToPath(new URL('../',import.meta.url))
const repo=path.dirname(root.slice(0,-1))
const [command,...args]=process.argv.slice(2)
const option=name=>args.find(arg=>arg.startsWith('--'+name+'='))?.slice(name.length+3)
const json=file=>JSON.parse(fs.readFileSync(file,'utf8'))
const state=option('state') ?? path.join(os.homedir(),'.local','state','xiaopi-model-scheduler')
const historyDir=path.join(state,'usage')
const catalogFile=option('catalog') ?? process.env.XIAOPI_MODEL_CATALOG ?? path.join(root,'model-scheduler/catalog/daily.json')
const output=data=>{
  if(args.includes('--json')) console.log(JSON.stringify(data,null,2))
  else for(const [key,value] of Object.entries(data)) {
    if(Array.isArray(value) && value.length && typeof value[0]==='object') {
      console.log(key)
      console.table(value)
    } else console.log(key,typeof value==='object'?JSON.stringify(value):value)
  }
}

function collect() {
  const planned=args.includes('--planned')
  const selected=args.filter(arg=>arg.startsWith('--file=')).map(arg=>arg.slice(7))
  const project=option('project')
  const adapter=option('adapter') ?? (project ? 'go' : 'miniapp')
  if(!['go','miniapp','generic'].includes(adapter) || (adapter!=='miniapp'&&!project) || (adapter==='miniapp'&&project)) {
    throw new Error('Use --adapter=go or --adapter=generic with --project=PATH for external repositories')
  }
  const target=project ? fs.realpathSync(project) : repo
  const gitRoot=execFileSync('git',['rev-parse','--show-toplevel'],{cwd:target,encoding:'utf8'}).trim()
  const changes=gitChanges(gitRoot,{selected,planned,
    ignored:adapter==='miniapp'?/^(?:miniapp\/tests\/reports\/|miniapp\/dist\/|test-reports\/|output\/|assets\/audit-media\/|\.playwright-cli\/)/:undefined})
  const impact=adapter==='go' ? goImpact(target,changes)
    : adapter==='miniapp' ? miniappImpact(root,changes,planned)
      : {scope:'full',dependency_graph:UNAVAILABLE,commands:[]}
  const testDirectory=adapter==='miniapp' ? root : target
  if(adapter==='generic' && changes.length) {
    const frontend=changes.every(({file})=>file.startsWith('frontend/'))
    const directory=frontend?path.join(target,'frontend'):target
    const manifest=path.join(directory,'package.json')
    if(fs.existsSync(manifest)) {
      const scripts=json(manifest).scripts??{}
      const script=scripts.test?'test':scripts.build?'build':null
      if(script) impact.steps=[{cwd:directory,argv:['npm','run',script]}]
    }
  }
  const testSteps=impact.steps??impact.commands.map(argv=>({
    cwd:adapter==='miniapp' && argv[0]==='go'?path.join(gitRoot,'backend'):testDirectory,argv,
  }))

  const report=option('test-report') ? json(option('test-report')) : null
  const cases=report ? (Array.isArray(report.tests)?report.tests:report.cases??[]) : []
  const testResults=report ? Object.create(null) : null
  for (const row of cases) {
    const status=row.status??'UNAVAILABLE'
    testResults[status]=(testResults[status]??0)+1
  }
  return {planned,task:analyze({description:option('task')??'',changes,impact,failures:testResults?.FAIL??0}),impact,testResults,gitRoot,testDirectory,adapter,testSteps}

}

try {
  if(command==='record') {
    if(option('rollout')) {
      const row=rolloutRecord(fs.readFileSync(option('rollout'),'utf8'))
      saveRecord(historyDir,row)
      output(row)
    } else {
      if(!option('metadata')) throw new Error('Use --metadata=FILE')
      const row=usageRecord(json(option('metadata')),option('events')?codexUsage(fs.readFileSync(option('events'),'utf8')):{})
      saveRecord(historyDir,row)
      output(row)
    }
  } else if(command==='stats') {
    const history=readRecords(historyDir)
    const sessions=history.filter(row=>row.usage_source==='codex_rollout_session_total')
    const turns=history.filter(row=>row.usage_source!=='codex_rollout_session_total')
    output({total_records:history.length,historical_session_count:sessions.length,historical_sessions:summarize(sessions),
      summary:summarize(turns),models:[...new Set(turns.map(row=>row.actual_model))].map(model=>({model,...summarize(turns.filter(row=>row.actual_model===model))}))})
  } else if(command==='analyze') {
    const {task,planned}=collect()
    output({...task,input_mode:planned?'planned_files':'git_diff'})
  } else {
    const catalog=validateCatalog(json(catalogFile))
    const history=readRecords(historyDir)
    const revision=archiveCatalog(state,catalog)
    if(command==='catalog') {
      output({version:catalog.version,revision,default_model:catalog.default_model,models:catalog.models.map(model=>({...model,
        evaluation_status:history.some(row=>row.actual_model===model.model_id && ['PASS','FAIL'].includes(row.final_result))?'HAS_RECORDS':'COLD_START'}))})
    } else if(command==='plan') {
      const {task,impact,planned,testResults,gitRoot,testDirectory,adapter,testSteps}=collect()
      output({...task,...recommend(task,catalog,{override:option('model'),effort:option('effort'),current:option('current'),
        extraHigh:args.includes('--extra-high-supported'),preference:option('preference'),history}),
        revision,project_root:gitRoot,test_directory:testDirectory,adapter,dependency_graph:impact.dependency_graph ?? 'available',dependency_status:impact.dependency_status,input_mode:planned?'planned_files':'git_diff',test_scope:impact.scope,go_impact:impact.dependency_status ? impact.reasons : undefined,test_plan:testSteps.map(step=>step.argv),test_steps:testSteps,test_results:testResults??UNAVAILABLE,actual_usage:UNAVAILABLE})
    } else if(command==='benchmark') {
      const suite=json(path.join(root,'model-scheduler/benchmark.json'))
      const rows=catalog.models.flatMap(model=>suite.tasks.map(task=>{
        const matches=history.filter(row=>row.usage_source==='codex_exec_jsonl' && (!option('effort') || row.actual_effort===option('effort')) && row.actual_model===model.model_id&&row.task_id==='benchmark:'+suite.version+':'+task.id)
        const metrics=summarize(matches)
        return {model:model.model_id,task:task.id,status:metrics.samples>=3?'EVALUATED':metrics.samples?'INSUFFICIENT_SAMPLES':'未评估',...metrics}
      }))
      output({suite_version:suite.version,mode:'offline_evidence_evaluation',tasks:suite.tasks,results:rows})
    } else throw new Error('Use plan, analyze, stats, catalog, benchmark or record')
  }
} catch(error) {
  console.error(error.message)
  process.exitCode=1
}
