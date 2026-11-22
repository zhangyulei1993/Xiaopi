/* Paste into the appservice Console of the isolated iPhone remote session. */
(() => {
  const key = '__qaDeviceKeyboard'
  if (globalThis[key]) globalThis[key].stop()
  let sequence = 0
  let height = null
  const entries = []
  function record(kind, value) {
    const entry = {sequence:++sequence,at:new Date().toISOString(),kind,...value}
    entries.push(entry)
    console.info('[QA_DEVICE_KEYBOARD]', JSON.stringify(entry))
  }
  function listener(event) {
    height = event.height
    record('keyboard-height',{height})
  }
  const bounds = node => node && {
    left:node.left,top:node.top,right:node.right,bottom:node.bottom,width:node.width,height:node.height,
  }
  async function snapshot(stage) {
    const route = getCurrentPages().slice(-1)[0]?.route
    const appointment = route === 'pages/properties/detail'
    if (!appointment && route !== 'pages/requests/index') throw new Error('Open the QA appointment sheet or request form first')
    const query = wx.createSelectorQuery()
    query.selectViewport().scrollOffset()
    query.select(appointment ? '.appointment-field input' : '.field input').boundingClientRect()
    query.select(appointment ? '.appointment-submit' : '.button').boundingClientRect()
    query.select(appointment ? '.appointment-sheet' : '.form').boundingClientRect()
    const [viewport,input,submit,container] = await new Promise(resolve=>query.exec(resolve))
    const window = wx.getWindowInfo()
    record('snapshot',{stage,route,height,window:{
      windowWidth:window.windowWidth,windowHeight:window.windowHeight,screenHeight:window.screenHeight,
      statusBarHeight:window.statusBarHeight,
    },scroll:{top:viewport?.scrollTop,left:viewport?.scrollLeft},input:bounds(input),submit:bounds(submit),container:bounds(container)})
  }
  wx.onKeyboardHeightChange(listener)
  globalThis[key] = {
    snapshot,
    entries:()=>entries.slice(),
    stop:()=>{
      wx.offKeyboardHeightChange(listener)
      delete globalThis[key]
    },
  }
  console.info('[QA_DEVICE_KEYBOARD] ready')
})()
