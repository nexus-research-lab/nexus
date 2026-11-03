// INPUT: Goal 生命周期、Execution 绑定、用量、长阻塞说明和操作阶段。
// OUTPUT: 证明状态与动作随语言更新、阻塞说明完整、忙碌归属准确，且复用 Badge/Typography。
// POS: Goal 状态条 DOM 合同；生命周期投影由纯模型负责。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { GoalStatusStrip } from "./goal-status-strip";

import { ACTIVE_GOAL, goalTestI18n } from "./goal.test-support";

describe("GoalStatusStrip", () => {
  it("uses shared badges, typography, and accessible actions", async () => {
    const user = userEvent.setup();
    const onPause = vi.fn();
    render(
<I18N_CONTEXT.Provider
value={goalTestI18n("zh")}
>
<GoalStatusStrip
canResume={false}
compact={false}
disabled={false}
executionBinding={{ state: "confirmed",execution_id: "execution-1" }}
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
scopeLabel="当前 Goal" />
</I18N_CONTEXT.Provider>
);

    await user.click(screen.getByRole("button", { name: "暂停" }));
    expect(onPause).toHaveBeenCalledOnce();
  });
});

const STRIP_PROPS = {
  canResume: false, compact: false, disabled: false, goal: ACTIVE_GOAL,
  isGenerating: false, isLoading: false, mutationBlockReason: null, mutationBlocked: false,
  scopeLabel: "Session Goal", onClearRequest: vi.fn(), onEdit: vi.fn(), onPause: vi.fn(),
  onRefresh: vi.fn(), onResume: vi.fn(),
};

describe("Goal status readability and busy actions", () => {
  it("localizes on language changes and assigns busy state to the actual command", () => {
    const element = (locale: "en" | "zh") => <I18N_CONTEXT.Provider value={goalTestI18n(locale)}>
      <GoalStatusStrip {...STRIP_PROPS} isLoading pendingAction="pause" />
    </I18N_CONTEXT.Provider>;
    const { rerender } = render(element("en"));
    const pause = screen.getByRole("button", { name: "Pause" });
    expect(pause.getAttribute("aria-busy")).toBe("true");

    expect(screen.getByRole("button", { name: "Refresh" }).getAttribute("aria-busy")).toBeNull();
    expect(screen.getByText("Active")).toBeTruthy();
    rerender(element("zh"));
    expect(screen.getByText("运行中")).toBeTruthy();
    expect(screen.getByRole("button", { name: "暂停" }).getAttribute("aria-busy")).toBe("true");
  });
  it("keeps long blocking instructions available and preserves the safe refresh action while mutations are locked", () => {
    const onRefresh = vi.fn();
    const reason = "A long reason ".repeat(25);
    const neededInput = "Please provide the complete review evidence.";
    render(<I18N_CONTEXT.Provider value={goalTestI18n("en")}>
      <GoalStatusStrip {...STRIP_PROPS} mutationBlocked mutationBlockReason="stale_read" onRefresh={onRefresh}
        goal={{ ...ACTIVE_GOAL, status: "blocked", blocker: {
          id: "internal-blocker", reason, needed_input: neededInput, since_objective_revision: 1,
        } }} />
    </I18N_CONTEXT.Provider>);
    const attention = screen.getByText((_, node) => node?.tagName === "DIV"
      && node.textContent?.includes(neededInput) === true && node.className.includes("whitespace-pre-wrap"));
    expect(attention.textContent).toContain(reason);

    expect((screen.getByRole("button", { name: /^Edit:/ }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
    expect(onRefresh).toHaveBeenCalledOnce();
  });
});
