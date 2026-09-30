# MCP authentication-helper header hardening

- Date: 2026-09-29
- Nexus commit: `31ef8d034`
- SDK commit: `3891dbdc`
- Bridge commit: `c251a8d`
- Branch: `codex/desktop-sandbox-approvals`
- nxs SHA-256: `1f863787e53eccee968bd3fc277c220bf7c625a99c2c23b4e1d88e175742ca1e`

The SDK now applies the same fail-closed validation to checked and compatibility
MCP `headersHelper` entry points: output is capped at 64 KiB, at most 128 headers
are accepted, names must be RFC token names, and CR/LF/NUL characters are
rejected. The SDK package test passed:

```text
GOWORK=off go test ./internal/mcp/client -count=1
ok   github.com/nexus-research-lab/nexus-agent-sdk-go/internal/mcp/client
```

The rebuilt nxs was exercised through the Nexus desktop host gate:

```text
NEXUS_SANDBOX_TEST_BINARY=<nxs> make check-desktop-sandbox
PASS: host-integration-only; installation and release acceptance remain separate.
```

This is an incremental MCP credential parsing and host integration result. It
does not prove full external MCP proxy isolation, graphical App acceptance,
macOS 14.0 exact termination, signed/notarized packaging, clean-host install,
or production release acceptance.
