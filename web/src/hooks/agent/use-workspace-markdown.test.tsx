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
