# macOS live third-party Provider acceptance

Date: 2026-09-27. Both nxs and Claude passed all five required checks against the
third-party Anthropic-compatible Provider configured in the user's local `.env`.
The final run exited 0: two passing backend subtests, no skipped backend, 57.08
seconds in the parent test. `releaseAccepted` remains **false**.

## Provenance

- macOS 27.0, arm64; Go 1.27.1; Claude CLI 2.1.273.
- Nexus production baseline: `f7b56fa22a197a285df7432a5132e68b10c7a15d`.
  The working tree adds the opt-in test and launcher, plus documentation. No
  production behavior was changed for this test. `report.json` pins both probe
  source hashes; the sources are committed alongside this record.
- nxs was built from SDK `9956def130da33af47accf799a9c27c16a551104`, with binary
  SHA-256 `0f91b17fc0ed6976e01a76363f466640a1cddfa63bc32338cb7647153e014270`.
  SDK branch head `ad1ad9f5` adds only draft archives and documentation.
- Bridge checkout: `6febf1b18a635edb383ff3f98bdab1d1ee5596fe`. All Go files,
  `go.mod` and `go.sum` match pinned commit `37434c2d38b1`; the newer commit adds
  archives/documentation only. The test used `GOWORK=off` and a temporary
  `-modfile` replacing Bridge with this checkout, avoiding the older noncanonical
  module ZIP still present in the shared local cache. The repository's `go.mod`
  and canonical module checksum were not changed by this run. This is a source
  equality check, not an additional fresh module-download acceptance result.
- Provider protocol and model alias are recorded in `report.json`. No endpoint,
  token, application `.env`, database or user state is archived. Official Claude
  account/OAuth authentication was not used.

## What passed

The test calls Nexus `BuildAgentClientOptions`, then the real Bridge and selected
runtime. It creates a fresh owner state root, workspace, scratch and empty denied
directories. The test host supplies explicit fixture rules after ordinary product
option assembly; no real user workspace or conversation is reused.

1. A real model invokes native Write, then Bash reads the file; the test checks
   both tool events, actual bytes and the command's unique content.
2. Native file writing to the forbidden fixture fails with a permission error;
   the file does not exist afterwards. nxs uses its file executor; Claude uses
   explicit native `Read(//absolute/scope/**)` and `Edit(...)` deny rules.
3. A Ruby child attempts file IO in a different forbidden directory. Both
   backends report `Operation not permitted` / `Errno::EPERM` and create no file.
   This directory has a sandbox filesystem rule but no Claude native-file deny
   rule; Ruby avoids mistaking shell-redirection preflight for OS enforcement.
4. A command requests explicitly denied `example.com`; the tool returns a policy
   or proxy rejection (403). Claude also reports `host is on the deny list`.
5. A real foreground sleep writes its PID. The host calls Interrupt and Close,
   then independently checks that this fixture PID is no longer present.

The fixture permission callback approves ordinary Read/Write/Bash requests only;
it grants neither sandbox escape nor extra network destinations. These callbacks
are test-host decisions, not evidence of the product's approval UI.

## Reproduce explicitly

Place the intended third-party model configuration in the repository's ignored
`.env`, or set `NEXUS_SANDBOX_LIVE_ENV_FILE` to an explicit file. From the Nexus
repository root, with a built nxs and installed Claude CLI:

```sh
NEXUS_SANDBOX_TEST_BINARY=/absolute/path/to/nxs \
NEXUS_SANDBOX_CLAUDE_BINARY=/absolute/path/to/claude \
node scripts/desktop/check-live-sandbox.mjs
```

`NEXUS_SANDBOX_LIVE_RUNTIME=nxs` or `claude` selects one backend; the default is
both. The runner reads only model/token/base-URL values from `.env`. It does not
load database, owner, Connector, debug or application state settings. The probe's
temporary credential environment key is cleared before the runtime starts;
host-projected provider authentication still uses each backend's ordinary path.
Output is redacted. Normal Go tests skip this opt-in external request test.

## Fixture correction and limits

`initial-fixture-scope-mismatch.log.gz` retains an earlier failed assertion:
the first fixture incorrectly expected Claude `sandbox.filesystem.denyWrite`
to block native Write. Claude's command sandbox and native Read/Edit permission
rules are separate contracts. The corrected fixture installs and tests them
separately; it does not claim a production fix for an unsupported rule mapping.
The rule syntax was checked against the
[Claude permissions reference](https://code.claude.com/docs/en/permissions#read-and-edit)
and the exact installed CLI. `Write(path)` is not used as a file permission rule.

`live-provider.log.gz` is the final successful run, including the actual denied
tool results. `manifest.sha256` covers the archived logs and machine-readable
report. All sensitive `.env` values were checked against the logs before archival.

This result does not prove App chat UI or DM/Room/automation Session manager
integration, persistent/human approval flows, dynamic network approval, Full
Access/backend transitions, arbitrary detached descendants, crash recovery,
whole-SDK IO or secret/handle/MCP/helper isolation, Provider OS egress controls,
other gateways or custom headers, signed packages, clean hosts or native Windows.
The remaining implementation and delivery work stays in the development plan.
