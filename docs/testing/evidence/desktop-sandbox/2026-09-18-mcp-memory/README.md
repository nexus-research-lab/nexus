# MCP helper and managed memory evidence (2026-09-18)

This archive records the clean local cross-repository baseline after SDK `81104dd9` and Nexus `064ccb7fa`. The fixed Bridge remains the local `a2316d7` pseudo-version in Nexus. The baseline command passed on macOS arm64 with `GOWORK=off GOPROXY=off`; `releaseAccepted=false` remains because publishing, installation, Windows/Linux native execution, Claude acceptance, and full background IO confinement are still pending.

The SDK change makes `headersHelper` consume the runtime-owned environment even when `NEXUS_PROVIDER_MANAGED_BY_HOST=1` exists only in `Options.Env`; known Provider credentials and managed memory roots are filtered while ordinary task variables remain. Runtime memory consumers ignore managed task settings/env roots. Nexus fixes the nxs memory root after `ConfigurationEnv` merging and includes memory ownership in the process identity.

`baseline-report.json` and the compressed command logs preserve the exact source revisions, commands, exits and skips. The archive intentionally omits generated nxs/sdk binaries; the report records their build inputs and hashes where applicable. This evidence does not prove secret-file, process/handle, external MCP process, network, descendant, durable-recovery, scratch, or release isolation.

Re-run:

```sh
GOWORK=off GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /absolute/nexus-agent-sdk-go --sdk-ref 81104dd9
```
