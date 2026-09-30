# macOS command hook execution and lifecycle

SDK commit: `db867f87`; Nexus source baseline: `5f8dd7c1d`; Bridge remains pinned to `c994b197e010`.
Host: macOS 27 arm64, Go 1.27.1. Every Go invocation used `GOWORK=off`.
Built nxs: `/tmp/nexus-hook-sandbox/nxs`, SHA256 `ecb1f7cf1b4650c1bb77f79a31e9812e4fcc3131e2af5e3de752b8f16b43438f`.

## Passed

- Native race tests `TestDarwinSandboxCommandHookFiles` and `TestDarwinSandboxAsyncHooksRetire`: shell/argv write policy, scratch positive control, both async forms on close/permission change, closed admission.
- Hook/executor race regression: output caps, preparation and cleanup errors; stdout 1 MiB and stderr 256 KiB. Truncated output cannot become success.
- Public client close race test: SessionEnd runs before the auxiliary admission fence.
- Runtime/client Hook/MCP race regression without native opt-in; native-only cases skipped in this run and covered separately where stated.
- Native `TestDarwinSandboxMCPRuntimeNetwork` without race: both cases passed.
- Real third-party model integration using the fresh nxs and Claude 2.1.273: workspace Write/Bash, public Skill Read, host-private native Read/Write and Bash denial, denied network, interrupt/close. Both backends passed (95.65 seconds); only Provider fields were extracted from main `.env`. This fixture does not itself invoke command hooks.
- Targeted `go vet` for hooks, executor, runtime and client; fresh nxs build.

## Preserved failure

The combined native race regression timed out in both MCP runtime initialization cases. Isolated repetition and `GORACE=atexit_sleep_ms=0` also timed out. The diagnostic 20-second timeout stack locates the wait in sandboxfs instruction/settings reads before MCP connection. sandboxfs deliberately constructs a minimal environment, so the parent GORACE setting does not reach workers. Race-built worker exit delay is a likely contributor, not a proven sole cause. No production timeout or file isolation was weakened. Native MCP race acceptance remains open; these logs must not be counted as passing.

## Boundary

This is command-hook component and integration evidence. Full Access has no isolation guarantee. HTTP hooks, all transcript/resume IO, arbitrary detached descendants, default App supervisor/recovery assembly, macOS 14.0, signed/clean-host/Intel and App DM/Room/background acceptance remain independent. `releaseAccepted=false`.
