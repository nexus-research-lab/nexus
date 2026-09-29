# Bundled runtime compatibility after restricted fork

- Date: 2026-09-29
- Nexus source: `d8eba8672`
- SDK source: `fd9a97ac`
- Bridge source: `c251a8d`
- Branch: `codex/desktop-sandbox-approvals`
- nxs SHA-256: `0175dce74068f9f54fb52107db8e31448727c9cad88a5bb5854cc3a159bfa1db`
- Result: bundled runtime compatibility check passed; isolated `make app-check` passed

The App was rebuilt with the current SDK nxs and matching ripgrep sidecar. The
bundled compatibility profiles (workspace-write, read-only and full-access)
completed, and the smoke used an isolated state root. This validates packaging
of the current runtime path; it remains ad-hoc signed and does not prove
Developer ID, notarization, clean-host Gatekeeper, Intel or production release
acceptance (`releaseAccepted=false`).
