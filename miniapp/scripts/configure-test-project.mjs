import fs from 'node:fs'
import path from 'node:path'

const output = path.resolve(process.argv[2] || '')
const configPath = path.join(output, 'project.private.config.json')
const sharedConfigPath = path.join(output, 'project.config.json')
if (!fs.existsSync(path.join(output, 'app.json')) || !fs.existsSync(configPath) || !fs.existsSync(sharedConfigPath)) {
  throw new Error(`Expected built test project with private config: ${output}`)
}
for (const target of [configPath, sharedConfigPath]) {
  const config = JSON.parse(fs.readFileSync(target, 'utf8'))
  config.setting ||= {}
  config.setting.urlCheck = false
  fs.writeFileSync(target, `${JSON.stringify(config, null, 2)}\n`)
}
console.log(JSON.stringify({ output, testOnly: true, urlCheck: false }))
