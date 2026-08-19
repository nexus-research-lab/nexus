/**
 * INPUT: Runtime Tool 名称（含 SDK、MCP 与 provider 包装形式）。
 * OUTPUT: WorkGraph Tool 节点的用户动作图标类别。
 * POS: 只负责“这是什么动作”的视觉语义，不决定节点可见性或运行状态色。
 */

export type ExecutionToolVisualKind =
  | "browser"
  | "external"
  | "fetch"
  | "generate"
  | "generic"
  | "inspect"
  | "search"
  | "send"
  | "terminal"
  | "workflow"
  | "write";

const INSPECT_TOOL_NAMES = new Set([
  "glob",
  "grep",
  "ls",
  "read",
]);

const TERMINAL_TOOL_NAMES = new Set([
  "bash",
  "killshell",
  "taskoutput",
]);

const WRITE_TOOL_NAMES = new Set([
  "edit",
  "multiedit",
  "notebookedit",
  "write",
]);

const WORKFLOW_TOOL_NAMES = new Set([
  "agent",
  "skill",
  "task",
]);

const BROWSER_ACTION_PREFIXES = [
  "click",
  "fill",
  "navigate",
  "open",
  "screenshot",
  "type",
];

const SEND_ACTION_PREFIXES = [
  "comment",
  "email",
  "message",
  "post",
  "publish",
  "reply",
  "send",
];

export function resolveExecutionToolVisualKind(
  toolName: string | null | undefined,
): ExecutionToolVisualKind {
  const parsed = parseExecutionToolName(toolName);
  if (!parsed.leaf) {
    return "generic";
  }
  if (
    parsed.leaf === "search"
    || parsed.leaf === "websearch"
    || parsed.leaf.endsWith("websearch")
  ) {
    return "search";
  }
  if (
    parsed.leaf === "fetch"
    || parsed.leaf === "webfetch"
    || parsed.leaf.endsWith("webfetch")
  ) {
    return "fetch";
  }
  if (INSPECT_TOOL_NAMES.has(parsed.leaf)) {
    return "inspect";
  }
  if (WORKFLOW_TOOL_NAMES.has(parsed.leaf)) {
    return "workflow";
  }
  if (
    TERMINAL_TOOL_NAMES.has(parsed.leaf)
    || parsed.leaf.endsWith("codeexecution")
    || parsed.server.includes("noderepl")
  ) {
    return "terminal";
  }
  if (
    WRITE_TOOL_NAMES.has(parsed.leaf)
    || (
      isWorkspaceToolServer(parsed.server)
      && startsWithAny(parsed.leaf, ["append", "edit", "patch", "upload", "write"])
    )
  ) {
    return "write";
  }
  if (
    isBrowserToolServer(parsed.server)
    || startsWithAny(parsed.leaf, BROWSER_ACTION_PREFIXES)
  ) {
    return "browser";
  }
  if (
    parsed.server.includes("imagegen")
    || startsWithAny(parsed.leaf, ["generate", "render"])
  ) {
    return "generate";
  }
  if (startsWithAny(parsed.leaf, SEND_ACTION_PREFIXES)) {
    return "send";
  }
  if (parsed.mcp) {
    return "external";
  }
  return "generic";
}

function parseExecutionToolName(toolName: string | null | undefined): {
  leaf: string;
  mcp: boolean;
  server: string;
} {
  const normalized = toolName?.trim().toLowerCase() ?? "";
  if (!normalized) {
    return { leaf: "", mcp: false, server: "" };
  }
  const mcp = normalized.startsWith("mcp__");
  const mcpParts = mcp
    ? normalized.slice("mcp__".length).split("__")
    : [];
  const server = canonicalToolName(mcpParts.length > 1 ? mcpParts[0] : "");
  let leaf = normalized;
  for (const separator of ["__", ".", "/"]) {
    const index = leaf.lastIndexOf(separator);
    if (index >= 0) {
      leaf = leaf.slice(index + separator.length);
    }
  }
  return { leaf: canonicalToolName(leaf), mcp, server };
}

function canonicalToolName(value: string): string {
  return Array.from(value)
    .filter((character) => /[\p{L}\p{N}]/u.test(character))
    .join("");
}

function startsWithAny(value: string, prefixes: readonly string[]): boolean {
  return prefixes.some((prefix) => value.startsWith(prefix));
}

function isBrowserToolServer(server: string): boolean {
  return ["browser", "chrome", "playwright"].some((marker) => (
    server.includes(marker)
  ));
}

function isWorkspaceToolServer(server: string): boolean {
  return ["filesystem", "localfs", "workspace"].some((marker) => (
    server.includes(marker)
  ));
}
