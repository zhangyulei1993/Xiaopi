import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const lockFile = path.join(os.tmpdir(), 'xiaopi-business-suite.flock')
const helper = fileURLToPath(new URL('./suite-lock.py', import.meta.url))

export function enterSuiteLock(executionId, file = lockFile) {
  if (!process.env.XIAOPI_SUITE_LOCK) {
    const child = spawnSync('python3', [helper, file, executionId, process.execPath, ...process.argv.slice(1)], {
      stdio: 'inherit',
    })
    if (child.error) console.error(child.error.message)
    process.exit(child.status ?? 1)
  }

  const {fd, ...owner} = JSON.parse(process.env.XIAOPI_SUITE_LOCK)
  assert.ok(Number.isInteger(fd) && fd >= 3, 'Invalid suite lock descriptor')
  const descriptor = fs.fstatSync(fd)
  const disk = fs.statSync(file)
  assert.ok(descriptor.isFile() && descriptor.ino === disk.ino && descriptor.dev === disk.dev,
    'Suite lock descriptor does not match the lock file')
  assert.deepEqual(owner, JSON.parse(fs.readFileSync(file,'utf8')), 'Invalid suite lock ownership')
  if (owner.pid !== process.pid) {
    const verification = spawnSync('python3', [helper,file,'--verify'], {
      stdio:['ignore','ignore','pipe',fd], encoding:'utf8',
    })
    assert.equal(verification.status,0,verification.stderr || verification.error?.message || 'Inherited suite lock is not held')
  }
  return { fd, executionId: owner.executionId }
}

export function suiteChildOptions(lock, env = process.env) {
  return {
    env: {...env, XIAOPI_SUITE_LOCK: JSON.stringify({...JSON.parse(process.env.XIAOPI_SUITE_LOCK),fd:3})},
    stdio: ['pipe','pipe','pipe',lock.fd],
  }
}
