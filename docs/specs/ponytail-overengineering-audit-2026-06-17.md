# Ponytail overengineering audit - 2026-06-17

## Scope

This audit complements `engineering-structure-audit-2026-06-16.md`.
The existing audit is mostly about file boundaries. This pass looks for code
that exists only because we are being defensive: silent fallback paths,
open-ended legacy support, and abstractions that hide wrong state instead of
failing clearly.

Worktree state during this pass:

- Branch: `main`
- Upstream: `origin/main`
- Local status: clean, ahead of `origin/main` by 50 commits
- Local sample DB: `deploy/data/.nexus/data/nexus.db`

This pass is read-mostly and safe to run alongside the active decomposition
work. Do not start broad rewrites from this document. Take one finding at a
time, verify current production data, add the smallest regression test, then
delete or shrink the path.

## Keep

These are not Ponytail debt:

- Validation at HTTP, WebSocket, CLI, runtime, IM, connector, and storage
  boundaries.
- Error handling that prevents data loss or leaves audit state behind.
- UI `Suspense` fallback views and preview fallbacks that keep the interface
  usable while a heavy parser loads.
- Type coercion at LLM/tool argument boundaries when the caller is not a typed
  program.
- Compatibility paths with a concrete migration window and a deletion trigger.

## Findings

### 1. delete: Responses parser silently falls back to Chat Completions

Location: `internal/service/llm/client.go:206`

Status: landed in this pass.

`parseTextResponse(APIFormatResponses, body)` first parses a Responses payload,
then silently tries the Chat Completions shape if no text is found. A provider
configured as Responses should return Responses. Accepting a different shape
makes endpoint/API-format mistakes look successful.

Replacement:

- Return an explicit empty-response error when a Responses payload parses but
  has no text.
- Keep the existing Chat Completions branch for `APIFormatChatCompletions`.
- Add one focused test for a Responses response with no text.

Current evidence:

- Existing tests cover normal Anthropic, Chat Completions, and Responses calls.
- No test asserts that a Responses endpoint may return Chat Completions.

Net: removed the silent fallback and added a focused regression test.

### 2. delete: `openclaw-weixin` session hiding is now covered by migration

Status: landed in this pass for local/dev data.

Location: `internal/service/session/service_util.go:51`

`shouldHideWorkspaceSession` still hides workspace sessions whose channel is
`openclaw-weixin`. The repo already has migration `00039_im_channels_rename_openclaw_weixin`
for SQLite and Postgres, and current sample data has no `openclaw-weixin`
matches.

Replacement:

- Delete `isLegacyWorkspaceSessionChannel`.
- Delete the old-session branch from `shouldHideWorkspaceSession`.
- Update the session test that manufactures `openclaw-weixin`.

Current evidence:

- `rg openclaw-weixin deploy/data ~/.nexus` found no local/dev data matches.
- After this change, `rg openclaw-weixin internal/service/session
  internal/service/channels internal/protocol` has no code matches.

Net: removed one branch, one helper, and one legacy test fixture.

### 3. migrate-then-delete: accountless IM pairing fallback

Status: deferred; current local/dev data is clean, supported-install migration
proof still missing.

Location: `internal/service/channels/service_pairing_ingress.go:99`

`findIngressPairingByTarget` retries with `account_id=''` for old pairings
created before `00042_im_account_scoped_pairings`. This is real migration
compatibility, not random fallback, but it has no deletion condition.

Replacement:

- Add a migration or maintenance check that proves no active non-personal IM
  pairings have empty `account_id`.
- After that check is true for supported installs, delete
  `usesAccountlessPairingFallback` and the empty-account retry.

Current evidence:

- `00042_im_account_scoped_pairings` added `account_id` with default `''`.
- Current `deploy/data/.nexus/data/nexus.db` has no empty-account pairings:
  `feishu|1|0`, `weixin-personal|1|0`.
- Current `~/.nexus/data/nexus.db` has no empty-account pairings:
  `weixin-personal|2|0`.

Net after migration: one fallback function and two extra lookup branches.

### 4. delete-or-gate: legacy external skill registry migration runs on load

Location: `internal/service/skills/service_registry.go:19`

`loadExternalRecords` calls `ensureLegacyRegistryMigrated` every time. The
function is a one-time migration, already marked with
`TODO(skill-legacy-registry)`, but it remains on the hot catalog load path.

Replacement:

- Preferred: if old global registry support is no longer needed, delete
  `ensureLegacyRegistryMigrated` and its helpers.
- Conservative: run it only behind an explicit sentinel or maintenance path,
  not every catalog read.

Current evidence:

- Current `cache/skills/registry` only contains `users/__system__`; no top-level
  legacy skill directories were found.
- This does not prove user installs are clean, so remove only after a product
  migration decision.

Net if deleted: about 120 lines of migration-only code.

### 5. delete-after-data-check: legacy runtime emotion JSON shape

Status: landed in this pass for local/dev data.

Location: `internal/service/agent/emotion_state.go:86`

`loadRuntimeEmotionState` supports an old flat JSON shape via
`runtimeEmotionLegacyState`. Current sample workspaces only have empty
`emotion.json` files, so this compatibility path is not exercised locally.

Replacement:

- Delete `runtimeEmotionLegacyState` and the second `json.Unmarshal` branch.

Current evidence:

- Repo sample data has two `emotion.json` files; both are zero-byte init files.
- Local `~/.nexus/workspace` has three `emotion.json` files; all are zero-byte
  init files.
- `TestLoadRuntimeEmotionViewIgnoresLegacyEmotionShape` now asserts old flat
  JSON falls back to the default emotion state instead of being migrated in
  memory.

Net: deleted one legacy struct and one fallback parse branch.

### 6. shrink: git clone fallback to hard-coded `master`

Status: landed in this pass.

Location: `internal/service/skills/marketplace_git.go:62`

`runGitCloneAttempt` resolves the default branch with `git ls-remote --symref`.
If that produces no branch, it first clones without `--branch`, then on failure
retries hard-coded `master`. Git already knows how to use the remote HEAD when
no branch is supplied; the extra `master` retry is an old convention fallback.

Replacement:

- Return the original clone error after the no-branch clone fails.
- Keep explicit branch behavior unchanged.

Current evidence:

- Existing skills.sh test asserts that default branch resolution uses `main`.
- `TestRunGitCloneAttemptDoesNotFallbackToMaster` now asserts that a missing
  remote HEAD runs exactly one clone without forcing `master`.

Net: removed the hard-coded `master` retry and locked the simpler behavior with
one focused regression test.

### 7. shrink: unknown runtime content is stringified into prompt context

Status: landed in this pass.

Location: `internal/runtime/contextual_input.go:157`

`prependContextualInputBlocks` handles string and known block-list shapes, then
falls back to `fmt.Sprint(value)`. That can turn an unsupported caller shape
into model-visible text instead of surfacing a caller bug.

Replacement:

- For unsupported content types, keep the original payload unchanged and let the
  SDK/client validation decide.
- Keep string, `[]map[string]any`, and `[]any` support.

Current evidence:

- `TestExecuteRoundLeavesUnknownContentShapeWhenInternalContextUnsupported`
  now asserts that unknown payload shapes are not stringified into prompt text.
- Unknown content types are internal misuse, not a trust-boundary input that
  benefits from loose coercion.

Net: deleted the stringification helper and kept one regression test.

### 8. shrink: agent JSON persistence fallback hides programmer errors

Status: landed in this pass.

Location: `internal/service/agent/factory_record.go:121`

`mustJSONString` accepted a fallback JSON string and returned it when
`json.Marshal` failed. Its callers pass slices and maps from API-decoded agent
options, which are marshalable. Falling back to `[]` or `{}` would silently drop
agent configuration on an internal misuse.

Replacement:

- Remove the fallback argument.
- Panic on marshal error so bad internal values fail loudly instead of writing
  default configuration.

Current evidence:

- Create/update agent paths exercise `mustJSONString` through
  `go test ./internal/service/agent`.
- `protocol.Options` fields used here are slices, strings, pointers, and
  `map[string]any` from JSON payloads.

Net: removed one fallback branch and ten fallback arguments.

### 9. delete: one-line inbound message migration wrapper

Status: landed in this pass.

Location: `internal/service/channels/message/migration.go:8`

`MigrateInbound` only delegated to `NormalizeInbound`. The name made the
normalization path look like a separate migration layer, but there was one
implementation and one production caller.

Replacement:

- Delete `MigrateInbound`.
- Call `NormalizeInbound` directly from `migrateIngressMessage`.
- Keep the actual envelope/fallback merge behavior in `NormalizeInbound`.

Current evidence:

- `rg MigrateInbound internal/service/channels -g '*.go'` has no matches after
  this change.
- `go test ./internal/service/channels ./internal/service/channels/message`
  covers the caller and the merge behavior.

Net: removed one exported wrapper and kept the existing behavior test.

### 10. delete: unused channel capability wrapper

Status: landed in this pass.

Location: `internal/service/channels/channel_capabilities.go:8`

`channelCapabilityMatrix` and `channelCapabilities` only delegated to
`internal/service/channels/management`. They had no callers, while the real
implementation is still used directly by the management catalog.

Replacement:

- Delete the wrapper file.
- Keep `ChannelCapabilityMatrix` and `ChannelCapabilities` in `management`.

Current evidence:

- `rg "channelCapabilityMatrix\\(|channelCapabilities\\("
  internal/service/channels -g '*.go'` only matched the deleted definitions.
- `go test ./internal/service/channels ./internal/service/channels/management`
  covers the public implementation that remains.

Net: removed one dead file and two unused forwarding functions.

### 11. delete: room stderr normalization wrapper

Status: landed in this pass.

Location: `internal/service/room/runtime_stderr.go:5`

`normalizeRuntimeStderrLine` only called
`runtime.NormalizeRuntimeStderrLine`. Its only production caller already
imports the runtime package, and the room-package tests duplicated the runtime
package tests.

Replacement:

- Call `runtimectx.NormalizeRuntimeStderrLine` directly from room diagnostics.
- Keep the existing runtime-package tests as the behavior check.

Current evidence:

- `rg "normalizeRuntimeStderrLine|NormalizeRuntimeStderrLine"
  internal/service/room internal/runtime -g '*.go'` showed one room caller and
  duplicate room/runtime tests before deletion.
- `go test ./internal/service/room ./internal/runtime` covers the direct caller
  and the canonical normalization tests.

Net: removed one wrapper file and one duplicate test file.

### 12. delete: delivery ID wrapper

Status: landed in this pass.

Location: `internal/service/channels/channel_util.go` (deleted)

`newDeliveryID` delegated to `contract.NewID`. Its callers only used it as the
default value for injectable ID factories; channel adapters that need generated
IDs already call `channelcontract.NewID` directly.

Replacement:

- Delete `newDeliveryID`.
- Use `channelcontract.NewID` as the default `idFactory`.
- Keep the remaining channel utility functions because they still have service
  package callers.

Current evidence:

- `rg "newDeliveryID" internal/service/channels -g '*.go'` showed three default
  factory assignments and the deleted wrapper.
- `go test ./internal/service/channels` covers the package after the helper
  deletion.

Net: removed one one-line helper while preserving injectable ID factories.

### 13. delete: unused runtime query wrapper

Status: landed in this pass.

Location: `internal/runtime/manager_streaming_input.go:78`

`QueryClientContent` only called `QueryClientContentWithOptions` with empty
options and had no callers. Runtime execution already uses the options-aware
entrypoint.

Replacement:

- Delete `QueryClientContent`.
- Keep `QueryClientContentWithOptions`.

