# Durable settings journal baseline (2026-09-20)

This evidence records the isolated Nexus worktree after SDK commit
`431966dd8862429f80a0bb555aef048d02dedf23` (`:lock: Recover settings writes from
durable transaction journals`) was exported and built into `nxs`. Bridge remains
the exact local module `v0.1.34-0.20260920021254-02fbc0e5f6a6` with checksum
`h1:4sQoNTQUHwidSCAP6DawenSZcrKIhwf23Gm542GI2XU=`. No model request, push, or
main-worktree change was involved.

## Commands and results

```text
GOWORK=off GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /Users/berhand/program/Work/Nexus/worktrees/nexus-agent-sdk-go/desktop-sandbox \
  --sdk-ref 431966dd8862429f80a0bb555aef048d02dedf23
```

The command exited `0` on macOS `27.0` arm64. The report contains 38 checks
with no skipped required test. The fixed SDK archive built an `nxs` binary with
SHA-256:

```text
96da0c6022a7eda42ffe5a3fb3a2df59a1dea80c0d38a67be6a6dbf98b36f4cd
```

The `settings-writers` group passed 46 tests, including:

- successful writes remove their journal;
- restart recovery of full-old and full-new states;
- mixed state fails closed and retains the journal for reconciliation;
- journal content does not contain configuration plaintext.

The settings contract, request admission, native settings paths, Provider
environment and host settings recovery groups also passed. The full JSON report
and selected JSONL logs are stored beside this file.

## Scope limits

The SDK journal provides crash classification and fail-closed recovery; it does
not make multi-root writes power-loss all-or-nothing, does not provide a
domain-level request/approval/revision receipt, and never replays an unknown
write. This is a local macOS development baseline, not clean-host, signed-package,
Windows/Linux native, authenticated Claude, external-provider, or release
acceptance. `releaseAccepted` remains `false`.

