// Package channelauthorization exposes a dedicated owner-main private-DM MCP
// for persistent Channel QR and verification-code authorization.
//
// L2 | parent: internal/mcp (L1 in AGENTS.md)
//
// Members:
//   - contract/: server-fixed actor and narrow service interface.
//   - tool/: start/status/cancel/submit-code-card tools with strict schemas.
//   - server.go: in-process MCP assembly.
//
// QR/device payloads and verification codes never appear in MCP results or
// arguments; they cross only the authenticated native human presentation path.
//
// Exposed: NewServer.
//
// [PROTOCOL]: update this header when the package contract changes.
package channelauthorization
