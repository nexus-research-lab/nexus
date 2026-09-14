# Desktop sandbox and approval implementation plan

Status: **non-normative / implementation in progress**, 2026-09-14.
This document describes the target and acceptance gates. It does not declare
unimplemented behavior part of the current product contract.

## Goal and scope

Two implementation tracks share one regression gate:

1. macOS and native Windows command sandboxing, using Codex desktop behavior
   as reference. No VM and no sudo/password-entry feature.
2. Distinct tool and sandbox authorization, integrated with existing human
   approval and “Approve for me”.

Full Access must preserve existing application authorization behavior while
disabling the additional desktop command sandbox. It does not remove operating
system permissions, domain capability checks, or Linux server owner isolation.

## Verified starting point

- Nexus already supports `default`, `auto`, and `bypassPermissions`. Its pending
  human approvals and review explanations use the existing permission pipeline.
- nxs has `auto_review_v1`: a separate reviewer without tools, constrained
  authorization evidence, risk thresholds, human fallback, and revision fences.
  Domain operations requiring human approval must retain that requirement.
- nxs already implements macOS Seatbelt execution and Linux bubblewrap support.
  Sandbox configuration is off by default. Native Windows is not a supported
  sandbox backend yet. PowerShell therefore needs explicit implementation.
- Bridge already transports sandbox settings. Nexus's normal agent client
  assembly does not currently enable them.
- Nexus fingerprints sandbox options for process reuse, whereas live permission
  mode changes currently update only the permission mode. Coupled mode/sandbox
  changes require an explicit lifecycle contract.
- The SDK sandbox network callback receives host/port. Its existing callback
  contract is insufficient to establish an exact tool execution approval across
  Bridge; this must be traced and extended before claiming network review.
- Bash's bypass hint recommends immediate retry. That is incompatible with the
  target rule against repeating partially applied or unknown-effect operations.
- Native file tools, browser operations, and MCP services are not automatically
  contained by a sandbox attached to command subprocesses.

Evidence lives in Nexus `internal/runtime/clientopts`, `internal/runtime/permission`,
`internal/runtime/process_policy.go`, `docs/auto-review.md`; SDK
`internal/tool/executor`, `internal/tool/builtin/bash/sandboxexec`,
`internal/agent/runtime/permission_review.go`; and Bridge's sandbox settings
transport. These repositories must evolve through explicit capability contracts.

## Execution and approval chain

1. Resolve a trusted execution snapshot: owner, session, round, tool-use identity,
   permission revision, sandbox policy revision, and effective operating mode.
2. Apply existing domain and tool rules. Explicit denial cannot be overridden
   by sandbox availability or an automatic reviewer.
3. Resolve the execution boundary independently. Ordinary command allow rules
   cannot grant sandbox escape. Model input, workspace settings, environment
   variables, or command exclusions cannot weaken a mandatory host boundary.
4. In restricted mode, launch inside the selected OS backend. Descendants inherit
   restrictions; unsupported or unavailable mandatory isolation fails closed
   before the target starts. Never silently run directly.
5. A request for expanded access carries the exact command/arguments, working
   directory, requested resources, reason, and immutable execution identity.
   `default` asks the user; `auto` uses the existing reviewer with sandbox-specific
   evidence and falls back to the user. The grant is scoped to this execution.
6. A runtime denial is a result, not permission to replay. Distinguish
   `not_started`, `started`, and `unknown`. Only proven not-started work is eligible
   for an automatic retry after authorization. Otherwise inspect effects or obtain
   a new explicit decision before rerunning a side-effecting command.
7. Cancellation, expired scope, policy changes, and late replies invalidate the
   execution grant. Reconnection may restore an approval view; it must not replay
   the command. Existing durable approval suggestions retain their separate scope.

Known out-of-bound requests may ask before execution; they need not intentionally
fail once. Full Access uses the direct desktop execution path and retains existing
explicit ask/deny behavior. Switching modes must stop admission of new commands
until the new effective snapshot is installed. Running processes keep their
original boundary; switching to a stricter mode cannot retroactively confine them.
The implementation must either drain/cancel them or visibly retain their prior
boundary until termination, without reporting that all work is already restricted.

## Platform and tool boundaries

