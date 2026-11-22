# Isolated Mini Program Test Environment

The test stack is independent of the production Compose project and database.
It uses Compose project `xiaopi-miniapp-test`, database `xiaopi_miniapp_test`,
private MySQL, and an API bound to `127.0.0.1:18083`. In `APP_ENV=test`, the
backend HTTP transport rejects outbound requests. The optional map fixture is
enabled only with `TEST_FAKE_MAP_ENABLED=true`; it returns a fixed QA location
without making network calls. For live map testing, create the ignored local
`tests/.env.map.test` from `.env.map.test.example` and set `TENCENT_MAP_KEY` and
`TEST_REAL_MAP_ENABLED=true` there.
In that mode the test transport permits only HTTPS GET requests to Tencent's
`/ws/place/v1/search` endpoint, with the configured key and known search
parameters; all other outbound requests remain blocked. The API key stays in
the backend container and is never sent to the mini program. When a key is
configured, the live response takes precedence over the QA fixture.
Do not copy admin, database, or production service credentials into
`tests/.env.test`; the optional map key belongs only in `tests/.env.map.test`.

## Commands

From the repository root:

```sh
tests/scripts/test-environment.sh config   # create local-only credentials and validate Compose; does not start services
tests/scripts/test-environment.sh init     # start isolated MySQL/API and seed synthetic QA fixtures
tests/scripts/test-environment.sh status
tests/scripts/test-environment.sh reset    # remove/reseed only qa-* records in xiaopi_miniapp_test
tests/scripts/test-environment.sh down     # stop only xiaopi-miniapp-test; preserves its test volume
node tests/scripts/test-business-api.mjs   # API + isolated MySQL business verification
```

From `miniapp/`:

```sh
npm run test:build
npm run test:smoke
npm run test:e2e
npm run test:report
```

Simulator runs default to `dist/mp-weixin-test`. The runner refuses the exact
production output directory unless `WECHAT_ALLOW_PRODUCTION_AUTOMATION=1` is
explicitly set. Keep that override unset. The native runner serializes access
with a project lock and waits for page/selector readiness before interaction.
For the test output it logs into the first synthetic user through the test-only
UI before running page cases, which refreshes the short-lived test token.
It retries only classified read-only requests after an Automator response
timeout; UI state changes and submissions are never retried automatically.

## Fixtures and roles

`seed.test.sql` owns only the `qa-*` synthetic accounts and `qa-property-1`.
The current backend implements `user` and `admin`; landlord and super-admin
roles are not implemented. Test login codes are allowed only when
`APP_ENV=test`, mapped server-side to seeded identities, and roles are resolved
from the database. The client cannot submit a role. This does not simulate a
real WeChat identity exchange or phone authorization.

`reset.test.sql` deletes only the named QA users, property, and their dependent
test rows, then `test-environment.sh reset` reseeds the fixtures. It never
references a production database or uses production credentials.

## New project onboarding

Run `npm run wechat-project-init` inside a compatible UniApp Mini Program
project. Review generated project test skeletons before adding cases. Set
`WECHAT_PROJECT` when invoking the shared runner for a different project, build
that project's test output to `dist/mp-weixin-test`, and use a unique project
name/database/port if its test backend is hosted on the same Mac. Never put
project-specific business names, credentials, or fixtures in the global Skill.
