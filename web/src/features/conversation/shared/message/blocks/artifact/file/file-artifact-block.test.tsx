// INPUT: Real file-artifact DOM, explicit/current Agent scope and independent open/download capabilities.
// OUTPUT: Exact-scope commands, disabled preview semantics and explicit label suppression.
// POS: File Artifact integration tests; only the API transport is replaced with a local spy.

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
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
});
