#!/usr/bin/env node
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..')
const envFile = path.join(root, '.env.test')
if (!fs.existsSync(envFile)) throw new Error('Missing tests/.env.test; initialize isolated test env first')
const compose = ['compose', '--project-name', 'xiaopi-miniapp-test', '--file', path.join(root, 'docker-compose.test.yml'), '--env-file', envFile]
const run = (args, input) => {
  const result = spawnSync('docker', [...compose, ...args], { encoding: 'utf8', input })
  if (result.status !== 0) throw new Error(result.stderr || result.stdout || `docker ${args[0]} failed`)
  return result.stdout.trim()
}
const config = JSON.parse(run(['config', '--format', 'json']))
if (config.services?.['api-test']?.environment?.MYSQL_HOST !== 'mysql-test' || config.services?.['api-test']?.environment?.MYSQL_DATABASE !== 'xiaopi_miniapp_test') throw new Error('Refusing business writes: test API database target is not the isolated test DB')
const base = 'http://127.0.0.1:18083/api'
const getJSON = async (url, token) => {
  const response = await fetch(`${base}${url}`, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
  return { status: response.status, body: await response.json() }
}
const postJSON = async (method, url, token, body) => {
  const response = await fetch(`${base}${url}`, { method, headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
  return { status: response.status, body: await response.json() }
}
const login = async code => {
  const result = await postJSON('POST', '/v1/auth/wechat/login', '', { code })
  if (result.status !== 200 || result.body.data?.testOnly !== true) throw new Error(`Isolated synthetic login failed: ${code} (HTTP ${result.status})`)
  return result.body.data.accessToken
}
const db = sql => run(['exec', '-T', 'mysql-test', 'sh', '-ec', 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -N "$MYSQL_DATABASE"'], sql)

const dbTarget = db('SELECT DATABASE();')
if (dbTarget !== 'xiaopi_miniapp_test') throw new Error(`Refusing fixture reset: connected database is ${dbTarget}`)
db("DELETE FROM rental_requests WHERE user_id='qa-user-a' AND property_id='qa-property-1' AND request_type='appointment';")

const tokenA = await login('qa-user-a')
const tokenB = await login('qa-user-b')
const id = 'qa-property-1'
const results = []
let favorite = await postJSON('PUT', `/v1/favorites/${id}`, tokenA)
if (favorite.status !== 200) throw new Error(`Favorite add failed (${favorite.status})`)
favorite = await postJSON('PUT', `/v1/favorites/${id}`, tokenA)
if (favorite.status !== 200) throw new Error('Duplicate favorite should remain successful')
let listA = await getJSON('/v1/favorites', tokenA)
let listB = await getJSON('/v1/favorites', tokenB)
if (!listA.body.data?.includes(id) || listB.body.data?.includes(id)) throw new Error('Favorite ownership isolation failed')
results.push({ name: 'favorite-add-idempotent-and-user-isolation', status: 'passed', evidence: 'User A has fixture favorite; user B does not.' })
favorite = await postJSON('DELETE', `/v1/favorites/${id}`, tokenA)
if (favorite.status !== 200 || favorite.body.data?.favorite !== false) throw new Error('Favorite removal failed')
listA = await getJSON('/v1/favorites', tokenA)
if (listA.body.data?.includes(id)) throw new Error('Favorite remained after removal')
results.push({ name: 'favorite-remove', status: 'passed' })

const payload = { requestType: 'appointment', propertyId: id, contactName: 'QA API only', preferredTime: '2030-01-05 10:00-12:00', phone: '13999999999', location: 'TEST Address', layout: '1bd 1ba', budgetMin: 1800, budgetMax: 1800, note: 'SYNTHETIC TEST DATA' }
const invalid = await postJSON('POST', '/v1/rental-requests', tokenA, { ...payload, contactName: '', preferredTime: '' })
if (invalid.status !== 400) throw new Error(`Invalid appointment should be rejected, got HTTP ${invalid.status}`)
results.push({ name: 'appointment-required-field-validation', status: 'passed', httpStatus: invalid.status })
const created = await postJSON('POST', '/v1/rental-requests', tokenA, payload)
if (created.status !== 201 || created.body.data?.userId !== 'qa-user-a' || created.body.data?.propertyId !== id) throw new Error(`Synthetic appointment create failed or association mismatch (HTTP ${created.status})`)
const duplicate = await postJSON('POST', '/v1/rental-requests', tokenA, payload)
if (duplicate.status !== 200 || duplicate.body.data?.id !== created.body.data.id) throw new Error('Appointment duplicate request did not return the existing fixture appointment')
const mineA = await getJSON('/v1/rental-requests/mine', tokenA)
const mineB = await getJSON('/v1/rental-requests/mine', tokenB)
if (!mineA.body.data?.some(row => row.id === created.body.data.id && row.userId === 'qa-user-a' && row.propertyId === id) || mineB.body.data?.some(row => row.id === created.body.data.id)) throw new Error('Appointment readback or cross-user isolation failed')
const sql = `SELECT CONCAT(DATABASE(),'|',COUNT(*),'|',COALESCE(MAX(CONCAT(user_id,':',property_id)),'')) FROM rental_requests WHERE id='${created.body.data.id}' AND user_id='qa-user-a' AND property_id='qa-property-1';`
const dbEvidence = db(sql)
if (!dbEvidence.startsWith('xiaopi_miniapp_test|1|qa-user-a:qa-property-1')) throw new Error(`Test DB association verification failed: ${dbEvidence}`)
results.push({ name: 'appointment-create-deduplicate-readback-and-db-association', status: 'passed', apiId: created.body.data.id, dbEvidence })

const report = { environment: 'isolated-local-test', api: base, database: 'xiaopi_miniapp_test', completedAt: new Date().toISOString(), status: 'passed', results }
const reportDir = path.join(root, 'reports')
fs.mkdirSync(reportDir, { recursive: true })
fs.writeFileSync(path.join(reportDir, 'business-api-latest.json'), `${JSON.stringify(report, null, 2)}\n`)
console.log(JSON.stringify(report, null, 2))
