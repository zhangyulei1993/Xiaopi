export function safeRequestUrl(input, apiBase, testMode) {
  if (!testMode) return `${apiBase}${input}`
  const allowed = 'http://127.0.0.1:18083/api'
  if (apiBase !== allowed || typeof input !== 'string' || /[\\\s#]/.test(input)) throw new Error('Request URL is not allowed in test mode')
  const path = input.startsWith(`${allowed}/`) ? input.slice(allowed.length) : input
  if (!path.startsWith('/') || path.startsWith('//') || /^(?:[^?]*)(?:%2f|%5c|%2e)/i.test(path) || /(?:^|\/)\.\.(?:\/|\?|$)/.test(path)) throw new Error('Request URL is not allowed in test mode')
  const result = `${allowed}${path}`
  if (!result.startsWith(`${allowed}/`) || /:\/\//.test(path)) throw new Error('Request URL is not allowed in test mode')
  return result
}
