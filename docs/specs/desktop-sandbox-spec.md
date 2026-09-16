# Desktop sandbox integration (experimental)

This document describes implemented host wiring, not acceptance of the full App
sandbox. Remaining design and delivery work is tracked in
[the development plan](../explorations/desktop-sandbox/development-plan.md);
the [documentation index](../explorations/desktop-sandbox/README.md) separates
the current contract, dated assessment, acceptance evidence and historical experiments.

## Activation and scope

`NEXUS_DESKTOP_SANDBOX_ENABLED=true` is an explicit host rollout switch, defaulting
to false. It is effective only with `NEXUS_APP_MODE=desktop` on macOS or Windows.
Server deployments retain their existing runtime identity/isolation policy even
if the flag is present. Agent settings cannot set the internal host policy marker.

DM, Room and background memory maintenance use the same client-options builder.
For restricted approval modes, it requires nxs `required_sandbox_v1`,
`sandbox_file_tools_v1` and `sandbox_search_tools_v1` negotiation;
an unsupported runtime fails instead of silently accepting an unenforced policy.
Negotiation is not proof of current dependencies or an installed effective policy.
Native file-tool capability is currently declared only by macOS nxs builds;
Windows native execution is still incomplete and these desktop sessions fail
closed before receiving a task. This wiring is not yet enabled by desktop packaging defaults.

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
