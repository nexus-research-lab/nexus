// INPUT: rejected、superseded 与普通 Provider tool_result。
// OUTPUT: 证明短状态复用有界共享 Notice，普通宽内容不继承该限制。
// POS: ToolBlock 结果详情 DOM 合同；mutation 语义解析由纯模型测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ToolResultContent } from "@/types/conversation/message/content";

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
      expect(notice.className).toContain("max-w-xl");
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
      .not.toContain("max-w-xl");
  });
});
