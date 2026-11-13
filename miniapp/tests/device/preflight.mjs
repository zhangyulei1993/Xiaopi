import assert from 'node:assert/strict'
import fs from 'node:fs'
import http from 'node:http'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'

const root = fileURLToPath(new URL('../../', import.meta.url))
const [mode, bind, phone] = process.argv.slice(2)
const local = 'http://127.0.0.1:18083'
const port = 18084
const privateV4 = ip => /^(?:10\.|192\.168\.|172\.(?:1[6-9]|2\d|3[01])\.)/.test(ip || '') &&
  ip.split('.').length === 4 && ip.split('.').every(part => /^\d{1,3}$/.test(part) && Number(part) <= 255)
assert.ok(privateV4(bind) && Object.values(os.networkInterfaces()).flat().some(i=>i?.address===bind && !i.internal), 'Specify a local private interface IPv4')
const origin = 'http://' + bind + ':' + port

if (mode === 'build') {
  execFileSync(process.execPath,['scripts/scan-build-security.mjs','dist/mp-weixin-test'],{cwd:root,stdio:'inherit'})
  const source = path.join(root,'dist/mp-weixin-test')
  const output = fs.mkdtempSync(path.join(root,'dist/mp-weixin-test-device-'))
  fs.cpSync(source,output,{recursive:true})
  let changed = 0
  function adapt(directory) {
    for (const entry of fs.readdirSync(directory,{withFileTypes:true})) {
      const file = path.join(directory,entry.name)
      if (entry.isDirectory()) adapt(file)
      else if (entry.isFile() && /\.(js|json|wxml|wxss)$/.test(file)) {
        const before = fs.readFileSync(file,'utf8')
        const after = before.replaceAll(local,origin)
        if (after !== before) {
          fs.writeFileSync(file,after)
          changed++
        }
      }
    }
  }
  adapt(output)
  assert.ok(changed > 0,'No test origin substitutions')
  console.log(JSON.stringify({output,origin,changed,derivedFrom:source}))
} else {
  assert.equal(mode,'serve')
  assert.ok(privateV4(phone) && bind !== phone, 'Specify the confirmed private IPv4 of the test iPhone')
  const owner = execFileSync('docker',['inspect','-f','{{index .Config.Labels "com.docker.compose.project"}}','xiaopi-miniapp-test-api-test-1'],{encoding:'utf8'}).trim()
  assert.equal(owner,'xiaopi-miniapp-test')
  assert.equal(execFileSync('docker',['port','xiaopi-miniapp-test-api-test-1','8080/tcp'],{encoding:'utf8'}).trim(),'127.0.0.1:18083')
  execFileSync('docker',['exec','xiaopi-miniapp-test-api-test-1','sh','-ec','[ "$APP_ENV" = test ] && [ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && [ "$MYSQL_HOST" = mysql-test ]'])
  const reads = new Set(['/api/health','/api/v1/config/public','/api/v1/properties','/api/v1/auth/profile','/api/v1/favorites'])
  const login = '/api/v1/auth/wechat/login'
  const server = http.createServer({requestTimeout:8000,headersTimeout:5000},async (req,res)=>{
    const reject = status => { res.writeHead(status,{connection:'close'}); res.end() }
    if (req.socket.remoteAddress !== phone || req.headers.host !== bind+':'+port) return reject(403)
    const isLogin = req.method==='POST' && req.url===login
    if (!(req.method==='GET' && reads.has(req.url)) && !isLogin) return reject(405)
    try {
      const headers = {}
      if (req.headers.authorization) headers.authorization=req.headers.authorization
      let body
      if (isLogin) {
        let bytes=0
        const parts=[]
        for await (const part of req) {
          bytes+=part.length
          if (bytes>512) return reject(413)
          parts.push(part)
        }
        const data=JSON.parse(Buffer.concat(parts))
        if (data?.code!=='qa-user-a' || Object.keys(data).length!==1) return reject(403)
        headers['content-type']='application/json'
        body=JSON.stringify({code:'qa-user-a'})
      }
      const response=await fetch(local+req.url,{method:req.method,headers,body,redirect:'error',signal:AbortSignal.timeout(5000)})
      console.log(JSON.stringify({at:new Date().toISOString(),peer:phone,method:req.method,path:req.url,status:response.status}))
      res.writeHead(response.status,{'content-type':response.headers.get('content-type')||'application/json','cache-control':'no-store'})
      res.end(Buffer.from(await response.arrayBuffer()))
    } catch {
      if (!res.headersSent) reject(502)
      else res.destroy()
    }
  })
  server.on('connect',(_request,socket)=>socket.destroy())
  server.on('upgrade',(_request,socket)=>socket.destroy())
  server.listen(port,bind,()=>console.log(JSON.stringify({origin,phone,expiresInMinutes:20})))
  setTimeout(()=>{server.close();server.closeAllConnections()},20*60*1000).unref()
}
