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
For restricted nxs sessions, it requires nxs `required_sandbox_v1`,
`sandbox_file_tools_v1` and `sandbox_search_tools_v1` negotiation. Restricted
Claude sessions use the separate Bridge `RequireClaudeNativeSandbox` contract
and generated native `sandbox` settings (`enabled=true`,
`failIfUnavailable=true`, `allowUnsandboxedCommands=false`); they do not claim
nxs capabilities or use `--restricted` tool removal as a command-sandbox proof.
An unsupported runtime fails instead of silently accepting an unenforced policy.
Negotiation is not proof of current dependencies or an installed effective policy.
Native file-tool capability is currently declared only by macOS nxs builds;
Windows native execution is still incomplete and these desktop sessions fail
closed before receiving a task. A desktop session therefore never silently
falls back to an unrestricted backend when the selected contract is unavailable.

The Windows Bridge lifecycle is implemented independently of SDK sandbox capability:
runtime and CLI probe processes start suspended, join a kill-on-close Job, then resume
their validated initial thread. Probe cancellation collects descendants before waiting
for output pipes. Native tests cover immediate descendants and host termination after
admission. Creation and Job assignment are still separate operations; a crash between
them can leave a suspended process. This lifecycle boundary does not authorize Windows
SDK file/network execution or establish an atomic creation/resource-recovery receipt.

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
and a host-prepared private scratch directory. Nexus now prepares owner/runtime
scoped leases for desktop nxs DM, Room and background memory maintenance and
releases them only after a confirmed Bridge close; each acquisition is an
independent handle over the shared resource, so preparation failure or an old
runtime generation cannot release a newer holder's scratch. Failed cleanup keeps
the runtime fence and exact lease for recovery and blocks new acquisition in that
scope. Durable markers and explicit stale-sweep primitives exist. A failed close
persists `cleanup_unknown`, its bounded error summary and update time through the
fixed lease directory handle; discovery exposes that state after a host restart,
while deletion still requires an explicit owner-scoped sweep; a `cleanup_unknown`
marker is retained even when its recorded PID is dead until a separate
reconciliation can prove the full runtime boundary is closed. Connect writes an
effective-policy receipt for each runtime generation to the host database and
keeps a clone in memory for the connected session. The receipt records the
required/acknowledged Bridge capabilities, policy digest, session identity and
(when present) exact scratch lease/round identity, with durable `confirmed`,
`retiring`, `retired` and `unknown` lifecycle phases. A process restart can read
the latest owner-scoped receipt, but a persisted receipt never represents a
currently connected runtime or grants permission. This receipt is host
diagnostic evidence, not proof of whole-SDK IO, OS descendants, network, secrets
or native platform isolation. Lifecycle updates are monotonic: a late callback
cannot reopen `retired` or `unknown` as `retiring`. There is currently no
automatic or browser-triggered receipt reconciliation to `reconciled`; an
`unknown` receipt remains unknown until a future control surface can prove the
complete runtime boundary. Within one owner/session/generation, the receipt
payload and `confirmed_at` are immutable; a duplicate connect observation may
refresh only `updated_at`, and a terminal row ignores late payload retries.
Automatic crash sweep, whole-SDK IO confinement and native platform acceptance
remain separate work.

Where the platform exposes a safe process-identity query, the marker also
records `process_start_time_unix_nano`. Windows recovery compares that value
with `GetProcessTimes` before treating an ordinary marker as stale, so a reused
PID cannot authorize cleanup. A permission or query failure remains unknown;
`cleanup_unknown` always takes precedence. Older markers and platforms without
this identity probe retain the conservative PID-liveness behavior.

The owner process reaper is part of that same close boundary. If Bridge close
has already reported `retired` but the owner-level reaper fails, the host
conservatively downgrades the exact generation back to `unknown` and records a
bounded reason. A clean Bridge close therefore never hides descendants that
the host could not prove were collected.

A fresh Claude connection may not publish its session identity until the first
user turn; its receipt is marked provisional until that identity is available.
During startup configuration changes, cleanup uses the exact captured lease
handle and keeps the current client lease available for retry. Lifecycle
invalidation reuses an existing cleanup owner for that handle rather than
transferring it twice. If the handle that first records `cleanup_unknown` is
released while sibling handles still reference the same resource, the cleanup
fence transfers to one live sibling; it cannot become attached to an already
released handle or be silently dropped.

