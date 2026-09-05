# Frontend Agent entry

Read the existing owner before changing code. Follow these canonical sources:

- [Engineering contract](../docs/specs/frontend-engineering-spec.md): ownership,
  dependencies, public APIs, file contracts, tests and migration procedure.
- [Design system](../design.md): visual and interaction decisions; token and
  recipe implementation remains in the files it names.
- [Current code map](CLAUDE.md), then the affected module's `CLAUDE.md`: existing
  owners, consumers and business boundaries. These maps do not define a second
  engineering or design standard.

Run focused tests while iterating. Before delivery, run `npm run check`;
shared UI, layout and overlay changes also require `npm run test:browser`.
Install its pinned browsers with `npx playwright install chromium webkit` when needed.
`make check-web` runs both gates. Report actual browser/host coverage and any
remaining migration debt without claiming untested native-host behavior.
