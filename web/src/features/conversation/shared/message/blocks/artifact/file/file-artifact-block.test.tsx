// INPUT: Real file-artifact DOM, explicit/missing scope and independent open/download capabilities.
// OUTPUT: Exact-scope commands, no ambient fallback, localized missing identity and label suppression.
// POS: File Artifact integration tests; only the API transport is replaced with a local spy.

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { useAgentStore } from "@/store/agent";
import { FileArtifactBlock } from "./file-artifact-block";

vi.mock("@/lib/api/agent/agent-api", () => ({ downloadWorkspaceFileApi: vi.fn().mockResolvedValue(undefined) }));

afterEach(() => {
  act(() => useAgentStore.setState({ current_agent_id: null }));
  vi.clearAllMocks();
});

describe("File Artifact", () => {
  it("keeps preview and download bound to the source Agent after global selection changes", () => {
    const open = vi.fn();
    useAgentStore.setState({ current_agent_id: "viewer" });
    render(<I18nProvider><FileArtifactBlock displayPath="reports/visible.md" path="reports/source.md" workspaceAgentId="author" onOpenWorkspaceFile={open} /></I18nProvider>);
    act(() => useAgentStore.setState({ current_agent_id: "another-viewer" }));
    fireEvent.click(screen.getByRole("button", { name: /^visible\.md/ }));
    expect(open).toHaveBeenCalledExactlyOnceWith("reports/source.md", "author");
    expect(downloadWorkspaceFileApi).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: /Download visible\.md|下载 visible\.md/ }));
    expect(downloadWorkspaceFileApi).toHaveBeenCalledExactlyOnceWith("author", "reports/source.md", "visible.md");
    expect(open).toHaveBeenCalledOnce();
  });

  it("keeps download available when preview has no handler, and preserves an explicitly hidden label", () => {
    render(<I18nProvider><FileArtifactBlock label="" path="reports/source.md" workspaceAgentId="author" /></I18nProvider>);
    const preview = screen.getByRole("button", { name: /^source\.md/ });
    expect(preview.hasAttribute("disabled")).toBe(true);
    fireEvent.click(preview);
    expect(downloadWorkspaceFileApi).not.toHaveBeenCalled();
    expect(screen.queryByText(/Saved to|已保存到/)).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Download source\.md|下载 source\.md/ }));
    expect(downloadWorkspaceFileApi).toHaveBeenCalledExactlyOnceWith("author", "reports/source.md", "source.md");
  });

  it.each([undefined, null, "", "  "])("keeps unscoped evidence visible and disables both actions with scope %s", (workspaceAgentId) => {
    const open = vi.fn();
    useAgentStore.setState({ current_agent_id: "unrelated-selection" });
    render(<I18nProvider><FileArtifactBlock path="reports/source.md" workspaceAgentId={workspaceAgentId} onOpenWorkspaceFile={open} /></I18nProvider>);
    const preview = screen.getByRole("button", { name: /^source\.md/ });
    expect(preview.hasAttribute("disabled")).toBe(true);
    const explanation = document.getElementById(preview.getAttribute("aria-describedby")!);
    expect(explanation?.textContent).toMatch(/来源工作区|source workspace/);
    expect(screen.getByText("reports")).toBeTruthy();
    expect(screen.getAllByRole("button")).toHaveLength(1);
    fireEvent.click(preview);
    expect(open).not.toHaveBeenCalled();
    expect(downloadWorkspaceFileApi).not.toHaveBeenCalled();
  });

  it("updates current-language missing-path explanations and localized default names", () => {
    const view = (locale: I18nContextValue["locale"]) => (
      <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
        <FileArtifactBlock label="" path="  " workspaceAgentId="author" onOpenWorkspaceFile={vi.fn()} />
      </I18N_CONTEXT.Provider>
    );
    const { rerender } = render(view("zh"));
    expect(screen.getByRole("button", { name: /^文件/ }).hasAttribute("disabled")).toBe(true);
    expect(screen.getByText("缺少文件路径，暂时无法打开此文件。")).toBeTruthy();
    rerender(view("en"));
    expect(screen.getByRole("button", { name: /^file/ }).hasAttribute("disabled")).toBe(true);
    expect(screen.getByText("The file path is unavailable, so this file cannot be opened.")).toBeTruthy();
    expect(screen.getAllByRole("button")).toHaveLength(1);
  });

  it("removes previously available actions when the source workspace is cleared", () => {
    const open = vi.fn();
    const view = (scope: string | null) => <I18nProvider><FileArtifactBlock path="reports/source.md" workspaceAgentId={scope} onOpenWorkspaceFile={open} /></I18nProvider>;
    const { rerender } = render(view("author"));
    expect(screen.getAllByRole("button")).toHaveLength(2);
    useAgentStore.setState({ current_agent_id: "viewer" });
    rerender(view(null));
    fireEvent.click(screen.getByRole("button", { name: /^source\.md/ }));
    expect(screen.getAllByRole("button")).toHaveLength(1);
    expect(open).not.toHaveBeenCalled();
    expect(downloadWorkspaceFileApi).not.toHaveBeenCalled();
  });
});
