import {spawn} from 'node:child_process'
import {createConnection,createServer} from 'node:net'
import {createInterface} from 'node:readline'
import {once} from 'node:events'
import {pathToFileURL} from 'node:url'
import {persistObservedUsage} from "./app-server-ledger.mjs"
import {followTurn,observeTurn} from './app-server-recovery.mjs'

const allowed = new Set(['initialize', 'thread/resume', 'thread/turns/list', 'turn/interrupt'])

export async function connectUnix(path, signal, {direct=false}={}) {
  let socket, output, bridge, peer, proxy, ws
  if (direct) {
    socket = output = createConnection({path, signal})
  } else {
    proxy = spawn("codex", ["app-server", "proxy", "--sock", path], {stdio:["pipe","pipe","inherit"]})
    bridge = createServer(connection => {
      if (peer) return connection.destroy()
      peer = connection
      peer.pipe(proxy.stdin)
      proxy.stdout.pipe(peer)
    })
  }
  const pending = new Map()
  let failure, sequence = 0
  function receive(raw) {
    try {
      const message = JSON.parse(raw)
      if (message.method) {
        if (message.id !== undefined) {
          send({id: message.id, error: {code: -32601, message: 'Observer cannot handle server requests'}})
        } else {
          events.push(message)
          wake?.()
        }
      } else {
        const call = pending.get(message.id)
        if (!call) return
        pending.delete(message.id)
        if (message.error) call.reject(new Error(JSON.stringify(message.error)))
        else call.resolve(message.result)
      }
    } catch (error) { finish(error) }
  }
  const events = []
  let wake, ended = false
  function finish(error) {
    failure ??= error
    ended = true
    for (const call of pending.values()) call.reject(failure)
    pending.clear()
    wake?.()
  }
  function send(message) {
    if (ended) throw failure
    const payload = JSON.stringify({jsonrpc: '2.0', ...message})
    if (ws) ws.send(payload)
    else output.write(payload + '\n')
  }
  function detach() {
    signal?.removeEventListener('abort',detach)
    finish(new Error('Observer detached'))
    if (ws?.readyState === WebSocket.OPEN) ws.close(1000)
    socket?.destroy()
    bridge?.close()
    proxy?.stdin.end()
    if (proxy && proxy.exitCode===null && proxy.signalCode===null) {
      const timer=setTimeout(()=>{peer?.destroy();proxy.kill()},500)
      timer.unref()
      proxy.once('exit',()=>{clearTimeout(timer);peer?.destroy()})
    } else peer?.destroy()
  }
  signal?.addEventListener('abort',detach,{once:true})
  try {
    if (direct) {
      socket.on('error',finish)
      socket.once('close',()=>finish(new Error('Unix socket closed')))
      createInterface({input:socket}).on('line',receive)
      await once(socket,'connect',{signal})
    } else {
      proxy.on('error',finish)
      proxy.stdin.on('error',finish)
      proxy.once('exit',(code,exitSignal)=>{
        finish(new Error('Proxy exit code: '+code+' signal: '+exitSignal))
        peer?.destroy()
      })
      bridge.listen(0,'127.0.0.1')
      await once(bridge,'listening',{signal})
      if (ended) throw failure
      ws=new WebSocket('ws://127.0.0.1:'+bridge.address().port+'/')
      ws.addEventListener('message',event=>receive(event.data))
      ws.addEventListener('error',event=>finish(event.error??new Error(event.message||'WebSocket error')))
      ws.addEventListener('close',event=>finish(new Error('WebSocket closed: '+event.code+' '+event.reason)))
      await Promise.race([
        once(ws,'open',{signal}),
        once(ws,'close',{signal}).then(()=>{throw failure}),
      ])
    }
  } catch(error) {
    detach()
    throw error
  }
  return {
    request(method, params) {
      if (!allowed.has(method)) throw new Error('Unsupported observer request: ' + method)
      return new Promise((resolve, reject) => {
        const id = ++sequence
        send({id, method, params})
        pending.set(id, {resolve, reject})
      })
    },
    notify(method) {
      if (method !== 'initialized') throw new Error('Unsupported observer notification')
      send({method})
    },
    events: (async function* () {
      while (events.length || !ended) {
        if (events.length) yield events.shift()
        else await new Promise(resolve => { wake = resolve })
      }
      throw failure
    })(),
    detach,
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [path, threadId, turnId] = process.argv.slice(2)
  if (!path || !threadId || !turnId) {
    console.error('Usage: node app-server-follow.mjs SOCKET THREAD_ID TURN_ID')
    process.exitCode = 2
  } else {
    const cancel = new AbortController()
    process.once('SIGINT', () => cancel.abort())
    const observer = observeTurn(threadId,turnId)
    const result = await followTurn({threadId, turnId, observer, cancelSignal: cancel.signal,
      connect: signal => connectUnix(path, signal, {direct:process.env.APP_SERVER_MOCK_DIRECT === "1"}),
      onTransition: entry => console.log(JSON.stringify(entry)),
    })
    const accounting = await persistObservedUsage(process.env.APP_SERVER_USAGE_LEDGER,observer.takeUsage())
    console.log(JSON.stringify({
      usageRecorded: accounting.status === 'RECORDED',
      accounting,
      state: result.state,
      lastKnownTurnStatus: result.lastKnownTurnStatus,
    }))
    process.exitCode = accounting.status === 'FAILED' ? 2 : result.state === 'COMPLETED' ? 0 : 1
  }
}