| Surface | Implementation target | Acceptance requirement |
| --- | --- | --- |
| macOS Bash and descendants | Reuse Seatbelt backend with host-owned policy | Real file/network/process tests on macOS |
| Windows Bash/PowerShell and descendants | Native restricted identity/token, ACL and network enforcement; evaluate dedicated identity setup and private desktop | Windows execution tests, installation/repair/uninstall and cancellation tests |
| Native Read/Write/Edit/search | Audit direct IO; enforce the same resource policy through trusted filesystem access where promised | Symlink/reparse-point and path-race tests; no shell-only coverage claim |
| Browser/MCP/connectors | Preserve domain permissions and provider authorization | No implied sandbox or approval bypass |
| Linux server | Preserve existing launcher, owner identity and filesystem confinement | Existing enforce/audit/off behavior unchanged |
| Claude runtime | Capability-gated documented behavior | Reject unsupported guarantees; never silently downgrade |

Windows implementation must account for inherited handles, process-tree cleanup,
reparse points, private desktop access, firewall scope, and installation privileges.
A restricted token alone is not evidence of complete filesystem/network isolation.
Do not advertise parity until native Windows tests demonstrate the promised boundary.

## Repository ownership and delivery order

1. **SDK**: exact sandbox capability and execution result contract; independent
   escape authorization; safe failure hints; trusted policy precedence; macOS
   integration; Windows backend; subprocess and native-file boundary tests.
2. **Bridge**: negotiated capability, typed approval/result fields, exact control
   cancellation and runtime differences. No SDK internals imported into Bridge.
3. **Nexus backend**: desktop-only policy assembly, mode lifecycle, existing
   permission controller integration and audit projection. Linux identity policy
   remains independent.
4. **Desktop hosts**: platform availability/setup reporting and lifecycle ownership;
   Windows bootstrap only after its privilege model is reviewed.
5. **Web**: reuse approval cards; explain requested extra access and effective mode,
   preserve pending approvals across DM/Room navigation and reconnect.
6. Update current specs and changelogs only for delivered behavior; package runtime
   versions only after cross-repository integration passes.

Do not globally enable sandboxing before escalation, mode switching, and backend
failure behavior pass their gates. Compatibility defaults remain unchanged during
implementation; this is staging, not permission to omit either target platform.

## Regression and acceptance matrix

- All three permission modes: sandboxed success, known extra access, runtime denial,
  explicit deny/ask rules, full access, unavailable backend, and mode switching.
- Approval: low-risk automatic approval, high-risk/manual fallback, malformed or
  timed-out review, cancellation, concurrent calls, stale revision, late reply,
  disconnected UI, no duplicate execution, no automatic permanent grant.
- Commands: builds, Git, package installation/network access, temporary files,
  streaming/PTY, background children, cancellation and process reuse.
- Resource boundary: outside-workspace writes, protected paths, symlink/reparse
  escapes, child processes, network denial and approved destinations.
- Existing features: DM/Room exact-source pending state, automation human-required
  operations, connector authorization, configuration mutations, artifacts and
  native file operations, Linux owner isolation, Claude compatibility.
- State claims must distinguish unit tests, local macOS integration, Windows
  cross-compilation, Windows real execution, and packaged application acceptance.

Baseline before changes: Nexus targeted permission/client-options/tool-policy/
message/app-runtime tests passed; nxs targeted PermissionReview/AutoReview/
NetworkReview tests passed. Earlier macOS isolation package checks passed but do
not execute Linux build-tagged enforcement or validate Windows. No platform parity
or end-to-end feature completion is claimed by these baselines.

## Reference behavior

