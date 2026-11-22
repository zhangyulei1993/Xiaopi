import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createHash } from 'node:crypto'
import { auditTree } from './audit-network-source.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const output = path.resolve(process.argv[2] || path.join(root, 'dist/mp-weixin-test'))
const reportDir = path.join(root, 'tests/reports/build-scan')
fs.mkdirSync(reportDir, { recursive: true })
const report = { output, status: 'PASS', files: 0, networkCalls: 0, allowed: [], rejected: [], dynamicRisks: [], artifactSha256: {}, buildProvenance: 'BLOCKED: no verified pre-build/post-build source manifest' }
const fail = message => { report.status = 'BLOCKED'; report.dynamicRisks.push(message) }
const proofPath = path.join(reportDir, 'build-provenance.json')
try {
  const proof = JSON.parse(fs.readFileSync(proofPath, 'utf8'))
  if (proof.exitCode !== 0 || !proof.inputUnchanged || proof.command !== 'npm run build:mp-weixin:test') throw new Error('build did not complete with unchanged inputs')
  for (const [name, digest] of Object.entries({...proof.sourceAfter, ...proof.artifacts})) {
    const resolved = path.resolve(root, name)
    if (!resolved.startsWith(root + path.sep) || !fs.existsSync(resolved) || createHash('sha256').update(fs.readFileSync(resolved)).digest('hex') !== digest) throw new Error(`missing or changed: ${name}`)
  }
  const list = dir => !fs.existsSync(dir) ? [] : fs.readdirSync(dir, {withFileTypes:true}).flatMap(entry => {
    const file = path.join(dir, entry.name)
    return entry.isDirectory() ? list(file) : entry.isFile() ? [path.relative(root, file)] : []
  })
  const configNames = ['package.json','pnpm-lock.yaml','package-lock.json','yarn.lock','vite.config.ts','vite.config.js','tsconfig.json','project.config.json','project.private.config.json']
  const inputNames = [...list(path.join(root,'src')), ...list(path.join(root,'scripts')), ...configNames.filter(name => fs.existsSync(path.join(root,name)))].sort()
  if (JSON.stringify(inputNames) !== JSON.stringify(Object.keys(proof.sourceAfter).sort())) throw new Error('build input file list changed')
  if (JSON.stringify(list(path.join(root,'dist/mp-weixin-test')).sort()) !== JSON.stringify(Object.keys(proof.artifacts).sort())) throw new Error('build output file list changed')
  report.buildProvenance = {status:'PASS', startedAt:proof.startedAt, scope:proof.scope}
} catch (error) { fail(`build provenance invalid: ${error.message}`) }
const packageJSON = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'))
const build = packageJSON.scripts['build:mp-weixin:test'] || ''
const match = build.match(/(?:^|\s)VITE_API_BASE=([^\s]+)/)
const expected = match?.[1]
if (!expected) fail('test build has no statically configured API base')
if (output !== path.resolve(root, 'dist/mp-weixin-test')) fail('only the project test build directory is permitted')
if (!fs.existsSync(path.join(output, 'app.json'))) fail('test build app.json is missing')
report.sourceCalls = auditTree(path.join(root, 'src'))
const sources = []
function walk(dir) { for (const e of fs.readdirSync(dir, { withFileTypes: true })) { const file = path.join(dir, e.name); if (e.isDirectory()) walk(file); else if (e.isFile() && /\.(js|json|wxml|wxss)$/i.test(e.name)) { const text=fs.readFileSync(file, 'utf8'); const name=path.relative(output,file); report.files++; report.artifactSha256[name]=createHash('sha256').update(text).digest('hex'); sources.push({ file: name, text }) } } }
if (fs.existsSync(output)) walk(output)
const combined = sources.map(x => x.text).join('\n')
if (expected && !combined.includes(expected)) fail('configured test API not found in output')
if (combined.includes('api.xiaopizufang.cn')) { report.rejected.push('production API domain present in output'); report.status = 'FAIL' }
// Pins cover the reviewed test callers and their validators. A rebuild does not refresh them.
const reviewed = {
  'api/client.js': '769f8c9eec21264099bf956779526661de51f254d2079afc01b7996e358f722d',
  'api/request-url.js': '0717acf313cd34bbb238750ba2fa8d5e9598f73b560b219ce28c23f72bd8e97e',
  'api/upload-url.js': '0483e3525aa2ff158e23c90b0025c68e6af41057c0fc8d45224da815cc3cef6e',
  'pages/admin/property-edit.js': '5430cd5fb5edd350d99c380124d2fd47e2f35a7b82e49e7cb16b9fd903184a8d',
  'pages/auth/login.js': '49dc6d0164e4c393b6df5939e740fd29aa006b2162364ad3a690e320f6b67404',
  'pages/me/index.js': '90e096aa97385064aa1beec52ab046d593a1b7846a882579648f2dd5a6f8d98b',
  'pages/properties/detail.js': '5c768dbbfd8f192a8e901999f3581574d23e78ea71dd826b0f5c02e775069398',
}
const verified = report.buildProvenance.status === 'PASS'
  && Object.entries(reviewed).every(([file, digest]) => report.artifactSha256[file] === digest)
  && report.sourceCalls.length === 5
  && report.sourceCalls.every(call => call.originVerified && call.importAlias && call.directGuard && !call.reassigned)
const matched = new Set()
report.guardedCalls = []
for (const {file, text} of sources) {
  if (!file.endsWith('.js')) continue
  const re = /(?:\b(?:uni|wx)|\b\w+\.index)\.(request|uploadFile|downloadFile|connectSocket)\s*\(|\bfetch\s*\(|\bnew\s+XMLHttpRequest\s*\(/g
  for (const m of text.matchAll(re)) {
    report.networkCalls++
    const source = report.sourceCalls.find(call =>
      path.relative(path.join(root, 'src'), call.file).replace(/\.(vue|ts)$/, '.js') === file
      && call.callee === 'uni.' + m[1])
    if (!verified || !Object.hasOwn(reviewed, file) || !source || matched.has(source)) {
      fail(`${file}: ${m[0]} dynamic target requires review`)
      continue
    }
    matched.add(source)
    report.guardedCalls.push({ file, offset: m.index, source: source.file, line: source.line })
  }
}
if (matched.size !== report.sourceCalls.length) fail('Source calls do not match reviewed output')
if (expected && report.status === 'PASS') report.allowed.push(expected)
fs.writeFileSync(path.join(reportDir, 'scan-report.json'), JSON.stringify(report, null, 2) + '\n')
fs.writeFileSync(path.join(reportDir, 'scan-report.md'), `# Build security scan\n\nStatus: ${report.status}\n\nFiles: ${report.files}\n\nNetwork calls: ${report.networkCalls}\n\nConfigured API: ${expected || 'unknown'}\n\nDynamic risks:\n${report.dynamicRisks.map(x => `- ${x}`).join('\n')}\n`)
console.log(JSON.stringify({status: report.status, files: report.files, networkCalls: report.networkCalls, report: path.join(reportDir, 'scan-report.json')}))
if (report.status !== 'PASS') process.exitCode = 1
