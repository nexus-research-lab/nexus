// INPUT: Structured file evidence entering body, collapsed process and list adapters.
// OUTPUT: Exact artifact/source precedence and readable evidence without preview or ambient scope.
// POS: Real message-to-file DOM regression; only workspace download transport is mocked.

import { createRef, type ReactNode } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { useWorkspaceFilesStore, resetWorkspaceFilesOwnerScope } from "@/store/workspace-files";
import { useAgentStore } from "@/store/agent";
import type { WorkspaceFileArtifactContent } from "@/types/conversation/message/content";
import { ContentBlockView } from "../../item/view/content/content-block-view";
import { projectStructuredContent } from "../../item/view/content/content-renderer-model";
import { AssistantMessageContent } from "../../item/view/assistant/assistant-message-content";
import type { AssistantActivityState, AssistantContentEnvironment, AssistantPermissionState } from "../../item/view/assistant/assistant-message-model";
import { WorkspaceFileArtifactList } from "./workspace-file-artifacts";

vi.mock("@/lib/api/agent/agent-api", () => ({ downloadWorkspaceFileApi: vi.fn().mockResolvedValue(undefined) }));

const ARTIFACT: WorkspaceFileArtifactContent = {
  type: "workspace_file_artifact", scope: "agentWorkspace", path: "reports/result.md", source_tool_use_id: "write-report",
};
const ACTIVITY: AssistantActivityState = { emptyStreamStatus: null, label: null, showCursor: false, standalone: false, state: null, toolUseSummary: null };
const PERMISSIONS: AssistantPermissionState = { all: [], matchedByToolUseId: new Map(), owner: "composer", unmatched: [] };

function localized(children: ReactNode, locale: I18nContextValue["locale"] = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider>;
}

function routeView(route: string, artifact: WorkspaceFileArtifactContent, workspaceAgentId: string | null, onOpenWorkspaceFile: WorkspaceFileOpenHandler) {
  const environment: AssistantContentEnvironment = { canRespondToPermissions: false, hiddenToolNames: [], mode: route === "room result" ? "room_result" : "dm_live", workspaceAgentId, onOpenWorkspaceFile };
  const projection = { content: [artifact], streamingIndexes: new Set<number>() };
  if (route === "body") return <ContentBlockView block={artifact} blockIndex={0} streaming={false} context={{ canRespondToPermissions: false, hiddenToolNames: new Set(), onOpenWorkspaceFile, pendingInteractionOwner: "composer", projection: projectStructuredContent([artifact]), workspaceAgentId }} />;
  if (["archived process", "expanded process", "tool process", "standalone artifact", "room result"].includes(route)) {
    const content = [
      { type: "tool_use" as const, id: "prepare-report", name: "Read", input: { file_path: "input.md" } },
      { type: "tool_result" as const, tool_use_id: "prepare-report", content: "Read" },
      { type: "tool_use" as const, id: "write-report", name: "Write", input: { file_path: artifact.path } },
      { type: "tool_result" as const, tool_use_id: "write-report", content: "Saved" },
      artifact,
    ];
    return <AssistantMessageContent activity={ACTIVITY} environment={environment} permissions={PERMISSIONS}
      direct={{ visible: route === "tool process" || route === "standalone artifact" || route === "room result", projection: { ...projection, content: route === "standalone artifact" ? [artifact] : content } }}
      process={{ anchorRef: createRef(), expanded: route === "expanded process", projection, summary: { errorCount: 0, kind: "details", latestDetail: null }, toggle: vi.fn(), visible: route === "archived process" || route === "expanded process" }}
      final={{ content: "Report complete", visible: true, isStreaming: false, streamingIndexes: new Set(), mentions: [] }}
      showMaxTokensWarning={false} />;
  }
  return <WorkspaceFileArtifactList artifacts={[artifact]} workspaceAgentId={workspaceAgentId} onOpenWorkspaceFile={onOpenWorkspaceFile} />;
}

afterEach(() => {
  act(() => resetWorkspaceFilesOwnerScope());
  act(() => useAgentStore.setState({ current_agent_id: null }));
  vi.clearAllMocks();
});

