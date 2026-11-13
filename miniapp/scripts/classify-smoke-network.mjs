import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export function classify({ url, method, status }) {
  if (method === 'POST' && url === 'https://report-online.sh.wxgateway.com/SdkReport') return 'PLATFORM_TELEMETRY'
  try {
    const target = new URL(url)
    if (method === 'GET' && target.origin === 'http://127.0.0.1:18083'
      && !target.username && !target.password && target.pathname.startsWith('/api/')
      && (status === undefined || status >= 200 && status < 300)) return 'APPLICATION_READ'
  } catch {
    // Malformed or missing destinations fail closed.
  }
  return 'FAIL'
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const directory = path.resolve(process.argv[2] || 'tests/reports/read-only-smoke')
  const resultPath = path.join(directory, 'result.json')
  const result = JSON.parse(fs.readFileSync(resultPath, 'utf8'))
  const previous = result.networkClassification?.originalOverallStatus ?? result.status
  for (const page of result.pages) {
    const capture = JSON.parse(fs.readFileSync(path.join(directory, page.name + '-network.json'), 'utf8'))
    if (!capture.ok || typeof capture.result !== 'string') throw new Error('Invalid network capture for ' + page.name)
    page.network = capture.result.split('\n').filter(Boolean).map(line => {
      const { type, detail } = JSON.parse(line.replace(/^\d+:/, ''))
      const { method, url, status } = detail
      return { type, method, url, status, classification: classify(detail) }
    })
    page.networkStatus = page.network.some(entry => entry.type === 'HTTP_RESPONSE' && entry.classification === 'APPLICATION_READ')
      && page.network.every(entry => entry.classification !== 'FAIL') ? 'PASS' : 'FAIL'
    page.status = page.uiStatus === 'PASS' && page.networkStatus === 'PASS' && !page.consoleErrors ? 'PASS' : 'FAIL'
  }
  result.status = result.pages.every(page => page.status === 'PASS') ? 'PASS' : 'FAIL'
  result.networkClassification = {
    status: result.pages.every(page => page.networkStatus === 'PASS') ? 'PASS' : 'FAIL',
    originalOverallStatus: previous,
    scope: 'Offline replay of original Network captures. No page actions replayed.',
    attribution: 'Platform attribution inferred from the endpoint, SDK reporting headers and the hostname found in DevTools WeappPlugin/inspector/sdk/sdk-features.js. The raw export exposes no initiator or call stack. Source and test bundle contain no endpoint match.',
    sdkDisableConfiguration: 'No official switch verified. The official project configuration reference could not be loaded during this review.',
    allNetworkReadOnly: result.networkReadOnlyStatus,
  }
  fs.writeFileSync(resultPath, JSON.stringify(result, null, 2) + '\n')
  const heading = '\n## Network 分类复验\n'
  const reportPath = path.join(directory, 'report.md')
  const original = fs.readFileSync(reportPath, 'utf8').split(heading)[0]
  const summary = result.pages.map(page => {
    const responses = page.network.filter(entry => entry.type === 'HTTP_RESPONSE')
    return '- ' + page.name + ' ' + page.networkStatus + '。' + responses.map(entry =>
      [entry.classification, entry.method, entry.url, entry.status].join(' ')).join('；')
  })
  fs.writeFileSync(reportPath, original + heading + '\n'
    + '只重放已有 Network 原始记录，未重复页面测试。应用 Network 分类 ' + result.networkClassification.status + '，原全网只读断言 ' + result.networkReadOnlyStatus + '。\n\n'
    + summary.join('\n') + '\n\n'
    + '仅精确匹配 HTTPS POST 到 report-online.sh.wxgateway.com/SdkReport 为 PLATFORM_TELEMETRY。外部 SDK 上报仍发生，不计入业务写入。\n\n'
    + result.networkClassification.attribution + '\n\n'
    + result.networkClassification.sdkDisableConfiguration + '\n')
  console.log(JSON.stringify({ status: result.networkClassification.status, allNetworkReadOnly: result.networkReadOnlyStatus }))
  if (result.networkClassification.status !== 'PASS') process.exitCode = 1
}
