import { createRequire } from 'node:module'
import fs from 'node:fs'

const require = createRequire(import.meta.url)
let PNG
try { ({ PNG } = require('pngjs')) }
catch (error) {
  if (error.code !== 'MODULE_NOT_FOUND') throw error
  // Reuse the PNG codec already installed in this workspace's pnpm store.
  ;({ PNG } = require('../../node_modules/.pnpm/node_modules/pngjs'))
}

export const readPng = file => PNG.sync.read(fs.readFileSync(file))

export function comparePixels(expected, actual, masks = [], classifyPixel) {
  const { width, height } = expected
  if (width !== actual.width || height !== actual.height) throw new Error('Screenshot dimensions changed')
  const diff = new PNG({ width, height })
  const tiles = new Map()
  const categories = Object.fromEntries(["application","statusBar","deviceFrame","battery"].map(name=>[name,{changed:0,excluded:0}]))
  let changed = 0
  let compared = 0
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const i = (y * width + x) * 4
      const area = classifyPixel ? classifyPixel(x,y,width,height) : "application"
      const masked = area === "deviceFrame" || area === "battery" || masks.some(m => x >= m.x && x < m.x + m.width && y >= m.y && y < m.y + m.height)
      const delta = Math.max(
        Math.abs(expected.data[i] - actual.data[i]), Math.abs(expected.data[i+1] - actual.data[i+1]),
        Math.abs(expected.data[i+2] - actual.data[i+2]), Math.abs(expected.data[i+3] - actual.data[i+3])
      )
      if (delta > 18) {
        categories[area].changed++
        if (masked) categories[area].excluded++
      }
      if (!masked) compared++
      if (!masked && delta > 18) {
        changed++
        diff.data.set([240, 0, 100, 255], i)
        const key = Math.floor(x / 32) + ':' + Math.floor(y / 32)
        const region = tiles.get(key) || { left:x, top:y, right:x, bottom:y, pixels:0 }
        region.left = Math.min(region.left,x)
        region.top = Math.min(region.top,y)
        region.right = Math.max(region.right,x)
        region.bottom = Math.max(region.bottom,y)
        region.pixels++
        tiles.set(key, region)
      } else {
        if (masked && delta > 18 && classifyPixel) {
          diff.data.set(area === "deviceFrame" ? [42,121,230,255] : [228,147,28,255], i)
          continue
        }
        const shade = masked ? 215 : Math.round(210 + actual.data[i] / 6)
        diff.data.set([shade, shade, shade, 255], i)
      }
    }
  }
  return { changed, compared, ratio: changed / compared, regions: [...tiles.values()], categories, image: PNG.sync.write(diff) }
}

export function exceedsVisualThreshold(metrics) {
  return metrics.ratio > 0.0015 || metrics.regions.some(region => region.pixels > 110)
}

export function checkRect(rect, width, height, visible) {
  const { left, top, width: w, height: h } = rect
  return [left, top, w, h].every(Number.isFinite) && w >= 2 && h >= 2 &&
    left >= -1 && left + w <= width + 1 && (!visible || (top >= -1 && top + h <= height + 1))
}

export function overlaps(a, b) {
  return Math.min(a.left+a.width,b.left+b.width)-Math.max(a.left,b.left) > 1 &&
    Math.min(a.top+a.height,b.top+b.height)-Math.max(a.top,b.top) > 1
}

export function makeFixturePng() {
  const image = new PNG({width:640,height:360})
  for (let y=0;y<image.height;y++) {
    for (let x=0;x<image.width;x++) {
      const color = x < 100 ? [226,86,73] : x >= 540 ? [68,104,173] : y < 180 ? [232,207,126] : [56,151,124]
      image.data.set([...color,255],(y*image.width+x)*4)
    }
  }
  return PNG.sync.write(image)
}
