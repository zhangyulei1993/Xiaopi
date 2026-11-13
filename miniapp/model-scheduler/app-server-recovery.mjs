// Transport adapters correlate request IDs and deliver decoded notifications in order.
// This observer is deliberately incapable of starting a turn.
export async function initializeConnection(transport) {
  await transport.request('initialize',{clientInfo:{name:'xiaopi-turn-observer',version:'1'}})
  await transport.notify('initialized')
}

export function observeTurn(threadId,turnId) {
  let status='UNKNOWN', usage=null, charged=false
  const terminal=new Set(['completed','failed','interrupted'])
  const acceptStatus = value => {
    if(!terminal.has(status) && (terminal.has(value)||value==='inProgress')) status=value
  }
  return {
    event({method,params={}}) {
      if(params.threadId!==threadId) return
      if(method==='turn/completed' && params.turn?.id===turnId) acceptStatus(params.turn.status)
      if(method==='thread/tokenUsage/updated' && params.turnId===turnId && !charged) {
        // This counter is cumulative, not an increment. Scope is the thread.
        const total=params.tokenUsage?.total
        if(total && ['inputTokens','outputTokens','cachedInputTokens'].every(key=>
          Number.isSafeInteger(total[key]) && total[key]>=0) && total.cachedInputTokens<=total.inputTokens &&
          (!usage || ['inputTokens','outputTokens','cachedInputTokens'].every(key=>total[key]>=usage[key]))) {
          usage={inputTokens:total.inputTokens,outputTokens:total.outputTokens,cachedInputTokens:total.cachedInputTokens}
        }
      }
    },
    async recover(transport,{recoverable=true,onPhase=()=>{}}={}) {
      if(!recoverable) return 'UNKNOWN'
      await initializeConnection(transport)
      // Caller wires notifications to event() before resuming the original thread.
      onPhase('RESUMING_THREAD')
      const resumed=await transport.request('thread/resume',{threadId,excludeTurns:true})
      if(resumed.thread?.id!==threadId) throw new Error('Resumed thread ID mismatch')
      onPhase('RESUMING_TURN')
      const cursors=new Set()
      let cursor
      for(let page=0;page<20;page++) {
        const response=await transport.request('thread/turns/list',{threadId,limit:20,itemsView:'summary',...(cursor?{cursor}:{})})
        const turn=response.data.find(item=>item.id===turnId)
        if(turn) { acceptStatus(turn.status); break }
        cursor=response.nextCursor
        if(!cursor||cursors.has(cursor)) break
        cursors.add(cursor)
      }
      return status
    },
    interrupt(transport) {
      return transport.request('turn/interrupt',{threadId,turnId})
    },
    result() { return {status,success:status==='completed'} },
    takeUsage() {
      if(charged||!terminal.has(status)||!usage) return null
      charged=true
      return {threadId,turnId,scope:'thread_total',...usage}
    },
  }
}

export function withDeadline(promise, milliseconds) {
  let timer
  return Promise.race([promise,new Promise((_,reject)=>{
    timer=setTimeout(()=>reject(new Error('Observation deadline exceeded')),milliseconds)
  })]).finally(()=>clearTimeout(timer))
}

// connect attaches to an independently owned server. It must honor the signal,
// buffer notifications before resume, and detach without terminating the server.
export async function followTurn({threadId, turnId, connect, timeoutMs=60000,
  maxReconnects=3, onTransition=()=>{}, cancelSignal, observer=observeTurn(threadId,turnId)}) {
  const lifetime=new AbortController()
  let state='UNKNOWN', connectionState='DISCONNECTED', processState='UNKNOWN'
  let cancelPending=Boolean(cancelSignal?.aborted), cancelSent=false, attempts=0
  const timeline=[]
  function transition(next, {method=null, willRetry=null, recovery=null, rawError=null, exitCode=null}={}) {
    state=next
    const entry={timestamp:new Date().toISOString(),threadId,turnId,state,method,willRetry,
      processState,connectionState,recovery,rawError,exitCode}
    timeline.push(entry)
    onTransition(entry)
  }
  const terminal=()=>({completed:'COMPLETED',failed:'FAILED',interrupted:'CANCELLED'})[observer.result().status]
  const cancel=()=>{cancelPending=true}
  cancelSignal?.addEventListener('abort',cancel)
  async function run() {
    while (!lifetime.signal.aborted) {
      let transport
      try {
        if(attempts) transition('RECONNECTING')
        transport=await connect(lifetime.signal)
        if(lifetime.signal.aborted) return
        connectionState='CONNECTED'
        processState='RUNNING'
        transition('CONNECTED')
        await observer.recover(transport,{onPhase:phase=>{
          if(!lifetime.signal.aborted) transition(phase)
        }})
        if(lifetime.signal.aborted) return
        if(terminal()) return transition(terminal(),{recovery:'turn_query'})
        transition(observer.result().status==='inProgress'?'RUNNING':'UNKNOWN',{recovery:'turn_query'})
        const requestCancel=()=>{
          cancel()
          if(!cancelSent) {
            cancelSent=true
            observer.interrupt(transport).catch(error=>{
              if(!lifetime.signal.aborted) transition(state,{method:'turn/interrupt',recovery:error.message})
            })
          }
        }
        cancelSignal?.addEventListener('abort',requestCancel)
        try {
          if(cancelPending) requestCancel()
          for await(const event of transport.events) {
            if(lifetime.signal.aborted) return
            const {method,params={}}=event
            if(method==='process/exit') {
              processState='EXITED'
              transition('DISCONNECTED',{method,exitCode:params.exitCode??null})
              throw new Error('App Server process exited')
            }
            if(params.threadId!==threadId || (params.turnId??params.turn?.id)!==turnId) continue
            observer.event(event)
            if(terminal()) return transition(terminal(),{method})
            if(method==='error') {
              if(params.willRetry===true || (params.willRetry==null && /^Reconnecting\.\.\./i.test(params.error?.message??''))) {
                transition('RECONNECTING',{method,rawError:params.error??null,willRetry:params.willRetry??null})
              } else {
                // Only a terminal turn status can establish failure.
                transition('UNKNOWN',{method,rawError:params.error??null,willRetry:params.willRetry??null,recovery:'awaiting_terminal_status'})
              }
            }
          }
          throw new Error('Notification stream ended')
        } finally {
          cancelSignal?.removeEventListener('abort',requestCancel)
        }
      } catch(error) {
        if(lifetime.signal.aborted) return
        connectionState='DISCONNECTED'
        transition('DISCONNECTED',{recovery:error.message})
        if(attempts++>=maxReconnects) return transition('UNKNOWN',{recovery:'reconnect_exhausted'})
      } finally {
        transport?.detach?.()
      }
    }
  }
  try {
    await withDeadline(run(),timeoutMs)
  } catch(error) {
    transition('UNKNOWN',{recovery:error.message})
  } finally {
    lifetime.abort()
    cancelSignal?.removeEventListener('abort',cancel)
  }
  return {state, timeline, lastKnownTurnStatus:observer.result().status}
}
