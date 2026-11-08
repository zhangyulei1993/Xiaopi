# Admin architecture

## Runtime layers

- `src/main.ts`: application bootstrap, Element Plus and router registration.
- `src/App.vue`: minimal application entry.
- `src/pages/AdminWorkspacePage.vue`: authenticated workspace coordinator; it owns shared operator state while route modules migrate out.
- `src/router.ts`: refresh-safe hash routes. The hash strategy matches the admin subdomain root deployment and requires no server rewrite.
- `src/services/`: API clients and transport concerns. Components should not assemble authorization headers themselves.
- `src/types/`: shared domain contracts returned by the API.
- `src/config/`: navigation, presentation defaults and compatibility normalization.
- `src/components/`: layout primitives shared by all modules (`AdminSidebar`, `AdminTopbar`).
- `src/pages/`: reserved for the next extraction step; each route can be moved here without changing API contracts.

## Conventions

1. API response parsing and 401 handling live in `services/`.
2. Route names are stable slugs; Chinese labels are presentation only.
3. Defaults and legacy-value migration live in `config/`, not in templates.
4. Build output is generated with `npm run build` and deployed as the contents of `dist/` to the admin subdomain site root.

The current business template remains in `App.vue` during this migration so the deployed behavior stays unchanged. New modules should be extracted into `pages/` one route at a time.
