export function validateUploadUrl(value, testMode, apiOrigin) {
  if (!testMode) return value
  const allowedOrigin = 'http://127.0.0.1:18083'
  if (apiOrigin !== allowedOrigin || typeof value !== 'string' || !value.startsWith(`${allowedOrigin}/api/`) || /[\\\s#]/.test(value)) {
    throw new Error('Upload URL is not allowed in test mode')
  }
  return value
}
