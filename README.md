# Nexus Core

Nexus Core is the main application repo for the Nexus stack.

## Development

Common commands:

- `make dev` — run frontend and backend in development mode
- `make check-private-sdk-access` — verify private Go SDK access before Go commands
- `make check` — run Go checks, frontend lint, and frontend type checks
- `make db-init` — run local database migrations
- `make gen-protocol-types` — regenerate frontend protocol types from Go definitions

## Private Go SDK dependency

This repository depends on the private Go module:

- `github.com/nexus-research-lab/nexus-agent-sdk-go`

Before running Go checks or backend commands, configure Go to treat the org as private:

```bash
go env -w GOPRIVATE=github.com/nexus-research-lab/*
go env -w GONOSUMDB=github.com/nexus-research-lab/*
```

### Recommended setup: SSH access

Use SSH for private repository access:

```bash
git config --global url."git@github.com:".insteadOf https://github.com/
ssh -T git@github.com
```

Then verify access:

```bash
make check-private-sdk-access
```

### Alternative setup: PAT access

If you use HTTPS, configure Git with a GitHub Personal Access Token that can read the private repositories under `nexus-research-lab`.

One common approach is to use Git Credential Manager, the macOS keychain helper, or another credential helper that can supply the token non-interactively.

After credentials are configured, verify access with:

```bash
make check-private-sdk-access
```

### If a failed checkout was cached

If you previously tried with the wrong HTTPS or auth setup, clear the cached module checkout and retry:

```bash
go clean -modcache
make check-private-sdk-access
```

If needed, rerun:

```bash
go env -w GOPRIVATE=github.com/nexus-research-lab/*
go env -w GONOSUMDB=github.com/nexus-research-lab/*
```

### Local `replace` during SDK development

The main branch expects direct access to the private SDK repository.

If `go.mod` contains a local `replace github.com/nexus-research-lab/nexus-agent-sdk-go => /some/path`, remove that local replace before running the normal project checks on main.