The desktop settings API exposes this recovery boundary through
`GET /settings/runtime/sandbox/resources` and
`POST /settings/runtime/sandbox/reconcile`. Both routes derive the owner from
the authenticated request and never accept an owner or filesystem root from the
caller. Inspection is read-only. Reconcile requires a positive
`older_than_seconds`; it is a dry run unless the request body explicitly sets
`apply=true`. Runtime checks still retain active, unknown, malformed and
`cleanup_unknown` markers, so the endpoint does not turn a dead PID into proof
that descendants and handles are gone. This is a local diagnostic/recovery
surface, not a substitute for native platform acceptance. `GET
/settings/runtime/sandbox/receipt?session_key=...` first exposes the current
owner-scoped connected generation's receipt; after a restart it falls back to the
latest durable receipt for that exact owner/session. A missing, closing,
cross-owner or non-desktop generation with no durable row returns not found. The
receipt's capability and lease fields remain admission evidence and never attest
OS or whole-SDK IO isolation.

This resource contract belongs to nxs only. When a host resource lease is
present, Nexus forces `allowUnsandboxedCommands=false` and rejects explicit
write-directory grants under the read-only scope; the Bridge rejects the
contradictory combination before transport startup. Claude's native sandbox
settings never receive an nxs resource lease, and accidental cross-backend
mixing fails closed. An active owner/session lease also keeps its write scope
immutable; a later round cannot silently widen or narrow the policy by reusing
the same scratch path.

The host creates and removes the scratch parent and lease directory through
`internal/infra/confinedfs` fixed directory handles. Marker reads and
stale-resource scans reject replaced parents, symlinks, non-regular marker
files, and (including on Windows through the opened file handle) hard-linked
marker identities; a cleanup failure keeps the exact lease registered instead
of treating a redirected path as success.

The current mandatory macOS SDK applies explicit read/write and protected-path
movement denials after ordinary directory, device and PTY grants. A read grant
cannot reopen content in a denied same, parent or child root; symlink reads use
the same enforced boundary. `denyRead` and `denyWrite` remain separate, so secrets
requiring both protections must appear in both. Legacy non-mandatory read
carve-outs retain their existing semantics. These macOS guarantees do not prove
equivalent Linux/Windows behavior or confinement of all SDK IO. Capability
acknowledgement also does not establish that an older binary includes later
policy fixes; fixed-version acceptance remains separate.

macOS nxs file approval recognizes both the configured workspace root and its
canonical spelling. Read, search and ordinary file edits keep their existing
local approval behavior when a resumed or switched backend supplies the canonical
path (for example `/private/var` for a `/var` workspace). Explicit ask/deny rules
match both spellings; descendant symlinks, hidden writes and protected write globs
retain their checks. This preflight does not rewrite the tool input or replace
the execution-time file sandbox, and it makes no additional Windows claim.

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
startup settings, Skills, background memory or the entire SDK process.
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
Claude or whole-SDK IO; those remain separate contracts.

Desktop nxs also requires `RequireMediaNetwork`, initialize
`required_sandbox_media_network` and `sandbox_media_network_v1`. This separate
contract requires command, file and local media capabilities. All remote image
sources are downloaded before dispatch to the main/auxiliary Provider, including
deferred references and nested tool images; Provider URL support cannot bypass
the captured policy. Every HTTP request and redirect is admitted against the
network policy. Deny and managed-only rules remain authoritative. Explicit
ViewImage network approvals bind the exact input, tool-use, cwd, destination
and permission epoch, reject input changes and persistent grants, and cannot
survive cancellation or a policy change. Preprocessing without a tool identity
uses existing network grants or an explicit host callback. Environment proxies
and Provider credentials are not inherited; configured host proxies remain
supported. Cleanup cancels pending approvals and body reads. This contract does
not extend to model Provider transport, WebFetch, external MCP or Claude.

The host separately requires `SandboxSettings.RequireNotebookFiles`, initialize
`required_sandbox_notebook_files` and the separate `sandbox_notebook_files_v1`
acknowledgement. Notebook content and cell outputs are parsed only after the
local bytes have been read through the restricted file executor. The requirement
depends on the command and native file contracts, participates in process-policy
identity and currently acknowledges only the macOS nxs backend. Old command,
file, search or media acknowledgements cannot substitute for it. This is a local
read/parse guarantee; Notebook execution, remote networking and whole-SDK IO
remain separate work.

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

