// INPUT: 活跃 Goal、Execution 绑定、用量和稳定的动作回调。
// OUTPUT: 证明状态复用 Badge、元信息复用 Typography，且动作仍可访问。
// POS: Goal 状态条 DOM 合同；生命周期投影由纯模型负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { Goal } from "@/types/conversation/goal";

import { GoalStatusStrip } from "./goal-status-strip";

const ACTIVE_GOAL: Goal = {
  continuation_count: 1,
  continuation_state: "ready",
  created_at: "2026-09-04T00:00:00Z",
  empty_progress_count: 0,
  id: "goal-1",
  objective: "统一前端组件规范",
  session_key: "session-1",
  status: "active",
  token_budget: 10_000,
  updated_at: "2026-09-04T00:01:00Z",
  usage: { actual_tokens: 2_500 },
  usage_finalized: false,
  version: 1,
};

describe("GoalStatusStrip", () => {
  it("uses shared badges, typography, and accessible actions", async () => {
    const user = userEvent.setup();
    const onPause = vi.fn();
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <GoalStatusStrip
          canResume={false}
          compact={false}
          disabled={false}
          executionBinding={{ state: "confirmed", execution_id: "execution-1" }}
          goal={ACTIVE_GOAL}
          isGenerating={false}
          isLoading={false}
          mutationBlockReason={null}
          mutationBlocked={false}
          onClearRequest={vi.fn()}
          onEdit={vi.fn()}
          onPause={onPause}
          onRefresh={vi.fn()}
          onResume={vi.fn()}
          scopeLabel="当前 Goal"
        />
      </I18N_CONTEXT.Provider>,
    );

    const status = screen.getByTitle("运行中");
    const binding = container.querySelector('[data-goal-binding-state="confirmed"]');
    const leading = container.querySelector('span[aria-hidden="true"]');
    const objective = screen.getByTitle("统一前端组件规范");
    const panel = container.querySelector("section");
    const usage = container.querySelector(".tabular-nums");

    expect(panel?.className).toContain("bg-transparent");
    expect(panel?.className).toContain("shadow-none");
    expect(leading?.className).toContain("min-h-5");
    expect(leading?.className).toContain("!p-0");
    expect(status.className).toContain("min-h-5");
    expect(status.className).toContain("radius-control-xs");
    expect(binding?.className).toContain("min-h-5");
    expect(objective.className).toContain("ui-type-supporting");
    expect(usage?.className).toContain("ui-type-caption");
    expect(usage?.className).not.toContain("rounded-");

    await user.click(screen.getByRole("button", { name: "暂停" }));
    expect(onPause).toHaveBeenCalledOnce();
  });
});
