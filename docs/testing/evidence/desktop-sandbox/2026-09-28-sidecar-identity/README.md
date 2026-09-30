# macOS sidecar exact identity and retained unknown

Host: macOS 27 arm64. Nexus baseline `0387282d5` plus this change; nxs `db867f87`; Bridge remains pinned to `c994b197e010`. All Go invocations used `GOWORK=off`.

## Change

Native sidecar records now carry version 2, executable path, boot UUID and kernel audit token. Observation checks audit identity before/after the executable query. Signals use `proc_signal_with_audittoken`, including normal shutdown; no bare-PID signal fallback. Records and process descriptors are not deleted because an identity query, signal, or parse failed. A reused PID receives no signal. Updated/development app paths can recover an original version-2 process without requiring its old executable path to equal the new installation path.

Legacy PID/path records remain readable. A provably absent process permits record cleanup. A live legacy process has no exact signal authority and blocks startup with guidance to close the old app; a failed query also remains unknown. This is safe refusal, not automatic retirement of arbitrary historical processes. Capture failure after launch persists a pending legacy-format record and grants no signal authority. File reads are bounded, reject symlinks, nonregular/multilink/wrong-owner files; deletion compares the current typed record with the original owned record.

The existing state-root `NexusSidecar.pid.json` path and canonical aliases are now denied for both reads and writes in restricted nxs and Claude execution policies. Full Access retains its explicit no-isolation meaning.

## Verification

- Nine production test bodies passed using the archived standalone Swift assertion harness because XCTest is unavailable on this CommandLineTools-only host.
- Legacy live/absent, denied observations, PID reuse, updated installation path, corrupt/replaced record, failed signal, symlink rejection, failed capture, and exact-identity escalation are covered.
- Native tests prove a stale audit token cannot terminate the currently live fixture, a matching token terminates it, and a real orphan remains inspectable and can be retired after its parent exits. Only fixed test-created sleep processes are signaled.
- Swift production build passed; affected clientopts package, targeted race, vet and architecture checks passed.
- Real third-party model integration passed for both nxs and Claude (116.70 seconds), including native Read denial and Bash write denial for the sidecar record, unchanged record content, workspace positive controls, public Skill reads, host-private denials, denied network and interrupt/close.

Standalone runner:

```sh
swiftc desktop/macos/Sources/NexusDesktop/Sidecar/SidecarProcessIdentity.swift \
  desktop/macos/Sources/NexusDesktop/Sidecar/SidecarOrphanReaper.swift \
  docs/testing/evidence/desktop-sandbox/2026-09-28-sidecar-identity/tests.swift \
  docs/testing/evidence/desktop-sandbox/2026-09-28-sidecar-identity/main.swift \
  -o /tmp/nexus-sidecar-identity-check
/tmp/nexus-sidecar-identity-check
```

## Limit

No claim that XCTest, signed/notarized package, Intel, clean-host installation or graphical App upgrade/quit/reopen passed. macOS 14.0 exact signal availability is still unresolved; missing API fails explicitly instead of falling back to PID signaling. Unknown/corrupt historical records may require explicit recovery. Full SDK auxiliary IO and graphical App acceptance remain separate. `releaseAccepted=false`.
