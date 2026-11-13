const chunks = []
for await (const chunk of process.stdin) chunks.push(chunk)
const config = JSON.parse(Buffer.concat(chunks).toString('utf8'))
const allowInternal = process.argv.includes('--allow-internal')
const api = config.services?.['api-test']
const db = config.services?.['mysql-test']
const fail = (message) => { console.error(message); process.exit(1) }
if (config.name !== 'xiaopi-miniapp-test') fail(`Unexpected Compose project: ${config.name}`)
if (!api || !db) fail('Expected isolated api-test and mysql-test services')
if (api.network_mode || db.network_mode || api.container_name || db.container_name) fail('Host networking or fixed container names are forbidden')
if (api.environment?.APP_ENV !== 'test' || api.environment?.MYSQL_HOST !== 'mysql-test' || api.environment?.MYSQL_DATABASE !== 'xiaopi_miniapp_test') fail('API is not bound to the isolated test database')
if (api.environment?.WECHAT_MOCK_ENABLED !== 'false') fail('Generic WeChat mock must remain disabled in the test API')
if (!api.environment?.AUTH_TOKEN_SECRET || !api.environment?.TEST_WECHAT_CODE_MAP) fail('Independent test auth settings are missing')
if (JSON.stringify(api).includes('api.xiaopizufang.cn')) fail('Production API host appears in test API configuration')
const apiPort = api.ports?.some((port) => port.host_ip === '127.0.0.1' && String(port.published) === '18083')
if (!apiPort) fail('Test API must bind only to loopback port 18083')
if (db.ports?.length) fail('Test MySQL must not publish a host port')
const network = config.networks?.test_internal
if (!network?.internal && !(allowInternal && network?.driver === 'bridge')) fail('Test network must be internal with no external egress')
if (!Object.hasOwn(api.networks ?? {}, 'test_internal') || Object.keys(api.networks ?? {}).length !== 1) fail('Test API must use only the internal test network')
if (!Object.hasOwn(db.networks ?? {}, 'test_internal') || Object.keys(db.networks ?? {}).length !== 1) fail('Test MySQL must remain private to the internal test network')
if (config.volumes?.test_mysql_data?.name !== 'xiaopi_miniapp_e2e_test_mysql_data') fail('Test DB volume name is not isolated')
console.log(`Compose declares loopback API mapping, private MySQL, ${network.internal ? 'internal-only' : 'standard'} test network, fixed test DB, unique volume.`)
