// Package tool defines strict, scope-free MCP tools for persistent Channel
// authorization.
//
// L2 | parent: internal/mcp/channelauthorization (L1 in AGENTS.md)
//
// Members:
//   - registry.go: owner-main private-DM visibility fence.
//   - start.go / status.go / cancel.go / submit_code.go: four operations.
//   - schema.go / helpers.go: strict schemas and transport-only rendering.
//
// Verification code is intentionally absent from every schema.
//
// [PROTOCOL]: update this header when the package contract changes.
package tool
