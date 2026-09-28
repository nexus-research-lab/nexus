# Real default-sidecar crash/restart

Implementation baseline: Nexus `77c7e2340`; nxs `db867f87`; pinned Bridge `c994b197e010`.
Host: macOS 27 arm64. Sidecar SHA256 `b2c6069a72233392230ba249492287c04632a32e8bb5ea8ef17a32a53b58587c`; nxs SHA256 `ecb1f7cf1b4650c1bb77f79a31e9812e4fcc3131e2af5e3de752b8f16b43438f`.

## Real path and assertions

`scripts/desktop/check-sidecar-crash.mjs` launches the actual canonical-layout sidecar bundle using an independent HOME/state root. Only Provider token/base URL/model are read from the explicitly selected main `.env`. Provider, model, Agent and DM are created through HTTP; chat enters the production WebSocket route. The real third-party model invokes nxs Bash to start one bounded Ruby child that calls setsid, detaches standard IO, writes its PID/count marker and sleeps for at most 180 seconds.

Two independent runs passed. The child is live before and after SIGKILL of the original sidecar. A fresh default sidecar starts against the same database and automatically recovers the original launch: phase `released` to `reaped`, resource phase `complete`, scratch recovery `complete`, policy `confirmed` to `reconciled`. Original launch IDs are unchanged and the detached process disappears. After a further two-second observation, the command count is unchanged and no new launch record exists. The actual JSON health endpoint is available after recovery; no direct Manager call, injected recovery result, manual receipt update or SDK mock participates.

The original policy is `confirmed` because abrupt SIGKILL cannot persist an `unknown` transition. Recovery reconciles the original process binding; this does not prove a model/tool business outcome, mark the task complete or authorize replay. Explicit preexisting unknown cases remain covered by separate runtime/storage tests.

## Health evidence correction

The previous `2026-09-28-default-supervisor/smoke.py` used `/health`, which can hit Web fallback content. Its 200 alone was not valid JSON health evidence. The script now uses `/nexus/v1/health` and verifies `data.status == "ok"`; `corrected-smoke.log` supersedes that earlier health assertion. Corrected default startup, duplicate-sidecar denial, normal exit and tampered-helper-manifest denial all passed. The crash script also validates JSON health.

## Invocation

```sh
NEXUS_SANDBOX_SIDECAR_BINARY=/absolute/Nexus.app/Contents/MacOS/nexus-server \
NEXUS_SANDBOX_TEST_BINARY=/absolute/nxs \
NEXUS_SANDBOX_LIVE_ENV_FILE=/absolute/main/.env \
GOWORK=off node scripts/desktop/check-sidecar-crash.mjs
```

The script reports its temporary fixture root. That private fixture contains Provider configuration and must not be uploaded wholesale. Archived snapshots include only sandbox process/policy/resource records, never the application database or Provider credentials.

## Remaining

This is a real sidecar/HTTP/WebSocket/nxs/model/crash recovery result on this host. It is not graphical App acceptance, Claude-backend crash acceptance, old-sidecar ownership validation, macOS 14/Intel proof, or signed/notarized/clean-host release acceptance. `releaseAccepted=false`.
