import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..')
const reportDir=path.join(root,'tests/reports/build-scan')
const hash=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
function files(dir) { if(!fs.existsSync(dir)) return []; return fs.readdirSync(dir,{withFileTypes:true}).flatMap(e=>{const p=path.join(dir,e.name);return e.isDirectory()?files(p):e.isFile()?[p]:[]}) }
function inputs() { const names=['package.json','pnpm-lock.yaml','package-lock.json','yarn.lock','vite.config.ts','vite.config.js','tsconfig.json','project.config.json','project.private.config.json']; return [...files(path.join(root,'src')),...files(path.join(root,'scripts')),...names.map(n=>path.join(root,n)).filter(fs.existsSync)].sort() }
const snapshot=paths=>Object.fromEntries(paths.map(f=>[path.relative(root,f),hash(f)]))
fs.mkdirSync(reportDir,{recursive:true})
const before=snapshot(inputs())
const startedAt=new Date().toISOString()
const result=spawnSync('npm',['run','build:mp-weixin:test'],{cwd:root,encoding:'utf8',maxBuffer:8*1024*1024})
process.stdout.write(result.stdout||''); process.stderr.write(result.stderr||'')
const after=snapshot(inputs())
const unchanged=JSON.stringify(before)===JSON.stringify(after)
const output=path.join(root,'dist/mp-weixin-test')
const report={startedAt,completedAt:new Date().toISOString(),command:'npm run build:mp-weixin:test',node:process.version,exitCode:result.status,inputUnchanged:unchanged,sourceBefore:before,sourceAfter:after,artifacts:fs.existsSync(output)?snapshot(files(output)): {},scope:'build input/output integrity; not per-line source mapping'}
fs.writeFileSync(path.join(reportDir,'build-provenance.json'),JSON.stringify(report,null,2)+'\n')
if(result.status!==0 || !unchanged || !fs.existsSync(path.join(output,'app.json'))) process.exitCode=1
