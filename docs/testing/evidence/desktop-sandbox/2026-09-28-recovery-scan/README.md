# Pending process recovery batches — 2026-09-28

Nexus base `82f3937e8`, plus this batch; Bridge remains fixed at
`v0.1.34-0.20260928040950-c994b197e010`. All Go commands use `GOWORK=off`.

## Implementation and verification

- Storage lists exact pending keys by immutable unique launch-ID cursor, with a
  1–256 page bound. Prepared/registered/released rows participate; terminal rows
  do not. Migration 146 adds a partial pending index in SQLite/Postgres.
- Manager holds verified ownership across each batch, re-reads each original
  record, retains item errors/fences and advances to later items. Aggregate errors
  prevent a caller from treating `HasMore=false` as complete success. Cancellation
  retains the last attempted cursor; no task is executed or replayed by the scan.
- `go test -race -count=1 -v ./internal/storage/sandbox ./internal/runtime -run '^TestPendingProcessKeys|^TestProcessRecoveryBatch|^TestProcessPurpose'` passed. Coverage includes terminal exclusion, cross-owner paging, retirement between pages, malformed intent body retention, page bounds, native failure without starvation, ownership rejection, cancellation and continuation, and legacy/multi-purpose migration behavior.
- Existing migration tests now target version 144 explicitly instead of assuming
  that one Down always reaches 144. This preserves the original lossy-rollback
  rejection check at version 145 when migration 146 is present.
- `NEXUS_SUPERVISION_TEST_HELPER=/tmp/nexus-bootstrap-longpaths go test -race -count=1 -v -timeout=90s ./internal/runtime -run '^TestRecoveryScanNativeHostExit$'` passed without skips. A separate host acquires the real state-root flock, persists and releases a real helper task, then exits directly without defers. The parent waits for host exit, acquires the lock, verifies the exact launchd job still reports running, scans SQLite and obtains original-coalition retirement evidence. The second scan is empty.
- Targeted vet and architecture gate passed. SQLite migration execution is covered;
  the Postgres index SQL was reviewed but not run against a Postgres server.

Native helper SHA-256:
`7e21e16effd3c00c0d2805d8c09cc850470cdf6b11d37eb5677c5f1dc31735a8`.
Native test runs a bounded `/bin/sleep` task, not a model request or App UI.
Logs normalize trailing whitespace.

## Remaining boundary

The App does not yet invoke this batch during normal startup. Policy/scratch
unknown reconciliation, older hosts without the instance-lock protocol, default
supervisor setup, platform compatibility and formal delivery acceptance remain
open. Process retirement is not a claim about unknown business/tool effects.
No Windows checks or official Claude authentication were performed.
