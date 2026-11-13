export const UNAVAILABLE = 'UNAVAILABLE'
export const efforts = ['Low', 'Medium', 'High', 'Extra High']
const scoreKeys = ['coding_score','reasoning_score','speed_score','cost_score']

export function validateCatalog(catalog) {
  if (!catalog.version || !Array.isArray(catalog.models)) throw new Error('Invalid catalog')
  const ids = new Set()
  for (const model of catalog.models) {
    if (!model.model_id || ids.has(model.model_id) || !model.model_name || !model.provider ||
        !['active','disabled','retired','UNVERIFIED'].includes(model.availability) ||
        (model.capability_level==='UNVERIFIED' ? false :
          !Number.isInteger(model.capability_level) || model.capability_level < 1 || model.capability_level > 5) ||
        scoreKeys.some(key => !(model[key]==='UNVERIFIED') &&
          (!Number.isFinite(model[key]) || model[key] < 0 || model[key] > 100)) ||
        !Array.isArray(model.supported_reasoning_efforts) || !model.supported_reasoning_efforts.length ||
        model.supported_reasoning_efforts.some(effort => !efforts.includes(effort)) ||
        !(model.last_verified_at === null || Number.isFinite(Date.parse(model.last_verified_at)))) {
      throw new Error('Invalid or duplicate model entry')
    }
    ids.add(model.model_id)
  }
  return catalog
}

export function analyze({description='', changes=[], impact={}, failures=0}) {
  const files = changes.map(change => change.file)
  const context = [description, ...files, ...(impact.reasons ?? []).flatMap(row => row.symbols ?? [])].join(' ')
  const modules = [...new Set((impact.reasons ?? []).flatMap(row=>row.modules ?? []))].sort()
  const dependencies = new Set((impact.reasons ?? []).flatMap(row=>[...(row.reverse_dependencies ?? []), ...(row.navigationTargets ?? []).map(route=>'miniapp/src/pages/'+route+'.vue')]))
  const api = /公共\s*API|shared API|\/src\/api\//i.test(context)
  const auth = /权限|认证|authorization|authentication|\bauth\b|权限控制/i.test(context) || (impact.reasons ?? []).some(row=>row.roleSensitive)
  const database = /数据库|migration|schema|database|\.sql\b/i.test(context)
  const security = /安全|security|漏洞|vulnerability|secret|token/i.test(context)
  const architecture = /架构|architecture|复杂故障|race condition|死锁/i.test(context)
  const shared = api || /公共组件|shared component|\/components\//i.test(context)
  const types = [...new Set(files.map(file=>file.split('.').at(-1)).filter(ext=>['vue','ts','go','js','mjs','sql','css','scss'].includes(ext)))]
  const lines = changes.reduce((sum,row)=>sum+(row.lines ?? 0),0)
  const simple = /文案|颜色|间距|copy|color|spacing|typo/i.test(description)
  const shape = Math.min(2, Math.floor(files.length/4)) + Math.min(2,Math.floor(lines/80)) +
    Number(types.length>1) + Number(modules.length>1) + Number(dependencies.size>=3) +
    Number(impact.scope==='full' && files.length>0) + Math.min(2,failures)
  const score = Math.min(100,(simple?5:20)+shape*9+Number(shared)*30+Number(auth||database)*45+Number(security||architecture)*65)
  const level = security || architecture || score>=80 ? 5 : auth || database || shared || score>=55 ? 4 : score>=32 ? 3 : score>=18 ? 2 : 1
  return {
    task_level:'L'+level, task_type:security?'安全':architecture?'架构或复杂故障':auth?'认证与权限':database?'数据库':api?'公共 API':shared?'公共组件':simple&&level===1?'文案与样式':level>=3?'多文件联动':'常规开发',
    complexity_score:score, risk_level:level>=5?'critical':level===4?'high':level===3?'medium':'low',
    affected_modules:modules, dependency_count:dependencies.size, changed_files:files.length, changed_lines:lines,
    file_types:types, failed_tests:failures,
    recommended_reasoning_effort:level===1?'Low':level<=3?'Medium':level===5?'Extra High':'High',
  }
}

export function summarize(rows) {
  const finished = rows.filter(row=>['PASS','FAIL'].includes(row.final_result))
  const rate = (items,predicate)=>items.length ? items.filter(predicate).length/items.length : UNAVAILABLE
  const average = key=>{
    const observed = rows.map(row=>row[key]).filter(Number.isFinite)
    return observed.length ? observed.reduce((a,b)=>a+b,0)/observed.length : UNAVAILABLE
  }
  const metered = finished.filter(row=>Number.isSafeInteger(row.input_tokens) && Number.isSafeInteger(row.output_tokens) &&
    row.input_tokens>=0 && row.output_tokens>=0 && row.input_tokens+row.output_tokens>0)
  const billed=finished.filter(row=>row.cost_source==='operator_supplied_provider_report' &&
    Number.isFinite(row.actual_usage_cost) && /^[A-Z]{3}$/.test(row.cost_currency))
  const currencies=new Set(billed.map(row=>row.cost_currency))
  const currency=currencies.size===1 ? billed[0].cost_currency : UNAVAILABLE
  const attempts = rows.filter(row=>Number.isInteger(row.retry_count))
  return {samples:finished.length, success_rate:rate(finished,r=>r.final_result==='PASS'),
    test_pass_rate:rate(rows.filter(r=>['PASS','FAIL'].includes(r.test_result)),r=>r.test_result==='PASS'),
    rework_rate:rate(attempts,r=>r.retry_count>0), average_retries:average('retry_count'),
    average_duration_ms:average('execution_duration'), average_input_tokens:average('input_tokens'),
    average_output_tokens:average('output_tokens'), average_cached_tokens:average('cached_tokens'),
    metered_samples:metered.length,
    successful_tasks_per_million_tokens:metered.length
      ? 1e6*metered.filter(row=>row.final_result==='PASS').length/metered.reduce((sum,row)=>sum+row.input_tokens+row.output_tokens,0)
      : UNAVAILABLE,
    code_quality_pass_rate:rate(rows.filter(row=>['PASS','FAIL','NEEDS_REWORK'].includes(row.code_quality)),row=>row.code_quality==='PASS'),
    billed_samples:billed.length,cost_currency:currency,
    average_cost:currency!==UNAVAILABLE ? billed.reduce((sum,row)=>sum+row.actual_usage_cost,0)/billed.length : UNAVAILABLE}
}

