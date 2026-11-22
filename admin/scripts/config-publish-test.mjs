import assert from 'node:assert/strict'
import { build } from 'esbuild'
const bundled=await build({entryPoints:['admin/src/config/configChanges.ts'],bundle:true,platform:'node',format:'esm',write:false})
const {configChanges,restoreChange,changeScope,registerCopyScope}=await import('data:text/javascript;base64,'+Buffer.from(bundled.outputFiles[0].text).toString('base64'))
const base={heroTitle:'小皮',contactPhone:'12345678',locations:[{id:'a',label:'金融港'}],featuredChannels:[{id:'a',title:'金融港'}],pagePresentations:{home:{title:'今天想住哪里？',copy:{'说明':'原文，保留标点。'}}},listingTitle:{enabledFields:['community','area'],customerAllowedFields:['community','area']},shareCard:{titleFields:['area']},propertyDetail:{facilities:[{key:'tv',label:'电视',icon:'tv'}]}}
const next=structuredClone(base);next.heroTitle='新品牌';next.locations[0].label='新地区';next.featuredChannels[0].title='新地区';next.propertyDetail.facilities[0]={key:'bed',label:'床',icon:'bed'};next.pagePresentations.home.copy['说明']='新文案\n第二行。'
let changes=configChanges(base,next);assert.equal(changes.length,4)
restoreChange(next,base,changes.find(c=>c.id==='coverage'));assert.deepEqual(next.locations,base.locations);assert.deepEqual(next.featuredChannels,base.featuredChannels);assert.equal(next.heroTitle,'新品牌')
restoreChange(next,base,changes.find(c=>c.id==='facilities'));assert.deepEqual(next.propertyDetail.facilities,base.propertyDetail.facilities)
const tags=structuredClone(base);tags.listingTitle.enabledFields=['community'];tags.shareCard.titleFields=['community'];let grouped=configChanges(base,tags);assert.equal(grouped.length,1);restoreChange(tags,base,grouped[0]);assert.deepEqual(tags,base)
const added=structuredClone(base);added.pagePresentations.home.copy['新增']='x';let addition=configChanges(base,added)[0];restoreChange(added,base,addition);assert.equal('新增' in added.pagePresentations.home.copy,false)
assert.match(changeScope(['listingTitle','adminEnabledFields']),/仅管理员/)
assert.equal(changeScope(['pagePresentations','home','title']),'全角色共用')
registerCopyScope('me','测试游客文案',['游客']);assert.equal(changeScope(['pagePresentations','me','copy','测试游客文案']),'游客')
console.log('PASS: change list, exact text/newlines, independent undo, related coverage/facility/tag undo, deleted key restoration, role boundaries')
