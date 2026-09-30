# Default macOS supervisor assembly

Host: macOS 27 arm64; Go 1.27.1. Every Go invocation used `GOWORK=off`.
Bridge: `v0.1.34-0.20260928040950-c994b197e010`.
nxs: SDK `db867f87`, `/tmp/nexus-hook-sandbox/nxs`, SHA256 `ecb1f7cf1b4650c1bb77f79a31e9812e4fcc3131e2af5e3de752b8f16b43438f`.
Native helper: `/tmp/nexus-bootstrap-longpaths`, SHA256 `7e21e16effd3c00c0d2805d8c09cc850470cdf6b11d37eb5677c5f1dc31735a8`.

## Implemented

The server entry passes its already-held pre-migration sidecar ownership guard into App construction. macOS desktop construction requires that guard, derives the helper version from the sidecar build, validates the bundle and installs the supervisor on confined `app/processes`. Native recovery pages complete before resource/policy pages. Errors persist across pages and abort this startup; no task is replayed. Runtime shutdown completes before the supervisor directory or DB is closed. Other platforms keep their existing assembly.

Development `make app-run-dev` builds a fixed-Bridge sidecar/helper bundle, then builds the Swift shell. The locator launches the real sidecar directly. Development pointer replacement does not overwrite earlier bundles that may still be in use; `.build` remains ordinary disposable build output.

## Verified

- Targeted race: App shutdown ordering, pending cleanup resource lifetime, recovery pagination, cancellation, no-ownership refusal, sidecar instance tests.
- Complete affected Go packages `internal/app`, `internal/app/server`, `cmd/nexus-server`; targeted vet and architecture check.
- Native race AutoDream fixture now uses the production App installation function with real nxs/helper, long paths, two generations, resource cleanup and durable policy/process facts. It disables model work and does not prove AutoDream model content.
- A real freshly compiled sidecar in the canonical bundle layout starts with an isolated home/state root, returns health 200 and creates `app/processes`; a second process is denied while the original remains healthy; SIGTERM exits 0; changing the helper manifest digest makes startup exit 1. The archived smoke script operates only on its temporary fixture bundle.
- Development sidecar build succeeds with validated helper manifest. Swift production build succeeds.
- Four locator test bodies passed in a standalone Swift assertion harness using production locator/error files. `swift test --filter SidecarBundleLocatorTests` could not compile XCTest because this machine has CommandLineTools but no XCTest framework; the failing log is preserved. Standalone assertions are not a claim that XCTest passed.

## Still unverified

Full graphical App DM/Room/background approval/cancel/permission/backend-switch acceptance, real default sidecar crash/restart with live task descendants, old unguarded sidecars, macOS 14.0 exact process termination support, full SDK auxiliary IO/HTTP hooks and egress, signed/notarized package, clean host/Intel and complete upgrade/rollback. Full Access provides no isolation guarantee. `releaseAccepted=false`.

## Follow-up correction

The original `/health` 200 could come from Web fallback content and is not valid JSON health evidence. The corrected script checks `/nexus/v1/health` and its JSON status; the corrected run is recorded in [sidecar crash evidence](../2026-09-28-sidecar-crash/README.md). That follow-up also verifies real default-sidecar crash/restart with a surviving setsid child.
