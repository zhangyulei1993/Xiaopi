#!/usr/bin/env node
import { enterSuiteLock, suiteChildOptions } from '../suite-lock.mjs'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'

const project = path.resolve(process.argv[2] || path.join(import.meta.dirname, '../../dist/mp-weixin'))
const outputDir = path.resolve(process.argv[3] || path.join(import.meta.dirname, '../reports/automator-diagnostics'))
const mode = process.argv[4] || 'full'
fs.mkdirSync(outputDir, { recursive: true })
const cli = process.env.WECHATIDE_CLI || 'wechatide'
const client = process.env.WECHATIDE_CLIENT || 'Codex'
const cases = []
const suiteLock = enterSuiteLock(new Date().toISOString())
const delay = ms => new Promise(resolve => setTimeout(resolve, ms))
function invoke(tool, args, timeout = 15000) {
  const start = performance.now()
  const child = spawnSync(cli, ['-c', client, tool, '--project', project, ...args], { ...suiteChildOptions(suiteLock), encoding: 'utf8', timeout, maxBuffer: 2 * 1024 * 1024 })
  const elapsedMs = Math.round(performance.now() - start)
  let data
  try { data = JSON.parse((child.stdout || '').slice((child.stdout || '').indexOf('{'))) } catch {}
  const row = { tool, args, elapsedMs, exitCode: child.status, ok: data?.ok === true, rawError: data?.message || child.error?.message || null, stderr: child.stderr || '', result: data?.result?.result ?? data?.result ?? null, stdout: child.stdout || '' }
  cases.push(row)
  return row
}
function report() {
  const result = { project, autoPort: 19333, generatedAt: new Date().toISOString(), sequential: true, summary: Object.fromEntries([...new Set(cases.map(x => x.tool))].map(tool => { const a = cases.filter(x => x.tool === tool); return [tool, { attempts: a.length, successes: a.filter(x => x.ok).length, failures: a.filter(x => !x.ok).length, meanMs: Math.round(a.reduce((n, x) => n + x.elapsedMs, 0) / a.length), maxMs: Math.max(...a.map(x => x.elapsedMs)) }] })), cases: cases.map(({ stdout, ...x }) => x) }
  fs.writeFileSync(path.join(outputDir, 'automator-stability.json'), JSON.stringify(result, null, 2))
  fs.writeFileSync(path.join(outputDir, 'automator-stability.raw.jsonl'), cases.map(x => JSON.stringify(x)).join('\n') + '\n')
  console.log(JSON.stringify(result.summary, null, 2))
}
try {
  if (mode === 'click-only') {
    for (const waitMs of [0, 250, 1000, 3000, 250, 1000, 0, 3000, 250, 1000]) {
      const opened = invoke('simulator_open_page', ['--page', 'pages/home/index'])
      await delay(waitMs)
      const tap = invoke('automation_element_action', ['--action', 'tap', '--selector', '.hero-search', '--wait-for-selector', '.hero-search'])
      const current = invoke('automation_runtime_info', ['--action', 'currentPage'])
      tap.expectedRoute = 'pages/properties/index'
      tap.actualRoute = current.result?.currentPage?.path || null
      tap.routeMatches = tap.actualRoute === tap.expectedRoute
      tap.navigationStatus = opened.ok ? 'opened' : 'open-failed'
    }
  } else if (mode === 'selector-audit') {
    invoke('simulator_open_page', ['--page', 'pages/home/index'])
    await delay(1000)
    for (const [tool, args] of [
      ['automation_page_action', ['--action', 'querySelector', '--selector', '.hero-search']],
      ['automation_page_action', ['--action', 'querySelector', '--selector', '[class=hero-search]']],
      ['automation_page_action', ['--action', 'querySelector', '--selector', '[aria-label="search"]']],
      ['automation_page_action', ['--action', 'querySelector', '--selector', '#5']],
      ['automation_page_action', ['--action', 'querySelector', '--selector', 'view:nth-child(1)']],
      ['automation_element_action', ['--action', 'size', '--selector', '.hero-search']],
      ['automation_element_action', ['--action', 'offset', '--selector', '.hero-search']],
      ['automation_element_action', ['--action', 'text', '--selector', '.hero-search']]
    ]) invoke(tool, args)
  } else {
  for (let i = 0; i < 10; i++) invoke('automation_runtime_info', ['--action', 'currentPage'])
  await delay(250)
  for (let i = 0; i < 10; i++) invoke('automation_page_action', ['--action', 'querySelectorAll', '--selector', 'view'])
  await delay(1000)
  for (let i = 0; i < 10; i++) invoke('simulator_screenshot', ['--path', path.join(outputDir, `simulator-${String(i + 1).padStart(2, '0')}.png`), '--optimize=false'])
  for (const waitMs of [0, 250, 1000, 3000, 250, 1000, 0, 3000, 250, 1000]) {
    await delay(waitMs)
    invoke('simulator_open_page', ['--page', 'pages/home/index'])
    await delay(150)
    invoke('automation_element_action', ['--action', 'tap', '--selector', '.hero-search', '--wait-for-selector', '.hero-search'])
  }
  }
} finally {
  report()
}
