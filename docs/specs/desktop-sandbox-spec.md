# Desktop sandbox integration (experimental)

This document describes implemented host wiring, not acceptance of the full App
sandbox. Remaining design and delivery work is tracked in
[the development plan](../explorations/desktop-sandbox/development-plan.md);
the [documentation index](../explorations/desktop-sandbox/README.md) separates
the current contract, dated assessment, acceptance evidence and historical experiments.

## Activation and scope

Desktop execution derives the host sandbox contract from `NEXUS_APP_MODE=desktop`;
there is no user-facing or process-environment on/off switch. Server deployments
retain their existing runtime identity/isolation policy. Agent settings cannot
set or clear the internal host policy marker.

DM, Room and background memory maintenance use the same client-options builder.
For restricted approval modes, it requires nxs `required_sandbox_v1`,
`sandbox_file_tools_v1` and `sandbox_search_tools_v1` negotiation;
an unsupported runtime fails instead of silently accepting an unenforced policy.
Negotiation is not proof of current dependencies or an installed effective policy.
Native file-tool capability is currently declared only by macOS nxs builds;
Windows native execution is still incomplete and these desktop sessions fail
closed before receiving a task. A desktop session therefore never silently
falls back to an unrestricted backend when the selected contract is unavailable.

The host passes Skill directories as read resources and user-mounted directories
as explicit sandbox write grants. The SDK additionally grants its stable workspace
and compatibility paths under its mandatory execution policy. Project settings
cannot expand that mandatory policy. Unknown outbound shell proxy destinations use
the SDK's independent `sandbox_network` approval boundary when a mandatory runtime
has no explicit host network callback. Explicit denied domains and managed-only
domain policy remain authoritative. Default mode asks the user; auto mode uses the
existing independent reviewer and human fallback. Explicit command escape continues
through the separate sandbox-bypass approval boundary.

The pinned Bridge also exposes host-only `SandboxSettings.Resources` and the
independent `sandbox_resources_v1` contract for macOS command/file write scopes
and a host-prepared private scratch directory. Nexus does not yet supply this
object: runtime-owned scratch allocation, leases and cleanup must be integrated
before the product can use these scopes. The SDK-level tests do not establish
default product enforcement, whole-SDK IO confinement or an effective-policy
receipt. The progress and acceptance boundaries remain in the development plan.

The current mandatory macOS SDK applies explicit read/write and protected-path
movement denials after ordinary directory, device and PTY grants. A read grant
cannot reopen content in a denied same, parent or child root; symlink reads use
the same enforced boundary. `denyRead` and `denyWrite` remain separate, so secrets
requiring both protections must appear in both. Legacy non-mandatory read
carve-outs retain their existing semantics. These macOS guarantees do not prove
equivalent Linux/Windows behavior or confinement of all SDK IO. Capability
acknowledgement also does not establish that an older binary includes later
policy fixes; fixed-version acceptance remains separate.

Network approval carries the command input, tool-use identity, captured working
directory and exact host/port, bound to the command's permission epoch. Nexus shows
one pending connection and offers no persistent grant. Input changes or permission
updates in the response are rejected by the SDK; Nexus also rejects persistent
updates. Approval resumes the pending connection without replaying the command.
Effective permission changes invalidate pending and subsequent network requests
from the old command epoch. Explicit SDK host callbacks retain their existing API;
legacy non-mandatory runtimes do not acquire this new fallback.

Direct Bash/PowerShell background startup carries the same scoped callback when it
rebuilds execution options. Completion of the foreground call does not itself
cancel the background proxy's approval; changing the permission epoch still rejects
its pending connection. Native macOS Bash tests cover this continuation and
invalidation, not complete background review/session recovery on all platforms.

Closing an SDK execution proxy cancels its pending network callback contexts,
rejects late callback allows, and closes owned SOCKS/HTTP CONNECT tunnels. Complete
durable execution-effect recovery and background/session turnover acceptance remain
outstanding; these callback guarantees do not prove every process has terminated.

