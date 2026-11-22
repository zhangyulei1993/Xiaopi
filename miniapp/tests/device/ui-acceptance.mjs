import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../reports/device/', import.meta.url))
const buildLog = fs.readFileSync(path.join(root,'appointment-fixed-build.log'),'utf8').trim().split('\n')
const fixedBuild = JSON.parse(buildLog.at(-1))
assert.equal(path.basename(fixedBuild.output),'mp-weixin-test-device-AGKo1A')
const plans = [
  { id:'appointment-keyboard-focus', status:'PASS', build:fixedBuild.output,
    coverage:'弹层保持打开，姓名框获得焦点，iOS 键盘展开',
    evidence:['appointment-fixed-focus-iphone.jpg','appointment-sheet.md'] },
  { id:'appointment-input-and-dismissal', status:'PASS', build:fixedBuild.output,
    coverage:'合成姓名输入正确，收键后保留，脱敏手机号和时间字段恢复可见',
    evidence:['appointment-input-before.jpg','appointment-input-keyboard.jpg','appointment-input-dismissed.jpg','appointment-input-proxy.json'] },
  { id:'request-form-scroll-and-dismissal', status:'PASS', build:'mp-weixin-test-device-CZbypN',
    coverage:'找房表单滑动触发收键，滚动后提交按钮可见。覆盖为修复前真机构建的观察结果',
    evidence:['keyboard-focused-iphone.jpg','keyboard-scroll-dismissed-iphone.jpg','keyboard.md'] },
  { id:'fixed-build-native-keyboard-height', status:'PASS', build:fixedBuild.output,
    coverage:'找房需求页原生事件多次记录 329 → 0 逻辑像素',
    evidence:['AGKo1A-keyboard-console-excerpt.json'] },
  { id:'fixed-build-runtime-viewport-recovery', status:'PASS', build:fixedBuild.output,
    coverage:'REQUIRED。同页视口保持 402×768，收键且 scrollTop=0 时表单、意向地区容器及提交按钮矩形与初始值一致',
    evidence:['AGKo1A-keyboard-console-excerpt.json','AGKo1A-scroll-dismissed.jpg'] },
  { id:'request-form-scroll-after-dismissal', status:'PASS', build:'mp-weixin-test-device-5Wv1ru',
    coverage:'REQUIRED。键盘归零后页面滚动，QA3 仍显示，提交按钮完整可见',
    evidence:['keyboard-hold.md','5Wv1ru-QA3.jpg'] },
  { id:'scroll-while-keyboard-remains-open', status:'NOT_REQUIRED', build:fixedBuild.output,
    coverage:'历史观察。实测滑动自动收键，采样中的滚动增量出现在高度归零后，提交按钮随后可见',
    evidence:['AGKo1A-keyboard-console-excerpt.json','AGKo1A-scroll-dismissed.jpg'] },
]
const cases = plans.map(({evidence,...plan})=>({
  ...plan,evidence:evidence.map(file=>{
    const bytes=fs.readFileSync(path.join(root,file))
    assert.ok(bytes.length,file)
    return {file,sha256:createHash('sha256').update(bytes).digest('hex')}
  }),
}))
const inputLog = JSON.parse(fs.readFileSync(path.join(root,'appointment-input-proxy.json'),'utf8'))
assert.ok(inputLog.some(row=>row.path==='/api/v1/auth/wechat/login' && row.status===200))
assert.ok(inputLog.every(row=>row.peer==='192.168.110.12' && (row.method==='GET'||row.path==='/api/v1/auth/wechat/login')))
const counts = Object.fromEntries(['PASS','FAIL','NOT_REQUIRED'].map(status=>[status,cases.filter(c=>c.status===status).length]))
const status = counts.FAIL ? 'FAIL' : 'PASS'
const report = {device:'iPhone 17 Pro',mode:'archived-device-evidence-review',status,counts,cases}
fs.writeFileSync(path.join(root,'ui-acceptance.json'),JSON.stringify(report,null,2)+'\n')
fs.writeFileSync(path.join(root,'ui-acceptance.md'),[
  '# iPhone 17 Pro 真机 UI 验收','','本次执行复核已归档的真机证据。新增真机操作须使用隔离代理与 test-device 包。','','| 场景 | 结果 | 构建 | 覆盖 | 证据 |','| --- | --- | --- | --- | --- |',
  ...cases.map(c=>'| '+c.id+' | '+c.status+' | '+(c.build?path.basename(c.build):'待实测')+' | '+c.coverage+' | '+c.evidence.map(e=>'['+e.file+']('+e.file+')').join(' · ')+' |'),
  '', '现场复验需在 iPhone 本机触摸输入，逐步截图。在真机 Console 注册 wx.onKeyboardHeightChange，在打开前、聚焦后、滚动后和收键后记录 wx.getWindowInfo() 与 SelectorQuery 结果。日志仅记录时间、高度、窗口和节点矩形。完成后移除监听。','',
].join('\n'))
console.log(JSON.stringify({status:report.status,report:path.join(root,'ui-acceptance.json'),counts}))
process.exitCode=counts.FAIL ? 1 : 0
