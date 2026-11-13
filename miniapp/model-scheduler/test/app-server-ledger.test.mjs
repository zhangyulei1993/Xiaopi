import test from 'node:test'
import assert from 'node:assert/strict'
import {mkdtemp,readdir,readFile,rm,writeFile} from 'node:fs/promises'
import {tmpdir} from 'node:os'
import {join} from 'node:path'
import {execFile} from 'node:child_process'
import {promisify} from 'node:util'

test('independent observers can publish terminal usage only once',async()=>{
  const dir=await mkdtemp(join(tmpdir(),'turn-ledger-'))
  const ledger=new URL('../app-server-ledger.mjs',import.meta.url)
  const recovery=new URL('../app-server-recovery.mjs',import.meta.url)
  const program=`
    import {recordTurnUsage} from ${JSON.stringify(ledger.href)}
    import {observeTurn} from ${JSON.stringify(recovery.href)}
    const observer=observeTurn('original-thread','original-turn')
    observer.event({method:'thread/tokenUsage/updated',params:{threadId:'original-thread',turnId:'original-turn',tokenUsage:{total:{inputTokens:12,outputTokens:4,cachedInputTokens:2}}}})
    for(let i=0;i<2;i++) observer.event({method:'turn/completed',params:{threadId:'original-thread',turn:{id:'original-turn',status:'completed'}}})
    console.log(await recordTurnUsage(process.argv[1],observer.takeUsage()))
  `
  try {
    const results=await Promise.all(Array.from({length:2},()=>promisify(execFile)(process.execPath,['--input-type=module','-e',program,dir])))
    assert.deepEqual(results.map(result=>result.stdout.trim()).sort(),['false','true'])
    const restarted=await promisify(execFile)(process.execPath,['--input-type=module','-e',program,dir])
    assert.equal(restarted.stdout.trim(),'false')
    const files=await readdir(dir)
    assert.equal(files.length,1)
    const usage=JSON.parse(await readFile(join(dir,files[0]),'utf8'))
    assert.equal(usage.inputTokens,12)
    assert.equal(usage.scope,'thread_total')
  } finally { await rm(dir,{recursive:true,force:true}) }
})


test('ledger write failure is explicit and can be retried',async()=>{
  const {persistObservedUsage}=await import('../app-server-ledger.mjs')
  const dir=await mkdtemp(join(tmpdir(),'turn-ledger-fail-'))
  const blocked=join(dir,'blocked')
  const usage={threadId:'thread',turnId:'turn',scope:'thread_total',inputTokens:12}
  try {
    await writeFile(blocked,'not a directory')
    const result=await persistObservedUsage(blocked,usage)
    assert.equal(result.status,'FAILED')
    assert.equal(result.turnTokens,'UNAVAILABLE')
    assert.equal(result.eventDelta,'UNAVAILABLE')
    assert.equal(result.actualCost,'UNAVAILABLE')
    assert.equal((await persistObservedUsage(dir,usage)).status,'RECORDED')
    assert.equal((await persistObservedUsage(dir,usage)).status,'ALREADY_RECORDED')
  } finally { await rm(dir,{recursive:true,force:true}) }
})
