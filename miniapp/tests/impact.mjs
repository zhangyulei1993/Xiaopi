import fs from 'node:fs'
import path from 'node:path'
import { spawnSync, execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const root = fileURLToPath(new URL('../', import.meta.url))
const repo = path.dirname(root.slice(0, -1))
const args = process.argv.slice(2)
const git = (...args) => execFileSync('git', args, { cwd: repo, encoding: 'utf8' })
const stdinFiles = args.includes('--files-stdin')
const files = [...new Set(stdinFiles ? fs.readFileSync(0,'utf8').split('\0').filter(Boolean) :
  args.filter(arg => arg.startsWith('--file=')).map(arg => arg.slice(7)))]
const moduleName = args.find(arg => arg.startsWith('--module='))?.slice(9)
if (args.includes('--module') || moduleName === '' || args.includes('--require-module') && !moduleName) throw new Error('Use --module=home|listings|favorites|appointment')
const analyze = args.includes('--plan')
if ((files.length || stdinFiles) && !analyze) throw new Error('--file is only supported in plan mode')
if (!files.length && !moduleName && !stdinFiles) files.push(...new Set(git('diff','--no-renames','--name-only','-z','HEAD').concat(git('ls-files','--others','--exclude-standard','-z')).split('\0').filter(Boolean)))
const modules = {
  home: { pages: ['home/index'], cases: ['home-load'] },
  listings: { pages: ['properties/index','properties/detail'], cases: ['property-list','property-detail'] },
  favorites: { pages: ['home/index','properties/index','properties/detail','me/index'], cases: ['home-load','property-list','property-detail','favorite-add-remove'] },
  appointment: { pages: ['properties/detail','requests/index','requests/history','admin/index'], cases: ['appointment-form','appointment-time-validation','appointment-submit','my-appointments','my-appointments-linked-property','appointment-property-association','my-footprints-read','general-form-validation','my-appointments-guest-gate','property-detail'] },
}
const normalize = file => file.split(path.sep).join('/')
const sources = fs.readdirSync(path.join(root,'src'),{recursive:true}).map(file=>normalize(path.join('miniapp/src',file)))
  .filter(file=>/\.(vue|ts|js|mjs|scss|css)$/.test(file))
const imports = new Map()
for (const file of sources) {
  const raw = fs.readFileSync(path.join(repo,file),'utf8')
  const script = file.endsWith('.vue') ? [...raw.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script>/g)].map(match=>match[1]).join('\n') : raw
  const edges = ts.preProcessFile(script,true,true).importedFiles.map(entry=>entry.fileName)
  for (const match of raw.matchAll(/@(?:use|import|forward)\s+['"]([^'"]+)|<style\b[^>]*\bsrc=['"]([^'"]+)/g)) edges.push(match[1] || match[2])
  imports.set(file, edges.filter(edge=>edge.startsWith('.') || edge.startsWith('@/')).map(edge=>
    normalize(edge.startsWith('@/') ? path.join('miniapp/src',edge.slice(2)) : path.join(path.dirname(file),edge))))
}
const apiExports = new Map()
const apiFile = path.join(root,'src/api/properties.ts')
const apiAst = ts.createSourceFile(apiFile,fs.readFileSync(apiFile,'utf8'),ts.ScriptTarget.Latest,true)
for (const node of apiAst.statements) {
  if (!node.modifiers?.some(modifier=>modifier.kind===ts.SyntaxKind.ExportKeyword)) continue
  const names = node.declarationList?.declarations.map(declaration=>declaration.name.getText(apiAst)) || (node.name ? [node.name.getText(apiAst)] : [])
  const endpoints = [...node.getText(apiAst).matchAll(/\/v1\/[a-zA-Z0-9_/-]+/g)].map(match=>match[0])
  for (const name of names) if (endpoints.length) apiExports.set(name,endpoints)
}
const routes = JSON.parse(fs.readFileSync(path.join(root,'src/pages.json'))).pages.map(page=>page.path)
const cases = new Set()
const reasons = []
let full = false
let backend = false
function add(name) {
  const entry = modules[name]
  if (!entry) throw new Error('Unknown module: ' + name)
  for (const test of entry.cases) cases.add(test)
}
if (moduleName) add(moduleName)
for (const file of files) {
  const affected = new Set([file])
  for (let changed = true; changed;) {
    changed = false
    for (const [source, edges] of imports) {
      if (!affected.has(source) && edges.some(edge=>[...affected].some(target=>target===edge || target.replace(/\.[^.\/]+$/,'')===edge || target.replace(/\/index\.[^.\/]+$/,'')===edge))) {
        affected.add(source)
        changed = true
      }
    }
  }
  const pages = routes.filter(route=>affected.has('miniapp/src/'+route+'.vue')).map(route=>route.slice(6))
  const sample = args.find(arg => arg.startsWith('--example='))?.slice(10)
  const diff = sample && analyze ? sample : git('diff','--unified=0','HEAD','--',file)
  const symbols = [...diff.matchAll(/^@@.*?@@\s*(.+)$/gm)].map(match=>match[1].trim())
  const edits = diff.split('\n').filter(line=>/^[+-](?![+-])/.test(line)).join('\n')
  const navigationTargets = routes.filter(route=>edits.includes('/'+route)).map(route=>route.slice(6))
  for (const target of navigationTargets) if (!pages.includes(target)) pages.push(target)
  const candidates = new Set()
  if (pages.length===1 && pages[0]==='home/index' && file.endsWith('/home/index.vue')) candidates.add('home')
  else for (const [name, entry] of Object.entries(modules)) if (pages.some(page=>entry.pages.includes(page))) candidates.add(name)
  if (/favorite|收藏|关注/i.test(edits+' '+symbols.join(' '))) candidates.add('favorites')
  if (/appointment|预约/i.test(edits+' '+symbols.join(' '))) candidates.add('appointment')
  const api = Object.fromEntries([...apiExports].filter(([name])=>new RegExp('\\b'+name+'\\b').test(edits+' '+symbols.join(' '))))
  const roleSensitive = /\b(?:hasRole|getSession|saveSession|clearSession|isAdmin|requireRoles)\b/.test(edits)
  const unknown = !pages.length || pages.some(page=>!Object.values(modules).some(entry=>entry.pages.includes(page)))
  const shared = /^(miniapp\/src\/(api|auth|store|stores)\/|backend\/)|(?:pages\.json|App\.vue|uni\.scss)$/.test(file)
  if (file.startsWith('backend/')) backend = true
  if (unknown || shared || roleSensitive) full = true
  for (const name of candidates) add(name)
  reasons.push({file,symbols,pages,reverse_dependencies:[...affected].filter(source=>source!==file),navigationTargets,api,roleSensitive,modules:[...candidates],reason:shared?'共享契约或后端变更':unknown?'映射缺失，扩大回归':'页面及反向导入依赖'})
}
if (!cases.size) full = true
const scope = full ? 'full' : moduleName ? 'module' : cases.size===1 ? 'local' : 'related'
const homeOnly = !full && cases.size === 1 && cases.has('home-load')
const commands = [
  ...(backend ? [['go',['test','./...'],path.join(repo,'backend')]] : []),
  ['npm',['run','test',...(full ? [] : ['--','--cases='+[...cases].join(','),homeOnly ? '--visual=home' : '--visual=core'])],root],
  ...(full ? [['npm',['run','test:ui:device'],root]] : []),
]
const allBusiness = JSON.parse(fs.readFileSync(path.join(root,'tests/fixtures/business-coverage.json'))).scenarios
  .filter(scenario=>scenario.status !== 'NOT_APPLICABLE').map(scenario=>scenario.name)
const unverified = backend ? ['运行中的隔离 API 未验证与后端源码一致，数据库迁移未重新应用'] : []
const plan = {
  scope, module:moduleName, reasons, business:full?allBusiness:[...cases],
  visual:full?'core, extended, archived device':homeOnly?'home baseline':'core baseline group',
  skipped:full?[]:[
    ...allBusiness.filter(test=>!cases.has(test)).map(test=>({test,reason:'未命中模块与依赖映射'})),
    {test:'权限、扩展视觉与归档真机专项',reason:'当前范围未触发全量回归'},
  ],
  unverified, commands:commands.map(([cmd,argv])=>[cmd,...argv]),
}
console.log(JSON.stringify(plan,null,2))
if (!analyze) {
  let failed = false
  for (const [command,argv,cwd] of commands) {
    const result = spawnSync(command,argv,{cwd,stdio:'inherit'})
    console.log(JSON.stringify({scope,command:[command,...argv],exitCode:result.status}))
    if (result.status!==0) {
      failed = true
      if (argv.includes('test:build-scan')) break
    }
  }
  process.exitCode = failed || unverified.length ? 1 : 0
}
