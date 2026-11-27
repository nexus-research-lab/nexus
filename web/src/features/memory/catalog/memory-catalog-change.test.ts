// INPUT: Workspace 实时事件（写入完成、删除、流式增量）与不同 Agent/路径。
// OUTPUT: 证明只有当前 Agent 的记忆文件落盘或删除才使目录失效。
// POS: isMemoryCatalogChange 纯匹配回归；目录读取仍由 use-agent-memory 负责。

import { describe, expect, it } from "vitest";

import type { EventMessage } from "@/types/generated/protocol";

import { isMemoryCatalogChange } from "./memory-catalog-model";

function workspaceEvent(data: Record<string, unknown>): EventMessage {
  return { event_type: "workspace_event", data: { agent_id: "agent-1", ...data } } as unknown as EventMessage;
}

describe("isMemoryCatalogChange", () => {
  it("matches committed writes and deletions of MEMORY.md and memory/ files", () => {
    expect(isMemoryCatalogChange(workspaceEvent({ type: "file_write_end", path: "MEMORY.md" }), "agent-1")).toBe(true);
    expect(isMemoryCatalogChange(workspaceEvent({ type: "file_deleted", path: "memory/notes.md" }), "agent-1")).toBe(true);
  });

  it("ignores streaming deltas, other files and other Agents", () => {
    expect(isMemoryCatalogChange(workspaceEvent({ type: "file_write_delta", path: "MEMORY.md" }), "agent-1")).toBe(false);
    expect(isMemoryCatalogChange(workspaceEvent({ type: "file_write_end", path: "AGENTS.md" }), "agent-1")).toBe(false);
    expect(isMemoryCatalogChange(workspaceEvent({ type: "file_write_end", path: "notes/memory/a.md" }), "agent-1")).toBe(false);
    expect(isMemoryCatalogChange(workspaceEvent({ type: "file_write_end", path: "MEMORY.md" }), "agent-2")).toBe(false);
  });
});
