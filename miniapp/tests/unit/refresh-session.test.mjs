import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

const source = readFileSync(new URL('../../src/auth/refresh-session.ts', import.meta.url), 'utf8')
const code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText

test('refresh shares requests within a session and ignores late results from an old session', async () => {
  let current = { accessToken: 'old', role: 'admin' }
  const requests = []
  const writes = []
  const modules = {
    '../api/properties': { refreshAuth: () => new Promise(resolve => requests.push(resolve)) },
    './session': {
      getSession: () => current,
      saveSession: (accessToken, role) => { writes.push(accessToken); current = { accessToken, role } },
    },
  }
  const exports = {}
  vm.runInNewContext(code, { exports, require: path => modules[path] })
  const old = exports.refreshCurrentSession('admin')
  const oldConcurrent = exports.refreshCurrentSession('admin')
  const oldRejected = assert.rejects(old, /登录账号已变化/)
  const concurrentRejected = assert.rejects(oldConcurrent, /登录账号已变化/)
  assert.equal(requests.length, 1)
  current = { accessToken: 'new', role: 'user' }
  const next = exports.refreshCurrentSession('user')
  assert.equal(requests.length, 2)
  requests[1]({ accessToken: 'new-refreshed', role: 'user' })
  await next
  requests[0]({ accessToken: 'old-refreshed', role: 'admin' })
  await Promise.all([oldRejected, concurrentRejected])
  assert.equal(current.accessToken, 'new-refreshed')
  assert.deepEqual(writes, ['new-refreshed'])
})