Mandatory SDK temporary-directory preparation uses a workspace directory handle
before the command sandbox starts, so descendant symlink replacement cannot redirect
the host's directory creation outside that root. Unsupported native backends reject
before temporary-directory or proxy allocation. These preparation guarantees do
not establish confinement for every other host-side filesystem operation.

Sandbox escape uses the typed `permission_boundary=sandbox_escape` classification.
Nexus shows an explicit outside-sandbox explanation and exposes only one-time
approval. Persistent rules supplied in a response are rejected by both Nexus and
the SDK; IM cannot persist a grant when no scope suggestion exists. The SDK also
rejects an allow response that adds escape or changes its reviewed JSON input while
remaining outside the sandbox. Ordinary tool input edits and returning an action
inside the sandbox retain their existing behavior. Unknown boundary classifications
are rejected before a pending approval is created.

For sandbox escape, the SDK captures the working-directory path before asking and
uses that path for Bash/PowerShell execution, including their streaming entrypoints.
A later session cwd update cannot redirect the approved relative command. The
approval explanation includes this directory, and automatic-review human fallback
preserves that explanation. This captures session state; it is not an inode lease
against arbitrary filesystem renames.

The host separately requires native Read/Write/Edit coverage through
`SandboxSettings.RequireFileTools` and initialize `required_sandbox_file_tools`.
The SDK must acknowledge `sandbox_file_tools_v1` as well as the command contract;
an older binary with only command acknowledgement is rejected before any task
or internal continuation is sent. File content, directory suggestions, link/metadata
and freshness checks use the restricted file executor, with no direct-IO fallback
on preparation or execution failure. This contract does not cover Glob/Grep,
Notebook, startup settings, Skills, background memory or the entire SDK process.
Normal settings cannot substitute for this host requirement, and changing it
requires runtime replacement. It is a coverage requirement, not a user sandbox toggle.

The host additionally sets `SandboxSettings.RequireSearchTools` and initialize
`required_sandbox_search_tools`. An older SDK that confirms commands and
Read/Write/Edit but lacks `sandbox_search_tools_v1` is rejected before task writes.
Search coverage is currently macOS-only: Glob/Grep path checks, missing-path
suggestions, rg and result metadata use the same restricted file environment.
The auxiliary process uses a minimal environment and denies network access,
including when the runtime resolves a custom rg executable. If a ResourcePolicy
is supplied, search uses the same write scope and scratch. Preparation, execution,
cancellation and output-limit failures do not fall back to direct host IO or retry
through the ordinary runner. Restricted searches return complete results or an
explicit failure; single-file content/count results retain their filename.
This independent requirement also participates in process-policy identity and
requires runtime replacement when changed. It does not cover Notebook, startup
configuration, Skills, background memory, full descendant supervision or an
effective-policy receipt; the older file capability retains its original scope.

The host also requires `SandboxSettings.RequireMediaFiles`, initialize
`required_sandbox_media_files` and the separate `sandbox_media_files_v1`
acknowledgement. Current macOS coverage includes local reads for ViewImage and
main-model preprocessing: local paths, file URLs, symlinks, deferred references,
user images and nested tool-result images all use the file executor. Local paths
are materialized before provider dispatch. Preparation and read failures do not
fall back; local access is checked before auxiliary analysis cache lookup.
Old command/file/search acknowledgements cannot substitute for this capability.
It participates in process-policy identity and requires runtime replacement when
changed. It does not cover HTTP image downloads, remote URL forwarding policy,
Claude or whole-SDK IO; those remain separate implementation and acceptance work.

The host separately requires `SandboxSettings.RequireSkillFiles`, initialize
`required_sandbox_skill_files` and `sandbox_skill_files_v1`. Current macOS
coverage includes initial/model/Slash catalogs, Skill bodies, Read-triggered
dynamic discovery, Git ignore queries and remember-availability settings.
They share the captured file context and cwd, including metadata and symlinks.
Allowed project/user/additional sources and conditional/Git ignore behavior
remain supported. Git uses a minimal environment and denies network; cancellation,
unknown exit results and preparation failures cannot trigger host IO fallback.
Uncertain dynamic observations can be checked again on a later file access.
The requirement participates in process identity and requires replacement when
changed. Global startup settings, hooks, background memory, effective-policy
receipts, other platforms and Claude remain separate acceptance work.