describe("Structured file source adapters", () => {
  it.each(["body", "archived process", "expanded process", "tool process", "standalone artifact", "room result", "list"])("preserves source and artifact scope through %s and disables unknown scope", (route) => {
    const open = vi.fn();
    useAgentStore.setState({ current_agent_id: "viewer" });
    const { rerender } = render(localized(routeView(route, ARTIFACT, "message-author", open)));
    if (route === "room result") expect(document.querySelector("[data-tool-run-id]")).toBeNull();
    if (route === "tool process") expect(document.querySelector('[data-tool-run-id] [aria-expanded="false"]')).toBeTruthy();
    if (["archived process", "expanded process", "tool process", "standalone artifact", "room result"].includes(route)) {
      const body = screen.getByText("Report complete");
      const file = screen.getByRole("button", { name: /^result\.md/ });
      expect(body.compareDocumentPosition(file) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      expect(screen.getAllByRole("button", { name: /^result\.md/ })).toHaveLength(1);
    }
    fireEvent.click(screen.getByRole("button", { name: /^result\.md/ }));
    expect(open).toHaveBeenLastCalledWith(ARTIFACT.path, "message-author");

    act(() => useAgentStore.setState({ current_agent_id: "other-viewer" }));
    rerender(localized(routeView(route, { ...ARTIFACT, workspace_agent_id: " artifact-author " }, "message-author", open)));
    fireEvent.click(screen.getByRole("button", { name: /^result\.md/ }));
    expect(open).toHaveBeenLastCalledWith(ARTIFACT.path, "artifact-author");
    fireEvent.click(screen.getByRole("button", { name: "Download result.md" }));
    expect(downloadWorkspaceFileApi).toHaveBeenCalledExactlyOnceWith("artifact-author", ARTIFACT.path, "result.md");

    rerender(localized(routeView(route, { ...ARTIFACT, workspace_agent_id: " " }, "new-message-author", open)));
    fireEvent.click(screen.getByRole("button", { name: /^result\.md/ }));
    expect(open).toHaveBeenLastCalledWith(ARTIFACT.path, "new-message-author");
    rerender(localized(routeView(route, ARTIFACT, null, open)));
    const unavailable = screen.getByRole("button", { name: /^result\.md/ });
    expect(unavailable.hasAttribute("disabled")).toBe(true);
    fireEvent.click(unavailable);
    expect(open).toHaveBeenCalledTimes(3);
    expect(screen.queryByRole("button", { name: "Download result.md" })).toBeNull();
    expect(screen.getByText("The source workspace is unavailable, so this file cannot be opened.")).toBeTruthy();
  });
});

it("does not turn references into generated files or replace durable delivery with a listed reference", () => {
  useWorkspaceFilesStore.getState().set_files("author", [{ name: "reference.md", path: "reference.md", depth: 1, is_dir: false, modified_at: "2026-09-10" }]);
  const show = (deliver: boolean) => localized(<AssistantMessageContent activity={ACTIVITY}
    environment={{ mode: "room_result", canRespondToPermissions: false, hiddenToolNames: [], workspaceAgentId: "author" }} permissions={PERMISSIONS}
    direct={{ visible: false, projection: { content: deliver ? [{ ...ARTIFACT, role: "deliverable" }] : [], streamingIndexes: new Set() } }}
    process={{ visible: false, expanded: false, anchorRef: createRef(), toggle: vi.fn(), projection: { content: [], streamingIndexes: new Set() }, summary: { errorCount: 0, kind: "details", latestDetail: null } }}
    final={{ visible: true, isStreaming: false, mentions: [], streamingIndexes: new Set(), content: "参考了 `reference.md` 和 [资料](reference.md)" }} showMaxTokensWarning={false}
  />);
  const { rerender } = render(show(false));
  expect(screen.queryByText("Generated files")).toBeNull();
  rerender(show(true));
  expect(screen.getByText("Generated files")).toBeTruthy();
  expect(screen.getByRole("button", { name: /^result\.md/ })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Download reference.md" })).toBeNull();
});
