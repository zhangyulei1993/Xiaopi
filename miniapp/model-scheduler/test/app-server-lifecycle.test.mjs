import test from 'node:test'
import assert from 'node:assert/strict'
import {followTurn} from '../app-server-recovery.mjs'

const ids={threadId:'thread',turnId:'turn'}
const completed={method:'turn/completed',params:{threadId:ids.threadId,turn:{id:ids.turnId,status:'completed'}}}
const retry={method:'error',params:{...ids,willRetry:true}}
const exited={method:'process/exit'}

for(const [name,streams,expected,reconnects] of [
  ['retry notice keeps transport attached',[[retry,completed]],'COMPLETED',0],
  ['two disconnections restore original turn',[[],[],[completed]],'COMPLETED',2],
  ['reconnect exhaustion preserves unknown',[[],[]],'UNKNOWN',1],
  ['server exit is recorded before recovery',[[exited],[completed]],'COMPLETED',1],
]) {
  test(name,async()=>{
    const calls=[]
    let attaches=0,detaches=0
    const result=await followTurn({...ids,maxReconnects:reconnects,connect:async()=>({
      events:streams[attaches++],
      notify(method){assert.equal(method,'initialized')},
      request(method,params){
        calls.push(method)
        if(method==='initialize') return {}
        assert.equal(params.threadId,ids.threadId)
        if(method==='thread/resume') return {thread:{id:ids.threadId}}
        assert.equal(method,'thread/turns/list')
        return {data:[{id:ids.turnId,status:'inProgress'}]}
      },
      detach(){detaches++},
    })})
    assert.equal(result.state,expected)
    assert.equal(attaches,reconnects+1)
    assert.equal(detaches,attaches)
    assert.equal(calls.filter(method=>method==='thread/turns/list').length,attaches)
    assert.ok(result.timeline.some(entry=>entry.state==='RESUMING_TURN'))
    if(streams[0].includes(exited)) assert.ok(result.timeline.some(entry=>entry.processState==='EXITED'))
    if(streams[0].includes(retry)) assert.ok(result.timeline.some(entry=>entry.willRetry===true))
  })
}

for(const cancelled of [true,false]) {
  test(cancelled?'cancel waits for interrupted status':'deadline preserves running turn evidence',async()=>{
    const controller=new AbortController()
    if(cancelled) controller.abort()
    let interrupts=0
    const result=await followTurn({...ids,cancelSignal:controller.signal,timeoutMs:50,connect:async signal=>({
      async *eventsIterator(){
        if(cancelled) yield {...completed,params:{...completed.params,turn:{id:ids.turnId,status:'interrupted'}}}
        else await new Promise(resolve=>signal.addEventListener('abort',resolve,{once:true}))
      },
      get events(){return this.eventsIterator()},
      notify(){},
      async request(method,params){
        if(method==='initialize') return {}
        if(method==='thread/resume') return {thread:{id:ids.threadId}}
        if(method==='thread/turns/list') return {data:[{id:ids.turnId,status:'inProgress'}]}
        assert.equal(method,'turn/interrupt')
        assert.deepEqual(params,ids)
        interrupts++
      },
    })})
    assert.equal(result.state,cancelled?'CANCELLED':'UNKNOWN')
    assert.equal(result.lastKnownTurnStatus,cancelled?'interrupted':'inProgress')
    assert.equal(interrupts,Number(cancelled))
  })
}
