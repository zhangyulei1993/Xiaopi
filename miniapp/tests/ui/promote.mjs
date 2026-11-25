import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createHash } from 'node:crypto'
import { deviceFrameMasks } from './device-frame.mjs'
import { readPng } from './pixels.mjs'

const root = fileURLToPath(new URL('../../', import.meta.url))
const [runId, version, reason, frameApproval] = process.argv.slice(2)
assert.match(runId || '', /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$/)
assert.match(version || '', /^[a-z0-9-]+$/)
assert.ok(reason?.trim(), 'An explicit reason is required to create a baseline version')
const source = path.join(root,'tests/reports/ui',runId)
const report = JSON.parse(fs.readFileSync(path.join(source,'result.json')))
assert.equal(report.runId,runId)
assert.ok(frameApproval === undefined || (frameApproval === '--approve-device-frame' && report.baselineId.startsWith('extended-')))
assert.ok(['CANDIDATE','PARTIAL_CANDIDATE'].includes(report.status))
assert.ok(report.cases.length && report.cases.every(c=>['CANDIDATE','BLOCKED'].includes(c.status)))
const images = report.cases.filter(c=>c.status==='CANDIDATE').map(({name,route,window,masks})=>{
  assert.match(name,/^[a-z0-9-]+$/)
  const file = path.join(source,name+'.png')
  const {width,height} = readPng(file)
  assert.deepEqual([width,height],[604,1312])
  const bytes = fs.readFileSync(file)
  return {name,route,window,masks:frameApproval ? [...masks,...deviceFrameMasks] : (masks || report.masks),bytes,sha256:createHash('sha256').update(bytes).digest('hex')}
})
assert.ok(images.length,'No candidate screenshots to approve')
const destination = path.join(root,'tests/ui/baselines',version)
fs.mkdirSync(path.dirname(destination),{recursive:true})
fs.mkdirSync(destination)
for (const {name,bytes} of images) fs.writeFileSync(path.join(destination,name+'.png'),bytes,{flag:'wx'})
fs.writeFileSync(path.join(destination,'manifest.json'),JSON.stringify({
  version,runId,reason,createdAt:new Date().toISOString(),device:report.device,masks:report.masks,
  blockedCases:report.cases.filter(c=>c.status==='BLOCKED').map(({name,findings})=>({name,findings})),
  images:images.map(({bytes,...info})=>info)
},null,2)+'\n',{flag:'wx'})
console.log(destination)
