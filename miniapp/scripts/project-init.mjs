#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'
const root = path.resolve(process.argv[2] || '.')
if (!fs.existsSync(path.join(root, 'project.config.json')) && !fs.existsSync(path.join(root, 'project.private.config.json'))) {
  console.error(`Not a WeChat Mini Program project: ${root}`)
  process.exit(2)
}
const created = []
for (const rel of ['tests/e2e', 'tests/fixtures', 'tests/reports', 'scripts']) {
  const target = path.join(root, rel)
  if (!fs.existsSync(target)) { fs.mkdirSync(target, { recursive: true }); created.push(rel) }
}
console.log(JSON.stringify({ project: root, created, preservedExisting: true }, null, 2))
