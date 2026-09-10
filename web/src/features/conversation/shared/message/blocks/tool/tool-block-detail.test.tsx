// INPUT: rejected、superseded 与普通 Provider tool_result。
// OUTPUT: 证明短状态复用共享紧凑 Notice，普通宽内容不继承该限制。
// POS: ToolBlock 结果详情 DOM 合同；mutation 语义解析由纯模型测试负责。

import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ToolResultContent } from "@/types/conversation/message/content";

import type { MessageDetailResponse } from "@/types/conversation/history";
import { getSessionMessageDetailApi } from "@/lib/api/conversation/session-api";
vi.mock("@/lib/api/conversation/session-api", () => ({ getSessionMessageDetailApi: vi.fn() }));

import { ToolBlockResult } from "./tool-block-detail";

function mutationResult(
  outcome: "rejected" | "superseded",
): ToolResultContent {
  return {
    content: null,
    is_error: false,
    structured_output: {
      message: outcome === "rejected"
        ? "abstraction omitted the terminal delivery"
        : "旧工作已被新目标替换",
      outcome,
      reason_code: outcome === "rejected"
        ? "terminal_delivery_missing"
        : "execution_terminal",
    },
    tool_use_id: `tool-${outcome}`,
    type: "tool_result",
  };
}

describe("ToolBlockResult", () => {
  it("ignores a late success from a cancelled detail request", async () => {
    let resolveOld!: (value: MessageDetailResponse) => void;
    let resolveNew!: (value: MessageDetailResponse) => void;
    vi.mocked(getSessionMessageDetailApi)
      .mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve; }))
      .mockImplementationOnce(() => new Promise((resolve) => { resolveNew = resolve; }));
    const view = (ref: string) => <I18nProvider><ToolBlockResult toolResult={{ type: "tool_result", tool_use_id: "tool", content: "Preview", detail_session_key: "session", detail_ref: ref }} /></I18nProvider>;
    const { rerender } = render(view("old"));
    rerender(view("new"));
    await act(async () => resolveNew({ ref: "new", kind: "tool_result", byte_size: 10, content: "New detail" }));
    expect(screen.getByText("New detail")).toBeTruthy();
    await act(async () => resolveOld({ ref: "old", kind: "tool_result", byte_size: 10, content: "Old detail" }));
    expect(screen.getByText("New detail")).toBeTruthy();
    expect(screen.queryByText("Old detail")).toBeNull();
  });

  it.each([
    ["rejected", "danger", "terminal_delivery_missing"],
    ["superseded", "neutral", "execution_terminal"],
  ] as const)(
    "renders %s mutations as bounded shared notices",
    (outcome, tone, reasonCode) => {
      render(
        <I18nProvider>
          <ToolBlockResult toolResult={mutationResult(outcome)} />
        </I18nProvider>,
      );

      const notice = screen.getByRole("status");
      expect(notice.getAttribute("data-inline-notice-variant")).toBe("contained");
      expect(notice.getAttribute("data-inline-notice-tone")).toBe(tone);
      expect(notice.getAttribute("data-tool-result-semantic-outcome")).toBe(outcome);
      expect(notice.getAttribute("data-inline-notice-width")).toBe("compact");
      expect(notice.className).toContain("max-w-sm");
      expect(screen.getByText(reasonCode).tagName).toBe("CODE");
    },
  );

  it("keeps ordinary tool output outside the short-status width contract", () => {
    render(
      <I18nProvider>
        <ToolBlockResult
          toolResult={{
            content: "ordinary full-width tool output",
            is_error: false,
            tool_use_id: "tool-success",
            type: "tool_result",
          }}
        />
      </I18nProvider>,
    );

    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.getByText("ordinary full-width tool output").className)
      .not.toContain("max-w-sm");
  });
});
