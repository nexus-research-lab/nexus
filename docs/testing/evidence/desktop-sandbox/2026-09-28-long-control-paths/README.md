# Long private control paths — 2026-09-28

Nexus base `0483ba793` plus this batch. Bridge `c994b197e010`, fixed module
`v0.1.34-0.20260928040950-c994b197e010`; no replace. Host macOS 27.0 arm64.
All Go commands use `GOWORK=off`.

## Results

- Native `TestControlSocket*` race tests pass for long-path communication,
  concurrent ambient relative reads, unchanged process cwd, retained host unlink
  ownership, existing socket rejection, invalid paths and final-parent symlinks.
- `NEXUS_NATIVE_SCOPE_TEST=1 go test -race -count=1 -v ./supervision -run '^TestMacOSSupervisedStart$'`
  passes, including real launchd/helper `long_state_root`, detached descendants,
  canceled close waiters and durable-stage failure cases.
- Native bootstrap/supervision package regression, targeted vet and no-cgo tests
  pass. `TestRecoveryNativeHostExit` uses a separate opt-in and was skipped in this
  regression; the detached-worker child entrypoint is also an expected parent skip.
- Nexus host race tests pass for exact persistence/cleanup, including retained long
  directory paths. `TestSandboxProcessHostNativeLaunch` was not enabled in that run;
  actual native integration was run separately below.
- `TestAppManagedAutoDreamSupervisedNative` passes under race with real fixed nxs,
  the newly built signed helper, and job paths longer than 103 bytes. Both runs
  preserve exact lease IDs, increasing generations, reaped process records,
  retired policy records and scratch deletion. No skips in this explicit test.
- New helper package manifest and loader tests pass; targeted Nexus vet and
  architecture check pass. Logs normalize trailing whitespace.

Helper SHA-256: `7e21e16effd3c00c0d2805d8c09cc850470cdf6b11d37eb5677c5f1dc31735a8`.
Built from the pinned module with native cgo, arm64, trimpath and ad-hoc signing.
nxs SHA-256: `6622c107941409c480a8fbb0a517edfde41f542cb450a365f0e37d047ebdd6aa`
from fixed SDK `2d1fd0d6`.

## Limits

Only the full-path Unix socket address limit is closed. Socket files remain in
original host directories; no short alias or shared temporary control root is
introduced. Host task isolation and filesystem limits still apply. Dedicated
native thread-local cwd support is weak-linked and fails closed if unavailable.
This does not solve the separate macOS 14.0 exact-signal gap or prove Intel,
clean-host, App UI, formal signing or production default supervisor integration.
The native AutoDream gate is disabled: no model consolidation or external request
was performed. No Windows validation.