The paired nxs build also routes memory recall and extraction manifests through
the current file executor. Directory discovery, link metadata, frontmatter and
selected content use that boundary; preparation or missing ports never fall back
to host IO. It preserves non-following directory traversal and re-reads selected
content after the selector, so replacement with a denied symlink is rejected.
Header and body reads use bounded prefixes enforced by both the helper and its
caller; ordinary full-file streaming retains its existing large-file behavior.
The existing 200-line/4096-byte per-memory and 60KB per-session injection budgets,
workspace paths and selection semantics remain unchanged. Recall and manifest
reads each have a 30-second total deadline, and canceled recall cannot publish a
partial attachment. This is an internal fix in the jointly released Nexus/nxs
pair, not a new capability or a broader `sandbox_context_files_v1` guarantee.
The paired build additionally routes memory-store initialization and Summary file,
template, prompt and compact input through the current file executor. Initial files
use exclusive creation: existing or concurrently created content is preserved,
unknown results stop the current operation without replay or path-based deletion.
Only confirmed absence permits initialization or built-in template fallback; denied
reads are errors. Summary preparation has a 30-second IO deadline, and model edits
continue through the existing exact-file Edit permission and sandbox. Exclusive
creation does not promise atomic content publication or power-loss transactions.
Read-only resource sessions remain usable and can read existing memories, but do
not initialize the layout or schedule AutoMemory, Summary or AutoDream persistent
updates. AutoDream scheduling also uses the current file executor for completion
timestamps, transcript directory discovery and each candidate's target metadata,
with a shared 30-second deadline. Only missing histories are empty; denied reads,
other IO errors or cancellation return no partial candidates, start no maintenance
and do not advance the successful scan interval. Files removed during a scan are
skipped, and non-regular targets are not accepted as markers or transcripts.
Memory writer locks are not reclaimed solely because their file is over an hour
old. A known holder must be confirmed exited; a live/current holder, missing probe
or malformed/partial record retains the lock. Permission and unsupported probe
errors are not exit evidence, and process-observation handles are released.
Controlled AutoDream lock/completion writes, replacement-safe lock ownership,
other transcript auxiliary reads and remaining IO stay separate work; these
internal fixes add no capability claim.

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

The write contract currently acknowledges native macOS only. The pinned SDK adds
per-root cross-process locks, post-lock snapshot checks, parent-directory syncing,
and reverse-order rollback when the changed documents can still be identified.
It does not establish multi-file power-loss atomicity or durable SDK execution
receipts. Nexus configuration-control receipts have their own unknown recovery,
human review/reconcile and stable revision contract in the
[configuration specification](conversational-configuration-control-spec.md);
they do not substitute for SDK file-transaction evidence. Neither path replays an
unknown write automatically. Unix replacement preserves
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

For Anthropic-compatible third-party models, the host-owned `BaseURL` is
projected as `ANTHROPIC_BASE_URL`. nxs projects the host-owned `AuthToken` through
the SDK's `ANTHROPIC_API_KEY` path for both first-party and compatible endpoints, which
emits `x-api-key` and the SDK's compatible-endpoint Bearer fallback. Claude
keeps `ANTHROPIC_AUTH_TOKEN` for its native CLI semantics. In addition to the
earlier local mock SSE and fixed SDK header checks, the 2026-09-27 live-provider
acceptance exercises both projections against one real third-party gateway,
including file/command execution, explicit denials and ordinary interruption.
This does not establish arbitrary gateway or official account/OAuth compatibility.
Provider-specific custom headers still have no declared Nexus field and are not
accepted by this contract.

The nxs `Sandbox.Network` object is currently consumed by command/tool
execution (including shell network preflight) and is not a host-level egress
firewall for the model Provider transport. Nexus therefore does not silently
add the resolved Provider host to `DesktopSandboxNetworkAdmission`; Provider
reachability is an input-ownership and host-integration guarantee only. A
complete OS-level Provider egress boundary still requires platform/Bridge
evidence and remains outside this receipt.

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

## Remote MCP endpoints on macOS

Nexus and bundled nxs are released as one application. Compatibility acceptance concerns existing user data, configuration, sessions and working features. The package handshake checks the contents assembled for that release; normal users do not manage a separate runtime upgrade.