Current evidence:

- `rg "QueryClientContent\\(" internal -g '*.go'` only matched the deleted
  definition.
- `go test ./internal/runtime` covers the streaming input helpers.

Net: removed one unused exported wrapper.

### 14. delete: unused no-reply stream helper

Status: landed in this pass.

Location: `internal/service/room/no_reply.go:14`

`shouldHoldNoReplyCandidateStream` duplicated the no-reply readiness check but
had no callers. The active event path uses `roomEventsReadyForEmission`.

Replacement:

- Delete `shouldHoldNoReplyCandidateStream`.
- Keep `roomEventsReadyForEmission`, which still has one room execution caller.

Current evidence:

- `rg "shouldHoldNoReplyCandidateStream" internal/service/room -g '*.go'`
  only matched the deleted definition.
- `go test ./internal/service/room` covers the room execution package.

Net: removed one dead helper.

### 15. delete: unused sorted channel type helpers

Status: landed in this pass.

Location: `internal/service/channels/management/catalog.go:138`

`SortedChannelTypes` and the local `sortedChannelTypes` wrapper had no callers.
They also kept a `sort` import alive only for dead code.

Replacement:

- Delete both helpers.
- Keep `ChannelCatalog`, `ChannelCatalogByType`, and `IsPlannedChannel`, which
  are still used by channel configuration and tests.

Current evidence:

- `rg "sortedChannelTypes|SortedChannelTypes" internal -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/channels ./internal/service/channels/management`
  covers the remaining catalog API.

Net: removed two dead helpers and one unused import.

### 16. delete: delivery result wrapper

Status: landed in this pass.

Location: `internal/service/channels/model_channel.go:47`

`newDeliveryResult` only delegated to `channelcontract.NewDeliveryResult`.
The production caller already imports `channelcontract`; tests can call the
same constructor directly.

Replacement:

- Delete `newDeliveryResult`.
- Call `channelcontract.NewDeliveryResult` at the existing call sites.

Current evidence:

- `rg "newDeliveryResult" internal/service/channels -g '*.go'` has no matches
  after deletion.
- `go test ./internal/service/channels` covers the delivery result call sites.

Net: removed one one-line wrapper.

### 17. delete: HTTP status formatting wrapper

Status: landed in this pass.

Location: `internal/handler/shared/api.go:161`

`FormatStatusCode` only wrapped `strconv.Itoa(status)` and was called once in
the same file. `Itoa` never returns whitespace, so `strings.TrimSpace` did
nothing.

Replacement:

- Use `strconv.Itoa(status)` directly in `WriteFailure`.
- Delete `FormatStatusCode`.

Current evidence:

- `rg "FormatStatusCode" internal/handler/shared -g '*.go'` has no matches
  after deletion.
- `go test ./internal/handler/shared` covers the shared handler package.

Net: removed one wrapper and one redundant trim.

### 18. delete: scheduled-task permission test wrapper

Status: landed in this pass.

Location: `internal/service/automation/permission_scheduled.go:24`

`scheduledTaskPermissionHandler(options)` only delegated to
`scheduledTaskPermissionHandlerForOptions(options, false)` and had no
production callers. The one test can call the options-aware helper directly.

Replacement:

- Delete the no-service wrapper.
- Keep the service method and `scheduledTaskPermissionHandlerForOptions`.

Current evidence:

- `rg "scheduledTaskPermissionHandler\\(" internal/service/automation -g '*.go'`
  only matches the service method after deletion.
- `go test ./internal/service/automation` covers scheduled task permission
  behavior.

Net: removed one test-only wrapper.

### 19. delete: channel config row scanner wrapper

Status: landed in this pass.

Location: `internal/service/channels/service_channel_config_store.go:116`

`scanChannelConfigRow` only delegated to `scanChannelConfigScanner`; the
multi-row path already called the scanner directly.

Replacement:

- Call `scanChannelConfigScanner` from the single-row lookup.
- Delete `scanChannelConfigRow`.

Current evidence:

- `rg "scanChannelConfigRow\\(" internal/service/channels -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/channels` covers channel config store behavior.

Net: removed one redundant scanner wrapper.

### 20. delete: skills command runner wrapper

Status: landed in this pass.

Location: `internal/service/skills/marketplace_git.go:13`

`runCommand` only delegated to `runCommandWithEnv` with a nil environment. The
two callers can pass nil directly.

Replacement:

- Delete `runCommand`.
- Call `runCommandWithEnv(ctx, dir, nil, ...)` at the two rev-parse call sites.

Current evidence:

- `rg "runCommand\\(" internal/service/skills -g '*.go'` has no matches after
  deletion.
- `go test ./internal/service/skills` covers marketplace import behavior.

Net: removed one command-runner wrapper.

### 21. delete: task create event detail wrapper

Status: landed in this pass.

Location: `internal/service/automation/service_task_event.go:83`

`createTaskEventDetail` only returned `taskEventJobSnapshot(job)` and had one
caller.

Replacement:

- Call `taskEventJobSnapshot` directly when recording task creation.
- Delete `createTaskEventDetail`.

Current evidence:

- `rg "createTaskEventDetail" internal/service/automation -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/automation` covers task event behavior.

Net: removed one event-detail wrapper.

### 22. delete: shared error string wrapper

Status: landed in this pass.

Location: `internal/handler/shared/api.go:161`

`ErrString` only guarded `err.Error()` for nil. Its callers were doing error
classification and can check `err != nil` directly.

Replacement:

- Replace `handlershared.ErrString(err)` checks with `err != nil &&
  strings.Contains(strings.ToLower(err.Error()), ...)`.
- Delete `ErrString`.

Current evidence:

- `rg "ErrString\\(" internal -g '*.go'` has no matches after deletion.
- `go test ./internal/handler/connector ./internal/handler/skill
  ./internal/handler/shared` covers the touched packages.

Net: removed one nil-defensive helper.

### 23. delete: unused default-owner router wrappers

Status: landed in this pass.

Location: `internal/service/channels/router_registry.go:30`

`RegisterAndStart` had no callers, and `Get` was only used by a test. Runtime
channel configuration already uses the owner-aware methods.

Replacement:

- Delete `RegisterAndStart`.
- Delete `Get`.
- Update the router test to call `GetForOwner("", channelType)` directly.

Current evidence:

- `rg "RegisterAndStart\\(" internal -g '*.go'` has no matches after deletion.
- `rg "\\.Get\\(" internal/service/channels -g '*.go'` has no router wrapper
  calls after deletion.
- `go test ./internal/service/channels` covers router registration behavior.

Net: removed two default-owner wrappers.

### 24. delete: default-owner router register wrapper

Status: landed in this pass.

Location: `internal/service/channels/router_registry.go:10`

`Register` only delegated to `RegisterForOwner("", channel)` and was used only
inside `NewRouter`.

Replacement:

- Delete `Register`.
- Call `RegisterForOwner("", ...)` directly for built-in default-owner
  channels.

Current evidence:

- `rg "\\.Register\\(" internal/service/channels -g '*.go'` has no matches
  after deletion.
- `go test ./internal/service/channels` covers router initialization and
  registration behavior.

Net: removed one default-owner wrapper.

### 25. delete: nxs runtime status mapping wrapper

Status: landed in this pass.

Location: `internal/service/nxsruntime/service.go:35`

`runtimeStatusFromBridge` was called once, and its `err` branch was unreachable
because the bridge inspector exposes status, not an error return.

Replacement:

- Build `RuntimeStatus` directly in `Status`.
- Keep the product-specific message switch, without the unused `err` parameter.

Current evidence:

- `rg "runtimeStatusFromBridge|runtimeStatusMessage" internal/service/nxsruntime
  -g '*.go'` shows only the remaining message helper.
- `go test ./internal/service/nxsruntime` covers the status mapping and
  missing-runtime message.

Net: removed one single-use mapper and one unreachable fallback branch.

### 26. delete: automation delivery delegate

Status: landed in this pass.

Location: `internal/service/automation/runtime_delivery.go:61`

`deliverChannelMessage` only called `delivery.DeliverMessage` and had two
callers that already held `s.delivery`.

Replacement:

- Call `s.delivery.DeliverMessage(...)` directly at the two delivery sites.
- Delete the helper.

Current evidence:

- `rg "deliverChannelMessage\\(" internal/service/automation -g '*.go'` has
  no matches after deletion.
- `go test ./internal/service/automation` covers job and heartbeat delivery
  behavior.

Net: removed one delegate function.

### 27. delete: session delivery history delegate

Status: landed in this pass.

Location: `internal/service/channels/channel_session_delivery.go:230`

`appendHistoryMessage` had one caller and only forwarded to
`c.history.AppendOverlayMessage`.

Replacement:

- Call `AppendOverlayMessage` directly inside `persistMessage`.
- Delete the helper.

Current evidence:

- `rg "appendHistoryMessage\\(" internal/service/channels -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/channels` covers session delivery channel
  behavior through the channel package tests.

Net: removed one single-use history delegate.

### 28. delete: DM synthetic history delegate

Status: landed in this pass.

Location: `internal/service/dm/service_history.go:104`

`appendSyntheticHistoryMessage` only forwarded to
`s.history.AppendOverlayMessage`; the runtime-history helper remains because it
skips transcript-native messages.

Replacement:

- Call `AppendOverlayMessage` directly for synthetic result messages.
- Delete the synthetic helper.

Current evidence:

- `rg "appendSyntheticHistoryMessage\\(" internal/service/dm -g '*.go'` has
  no matches after deletion.
- `go test ./internal/service/dm` covers DM round/history behavior.

Net: removed one DM history delegate.

### 29. delete: agent prompt default-context wrapper

Status: landed in this pass.

Location: `internal/service/agent/service_prompt.go:18`

`BuildRuntimeUserMessageSuffix` only delegated to
`BuildRuntimeUserMessageSuffixForContext(ctx, agent, "")` and had no production
callers.

Replacement:

- Use `BuildRuntimeUserMessageSuffixForContext(..., "")` directly in the test.
- Delete the default-context wrapper.

Current evidence:

- `rg "BuildRuntimeUserMessageSuffix\\(" internal -g '*.go'` has no matches
  after deletion.
- `go test ./internal/service/agent` covers prompt suffix behavior.

Net: removed one unused default-argument wrapper.

### 30. delete: unused desktop session token wrapper

Status: landed in this pass.

Location: `internal/handler/shared/middleware.go:315`

`validDesktopSessionToken` had no callers; middleware already uses
`validateDesktopSessionToken` directly so it can log the token source and
failure reason.

Replacement:

- Delete the unused bool-only wrapper.

Current evidence:

- `rg "validDesktopSessionToken\\(" internal/handler/shared -g '*.go'` has no
  matches after deletion.
- `go test ./internal/handler/shared` covers desktop session token validation.

Net: removed one dead wrapper.

### 31. delete: single-use handler owner wrappers

Status: landed in this pass.

Location: `internal/handler/agent/handlers.go:134`,
`internal/handler/auth/profile.go:195`

Two `currentOwnerUserID` helpers had one caller each and only forwarded to
`authsvc.OwnerUserID`.

Replacement:

- Call `authsvc.OwnerUserID(...)` at the two call sites.
- Leave the multi-call channel/connector helpers alone for now; expanding
  those would add noise instead of removing it.

Current evidence:

- `rg "func currentOwnerUserID|currentOwnerUserID\\(" internal/handler/agent
  internal/handler/auth -g '*.go'` has no matches after deletion.
- `go test ./internal/handler/agent ./internal/handler/auth` covers the touched
  packages.

