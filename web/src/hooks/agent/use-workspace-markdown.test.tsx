// INPUT: exact Agent scope、当前选择、其他 Agent 刷新与跨 owner 的迟到文件请求。
// OUTPUT: 验证资源能力身份稳定且 owner reset 后不会恢复旧文件缓存。
// POS: Workspace Markdown 业务资源 Hook 的行为测试。

import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import * as agentApi from "@/lib/api/agent/agent-api";
import { useAgentStore } from "@/store/agent";
import { resetWorkspaceFilesOwnerScope, useWorkspaceFilesStore } from "@/store/workspace-files";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import { useWorkspaceMarkdown } from "./use-workspace-markdown";

const FILE: WorkspaceFileEntry = {
  depth: 1,
  is_dir: false,
  modified_at: "2026-09-05",
  name: "report.md",
  path: "private/report.md",
};

afterEach(() => {
  cleanup();
  resetWorkspaceFilesOwnerScope();
  useAgentStore.setState({ current_agent_id: null });
  vi.restoreAllMocks();
});

it("keeps exact Agent capabilities stable during unrelated selection and file updates", () => {
  useWorkspaceFilesStore.getState().set_files("exact-agent", [FILE]);
  const openFile = vi.fn();
  const { result } = renderHook(() => useWorkspaceMarkdown("exact-agent", openFile));
  const before = result.current;

  act(() => {
    useAgentStore.setState({ current_agent_id: "other-agent" });
    useWorkspaceFilesStore.getState().set_files("other-agent", [FILE]);
  });
  expect(result.current.resolveFilePath).toBe(before.resolveFilePath);
  expect(result.current.getFilePreviewUrl).toBe(before.getFilePreviewUrl);
  expect(result.current.onOpenWorkspaceFile).toBe(before.onOpenWorkspaceFile);
  result.current.onOpenWorkspaceFile?.("private/report.md");
  expect(openFile).toHaveBeenCalledWith("private/report.md", "exact-agent");
});

it("treats explicitly unknown scope as unbound and only omitted scope follows selection", () => {
  useAgentStore.setState({ current_agent_id: "selected-agent" });
  useWorkspaceFilesStore.getState().set_files("selected-agent", [FILE]);
  const initialProps: { agentId?: string | null } = { agentId: null };
  const { result, rerender } = renderHook(({ agentId }: { agentId?: string | null }) => useWorkspaceMarkdown(agentId, vi.fn()), {
    initialProps,
  });
  expect(result.current.currentAgentId).toBeNull();
  expect(result.current.getFilePreviewUrl).toBeUndefined();
  expect(result.current.onOpenWorkspaceFile).toBeUndefined();
  expect(result.current.resolveFilePath("report.md")).toBeNull();

  rerender({ agentId: undefined });
  expect(result.current.currentAgentId).toBe("selected-agent");
  expect(result.current.resolveFilePath("report.md")).toBe("private/report.md");
});

it("does not repopulate the new owner's resolver when an old owner request settles late", async () => {
  let resolveRequest!: (files: WorkspaceFileEntry[]) => void;
  vi.spyOn(agentApi, "getWorkspaceFilesApi").mockImplementationOnce(() => new Promise((resolve) => {
    resolveRequest = resolve;
  }));
  useWorkspaceFilesStore.getState().set_files("same-agent", [FILE]);
  const { result } = renderHook(() => useWorkspaceMarkdown("same-agent"));
  const pending = useWorkspaceFilesStore.getState().refresh_files("same-agent");

  act(() => resetWorkspaceFilesOwnerScope());
  expect(result.current.resolveFilePath("report.md")).toBeNull();
  await act(async () => {
    resolveRequest([FILE]);
    await pending;
  });
  expect(result.current.resolveFilePath("report.md")).toBeNull();
  expect(useWorkspaceFilesStore.getState().files_by_agent).toEqual({});
});