macOS nxs options now require `sandbox_mcp_network_v1` through `RequireMCPNetwork` and `MCP.StrictConfig`. Both persisted Agent HTTP/SSE configuration and typed Connector servers are explicit host inputs. A configured endpoint receives a separate grant for its scheme, host and port; redirects and legacy SSE POST endpoints cannot leave that origin. This does not add the MCP domain to command/image network allowlists. Explicit denied domains and managed-only domain restrictions still apply. Task-settings HTTP/SOCKS/MITM proxy routes are rejected before connecting; MCP credentials require a separate host-owned proxy contract, and the runtime must not silently bypass an explicit proxy. The other platform admission paths and Claude's native behavior are unchanged by this macOS contract.

Each request and response body is canceled when its connection is retired or its permission epoch changes. Removal, disable, replacement and session shutdown retire owned connections; a delayed discovery cannot revive an old configuration. Failed or canceled operations are not automatically replayed. Connection failures are scoped to their MCP server, so an unavailable remote endpoint does not stop the entire Agent. macOS nxs options also require `sandbox_mcp_helpers_v1` through `RequireMCPHelpers`. Persisted and Connector `headersHelper` configurations use the current command sandbox, configuration checks and filtered task environment. They do not inherit the MCP endpoint grant or tool approvals. Each request refreshes authentication after network admission; invalid output or execution failure stops that request without using stale/static credentials. Helpers have a 10-second execution deadline, 64 KiB stdout and 16 KiB stderr limits; stderr is never included in service errors. Permission changes and connection retirement cancel in-flight helpers; session close also waits for owned helper cleanup and propagates failures. Ordinary process-group cleanup is tested, while independently detached descendants, OAuth discovery/token exchange and model Provider networking remain separate boundaries.

Nexus also requires `sandbox_mcp_stdio_v1` / `RequireMCPStdio` on macOS. Persisted command configurations (including inferred stdio type) and typed Connector configurations now start actual services through the command sandbox. The executor owns argv execution, pipes, configuration checks, permission epochs and process/proxy cleanup; the MCP client owns bounded JSONL and unique IDs for concurrent replies. Cancellation or timeout retires the whole service and its other pending calls, with no replay. Same-name replacement waits for the previous process; session close awaits all owned stdio/helper processes and preserves cleanup errors. Inherited Provider credentials are filtered before explicit service credentials are added; reserved runtime/home/temporary-root environment fields cannot override host policy. Network access uses command policy and its proxy, without endpoint grants or current tool approvals. Messages are capped at 10 MiB and pending stdio requests at 64; stderr is drained without retaining credential-bearing logs. HTTP/SSE also bound aggregate event and JSON body sizes. This covers ordinary process groups; detached descendants, host crash recovery, trusted MCP proxy routes and full secret/handle isolation remain separate acceptance work.

## Approval modes and runtime replacement

The host does not change the selected approval mode to enable sandboxing: the
restricted runtime is already part of every desktop task contract. A fresh nxs
Full Access (`bypassPermissions`) runtime still installs the nxs capability and
lifecycle boundary; it broadens the command/file resource policy through the
SDK setting instead of disabling the runtime. This does not grant OS
administrator privileges or override domain authorization. A restricted Claude
session installs Bridge's typed `RequireClaudeNativeSandbox` contract, which
generates one host-owned `--settings` object and rejects missing, duplicate or
overridden JSON, bypass permissions, `--restricted` tool-mode mixing, and any
unsandboxed-command setting before transport startup. Bridge also probes the exact
resolved CLI with `--settings <generated-json> --help` using a bounded timeout,
bounded output and scrubbed environment; a rejected or unadvertised settings
entry point prevents startup. Help output does not attest that the policy is
effective. The corresponding
`CapabilityClaudeNativeSandbox` is a local Bridge configuration capability, not
a Claude wire response or proof of OS/file/network/Provider isolation. Claude
Full Access is an explicit exception and does not install the contract or
settings; it still retains host lifecycle, domain authorization, and other
mandatory policy. Fixed CLI-version, native behavior, and clean-host acceptance
remain separate release evidence.

The recorded Claude CLI 2.1.273 help describes `--restricted` as removing code
execution tools and limiting file tools to the working directory. The desktop
contract therefore uses Claude's native command sandbox settings so Bash and
build commands remain available. Bridge settings validation is implemented;
Claude's effective-policy admission and real allowed/denied command, network,
credential, cancellation and cleanup tests remain separate unfinished
integration work. Native Windows Claude is rejected until a supported native
environment is verified.

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

