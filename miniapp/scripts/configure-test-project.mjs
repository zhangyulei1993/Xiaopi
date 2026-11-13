import fs from 'node:fs'
import path from 'node:path'

const output = path.resolve(process.argv[2] || '')
const configPath = path.join(output, 'project.private.config.json')
if (!fs.existsSync(path.join(output, 'app.json')) || !fs.existsSync(configPath)) {
  throw new Error(`Expected built test project with private config: ${output}`)
}
const config = JSON.parse(fs.readFileSync(configPath, 'utf8'))
config.setting ||= {}
config.setting.urlCheck = false
fs.writeFileSync(configPath, `${JSON.stringify(config, null, 2)}\n`)
console.log(JSON.stringify({ output, testOnly: true, urlCheck: false }))