Net: removed two single-use owner wrappers.

### 32. shrink: Feishu callback security wrappers

Status: landed in this pass.

Location: `internal/service/channels/adapters/feishu_callback_security.go:24`

The Feishu callback security file kept private implementations plus exported
one-line wrappers. The exported names are the cross-package API; the private
layer added nothing.

Replacement:

- Move the implementations onto the exported functions.
- Update same-package callers/tests to use those exported functions.
- Delete the unused raw-envelope `DecryptFeishuCallback` helper.

Current evidence:

- `rg "verifyFeishuCallbackSignature|verifyFeishuCallbackToken|feishuEncryptEnvelope|decryptFeishuCallback|decryptFeishuEncryptedPayload"
  internal/service/channels/adapters -g '*.go'` has no matches after deletion.
- `go test ./internal/service/channels ./internal/service/channels/adapters`
  covers Feishu ingress preparation and callback security.

Net: removed five private implementation wrappers and one unused helper.

### 33. delete: single-use automation owner wrapper

Status: landed in this pass.

Location: `internal/service/automation/service_task_support.go:12`

`effectiveOwnerUserID` had one caller and only forwarded to
`authctx.OwnerUserID`.

Replacement:

- Call `authctx.OwnerUserID(ctx)` directly in `resolveTaskOwnerUserID`.

Current evidence:

- `rg "effectiveOwnerUserID\\(" internal/service/automation -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/automation` covers automation task ownership.

Net: removed one single-use owner wrapper.

### 34. delete: empty default tools wrapper

Status: landed in this pass.

Location: `internal/service/preferences/model_preferences.go:39`

`DefaultAllowedTools` returned only `[]string{}` and had one caller.

Replacement:

- Inline `[]string{}` in `DefaultPreferences`.

Current evidence:

- `rg "DefaultAllowedTools\\(" internal -g '*.go'` has no matches after
  deletion.
- `go test ./internal/service/preferences` covers default preferences.

Net: removed one speculative default function.

### 35. delete: preferences runtime-kind wrapper

Status: landed in this pass.

Location: `internal/service/preferences/model_preferences.go:57`

`normalizeRuntimeKind` had one caller and only forwarded to
`runtimeprovider.NormalizeRuntimeKind`.

Replacement:

- Call `runtimeprovider.NormalizeRuntimeKind` directly.

Current evidence:

- `rg "normalizeRuntimeKind\\(" internal/service/preferences -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/preferences` covers preference normalization.

Net: removed one single-use normalization wrapper.

### 36. delete: single-use resume retry wrappers

Status: landed in this pass.

Location: `internal/service/dm/service_runtime_client.go:112`,
`internal/service/room/execution.go:262`

`shouldRetryDMClientWithoutResume` and `shouldRetryRoomClientWithoutResume`
were each called once and only combined a non-empty resume ID check with
`runtimectx.IsRuntimeTransportClosedError`.

Replacement:

- Inline the condition in the retry branches.

Current evidence:

- `rg "shouldRetryDMClientWithoutResume|shouldRetryRoomClientWithoutResume"
  internal/service/dm internal/service/room -g '*.go'` has no matches after
  deletion.
- `go test ./internal/service/dm ./internal/service/room` covers stale resume
  retry behavior in DM and Room.

Net: removed two single-use retry wrappers.

### 37. delete: single-use runtime session prefix wrapper

Status: landed in this pass.

Location: `internal/runtime/manager_round.go:84`

`sessionBelongsToAgent` had one caller and only wrapped `strings.HasPrefix`.

Replacement:

- Use `strings.HasPrefix(sessionKey, "agent:"+agentID+":")` directly in
  `CountRunningRounds`.

Current evidence:

- `rg "sessionBelongsToAgent\\(" internal/runtime -g '*.go'` has no matches
  after deletion.
- `go test ./internal/runtime` covers runtime manager behavior.

Net: removed one single-use prefix wrapper.

### 38. delete: ingress managed-tool sub-wrappers

Status: landed in this pass.

Location: `internal/service/channels/ingress_permission.go:103`

`isManagedScheduledTaskTool` and `isManagedGoalTool` were only called by
`isManagedIngressTool` and only forwarded to `toolpolicy`.

Replacement:

- Keep `isManagedIngressTool` as the domain predicate.
- Inline the two forwarded checks inside it.

Current evidence:

- `rg "isManagedScheduledTaskTool|isManagedGoalTool" internal/service/channels
  -g '*.go'` has no matches after deletion.
- `go test ./internal/service/channels` covers ingress permission behavior.

Net: removed two sub-wrappers.

### 39. delete: goal created-by skip wrapper

Status: landed in this pass.

Location: `internal/service/goal/service_objective.go:21`

`skipObjectiveRewriteForCreatedBy` had one caller and only checked whether
`created_by` was `model`.

Replacement:

- Inline `strings.TrimSpace(request.CreatedBy) == "model"` in the create
  objective rewrite path.

Current evidence:

- `rg "skipObjectiveRewriteForCreatedBy\\(" internal/service/goal -g '*.go'`
  has no matches after deletion.
- `go test ./internal/service/goal` covers objective rewrite behavior.

Net: removed one single-use goal wrapper.

### 40. delete: scheduler recovery reason wrapper

Status: landed in this pass.

Location: `internal/service/automation/service_scheduler.go:16`

`recoverInterruptedJobRuntime` had one caller and only passed the scheduler
restart reason into `recoverJobRuntimeAsCancelled`.

Replacement:

- Call `recoverJobRuntimeAsCancelled` directly from scheduler bootstrap.

Current evidence:

- `rg "recoverInterruptedJobRuntime\\(" internal/service/automation -g '*.go'`
  has no matches after deletion.
- `go test ./internal/service/automation` covers scheduler recovery behavior.

Net: removed one fixed-reason wrapper.

### 41. delete: goal accounting source wrapper

Status: landed in this pass.

Location: `internal/service/goal/service_transition.go:79`

`shouldClearAccountingAfterMutation` only checked
`source != protocol.GoalUpdateSourceModel` and had two nearby callers.

Replacement:

- Inline the source check in the two transition branches.

Current evidence:

- `rg "shouldClearAccountingAfterMutation\\(" internal/service/goal -g '*.go'`
  has no matches after deletion.
- `go test ./internal/service/goal` covers goal runtime accounting behavior.

Net: removed one single-condition wrapper.

### 42. delete: titlegen request predicate wrappers

Status: landed in this pass.

Location: `internal/service/conversation/titlegen/service.go:81`

`Request.hasTarget` duplicated `targetKey() != ""`, and
`Request.shouldGenerateTitle` only OR-ed two existing predicates that are still
used by generation.

Replacement:

- Compute `targetKey` once and use it for the missing-target check.
- Inline the OR at the scheduler gate.

Current evidence:

- `rg "hasTarget\\(|shouldGenerateTitle\\(" internal/service/conversation/titlegen
  -g '*.go'` has no matches after deletion.
- `go test ./internal/service/conversation/titlegen` covers title scheduling
  behavior.

Net: removed two Request predicate wrappers.

### 43. delete: room no-reply event wrapper

Status: landed in this pass.

Location: `internal/service/room/execution.go:406`

`roomEventsReadyForEmission` had one caller and only nil-guarded a call to the
current slot's `eventsReadyForEmission`.

Replacement:

- Call `slot.eventsReadyForEmission(event)` directly from the room execution
  event emitter.
- Delete `no_reply.go`.

Current evidence:

- `rg "roomEventsReadyForEmission\\(" internal/service/room -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/room` covers no-reply event behavior.

Net: removed one single-use event wrapper file.

### 44. delete: private-domain visibility wrapper

Status: landed in this pass.

Location: `internal/service/room/private_domain.go:218`

`privateDomainMessageVisible` had one caller and only checked that
`SourceAgentID` and `MessageID` were present.

Replacement:

- Inline the two required-field checks in `buildPrivateDomainEvent`.

Current evidence:

- `rg "privateDomainMessageVisible\\(" internal/service/room -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/room` covers private-domain projection.

Net: removed one single-use visibility wrapper.

### 45. delete: memory section max-index wrapper

Status: landed in this pass.

Location: `internal/workspace/memory/repository_entry.go:132`

`maxIndex` had one caller and only selected `nextSection` when present,
otherwise `len(normalized)`.

Replacement:

- Keep the section end index as a local variable near the slice.
- Delete the generic fallback helper.

Current evidence:

- `rg "maxIndex\\(" internal/workspace/memory -g '*.go'` has no matches after
  deletion.
- `go test ./internal/workspace/memory` covers memory file editing behavior.

Net: removed one single-use fallback helper.

### 46. delete: automation session-label fallback wrapper

Status: landed in this pass.

Location: `internal/runtime/mcp/automation/internal/semantic/resolver.go:301`

`sessionLabelFallback` had one caller and only returned the fixed label
`当前对话` when a current session key existed.

Replacement:

- Resolve `sessionLabel` locally while building `Source`.
- Keep the existing non-empty `CurrentSessionKey` behavior.

Current evidence:

- `rg "sessionLabelFallback\\(" internal/runtime/mcp/automation -g '*.go'` has
  no matches after deletion.
- `go test ./internal/runtime/mcp/automation/internal/semantic` covers the
  default and explicit session-label behavior.

Net: removed one single-use fallback helper.

### 47. stdlib: memory root file existence wrapper

Status: landed in this pass.

Location: `internal/workspace/memory/repository_file.go:47`

`fileExists` had one caller and only wrapped `os.Stat(path) == nil`.

Replacement:

- Use `os.Stat` directly at the call site.

Current evidence:

- `rg "fileExists\\(" internal/workspace/memory -g '*.go'` has no matches after
  deletion.
- `go test ./internal/workspace/memory` covers memory file iteration.

Net: removed one stdlib wrapper.

### 48. delete: duplicated goal runtime ID wrappers

Status: landed in this pass.

Locations:

- `internal/service/dm/service_runtime_client.go:148`
- `internal/service/room/goal_runtime.go:76`

Both `goalIDForRuntimeUsage` helpers had one caller and only nil-guarded
`strings.TrimSpace(goal.ID)`.

Replacement:

- Keep the nil guard inline where the runtime goal context is resolved.

Current evidence:

- `rg "goalIDForRuntimeUsage\\(" internal/service/dm internal/service/room -g
  '*.go'` has no matches after deletion.
- `go test ./internal/service/dm ./internal/service/room` covers DM and Room
  goal-runtime paths.

Net: removed two duplicated single-use wrappers.

### 49. stdlib: memory checkpoint membership wrapper

Status: landed in this pass.

Location: `internal/workspace/memory/scheduler.go:46`

`roundIDProcessed` had one caller and only checked whether a non-empty round ID
was already present in the checkpoint list.

Replacement:

- Use `slices.Contains` at the scheduler decision point.

Current evidence:

- `rg "roundIDProcessed\\(" internal/workspace/memory -g '*.go'` has no matches
  after deletion.
- `go test ./internal/workspace/memory` covers checkpoint scheduling behavior.

Net: removed one hand-rolled membership wrapper.

### 50. stdlib: directed-message wake target membership

Status: landed in this pass.

Location: `internal/service/room/directed_message_wake.go:130`

`containsRoomDirectedMessageWakeTarget` had one caller and only checked whether
the already-normalized wake target list contained a value.

Replacement:

- Use `slices.Contains` at the dedupe point.

Current evidence:

- `rg "containsRoomDirectedMessageWakeTarget\\(" internal/service/room -g
  '*.go'` has no matches after deletion.
