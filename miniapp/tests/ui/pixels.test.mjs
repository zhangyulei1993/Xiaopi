import test from 'node:test'
import assert from 'node:assert/strict'
import { comparePixels, exceedsVisualThreshold, checkRect, overlaps } from './pixels.mjs'

import { classifyIPhone17Pixel } from './device-frame.mjs'

const frame = () => ({width:64,height:64,data:Buffer.alloc(64*64*4,255)})
const set = (image,x,y) => image.data[(y*64+x)*4] = 0

test('precise masks suppress only the selected dynamic area',()=>{
  const expected = frame(), actual = frame()
  set(actual,10,10)
  set(actual,42,48)
  const diff = comparePixels(expected,actual,[{x:9,y:9,width:3,height:3}])
  assert.equal(diff.changed,1)
  assert.equal(diff.compared,4096-9)
  assert.deepEqual(diff.regions,[{left:42,right:42,top:48,bottom:48,pixels:1}])
  assert.equal(diff.image.subarray(1,4).toString(),'PNG')
})
test('mismatched screenshot sizes are rejected',()=>{
  assert.throws(()=>comparePixels(frame(),{...frame(),width:65}),/dimensions/)
})
test('layout bounds reject horizontal overflow, clipped visible controls and zero sizes',()=>{
  const box = {left:10,top:10,width:40,height:20}
  assert.ok(checkRect(box,100,100,true))
  assert.ok(!checkRect({...box,left:80},100,100,false))
  assert.ok(!checkRect({...box,top:90},100,100,true))
  assert.ok(checkRect({...box,top:90},100,100,false))
  assert.ok(!checkRect({...box,width:0},100,100,true))
  assert.ok(overlaps(box,{...box,left:40}))
  assert.ok(!overlaps(box,{...box,left:50}))
})

const deviceFrame = () => ({width:636,height:1382,data:Buffer.alloc(636*1382*4,255)})
function paint(image, x, y, width, height) {
  for(let row=y;row<y+height;row++) {
    for(let column=x;column<x+width;column++) {
      image.data[(row*image.width+column)*4] = 0
    }
  }
}
const compareDevice = (before,after) => comparePixels(before,after,[],classifyIPhone17Pixel)

test('capture seams, rounded corners and battery fill are counted but excluded',()=>{
  const before=deviceFrame(),after=deviceFrame()
  paint(after,0,0,636,1)
  paint(after,0,1,20,4)
  paint(after,0,1368,10,10)
  paint(after,609,41,8,10)
  const result=compareDevice(before,after)
  assert.equal(result.categories.deviceFrame.excluded,816)
  assert.equal(result.categories.battery.excluded,80)
  assert.equal(result.categories.application.changed,0)
  assert.equal(result.changed,0)
  assert.ok(!exceedsVisualThreshold(result))
})

test('unmasked status bar stays in the comparison',()=>{
  const before=deviceFrame(),after=deviceFrame()
  paint(after,550,38,12,10)
  const result=compareDevice(before,after)
  assert.deepEqual(result.categories.statusBar,{changed:120,excluded:0})
  assert.ok(exceedsVisualThreshold(result))
})

for(const [name,drawBefore,drawAfter] of [
  ['button moves',before=>paint(before,40,230,28,16),after=>paint(after,40,260,28,16)],
  ['visible text changes',
    before=>{ paint(before,40,230,5,21); paint(before,40,230,15,4) },
    after=>{ paint(after,51,230,5,21); paint(after,40,247,15,4) }],
  ['component disappears',before=>paint(before,40,230,28,16),()=>{}],
]) test(name+' triggers FAIL under the original thresholds',()=>{
  const before=deviceFrame(),after=deviceFrame()
  drawBefore(before)
  drawAfter(after)
  const result=compareDevice(before,after)
  assert.ok(result.categories.application.changed>0)
  assert.ok(exceedsVisualThreshold(result))
})
