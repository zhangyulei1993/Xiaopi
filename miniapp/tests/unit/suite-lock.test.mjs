import assert from 'node:assert/strict'
import { spawn, spawnSync } from 'node:child_process'
import { once } from 'node:events'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { test } from 'node:test'
import { executionPlan } from '../execution-plan.mjs'

const helper = new URL('../suite-lock.py', import.meta.url).pathname
const moduleUrl = new URL('../suite-lock.mjs', import.meta.url).href
const cleanEnv = {...process.env}
delete cleanEnv.XIAOPI_SUITE_LOCK
const setup = t => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(),'xiaopi-lock-test-'))
  t.after(()=>fs.rmSync(dir,{force:true,recursive:true}))
  const lock = path.join(dir,'lock')
  const fixture = path.join(dir,'runner.mjs')
  fs.writeFileSync(fixture, `
    import {enterSuiteLock,suiteChildOptions} from ${JSON.stringify(moduleUrl)}
    import {spawn,spawnSync} from 'node:child_process'
    const [file,mode,...suites]=process.argv.slice(2)
    const lock=enterSuiteLock('fixture',file)
    if (mode==='orphan-parent') {
      const options=suiteChildOptions(lock)
      const child=spawn(process.execPath,[process.argv[1],file,'hold'],{...options,stdio:['ignore','inherit','inherit',lock.fd]})
      child.unref()
      setInterval(()=>{},1000)
    } else if (mode==='full') {
      for(const suite of suites) {
        const child=spawnSync(process.execPath,[process.argv[1],file,suite],suiteChildOptions(lock))
        if(child.status!==0) throw Error(child.stderr.toString())
        process.stdout.write(child.stdout)
      }
    } else {
      console.log(JSON.stringify({mode,executionId:lock.executionId,pid:process.pid,owner:JSON.parse(process.env.XIAOPI_SUITE_LOCK).pid}))
      if(mode==='hold') setInterval(()=>{},1000)
    }`)
  return {lock, fixture}
}
test('full plan gives each visual child the same lock and execution ID exactly once', t=>{
  const {lock,fixture}=setup(t)
  const suites=executionPlan({full:true}).filter(name=>name.endsWith('-ui')&&name!=='role-ui')
  const r=spawnSync(process.execPath,[fixture,lock,'full',...suites],{env:cleanEnv,encoding:'utf8'})
  assert.equal(r.status,0,r.stderr)
  const rows=r.stdout.trim().split('\n').map(JSON.parse)
  assert.deepEqual(rows.map(row=>row.mode),['core-ui','extended-ui'])
  assert.ok(rows.every(row=>row.executionId==='fixture'))
  assert.equal(new Set(rows.map(row=>row.owner)).size,1)
})
test('independent visual and full runs are excluded, and SIGKILL releases the lock',async t=>{
  const {lock,fixture}=setup(t)
  const owner=spawn('python3',[helper,lock,'owner',process.execPath,fixture,lock,'hold'],{env:cleanEnv})
  t.after(()=>owner.kill('SIGKILL'))
  await once(owner.stdout,'data')
  for(const mode of ['core-ui','extended-ui','full']) {
    const r=spawnSync(process.execPath,[fixture,lock,mode],{env:cleanEnv,encoding:'utf8'})
    assert.notEqual(r.status,0)
    assert.match(r.stderr,/already holds/)
  }
  const ended=once(owner,'exit')
  owner.kill('SIGKILL')
  await ended
  const recovered=spawnSync(process.execPath,[fixture,lock,'core-ui'],{env:cleanEnv,encoding:'utf8'})
  assert.equal(recovered.status,0,recovered.stderr)
})

test('a surviving child keeps the lock after its parent is killed', {timeout:5000}, async t=>{
  const {lock,fixture}=setup(t)
  const owner=spawn('python3',[helper,lock,'owner',process.execPath,fixture,lock,'orphan-parent'],{env:cleanEnv})
  t.after(()=>owner.kill('SIGKILL'))
  const [line]=await once(owner.stdout,'data')
  const {pid}=JSON.parse(line)
  t.after(()=>{try { process.kill(pid,'SIGKILL') } catch {}})
  const ended=once(owner,'exit')
  owner.kill('SIGKILL')
  await ended
  const blocked=spawnSync(process.execPath,[fixture,lock,'core-ui'],{env:cleanEnv,encoding:'utf8'})
  assert.match(blocked.stderr,/already holds/)
  process.kill(pid,'SIGKILL')
  await once(owner.stdout,'end')
  const recovered=spawnSync(process.execPath,[fixture,lock,'core-ui'],{env:cleanEnv,encoding:'utf8'})
  assert.equal(recovered.status,0,recovered.stderr)
})