- `go test ./internal/service/room` covers directed-message wake behavior.

Net: removed one hand-rolled membership wrapper.

### 51. stdlib: private-domain participant membership

Status: landed in this pass.

Location: `internal/service/room/private_domain.go:215`

`containsPrivateDomainAgent` had one caller and only checked whether the
normalized participant ID list contained the current agent.

Replacement:

- Use `slices.Contains` at the visibility gate.

Current evidence:

- `rg "containsPrivateDomainAgent\\(" internal/service/room -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/room` covers private-domain projection.

Net: removed one hand-rolled membership wrapper.

### 52. stdlib: room MCP string-list membership

Status: landed in this pass.

Location: `internal/runtime/mcp/room/tool/arg.go:90`

`containsString` had one caller and only checked whether the normalized room
MCP string-list accumulator already contained a value.

Replacement:

- Use `slices.Contains` inside `normalizeStrings`.

Current evidence:

- `rg "containsString\\(" internal/runtime/mcp/room -g '*.go'` has no matches
  after deletion.
- `go test ./internal/runtime/mcp/room` covers room MCP tool wiring.

Net: removed one hand-rolled membership wrapper.

### 53. stdlib: channel connected account wrapper

Status: landed in this pass.

Location: `internal/service/channels/service_channel_config.go:99`

`hasConnectedChannelAccount` had one caller and only scanned account rows for
`ChannelConfigStatusConnected`.

Replacement:

- Use `slices.ContainsFunc` at the connected-channel count gate.

Current evidence:

- `rg "hasConnectedChannelAccount\\(" internal/service/channels -g '*.go'` has
  no matches after deletion.
- `go test ./internal/service/channels` covers channel count/status behavior.

Net: removed one single-use scanner wrapper.

### 54. delete: automation interactive-source wrapper

Status: landed in this pass.

Location: `internal/runtime/mcp/automation/internal/semantic/resolver.go:228`

`isInteractiveSourceContext` had one caller and only checked whether
`SourceContextType` was `room`.

Replacement:

- Check the trimmed source context type directly in `executionReply`.

Current evidence:

- `rg "isInteractiveSourceContext\\(" internal/runtime/mcp/automation/internal/semantic -g '*.go'`
  has no matches after deletion.
- `go test ./internal/runtime/mcp/automation/internal/semantic` covers the
  semantic resolver package.

Net: removed one single-use predicate wrapper.

### 55. delete: automation source-context snapshot wrapper

Status: landed in this pass.

Location: `internal/runtime/mcp/automation/internal/semantic/resolver.go:274`

`sourceContextTypeForSnapshot` had one caller and only mapped `room` to `room`,
with every other value becoming `agent`.

Replacement:

- Resolve `contextType` locally inside `Source`.

Current evidence:

- `rg "sourceContextTypeForSnapshot\\(" internal/runtime/mcp/automation/internal/semantic -g '*.go'`
  has no matches after deletion.
- `go test ./internal/runtime/mcp/automation/internal/semantic` covers the
  semantic resolver package.

Net: removed one single-use context wrapper.

### 56. stdlib: memory score min wrapper

Status: landed in this pass.

Location: `internal/workspace/memory/engine_scope_score.go:32`

`minFloat` had one caller and duplicated Go's built-in `min`.

Replacement:

- Use `min(float64(...), 0.12)` directly.

Current evidence:

- `rg "minFloat\\(" internal/workspace/memory -g '*.go'` has no matches after
  deletion.
- `go test ./internal/workspace/memory` covers memory scoring behavior.

Net: removed one hand-rolled builtin wrapper.

### 57. stdlib: integer max wrappers

Status: landed in this pass.

Locations:

- `internal/service/goal/runtime_usage_accumulator.go:65`
- `internal/infra/logx/writer_rolling.go:120`

`maxInt64` and `maxInt` duplicated Go's built-in `max`.

Replacement:

- Use `max(...)` directly at the call sites.

Current evidence:

- `go 1.26.2` in `go.mod` supports built-in `max`.
- `rg "maxInt64\\(|maxInt\\(" internal/service/goal internal/infra/logx -g '*.go'`
  has no matches after deletion.
- `go test ./internal/service/goal ./internal/infra/logx` covers the touched
  packages.

Net: removed two hand-rolled builtin wrappers.

### 58. stdlib: room string membership helper

Status: landed in this pass.

Locations:

- `internal/chat/room/records.go:131`
- `internal/service/room/service_host.go:31`
- `internal/service/room/directed_message.go:112`
- `internal/service/room/public_message.go:36`
- `internal/service/room/service_agent_resolution.go:35`

`roomdomain.ContainsString` wrapped plain string slice membership and had no
room-specific semantics.

Replacement:

- Use `slices.Contains(...)` directly at each call site.

Current evidence:

- `rg "ContainsString\\(" internal/chat/room internal/service/room -g '*.go'`
  has no matches after deletion.
- `go test ./internal/chat/room ./internal/service/room` covers the touched
  room domain and service packages.

Net: removed one exported helper wrapper.

### 59. stdlib: toolpolicy allowlist helper

Status: landed in this pass.

Location: `internal/service/toolpolicy/toolpolicy.go:210`

`isManagedImagegenAllowedTool` had one caller and wrapped string allowlist
membership.

Replacement:

- Use `slices.Contains(managedImagegenAllowedTools, strings.TrimSpace(tool))`
  directly in `withoutManagedImagegenAllowedTools`.

Current evidence:

- `rg "isManagedImagegenAllowedTool" internal/service/toolpolicy -g '*.go'`
  has no matches after deletion.
- `go test ./internal/service/toolpolicy` covers the touched package.

Net: removed one single-use allowlist wrapper.

### 60. stdlib: workspace path segment membership

Status: landed in this pass.

Location: `internal/service/workspace/service_path.go:70`

`hasWorkspacePathSegment` hand-rolled membership over the target segment list.

Replacement:

- Use `slices.Contains(targets, segment)` inside the existing path segment scan.

Current evidence:

- `go test ./internal/service/workspace` covers the touched package.

Net: replaced an inner loop with stdlib membership.

### 61. stdlib: permission event trim alias

Status: landed in this pass.

Location: `internal/runtime/permission/request.go:150`

`emptyStringToOmit` only returned `strings.TrimSpace(value)`; `omitempty` is
already handled by the event serialization layer.

Replacement:

- Use `strings.TrimSpace(...)` directly at permission event assignment sites.

Current evidence:

- `rg "emptyStringToOmit" internal/runtime/permission -g '*.go'` has no
  matches after deletion.
- `go test ./internal/runtime/permission` covers permission event behavior.

Net: removed one stdlib alias.

### 62. shrink: workspace history clone alias

Status: landed in this pass.

Location: `internal/storage/workspace/history_order.go:60`

`cloneMessage` only delegated to `protocol.Clone` and hid the real clone API.

Replacement:

- Use `protocol.Clone(...)` directly at the four call sites.

Current evidence:

- `rg "cloneMessage\\(" internal/storage/workspace -g '*.go'` has no matches
  after deletion.
- `go test ./internal/storage/workspace` covers history compaction and
  transcript projection behavior.

Net: removed one local delegate wrapper.

### 63. stdlib: runtime debug trim aliases

Status: landed in this pass.

Location: `internal/runtime/debug_message_value.go:89`

`messageDebugText` and `safeToolName` only returned `strings.TrimSpace(value)`.

Replacement:

- Use `strings.TrimSpace(...)` directly at debug field and summary call sites.

Current evidence:

- `rg "messageDebugText\\(|safeToolName\\(" internal/runtime -g '*.go'` has
  no matches after deletion.
- `go test ./internal/runtime` covers runtime debug message helpers.

Net: removed two stdlib alias helpers.

### 64. shrink: runtime system summary first-non-empty helper

Status: landed in this pass.

Location: `internal/runtime/summary_system_message.go:28`

`firstNonEmptySummary` duplicated the runtime package's existing
`firstNonEmpty` helper with an extra trim.

Replacement:

- Use `strings.TrimSpace(firstNonEmpty(...))` at the three summary call sites.

Current evidence:

- `rg "firstNonEmptySummary" internal/runtime -g '*.go'` has no matches after
  deletion.
- `go test ./internal/runtime` covers runtime system message summaries.

Net: removed one duplicate helper.

### 65. stdlib: session title default helper

Status: landed in this pass.

Location: `internal/service/session/service_mutation.go:48`

The session package `firstNonEmpty` helper only served two title defaulting call
sites, and both inputs were already trimmed.

Replacement:

- Use `cmp.Or(strings.TrimSpace(...), "New Chat")` directly.

Current evidence:

- `rg "firstNonEmpty\\(" internal/service/session -g '*.go'` has no matches
  after deletion.
- `go test ./internal/service/session` covers session create/update behavior.

Net: removed one local defaulting helper.

### 66. stdlib: runtime selection fallback helper

Status: landed in this pass.

Location: `internal/service/runtimeselection/service.go:68`

`firstNonEmpty` only served two provider/model fallback assignments.

Replacement:

- Use `cmp.Or(strings.TrimSpace(selection.Provider), agentProvider)` and the
  same form for model selection.

Current evidence:

- `rg "firstNonEmpty\\(" internal/service/runtimeselection -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/runtimeselection` covers runtime selection
  behavior.

Net: removed one local fallback helper.

### 67. stdlib: goal objective owner fallback helper

Status: landed in this pass.

Location: `internal/service/goalobjective/runtime_selection.go:123`

`firstNonEmpty` had one caller and only selected the first non-empty owner ID.

Replacement:

- Use `cmp.Or(strings.TrimSpace(ownerUserID), strings.TrimSpace(agentValue.OwnerUserID))`.

Current evidence:

- `rg "firstNonEmpty\\(" internal/service/goalobjective -g '*.go'` has no
  matches after deletion.
- `go test ./internal/service/goalobjective` covers goal objective runtime
  selection behavior.

Net: removed one single-use fallback helper.

### 68. stdlib: auth bearer query fallback helper

Status: landed in this pass.

Location: `internal/service/auth/service_principal.go:84`

The auth package `firstNonEmpty` helper had one caller for access-token query
fallbacks.

Replacement:

- Use `cmp.Or(strings.TrimSpace(query.Get("access_token")),
  strings.TrimSpace(query.Get("token")))`.

Current evidence:

- `rg "firstNonEmpty\\(" internal/service/auth -g '*.go'` has no matches
  after deletion.
- `go test ./internal/service/auth` covers auth principal behavior.

Net: removed one single-use fallback helper.

### 69. stdlib: connector shop fallback helper

Status: landed in this pass.

Location: `internal/runtime/mcp/connectors/tool/call.go:87`

The connector call tool `firstNonEmpty` helper had one caller for Shopify shop
fallback selection.

Replacement:

- Use `cmp.Or(strings.TrimSpace(shopDomain), strings.TrimSpace(extra["shop"]),
  strings.TrimSpace(extra["shop_domain"]))`.

Current evidence:

- `rg "firstNonEmpty\\(" internal/runtime/mcp/connectors/tool -g '*.go'` has
  no matches after deletion.
- `go test ./internal/runtime/mcp/connectors/tool` covers the touched package.

Net: removed one single-use fallback helper.

### 70. delete: dead transcript user-turn counter

Status: landed in this pass.

Location: `internal/storage/workspace/store_agent_history_transcript_marker.go:51`

`countTranscriptUserTurns` wrapped `len(collectTranscriptUserTurns(...))` and
had no callers.

Replacement:

- Nothing.

Current evidence:

