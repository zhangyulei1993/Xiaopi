import test from 'node:test'
import assert from 'node:assert/strict'
import {createServer} from 'node:net'
import {createInterface} from 'node:readline'
import {mkdtemp, rm} from 'node:fs/promises'
import {once} from 'node:events'
import {execFile} from 'node:child_process'
import {promisify} from 'node:util'
import {fileURLToPath} from 'node:url'

test('CLI reconnects through the same independent server and never submits a turn', async t => {
  const dir = await mkdtemp('/tmp/app-rpc-')
  const path = dir + '/socket'
  const methods = []
  let connections = 0
  const server = createServer(socket => {
    const connection = ++connections
    const send = message => socket.write(JSON.stringify({jsonrpc:'2.0', ...message}) + '\n')
    createInterface({input:socket}).on('line', line => {
      const {method,id,params} = JSON.parse(line)
      methods.push(method)
      if (method.startsWith('thread/')) assert.equal(params.threadId,'original-thread')
      if (method === 'initialized') return
      if (method === 'initialize') return send({id,result:{}})
      if (method === 'thread/resume') return send({id,result:{thread:{id:'original-thread'}}})
      assert.equal(method,'thread/turns/list')
      send({id,result:{data:[{id:'original-turn',status:'inProgress'}]}})
      if (connection === 1) return socket.end()
      const event = {method:'turn/completed',params:{threadId:'original-thread',turn:{id:'original-turn',status:'completed'}}}
      send(event)
      send(event)
    })
  })
  server.listen(path)
  await once(server,'listening')
  t.after(async () => {
    await new Promise(resolve => server.close(resolve))
    await rm(dir,{recursive:true,force:true})
  })
  const cli = fileURLToPath(new URL('../app-server-follow.mjs',import.meta.url))
  const run = () => promisify(execFile)(process.execPath,[cli,path,'original-thread','original-turn'],{timeout:5000,env:{...process.env,APP_SERVER_MOCK_DIRECT:"1"}})
  const {stdout} = await run()
  const records = stdout.trim().split('\n').map(JSON.parse)
  assert.ok(records.some(record => record.state === 'DISCONNECTED'))
  assert.equal(records.filter(record => record.state === 'COMPLETED' && record.timestamp).length,1)
  assert.equal(records.at(-1).state,'COMPLETED')
  assert.ok(server.listening)
  await run()
  assert.equal(connections,3)
  assert.equal(methods.filter(method => method === 'thread/turns/list').length,3)
})