The host separately requires `SandboxSettings.RequireContextFiles`, initialize
`required_sandbox_context_files` and `sandbox_context_files_v1`. Startup and
compact instruction loading, dynamic instruction discovery, and recent-file
restoration use the file execution boundary for contents, metadata, directories,
symlinks and instruction exclusion settings. A denied optional instruction is not
injected. Unreadable or malformed selected exclusion settings stop startup or
reload, rather than removing the exclusion policy. Failed reloads clear stale
instructions and prevent the next model request until reading recovers. Query,
manual compact and child-agent dispatch preserve the current cancellation context;
startup/reload and compact file restoration have bounded total read time.
This requirement participates in process identity. Global permission/provider
settings, project definitions, hooks, persistence, background IO, effective-policy
receipts and other runtime/platform acceptance remain separate work.

The host separately requires `SandboxSettings.RequireProjectFiles`, initialize
`required_sandbox_project_files` and `sandbox_project_files_v1`. Before tool
assembly, project discovery uses the file boundary for user/project Agent and
command definitions, project Skill definitions, and selected hook-setting files.
Directory entries, metadata, symlinks and contents share the same worker. Missing
settings are allowed; denied, canceled or malformed settings reject the whole
snapshot. Failed refresh clears the catalog and prevents subsequent model
requests. Agent/hook changes require a new runtime because they are bound during
assembly; a successful refresh may update Slash bodies directly. This requirement
participates in process replacement. Global permission/provider and managed-policy
loading, persistence and hook execution remain separate work.

The host also requires `SandboxSettings.RequireManagedPolicy`, initialize
`required_sandbox_managed_policy` and `sandbox_managed_policy_v1`. Before settings
environment projection, nxs fixes the managed root and an immutable policy
snapshot. All later policy consumers use that source, including child runtimes.
Malformed JSON, invalid known safety-field types, unreadable files and effective
policy changes block query, manual compact, tool dispatch, file-context preparation
and permission updates. The original effective policy can be restored; applying a
new policy requires runtime recreation. The control-plane reader accepts regular
files up to 16 MiB each, with nonblocking Unix open and descriptor type validation.
Required execution excludes task settings before any such read. This requirement
participates in process identity and currently acknowledges the macOS backend.
Ordinary settings and credentials, permission-persistence concurrency, hook
execution, background IO and effective-policy receipts remain separate work.

The host additionally requires `SandboxSettings.RequireSettingsFiles`, initialize
`required_sandbox_settings_files` and `sandbox_settings_files_v1`. nxs fixes the
config root and selected sources before profile projection. Required execution
uses the file worker for ordinary user/project/local/flag settings, filters disabled
sources before IO, applies a 16 MiB limit to every selected document, and rejects
incomplete or invalid snapshots. Runtime consumers share a bound snapshot; child
runtimes keep independent logical snapshots. Source changes or read errors block
query, compact, tool dispatch, file-context preparation and settings controls or
permission updates. Restoring the original content permits recovery; new file
contents require runtime recreation. `get_settings` uses the bound flag sources.
Dynamic updates reject fields whose execution configuration is static. This
requirement participates in process identity and currently acknowledges macOS.

Restricted desktop sessions also require `SandboxSettings.RequireSettingsWrites`,
initialize `required_sandbox_settings_writes` and `sandbox_settings_writes_v1`.
The write contract depends on required sandbox, file tools and settings files, and
is admitted only for nxs. Config and permission updates share the checked Binding,
physical directory identity and process-local write transaction. Existing or newly
created parents must remain real directories beneath the fixed physical root;
symlink swaps, directory generation changes, special files and read-only targets
fail closed. A single document is replaced through a same-directory temporary file,
and task sandbox rules deny both lexical and physical aliases of protected settings
and temporary names. Multi-document permission updates use deterministic order and
mark the shared store unknown after a partial commit.

