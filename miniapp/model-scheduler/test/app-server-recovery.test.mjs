import assert from 'node:assert/strict'
import {test} from 'node:test'
import {initializeConnection,observeTurn,withDeadline} from '../app-server-recovery.mjs'

const threadId='test-thread',turnId='test-turn'
const mock = (replies={}) => {
  const calls=[]
  return {calls,async request(method,params) {
    calls.push({method,params})
    assert.notEqual(method,'turn/start')
    if(replies[method] instanceof Error) throw replies[method]
    return replies[method]??{}
  },async notify(method) {calls.push({method})}}
}
const completed = (status='completed',id=turnId) => ({method:'turn/completed',params:{threadId,turn:{id,status}}})
test('connection performs the official initialize handshake',async()=>{
  const server=mock()
  await initializeConnection(server)
  assert.deepEqual(server.calls.map(call=>call.method),['initialize','initialized'])
})
test('transport failure leaves the turn unknown instead of inferring failure or success',async()=>{
  const turn=observeTurn(threadId,turnId)
  await assert.rejects(turn.recover(mock({'thread/resume':new Error('connection closed')})),/connection closed/)
  assert.deepEqual(turn.result(),{status:'UNKNOWN',success:false})
})
test('reconnect rejoins the same thread, reconciles state, then accepts completion',async()=>{
  const turn=observeTurn(threadId,turnId)
  const server=mock({'thread/resume':{thread:{id:threadId}},'thread/turns/list':{data:[{id:turnId,status:'inProgress'}]}})
  assert.equal(await turn.recover(server),'inProgress')
  assert.deepEqual(server.calls.map(call=>call.method),['initialize','initialized','thread/resume','thread/turns/list'])
  turn.event(completed())
  assert.equal(turn.result().success,true)
})
test('completion correlates both thread and turn and ignores retry notices',()=>{
  const turn=observeTurn(threadId,turnId)
  turn.event(completed('completed','another-turn'))
  turn.event({method:'error',params:{threadId,turnId,willRetry:true,error:{message:'Reconnecting... 2/5'}}})
  assert.equal(turn.result().status,'UNKNOWN')
  turn.event(completed())
  assert.equal(turn.result().status,'completed')
})
test('duplicate completion and cumulative usage are accounted for once',()=>{
  const turn=observeTurn(threadId,turnId)
  const event={method:'thread/tokenUsage/updated',params:{threadId,turnId,tokenUsage:{total:{inputTokens:25,outputTokens:4,cachedInputTokens:12}}}}
  turn.event(event)
  turn.event(event)
  turn.event(completed())
  turn.event(completed())
  assert.equal(turn.takeUsage().inputTokens,25)
  assert.equal(turn.takeUsage(),null)
})
test('observation timeout neither starts nor resubmits a turn',async()=>{
  const server=mock()
  const turn=observeTurn(threadId,turnId)
  await assert.rejects(withDeadline(new Promise(()=>{}),10),/deadline exceeded/)
  assert.deepEqual(server.calls,[])
  assert.equal(turn.result().status,'UNKNOWN')
})
test('interrupt acknowledgment does not mark success and interrupted is terminal',async()=>{
  const server=mock()
  const turn=observeTurn(threadId,turnId)
  await turn.interrupt(server)
  assert.equal(turn.result().success,false)
  turn.event(completed('interrupted'))
  turn.event(completed())
  assert.deepEqual(turn.result(),{status:'interrupted',success:false})
  assert.deepEqual(server.calls,[{method:'turn/interrupt',params:{threadId,turnId}}])
})