- `rg "countTranscriptUserTurns" internal/storage/workspace -g '*.go'` has no
  matches after deletion.
- `go test ./internal/storage/workspace` covers transcript marker code.

Net: removed one dead helper.

### 71. stdlib: clamp upper-bound branches

Status: landed in this pass.

Locations:

- `internal/storage/workspace/history_pagination.go:24`
- `internal/service/automation/service_observability_util.go:160`
- `internal/service/room/private_domain.go:383`

Three limit helpers hand-rolled the same upper-bound `min` branch after
handling default values.

Replacement:

- Use Go's built-in `min(value, maxValue)` / `min(limit, maxMessageHistoryRoundPageSize)`.

Current evidence:

- `go 1.26.2` in `go.mod` supports built-in `min`.
- `go test ./internal/storage/workspace ./internal/service/automation ./internal/service/room`
  covers the touched packages.

Net: replaced three manual clamp tails with built-in `min`.

### 72. stdlib: negative runtime elapsed clamps

Status: landed in this pass.

Locations:

- `internal/service/dm/goal_runtime.go:278`
- `internal/service/goal/runtime_wall_clock.go:73`
- `internal/service/goal/runtime_usage_accumulator.go:110`
- `internal/service/room/goal_runtime.go:438`

Four helpers hand-rolled negative-to-zero clamps.

Replacement:

- Use Go's built-in `max(value, 0)` at the return sites.

Current evidence:

- `go 1.26.2` in `go.mod` supports built-in `max`.
- `go test ./internal/service/dm ./internal/service/goal ./internal/service/room`
  covers goal runtime usage accounting.

Net: replaced four manual lower-bound clamps with built-in `max`.

### 73. stdlib: goal usage saturating subtraction

Status: landed in this pass.

Location: `internal/service/goal/runtime_usage_accumulator.go:103`

`saturatingSub` hand-rolled a lower-bound clamp around subtraction.

Replacement:

- Keep the semantic helper name, but implement it as `max(current-previous, 0)`.

Current evidence:

- `go 1.26.2` in `go.mod` supports built-in `max`.
- `go test ./internal/service/goal` covers runtime usage accumulation.

Net: replaced one manual clamp branch with built-in `max`.

### 74. stdlib: agent emotion score clamps

Status: landed in this pass.

Location: `internal/service/agent/emotion_state.go:375`

Two score clamp helpers hand-rolled lower and upper bounds.

Replacement:

- Use `min(max(value, 0), limit)` directly in each helper.

Current evidence:

- `go 1.26.2` in `go.mod` supports built-in `min` and `max`.
- `go test ./internal/service/agent` covers agent service behavior.

Net: replaced two manual clamp blocks with built-ins.

### 75. stdlib: Feishu wiki page-size clamp

Status: landed in this pass.

Location: `internal/connectors/feishudocx/client_wiki.go:198`

`normalizePageSize` hand-rolled an upper-bound clamp after handling the
fallback value.

Replacement:

- Use `min(value, max)`.

Current evidence:

- `go 1.26.2` in `go.mod` supports built-in `min`.
- `go test ./internal/connectors/feishudocx` covers the touched package.

Net: replaced one manual clamp branch with built-in `min`.

### 76. stdlib: automation history tool limit clamp

Status: landed in this pass.

Location: `internal/runtime/mcp/automation/tool/history_context_query.go:216`

`normalizedTaskHistoryToolLimit` hand-rolled its upper-bound clamp.

Replacement:

- Use `min(limit, 50)` after the default branch.

Current evidence:

- `go 1.26.2` in `go.mod` supports built-in `min`.
- `go test ./internal/runtime/mcp/automation ./internal/runtime/mcp/automation/tool`
  covers the automation MCP packages.

Net: replaced one manual clamp branch with built-in `min`.

### 77. stdlib: runtime emotion context default

Status: landed in this pass.

Location: `internal/service/agent/emotion_state.go:362`

`normalizeRuntimeEmotionContextID` hand-rolled a trim-then-default string
fallback.

Replacement:

- Use `cmp.Or(strings.TrimSpace(contextID), defaultRuntimeEmotionContextID)`.

Current evidence:

- `go 1.26.2` in `go.mod` supports `cmp.Or`.
- `go test ./internal/service/agent` covers the touched package.

Net: replaced one manual default branch with the standard library helper.

### 78. stdlib: channel owner default

Status: landed in this pass.

Location: `internal/service/channels/service_control_values.go:53`

`normalizeChannelOwnerUserID` hand-rolled a trim-then-default string fallback.

Replacement:

- Use `cmp.Or(strings.TrimSpace(ownerUserID), authctx.SystemUserID)`.

Current evidence:

- `go 1.26.2` in `go.mod` supports `cmp.Or`.
- `go test ./internal/service/channels` covers the touched package.

Net: replaced one manual default branch with the standard library helper.

### 79. stdlib: auth cookie defaults

Status: landed in this pass.

Location: `internal/service/auth/service_cookie.go:49`

`cookieName` and `cookiePath` hand-rolled trim-then-default string fallbacks.

Replacement:

- Use `cmp.Or(strings.TrimSpace(...), default)`.

Current evidence:

- `go 1.26.2` in `go.mod` supports `cmp.Or`.
- `go test ./internal/service/auth` covers the touched package.

Net: replaced two manual default branches with standard library helpers.

### 80. stdlib: usage owner default

Status: landed in this pass.

Location: `internal/service/usage/service.go:107`

`normalizeOwnerUserID` hand-rolled a trim-then-default string fallback.

Replacement:

- Use `cmp.Or(strings.TrimSpace(ownerUserID), authctx.SystemUserID)`.

Current evidence:

- `go 1.26.2` in `go.mod` supports `cmp.Or`.
- `go test ./internal/service/usage` covers the touched package.

Net: replaced one manual default branch with the standard library helper.

### 81. stdlib: agent workspace dir default

Status: landed in this pass.

Location: `internal/service/agent/policy_name.go:42`

`BuildWorkspaceDirName` hand-rolled the final empty-name fallback after
normalizing and filtering characters.

Replacement:

- Use `cmp.Or(result, "agent")`.

Current evidence:

- `go 1.26.2` in `go.mod` supports `cmp.Or`.
- `go test ./internal/service/agent` covers the touched package.

Net: replaced one manual default branch with the standard library helper.

### 82. stdlib: runtime stderr decode fallback

Status: landed in this pass.

Location: `internal/runtime/stderr_line.go:21`

`NormalizeRuntimeStderrLine` hand-rolled the decoded-empty fallback after GBK
decode and trimming.

Replacement:

- Use `cmp.Or(decoded, trimmed)`.

Current evidence:

- `go 1.26.2` in `go.mod` supports `cmp.Or`.
- `go test ./internal/runtime` covers the touched package.

Net: replaced one manual default branch with the standard library helper.

### 83. stdlib: automation artifact segment fallback

Status: landed in this pass.

Location: `internal/service/automation/runtime_artifact.go:63`

`safeArtifactSegment` checked the empty input fallback before filtering, then
checked the empty filtered result again.

Replacement:

- Let the filter run over the trimmed value.
- Use `cmp.Or(result, fallback)` for the single fallback decision.

Current evidence:

- `go 1.26.2` in `go.mod` supports `cmp.Or`.
- `go test ./internal/service/automation` covers the touched package.

Net: removed one redundant empty-input branch and kept one final fallback.

### 84. stdlib: connector owner default

Status: landed in this pass.

Location: `internal/service/connectors/service_connection.go:303`

`normalizeConnectorOwnerUserID` hand-rolled a trim-then-current-owner fallback.

Replacement:

- Use `cmp.Or(strings.TrimSpace(ownerUserID), authctx.OwnerUserID(ctx))`.

Current evidence:

- `authctx.OwnerUserID` is a pure context read with a system-user fallback.
- `go 1.26.2` in `go.mod` supports `cmp.Or`.
- `go test ./internal/service/connectors` covers the touched package.

Net: replaced one manual default branch with the standard library helper.

### 85. delete: repeated channel status normalization

Status: landed in this pass.

Location: `internal/service/channels/service_control_values.go:40`

Three channel status/source helpers recalculated
`strings.ToLower(strings.TrimSpace(value))` inside both the `switch` expression
and the valid-value return path.

Replacement:

- Normalize once into `normalized`.
- Return that value from valid cases.

Current evidence:

- `go test ./internal/service/channels` covers the touched package.

Net: deleted repeated trim/lower calls without changing fallback behavior.

### 86. delete: repeated channel string trims

Status: landed in this pass.

Locations:

- `internal/service/channels/contract/util.go:13`
- `internal/service/channels/service_control_values.go:128`

Two channel string helpers recalculated `strings.TrimSpace` in both the empty
check and return path.

Replacement:

- Store the trimmed value once.
- Return that value from the non-empty path.

Current evidence:

- `go test ./internal/service/channels ./internal/service/channels/contract`
  covers/builds the touched packages.

Net: deleted repeated trim calls without changing nil/empty fallback behavior.

### 87. delete: repeated OAuth credential trims

Status: landed in this pass.

Location: `internal/service/connectors/service_oauth_config.go:133`

OAuth config helpers recalculated `strings.TrimSpace` in validation and return
paths; default credentials also returned the untrimmed values after validating
the trimmed values.

Replacement:

- Trim each credential once.
- Validate and return the normalized values.

Current evidence:

- `go test ./internal/service/connectors` covers the touched package.

Net: deleted repeated trim calls and aligned default OAuth credentials with the
user-configured credential path.

### 88. delete: repeated user OAuth credential trims

Status: landed in this pass.

Location: `internal/service/connectors/service_oauth_config.go:107`

The user OAuth credential path trimmed client ID and secret once for validation
and again for the return value.

Replacement:

- Keep the nil-client guard separate.
- Trim client ID and secret once, then validate and return those values.

Current evidence:

- `go test ./internal/service/connectors` covers the touched package.

Net: deleted repeated trim calls without changing missing-credential behavior.

### 89. delete: repeated image model selection trims

Status: landed in this pass.

Location: `internal/service/imagegen/service_config.go:12`

Image generation config resolution recalculated provider/model trims in
branching and then passed the original untrimmed values into provider
resolution.

Replacement:

- Trim explicit provider/model once at entry.
- Trim stored preference selection once before checking and resolving.

Current evidence:

- `go test ./internal/service/imagegen` covers the touched package.

Net: deleted repeated trim calls and resolved with normalized provider/model
values.

### 90. delete: repeated core default model trims

Status: landed in this pass.

Location: `internal/handler/core/handlers.go:83`

The core handler trimmed default provider/model once for the condition and
again for the response pointers.

Replacement:

- Trim provider/model once before the condition.
- Reuse those values for the response defaults.

Current evidence:

- `go test ./internal/handler/core` covers the touched package.

Net: deleted repeated trim calls without changing the response fallback.

### 91. delete: repeated goal objective model trims

Status: landed in this pass.

Location: `internal/service/goalobjective/service.go:116`

Goal objective runtime selection trimmed provider/model for checks, then passed
the original values into config resolution.

Replacement:

- Trim request provider/model once.
- Trim default background model selection once before resolving.

Current evidence:

- `go test ./internal/service/goalobjective` covers the touched package.

Net: deleted repeated trim calls and resolved with normalized provider/model
values.

### 92. delete: repeated titlegen model selection trims

Status: landed in this pass.

Location: `internal/service/conversation/titlegen/generation.go:193`

Title generation checked trimmed default background provider/model, then passed
the original preference values into config resolution.

Replacement:

- Trim the preference selection once.
- Resolve with the normalized values.

Current evidence:

- `go test ./internal/service/conversation/titlegen` covers the touched package.

Net: deleted repeated trim calls and resolved with normalized provider/model
values.

### 93. delete: repeated room SDK session trims

Status: landed in this pass.

Location: `internal/service/room/service_conversation_crud.go:132`

`UpdateSessionSDKSessionID` trimmed both IDs for the empty check, then trimmed
them again for the repository call.

Replacement:

- Trim each ID once.
- Reuse the normalized values for the check and repository call.

Current evidence:

- `go test ./internal/service/room` covers the touched package.

Net: deleted repeated trim calls without changing the empty-input no-op.

### 94. delete: task event ID/action trim-only check

Status: landed in this pass.

Location: `internal/service/automation/service_task_event.go:37`

`recordTaskEvent` trimmed job ID and action only for the guard, then persisted
the original values.

Replacement:

- Normalize job ID and action once.
- Use those values for the guard and emitted event.

Current evidence:

- `go test ./internal/service/automation` covers the touched package.

Net: removed a guard-only normalization path and persisted normalized event
identity fields.

### 95. delete: adapter credential re-trims

Status: landed in this pass.

Locations:

- `internal/service/channels/adapters/dingtalk_stream.go:16`
- `internal/service/channels/adapters/dingtalk_delivery.go:59`
- `internal/service/channels/adapters/feishu_channel.go:125`
- `internal/service/channels/adapters/feishu_token.go:21`
- `internal/service/channels/adapters/wecom_bot_channel.go:86`

Channel adapter constructors already trim credential fields into unexported
struct fields. Start/token paths re-trimmed those fields before checking them.

Replacement:

- Check the normalized fields directly.

Current evidence:

- `go test ./internal/service/channels/adapters` covers the touched package.

Net: deleted repeated credential trims without changing configured/unconfigured
behavior.

### 96. delete: QR login trim-only check

Status: landed in this pass.

Location: `internal/service/channels/service_login_flow.go:69`

The personal Weixin QR login path trimmed the QR token only for validation,
then stored the original token on the login session.

Replacement:

- Trim the QR token once.
- Use the normalized token for validation, payload fallback, and polling.

Current evidence:

- `go test ./internal/service/channels` covers the touched package.

Net: removed a guard-only normalization path and kept the session QR token
normalized.

### 97. delete: repeated room transcript path trims

Status: landed in this pass.

Location: `internal/storage/workspace/store_room_history.go:174`

Room transcript reference creation trimmed workspace path and private session
key for the empty guard, then trimmed them again for the stored overlay row.

Replacement:

- Trim each value once.
- Reuse the normalized values for the guard and overlay row.

Current evidence:

- `go test ./internal/storage/workspace` covers the touched package.

Net: deleted repeated trim calls without changing the empty-input no-op.

### 98. delete: repeated message helper trims

Status: landed in this pass.

Location: `internal/message/helpers.go:11`

`firstNonEmpty` and `emptyToNil` recalculated `strings.TrimSpace` in the check
and return path.

Replacement:

- Store the trimmed value once.
- Return the normalized value from the non-empty path.

Current evidence:

- `go test ./internal/message` covers the touched package.

Net: deleted repeated trim calls without changing empty-string handling.

### 99. delete: repeated storage nullable string trims

Status: landed in this pass.

Locations:

- `internal/storage/goal/value_sql.go:10`
- `internal/storage/auth/repository_value.go:9`
- `internal/storage/workspace/history_unfinished_round.go:166`
- `internal/storage/automation/value_sql.go:9`

Storage nullable-string helpers recalculated `strings.TrimSpace` in the nil
guard and return path.

Replacement:

- Trim once per helper.
- Return the normalized value from the non-empty path.

Current evidence:

- `go test ./internal/storage/goal ./internal/storage/auth ./internal/storage/workspace ./internal/storage/automation`
  covers/builds the touched packages.

Net: deleted repeated trim calls without changing nil/empty persistence
behavior.

### 100. delete: repeated memory scope trims

Status: landed in this pass.

Location: `internal/workspace/memory/model_engine.go:48`

`MemoryScope.inferredKey` repeatedly trimmed the same scope fields while trying
fallback scope shapes; `buildScopeKey` also trims parts.

Replacement:

- Trim each scope field once.
- Reuse those normalized values for inference and key construction.

Current evidence:

- `go test ./internal/workspace/memory` covers the touched package.

Net: deleted repeated trims without changing inferred scope precedence.

### 101. delete: repeated memory scope scoring trims

Status: landed in this pass.

Location: `internal/workspace/memory/engine_scope_score.go:39`

Memory scope scoring/access checks repeatedly trimmed the same agent, user, room
and conversation fields inside switch cases.

Replacement:

- Trim each needed scope field once per helper.
- Reuse the normalized values in comparisons.

Current evidence:

- `go test ./internal/workspace/memory` covers the touched package.

Net: deleted repeated trims without changing scope matching rules.

### 102. delete: repeated CLI skill JSON input trims

Status: landed in this pass.

Location: `internal/cli/command_skill_external.go:171`

`readOptionalJSONPayload` trimmed `--item-json` and `--item-file` at every
branch and again before reading the file.

Replacement:

- Trim both arguments once at entry.
- Reuse the normalized values for conflict checks and reads.

Current evidence:

- `go test ./internal/cli` covers the touched package.

Net: deleted repeated trim calls without changing CLI argument behavior.

### 103. delete: repeated private event identity trims

Status: landed in this pass.

Location: `internal/service/room/private_domain.go:219`

Private room event construction trimmed message/source agent IDs for the guard,
then trimmed them again for the event payload.

Replacement:

- Trim both IDs once.
- Reuse those values for validation and event construction.

Current evidence:

- `go test ./internal/service/room` covers the touched package.

Net: deleted repeated trim calls without changing private event validation.

### 104. delete: repeated GitHub device code trims

Status: landed in this pass.

Location: `internal/connectors/providers/github.go:94`

GitHub Device Flow response validation trimmed the same required fields only to
discard the normalized values.

Replacement:

- Normalize the response fields once after JSON decode.
- Reuse those fields for validation and the returned response.

Current evidence:

- `go test ./internal/connectors/providers` covers the touched package.

Net: deleted repeated trim calls while returning canonical device flow fields.

### 105. delete: repeated marketplace GitHub tree trims

Status: landed in this pass.

Location: `internal/service/skills/marketplace_external_url.go:28`

GitHub marketplace detail URL construction trimmed `gitURL` and `gitPath` for
the guard, then repeated the same normalization for URL assembly.

Replacement:

- Trim both inputs once at function entry.
- Reuse those values for the guard and tree URL construction.

Current evidence:

- `go test ./internal/service/skills` covers the touched package.

Net: deleted repeated trim calls without changing GitHub tree URL shape.

### 106. delete: repeated Feishu ingress identity trims

Status: landed in this pass.

Location: `internal/service/channels/service_feishu_ingress.go:60`

Feishu ingress matching trimmed callback/config identity fields after the
decoder and config loader had already normalized them.

Replacement:

- Reuse normalized callback `Challenge`/`AppID` values directly.
- Reuse pre-trimmed Feishu config `AppID` and verification token.

Current evidence:

- `go test ./internal/service/channels` covers the touched package.

Net: deleted redundant trim calls without weakening Feishu ingress matching.

### 107. delete: repeated session target key trims

Status: landed in this pass.

Location: `internal/protocol/model_automation_target.go:17`

Session target validation trimmed `bound_session_key` and `named_session_key`
at each branch even though the function only needs one normalized value for
each field.

Replacement:

- Trim both keys once at validation entry.
- Reuse those values for empty checks and structured-key validation.

Current evidence:

- `go test ./internal/protocol` covers the touched package.

Net: deleted repeated trim calls without weakening session target validation.

### 108. delete: repeated automation delivery target trims

Status: landed in this pass.

Location: `internal/service/automation/runtime_delivery.go:35`

Automation delivery skipped/not-required checks trimmed the same delivery mode,
channel and target values multiple times before deciding whether to deliver.

Replacement:

- Trim the delivery mode/channel/target once at function entry.
- Reuse those values for the not-required and websocket self-delivery checks.

Current evidence:

- `go test ./internal/service/automation` covers the touched package.

Net: deleted repeated trim calls without changing delivery routing behavior.

### 109. delete: repeated heartbeat target mode trims

Status: landed in this pass.

Location: `internal/service/automation/runtime_delivery.go:157`

Heartbeat delivery checked the target mode after trimming it, then trimmed the
same field again when building the delivery target.

Replacement:

- Trim the heartbeat target mode once.
- Reuse it for the guard and delivery target construction.

Current evidence:

- `go test ./internal/service/automation` covers the touched package.

Net: deleted repeated trim calls without changing heartbeat delivery behavior.

### 110. delete: repeated message envelope ID trims

Status: landed in this pass.

Location: `internal/message/processor.go:323`

Message envelope construction checked session, parent, room and conversation
IDs after trimming them, then trimmed the same values again for the payload.

Replacement:

- Trim envelope IDs once at helper entry.
- Reuse those normalized values for guards and payload fields.

Current evidence:

- `go test ./internal/message` covers the touched package.

Net: deleted repeated trim calls without changing message envelope fields.

### 111. delete: repeated assistant metadata trims

Status: landed in this pass.

Location: `internal/message/segment_assistant.go:101`

Assistant metadata updates trimmed model and stop reason values for the guard,
then trimmed the same values again before storing them.

Replacement:

- Trim model and stop reason once at update entry.
- Reuse those values for guards and assignments.

Current evidence:

- `go test ./internal/message` covers the touched package.

Net: deleted repeated trim calls without changing assistant metadata updates.

### 112. delete: repeated LLM system prompt trims

Status: landed in this pass.

Location: `internal/service/llm/client.go:133`

LLM request payload construction trimmed the system prompt before passing it
to `messagesWithSystem`, then the helper trimmed the same value again.

Replacement:

- Keep the single trim at request payload entry.
- Let the private helper reuse the already-normalized prompt.

Current evidence:

- `go test ./internal/service/llm` covers the touched package.

Net: deleted repeated trim calls without changing generated LLM requests.

### 113. delete: repeated result terminal reason trims

Status: landed in this pass.

Location: `internal/message/processor_result.go:10`

Result message projection trimmed `terminal_reason` for the guard, then
trimmed the same value again before writing it to the payload.

Replacement:

- Trim the terminal reason once.
- Reuse it for the guard and payload field.

Current evidence:

- `go test ./internal/message` covers the touched package.

Net: deleted repeated trim calls without changing result message projection.

### 114. delete: repeated room trigger trims

Status: landed in this pass.

Location: `internal/chat/room/visible_format.go:20`

Room visible trigger formatting trimmed the trigger type and content at each
branch while building one display string.

Replacement:

- Trim trigger type and content once at formatter entry.
- Reuse those values for branch checks and line construction.

Current evidence:

- `go test ./internal/chat/room` covers the touched package.

Net: deleted repeated trim calls without changing Room trigger text.

### 115. delete: repeated assistant API error trims

Status: landed in this pass.

Location: `internal/message/processor_api_error.go:10`

Assistant API error projection trimmed error fields for the guard, then passed
the raw fields back through trimming helpers for result text and terminal
reason.

Replacement:

- Trim assistant error fields once.
- Reuse those values for the guard, result text fallback and terminal reason.

Current evidence:

- `go test ./internal/message` covers the touched package.

Net: deleted repeated trim calls without changing API error projection.