Config writes canonical nested settings keys. Explicit SDK Options and process
environment values retain their higher precedence, so a persisted value is a
settings default rather than proof of the effective runtime value. A real Config
change marks the runtime for recreation; the query loop checks this fence before
every provider turn, while a no-op leaves the runtime usable. WebFetch also checks
the binding immediately before calling its selected summary provider, environment
endpoint or host adapter, including changes during page retrieval. Initialization
reserves its admission state when enqueued; at most 32 ordinary stream messages
wait for success, and initialization failure discards them without creating a
base-config Session. The requirement and its host-only flag participate in Bridge
and Nexus process identity.

The write contract currently acknowledges native macOS only. It does not establish
cross-process locking or CAS, multi-file atomicity, parent-directory fsync or
power-loss durability, exact request/approval/revision persistence, durable receipts,
restart recovery of unknown outcomes or automatic replay. Unix replacement preserves
ordinary permission bits but does not claim owner, ACL, xattr or file flags. Windows
only has compile coverage and its Go writable-bit checks do not establish DACL
privacy. Whole-process Provider credential isolation and full background IO confinement remain pending.

## Host-owned Provider inputs

Nexus finalizes `NEXUS_PROVIDER_MANAGED_BY_HOST=1` and host-owned AutoDream wake
after every environment merge. `ExtraEnv` and `ConfigurationEnv` cannot revoke
these nxs-specific declarations; Claude does not receive an nxs ownership claim.
Provider ownership, subprocess scrub and background-wake declarations are part of
the process-policy fingerprint. Changing them replaces the old process before
reconfiguration; ordinary Provider credential rotation remains a hot update.
The fixed SDK checks Provider ownership before projecting ordinary settings.
In host-managed mode, settings cannot supply Provider/main/fallback/background
models, vision routes, credentials, custom headers, request-body overrides,
proxy or certificate inputs. Explicit host Options/environment remain authoritative,
and ordinary task environment values remain available. Standalone SDK settings
retain their existing routing semantics. Background-model settings updates that
cannot take effect in host-managed mode return an error.

Command and hook environment builders remove known SDK main/auxiliary credentials
after applying runtime environment values. Task values cannot disable a Provider
ownership declaration already present in the host process. This is a versioned
implementation guarantee checked by the fixed-source baseline, not a new wire
capability inferred from settings-write acknowledgement. It does not establish
credential secrecy against host-file reads, process inspection, inherited handles,
external MCP or network egress; those boundaries and default product activation
remain separately pending.

HTTP hook Header interpolation also uses the task-visible environment, even when
the hook allowlist names a Provider credential. nxs MCP configuration interpolation
treats such process credentials as missing, including URL, argument, environment
alias and Header locations, and preserves the existing missing/fallback semantics.
Dedicated hook/MCP authentication variables and explicit host-provided values
remain independent. This prevents implicit credential borrowing; it does not
establish confinement of external MCP servers. The MCP registry passes the
runtime-owned environment into `headersHelper` and refreshes it on an environment
update; managed helpers cannot read known Provider credentials or redirect the
managed memory root. This remains an environment-source boundary, not an OS
boundary for external MCP processes.

For nxs, Nexus fixes `NEXUS_MEMORY_DIR` to the current Agent workspace after all
configuration capability merges and clears remote-memory overrides. The SDK typed
memory profile applies the same rule to Summary, AutoMemory and AutoDream
consumers. These ownership inputs participate in the process-policy fingerprint,
so a changed root cannot reuse an old runtime. Claude does not receive this
nxs-specific ownership claim.

The [dated assessment](../explorations/desktop-sandbox/current-assessment-2026-09-15.md)
records the verified SDK baseline and its remaining IO paths. MCP servers,
Connectors and the desktop UI retain their separate authorization. The feature
must not be represented as fully accepted App isolation.

