# Claude Bridge restricted admission evidence

This batch connects Nexus' Claude restricted selection to Bridge's typed
`RequireClaudeRestricted` process admission. It is local evidence only; no
repository was pushed and no authenticated model request was made.

## Fixed inputs

- Bridge commit: `6ea77309fdd4ed4f8177e9d9731253cd1f03879c`
- Bridge module: `v0.1.34-0.20260918074231-6ea7730`
- Module checksum: `h1:xRd4iHVLEDL08Ey0FqYk66D087/SkkQsGrl8B0LfxVQ=`
- SDK commit: `9d60e166`
- nxs SHA-256: `374a022e84a1dd081c2c9e2b56474dcc61dbfd4868b70f9fbeaf05de8ff49330`

## Checks

- `bridge-target.log`: `GOWORK=off GOPROXY=off go test ./client ./internal/transport -count=1`
- `bridge-race.log`: `GOWORK=off GOPROXY=off go test -race ./client ./internal/transport -count=1`
- `bridge-vet.log`: `GOWORK=off GOPROXY=off go vet ./client ./internal/transport`
- `nexus-clientopts.log`: Nexus Claude restricted/Full Access wiring tests with the
  exact local Bridge module and `GOWORK=off`
- `nexus-desktop-gate.log`: `NEXUS_SANDBOX_TEST_BINARY=/tmp/nxs-settings-rollback make check-desktop-sandbox`
- `claude-cli-probe.json`: macOS CLI `2.1.273`, help advertisement and restricted
  bypass rejection, using a temporary config root and no prompt

Bridge's preflight runs the exact resolved CLI with `--restricted --help` before
stream-json admission. It requires successful exit and advertised help text,
with bounded output/time and common Provider/proxy secret variables removed.

This does not prove an authenticated Claude session, Provider/file/network/OS
isolation, descendant cleanup, cancellation, Windows/Linux behavior, or package
installation/upgrade. The overall desktop acceptance remains `releaseAccepted=false`.