### 116. delete: repeated guided room trigger emptiness trims

Status: landed in this pass.

Location: `internal/chat/room/visible_context.go:54`

Guided public Room context checked whether the latest trigger was empty by
trimming trigger type and content inline.

Replacement:

- Trim trigger type and content once before the emptiness guard.
- Reuse those values for the existing no-context return.

Current evidence:

- `go test ./internal/chat/room` covers the touched package.

Net: deleted repeated trim calls without changing guided Room context output.

### 117. delete: repeated parsed session chat type trims

Status: landed in this pass.

Location: `internal/protocol/model_session_key.go:180`

Session key parsing trimmed chat type segments to test for emptiness, then
trimmed the same segment again for assignment.

Replacement:

- Set the default chat type first.
- Trim each chat type segment once only when the segment exists.

Current evidence:

- `go test ./internal/protocol` covers the touched package.

Net: deleted repeated trim calls without changing parsed session key defaults.

### 118. delete: repeated Feishu message text trims

Status: landed in this pass.

Location: `internal/service/channels/adapters/feishu_ingress.go:244`

Feishu ingress text extraction trimmed message type and JSON text repeatedly
while deciding whether a text payload should be decoded.

Replacement:

- Trim message type once before the text/default-type branch.
- Trim decoded text once before returning it.

Current evidence:

- `go test ./internal/service/channels/adapters` covers the touched package.

Net: deleted repeated trim calls without changing Feishu text extraction.

### 119. delete: repeated Feishu message identity trims

Status: landed in this pass.

Location: `internal/service/channels/adapters/feishu_ingress.go:120`

Feishu message ingress decoded `message_id`, `chat_id` and app ID by trimming
the same fields across guards, routing, delivery and inbound envelope fields.

Replacement:

- Trim message ID, chat ID and app ID once near payload decode.
- Reuse those values for route, delivery and message envelope construction.

Current evidence:

- `go test ./internal/service/channels/adapters` covers the touched package.

Net: deleted repeated trim calls without changing Feishu message routing.

### 120. delete: repeated Feishu reaction identity formatting

Status: landed in this pass.

Location: `internal/service/channels/adapters/feishu_ingress.go:178`

Feishu reaction ingress rebuilt the same chat ID, app ID, chat type and reaction
text for both the request and inbound message envelope.

Replacement:

- Normalize reaction chat/app identity once.
- Build the reaction text once and reuse it in both outputs.

Current evidence:

- `go test ./internal/service/channels/adapters` covers the touched package.

Net: deleted repeated formatting without changing Feishu reaction routing.

### 121. delete: repeated automation retry status trims

Status: landed in this pass.

Location: `internal/service/automation/service_task_run.go:75`

Run delivery retry checks trimmed run status and delivery status repeatedly
while deciding whether a failed delivery can be retried.

Replacement:

- Trim run status once before pending/running checks.
- Trim delivery status once and reuse it in the error message.

Current evidence:

- `go test ./internal/service/automation` covers the touched package.

Net: deleted repeated trim calls without changing retry eligibility.

### 122. delete: repeated WeCom request ID trims

Status: landed in this pass.

Location: `internal/service/channels/adapters/wecom_bot_socket.go:318`

WeCom long-connection request ID matchers trimmed the same incoming request ID
for both non-empty checks and equality checks.

Replacement:

- Trim the incoming request ID once per matcher.
- Reuse it for the empty check and stored ID comparison.

Current evidence:

- `go test ./internal/service/channels/adapters` covers the touched package.

Net: deleted repeated trim calls without changing WeCom socket matching.

### 123. delete: duplicated HTTP delivery success handling

Status: landed in this pass.

Location: `internal/service/channels/transport/http.go:76`

`ExpectSuccess` and `ExpectSuccessDecode` carried the same success-body discard
and failure error formatting. The no-decode helper only needs the decode helper
with a nil output.

Replacement:

- Delegate `ExpectSuccess` to `ExpectSuccessDecode(response, nil)`.
- Keep the public helper name for existing delivery adapters.

Current evidence:

- `go test ./internal/service/channels/transport` covers the touched package.

Net: deleted the duplicated HTTP response branch without changing adapter APIs.

### 124. delete: repeated message first-non-empty trims

Status: landed in this pass.

Location: `internal/message/processor_system.go:62`

Message processors trimmed values before passing them to `firstNonEmpty`, even
though `firstNonEmpty` already trims every candidate and returns the trimmed
winner.

Replacement:

- Pass raw candidates to `firstNonEmpty`.
- Keep explicit trims only where the value is used directly without that helper.

Current evidence:

- `go test ./internal/message` covers the touched package.

Net: deleted repeated trim calls without changing message projection output.

### 125. delete: repeated automation first-non-empty trims

Status: landed in this pass.

Location: `internal/service/automation/runtime_delivery.go:56`

Automation delivery and daily report code trimmed candidates before calling
`firstNonEmpty`, while the helper already trims each candidate and returns the
trimmed winner.

Replacement:

- Pass raw candidates to `firstNonEmpty`.
- Keep direct `TrimSpace` calls where values are not going through the helper.

Current evidence:

- `go test ./internal/service/automation` covers the touched package.

Net: deleted repeated trim calls without changing delivery or timezone output.

### 126. delete: repeated permission event agent trims

Status: landed in this pass.

Location: `internal/runtime/permission/request.go:152`

Permission events trimmed `firstNonEmpty` results even though this package's
helper already trims candidates and returns the trimmed winner.

Replacement:

- Assign the helper result directly for request and resolution event agent IDs.

Current evidence:

- `go test ./internal/runtime/permission` covers the touched package.

Net: deleted duplicate trim calls without changing permission event routing.

### 127. delete: repeated skill first-non-empty trims

Status: landed in this pass.

Location: `internal/service/skills/marketplace_import.go:99`

Skill marketplace and registry code trimmed candidates before calling
`firstNonEmpty`, while the shared skill helper already trims each candidate and
returns the trimmed winner.

Replacement:

- Pass raw config URL, commit output, source, and slug candidates to
  `firstNonEmpty`.
- Keep path-specific trimming outside the helper where it removes slashes.

Current evidence:

- `go test ./internal/service/skills` covers the touched package.

Net: deleted repeated trim calls without changing skill marketplace defaults.

### 128. delete: repeated transcript content override trim

Status: landed in this pass.

Location: `internal/storage/workspace/store_agent_history_transcript_project.go:138`

Transcript projection trimmed a user content override before passing it to
`firstNonEmpty`, while the storage helper already trims candidates and returns
the trimmed winner.

Replacement:

- Pass the raw override to `firstNonEmpty`.

Current evidence:

- `go test ./internal/storage/workspace` covers the touched package.

Net: deleted one repeated trim without changing transcript projection content.

### 129. delete: repeated first-non-empty helper trims

Status: landed in this pass.

Location: `internal/service/automation/service_util.go:52`

Several local `firstNonEmpty` helpers trimmed the same candidate for the empty
check and again for the return value.

Replacement:

- Store the trimmed candidate once and return it.

Current evidence:

- `go test ./internal/service/automation ./internal/runtime/permission ./internal/service/skills ./internal/storage/workspace` covers the touched packages.

Net: deleted duplicate helper trim calls without changing helper outputs.

### 130. delete: repeated domain first-non-empty helper trims

Status: landed in this pass.

Location: `internal/chat/dm/session.go:60`

DM session, agent prompt, and chat attachment helpers trimmed the same candidate
for the empty check and return value.

Replacement:

- Store the trimmed candidate once and return it.

Current evidence:

- `go test ./internal/chat/dm ./internal/service/agent ./internal/protocol` covers the touched packages.

Net: deleted duplicate helper trim calls without changing helper outputs.

### 131. delete: repeated connector first-non-empty helper trims

Status: landed in this pass.

Location: `internal/service/connectors/credential_payload.go:132`

Channel ingress, connector credential, and Feishu Docx helpers trimmed the same
candidate for the empty check and return value.

Replacement:

- Store the trimmed candidate once and return it.

Current evidence:

- `go test ./internal/service/channels ./internal/service/connectors ./internal/connectors/feishudocx` covers the touched packages.

Net: deleted duplicate helper trim calls without changing helper outputs.

### 132. delete: repeated runtime ID trims

Status: landed in this pass.

Location: `internal/runtime/executor_round_stream_diagnostics.go:106`

Runtime session and stream message ID helpers trimmed the same candidate for
the empty check and return value.

Replacement:

- Store the trimmed ID once and return it.

Current evidence:

- `go test ./internal/runtime` covers the touched package.

Net: deleted duplicate trim calls without changing session or message ID output.

### 133. delete: repeated automation owner and status trims

Status: landed in this pass.

Location: `internal/service/automation/service_task_support.go:18`

Automation helpers trimmed owner, workspace path, and delivery status values for
empty checks and again for returned values.

Replacement:

- Store the trimmed value once and return it.
- Leave agent lookup inputs unchanged.

Current evidence:

- `go test ./internal/service/automation` covers the touched package.

Net: deleted duplicate trim calls without changing automation owner, workspace,
or delivery status output.

### 134. delete: repeated execution sink result trims

Status: landed in this pass.

Location: `internal/automation/execution_sink.go:183`

Execution sink error helpers trimmed result text for the empty check and again
for the returned message.

Replacement:

- Store the trimmed result text once and return it.

Current evidence:

- `go test ./internal/automation` covers the touched package.

Net: deleted duplicate trim calls without changing execution error messages.

### 135. delete: repeated personal Weixin client ID trim

Status: landed in this pass.

Location: `internal/service/channels/adapters/personal_weixin_channel.go:249`

Personal Weixin inbound message ID fallback trimmed the client ID for the empty
check and again for the returned ID.

Replacement:

- Store the trimmed client ID once and return it.

Current evidence:

- `go test ./internal/service/channels/adapters` covers the touched package.

Net: deleted one duplicate trim without changing message ID fallback behavior.

### 136. delete: repeated room and run delivery trims

Status: landed in this pass.

Location: `internal/service/room/round_registry.go:112`

Room helpers and automation run delivery defaults trimmed IDs or delivery
statuses for empty checks and again for returned values.

Replacement:

- Store the trimmed value once and return it.

Current evidence:

- `go test ./internal/service/room ./internal/storage/automation` covers the touched packages.

Net: deleted duplicate trim calls without changing room round, workspace path,
or run delivery status output.

### 137. stdlib: hand-rolled map clone loops

Status: landed in this pass.

Location: `internal/message/helpers.go:57`

Several helper functions hand-rolled shallow map cloning even though the
standard library already provides `maps.Clone`.

Replacement:

- Use `maps.Clone`.
- Preserve local nil/empty-map behavior where callers already depended on it.

Current evidence:

- `go test ./internal/runtime ./internal/runtime/clientopts ./internal/message ./internal/service/goal ./internal/service/toolpolicy ./internal/storage/workspace` covers the touched packages.

Net: replaced local clone loops with the standard library without changing clone
semantics.

### 138. stdlib: hand-rolled string slice concatenation

Status: landed in this pass.

Location: `internal/runtime/clientopts/options_agent_client.go:127`

Runtime, room, and tool policy helpers built temporary string slices with nested
`append` just to range over `base + extra`.

Replacement:

- Use `slices.Concat(base, extra)`.

Current evidence:

- `go test ./internal/runtime/clientopts ./internal/service/room ./internal/service/toolpolicy` covers the touched packages.

Net: replaced nested append expressions with the standard library without
changing distinct-tool output.

### 139. stdlib: split-once defensive length checks

Status: landed in this pass.

Location:

