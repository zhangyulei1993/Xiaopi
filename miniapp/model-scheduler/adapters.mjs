import fs from 'node:fs'
import path from 'node:path'
import {execFileSync} from 'node:child_process'

export function gitChanges(repo,{selected=[],planned=false,ignored=/$^/}={}) {
  const git=(...argv)=>execFileSync('git',argv,{cwd:repo,encoding:'utf8',maxBuffer:32*1024*1024})
  if(planned && !selected.length) throw new Error('--planned requires at least one --file')
  const include=file=>selected.length ? selected.includes(file) :
    !ignored.test(file)
  const changes=new Map()
  let base=[]
  if(!planned) {
    try { git('rev-parse','--verify','--quiet','HEAD'); base=['HEAD'] }
    catch(error) { if(error.status!==1) throw error }
  }
  for(const row of (planned?'':git('diff','--numstat','--no-renames','-z',...base)).split('\0').filter(Boolean)) {
    const [,add,remove,file]=row.match(/^([^\t]+)\t([^\t]+)\t([\s\S]+)$/)
    if(include(file)) changes.set(file,{file,lines:(Number(add)||0)+(Number(remove)||0)})
  }
  for(const file of (planned?'':git('ls-files','--others','--exclude-standard','-z')).split('\0').filter(Boolean)) {
    if(!include(file)) continue
    const stat=fs.lstatSync(path.join(repo,file))
    const code=/\.(vue|ts|js|mjs|go|css|scss|sql)$/.test(file)
    const lines=stat.isFile() && code && stat.size<=1024*1024
      ? fs.readFileSync(path.join(repo,file),'utf8').split('\n').filter(Boolean).length : 0
    changes.set(file,{file,lines})
  }
  for(const file of selected) if(!changes.has(file)) changes.set(file,{file,lines:0})
  return [...changes.values()]
}

export function miniappImpact(root,changes,planned) {
  const files=changes.map(row=>row.file)
  return JSON.parse(execFileSync(process.execPath,[path.join(root,'tests/impact.mjs'),'--plan','--files-stdin',...(planned?['--example=+planned change']:[])],{cwd:root,input:files.join('\0'),encoding:'utf8',maxBuffer:32*1024*1024}))
}

export function goImpact(project,changes) {
  project=fs.realpathSync(project)
  const full = (reason, reasons=[]) => ({
    scope:'full', dependency_graph:reason, reasons, commands:[['go','test','./...']],
  })
  const env={...process.env,GOPROXY:'off',GOSUMDB:'off',GOTOOLCHAIN:'local',GOWORK:'off',GOFLAGS:'-mod=readonly'}
  let packages, prefix
  try {
    prefix=execFileSync('git',['rev-parse','--show-prefix'],{cwd:project,encoding:'utf8'}).trim()
    const format='{{if .Error}}true{{else}}false{{end}}\t{{if .DepsErrors}}true{{else}}false{{end}}\t{{printf "%q" .ImportPath}}\t{{printf "%q" .Dir}}\t{{printf "%q" (join .Imports " ")}}\t{{printf "%q" (join .TestImports " ")}}\t{{printf "%q" (join .XTestImports " ")}}'
    const output=execFileSync('go',['list','-e','-f',format,'./...'],{
      cwd:project,env,encoding:'utf8',timeout:30000,maxBuffer:16*1024*1024,stdio:['ignore','pipe','pipe'],
    })
    packages=output.trim().split('\n').filter(Boolean).map(line=>{
      const [ownError,dependencyErrors,name,dir,...imports]=line.split('\t').map(JSON.parse)
      return {ownError,dependencyErrors,name,dir:path.relative(project,dir)||'.',imports:imports.flatMap(list=>list.split(' ').filter(Boolean))}
    })
  } catch {
    return full('UNAVAILABLE')
  }
  if(!packages.length || packages.some(pkg=>pkg.ownError)) return full('UNAVAILABLE')
  const module=execFileSync('go',['list','-m'],{cwd:project,env,encoding:'utf8',timeout:30000}).trim()
  const known=new Set(packages.map(pkg=>pkg.name))
  if(packages.some(pkg=>pkg.imports.some(name=>name.startsWith(module+'/') && !known.has(name)))) return full('UNAVAILABLE')
  const dependency_status=packages.some(pkg=>pkg.dependencyErrors)?'EXTERNAL_DEPENDENCIES_UNAVAILABLE':'available'
  const byDir=new Map(packages.map(pkg=>[pkg.dir,pkg]))
  const callers=new Map(packages.map(pkg=>[pkg.name,[]]))
  for(const pkg of packages) for(const dependency of new Set(pkg.imports)) callers.get(dependency)?.push(pkg.name)
  const selected=new Set()
  const reasons=[]
  let unknown=false
  for(const {file} of changes) {
    const relative=file.startsWith(prefix)?file.slice(prefix.length):null
    const owner=relative?.endsWith('.go') && byDir.get(path.posix.dirname(relative))
    if(!owner) {
      unknown=true
      reasons.push({file,modules:[],reverse_dependencies:[],reason:'Package could not be mapped conservatively'})
      continue
    }
    const closure=new Set([owner.name])
    for(const dependency of closure) for(const caller of callers.get(dependency)) closure.add(caller)
    for(const pkg of closure) selected.add(pkg)
    reasons.push({file,modules:[...closure].sort(),reverse_dependencies:[...closure].filter(name=>name!==owner.name).sort()})
  }
  if(unknown || !selected.size || selected.size===packages.length) return {...full('available',reasons),dependency_status}
  return {scope:'local',dependency_graph:'available',dependency_status,reasons,commands:[['go','test',...[...selected].sort()]]}
}
