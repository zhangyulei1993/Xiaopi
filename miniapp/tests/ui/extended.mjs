import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { makeFixturePng } from './pixels.mjs'

export async function runExtended({ state, cli, el, open, evaluate, run, out, db, waitForPage, retryOnly }) {
  const listing = 'pages/properties/index'
  const detail = 'pages/properties/detail'
  const form = 'pages/requests/index'
  const id = 'qa-property-1'
  const api = 'xiaopi-miniapp-test-api-test-1'
  const mediaId = 'qa-ui-visual-media'
  const mediaPath = '/workspace/backend/uploads/qa-ui-visual.png'
  const mysql = sql => run('docker',['exec',db,'sh','-ec',
    '[ "$MYSQL_DATABASE" = xiaopi_miniapp_test ] && MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -N "$MYSQL_DATABASE" -e "$1"',
    'sh',sql]).trim()
  const rect = selector => ({...el('size',selector),...el('offset',selector)})
  const scroll = (selector, top) => cli('automation_viewport_action',['--action','pageScrollTo','--scroll-top',String(top),'--wait-for-selector',selector])
  const tap = selector => el('trigger',selector,['--type','tap'])
  const detailOpen = () => open(detail,'id='+id)

  // Probe native viewport scrollLeft and the rendered width of the route's root.
  function horizontal(selector) {
    return result => {
      const source = `function(){return new Promise(resolve=>{const q=wx.createSelectorQuery();q.selectViewport().scrollOffset();q.select(${JSON.stringify(selector)}).boundingClientRect();q.exec(resolve)})}`
      const [viewport, content] = evaluate(source,true)
      result.horizontal = { viewport, content }
      assert.ok(Number.isFinite(viewport?.scrollLeft),'Native horizontal offset unavailable')
      assert.ok(content && Number.isFinite(content.right),'Native content bounds unavailable')
      assert.equal(viewport.scrollLeft,0)
      assert.ok(Number.isFinite(viewport.scrollWidth) && viewport.scrollWidth <= result.layoutViewport.width+1,'Native page scroll width exceeds viewport')
      assert.ok(content.left >= -1 && content.right <= result.layoutViewport.width+1,'Page root exceeds horizontal viewport')
    }
  }
  await state('horizontal-home','pages/home/index','.hero-search',[['.hero-search',true]],{verify:horizontal('.home-page')})
  await state('horizontal-list',listing,'.masonry-property',[['.property-listing-title',true]],{setup:()=>open(listing),verify:horizontal('.page')})
  await state('horizontal-detail',detail,'.overview',[['.overview .title',true]],{setup:detailOpen,verify:horizontal('.detail-page')})
  await state('horizontal-form',form,'.request-hero',[['.request-hero',true]],{setup:()=>open(form),verify:horizontal('.page')})

  assert.equal(mysql("SELECT COUNT(*) FROM property_media WHERE id='"+mediaId+"' OR property_id='"+id+"'"),'0')
  run('docker',['exec',api,'sh','-ec','test ! -e "$1"','sh',mediaPath])
  const asset = path.join(out,'synthetic-image.png')
  fs.writeFileSync(asset,makeFixturePng())
  try {
    run('docker',['exec',api,'mkdir','-p','/workspace/backend/uploads'])
    run('docker',['cp',asset,api+':'+mediaPath])
    mysql("INSERT INTO property_media (id,property_id,media_type,url,sort_order) VALUES ('"+mediaId+"','"+id+"','image','/uploads/qa-ui-visual.png',0)")
    await state('detail-with-image',detail,'.media-image',[['.media-image',true],['.media-swiper',true]],{
      setup:detailOpen,absent:'.detail-cover-empty',
      verify:result=>{
        assert.equal(el('attribute','.media-image',['--name','mode']),'aspectFill')
        const image = result.nodes['.media-image']
        const frame = result.nodes['.media-swiper']
        assert.ok(Math.abs(image.width-frame.width)<=2 && Math.abs(image.height-frame.height)<=2,'Image does not fill frame')
      },
      verifyImage:(actual,result)=>{
        const frame = result.nodes['.media-swiper']
        if (!frame) return
        const scale = actual.width / 402
        // Simulator color management shifts the synthetic PNG channels.
        const colors = [[232,207,126],[56,151,124],[226,86,73],[68,104,173]]
        result.fixturePixels = colors.map(color=>{
          let count = 0
          for (let y=Math.ceil(frame.top*scale); y<Math.floor((frame.top+frame.height)*scale); y+=3) {
            for (let x=Math.ceil(frame.left*scale); x<Math.floor((frame.left+frame.width)*scale); x+=3) {
              const offset = (y*actual.width+x)*4
              if (color.every((channel,index)=>Math.abs(actual.data[offset+index]-channel)<=32)) count++
            }
          }
          return count
        })
        assert.ok(result.fixturePixels.every(count=>count>100),'Synthetic image colors absent from rendered frame')
      },
    })
  } finally {
    mysql("DELETE FROM property_media WHERE id='"+mediaId+"' AND property_id='"+id+"'")
    run('docker',['exec',api,'rm','-f',mediaPath])
  }

  const favoriteFilter = '.favorite-filter'
  const heart = '.masonry-property .favorite'
  const favoriteWhere = "user_id='qa-user-a' AND property_id='"+id+"'"
  const favoriteCount = () => mysql('SELECT COUNT(*) FROM property_favorites WHERE '+favoriteWhere)
  assert.equal(favoriteCount(),'0')
  try {
    await state('favorite-added',listing,heart+'.active',[[heart+'.active',true]],{
      setup:async()=>{ open(listing); await waitForPage(listing,heart); tap(heart) },verify:()=>assert.equal(favoriteCount(),'1'),
    })
    await state('favorite-list',listing,'.masonry-property',[[heart+'.active',true],['.property-listing-title',true]],{
      setup:async()=>{ await waitForPage(listing,heart+'.active'); el('tap',favoriteFilter) },text:[[favoriteFilter,/只看已关注/],['.property-listing-title',/TEST Community A/]],
    })
    await state('favorite-removed',listing,'.favorites-empty',[['.favorites-empty',true]],{
      setup:async()=>{ await waitForPage(listing,heart+'.active'); tap(heart+'.active') },verify:()=>assert.equal(favoriteCount(),'0'),
    })
  } finally {
    if (favoriteCount() !== '0') mysql('DELETE FROM property_favorites WHERE '+favoriteWhere)
    assert.equal(favoriteCount(),'0')
  }

  if (!retryOnly) {
    await state('guest-login-prompt','pages/requests/history','.empty.state',[['.empty.state button',true]],{
      setup:()=>{
        evaluate('function(){wx.removeStorageSync("xiaopi_access_token");wx.removeStorageSync("xiaopi_session_role");return true}')
        open('pages/requests/history')
      },text:[['.empty.state',/登录后查看找房记录/]],
    })
    await state('guest-login-destination','pages/auth/login','.login-title',[['.login-title',true]],{
      setup:()=>el('tap','.empty.state button'),
      verify:()=>assert.equal(cli('automation_runtime_info',['--action','currentPage'],true).currentPage.query.reason,'requests'),
    })

  }

  const response = await fetch('http://127.0.0.1:18083/api/v1/auth/wechat/login',{
    method:'POST',redirect:'error',signal:AbortSignal.timeout(5000),
    headers:{'Content-Type':'application/json'},body:JSON.stringify({code:'qa-admin'}),
  })
  assert.equal(response.status,200)
  const {data:session} = await response.json()
  assert.ok(session.testOnly && session.role==='admin' && session.accessToken)
  const args = path.join(out,'.admin-args.json')
  try {
    fs.writeFileSync(args,JSON.stringify([session.accessToken]),{mode:0o600,flag:'wx'})
    cli('automation_evaluate',['--fn-source','function(token){wx.setStorageSync("xiaopi_access_token",token);wx.setStorageSync("xiaopi_session_role","admin");wx.setStorageSync("xiaopi_admin_tab","requests");return true}','--args-file',args])
  } finally { fs.rmSync(args,{force:true}) }
  await state('admin-appointment-list','pages/admin/index','.request-property',[['.request-property',true],['.card.item .status',true]],{
    setup:()=>open('pages/admin/index'),text:[['.request-property',/预约房源[\s\S]*TEST Community A/]],
  })
  await state('admin-appointment-scroll','pages/admin/index','.request-property',[['.request-property',false]],{
    setup:async()=>{
      await waitForPage('pages/admin/index','.request-property')
      const before = rect('.request-property').top
      scroll('.request-property',320)
      assert.ok(rect('.request-property').top < before-30,'Appointment list did not scroll')
    },
  })
}