- `internal/service/channels/ingress_session.go`
- `internal/service/skills/frontmatter.go`
- `internal/cli/command_memory_scope.go`
- `internal/workspace/memory/parser.go`
- `internal/runtime/mcp/automation/internal/builder/builder_schedule.go`

Several call sites used `SplitN(..., 2)` plus length checks, or `Contains`
followed by `SplitN`, for "split once if the separator exists" parsing.

Replacement:

- Use `strings.Cut`, which returns the left side, right side, and separator
  presence in one standard-library call.

Net: fewer defensive sentinel branches and no repeated separator scan.

### 140. stdlib: remaining hand-rolled map clone/copy loops

Status: landed in this pass.

Location:

- `internal/service/toolpolicy/toolpolicy.go`
- `internal/runtime/clientopts/options_agent_client.go`
- `internal/runtime/clientopts/options_runtime_env.go`
- `internal/service/connectors/service_listing.go`
- `internal/connectors/feishudocx/render_markdown.go`

Several helpers manually looped over map entries only to shallow-copy or merge
maps.

Replacement:

- Use `maps.Clone` for pure shallow clones.
- Use `maps.Copy` for map merge helpers.

Net: removes boilerplate loops while preserving each function's existing
nil/empty return contract.

### 141. stdlib: hand-rolled slice membership loops

Status: landed in this pass.

Location:

- `internal/storage/workspace/store_input_queue_order.go`
- `internal/storage/workspace/store_input_queue.go`
- `internal/service/automation/service_observability_util.go`
- `internal/chat/room/records.go`
- `internal/storage/workspace/store_room_directed_message.go`

Small helpers manually walked slices to delete a matching ID, append a unique
string, or answer "does this list contain this value?".

Replacement:

- Use `slices.DeleteFunc`, `slices.Contains`, and `slices.ContainsFunc`.

Net: removes bespoke loop bodies for plain slice membership operations.

### 142. stdlib: log pretty handler nested slice append

Status: landed in this pass.

Location:

- `internal/infra/logx/handler_pretty.go`
- `internal/infra/logx/handler_pretty_value.go`

The pretty log handler used nested `append(append([]T{}, base...), extra...)`
to copy slices before adding attributes or group names.

Replacement:

- Use `slices.Concat`.

Net: deletes the allocation choreography from log handler slice assembly.

### 143. stdlib: remaining dynamic map clone loops

Status: landed in this pass.

Location:

- `internal/message/helpers.go`
- `internal/runtime/permission/request.go`
- `internal/runtime/contextual_input.go`
- `internal/service/skills/service_catalog.go`

Several helpers manually copied `map[string]any` or `map[string]string`
values before adding their own normalization or returning defensive snapshots.

Replacement:

- Use `maps.Clone` for the shallow copy.

Net: deletes four map-copy loops while preserving each helper's existing
nil/empty behavior.

### 144. stdlib: guarded drain slice clones

Status: landed in this pass.

Location:

- `internal/service/room/slot_input.go`
- `internal/runtime/guidance.go`

Queue-drain helpers already returned `nil` for empty queues, then manually
copied non-empty slices before clearing the source.

Replacement:

- Use `slices.Clone` after the existing empty guard.

Net: deletes three manual slice-copy expressions without changing nil/empty
return behavior.

### 145. stdlib: guarded byte and slice clone expressions

Status: landed in this pass.

Location:

- `internal/protocol/model_appserver_rpc.go`
- `internal/service/channels/adapters/feishu_callback_security.go`
- `internal/chat/room/visible_batch.go`
- `internal/storage/workspace/store_agent_history_transcript_marker.go`
- `internal/storage/workspace/store_agent_history_transcript_reader.go`
- `internal/service/automation/service_heartbeat_runtime.go`

These sites manually cloned bytes or non-empty/internal slices with
`append([]T(nil), values...)`.

Replacement:

- Use `bytes.Clone` for byte payloads.
- Use `slices.Clone` where the existing guard or internal use keeps nil/empty
  behavior unchanged.

Net: deletes clone boilerplate without touching API-visible empty-slice cases.

### 146. stdlib: simple slice search helpers

Status: landed in this pass.

Location:

- `internal/service/connectors/service_listing.go`
- `internal/service/skills/service_file.go`
- `internal/service/skills/marketplace_search_filter.go`
- `internal/chat/room/records.go`
- `internal/service/channels/management/catalog.go`

Several helpers manually looped over slices only to answer "any match?" or
return the first matching item.

Replacement:

- Use `slices.ContainsFunc` for boolean membership checks.
- Use `slices.IndexFunc` when the matching item is returned.

Net: replaces local search loops with standard-library slice search helpers.

### 147. stdlib: split-first delimiter parsing

Status: landed in this pass.

Location:

- `internal/handler/shared/middleware.go`
- `internal/service/auth/service_validate.go`
- `internal/automation/heartbeat_prompt.go`
- `internal/runtime/clientopts/log_runtime.go`

Several parsers only needed the text before or after the first delimiter but
used `strings.Split` or manual `Index` slicing.

Replacement:

- Use `strings.Cut`.

Net: removes unnecessary slice allocation and index arithmetic for one-shot
delimiter parsing.

### 148. stdlib: small clone and contains loops

Status: landed in this pass.

Location:

- `internal/infra/logx/handler_pretty_value.go`
- `internal/service/skills/service_file.go`
- `internal/service/skills/frontmatter.go`
- `internal/service/skills/marketplace_git.go`
- `internal/workspace/memory/engine_capture.go`

Several helpers still hand-rolled slice clones, suffix extraction, or
contains-any loops.

Replacement:

- Use `slices.Clone` for defensive slice copies.
- Use `slices.ContainsFunc` for contains-any predicates.
- Use `strings.LastIndex` instead of splitting a session key just to read the
  last segment.

Net: folds the remaining small stdlib substitutions into the same backend
cleanup stage.

### 149. stdlib: delimiter cuts and sorted compaction

Status: landed in this pass.

Location:

- `internal/config/loadenv.go`
- `internal/service/room/service_room_skills.go`
- `internal/service/skills/frontmatter.go`
- `internal/workspace/memory/engine_scope_score.go`
- `internal/workspace/memory/repository_entry.go`
- `internal/workspace/memory/repository_file.go`

Several parsers hand-calculated delimiter offsets or kept a local dedupe helper
after sorting paths.

Replacement:

- Use `strings.Cut` for one-shot delimiter parsing.
- Use the already split scope parts instead of splitting the same scope again.
- Use `slices.Compact` after sorting paths.

Net: deletes one local dedupe helper and removes more index arithmetic without
changing parser contracts.

### 150. stdlib: sorted map keys

Status: landed in this pass.

Location:

- `internal/runtime/manager_round.go`
- `internal/runtime/manager_goal_accounting.go`
- `internal/runtime/manager_interrupt.go`
- `internal/runtime/manager_streaming_input.go`
- `internal/runtime/guidance.go`
- `internal/service/channels/router_registry.go`
- `internal/handler/websocket/registry_room_subscription.go`
- `internal/workspace/memory/engine_util.go`

Several paths built a string slice from map keys and then sorted it manually.

Replacement:

- Use `slices.Sorted(maps.Keys(...))`.

Net: removes repeated key collection loops while keeping deterministic order.

### 151. stdlib: remaining sorted map keys

Status: landed in this pass.

Location:

- `internal/service/skills/service_catalog.go`
- `internal/service/skills/marketplace_update.go`
- `internal/storage/workspace/store_room_directed_message.go`
- `internal/runtime/permission/context.go`
- `internal/handler/websocket/registry_workspace_subscription.go`
- `internal/runtime/mcp/automation/internal/builder/builder_schedule.go`

More code paths still collected map keys with a manual loop before sorting or
iterating them.

Replacement:

- Use `slices.Sorted(maps.Keys(...))`.

Net: removes duplicate key collection loops and keeps deterministic ordering.

### 152. stdlib: remaining slice clones

Status: landed in this pass.

Location:

- `internal/service/connectors/service_listing.go`
- `internal/service/workspace/initializer_skill.go`
- `internal/workspace/memory/engine_mutation.go`
- `internal/workspace/memory/engine_item.go`
- `internal/app/server/dm_external_reply.go`
- `internal/chat/room/visible_batch.go`
- `internal/chat/room/visible_history.go`
- `internal/handler/websocket/registry_room_subscription.go`
- `internal/message/processor_result.go`
- `internal/runtime/clientopts/log_runtime.go`
- `internal/runtime/clientopts/options_agent_client.go`
- `internal/runtime/guidance.go`
- `internal/runtime/permission/request.go`
- `internal/service/agent/service_crud.go`
- `internal/service/dm/service_external_reply.go`
- `internal/service/provider/catalog_provider.go`
- `internal/service/room/chat.go`
- `internal/service/room/directed_message.go`
- `internal/service/room/directed_message_projection.go`
- `internal/service/room/private_domain.go`
- `internal/service/room/public_mentions.go`
- `internal/service/room/slot_state.go`
- `internal/storage/workspace/history_external_delivery.go`
- `internal/storage/workspace/store_room_directed_message.go`

Several defensive copies still used `append([]T(nil), values...)` or an
empty-slice variant where `slices.Clone` says the same thing directly.

Replacement:

- Use `slices.Clone`.

Skipped:

- `internal/runtime/mcp/goal/tool/schema.go` keeps `append([]string{}, required...)`
  because tests require JSON `required` to marshal as `[]`, not `null`.

Net: keeps the defensive-copy contract while making it obvious that this is a
plain slice clone.

### 153. stdlib: test contains helpers

Status: landed in this pass.

Location:

- `internal/automation/task_search_test.go`
- `internal/handler/automation/handlers_test.go`
- `internal/handler/core/handlers_test.go`
- `internal/runtime/clientopts/log_runtime_test.go`
- `internal/service/automation/service_test_helpers_test.go`
- `internal/service/channels/service_control_test.go`
- `internal/service/room/directed_message_test.go`
- `internal/service/room/service_realtime_test.go`
- `internal/service/skills/marketplace_external_test.go`
- `internal/service/skills/service_test.go`
- `internal/service/workspace/service_test_helpers_test.go`

Several tests kept tiny hand-written contains loops.

Replacement:

- Use `slices.Contains` for plain string or enum membership.
- Use `slices.ContainsFunc` when the assertion matches a struct field or trims
  input.

Net: deletes test helper loops while keeping assertion intent local.

### 154. stdlib: pure first-non-empty helpers

Status: landed in this pass.

Location:

- `internal/connectors/providers/registry.go`
- `internal/connectors/providers/feishu_docx.go`
- `internal/service/room/chat.go`
- `internal/service/room/chat_target.go`
- `internal/service/room/execution_private_overlay.go`
- `internal/service/room/goal_continuation.go`
- `internal/service/room/goal_runtime.go`
- `internal/service/room/input_queue_context.go`
- `internal/service/room/input_queue_dispatch.go`
- `internal/service/room/public_mentions.go`
- `internal/service/room/util.go`

Two local helpers only returned the first non-empty string without extra trim or
validation.

Replacement:

- Use `cmp.Or`.

Net: deletes the helper implementations and keeps the fallback choice at the
call site.

## Parallel work rules

- The decomposition process should keep owning file-splitting and directory
  structure.
- A Ponytail slice should only delete or shrink a fallback path after proving
  the path is not needed by current data or supported migrations.
- Do not mix these deletions with large frontend/component splits.
- Recommended first slice: remove the Responses-to-ChatCompletions parser
  fallback, because it is small, isolated, and testable.
