# Nexus Core

Nexus Core is the main application repo for the Nexus stack.

## Development

Common commands:

- `make dev` — run frontend and backend in development mode
- `make check-bridge-sdk-access` — verify the public Go bridge SDK can be resolved
- `make check` — run Go checks, frontend lint, and frontend type checks
- `make db-init` — run local database migrations
- `make gen-protocol-types` — regenerate frontend protocol types from Go definitions

## Go bridge SDK dependency

This repository depends on the public Go bridge module:

- `github.com/nexus-research-lab/nexus-agent-sdk-bridge`

The bridge module contains the shared client, protocol, permission, hook, and MCP contracts used by Nexus. The private runtime SDK is not required for the default open-source build.

Verify the bridge dependency before running backend commands:

```bash
make check-bridge-sdk-access
```

### Local `replace` during bridge development

When developing the bridge and Nexus together, you can temporarily point Go at a local checkout:

```bash
go mod edit -replace github.com/nexus-research-lab/nexus-agent-sdk-bridge=/Users/leemysw/Projects/nexus-agent-sdk/nexus-agent-sdk-bridge
```

Before committing or running normal project checks on main, remove the local replace and use the published bridge module version:

```bash
go mod edit -dropreplace github.com/nexus-research-lab/nexus-agent-sdk-bridge
go mod tidy
```
