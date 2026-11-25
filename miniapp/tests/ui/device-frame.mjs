// Pixel coordinates in the fixed 636 × 1382 iPhone 17 capture.
const originalDeviceFrameMasks = [
  { x:0,y:0,width:636,height:2,reason:'DevTools top capture seam' },
  { x:14,y:2,width:2,height:12,reason:'DevTools top-left rounded-frame seam' },
  { x:34,y:2,width:2,height:1,reason:'DevTools top-left rounded-frame seam' },
  { x:0,y:1360,width:30,height:21,reason:'DevTools text visible outside the lower rounded frame' },
]

export const deviceFrameMasks = originalDeviceFrameMasks.map(({x,y,width,height,...rest}) => ({x:Math.round(x*604/636),y:Math.round(y*1312/1382),width:Math.max(1,Math.round(width*604/636)),height:Math.max(1,Math.round(height*1312/1382)),...rest}))

// Row widths measured against the nine archived core-v1 failure pairs.
// Coordinates cover the outside silhouette, capture seams and battery fill.
const upperLeft = [
  0,43,37,33,31,28,26,25,23,21,20,19,18,16,15,14,13,12,12,11,
  10,9,9,8,7,7,6,6,5,5,4,4,4,3,3,3,2,2,2,2,2,
]
const lowerLeft = [17,17,29,30,28,26,24,22,21,20,19,17,16,15,14,13,12]
const lowerRight = [0,0,0,25,27,25,14,22,20,18,17,16,15,7,7,8,9,10,10]

export function classifyIPhone17Pixel(x, y, width, height) {
  if (width === 604 && height === 1312) { x = Math.floor(x*636/604); y = Math.floor(y*1382/1312); width = 636; height = 1382 }
  if (width !== 636 || height !== 1382) throw new Error('Unexpected iPhone 17 capture dimensions')
  const fromBottom = height - 1 - y
  if (y === 0 || x === 0 || x < (upperLeft[y] ?? 0) ||
      x < (lowerLeft[fromBottom] ?? 0) ||
      width - x <= (lowerRight[fromBottom] ?? 0)) return 'deviceFrame'
  if (x >= 609 && x <= 616 && y >= 41 && y <= 50) return 'battery'
  // The fixture records a 54 logical pixel status bar on a 402-wide screen.
  return y < Math.round(54 * width / 402) ? 'statusBar' : 'application'
}
