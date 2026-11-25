// Keep test navigation deterministic across WeChat DevTools versions.
// simulator_open_page can compile without changing the active page stack in some
// DevTools builds, so use the runtime navigation API for actual E2E navigation.
export function normalizeDevtoolsCall(tool, args) {
  if (tool !== 'simulator_open_page') return { tool, args }
  const pageIndex = args.indexOf('--page')
  if (pageIndex < 0 || !args[pageIndex + 1]) return { tool, args }
  const queryIndex = args.indexOf('--query')
  const page = args[pageIndex + 1].replace(/^\//, '')
  const query = queryIndex >= 0 ? args[queryIndex + 1] : ''
  const url = '/' + page + (query ? '?' + query : '')
  return { tool: 'automation_navigate', args: ['--action', 'reLaunch', '--url', url] }
}
