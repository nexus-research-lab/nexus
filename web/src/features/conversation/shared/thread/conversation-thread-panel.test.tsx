// INPUT: File-only/final replies through real message and Thread composition.
// OUTPUT: Visible artifact evidence and source callbacks, with Room inspector final-content boundaries intact.
// POS: Whole message visibility regression; no domain services or rendering components are mocked.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { AssistantMessage } from "@/types/conversation/message/entity";
import { MessageItem } from "../message/item/message-item";
import { ConversationThreadPanel } from "./conversation-thread-panel";

const MESSAGE: AssistantMessage = {
  message_id: "final-message", agent_id: "source-author", session_key: "session", round_id: "round", role: "assistant", timestamp: 1, is_complete: true,
  content: [{ type: "text", text: "Final answer" }, { type: "workspace_file_artifact", path: "reports/result.md", workspace_agent_id: "artifact-author" }],
};

describe("File evidence visibility", () => {
  it.each(["dm_live", "dm_archived", "room_result"] as const)("shows one file-only reply in %s without requiring invented prose", (assistantContentMode) => {
    const open = vi.fn();
    const messages: AssistantMessage[] = [
      { ...MESSAGE, message_id: "earlier-progress", content: [{ type: "text", text: "Preparing the report" }] },
      { ...MESSAGE, content: MESSAGE.content.slice(1) },
    ];
    render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}><MessageItem assistantContentMode={assistantContentMode} messages={messages} roundId="round" onOpenWorkspaceFile={open} workspaceAgentId="source-author" /></I18N_CONTEXT.Provider>);
    const artifact = screen.getAllByRole("button", { name: /^result\.md/ });
    expect(artifact).toHaveLength(1);
    fireEvent.click(artifact[0]);
    expect(open).toHaveBeenCalledExactlyOnceWith("reports/result.md", "artifact-author");
  });

  it("shows the full transcript while the Room inspector omits the final answer and its artifact", () => {
    const open = vi.fn();
    const view = (presentation: "transcript" | "inspector") => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}><ConversationThreadPanel agentId="source-author" agentName="Source" messages={[MESSAGE]} roundId="round" presentation={presentation} onClose={vi.fn()} onOpenWorkspaceFile={open} /></I18N_CONTEXT.Provider>;
    const { rerender } = render(view("transcript"));
    expect(screen.getByText("Final answer")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: /^result\.md/ }));
    expect(open).toHaveBeenCalledExactlyOnceWith("reports/result.md", "artifact-author");
    rerender(view("inspector"));
    expect(screen.queryByText("Final answer")).toBeNull();
    expect(screen.queryByRole("button", { name: /^result\.md/ })).toBeNull();
  });

  it("keeps an earlier tool artifact in the inspector process when the final reply contains only prose", () => {
    const process: AssistantMessage = { ...MESSAGE, message_id: "tool-process", content: [
      { type: "tool_use", id: "write", name: "Write", input: { file_path: "reports/result.md" } },
      { type: "tool_result", tool_use_id: "write", content: "Saved" },
      { type: "workspace_file_artifact", path: "reports/result.md", source_tool_use_id: "write", workspace_agent_id: "artifact-author" },
    ] };
    const open = vi.fn();
    render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}><ConversationThreadPanel agentId="source-author" agentName="Source" messages={[process, { ...MESSAGE, content: MESSAGE.content.slice(0, 1) }]} roundId="round" presentation="inspector" onClose={vi.fn()} onOpenWorkspaceFile={open} /></I18N_CONTEXT.Provider>);
    expect(screen.queryByText("Final answer")).toBeNull();
    const files = screen.getAllByRole("button", { name: /^result\.md/ });
    expect(files).toHaveLength(1);
    fireEvent.click(files[0]);
    expect(open).toHaveBeenCalledExactlyOnceWith("reports/result.md", "artifact-author");
  });
});
