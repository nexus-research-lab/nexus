// INPUT: 真实工作图选择器、owner 目录快照与用户搜索/键盘动作。
// OUTPUT: 选项与预览一致，浏览不执行指令，明确复用才写入原始 Slash。
// POS: 无真实目录请求或 Execution mutation 的选择器集成回归。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";
import { WorkGraphDistillationPickerDialog } from "./workgraph-distillation-picker-dialog";

const list = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/conversation/execution-api", () => ({ getWorkGraphWorkflowsApi: list }));
const GRAPH: WorkGraphWorkflow = { id: "review", slash_name: "review-evidence", title: "Evidence review",
  description: "Verify the source evidence.", objective: "Inspect references", built_in: true,
  source_execution_id: "", source_session_key: "", nodes: [], dependencies: [], version: 1, created_at: "", updated_at: "" };
const SECOND = { ...GRAPH, id: "research", slash_name: "research-topic", title: "Research topic", built_in: false };
beforeEach(() => {
  localStorage.setItem(LOCALE_STORAGE_KEY, "en");
  list.mockReset().mockResolvedValue([GRAPH, SECOND]);
});

describe("WorkGraph picker", () => {
  it("focuses search, navigates one option tab stop and only emits the selected command on explicit use", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onUseCommand = vi.fn();
    render(<WorkGraphDistillationPickerDialog isOpen onClose={onClose} onUseCommand={onUseCommand} />,
      { wrapper: I18nProvider });
    const search = screen.getByLabelText("Search WorkGraphs...");
    await waitFor(() => expect(document.activeElement).toBe(search));
    const listbox = await screen.findByRole("listbox");
    const options = within(listbox).getAllByRole("option");
    expect(options.map((option) => option.tabIndex)).toEqual([0, -1]);
    options[0].focus();
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(options[1]);
    expect(options[1].getAttribute("aria-selected")).toBe("true");
    expect(options[0].tabIndex).toBe(-1);
    expect(screen.getByRole("heading", { name: SECOND.title })).toBeTruthy();
    await user.keyboard("{Home}{End}{ArrowUp}");
    expect(document.activeElement).toBe(options[0]);
    expect(onUseCommand).not.toHaveBeenCalled();
    await user.type(search, "research-topic");
    expect(within(listbox).getAllByRole("option")).toHaveLength(1);
    expect(screen.getByRole("heading", { name: SECOND.title })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Use this WorkGraph" }));
    expect(onUseCommand).toHaveBeenCalledExactlyOnceWith("/research-topic ");
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("retries only explicitly and reopens with fresh search state", async () => {
    list.mockRejectedValueOnce(new Error("Offline"));
    const user = userEvent.setup();
    const props = { onClose: vi.fn(), onUseCommand: vi.fn() };
    const { rerender } = render(<WorkGraphDistillationPickerDialog {...props} isOpen />, { wrapper: I18nProvider });
    await screen.findByText("Failed to load WorkGraphs");
    expect(list).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("listbox")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    await screen.findByRole("listbox");
    await user.type(screen.getByLabelText("Search WorkGraphs..."), "missing");
    await user.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(screen.getAllByRole("option")).toHaveLength(2);
    await user.type(screen.getByLabelText("Search WorkGraphs..."), "research");
    rerender(<WorkGraphDistillationPickerDialog {...props} isOpen={false} />);
    rerender(<WorkGraphDistillationPickerDialog {...props} isOpen />);
    await screen.findByRole("listbox");
    expect(screen.getAllByRole("option")).toHaveLength(2);
    expect(props.onUseCommand).not.toHaveBeenCalled();
  });
});
