# 2026-09-20 Bridge provider environment boundary

This local batch hardens the final Bridge process environment. The Bridge drops
credential-like names inherited from the host shell before applying typed
`Options.Env` overrides, so a selected Provider can be projected explicitly
without lending the child a different host login or proxy credential.

## Fixed inputs

- Nexus worktree: `codex/desktop-sandbox-isolated`
- SDK commit: `431966dd8862429f80a0bb555aef048d02dedf23`
- Bridge commit: `436346420c2905907375cc63b8fee9b88bc07287`
- Bridge module: `v0.1.34-0.20260920062457-436346420c29`
- Bridge module checksum: `h1:kwNq8HuqV0NihWeE96YulddxdlX6Gbq+XiAJEkGs8nM=`
- Bridge module proxy: `/private/tmp/nexus-bridge-4363464-proxy`

## Checks

- `bridge-transport-target.log`: Bridge transport target tests, including
  inherited secret removal and explicit override preservation.
- `bridge-transport-race.log` and `bridge-transport-vet.log`: race and vet for
  the changed transport package.
- `bridge-client-transport.log` and `bridge-client-transport-vet.log`: client
  plus transport regression and vet.
- `bridge-cross-compile.log`: Windows amd64 and Linux amd64 test-binary
  compilation for Bridge client/transport.
- `sdk-settings-target.log`, `sdk-settings-race.log`, `sdk-settings-vet.log`:
  the fixed SDK settings package remains green.
- `sdk-settings-cross-compile.log`: Windows/Linux amd64 compilation for the
  SDK settings package.
- `nexus-runtime-target.log` and `nexus-runtime-vet.log`: Nexus clientopts and
  runtime checks with the exact local Bridge module.

All commands completed with exit code 0. No model request or external Provider
request was made. This evidence covers process environment input filtering only;
it does not prove secret-file, inherited-handle, helper-process, network-egress,
Claude OS sandbox, descendant cleanup, native Windows/Linux runtime, or package
acceptance. `releaseAccepted=false` remains intentional.
