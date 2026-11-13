import fs from 'node:fs'
import path from 'node:path'

const [outputArg, expectedApi, environment] = process.argv.slice(2)
if (!outputArg || !expectedApi || !['development', 'test', 'production'].includes(environment)) {
  throw new Error('Usage: verify-build-api.mjs <output-dir> <expected-api> <development|test|production>')
}
const output = path.resolve(process.cwd(), outputArg)
if (!fs.existsSync(path.join(output, 'app.json'))) throw new Error(`Not a built Mini Program output: ${output}`)
const privateConfigPath = path.join(output, 'project.private.config.json')
if (fs.existsSync(privateConfigPath)) {
  const privateConfig = JSON.parse(fs.readFileSync(privateConfigPath, 'utf8'))
  const urlCheck = privateConfig.setting?.urlCheck
  if (environment === 'test' && urlCheck !== false) throw new Error('Test build must disable URL checks for the loopback API')
  if (environment !== 'test' && urlCheck !== true) throw new Error(`${environment} build must preserve URL checks`)
}
const sources = []
function visit(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name)
    if (entry.isDirectory()) visit(file)
    else if (entry.isFile() && file.endsWith('.js')) sources.push(fs.readFileSync(file, 'utf8'))
  }
}
visit(output)
const bundle = sources.join('\n')
if (!bundle.includes(expectedApi)) throw new Error(`Expected API base is absent from ${output}: ${expectedApi}`)
const productionApi = 'https://api.xiaopizufang.cn/api'
if (environment !== 'production' && bundle.includes(productionApi)) throw new Error(`Production API leaked into ${environment} build`)
const testOnlyCode = ['qa-user-a', 'qa-user-b', 'qa-disabled']
if (environment === 'test' && !testOnlyCode.every(code => bundle.includes(code))) throw new Error('Test login accounts are missing from the test build')
if (environment !== 'test' && testOnlyCode.some(code => bundle.includes(code))) throw new Error(`Test login account leaked into ${environment} build`)
if (environment === 'production' && (expectedApi !== productionApi || bundle.includes('127.0.0.1:18080') || bundle.includes('127.0.0.1:18083'))) {
  throw new Error('Production build contains a non-production API configuration')
}
console.log(JSON.stringify({ output, environment, apiBase: expectedApi, javascriptFiles: sources.length, verified: true }))
