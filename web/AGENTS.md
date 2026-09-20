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

Run focused tests while iterating. Before delivery, run `npm run check`;
shared UI, layout and overlay changes also require the explicit `npm run test:browser:full` browser matrix.
Install its pinned browsers with `npx playwright install chromium webkit` when needed.
Use `npm run test:browser:smoke` for the isolated UI Gallery project and `make check-web-ui` for
the standard checks plus the complete browser gate. `make check-web` stays on the
standard frontend checks and does not start Playwright. Report actual browser/host coverage and any
remaining migration debt without claiming untested native-host behavior.
