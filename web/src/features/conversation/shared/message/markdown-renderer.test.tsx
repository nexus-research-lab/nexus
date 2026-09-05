// INPUT: 会话 Markdown、Agent mention/handoff、流式终态与不同 Agent/owner 的文件快照。
// OUTPUT: 验证领域链接仅在消费侧解释，资源交互始终使用消息归属。
// POS: Message Markdown 适配器的集成 DOM 行为测试。

import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY, MESSAGES } from "@/shared/i18n/messages";
import { useAgentStore } from "@/store/agent";
import { resetWorkspaceFilesOwnerScope, useWorkspaceFilesStore } from "@/store/workspace-files";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import { AgentHandoffStatusProvider } from "./agent-handoff-status-context";
import { MarkdownRenderer } from "./markdown-renderer";

function workspaceFile(path: string): WorkspaceFileEntry {
  return { depth: 1, is_dir: false, modified_at: "2026-09-05", name: path.split("/").at(-1)!, path };
}

beforeEach(() => {
  window.localStorage.setItem(LOCALE_STORAGE_KEY, "en");
  resetWorkspaceFilesOwnerScope();
  useAgentStore.setState({ current_agent_id: "selected-agent" });
});

afterEach(() => {
  cleanup();
  resetWorkspaceFilesOwnerScope();
  useAgentStore.setState({ current_agent_id: null });
});

describe("Message Markdown domain adaptation", () => {
  it("keeps mention identity and exact handoff context through streaming completion", async () => {
    const user = userEvent.setup();
    const openContact = vi.fn();
    const directory = { names: { "target/agent": "Teammate" } };
    const renderConversation = (streaming: boolean, phase: "active" | "responded") => (
      <I18nProvider>
        <AgentHandoffStatusProvider statuses={{ "handoff-one": phase, "handoff-other": "queued" }}>
          <MarkdownRenderer
            agentMentionDirectory={directory}
            content="[/goal](#nexus-slash-command=goal) [@Teammate](agent-mention://target%2Fagent?handoff_id=handoff-one) [Docs](https://example.com/docs)"
            isStreaming={streaming}
            onOpenAgentContact={openContact}
            workspaceAgentId="message-agent"
          />
        </AgentHandoffStatusProvider>
      </I18nProvider>
    );
    const { container, rerender } = render(renderConversation(true, "active"));
    const mention = screen.getByRole("button", { name: /Teammate/ });
    expect(within(mention).getByRole("status").textContent).toBe(MESSAGES.en["room.agent_handoff_active"]);
    expect(container.querySelector('[data-slash-command-token="true"]')?.textContent).toBe("/goal");
    await user.click(mention);
    expect(openContact).toHaveBeenLastCalledWith("target/agent");

    rerender(renderConversation(false, "responded"));
    await user.click(screen.getByRole("button", { name: /Teammate/ }));
    expect(openContact).toHaveBeenLastCalledWith("target/agent");
    expect(screen.getByRole("link", { name: "Docs" }).getAttribute("href")).toBe("https://example.com/docs");
    expect(screen.queryByRole("link", { name: "/goal" })).toBeNull();
    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(screen.getByRole("status").textContent).toBe(MESSAGES.en["room.agent_handoff_responded"]);
  });

  it("decorates accepted mention spans and leading Slash while rejecting malformed or unsafe links", () => {
    const { container } = render(
      <I18nProvider>
        <MarkdownRenderer
          agentMentions={[{ agent_id: "exact-agent", content_block_index: 0, end_rune: 12, handoff_id: "handoff", label: "@Agent", start_rune: 6 }]}
          content="/goal @Agent [broken](agent-mention://%ZZ) [unsafe](javascript:alert%281%29)"
          renderLeadingSlashCommand
        />
      </I18nProvider>,
    );
    expect(container.querySelector('[data-slash-command-token="true"]')?.textContent).toBe("/goal");
    expect(screen.getByRole("button", { name: /@Agent/ })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "broken" })).toBeNull();
    expect(screen.queryByRole("link", { name: "unsafe" })).toBeNull();
  });

  it("binds file and image commands to exact message Agent across selection and scope changes", async () => {
    const user = userEvent.setup();
    const openFile = vi.fn();
    useWorkspaceFilesStore.getState().set_files("message-agent", [workspaceFile("message/report.md")]);
    useWorkspaceFilesStore.getState().set_files("other-agent", [workspaceFile("other/report.md")]);
    useWorkspaceFilesStore.getState().set_files("selected-agent", [workspaceFile("wrong/report.md")]);
    const content = '`report.md` [Read](report.md) ![Chart](images/chart.png)';
    const { rerender } = render(<MarkdownRenderer content={content} onOpenWorkspaceFile={openFile} workspaceAgentId="message-agent" />);

    act(() => useAgentStore.setState({ current_agent_id: "other-agent" }));
    await user.click(screen.getByRole("button", { name: "report.md" }));
    await user.click(screen.getByRole("button", { name: "Read" }));
    await user.click(screen.getByRole("button", { name: "Chart" }));
    expect(openFile.mock.calls).toEqual([["message/report.md", "message-agent"], ["message/report.md", "message-agent"], ["images/chart.png", "message-agent"]]);
    expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toContain("/agents/message-agent/workspace/download?");

    rerender(<MarkdownRenderer content={content} onOpenWorkspaceFile={openFile} workspaceAgentId="other-agent" />);
    await user.click(screen.getByRole("button", { name: "report.md" }));
    expect(openFile).toHaveBeenLastCalledWith("other/report.md", "other-agent");
    expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toContain("/agents/other-agent/workspace/download?");
  });

  it("removes old owner file lookup immediately and refuses ambiguous basenames", () => {
    useWorkspaceFilesStore.getState().set_files("message-agent", [workspaceFile("owner-one/report.md")]);
    render(<MarkdownRenderer content="`report.md`" onOpenWorkspaceFile={vi.fn()} workspaceAgentId="message-agent" />);
    expect(screen.getByRole("button", { name: "report.md" })).toBeTruthy();

    act(() => resetWorkspaceFilesOwnerScope());
    expect(screen.queryByRole("button", { name: "report.md" })).toBeNull();
    act(() => useWorkspaceFilesStore.getState().set_files("message-agent", [workspaceFile("a/report.md"), workspaceFile("b/report.md")]));
    expect(screen.queryByRole("button", { name: "report.md" })).toBeNull();
  });
});