## Approval modes and runtime replacement

The host does not change the selected approval mode to enable sandboxing. A fresh
Full Access (`bypassPermissions`) runtime receives no additional host sandbox
policy; existing legacy settings and OS permissions retain their prior semantics.
This does not grant OS administrator privileges or override domain authorization.

For host-managed desktop policy, a live change crossing into or out of Full Access
retires the old client before returning the transition signal. DM closes the old
session; Room cancels the exact slot and its pending approval requests, retires the
client, and waits for cleanup. A confirmed close is an expected mode transition;
cleanup failure is reported. A subsequent request constructs fresh options and
the existing process-policy fingerprint requires replacement of the old runtime.
HTTP updates process both DM and Room even if one domain fails; their errors are
reported together instead of leaving the later domain on its old policy.

Changing between restricted approval modes keeps the sandbox boundary and uses
the existing permission-mode update path. This document does not claim complete
approval-revision invalidation across every host callback yet.
For SDK manual/automatic permission callbacks, effective rule/mode changes cancel
the pending policy epoch and invalidate even a late allow response. Identical
refreshes preserve the pending epoch. A cancelled request cannot create a fresh
Nexus pending prompt. The nxs proxy preserves human-only requirements and review
evidence; the reviewer treats sandbox escape as additional execution authority.

Mode changes do not resubmit a prompt or repeat a tool invocation. Stopping a
process does not roll back prior side effects; an unknown or partial command result
must not be interpreted as permission to replay it. Bridge cleanup waits for
transport exit, not merely stream closure. This confirms the runtime main process;
full descendant cleanup needs the remaining platform backend integration.

Bridge returns `ProcessCleanupError` for failed process-session cleanup, including
after main-process success, forced termination and repeated closes. Nexus preserves
this error even when joined with an ordinary closed-pipe error. Failed client
cleanup blocks reconnect and stale-startup retry; Manager retains the exact failed
session in closing state rather than publishing a replacement. Explicit close and
owner/Agent close callers can read the retained result. Sandbox file requirements
and resource scopes participate explicitly in the process-policy fingerprint,
even though ordinary settings serialization excludes those host-only fields.

The current failed-close fence exists in memory. It does not survive a host restart
or prove that detached descendants have terminated. The Bridge Unix sweep observes
only visible members of the original session; another session, PID namespace or
host signal callback needs its own supervision and exit proof. This change does
not create, lease or reclaim scratch, and must not be used as a complete cleanup
receipt for the future default resource policy.

## Explicit local diagnostics

The runtime settings page offers an explicit sandbox-support check. Only
`GET /settings/runtime/nxs/status?include_sandbox=true` launches the bounded Bridge
query; the existing request without this option remains a file-only check used
when selecting nxs. Responses keep `available` independent and optionally include
`sandbox.state`: `unknown`, `unsupported`, `missing_dependencies`, or
`dependencies_available`, plus a known platform. The latter state does not mean
sandboxing is enabled or tested for the active task. Failures remain unknown and
do not change preferences, approval mode, or execution policy.

The Bridge module version and checksum are owned by `go.mod` and `go.sum`.
Dependency publication, the configured nxs binary, and packaged application
acceptance are separate delivery facts, recorded in the
[assessment](../explorations/desktop-sandbox/current-assessment-2026-09-15.md)
and [acceptance matrix](../testing/desktop-sandbox-acceptance.md). Dependency
availability must not be inferred from a successful local workspace build.

## Main integration compatibility

The local pinned Bridge `v0.1.34-0.20260916063139-6325d2acc450` combines the sandbox requirements above with main's MCP call-context contract: runtime `params._meta["claudecode/toolUseId"]` reaches the host callback unchanged. Missing metadata remains empty; business arguments cannot supply this identity. The combined module is locally verified and unpublished. [Acceptance evidence](../testing/desktop-sandbox-acceptance.md#2026-09-16main-同步与-bridge-兼容) records this integration separately from default sandbox rollout and platform acceptance.
