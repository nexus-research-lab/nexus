// INPUT: 可控目录请求结果、缓存和当前 Agent。
// OUTPUT: 首次失败/确认空目录、缓存失败反馈、重试及跨作用域迟到结果的回归。
// POS: 真实 Workspace 控制器与目录视图接入测试；目录 API 使用离线结果。

import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef, useState, type ComponentProps, type ReactNode } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { getWorkspaceFilesApi } from "@/lib/api/agent/agent-api";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { resetWorkspaceFilesOwnerScope, useWorkspaceFilesStore } from "@/store/workspace-files";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import { WorkspaceFileBrowser } from "../view/workspace-file-browser";
import { RoomWorkspaceView } from "../room-workspace-view";
import { useRoomWorkspaceController } from "./use-room-workspace-controller";

vi.mock("@/lib/api/agent/agent-api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/lib/api/agent/agent-api")>(),
  getWorkspaceFilesApi: vi.fn(),
}));
vi.mock("@/features/conversation/shared/editor/workspace-file-preview-panel", () => ({
  WorkspaceFilePreviewPanel: ({ onTogglePreviewFocus }: { onTogglePreviewFocus: () => void }) => (
    <button onClick={onTogglePreviewFocus}>Toggle preview focus</button>
  ),
}));

const openFile = vi.fn();
const cachedFile: WorkspaceFileEntry = { name: "notes.md", path: "notes.md", depth: 0, is_dir: false, modified_at: "" };
function pendingFiles() {
  let resolve!: (files: WorkspaceFileEntry[]) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<WorkspaceFileEntry[]>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
function Workspace({ agentId }: { agentId: string }) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const controller = useRoomWorkspaceController({
    agentId, activeWorkspacePath: null, composerDraftScopeKey: null,
    fileInputRef, isDm: true, onOpenWorkspaceFile: openFile, roomMembers: [],
  });
  return <>
    <WorkspaceFileBrowser activePath={null} controller={controller.browser} onResizeStart={vi.fn()} resizeControl={null} stacked width={240} />
    <FeedbackBannerViewport item={controller.feedback} />
  </>;
}
function renderWorkspace(agentId = "a") {
  return render(<Workspace agentId={agentId} />, { wrapper });
}
function wrapper({ children }: { children: ReactNode }) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider>;
}

beforeEach(() => {
  resetWorkspaceFilesOwnerScope();
  vi.clearAllMocks();
  vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new Error("Unexpected network request"))));
});
afterEach(() => vi.unstubAllGlobals());

it("shows one retry surface after an initial failure and only reports empty after a successful read", async () => {
  const initial = pendingFiles();
  const retry = pendingFiles();
  vi.mocked(getWorkspaceFilesApi).mockReturnValueOnce(initial.promise).mockReturnValueOnce(retry.promise);
  const user = userEvent.setup();
  renderWorkspace();
  expect(screen.getByRole("status").textContent).toBe("common.loading");
  await act(async () => initial.reject(new Error("internal diagnostic text")));
  expect(screen.queryByText("room.no_files")).toBeNull();
  expect(screen.getAllByText("room.workspace_list_failed_title")).toHaveLength(1);
  expect(screen.getByText("room.workspace_list_unavailable_impact")).toBeTruthy();
  expect(screen.queryByText("internal diagnostic text")).toBeNull();
  expect(document.querySelector("[data-feedback-viewport]")).toBeNull();
  await user.click(screen.getByRole("button", { name: "room.workspace_refresh_action" }));
  expect(getWorkspaceFilesApi).toHaveBeenCalledTimes(2);
  expect(screen.getByRole("status").textContent).toBe("common.loading");
  expect(screen.queryByText("room.workspace_list_failed_title")).toBeNull();
  await act(async () => retry.resolve([]));
  expect(screen.getByText("room.no_files")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "room.workspace_refresh_action" })).toBeNull();
  expect(fetch).not.toHaveBeenCalled();
});

