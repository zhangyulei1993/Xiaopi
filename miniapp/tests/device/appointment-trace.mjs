import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

const source = path.resolve(process.argv[2] || '')
assert.match(path.basename(source), /^mp-weixin-test-device-[a-zA-Z0-9]+$/)
assert.equal(path.dirname(source),path.resolve('dist'))
const destination = fs.mkdtempSync(path.resolve('dist/mp-weixin-test-device-trace-'))
fs.cpSync(source,destination,{recursive:true})
const base = path.join(destination,'pages/properties/detail')
let js = fs.readFileSync(base+'.js','utf8')
let wxml = fs.readFileSync(base+'.wxml','utf8')
function replace(text, from, to) {
  assert.equal(text.split(from).length,2,from)
  return text.replace(from,to)
}
const trace = `let apSeq=0,apHeight=0;
const apLog=(kind,event,reason)=>{const node=n=>n?{id:n.id||"",marker:n.dataset?.apTrace||""}:null;
const point=event?.changedTouches?.[0]||event?.touches?.[0];
console.info("[APPOINTMENT_TRACE]",JSON.stringify({seq:++apSeq,at:Date.now(),kind,target:node(event?.target),currentTarget:node(event?.currentTarget),point:point?{x:point.clientX??point.pageX,y:point.clientY??point.pageY}:null,open:T.value,height:apHeight,reason:reason||null,stack:reason?new Error().stack:null}));};
const apSet=(value,reason,event)=>{apLog(value?"open-call":"close-call",event,reason);return T.value=value;};
e.watch(T,(value,previous)=>apLog("state",null,previous+" -> "+value),{flush:"sync"});
const apKeyboard=event=>{apHeight=event.height;apLog("keyboard-height");};
e.onShow(()=>{if(globalThis.__apKeyboard)wx.offKeyboardHeightChange?.(globalThis.__apKeyboard);globalThis.__apKeyboard=apKeyboard;wx.onKeyboardHeightChange?.(apKeyboard);apLog("page-show");});`
js = replace(js,';e.onLoad(l=>',';'+trace+'e.onLoad(l=>')
js = replace(js,'T.value=!1,v.peekPendingAction()','apSet(false,"syncAudience:admin"),v.peekPendingAction()')
js = replace(js,'T.value=!0}},Ue','apSet(true,"openAppointment")}},Ue')
js = replace(js,'T.value=!1,e.index.showModal','apSet(false,"submitAppointment:success"),e.index.showModal')
js = replace(js,'bl:e.o(e=>T.value=!1,"13")','bl:e.o(event=>apSet(false,"close-button",event),"13")')
js = replace(js,'bw:e.o(e=>T.value=!1,"01")','bw:e.o(event=>{apLog("mask-tap",event);apSet(false,"mask-tap",event)},"01"),apSheet:e.o(event=>apLog("sheet-tap",event),"ap-sheet")')
const events = ['touchstart','touchend','tap','focus','blur']
js = replace(js,'bn:P.value,bo:', 'bn:P.value,'+events.map((event,index)=>'ap'+index+':e.o(event=>apLog("input-'+event+'",event),"ap-'+index+'")').join(',')+',bo:')
wxml = replace(wxml,'bindtap="{{bw}}"><view class="appointment-sheet data-v-743750b8">','id="ap-mask" data-ap-trace="mask" bindtap="{{bw}}"><view id="ap-sheet" data-ap-trace="sheet" class="appointment-sheet data-v-743750b8" bindtap="{{apSheet}}">')
wxml = replace(wxml,'value="{{bn}}" bindinput="{{bo}}"','id="ap-input" data-ap-trace="input" '+events.map((event,index)=>'bind'+event+'="{{ap'+index+'}}"').join(' ')+' value="{{bn}}" bindinput="{{bo}}"')
fs.writeFileSync(base+'.js',js)
fs.writeFileSync(base+'.wxml',wxml)
console.log(destination)