Fresh Manager client creation reads the latest receipt for the exact owner/session
before invoking the factory. A retired or explicitly reconciled receipt provides
the generation lower bound, so clean App restart or idle-session recreation cannot
reuse an old durable identity. Confirmed, retiring or unknown history without the
original live client blocks recreation; read/identity failures also stop startup.
This check applies before choosing the new runtime, so a backend or Full Access
change cannot bypass unresolved previous execution. It does not infer an exit
from an absent in-memory client or automatically replay a request.

Normal host shutdown closes Manager admission for clients, rounds and background
tasks before releasing the App database. It cancels existing round/background
work, drains in-flight startup and receipt insertion, then closes sessions in
parallel through the existing cleanup and receipt lifecycle. Repeated close calls
wait for the same result. A caller timeout does not cancel shared cleanup or close
the database while runtime writes remain possible. This orderly exit path does
not clear receipts left unresolved by a crash or failed descendant cleanup.

Scratch allocation independently checks persisted markers through its fixed parent
directory handle. The same owner/session's cleanup-unknown marker, including one
under an older replacement path, blocks a new lease after restart. Invalid markers
at that scope's expected path also block allocation. Unrelated sessions remain
independent; concurrent preparation in one host cannot misread a half-written marker.
These are durable startup fences, not proof that detached descendants terminated.
The Bridge Unix sweep still observes only visible members of the original session;
complete supervision and safe reconciliation of uncertain execution remain separate.

## Explicit local diagnostics

The runtime settings page offers an explicit sandbox-support check. Only
`GET /settings/runtime/nxs/status?include_sandbox=true` launches the bounded Bridge
query; the existing request without this option remains a file-only check used
when selecting nxs. Responses keep `available` independent and optionally include
`sandbox.state`: `unknown`, `unsupported`, `missing_dependencies`, or
`dependencies_available`, plus a known platform. `dependencies_available`
means the default local prerequisites for the restricted runtime are present;
task admission still confirms the exact negotiated capability and effective
policy before starting. Failures remain unknown and do not change preferences,
approval mode, or execution policy. This diagnostic never acts as an on/off
control.

The Bridge module version and checksum are owned by `go.mod` and `go.sum`.
Dependency publication, the configured nxs binary, and packaged application
acceptance are separate delivery facts, recorded in the
[assessment](../explorations/desktop-sandbox/current-assessment-2026-09-15.md)
and [acceptance matrix](../testing/desktop-sandbox-acceptance.md). Dependency
availability must not be inferred from a successful local workspace build.

## Main integration compatibility

### macOS App/runtime pairing and existing data

Nexus and nxs ship together in the macOS App. Normal App upgrades replace the
matched pair; there is no separate runtime upgrade step for existing users.
The existing development/environment override precedence remains unchanged.
The compatibility requirement is that the new pair preserves existing settings,
sessions, memory, workspaces and previously supported product behavior.

Bundled builds and packages run `nexus-server check-desktop-runtime --nxs <bundled path>`
from the actual assembled App. The diagnostic bypasses server startup, `.env`, database
migration and model requests; the packaging runner supplies an empty environment and
fresh temporary HOME. The product options builder and sidecar's linked Bridge must
complete initialize and close for workspace-write, read-only and Full Access. There is
no skip flag for this check when nxs is bundled, including skip-build/skip-smoke packaging.
The report stays outside the signed bundle; package metadata includes the binary SHA-256,
Bridge version and confirmed profiles. An incompatible rolling-channel download stops
the build before distribution. This is a package compatibility gate, not an automatic
runtime download or permission downgrade at user startup.

`scripts/desktop/check-runtime-upgrade.mjs` accepts explicit previous-release and
candidate binaries and requires all three phases: create with the old runtime, resume
under current desktop policy, then resume with the old runtime. A local Provider fixture
verifies the actual model history, stable session ID and append-only transcript; settings,
memory and workspace fixtures must remain unchanged. The old phase uses its supported
pre-upgrade policy, rather than asking the old runtime to advertise newly added security
capabilities. This does not prove database downgrade, signed App installation, every
historical release or an external Provider; those remain separate acceptance evidence.

### Historical main integration

The local pinned Bridge `v0.1.34-0.20260916063139-6325d2acc450` combines the sandbox requirements above with main's MCP call-context contract: runtime `params._meta["claudecode/toolUseId"]` reaches the host callback unchanged. Missing metadata remains empty; business arguments cannot supply this identity. The combined module is locally verified and unpublished. [Acceptance evidence](../testing/desktop-sandbox-acceptance.md#2026-09-16main-同步与-bridge-兼容) records this integration separately from default sandbox rollout and platform acceptance.