it("keeps cached files and does not turn dismissed read failure into a confirmed empty directory", async () => {
  useWorkspaceFilesStore.getState().set_files("a", [cachedFile]);
  vi.mocked(getWorkspaceFilesApi).mockRejectedValueOnce(new Error("offline"));
  const user = userEvent.setup();
  renderWorkspace();
  await screen.findByText("room.workspace_list_failed_title");
  expect(screen.getByRole("button", { name: "notes.md" })).toBeTruthy();
  expect(screen.queryByText("room.workspace_list_unavailable_impact")).toBeNull();
  expect(document.querySelector("[data-feedback-viewport]")).not.toBeNull();
  await user.click(screen.getByRole("button", { name: "common.close" }));
  expect(document.querySelector("[data-feedback-viewport]")).toBeNull();
  expect(screen.getByRole("button", { name: "notes.md" })).toBeTruthy();
  act(() => useWorkspaceFilesStore.getState().clear_agent("a"));
  expect(screen.getByText("room.workspace_list_unavailable_impact")).toBeTruthy();
  expect(screen.queryByText("room.no_files")).toBeNull();
});

it("ignores a prior Agent failure after the new directory has loaded", async () => {
  const oldRead = pendingFiles();
  const newRead = pendingFiles();
  vi.mocked(getWorkspaceFilesApi).mockImplementation((agentId) => agentId === "a" ? oldRead.promise : newRead.promise);
  const { rerender } = renderWorkspace();
  rerender(<Workspace agentId="b" />);
  await waitFor(() => expect(getWorkspaceFilesApi).toHaveBeenCalledWith("b"));
  await act(async () => newRead.resolve([]));
  expect(screen.getByText("room.no_files")).toBeTruthy();
  await act(async () => oldRead.reject(new Error("late failure")));
  expect(screen.queryByText("room.workspace_list_failed_title")).toBeNull();
  expect(screen.getByText("room.no_files")).toBeTruthy();
});

it("preserves directory expansion across preview focus and resets it for another Agent workspace", async () => {
  const files: WorkspaceFileEntry[] = [
    { name: "docs", path: "docs", is_dir: true, depth: 0, modified_at: "" },
    { name: "nested", path: "docs/nested", is_dir: true, depth: 1, modified_at: "" },
    { name: "readme.md", path: "docs/nested/readme.md", is_dir: false, depth: 2, modified_at: "" },
  ];
  vi.mocked(getWorkspaceFilesApi).mockResolvedValue(files);
  const user = userEvent.setup();
  function FullWorkspace({ agentId }: Pick<ComponentProps<typeof RoomWorkspaceView>, "agentId">) {
    const [path, setPath] = useState<string | null>("docs/nested/readme.md");
    return <RoomWorkspaceView activeWorkspacePath={path} agentId={agentId} composerDraftScopeKey={null} isDm onOpenWorkspaceFile={setPath} roomMembers={[]} />;
  }
  const { rerender } = render(<FullWorkspace agentId="a" />, { wrapper });
  await user.click(await screen.findByRole("button", { name: "nested" }));
  expect(screen.getByRole("button", { name: "readme.md" })).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Toggle preview focus" }));
  expect(screen.queryByRole("button", { name: "nested" })).toBeNull();
  expect(screen.queryByRole("button", { name: "room.workspace_action_upload" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "Toggle preview focus" }));
  expect(screen.getByRole("button", { name: "readme.md" })).toBeTruthy();
  rerender(<FullWorkspace agentId="b" />);
  const nested = await screen.findByRole("button", { name: "nested" });
  expect(nested.getAttribute("aria-expanded")).toBe("false");
  expect(screen.queryByRole("button", { name: "readme.md" })).toBeNull();
  expect(getWorkspaceFilesApi).toHaveBeenCalledWith("b");
  expect(fetch).not.toHaveBeenCalled();
});
