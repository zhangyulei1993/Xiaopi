const COS_HOST = 'cos.xiaopizufang.cn'

export function validateDownloadUrl(value, apiOrigin) {
  if (typeof value !== 'string' || !value.trim()) throw new Error('Download URL is invalid')
  let url
  let api
  try {
    url = new URL(value)
    api = new URL(apiOrigin)
  } catch {
    throw new Error('Download URL is invalid')
  }
  const apiAsset = url.origin === api.origin
  const cosAsset = url.protocol === 'https:' && url.hostname === COS_HOST
  if (!apiAsset && !cosAsset) throw new Error('Download URL is not allowed')
  return url.toString()
}
