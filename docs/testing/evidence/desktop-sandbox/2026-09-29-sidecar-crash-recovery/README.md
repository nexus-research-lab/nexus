# macOS sidecar crash/restart recovery

- Date: 2026-09-29
- Nexus commit: `48b267de9`
- SDK commit: `29416cd7`
- Bridge commit: `c251a8d`
- Branch: `codex/desktop-sandbox-approvals`
- Harness: `scripts/desktop/check-sidecar-crash.mjs`
- Runtime: bundled macOS arm64 App, restricted nxs fork, isolated temporary state root

The real sidecar harness started a detached descendant, terminated the original
host, confirmed that the descendant outlived the host, restarted the App and
reconciled durable records. The captured harness result was:

```text
detached-child-alive
original-host-killed
descendant-survived-host
PASS-default-crash-recovery
  processes: 1
  policies: 1
  scratch: 1
  noReplay: true
  detachedChildGone: true
fixture-stopped
```

The compact report records the result facts only. The temporary fixture root
and raw event/state files were cleaned before archival, so this commit does not
claim raw-log reproducibility. This evidence supports the current default
macOS startup recovery path for the tested nxs fixture. It does not prove
macOS 14.0 exact signal support, full graphical App acceptance, Developer ID
signing, notarization, clean-host Gatekeeper installation, Intel packaging, or
production release acceptance. `releaseAccepted=false` remains required.