export function recommend(task,catalog,{override, effort, current, extraHigh=false, history=[], preference='balanced'}={}) {
  const level = Number(task.task_level.slice(1))
  const chooseEffort = (model, manual=false)=>{
    const supported = efforts.filter(e=>model.supported_reasoning_efforts.includes(e) && (e!=='Extra High'||extraHigh))
    if (effort) return supported.includes(effort) && (manual || level<4 || efforts.indexOf(effort)>=2) ? effort : null
    const desired = efforts.indexOf(task.recommended_reasoning_effort)
    const selected = supported.filter(e=>efforts.indexOf(e)<=desired).at(-1) ?? supported[0]
    return !manual && level>=4 && efforts.indexOf(selected)<2 ? null : selected
  }
  const modelHistory = model=>history.filter(row=>row.usage_source==='codex_exec_jsonl' &&
    row.actual_model===model.model_id && row.task_level===task.task_level)
  const stats = (model,manual=false)=>summarize(modelHistory(model).filter(row=>row.actual_effort===chooseEffort(model,manual)))
  const efficiency = model=>stats(model).metered_samples>=3 ? stats(model).successful_tasks_per_million_tokens : null
  const result = (model,manual=false)=>{
    const measured = stats(model,manual)
    return {model_id:model.model_id,model_name:model.model_name,reasoning_effort:chooseEffort(model,manual),
      evaluation_status:measured.samples>=3?'EVALUATED':measured.samples||modelHistory(model).length?'INSUFFICIENT_SAMPLES':'COLD_START',
      estimated_consumption:!Number.isFinite(model.cost_score)?UNAVAILABLE:model.cost_score>=70?'high':model.cost_score>=35?'medium':'low',history:measured}
  }
  const eligible = model=>model.availability==='active' && scoreKeys.every(key=>Number.isFinite(model[key])) && model.capability_level>=level &&
    (level<4 || Math.min(model.coding_score,model.reasoning_score)>=70) && chooseEffort(model)
  const rank = (model, bestEfficiency, maxCost)=>{

    const measured = stats(model)
    const learning = measured.samples>=3 ? 12*(measured.success_rate-.5) -
      (Number.isFinite(measured.rework_rate)?6*measured.rework_rate:0) : 0
    const costWeight = preference==='economy'?.42:preference==='speed'?.18:.30
    const measuredEfficiency=efficiency(model)
    const efficiencyBonus=bestEfficiency>0 && measuredEfficiency!==null ? 6*measuredEfficiency/bestEfficiency : 0
    const costScore=maxCost===null ? model.cost_score :
      maxCost===0 ? 0 : 100*measured.average_cost/maxCost
    return efficiencyBonus+(level<4?costWeight:.12)*(100-costScore)+(preference==='speed'?.25:.12)*model.speed_score+
      (level>=4?.38:.27)*model.reasoning_score+.25*model.coding_score+learning
  }
  const ranked = catalog.models.filter(eligible)
  const bestEfficiency=Math.max(0,...ranked.map(model=>efficiency(model)??0))
  const costs=ranked.map(model=>stats(model))
  const maxCost=costs.length && costs.every(row=>row.billed_samples>=3 &&
    row.cost_currency===costs[0].cost_currency && Number.isFinite(row.average_cost))
    ? Math.max(...costs.map(row=>row.average_cost)) : null
  ranked.sort((a,b)=>rank(b,bestEfficiency,maxCost)-rank(a,bestEfficiency,maxCost)||a.model_id.localeCompare(b.model_id))
  const pinned = catalog.models.find(model=>model.model_id===override)
  const fallback = level===1 && ranked.find(model=>model.model_id===catalog.default_model)
  const selected = override ? pinned : fallback || ranked[0]
  const valid = selected?.availability==='active' && chooseEffort(selected,Boolean(override))
  const currentEntry = catalog.models.find(model=>model.model_id===current)
  return {
    mode:'recommendation_only', catalog_version:catalog.version, estimated_task_difficulty:task.task_level,
    status:override&&!pinned?'UNKNOWN_OVERRIDE':override&&!valid?'OVERRIDE_UNAVAILABLE':valid?'RECOMMENDED':'NO_ELIGIBLE_MODEL',
    requested_override:override ?? null,
    task_effort:task.recommended_reasoning_effort, recommendation:valid?result(selected,Boolean(override)):null,
    alternatives:ranked.filter(model=>model!==selected).slice(0,3).map(model=>result(model)),
    override_below_task_requirements:Boolean(override && valid && !eligible(selected)),
    upgrade_suggested:Boolean(valid && !override && currentEntry && currentEntry!==selected && (!eligible(currentEntry)||selected.capability_level>currentEntry.capability_level)),
    reason:override?(valid?'沿用用户指定模型，任务适配性以已验证评分为准':'未知、停用或不支持该强度的配置需要人工确认'):fallback?'简单任务使用配置的默认模型':
      valid?'按能力门槛、配置的成本与速度评分及同级历史记录排序':'注册表缺少满足能力和推理强度要求的可用模型',
  }
}
