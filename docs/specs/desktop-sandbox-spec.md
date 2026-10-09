# Desktop sandbox integration (experimental)

This spec states implemented host wiring. It is not acceptance of a full App sandbox, and the feature must not be presented as fully accepted App isolation.

- Remaining design and delivery work: [development plan](../explorations/desktop-sandbox/development-plan.md).
- Verified SDK baseline and remaining IO paths: [dated assessment](../explorations/desktop-sandbox/current-assessment-2026-09-15.md).
- Evidence and history: [acceptance matrix](../testing/desktop-sandbox-acceptance.md), [documentation index](../explorations/desktop-sandbox/README.md).
- MCP servers, Connectors and the desktop UI keep their separate authorization.

## Activation and scope

- Desktop execution derives the host sandbox contract from `NEXUS_APP_MODE=desktop`. There is no user-facing or process-environment on/off switch.
- Server deployments keep their existing runtime identity/isolation policy.
- Agent settings cannot set or clear the internal host policy marker.
- DM, Room and background memory maintenance use the same client-options builder.
- Restricted nxs sessions require negotiation of `required_sandbox_v1`, `sandbox_file_tools_v1`, `sandbox_search_tools_v1` and the per-area capabilities in [Per-area capabilities](#per-area-capabilities).
- Restricted Claude sessions use the separate [Bridge `RequireClaudeNativeSandbox` contract](#restricted-claude-contract) with generated native `sandbox` settings (`enabled=true`, `failIfUnavailable=true`, `allowUnsandboxedCommands=false`). They claim no nxs capabilities and never use `--restricted` tool removal as command-sandbox proof.
- An unsupported runtime fails. A desktop session never silently falls back to an unrestricted backend or accepts an unenforced policy.
- Negotiation does not prove current dependencies or an installed effective policy. A capability acknowledgement does not prove that an older binary includes later policy fixes; fixed-version acceptance is separate.
- Only macOS nxs builds declare native file-tool capability. Windows native execution is incomplete, so Windows desktop sessions fail closed before receiving a task.

### Windows Bridge process lifecycle

- Implemented independently of SDK sandbox capability.
- Runtime and CLI probe processes start suspended, join a kill-on-close Job, then resume their validated initial thread.
- Probe cancellation collects descendants before waiting for output pipes.
- Native tests cover immediate descendants and host termination after admission.
- Creation and Job assignment are separate operations; a crash between them can leave a suspended process.
- This boundary does not authorize Windows SDK file/network execution and is not an atomic creation/resource-recovery receipt.
- Native component gate: `node scripts/desktop/check-windows-sandbox.mjs --native`. With a specified local SDK, every required case must actually pass. Process exit must be shown by a kernel signal; a residual handle, a queryable PID/creation time or exit code 259 alone cannot prove liveness. Passing the gate does not grant Windows backend release acceptance.

## Grants and mandatory policy

- The host passes Skill directories as read resources and user-mounted directories as explicit sandbox write grants.
- The SDK additionally grants its stable workspace and compatibility paths under its mandatory execution policy. Project settings cannot expand that policy.
- macOS mandatory SDK precedence:
  - Explicit read/write and protected-path movement denials apply after ordinary directory, device and PTY grants.
  - A read grant cannot reopen content in a denied same, parent or child root; symlink reads use the same boundary.
  - `denyRead` and `denyWrite` are separate; a secret needing both protections must appear in both.
  - Legacy non-mandatory read carve-outs keep their semantics.
  - These guarantees do not prove equivalent Linux/Windows behavior or confinement of all SDK IO.
- Mandatory SDK temporary-directory preparation uses a workspace directory handle before the command sandbox starts, so descendant symlink replacement cannot redirect it outside that root. Unsupported native backends reject before temporary-directory or proxy allocation. Other host-side filesystem operations are not covered by this.

### Workspace root aliases (macOS nxs)

- File approval recognizes both the configured workspace root and its canonical spelling (for example `/private/var` for a `/var` workspace). Read, search and ordinary edits keep their local approval behavior when a resumed or switched backend supplies the canonical path.
- Explicit ask/deny rules match both spellings. Descendant symlinks, hidden writes and protected write globs keep their checks.
- This preflight does not rewrite tool input or replace the execution-time file sandbox. It makes no Windows claim.
- The macOS fixed-source baseline must cover alias file approval and ask/deny, plus child-link and protected-write counterexamples. Switching a UI menu cannot replace verification through actual tools and the effective policy.

## Network approval

- When a mandatory runtime has no explicit host network callback, unknown outbound shell proxy destinations use the SDK's independent `sandbox_network` approval boundary. Explicit SDK host callbacks keep their API; legacy non-mandatory runtimes do not get this fallback.
- Explicit denied domains and managed-only domain policy remain authoritative.
- Default mode asks the user; auto mode uses the independent reviewer with human fallback.
- A request carries the command input, tool-use identity, captured working directory and exact host/port, bound to the command's permission epoch.
- Nexus shows one pending connection and offers no persistent grant. The SDK rejects input changes or permission updates in the response; Nexus also rejects persistent updates.
- Approval resumes the pending connection without replaying the command.
- Effective permission changes invalidate pending and later network requests from the old command epoch.
- Direct Bash/PowerShell background startup carries the same scoped callback when it rebuilds execution options. Foreground completion does not cancel the background proxy's approval; a permission-epoch change still rejects its pending connection. Native macOS Bash tests cover only this continuation and invalidation.
- Closing an SDK execution proxy cancels its pending callback contexts, rejects late allows and closes owned SOCKS/HTTP CONNECT tunnels. This does not prove every process has terminated; durable execution-effect recovery and background/session turnover acceptance are outstanding.

### Automatic-review reminder

- When the automatic reviewer approves a sandbox escape or one pending network connection, the SDK attaches a short provider-neutral reminder to that exact tool call's result. It names the boundary and tool-use identity, states the approval is one-shot, and says it neither changes policy nor authorizes replay or a broader action.
- Ordinary automatic tool approvals stay result-based, with no reminder.
- UI and audit may keep the full review rationale; it is not copied into model context.
- A persistent network policy amendment explicitly chosen by the user is a separate decision with its own policy-change result. It is never inferred from a one-shot approval.

## Sandbox escape

- Classified by typed `permission_boundary=sandbox_escape`. Unknown boundary classifications are rejected before a pending approval is created.
- Covers explicit Bash/PowerShell escape requests and ordinary external `Write`/`Edit` targets. Explicit command escape goes through the separate sandbox-bypass approval boundary.
- Nexus shows an explicit outside-sandbox explanation and offers only one-time approval.
- An approved file action gets a temporary helper-scoped write capability for the reviewed operation (including same-directory atomic staging) only while that helper runs. It is not a user directory whitelist and is not persisted in Agent settings.
- Persistent rules in a response are rejected by both Nexus and the SDK. IM cannot persist a grant when no scope suggestion exists.
- The SDK rejects an allow response that adds escape, or that changes the reviewed JSON input while staying outside the sandbox. Ordinary input edits and returning an action inside the sandbox keep existing behavior.
- Hidden paths, symlinked ancestors, read-only resource scopes and configured protected paths stay denied even when the user is asked to approve.
- The SDK captures the working directory before asking and uses it for Bash/PowerShell execution, including streaming entrypoints. A later session cwd update cannot redirect the approved relative command. The explanation includes this directory, and automatic-review human fallback preserves it. This captures session state; it is not an inode lease against filesystem renames.

## Host scratch resources and policy receipts

### Scratch leases (nxs only)

- The pinned Bridge exposes host-only `SandboxSettings.Resources` and the independent `sandbox_resources_v1` contract for macOS command/file write scopes and a host-prepared private scratch directory.
- Nexus prepares owner/runtime-scoped leases for desktop nxs DM, Room and background memory maintenance, and releases them only after a confirmed Bridge close.
- Each acquisition is an independent handle over the shared resource. Preparation failure or an old runtime generation cannot release a newer holder's scratch.
- Failed cleanup keeps the runtime fence and exact lease for recovery and blocks new acquisition in that scope.
- With a lease present, Nexus forces `allowUnsandboxedCommands=false` and rejects explicit write-directory grants under the read-only scope; the Bridge rejects that combination before transport startup.
- Claude's native sandbox settings never receive an nxs lease; cross-backend mixing fails closed.
- An active owner/session lease keeps its write scope immutable. A later round cannot widen or narrow the policy by reusing the same scratch path.
- During startup configuration changes, cleanup uses the exact captured lease handle and keeps the current client lease available for retry. Lifecycle invalidation reuses an existing cleanup owner for that handle instead of transferring it twice.
- If the handle that first recorded `cleanup_unknown` is released while sibling handles still reference the resource, the cleanup fence moves to one live sibling. It never attaches to an already released handle and is never dropped.

### Markers and stale sweep

- The host creates and removes the scratch parent and lease directory through `internal/infra/confinedfs` fixed directory handles.
- Marker reads and stale scans reject replaced parents, symlinks, non-regular marker files and hard-linked marker identities (on Windows via the opened file handle). A cleanup failure keeps the exact lease registered rather than treating a redirected path as success.
- A failed close persists `cleanup_unknown`, a bounded error summary and the update time through the fixed lease directory handle. Discovery exposes it after restart; deletion still requires an explicit owner-scoped sweep.
- A `cleanup_unknown` marker is kept even when its recorded PID is dead, until a separate reconciliation proves the full runtime boundary closed.
- Where a safe process-identity query exists, the marker also records `process_start_time_unix_nano`. Windows compares it with `GetProcessTimes` before treating an ordinary marker as stale, so a reused PID cannot authorize cleanup. Permission or query failure stays unknown; `cleanup_unknown` always wins. Older markers and platforms without this probe keep the conservative PID-liveness check.
- Scratch allocation checks persisted markers through its fixed parent handle. The same owner/session's `cleanup_unknown` marker, including one under an older replacement path, and invalid markers at that scope's expected path block a new lease after restart. Unrelated sessions stay independent; concurrent preparation in one host cannot misread a half-written marker.
- These are durable startup fences, not proof that detached descendants terminated. The Bridge Unix sweep only observes visible members of the original session.
- Automatic crash sweep, complete supervision and safe reconciliation of uncertain execution are separate work.

### Effective-policy receipts

- Connect writes a receipt for each runtime generation to the host database and keeps a clone in memory for the connected session.
- Fields: required/acknowledged Bridge capabilities, policy digest, session identity and (when present) exact scratch lease/round identity.
- Lifecycle phases: `confirmed`, `retiring`, `retired`, `unknown`. Updates are monotonic: a late callback cannot reopen `retired` or `unknown` as `retiring`.
- Within one owner/session/generation the payload and `confirmed_at` are immutable. A duplicate connect may refresh only `updated_at`; a terminal row ignores late payload retries.
- A fresh Claude connection may not publish its session identity until the first user turn; its receipt is provisional until then.
- After a restart the latest owner-scoped receipt is readable, but a persisted receipt never represents a connected runtime or grants permission.
- A receipt is host diagnostic/admission evidence. It does not attest whole-SDK IO, OS descendants, network, secrets or native platform isolation.
- There is no automatic or browser-triggered reconciliation to `reconciled`; an `unknown` receipt stays unknown until a control surface can prove the complete runtime boundary.
- The owner process reaper is part of the close boundary. If Bridge close reported `retired` but the owner-level reaper fails, the host downgrades that exact generation to `unknown` with a bounded reason, so a clean Bridge close never hides descendants the host could not prove collected.

### Recovery API

- `GET /settings/runtime/sandbox/resources` (read-only) and `POST /settings/runtime/sandbox/reconcile` derive the owner from the authenticated request and never accept an owner or filesystem root from the caller.
- Reconcile requires a positive `older_than_seconds` and is a dry run unless the body sets `apply=true`. Active, unknown, malformed and `cleanup_unknown` markers are still kept; a dead PID is not proof that descendants and handles are gone.
- `GET /settings/runtime/sandbox/receipt?session_key=...` returns the current owner-scoped connected generation's receipt, falling back after a restart to the latest durable receipt for that exact owner/session. A missing, closing, cross-owner or non-desktop generation with no durable row returns not found.
- These are local diagnostic/recovery surfaces, not native platform acceptance.

## Per-area capabilities

Common rules:

- Admission requires the exact acknowledgement; acknowledgements of other capabilities (command, file, search, media, …) never substitute.
- These are host requirements, not user sandbox toggles; normal settings cannot substitute for them. Requirements marked "identity" participate in the process-policy fingerprint and require runtime replacement when changed.
- Preparation, execution, cancellation and limit failures never fall back to direct host IO or the ordinary runner.
- Each guarantee covers only its listed scope. None of them covers whole-SDK IO, Claude, other platforms, hook execution, persistence, background IO, full descendant supervision or effective-policy receipts.

| Area | Setting · initialize · acknowledgement | Requires |
| --- | --- | --- |
| File tools | `RequireFileTools` · `required_sandbox_file_tools` · `sandbox_file_tools_v1` | command |
| Search tools | `RequireSearchTools` · `required_sandbox_search_tools` · `sandbox_search_tools_v1` | — |
| Local media | `RequireMediaFiles` · `required_sandbox_media_files` · `sandbox_media_files_v1` | — |
| Remote media | `RequireMediaNetwork` · `required_sandbox_media_network` · `sandbox_media_network_v1` | command, file, local media |
| Notebook | `RequireNotebookFiles` · `required_sandbox_notebook_files` · `sandbox_notebook_files_v1` | command, native file |
| Skill | `RequireSkillFiles` · `required_sandbox_skill_files` · `sandbox_skill_files_v1` | — |
| Context | `RequireContextFiles` · `required_sandbox_context_files` · `sandbox_context_files_v1` | — |
| Project definitions | `RequireProjectFiles` · `required_sandbox_project_files` · `sandbox_project_files_v1` | — |
| Managed policy | `RequireManagedPolicy` · `required_sandbox_managed_policy` · `sandbox_managed_policy_v1` | — |
| Settings files | `RequireSettingsFiles` · `required_sandbox_settings_files` · `sandbox_settings_files_v1` | — |
| Settings writes | `RequireSettingsWrites` · `required_sandbox_settings_writes` · `sandbox_settings_writes_v1` | required sandbox, file tools, settings files |

### File tools (identity)

- Read/Write/Edit content, directory suggestions, link/metadata and freshness checks use the restricted file executor.
- A binary acknowledging only the command contract is rejected before any task or internal continuation is sent.

### Search tools (identity, macOS only)

- An SDK confirming commands and Read/Write/Edit but lacking search is rejected before task writes.
- Glob/Grep path checks, missing-path suggestions, rg and result metadata use the restricted file environment.
- The auxiliary process gets a minimal environment and no network, including with a custom rg executable. With a ResourcePolicy, search uses the same write scope and scratch.
- Restricted searches return complete results or an explicit failure; single-file content/count results keep their filename.
- The file capability keeps its original scope.

### Local media (identity, macOS)

- ViewImage and main-model preprocessing read local paths, file URLs, symlinks, deferred references, user images and nested tool-result images through the file executor.
- Local paths are materialized before provider dispatch. Local access is checked before auxiliary analysis cache lookup.
- HTTP image downloads and remote URL forwarding belong to the remote media capability.

### Remote media

- All remote image sources, including deferred references and nested tool images, are downloaded before dispatch to the main/auxiliary Provider. Provider URL support cannot bypass the captured policy.
- Every HTTP request and redirect is admitted against the network policy; deny and managed-only rules stay authoritative.
- Explicit ViewImage network approvals bind exact input, tool-use, cwd, destination and permission epoch; they reject input changes and persistent grants and do not survive cancellation or a policy change.
- Preprocessing without a tool identity uses existing network grants or an explicit host callback.
- Environment proxies and Provider credentials are not inherited; configured host proxies remain supported.
- Cleanup cancels pending approvals and body reads.
- Out of scope: model Provider transport, WebFetch, external MCP.

### Notebook (identity, macOS nxs only)

- Notebook content and cell outputs are parsed only after local bytes are read through the restricted file executor.
- Out of scope: Notebook execution, remote networking.

### Skill (identity, macOS)

- Covers initial/model/Slash catalogs, Skill bodies, Read-triggered dynamic discovery, Git ignore queries and remember-availability settings. They share the captured file context and cwd, including metadata and symlinks.
- Allowed project/user/additional sources and conditional/Git ignore behavior remain supported.
- Git gets a minimal environment and no network. Cancellation, unknown exit results and preparation failures never trigger host IO fallback.
- Uncertain dynamic observations may be rechecked on a later file access.
- Out of scope: global startup settings, hooks.

### Context (identity)

- Startup and compact instruction loading, dynamic instruction discovery and recent-file restoration read contents, metadata, directories, symlinks and instruction-exclusion settings through the file boundary.
- A denied optional instruction is not injected.
- Unreadable or malformed selected exclusion settings stop startup or reload instead of removing the exclusion policy.
- A failed reload clears stale instructions and blocks the next model request until reading recovers.
- Query, manual compact and child-agent dispatch keep the current cancellation context. Startup/reload and compact file restoration have bounded total read time.
- Out of scope: global permission/provider settings, project definitions, hooks.

### Project definitions (identity)

- Before tool assembly, discovery of user/project Agent and command definitions, project Skill definitions and selected hook-setting files uses the file boundary. Entries, metadata, symlinks and contents share one worker.
- Missing settings are allowed. Denied, canceled or malformed settings reject the whole snapshot.
- A failed refresh clears the catalog and blocks later model requests.
- Agent/hook changes need a new runtime (bound at assembly); a successful refresh may update Slash bodies directly.
- Out of scope: global permission/provider and managed-policy loading.

### Managed policy (identity, macOS)

- Before settings environment projection, nxs fixes the managed root and an immutable policy snapshot. All later consumers, including child runtimes, use it.
- Malformed JSON, invalid known safety-field types, unreadable files and effective policy changes block query, manual compact, tool dispatch, file-context preparation and permission updates.
- Restoring the original effective policy recovers; applying a new policy requires runtime recreation.
- The reader accepts regular files up to 16 MiB each, with nonblocking Unix open and descriptor type validation. Required execution excludes task settings before any such read.
- Out of scope: ordinary settings and credentials, permission-persistence concurrency.

### Settings files (identity, macOS)

- nxs fixes the config root and selected sources before profile projection.
- The file worker reads ordinary user/project/local/flag settings. Disabled sources are filtered before IO; each selected document is limited to 16 MiB; incomplete or invalid snapshots are rejected.
- Runtime consumers share a bound snapshot; child runtimes keep independent logical snapshots.
- Source changes or read errors block query, compact, tool dispatch, file-context preparation, settings controls and permission updates. Restoring the original content recovers; new contents require runtime recreation.
- `get_settings` uses the bound flag sources. Dynamic updates reject fields whose execution configuration is static.

### Settings writes (identity, nxs on native macOS only)

- Config and permission updates share the checked Binding, physical directory identity and process-local write transaction.
- Existing or newly created parents must remain real directories beneath the fixed physical root. Symlink swaps, directory generation changes, special files and read-only targets fail closed.
- A single document is replaced via a same-directory temporary file. Task sandbox rules deny both lexical and physical aliases of protected settings and temporary names.
- Multi-document permission updates use deterministic order and mark the shared store unknown after a partial commit.
- The pinned SDK adds per-root cross-process locks, post-lock snapshot checks, parent-directory syncing, and reverse-order rollback when changed documents can still be identified. There is no multi-file power-loss atomicity or durable SDK execution receipt.
- Config writes canonical nested settings keys. Explicit SDK Options and process environment keep higher precedence, so a persisted value is a settings default, not proof of the effective runtime value.
- A real Config change marks the runtime for recreation; the query loop checks this fence before every provider turn. A no-op leaves the runtime usable.
- WebFetch checks the binding immediately before calling its selected summary provider, environment endpoint or host adapter, including changes during page retrieval.
- Initialization reserves its admission state when enqueued. At most 32 ordinary stream messages wait for success; initialization failure discards them without creating a base-config Session.
- The requirement and its host-only flag participate in Bridge and Nexus process identity.
- Nexus configuration-control receipts have their own unknown recovery, review/reconcile and revision contract in the [configuration specification](conversational-configuration-control-spec.md); they do not substitute for SDK file-transaction evidence. Neither path replays an unknown write.
- Unix replacement keeps ordinary permission bits but makes no owner, ACL, xattr or file-flag claim. Windows has compile coverage only; its Go writable-bit checks do not establish DACL privacy.

## Memory IO (paired nxs build)

These are internal fixes in the jointly released Nexus/nxs pair, not a new capability or a broader `sandbox_context_files_v1` guarantee.

### Recall and manifests

- Directory discovery, link metadata, frontmatter and selected content use the current file executor. Missing ports or preparation never fall back to host IO.
- Directory traversal does not follow links. Selected content is re-read after the selector, so replacement with a denied symlink is rejected.
- Header and body reads use bounded prefixes enforced by both helper and caller; ordinary full-file streaming keeps its large-file behavior.
- Budgets unchanged: 200 lines / 4096 bytes per memory, 60KB per session. Workspace paths and selection semantics are unchanged.
- Recall and manifest reads each have a 30-second total deadline. Canceled recall cannot publish a partial attachment.

### Store initialization and Summary

- Memory-store initialization and Summary file, template, prompt and compact input use the current file executor.
- Initial files use exclusive creation: existing or concurrently created content is kept; unknown results stop the operation without replay or path-based deletion.
- Only confirmed absence permits initialization or built-in template fallback; denied reads are errors.
- Summary preparation has a 30-second IO deadline. Model edits go through the existing exact-file Edit permission and sandbox.
- Exclusive creation does not promise atomic content publication or power-loss transactions.
- Read-only resource sessions can read existing memories but do not initialize the layout or schedule AutoMemory, Summary or AutoDream persistent updates.

### AutoDream scheduling

- Completion timestamps, transcript directory discovery and each candidate's target metadata use the current file executor, with a shared 30-second deadline.
- Only missing histories are empty. Denied reads, other IO errors or cancellation return no partial candidates, start no maintenance and do not advance the scan interval.
- Files removed during a scan are skipped; non-regular targets are not accepted as markers or transcripts.

### Writer locks and maintenance lease

- A writer lock is never reclaimed only because its file is over an hour old. A known holder must be confirmed exited; a live/current holder, missing probe or malformed/partial record keeps the lock. Permission and unsupported-probe errors are not exit evidence. Process-observation handles are released.
- macOS AutoMemory/AutoDream acquire a maintenance writer through the current file executor. A dedicated sandboxed worker holds a pinned directory descriptor and a kernel lock; release closes owned descriptors without deleting the guard path.
- Completion checks directory/guard identity, uses exclusive temporary files and a rename within the pinned directory, and needs a valid terminal reply plus confirmed successful worker exit. A lost reply stays unknown, without replay or path-based compensation.
- Acquisition and terminal operations each have a 30-second deadline. The lease follows the task lifetime; cancellation, parent EOF or worker failure cancels maintenance.
- Failed completion/release cannot publish a saved event or advance an extraction cursor.
- Existing active PID records are kept; only confirmed dead holders permit continuation. A busy result may still follow creation of the stable guard.
- Memory layout and data are unchanged.
- Not covered: arbitrary detached-descendant supervision, general unknown-cleanup recovery, resistance to arbitrary unconfined same-UID tampering.

### Background content-replacement reads

- Summary/AutoMemory/AutoDream read the current recorder transcript through an explicit file-executor streaming port. Missing ports fail closed; only a confirmed missing current transcript is empty.
- Denied, canceled or incomplete reads stop the background model, with no fallback to a host catalog or Git worktree scan. Preparation and IO share a 30-second deadline.
- The transport does not buffer the whole file. The session layer keeps its 5 MiB threshold, last non-preserved compact suffix, metadata and explicit skip opt-out; the valid suffix has no new hard size cap.
- Records are published only after complete length/result/EOF verification and successful worker exit; late errors discard already-delivered data.
- Session recording/resume, transcript format and stored user data are unchanged.

### Transcript writes and fork

- While restricted, session fork materialization fails closed: the SDK's legacy multi-file fork writer cannot run before the file executor is installed, so a pre-runtime transcript/plan write cannot bypass the host boundary. Restricted fork needs an atomic fork port first. Full Access keeps its fork behavior.
- Restricted recorder updates and tombstone/UUID-based transcript rewrites use the executor-backed transcript mutation port; the recorder port uses the native helper's same-directory atomic replacement for append/update/delete and artifact writes. Full Access keeps its compatibility path.
- Live context-state rewrites fail closed until an atomic mutation port exists.
- Other transcript auxiliary reads and remaining IO are separate work; other platforms keep their local coordination path.

## Host-owned Provider inputs

- After every environment merge, Nexus finalizes `NEXUS_PROVIDER_MANAGED_BY_HOST=1` and host-owned AutoDream wake (`runtime/clientopts`). `ExtraEnv`, `ConfigurationEnv` and task settings cannot revoke these nxs declarations. Claude receives no nxs ownership claim.
- Provider ownership, subprocess scrub and background-wake declarations are part of the process-policy fingerprint; changing them replaces the old process before reconfiguration. Ordinary Provider credential rotation stays a hot update.
- The fixed SDK checks Provider ownership before projecting ordinary settings. In host-managed mode, settings cannot supply Provider/main/fallback/background models, vision routes, credentials, custom headers, request-body overrides, proxy or certificate inputs. Explicit host Options/environment stay authoritative; ordinary task environment values stay available.
- Background-model settings updates that cannot take effect in host-managed mode return an error. Standalone SDK settings keep their routing semantics.
- Anthropic-compatible third-party models: host-owned `BaseURL` is projected as `ANTHROPIC_BASE_URL`. nxs projects host-owned `AuthToken` through the SDK's `ANTHROPIC_API_KEY` path for first-party and compatible endpoints (emitting `x-api-key` and the compatible-endpoint Bearer fallback). Claude keeps `ANTHROPIC_AUTH_TOKEN` for its native CLI semantics.
- [Live-provider evidence](../testing/desktop-sandbox-acceptance.md#2026-09-27真实第三方模型与两种-macos-后端) covers one real gateway; arbitrary gateways and official account/OAuth compatibility are not established. Provider-specific custom headers have no Nexus field and are not accepted.
- The nxs `Sandbox.Network` object governs command/tool execution (including shell network preflight), not the model Provider transport. Nexus does not add the resolved Provider host to `DesktopSandboxNetworkAdmission`. Provider reachability is an input-ownership guarantee only; an OS-level Provider egress boundary needs platform/Bridge evidence and is outside the receipt.
- Command and hook environment builders remove known SDK main/auxiliary credentials after applying runtime environment values. Task values cannot disable an ownership declaration already in the host process. This is a versioned guarantee checked by the fixed-source baseline, not a wire capability inferred from settings-write acknowledgement.
- HTTP hook header interpolation uses the task-visible environment even when the hook allowlist names a Provider credential. nxs MCP configuration interpolation treats such process credentials as missing (URL, argument, environment alias and header locations), keeping existing missing/fallback semantics.
- Dedicated hook/MCP authentication variables and explicit host-provided values stay independent.
- The MCP registry passes the runtime-owned environment into `headersHelper` and refreshes it on environment update. Managed helpers cannot read known Provider credentials or redirect the managed memory root.
- For nxs, Nexus fixes `NEXUS_MEMORY_DIR` to the current Agent workspace after all configuration capability merges and clears remote-memory overrides. The SDK typed memory profile applies the same rule to Summary, AutoMemory and AutoDream. These inputs participate in the process-policy fingerprint, so a changed root cannot reuse an old runtime. Claude does not receive this claim.
- These are environment-source boundaries. They do not establish credential secrecy against host-file reads, process inspection, inherited handles, external MCP processes or network egress, nor whole-process Provider credential isolation or full background IO confinement.

## Remote MCP endpoints on macOS

### Endpoint network (`sandbox_mcp_network_v1`)

- Required through `RequireMCPNetwork` and `MCP.StrictConfig`.
- Persisted Agent HTTP/SSE configuration and typed Connector servers are explicit host inputs.
- A configured endpoint gets a separate grant for its scheme, host and port. Redirects and legacy SSE POST endpoints cannot leave that origin. The MCP domain is not added to command/image network allowlists.
- Explicit denied domains and managed-only domain restrictions still apply.
- Task-settings HTTP/SOCKS/MITM proxy routes are rejected before connecting. MCP credentials require a separate host-owned proxy contract; the runtime must not silently bypass an explicit proxy.
- Persisted MCP and OAuth metadata URLs reject userinfo and fragments before entering the runtime.
- Static MCP headers fail closed at the Nexus configuration boundary: names must be HTTP token names, values cannot contain CR/LF/NUL, at most 128 headers per server. The SDK applies the same limits to dynamic `headersHelper` output, plus a 64 KiB output bound. These checks protect configuration and credential transport only; they are not the host-owned proxy or provider-egress guarantee.
- Request and response bodies are canceled when their connection is retired or its permission epoch changes. Removal, disable, replacement and session shutdown retire owned connections; a delayed discovery cannot revive an old configuration.
- Failed or canceled operations are not replayed. Connection failures are scoped to their MCP server, so an unavailable endpoint does not stop the Agent.
- Other platform admission paths and Claude's native behavior are unchanged.

### Authentication helpers (`sandbox_mcp_helpers_v1`)

- Required through `RequireMCPHelpers`.
- Persisted and Connector `headersHelper` configurations use the current command sandbox, configuration checks and filtered task environment. They do not inherit the endpoint grant or tool approvals.
- Each request refreshes authentication after network admission. Invalid output or execution failure stops that request without using stale/static credentials.
- Limits: 10-second deadline, 64 KiB stdout, 16 KiB stderr. Stderr is never included in service errors.
- Permission changes and connection retirement cancel in-flight helpers. Session close waits for owned helper cleanup and propagates failures.
- Not covered: independently detached descendants, OAuth discovery/token exchange, model Provider networking.

### stdio servers (`sandbox_mcp_stdio_v1`)

- Required through `RequireMCPStdio`.
- Persisted command configurations (including inferred stdio type) and typed Connector configurations start services through the command sandbox.
- The executor owns argv execution, pipes, configuration checks, permission epochs and process/proxy cleanup. The MCP client owns bounded JSONL and unique IDs for concurrent replies.
- Cancellation or timeout retires the whole service and its other pending calls, without replay.
- Same-name replacement waits for the previous process. Session close awaits all owned stdio/helper processes and preserves cleanup errors.
- Inherited Provider credentials are filtered before explicit service credentials are added. Reserved runtime/home/temporary-root environment fields cannot override host policy.
- Network uses the command policy and its proxy, without endpoint grants or current tool approvals.
- Limits: 10 MiB per message, 64 pending stdio requests. Stderr is drained without retaining credential-bearing logs. HTTP/SSE also bound aggregate event and JSON body sizes.
- Not covered: detached descendants, host crash recovery, trusted MCP proxy routes, full secret/handle isolation.

## Approval modes and runtime replacement

### Full Access

- Full Access is an explicit user choice to access local files available to the current OS account, including the host app directory, with **no sandbox isolation guarantee**.
- It grants no administrator rights and does not bypass Nexus domain authorization. Remaining nxs safety checks are not a promise of isolation.
- No separate OS identity is required solely to isolate Full Access tasks.
- Leftover capability handshakes or lifecycle receipts must not be treated as isolation evidence.
- Restricted-mode protection and Full Access lifecycle tests are reported separately.
- A fresh nxs Full Access (`bypassPermissions`) runtime still installs the nxs capability and lifecycle boundary; it broadens the command/file resource policy via the SDK setting instead of disabling the runtime.
- Claude Full Access is an explicit exception: it installs neither the contract nor the settings, but keeps host lifecycle, domain authorization and other mandatory policy.

### Restricted macOS host paths

- Client options derive `appfs.AppDir()` from the host process, never from task `ExtraEnv`.
- Relative state roots resolve against the host working directory. Both lexical and canonical paths are kept, including existing private-directory symlink targets (resolving the existing parent for a new state directory).
- Writes to the entire app tree are denied. Reads are denied for its private `data`, `config`, `cache`, `logs`, `rooms`, `processes`, `.migrations`, `.agents` and `sidecar.lock` paths, and for the state-root `NexusSidecar.pid.json` and its physical aliases (also write-denied).
- Read-only `platform-skills` and `host-skills` projections stay available.
- nxs enforces this with its file sandbox. Claude additionally gets absolute Read/Edit rules for the same private read and whole-tree write scopes, on top of its command sandbox.
- New host secrets must live in these private directories, or the registry in `internal/runtime/clientopts/desktop_host_paths.go` must be extended before a new private path is introduced.
- This protects those execution paths, not arbitrary SDK IO, hooks, IPC or delegated external services.

### Restricted Claude contract

- The host does not change the approval mode to enable sandboxing; the restricted runtime is part of every desktop task contract.
- Bridge `RequireClaudeNativeSandbox` generates one host-owned `--settings` object and, before transport startup, rejects missing, duplicate or overridden JSON, bypass permissions, `--restricted` tool-mode mixing and any unsandboxed-command setting.
- Bridge probes the exact resolved CLI with `--settings <generated-json> --help` (bounded timeout and output, scrubbed environment). A rejected or unadvertised settings entry point prevents startup. Help output does not attest the policy is effective.
- Native command sandbox settings are used instead of `--restricted` (which removes code execution tools) so Bash and build commands remain available.
- `CapabilityClaudeNativeSandbox` is a local Bridge configuration capability, not a Claude wire response or proof of OS/file/network/Provider isolation.
- Bridge settings validation is implemented. Claude effective-policy admission and real allowed/denied command, network, credential, cancellation and cleanup tests are unfinished. Native Windows Claude is rejected until a supported environment is verified.

### Mode changes

- A live host-managed change into or out of Full Access retires the old client before returning the transition signal:
  - DM closes the old session.
  - Room cancels the exact slot and its pending approval requests, retires the client and waits for cleanup.
  - A confirmed close is an expected transition; cleanup failure is reported.
  - The next request builds fresh options, and the process-policy fingerprint forces replacement of the old runtime.
  - HTTP updates process both DM and Room even if one fails; errors are reported together.
- Changes between restricted modes keep the sandbox boundary and use the existing permission-mode update path. Complete approval-revision invalidation across every host callback is not claimed.
- For SDK manual/automatic permission callbacks, effective rule/mode changes cancel the pending policy epoch and invalidate even a late allow. Identical refreshes keep the epoch. A cancelled request cannot create a fresh Nexus pending prompt.
- The nxs proxy preserves human-only requirements and review evidence; the reviewer treats sandbox escape as additional execution authority.
- Mode changes never resubmit a prompt or repeat a tool invocation. Stopping a process does not roll back side effects; an unknown or partial command result is never permission to replay it.

### Cleanup errors

- Bridge cleanup waits for transport exit, not just stream closure. This confirms the runtime main process only; full descendant cleanup needs the platform backend integration.
- Bridge returns `ProcessCleanupError` for failed process-session cleanup, including after main-process success, forced termination and repeated closes. Nexus preserves it even when joined with an ordinary closed-pipe error.
- Failed client cleanup blocks reconnect and stale-startup retry. Manager keeps the exact failed session in closing state instead of publishing a replacement. Explicit close and owner/Agent close callers can read the retained result.
- Sandbox file requirements and resource scopes participate explicitly in the process-policy fingerprint, although ordinary settings serialization excludes these host-only fields.

## Process launch records and recovery

### Launch records

- `sandbox_process_launches` stores process-launch facts under the existing owner/session/generation identity: unique launch ID, boot/user identity, job label, helper digest and optional lease binding. No task arguments, environment or Provider credentials. This is distinct from the policy receipt and from user-writable scratch markers.
- Phases:
  - `prepared` may be canceled as `aborted` before registration.
  - Registration writes the exact original coalition and moves to `registered`.
  - `ClaimProcessRelease` moves to `released` exactly once. A lost response or restarted host must not repeat the claim or resend execution.
  - `registered`/`released` become `reaped` only with an exact original registration plus kernel-coalition retirement or changed-boot observation. The repository validates the binding, not the kernel. Root exit, empty enumeration and a missing launchd job are not accepted proof.
- Aborting an unregistered intent revokes any late registration/release CAS but does not prove the trusted helper exited; its job still needs cleanup.

### Manager admission gates

- For explicitly supervised DM/Room startup, `GetOrCreateWithLease` supplies the already acquired scratch handle before client creation. Required resources without a live matching owner/session/policy handle fail before the factory. Launch intents persist its exact lease ID.
- Each supervised launch revalidates the original handle; a different handle cannot replace it at ownership transfer. Reading this identity does not transfer cleanup responsibility: the caller keeps it until `BindSandboxLease` succeeds. No resource is discovered by path. App default supervisor setup remains unconnected.
- Fresh Manager creation checks launch records when the configured repository provides them. `prepared`, `registered` and `released` block the factory even without a policy receipt or when another backend is selected. `aborted` and properly evidenced `reaped` records feed the same generation lower bound. Read errors and malformed identity/evidence fail closed.
- Current production startup does not yet write these records or launch the bootstrap helper, so storage and the read gate alone do not close the pre-execution crash window or reconcile old unknown receipts.
- Fresh client creation also reads the latest receipt for the exact owner/session before the factory. A retired or explicitly reconciled receipt gives the generation lower bound, so clean App restart or idle-session recreation cannot reuse an old durable identity. Confirmed, retiring or unknown history without the original live client blocks recreation; read/identity failures stop startup.
- This check runs before choosing the new runtime, so a backend or Full Access change cannot bypass unresolved execution. Absence of an in-memory client is not exit evidence, and requests are not replayed.

### Host AutoDream

- Uses the same Manager under an isolated `memory-maintenance:<agent>` key.
- Its owner-scoped background registration starts before the startup transaction, so host shutdown, owner cancellation and Agent revocation cancel maintenance.
- The scratch handle follows the same pre-factory supervision binding and explicit ownership transfer as DM/Room.
- AutoDream is a control request, not a chat round. A bounded cancellation watcher retires/disconnects the exact client without touching the startup transaction. The transaction retires the client after control completion or failure, persisting policy/process terminal facts and keeping failed cleanup fences.
- No raw Bridge session bypass remains in the host maintenance runner.
- Scheduling and consolidation rules belong to nxs. Native disabled-gate control tests establish lifecycle only, not real model consolidation or App UI acceptance.

### Supervised sockets (macOS)

- Sockets stay under the original protected host job directory.
- The Bridge binds/connects by parent directory descriptor and basename on a dedicated native thread, so the absolute path is not limited by `sockaddr_un.sun_path`.
- It does not change process cwd, create a short alias or use shared temporary control directories. Host cleanup keeps unlink ownership; missing thread-local cwd support fails closed.

### Explicit process recovery

- Requires `SandboxProcessRecoveryOwnership`, implemented by the macOS sidecar instance Guard. `WithOwnership` verifies the original lock and app-directory inodes, holds the lock handle through the whole callback (native recovery and durable terminal commit), and blocks concurrent Guard closure.
- The Manager rejects a process directory outside that app root, linked traversal or a different directory inode before reading the original process.
- Missing, closed or replaced ownership cannot reach native job revocation or clear a durable fence.
- This only coordinates participating sidecars. Older uncoordinated hosts and automatic startup recovery remain separate integration requirements. Policy and scratch reconciliation are independent.
- `RecoverPendingSandboxProcesses` handles one ownership-protected batch of at most 256 pending records:
  - The scan uses immutable unique launch IDs as keyset cursors over an index limited to prepared/registered/released rows; retiring earlier rows does not shift later pages.
  - It lists original exact keys, and each recovery re-reads and validates the original intent/registration.
  - An item failure keeps its record pending, is returned on the item and in an aggregate error, and does not starve later items.
  - Cancellation stops before the next item and keeps the last attempted cursor.
  - Callers must inspect errors independently of `HasMore`; failed records can be revisited from their original keys or a new scan.
  - Internal cross-owner query for the lock-holding host only; no user API. It does not replay tasks, reconcile tool outcomes or clear policy/scratch unknown records. Startup invocation remains unconnected.

### Shutdown

- Normal host shutdown closes Manager admission for clients, rounds and background tasks before releasing the App database.
- It cancels round/background work, drains in-flight startup and receipt insertion, then closes sessions in parallel through the existing cleanup and receipt lifecycle.
- Repeated close calls wait for the same result. A caller timeout does not cancel shared cleanup or close the database while runtime writes remain possible.
- This does not clear receipts left unresolved by a crash or failed descendant cleanup.

## Explicit local diagnostics

- Only `GET /settings/runtime/nxs/status?include_sandbox=true` launches the bounded Bridge query behind the settings page's sandbox-support check. Without the option the request stays a file-only check used when selecting nxs.
- `available` stays independent. The response may include `sandbox.state` (`unknown`, `unsupported`, `missing_dependencies`, `dependencies_available`) plus a known platform.
- `dependencies_available` means default local prerequisites are present; task admission still confirms the exact negotiated capability and effective policy.
- Failures stay unknown and never change preferences, approval mode or execution policy. The diagnostic is never an on/off control.
- Bridge module version and checksum are owned by `go.mod`/`go.sum`. Dependency publication, the configured nxs binary and packaged-App acceptance are separate delivery facts (see assessment and acceptance matrix); availability must not be inferred from a successful local workspace build.

## Main integration compatibility

### macOS App/runtime pairing and existing data

- Nexus and nxs ship together in the macOS App. App upgrades replace the matched pair; users have no separate runtime upgrade step.
- Existing development/environment override precedence is unchanged.
- The new pair must preserve existing settings, sessions, memory, workspaces and previously supported behavior.

**Bootstrap helper.**

- The build includes `Contents/Resources/bin/nexus-runtime-bootstrap`, built from the sidecar's pinned Bridge module with native cgo.
- After signing the helper, the build records its SHA-256, exact module version, entrypoint-derived architecture and fixed relative path in `Resources/runtime-bootstrap.json`, then signs the App.
- Assembly and packaging (including skip-build) verify the manifest against the actual helper build identity and bytes. A replacement Bridge module is not a distributable helper source.
- Manifest authenticity relies on the App signing boundary; it is not an independent trust root.
- `infra/runtimebootstrap.LoadCurrent` derives the expected Bridge version from the running sidecar's build information, never from task settings or the manifest. It uses confined file access and verifies path, version, architecture, native cgo build identity and digest.
- The desktop sidecar passes its migration-before-start ownership guard into App assembly. The default Manager uses the verified helper and confined `app/processes`. All native process recovery pages run before lifecycle/resource reconciliation, before HTTP or background admission. Any recovery error fails startup without replay.
- Local ad-hoc assembly does not establish Developer ID, notarization, clean-host or supported-version acceptance.

**Package gate.**

- Bundled builds and packages run `nexus-server check-desktop-runtime --nxs <bundled path>` from the assembled App. It bypasses server startup, `.env`, database migration and model requests; the packaging runner supplies an empty environment and a fresh temporary HOME.
- The product options builder and the sidecar's linked Bridge must complete initialize and close for workspace-write, read-only and Full Access.
- There is no skip flag when nxs is bundled, including skip-build/skip-smoke packaging.
- The report stays outside the signed bundle; package metadata includes the binary SHA-256, Bridge version and confirmed profiles.
- An incompatible rolling-channel download stops the build before distribution. This is a package gate, not a runtime download or permission downgrade at user startup.

**Upgrade check.**

- `scripts/desktop/check-runtime-upgrade.mjs` takes explicit previous-release and candidate binaries and requires three phases: create with the old runtime, resume under current desktop policy, resume with the old runtime.
- A local Provider fixture verifies the actual model history, stable session ID and append-only transcript; settings, memory and workspace fixtures must stay unchanged.
- The old phase uses its supported pre-upgrade policy instead of asking the old runtime to advertise new security capabilities.
- It does not prove database downgrade, signed App installation, every historical release or an external Provider.

### Historical main integration

The pinned Bridge carries main's MCP call-context contract: runtime `params._meta["claudecode/toolUseId"]` reaches the host callback unchanged; missing metadata stays empty; business arguments cannot supply this identity. Integration evidence: [acceptance matrix](../testing/desktop-sandbox-acceptance.md#2026-09-16main-同步与-bridge-兼容).

### 宿主监督启动登记（显式装配）

- Manager 的 `SetSandboxProcessSupervisor` 只允许在启动前由宿主配置可信 helper 摘要和受保护目录句柄。
- client factory 前冻结 owner/session/generation；普通配置热更新保留该 client 的监督工厂。
- Bridge 为 runtime 与各 CLI probe 分别创建 Host。同代次固定顺序：Claude sandbox probe → restricted probe → version probe → runtime；可跳过不适用的探测，不能倒退或重放。
- 唯一索引保证一个 owner/session 只存在一个 prepared/registered/released 启动；未清理 probe 与主进程一样阻断新 factory。
- 迁移 145 保留旧 intent JSON；旧 purpose 空值固定解释为 runtime。同代次存在多个启动记录时，回退到旧唯一键事务失败，不删除部分证据凑成可回退状态。
- macOS App 默认装配此监督器：helper 来自随包校验，根固定为持锁的 `app/processes`。发布验收仍未完成。
- `Manager.RecoverSandboxProcess` 是宿主内部显式入口：
  - 调用方先取得跨进程独占实例锁并确认旧宿主退出；Manager 再取得会话启动 gate 并拒绝活动 client。
  - 按 exact key 读取原记录并调用 Bridge 恢复；不重新 Reserve/ClaimRelease，不重放命令。
  - 终态重复调用只读原结果。失败保留原记录；成功只代表原进程记录收口，不清除 policy unknown 或 scratch 栅栏。
- macOS 桌面 `nexus-server` 在布局迁移前获取 canonical `app/sidecar.lock` 的非阻塞内核独占锁，持有到服务关闭后：
  - 锁文件不保存 PID，不按年龄删除，不 unlink；描述符 CLOEXEC。
  - 第二个采用同协议的 sidecar 拒绝启动。恢复前可用 Guard.Verify 校验目录和锁 inode 未被替换。
  - 该锁不覆盖旧版未持锁宿主，不替代原生窗口锁、原任务集合退出证据或旧版未持锁宿主的退出证明。

### 策略回执与监督进程的身份关联

- 显式启用进程监督时，Manager 在创建 client 前冻结原进程代次；同一 client 的 warm 请求继续增加策略回执代次，不改写原进程代次。
- Connect 持久化策略前，按 owner/session/原进程代次精确读取 runtime 用途记录，将唯一 launch ID 与原进程代次一同保存。
- 探测、未放行进程、跨 owner/session、runtime 或 lease 不匹配均拒绝。进程可以已有精确回收终态；关联本身不宣称其存活。
- 既有关联不可改绑，也不能从无关联升级为推测关联。
- 迁移 147 保留历史无关联记录及 unknown 状态；已有绑定事实时拒绝丢失该事实的数据库回退。
- 该关联用于显式进程/资源恢复后的策略收口，不证明业务动作结果。

### macOS scratch 的可信目录身份

- 监督启动在 Host Reserve 写入启动意图前，从 Acquire 保存的原 parent/leaf 文件信息生成身份（device、inode、generation、birth time），与固定父路径和 leaf name 一同保存。
- 每次 probe/runtime 启动工厂调用都重新打开当前目录核对原身份；已释放、cleanup unknown 或目录被替换时拒绝。
- 身份不从任务可写的 `.nexus-sandbox-lease.json` 读取。原进程恢复保留完整身份，不重新采样。
- 旧记录及非 macOS 路径保留空身份，不能据此自动删除资源。该字段是恢复的必要输入，不是删除已完成的证据。

### 显式资源与策略恢复

`RecoverSandboxScratch`（macOS）：

- 在同 app 根独占实例锁、会话启动 gate 和资源 Acquire gate 内运行。
- 前提：原进程已回收，或从未放行的启动意图已撤销。同会话其他活跃启动、本实例 client 或 lease 存活时拒绝清理。
- 宿主数据库先保存不可改绑的资源回收记录；pending 记录同时阻断 Manager factory 与新进程登记。
- 回收源只接受实例所属 canonical owner runtime/sandbox；任务不能指定删除目标。
- 阶段固定为 `prepared → quarantined → deleting → complete`：
  - 以固定 parent 句柄将原目录不覆盖地移入任务不可访问的宿主回收区，核对移动后身份并同步目录后，才提交 quarantined。
  - 删除前先提交 deleting；删除并同步后才提交 complete。
  - prepared 时源路径缺失不构成成功；只有 durable deleting 阶段下回收区目标缺失才能作为删除完成对账。
  - 任何身份不符、占用、移动或同步失败都保留记录和栅栏。
  - 完成后重复调用不访问或删除后来同名的新目录。
- 依赖 Supervisor Root 任务不可访问的配置合同；受限模式对该根的 deny 由统一 options 构建器投影。完整 App 场景仍待验收。
- 迁移 148 保存资源恢复阶段；已有记录时拒绝丢失恢复事实的降级。本机测试覆盖 SQLite；PostgreSQL 迁移仅有 SQL 审查证据。

`ReconcileSandboxPolicy`：

- 独立核对原进程 reaped 和（存在 lease 时）同资源 complete，再将明确绑定该 process key 的 confirmed/retiring/unknown 策略改为 reconciled。
- 原 unknown reason 保留用于审计；没有进程关联的历史 unknown 不改变。
- 不修改消息、工具、任务或外部副作用结果，不授权重放。
- 资源删除后、策略提交前崩溃可重复调用；启动自动扫描必须覆盖已回收进程的未完成后续步骤（已接入 App 默认启动）。

### 正常退出与终态后续扫描

- 显式 macOS 监督在 client factory 前把正常最终 lease Release 绑定到原 supervisor、store、owner/session、资源身份及启动代次下界。
  - 只有确认该资源从未登记启动时才沿用原句柄删除；存在启动记录时必须匹配原资源，并复用持久隔离删除阶段。
  - 阶段提交响应丢失时保留 owning handle，按原记录重试；不因目录已删除而重新猜测结果。
  - 此回调在资源锁内运行，不反向取得 Acquire gate。
- 迁移 149 为原进程增加独立 resource_phase，并索引资源待办及明确绑定的策略待办；已有进程记录时拒绝丢失扫描进度的回退。
- `RecoverPendingSandboxLifecycles` 是原生 pending 进程扫描之后的第二阶段：
  - 持同实例所有权，按稳定 launch ID 有界发现 terminal 进程，完成原 scratch 后确认资源阶段，再收口 reaped 进程的 exact 关联策略。
  - 正常清理已 complete 的记录无需重访已删除的源目录。无资源身份证明的历史记录不能推测删除或宣称成功。
  - 单项失败保留待办并返回聚合错误，后续项继续；取消保留未处理游标。
  - 游标只代表处理进度，`HasMore=false` 不代表全部恢复成功。
- 调用方须在任务准入前跑完原生和生命周期两阶段并处理失败。App 默认装配已接入：60 秒启动上下文、每页 64 条、错误跨页保留，游标不前进立即失败。
- 真实 nxs AutoDream 正常退出及独立宿主崩溃恢复测试只证明组件链路，不证明 App UI、签名安装包或干净机器验收。
- 开发桌面构建同样固定 Bridge、禁用 go.work，生成带 helper 清单的独立 sidecar bundle；原生壳直接启动校验过布局的 sidecar，不使用 go run。每次构建生成新目录并原子切换开发指针，保留旧目录以免替换仍在运行的 helper。开发产物不是签名发布验收。

### 原生 sidecar 身份与旧记录

- 原生壳读取 `NexusSidecar.pid.json`。version 2 记录保存 PID、可执行路径、系统 boot session UUID 与 kernel audit token。
- 正常退出和孤儿清理都通过 `proc_signal_with_audittoken` 发信号，不回退裸 PID kill；发送前验证同用户、PID/version 与本次 boot。
- 信号调用返回正值时直接按该错误码处理，仅负值读取 errno；只接受成功或精确目标已不存在，其他失败保留恢复记录。
- 旧记录（只有 PID/path）：明确查无进程才清理记录；仍存活或观察未知时保留并拒绝启动，提示先退出旧版 Nexus；不因路径不同而删除。
- 新格式 PID 已复用或 boot 已变化时不向新进程发信号；同一内核进程但可执行路径改变时保留 unknown。
- 损坏、链接、非普通文件、读取或精确终止失败都不当作退出。记录只在当前内容仍匹配本次所有权时删除。
- 这保护原生 sidecar 身份，不替代 Go 实例锁与任务后代恢复。macOS App 最低系统版本 14.2（精确 audit-token 信号接口），更低版本由部署目标直接拒绝。

## 实现约束

- `NEXUS_SANDBOX_TEST_BINARY=/absolute/nxs make check-desktop-sandbox` 是显式桌面沙箱基线：脱离 go.work 验证固定 Bridge 与真实 nxs，缺失或跳过必测用例即失败。原生 macOS 完整入口见 [验收矩阵](../testing/desktop-sandbox-acceptance.md)。
