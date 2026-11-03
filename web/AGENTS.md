# Frontend Agent entry

Use the pinned pnpm version in `package.json`; `pnpm-lock.yaml` is the only frontend dependency lockfile.

Read the existing owner before changing code. Follow these canonical sources:

- [Engineering contract](../docs/specs/frontend-engineering-spec.md): ownership,
  dependencies, public APIs, file contracts, tests and migration procedure.
- [Design system](../design.md): visual and interaction decisions; token and
  recipe implementation remains in the files it names.
- [Current code map](CLAUDE.md), then the affected module's `CLAUDE.md`: existing
  owners, consumers and business boundaries. These maps do not define a second
  engineering or design standard.

Run focused tests while iterating. Before delivery, run `npm run check` (`make check-web`);
it does not start Playwright. To see what shared components look like, open the dev-only
`ui-gallery.html` from `npm run dev`; it is a viewing page, not a gate. `npm run test:browser`
runs the small set of real-browser flow tests (scroll, auth, DM/Team, settings) on one Chromium
project; run it manually when touching those flows (`npx playwright install chromium` first).
Report remaining migration debt without claiming untested native-host behavior.

Behavior tests use Vitest beside their owners (`*.test.ts`, `*.test.tsx`, or
`*.test.mjs`; Node-only tests declare their environment). Do not start Vite SSR
servers or manually transpile TypeScript in tests. Assert behavior, requests and
accessible state, not source text, comment wording, Tailwind classes or decorative
styles. Computed geometry, scroll locking and user preferences remain behavioral
contracts. Architecture and file-header structure belong to ESLint; CSS token
integrity remains an AST/CSS tool check. Gallery registration is voluntary.
Real-backend smoke runs through `TestBrowserSmoke`; see
[`test-system.md`](../docs/testing/test-system.md).
