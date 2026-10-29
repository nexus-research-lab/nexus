# Desktop sandbox integration (experimental)

This spec states implemented host wiring. It is not acceptance of a full App sandbox, and the feature must not be presented as fully accepted App isolation.

- 开发计划、历史评估和实验记录保存在本地或 CI artifact，不随公开仓库发布。
- 验收记录由内部 CI 和本地维护，不随公开仓库发布。
- MCP servers, Connectors and the desktop UI keep their separate authorization.

## Activation and scope

- The host sandbox contract is only available with `NEXUS_APP_MODE=desktop`, and is applied only when the owner's preference `desktop_sandbox_enabled` is on (Settings › Sandbox › Desktop sandbox). The preference defaults to off; with it off, desktop runtimes start without sandbox settings, host-path protection or network admission. There is no process-environment switch.
- Windows desktop sessions use the nxs MXC ProcessContainer backend; contract details are in [Windows backend](#windows-backend). Restricted sessions require nxs, a confined scratch directory outside the workspace and a standard (non-elevated) user token, and receive the same nxs contract as macOS, including strict MCP configuration and the MCP network/helper/stdio requirements. An elevated process rejects restricted sessions before startup, and the SDK also reports MXC unavailable under an elevated token. Full Access projects no sandbox settings or network admission; restricted Claude is rejected. When MXC is unavailable the capability handshake fails closed. Native acceptance steps are in [desktop-sandbox-acceptance.md](../testing/desktop-sandbox-acceptance.md).
- Server deployments keep their existing runtime identity/isolation policy.
- Agent settings cannot set or clear the internal host policy marker.
- DM, Room and background memory maintenance use the same client-options builder.
- Restricted nxs sessions require negotiation of `required_sandbox_v1`, `sandbox_file_tools_v1`, `sandbox_search_tools_v1` and the per-area capabilities in [Per-area capabilities](#per-area-capabilities).
- Restricted Claude sessions use the separate [Bridge `RequireClaudeNativeSandbox` contract](#restricted-claude-contract) with generated native `sandbox` settings (`enabled=true`, `failIfUnavailable=true`, `allowUnsandboxedCommands=false`). They claim no nxs capabilities and never use `--restricted` tool removal as command-sandbox proof.
- An unsupported runtime fails. A desktop session never silently falls back to an unrestricted backend or accepts an unenforced policy.
- Negotiation does not prove current dependencies or an installed effective policy. A capability acknowledgement does not prove that an older binary includes later policy fixes; fixed-version acceptance is separate.
- macOS nxs builds declare native file-tool capability through Seatbelt; Windows nxs builds declare it only when the bundled `nxs-mxc.exe --probe` succeeds, otherwise Windows desktop sessions fail closed before receiving a task.

### Windows desktop shell shutdown

- Window-close exit, tray exit and installer activation share one idempotent asynchronous exit coordinator. The `Closing` callback cancels the initial close; final shutdown occurs on a later dispatcher turn after backend cleanup. WPF uses explicit shutdown, and the single-instance mutex is retained until cleanup finishes or is recorded as failed.
- The backend is created suspended with an atomic `PROC_THREAD_ATTRIBUTE_JOB_LIST` assignment to a shell-owned kill-on-close Job. Only its explicit stdin/stdout/stderr handles are inherited; execution resumes after ownership is established. Normal exit first closes the parent stdin pipe, allowing the backend up to five seconds to drain runtimes and drain runtime cleanup. It then terminates the Job and allows up to five further seconds for its active process count to reach zero. Abnormal shell termination closes the Job through kernel handle cleanup. This is process ownership, not an SDK sandbox policy claim.
- Startup recovery uses a held process handle plus PID, UTC creation time, executable path and state root recorded at launch; the record is replaced atomically through a temporary file. The executable may be at the previous package location. There is no same-name process sweep. Reused PIDs are left alone; missing ownership fields, unreadable records and identity-query errors fail closed and retain evidence. Legacy records without creation time require manual identification/exit of the old application.
- Output draining and health polling are cancellable/bounded, and port waiting runs off the dispatcher. Shutdown failure writes `shutdown-failure-*.json` and retains the process receipt; closing a handle alone is not reported as confirmed cleanup.
- `dotnet run --project desktop/windows/Nexus.Desktop.LifecycleTests -c Release` is Windows-native acceptance for inherited output/environment, tree termination and owner-crash cleanup. `-- identity-only` runs only platform-neutral receipt checks. Cross-compilation and source contracts do not replace native window-close, immediate relaunch, repackaging or PSEC nested-Job validation.

### Windows Bridge process lifecycle

- Implemented independently of SDK sandbox capability.
- Runtime and CLI probe processes start suspended, join a kill-on-close Job, then resume their validated initial thread.
- Probe cancellation collects descendants before waiting for output pipes.
- Native tests cover immediate descendants and host termination after admission.
- Creation and Job assignment are separate operations; a crash between them can leave a suspended process.
- This boundary does not authorize Windows SDK file/network execution and is not an atomic creation/resource-recovery receipt.
- Native component gate: `node scripts/desktop/check-windows-sandbox.mjs --native`. With a specified local SDK, every required case must actually pass. CI runs elevated, so the CI gate covers platform-neutral MXC policy, the read-only file helper policy, junction rejection and settings/Bridge lifecycle; native ProcessContainer acceptance runs with `NEXUS_ALLOW_LOCAL_NATIVE_EVIDENCE=1` from a standard-user interactive session. Process exit must be shown by a kernel signal; a residual handle, a queryable PID/creation time or exit code 259 alone cannot prove liveness. Passing the gate does not grant Windows backend release acceptance.

## Windows backend

- The bundled Rust `nxs-mxc.exe` directly calls the pinned Microsoft mxc-sdk BaseContainerRunner. Go owns authorization only; the library owns PSEC encoding, negotiation and process lifetime. The helper and its license ship beside `nxs.exe`, are signed with the package, and require no user-installed Rust/MXC. No AppContainer or SBOX fallback is exposed.
- Write-only protections become read-only carveouts: commands and project discovery can read protected metadata but cannot write it. Local volume roots, their existing first-level directories and system directories are readable unless explicitly denied; writes remain grant-scoped.
- Windows 11 24H2 build 26100.9278 / 25H2 build 26200.9278 or a newer supported version is required. Unconfigured restricted initialization receives a three-minute budget; explicit budgets and Full Access remain unchanged.
- Windows nxs owns file-read workers per runtime rather than per agent. Unchanged read policies may reuse a read-only worker; policy changes, cancellation and runtime closure retire it. Explicit deny-read root identities participate in worker reuse and are checked before and after each request; changes or inspection failures reject the result and retire the worker. This is not an atomic filesystem snapshot. Restricted Windows sessions expose native PowerShell instead of Git Bash, whose MSYS object namespace is unavailable under the current policy. The contained bootstrap restores target TEMP/TMP to the existing granted scratch before tool startup. Ordinary shell failures affect only their own call; independent calls continue, while explicit user/session cancellation still propagates. Recorder persistence has a separate restricted project-directory executor and does not enlarge model tool grants. The SDK defines the worker protocol and bootstrap details in `docs/sdk/windows-mxc.md`.
- The Rust helper protocol and pinned Microsoft revision must match the Go host probe. The helper uses Microsoft's Job type to own itself before spawning; descendants inherit this outer kill-on-close Job while the official runner owns its inner Job. Native PSEC plus nested-Job acceptance remains required.
- PSEC API export presence and Windows build numbers are not availability guarantees. Default-deny ingress omits the optional table; explicit ingress/host-loopback allow negotiates schema 1.1 and NetworkIngress support, otherwise preparation fails without dropping policy. Create HRESULT 0x80070032 is presented as an unsupported creation request, not a missing downloadable dependency. Native diagnostics and enforcement acceptance remain separate.
- Explicit sandbox diagnosis exposes a safe, actionable message for a missing or mismatched `nxs-mxc.exe`, elevated tokens, unsupported PSEC contracts or creation requests, and a missing `processmodel.dll` API. These messages also offer the explicit Settings → Sandbox opt-out followed by a new session, explaining that command/file-tool desktop sandbox isolation is then disabled; the error never disables the preference automatically. Unknown diagnostics retain the generic state; raw diagnostic output is not exposed. Session startup preserves the original error while adding the same explanation.
- Windows 的显式 scratch 位于 `users/<owner>/runtime/tmp/sandbox/<workspace-hash>`，必须与 workspace 不重叠，通过 confinedfs 拒绝符号链接后创建；不再使用持久化租约、数据库恢复回执或启动回收门禁。PSEC 所需的合成 profile AppData 目录在启动前通过 confinedfs 创建。凭据拒绝目标若有不存在的中间目录，保护上移至第一个缺失目录，避免 PSEC 路径解析失败；不删除保护，也不创建用户凭据目录。scratch 与 workspace 具有相同会话共享边界，关闭只回收进程；此目录不包含宿主凭据。原生壳清理旧后端后、任何 runtime 启动前清理其 64 位十六进制 workspace 子目录；运行期间不清理共享目录，避免影响并发会话。

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
- Covers explicit Bash/PowerShell escape requests and ordinary external `Write`/`Edit` targets. Explicit command escape goes through the separate sandbox-bypass approval boundary. Restricted macOS nxs keeps command escape available behind this approval. Restricted Windows nxs sets `allowUnsandboxedCommands=false`: it has no command escape, and Full Access is its only opt-out.
- Nexus shows an explicit outside-sandbox explanation and offers only one-time approval.
- An approved file action gets a temporary helper-scoped write capability for the reviewed operation (including same-directory atomic staging) only while that helper runs. It is not a user directory whitelist and is not persisted in Agent settings.
- Persistent rules in a response are rejected by both Nexus and the SDK. IM cannot persist a grant when no scope suggestion exists.
- The SDK rejects an allow response that adds escape, or that changes the reviewed JSON input while staying outside the sandbox. Ordinary input edits and returning an action inside the sandbox keep existing behavior.
- Hidden paths, symlinked ancestors, read-only resource scopes and configured protected paths stay denied even when the user is asked to approve.
- The SDK captures the working directory before asking and uses it for Bash/PowerShell execution, including streaming entrypoints. A later session cwd update cannot redirect the approved relative command. The explanation includes this directory, and automatic-review human fallback preserves it. This captures session state; it is not an inode lease against filesystem renames.

## Stateless per-command execution (macOS)

- Like Codex's macOS backend, every sandboxed command or helper is a fresh `sandbox-exec` invocation with a policy computed for that call. The host keeps no launch records, policy receipts or crash-recovery state for the sandbox.
- Restricted nxs runs without a host resource contract. Writes are limited to the workspace, user-mounted directories, the SDK's temporary paths (`TMPDIR`, `/tmp/nexus`) and its debug/log locations; everything else is read-only unless explicitly denied.
- A command that fails inside the sandbox may be rerun outside it only through the SDK's one-time `sandbox_escape` approval (`allowUnsandboxedCommands=true`). Full Access remains the only standing opt-out.
- After connecting, the host checks that a desktop-sandbox runtime acknowledged every required Bridge capability and closes it otherwise (`internal/runtime/sandbox_admission.go`). Nothing about the effective policy is persisted.
- Without a resource contract, nxs points `TMPDIR` at `<workspace>/.nexus/sandbox-tmp`. Tools that ignore `TMPDIR` and write to the per-user Darwin temp directory or `/tmp` directly (for example a bare `mktemp`) are denied, as they were under the old scratch lease.
- Upgrading from the stateful model removes its leftovers once: SQL 00159 drops the launch, policy-receipt and scratch-recovery tables, and the file migration `20261029_remove_retired_sandbox_resources` deletes `app/processes` and the lease-marked leaves under `users/<owner>/runtime/sandbox`. Leaves are renamed to a tombstone before deletion so a partial failure is still recognized; any failure is logged and retried on the next start without blocking startup.
- A crash leaves no sandbox state that can block a later session; the Bridge's kill-on-close process ownership and ordinary session cleanup remain the only process boundary. Detached descendants are not supervised.

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
- 真实 Provider 验收记录由内部 CI 和本地维护，不随公开仓库发布。
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
- Reads and writes to the entire private app tree are denied, including descendants created after options assembly. Existing child aliases remain protected. State-root Control data and sidecar identity records are also read- and write-denied.
- Platform and host Skill projections live under `skills-public/{platform-skills,host-skills}`, outside the private app root. The host migrates old projections before runtime startup; conflicting copies stop migration without overwriting either. Public projections are readable but not writable by restricted tools. User/task grants cannot reopen the private app root.
- Host credential locations are read-denied: SSH/cloud credentials, `.npmrc`, `.pypirc`, Cargo credentials, `.claude`, `.claude.json`, `.codex`, and Chrome/Firefox/Edge profiles on macOS and Windows. These are the real host home paths; synthetic runtime profiles remain separate. Full Access is the explicit exception.
- nxs enforces this with its file sandbox. Claude additionally gets absolute Read/Edit rules for the same private read and whole-tree write scopes, on top of its command sandbox.
- New host secrets outside the app tree require extending the registry in `internal/runtime/clientopts/desktop_host_paths.go` before the path is introduced.
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
- Sandbox file requirements participate explicitly in the process-policy fingerprint, although ordinary settings serialization excludes these host-only fields.

## Host AutoDream and shutdown

- Host AutoDream uses the same Manager under an isolated `memory-maintenance:<agent>` key. Its owner-scoped background registration starts before the startup transaction, so host shutdown, owner cancellation and Agent revocation cancel maintenance. AutoDream is a control request, not a chat round; a bounded cancellation watcher retires the exact client, and failed cleanup keeps that session closed.
- Normal host shutdown closes Manager admission for clients, rounds and background tasks before releasing the App database, cancels round/background work, drains in-flight startup, then closes sessions in parallel. Repeated close calls wait for the same result; a caller timeout does not cancel shared cleanup or close the database while runtime writes remain possible.

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

**Package gate.**

- Bundled builds and packages run `nexus-server check-desktop-runtime --nxs <bundled path>` from the assembled App. It bypasses server startup, `.env`, database migration and model requests; the packaging runner supplies an empty environment and a fresh temporary HOME.
- The product options builder and the sidecar's linked Bridge must complete initialize and close for the restricted and Full Access profiles.
- There is no skip flag when nxs is bundled, including skip-build/skip-smoke packaging.
- The report stays outside the signed bundle; package metadata includes the binary SHA-256, Bridge version and confirmed profiles.
- An incompatible rolling-channel download stops the build before distribution. This is a package gate, not a runtime download or permission downgrade at user startup.

**Upgrade check.**

- `scripts/desktop/check-runtime-upgrade.mjs` takes explicit previous-release and candidate binaries and requires three phases: create with the old runtime, resume under current desktop policy, resume with the old runtime.
- A local Provider fixture verifies the actual model history, stable session ID and append-only transcript; settings, memory and workspace fixtures must stay unchanged.
- The old phase uses its supported pre-upgrade policy instead of asking the old runtime to advertise new security capabilities.
- It does not prove database downgrade, signed App installation, every historical release or an external Provider.

### 宿主实例锁

- macOS 桌面 `nexus-server` 在布局迁移前获取 canonical `app/sidecar.lock` 的非阻塞内核独占锁，持有到服务关闭后；锁文件不保存 PID，不按年龄删除，不 unlink，描述符 CLOEXEC。
- 第二个采用同协议的 sidecar 拒绝启动。该锁不覆盖旧版未持锁宿主，也不替代原生窗口锁。

### macOS 原生 sidecar 身份与旧记录

- 原生壳读取 `NexusSidecar.pid.json`。version 2 记录保存 PID、可执行路径、系统 boot session UUID 与 kernel audit token。
- 正常退出和孤儿清理都通过 `proc_signal_with_audittoken` 发信号，不回退裸 PID kill；发送前验证同用户、PID/version 与本次 boot。
- 信号调用返回正值时直接按该错误码处理，仅负值读取 errno；只接受成功或精确目标已不存在，其他失败保留恢复记录。
- 旧记录（只有 PID/path）：明确查无进程才清理记录；仍存活或观察未知时保留并拒绝启动，提示先退出旧版 Nexus；不因路径不同而删除。
- 新格式 PID 已复用或 boot 已变化时不向新进程发信号；同一内核进程但可执行路径改变时保留 unknown。
- 损坏、链接、非普通文件、读取或精确终止失败都不当作退出。记录只在当前内容仍匹配本次所有权时删除。
- 这保护原生 sidecar 身份，不替代 Go 实例锁与任务后代恢复。macOS App 最低系统版本 14.2（精确 audit-token 信号接口），更低版本由部署目标直接拒绝。

## 实现约束

- `NEXUS_SANDBOX_TEST_BINARY=/absolute/nxs make check-desktop-sandbox` 是显式桌面沙箱基线：脱离 go.work 验证固定 Bridge 与真实 nxs，缺失或跳过必测用例即失败。其中 `TestDesktopSandboxScriptedCommandBoundaries` 以脚本化模型夹具驱动真实 Bash 调用，断言 TMPDIR/workspace 可写、宿主 app 树与凭据位置被拒绝，不需要 Provider 凭据。
- macOS Desktop Build 工作流的 `sandbox_baseline` job 在 macos-15 上拉取 `nxs-stable` 配对运行时并执行上述基线；fork PR 读不到下载凭据时跳过。完整验收记录由内部 CI 和本地维护。

Windows scratch is shared only within the same workspace boundary. After the native shell has reaped the previous backend and before any runtime starts, the host removes disposable 64-hex workspace leaves under `users/*/runtime/tmp/sandbox`. It refuses linked parents, keeps unrelated names, and retries retained failures on the next startup. It never prunes during live sessions. Standalone backends without desktop parent ownership do not perform this cleanup.