- [Codex approvals and security](https://learn.chatgpt.com/docs/agent-approvals-security)
- [Codex Windows sandbox](https://learn.chatgpt.com/docs/windows/windows-sandbox)

Codex is a behavioral reference; Nexus domain grants and runtime capabilities remain
the authority for this implementation.

## Implementation log

2026-09-14: Corrected the SDK Bash prompt and sandbox failure annotation so they
no longer direct immediate unsandboxed retries or promise that every bypass will
necessarily display a human prompt. Both now require effect inspection and an
explicit decision before repeating partial or unknown side effects. This is a
guidance correction, **not** the execution-state enforcement described above.
Targeted SDK Bash, sandboxexec, and executor package tests passed on local macOS;
both repository diff checks passed. No default policy was changed or feature enabled.

2026-09-14, execution boundary: nxs now routes explicit sandbox bypass in restricted
modes to its permission handler independently of local command allow rules and
PermissionRequest hook allows. Missing handlers fail closed. Hook input updates
are checked again before marking permission handled. Requests retain tool-use
identity and input, add an explanation, and omit persistent command suggestions.
The existing auto reviewer remains the auto-mode handler; full access and disabled
sandbox paths are unchanged. Executor/Bash/sandboxexec tests passed; additional
execution-entry tests cover approval, rejection, absent handlers, allow hooks,
hook-injected bypass and compatibility. Test handlers record execution without
launching commands. This covers explicit bypass only: exclusions, unavailable
backend fallback, mandatory host policy, network grants and effect-state enforcement
remain pending. SDK changes are local and are not yet bundled into Nexus.

2026-09-14, required execution: explicit SDK Sandbox options now support opt-in
`RequireSandbox`. It forces enabled/fail-if-unavailable behavior and prevents
command exclusions or platform adapters from implicitly selecting direct execution.
Environment bypass overrides cannot weaken this mode. Native Windows currently
rejects required execution until its backend exists. Tests cover settings/env
weakening, exclusions, platform opt-out, forced direct adapters, and Windows policy;
Bash/sandboxexec/PowerShell/executor package tests passed on macOS. This is the
execution requirement only, not a complete host-owned resource policy. Ordinary
settings do not activate this field; Bridge/CLI negotiation, file/network policy
source ownership, effective mode switching, and Windows implementation remain open.

2026-09-14, transport integration: Bridge now negotiates `required_sandbox_v1` and
sends trusted boolean `required_sandbox` during initialization when its Sandbox
RequireSandbox option is enabled. nxs validates capability/type before creating its
session and installs the SDK requirement. Bridge rejects Claude for this option
before transport startup and disconnects old nxs versions without acknowledgement
before ConnectWithPrompt sends a task. Targeted Bridge client/protocol and nxs
cmd/protocol tests passed; scripted transport tests cover accepted and rejected
negotiation. This is not a real packaged process integration test or OS sandbox
test. Both repositories include updated contracts/changelogs. Nexus still uses its
released dependencies; a cross-repository integration build and product wiring are
pending. Live mode changes do not yet clear or replace this execution requirement.

2026-09-14, native evidence: built the modified nxs binary and successfully connected
the modified Bridge to that actual process using a temporary config/work directory;
`required_sandbox_v1` was acknowledged, with no model request sent. Added an opt-in
Bridge real-process test (`NEXUS_SANDBOX_TEST_BINARY=<built-nxs>`). Added and passed
the macOS-only opt-in `TestDarwinRequiredSandboxChildWriteBoundary`
(`NEXUS_SANDBOX_INTEGRATION=1`): the real Bash tool writes to the allowed workspace,
then its `/bin/sh` child is denied a protected temporary destination; the earlier
write occurs exactly once and the tool returns an error. This proves that tested
file restriction and child inheritance, not network confinement or a durable
unknown-effect recovery state machine. Those and Windows native verification remain
open. Concurrent unrelated Nexus changes were observed and left untouched.

2026-09-14, direct network boundary: added and passed the native macOS
`TestDarwinRequiredSandboxBlocksDirectNetwork` test. A temporary local HTTP server
is first proven reachable outside the sandbox. The actual Bash tool then writes a
startup marker and invokes `/usr/bin/curl --noproxy '*'` toward that server. The
command starts, returns failure, and no HTTP request reaches the listener. The
child-file-boundary test also passed again. These checks establish tested direct
network denial, not approved proxy access, dynamic network review, DNS leakage
coverage, or all socket protocols. Those remain separate acceptance items.

Mode lifecycle review confirms Nexus currently updates only the runtime permission
mode; it does not replace sandbox policy or retroactively constrain running children.
Product integration must resolve process draining/termination and effective-policy
replacement before reporting a stricter boundary or Full Access transition complete.

2026-09-14, concurrent admission: found that Bridge marks transport connected before
initialization finishes. Added a separate required-sandbox check before raw/user
and internal continuation writes; concurrent sends cannot enter an unconfirmed
runtime or consume queued next-turn context. The controlled initialization-race
test and race-detector run passed. Bridge client/protocol regression initially hit
the tool environment's chmod restrictions; rerunning with an isolated temporary
config root and approved filesystem access passed. No product runtime default was
changed. Mode lifecycle replacement and the remaining platform/product scope are
still pending.

2026-09-14, trusted resources: required-mode resource settings now come only from
explicit SDK host settings and system-managed policy. User/project/flag settings
cannot add grants. The default workspace and sandbox temporary root remain anchored
to the initial workspace; later cwd changes and discovered worktree main directories
do not automatically add write grants. Bridge sends the complete host Sandbox
configuration as initialize `sandbox_policy`, and nxs rejects malformed objects
before creating the session. Resource-source/cwd and protocol tests passed, as did
the native file/child and direct-network regressions. A test initially compared
lexical macOS /var paths against canonical /private/var; corrected physical-path
comparison passed. Standard compatible temp/cache roots remain and native file-tool
coverage is still a separate audit item. No product switch has been enabled.

2026-09-14, policy replacement: fixed Bridge Reconfigure accepting a changed Sandbox
object without applying it to the runtime. It now returns `ErrRestartRequired` with
`sandbox_policy_changed` before sending other hot controls or updating saved options.
Both directions and failure atomicity tests passed, along with Bridge client/protocol
regression. Added a Nexus Manager sandbox-transition test; it confirms replacement
and disconnect instead of Reconfigure. Source inspection confirms the existing
manager retires the old client and waits for disconnect before publishing the new
one. Actual desktop permission-mode selection still needs to build and route the
effective sandbox policy through this replacement mechanism. This is not yet UI or
packaged runtime acceptance.

2026-09-14, Windows implementation begins: added a native restricted-token component
using CreateRestrictedToken with disabled maximum privileges, LUA and write-restricted
flags, plus returned-token verification and explicit handle ownership. Added Windows
tests for missing authority and actual token creation. The component is intentionally
not wired to command execution until ACL lifecycle, network enforcement, private
desktop, Job cleanup and IPC defaults exist; current Windows required execution
continues to fail closed. Windows amd64 and arm64 test binaries cross-compiled on
macOS. These are compilation results only; Windows native tests have not run.
Requested a Windows machine/CI option from the user while continuing independent work.

Primary references: [Microsoft CreateRestrictedToken](https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-createrestrictedtoken)
and [Codex Windows token implementation](https://github.com/openai/codex/blob/main/codex-rs/windows-sandbox-rs/src/token.rs).

2026-09-14, Windows Job component: added anonymous non-inheritable Job ownership,
kill-on-close without breakaway flags, suspended-process assignment, and explicit
termination followed by active-process-count verification. Termination first closes
new admission; error or cancellation cannot authorize ACL/network lease reclamation.
Added a Windows native test that creates a suspended test process, assigns it,
terminates and verifies exit, and checks closed/terminating admission. Windows x64
and ARM64 binaries compile; these tests have not run on Windows. The components
remain unconnected to command execution until the remaining backend layers exist.
Reference: [Microsoft Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects).

2026-09-14, Windows private desktop: implemented random per-instance desktop names,
explicit protected DACLs for host/execution/capability identities, non-inherited
handles, no desktop-switch or hook/journal access grants, and idempotent close.
Creation uses a dedicated OS thread whose exit is checked before returning the
desktop, avoiding a creator/close race or changing the host UI thread. Added native
lifecycle and switch-access denial tests. Windows x64/ARM64 cross-compilation passed;
native execution remains unverified. Window-station access, actual restricted-process
startup, ACL lifecycle and network enforcement remain incomplete, so the Windows
command backend stays unavailable.
Reference: [Microsoft CreateDesktop](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-createdesktopw).

2026-09-14, Windows grant path prerequisites: added portable validation for local
absolute grant directories, rejecting device/UNC/volume-root, ADS, reserved device
names including superscript aliases, dot segments and ambiguous suffixes. Added
Windows ancestor/target handle pinning with reparse/non-directory rejection and
no write/delete sharing; no ACL mutation occurs yet. Native tests check ancestor
rename rejection while allowing child file IO, then successful rename after close.
Portable validation tests passed; Windows x64/ARM64 builds passed. Handle behavior,
subtree reparse handling, ACL leases and UNC support remain unverified/incomplete.
Reference: [Microsoft CreateFile](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew).

2026-09-14, denial attribution regression: macOS execution profiles now receive
independent random observation tags, including for identical concurrent commands.
Missing/foreign tags cannot broadcast a denial to active subscribers. Stored tails
require the same tool-use identity, tool name and workspace; unattributed historical
command matches are no longer assigned to a new request. Updated existing PowerShell
fixtures to carry their observed identity and added concurrent/repeated-command and
unattributed-event regressions. Bash, PowerShell and sandboxexec package tests passed;
macOS real child-write and direct-network-denial tests passed after the change.
Windows x64 cross-compilation passed. These observations are diagnostics, not proof
that a command made no changes, and must not themselves authorize a replay.

Product integration audit: Nexus currently hot-updates approval mode through
`agentClient.SetPermissionMode`; it does not derive or replace the sandbox policy.
The existing process-policy fingerprint already includes sandbox options, but the
desktop option builder does not yet enable the new requirement. Full Access mode
integration and pending-approval invalidation therefore remain explicit outstanding
work, not completed behavior.

2026-09-14, runtime exit confirmation: Bridge session cleanup now waits for the
transport's exit confirmation after its close attempt, even when termination
reports an error. A closed stream or non-timeout close error cannot independently
release the cleanup fence. Cancellation only ends the individual caller's wait;
both close and exit diagnostics survive. A transport regression simulates failed
termination with a closed stream and a still-live process, verifies repeated
waiters remain fenced, then confirms completion only after explicit exit.
Bridge client/transport tests and focused race tests passed, including the opt-in
real nxs handshake/close test. Nexus runtime/clientopts tests passed against the
modified sibling Bridge through `/private/tmp/nexus-desktop-sandbox.work`; this
temporary workspace is validation wiring, not a published dependency upgrade.
This confirms the runtime transport/main process, not arbitrary descendant exit.
Windows Job execution wiring and native acceptance remain outstanding.

2026-09-14, host integration: the Bridge development branch
`codex/desktop-sandbox-approvals` now contains commit `0c82407d16ee`, including the
original v0.1.33 release ancestor. Nexus pins its reproducible pseudo-version
`v0.1.34-0.20260914044340-0c82407d16ee`; no stable release tag was changed.
The common DM/Room/background options builder now receives an explicit desktop
rollout flag and constructs the mandatory policy. Skill resources are separated
from writable user mounts; SDK mandatory mode no longer interprets additional
resource discovery directories as implicit write grants. Legacy mode is unchanged.
The desktop rollout is default-off while remaining execution/approval work continues.

Crossing Full Access retires the client instead of hot-updating its permission mode.
DM closes its session; Room cancels its exact slot and pending approvals and confirms
cleanup without replay. HTTP permission updates also continue to Room after a DM
failure. Targeted and package regressions for runtime, clientopts, config, DM, Room,
memory maintenance, configuration, permission and Agent handlers passed; focused
runtime/Room race tests passed. The architecture checker caught and prompted removal
of a forbidden runtime-to-permission dependency; the host policy marker now belongs
to protocol and the architecture check passed. Current implemented scope and explicit
limitations are maintained in `docs/specs/desktop-sandbox-spec.md`.

Validation follow-up: host-built options successfully negotiated with the freshly
built real nxs through the pinned Bridge and confirmed cleanup without a model
request. The Go gate selected the full backend due to the dependency update. It
reported two migration fixture failures under the Makefile-exported desktop mode,
plus the unchanged nexus-configuration Skill size check (5331 bytes versus 5120).
The migration fixtures now explicitly select their intended web/server mode;
dedicated desktop-preservation tests remain separate. Skill contents and its limit
are unchanged from HEAD. Full-gate success is not claimed while that size failure
remains. Full logs: `/private/tmp/nexus-sandbox-go-gate.log`.

2026-09-14, scoped approval delivery: Nexus now pins Bridge development version
`v0.1.34-0.20260914050823-85771d895ee2`. The typed sandbox-escape boundary survives
nxs and Bridge transport; Nexus explains outside-sandbox execution and restricts
the decision to one invocation. Persistent updates are rejected, including forged
responses, and unknown boundaries fail before creating pending UI. The SDK binds
escape approval to exact JSON input and cancels manual/automatic callbacks on
effective permission changes, rejecting late allow responses. Human-only flags and
review evidence now survive the nxs proxy. A local Full Access update cannot disable
a host-required sandbox; the App transition continues through process replacement.
These checks do not constitute complete admission/replay protection for every hook
or network callback.

SDK executor/runtime/proxy/permission tests and focused race tests passed, as did
the real macOS child-write and direct-network-denial tests. Bridge client/permission/
protocol tests passed. Nexus permission/configuration/channels/runtime/clientopts/
DM/Room/Agent-handler regressions, permission race tests and architecture checks
passed. The SDK merged upstream history-rewrite fixes and passed the affected
client/context/query-loop/MCP tests in addition to sandbox/approval regressions.

Windows native component CI is now available on the SDK development branch. Its
first run (`34809309693`) passed restricted-token, Job termination and pinned-path
tests but exposed a private-desktop creator-thread exit failure. This is native
component evidence, not complete Windows sandbox acceptance; required Windows
command execution remains disabled while the backend is incomplete.

The follow-up Windows native run
[34809567020](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34809567020)
passed at SDK commit `df4afc5e`. The creator now restores its original thread desktop
before scheduler reuse, instead of assuming every locked Go thread exits. A failure
to restore still fails admission and does not return that thread to the scheduler.
This follows the Windows
[thread desktop API](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setthreaddesktop)
contract. Token/Job/path/private-desktop component tests ran on Windows x64; ARM64
and the complete command backend have not received native acceptance. The rebuilt
nxs also passed the opt-in Nexus host/Bridge initialization and confirmed-cleanup
test without a model request. No shell permission end-to-end inference is made from
that handshake test.

2026-09-14, Windows process-tree evidence: native run
[34809976111](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34809976111)
passed at SDK commit `08e413d8`. A suspended root was assigned to the Job and then
resumed; its explicit breakaway attempt was rejected, a live descendant was verified
as belonging to the same Job, and termination confirmed both processes exited.
This strengthens descendant lifecycle evidence but does not establish restricted
token execution, filesystem grants, network isolation or packaged App acceptance.

Startup-path audit also found unsupported native preparation could allocate a proxy
before returning its platform error. SDK `396b4216` rejects unsupported backends
before temporary directory/proxy allocation and retains defensive proxy cleanup on
the terminal failure path. macOS sandboxexec/Bash tests and Windows x64 compilation
passed; native unavailable-backend regression is tracked by the branch CI.

Native run
[34810133925](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34810133925)
confirmed the unavailable-backend regression. The subsequent preparation audit found
`os.MkdirAll` could follow a workspace metadata symlink outside the root before OS
sandbox activation. SDK `7a97baee` uses `os.Root` for mandatory preparation while
preserving the legacy path. Static escaping links are rejected, internal links stay
usable, and concurrent link replacement cannot create a directory outside the root.
macOS package/race tests and real child-write/direct-network-denial tests passed;
Windows native run
[34810349079](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34810349079)
also passed these preparation and process components. This does not cover every
host-side file operation. SDK draft
[PR #3](https://github.com/nexus-research-lab/nexus-agent-sdk-go/pull/3)
now tracks review and full repository regression CI without merging or releasing
the unfinished backend.

2026-09-14, upstream integration and complete SDK regression: merged the latest
tool-description changes at `a2026633`, preserving sandbox recovery guidance.
[Linux full SDK CI](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34810691819)
and [Windows native components](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34810691834)
both passed. This Linux run covers the SDK suite, not Nexus server owner-isolation
deployment or the previously recorded Nexus Skill-size gate failure.

The next approval audit found shell execution reread session cwd after approval.
SDK `a2c20abc` captures the sandbox-escape working-directory path in host-only
context, uses it in Bash/PowerShell launch entrypoints, and preserves the original
scope explanation when automatic review falls back to a human. A real relative
write test changes session cwd inside the approval callback and verifies the write
lands only in the approved directory. Executor/runtime package tests and focused
race tests passed. This closes mutable session-directory redirection; filesystem
inode replacement and full execution snapshot/replay protection remain separate
outstanding work.

2026-09-14, proxy cancellation: the directory-binding SDK commit passed
[full Linux CI](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34811261124).
The network audit then found SOCKS callbacks used a detached background context and
proxy closure did not own established SOCKS/hijacked CONNECT connections. SDK
`89b6172b` adds one proxy lifetime context and a connection registry: closing the
proxy cancels pending callbacks, rejects late allows, and closes both tunnel ends.
Tests exercise a callback returning allow after cancellation without opening an
upstream connection, plus real established SOCKS/CONNECT tunnels. These tests passed
with the race detector; sandboxexec/Bash/PowerShell packages and real macOS
child-write/direct-network-denial regressions passed. Native Windows and full Linux
CI are rerun by the draft PR. Exact command-scoped Nexus destination approval is
still unimplemented; this lifecycle fix is its prerequisite, not its completion.

2026-09-14, command-scoped destination approval: proxy-lifecycle SDK full Linux run
[34811618314](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34811618314)
passed. SDK `37c8d774` now installs a `sandbox_network` callback for mandatory Bash/
PowerShell execution when the host has not supplied an explicit network callback.
It captures command input, tool-use identity, cwd and permission epoch. The same
one-connection decision uses the manual handler or automatic reviewer, preserving
trusted query context while cancellation follows the proxy and epoch. Changed
input, persistent updates and old-epoch requests are refused; each connection is
reviewed independently. Static domain denial and managed-only policy remain above
this callback. Legacy optional sandbox/custom callback behavior is retained.

Bridge `42f368f` preserves the new classification; Nexus pins
`v0.1.34-0.20260914060629-42f368fbc137` and displays the pending destination as a
one-connection approval without persistent suggestions. The SDK native macOS test
writes a marker before connecting, checks the marker in the network approval,
permits exactly one request to a local server, and verifies the marker was not
written twice. Executor/runtime/nxs/permission tests and focused race tests passed;
Nexus permission/configuration/channels and architecture checks passed. Bridge
client/permission/protocol checks passed. This is component and native SDK evidence,
not a packaged end-to-end App acceptance claim. Windows command startup and complete
durable effect/background recovery remain outstanding.

2026-09-14, direct background startup: network-approval SDK full Linux run
[34812602153](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34812602153)
passed. The next audit found direct background shell preparation rebuilt options
without the command-scoped proxy callback. SDK `ce4415a3` preserves it in Bash and
PowerShell preparation. A native macOS Bash test waits for background network
approval, ends the foreground context, and verifies one successful connection and
one marker write. Its second case changes permission mode while approval is pending
and deliberately returns a late allow; no destination request occurs and the
background task fails without replaying the marker write. These native cases and
executor/runtime package regressions passed. PowerShell native background network
acceptance and reviewer/session-turnover recovery remain separate outstanding work.

2026-09-14, Windows object identity audit: background-network SDK full Linux run
[34813170762](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34813170762)
passed. SDK `e09ed14a` adds explicit default object DACL installation to the token
prototype. Initial Windows native run
[34813605676](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34813605676)
failed to create a named event under a capability-only restricted token.
The [Codex token implementation](https://github.com/openai/codex/blob/main/codex-rs/windows-sandbox-rs/src/token.rs)
also includes logon/world restricting identities for namespace compatibility.
SDK `46819f3d` obtains the logon SID through bounded buffer parsing and adds those
identities, while keeping default object grants limited to execution user/capability.

The follow-up native run
[34813978450](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34813978450)
created and reopened the event, but FAILED the cross-capability checks: another
restricted token for the same Windows user obtained EVENT_MODIFY_STATE and WRITE_DAC.
Object isolation is therefore unproven and contradicted for this same-user setup.
Do not remove this failure from acceptance or claim that capability SIDs isolate
host IPC. The production launcher must use a dedicated execution identity distinct
from the host, with explicit admission validation; its object/process access checks
must be tested using that actual identity arrangement. Logon/world identities can
also match existing resource ACLs, so their presence does not replace filesystem
and network enforcement. Native command execution remains disabled. The current
Windows component gate is failing on this new acceptance test; token construction
or cross-compilation success must not be reported as complete backend acceptance.

2026-09-14, Windows identity test interpretation correction: SDK `76116907`
rejects host and built-in service identities at token admission. Native run
[34814817347](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34814817347)
passed that guard and token construction but failed named-event creation while
impersonating the disposable ordinary execution account. Account cleanup succeeded;
its separate Linux CI run `34814817354` passed. These event tests use thread
impersonation inside the host process, not a process launched under the restricted
primary token. Therefore the preceding same-user event result is evidence about
that test arrangement only; it cannot establish a general capability-SID isolation
conclusion for a separately launched process. Microsoft's
[CreateEventW documentation](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-createeventw)
explicitly calls out security-descriptor handling for impersonating creators.
SDK `09093eda` retains the failing named-event case and adds anonymous-event checks
with default versus explicit token DACLs to separate namespace access from object
security. Windows x64 cross-compilation passed; native evidence is pending. Actual
primary-token launch, object access and process-tree confinement remain required
acceptance work; the Windows product execution path remains disabled.

The diagnostic native run
[34816428447](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34816428447)
completed: both anonymous-event cases passed (creation and signaling with default
and explicit token DACLs), while the retained named-event creation still failed
with Access Denied. This narrows the next investigation to named-object namespace
access and the primary-process launch arrangement; it does not justify broadening
object DACL grants or establish complete IPC isolation. The overall Windows gate
remains failed.

2026-09-14, real Windows primary-process composition: SDK `315bd133` adds a
native launch test using the dedicated restricted primary token, an explicit
private desktop and a minimal environment without CI credentials. The process is
created suspended, assigned to the Job, checked for the expected non-host identity
and restricted token, then resumed. Native run
[34816723985](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34816723985)
passed this composition and observed the command's expected exit code. Disposable
account cleanup passed. The existing impersonated named-event test still failed,
so the whole component gate remained red. SDK `5581e652` extends the same path to
PowerShell creating/signaling a named event within the actual child process;
Windows x64 and ARM64 cross-compilation passed, with native results pending.
This test-only CreateProcessWithTokenW path is not the finished production broker:
explicit inherited-handle control, filesystem/network enforcement, installation
and end-to-end tool integration remain required. The
[Codex process implementation](https://raw.githubusercontent.com/openai/codex/main/codex-rs/windows-sandbox-rs/src/process.rs)
uses CreateProcessAsUserW with a process attribute list for Job attachment and
explicit stdio handle inheritance; production integration must account for these
boundaries instead of treating successful process creation as sandbox acceptance.

Native follow-up
[34816905707](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34816905707)
passed the cmd case but PowerShell did not exit within ten seconds; the test then
terminated it and the Job, and account cleanup succeeded. This timeout does not
identify whether initialization or the event operation stalled. A separate
PowerShell immediate-exit control and a thirty-second observation window are added
next; no retry of the timed-out process or production capability claim is made.

2026-09-14, PowerShell startup and desktop qualification: native run
[34817081291](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34817081291)
passed cmd but both PowerShell immediate-exit and named-event cases timed out at
thirty seconds. Cleanup succeeded. The startup-only failure means the named-event
operation cannot yet be blamed. SDK `e5d59e53` now queries the creator's actual
window-station name and retains a qualified station/desktop startup path; this
avoids relying on the cross-account launch API's implicit station selection.
No desktop rights were broadened. SDK `8ac1f7ed` also adds an ordinary dedicated
account token control through the same launch path, with process identity and
restriction-state verification, to separate restricted-token behavior from launch
environment behavior. Windows x64 cross-compilation passed; native runs are pending.

The ordinary-account control in native run
[34817455500](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34817455500)
also timed out, as did both restricted PowerShell cases; cmd still passed. Qualified
desktop naming did not resolve this symptom. The current evidence therefore does
not isolate the restriction layer as its cause. The next test initializes the
dedicated account profile via LOGON_WITH_PROFILE and asks Windows to construct that
account's environment instead of passing only SystemRoot. This is confined to the
disposable CI fixture and does not inherit the runner's credential environment or
relax token/desktop permissions. Complete Windows acceptance remains outstanding.

2026-09-14, profile initialization boundary: native run
[34818078641](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34818078641)
showed LOGON_WITH_PROFILE fails restricted process creation with an invalid
resource-state error, including the previously passing cmd case. The ordinary
account PowerShell case still timed out. This experiment is not a startup fix.
SDK `15c37259` removes profile loading from restricted process creation while
retaining Windows-generated target-account environment; production account/profile
initialization belongs to the trusted provisioning stage. SDK `b684f130` adds
bounded wait-chain metadata on test timeout, without reading process memory,
credentials or object names. Native diagnosis is pending; prior failures remain
recorded and no Windows product capability is enabled.

Native wait-chain run
[34818418873](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34818418873)
restored cmd success after removing profile loading. All three PowerShell cases
still timed out; each reported a blocked thread, an owned ALPC object and a final
PID-only thread node, with no detected cycle. This points the next investigation
toward the system process serving that ALPC interaction; it does not identify the
specific initialization failure yet. The test diagnostic is extended to query
only executable basenames for those process nodes, without logging their command
lines, environments or process memory. The ordinary-account control remains
necessary; current evidence still does not distinguish the restriction layer as
the cause. Native Windows acceptance remains incomplete.

2026-09-14, Windows cross-account launch contract: native run
[34818791059](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34818791059)
identified csrss.exe as the ALPC wait endpoint for ordinary and restricted
PowerShell launches. This identifies the endpoint, not a specific CSRSS failure.
SDK `595bc8c9` tests CreateProcessAsUserW, matching the Codex process API, with
explicit environment and no automatic fallback. Native run
[34819142009](https://github.com/nexus-research-lab/nexus-agent-sdk-go/actions/runs/34819142009)
then failed all cross-account launches immediately with a required-privilege error;
the ordinary CI host does not satisfy that direct launch contract. Next work must
validate the dedicated-account runner bootstrap and same-identity restricted child
launch arrangement, rather than granting the desktop App ambient administrator
privileges or interpreting a component-only token test as a working backend.
The remaining account provisioning, authenticated runner transport, inherited
handle allowlist, filesystem and network policies remain part of the full goal.
