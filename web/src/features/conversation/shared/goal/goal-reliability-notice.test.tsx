// INPUT: Proven mutation outcomes, unresolved reads and explicit refresh callbacks.
// OUTPUT: Evidence that stale success remains success and offers only read recovery.
// POS: Goal feedback DOM regressions; no mutation transport or raw diagnostics are rendered.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { GoalReliabilityState } from "./goal-lifecycle-recovery";
import { GoalReliabilityNotice } from "./goal-reliability-notice";
import { goalTestI18n } from "./goal.test-support";

const STATE: GoalReliabilityState = {
  kind: "mutation_applied", operation: "update", stale: true, access: null,
  sessionKey: "internal-session", detail: "internal-agent-123 and raw transport details",
};

describe("GoalReliabilityNotice", () => {
  it.each(["en", "zh"] as const)("retains proven success and read recovery in %s", (locale) => {
    const context = goalTestI18n(locale);
    const onRefresh = vi.fn();
    const { container, rerender } = render(<I18N_CONTEXT.Provider value={context}>
      <GoalReliabilityNotice state={STATE} isRefreshing={false} mutationBlocked={false} onRefresh={onRefresh} />
    </I18N_CONTEXT.Provider>);
    expect(container.querySelector('[data-resource-state="success"]')).toBeTruthy();
    expect(container.textContent).not.toContain("internal-");
    fireEvent.click(screen.getByRole("button", { name: context.t("state.reload_check") }));
    expect(onRefresh).toHaveBeenCalledOnce();
    rerender(<I18N_CONTEXT.Provider value={context}>
      <GoalReliabilityNotice state={STATE} isRefreshing mutationBlocked={false} onRefresh={onRefresh} />
    </I18N_CONTEXT.Provider>);
    const refresh = screen.getByRole("button", { name: context.t("state.reload_check") }) as HTMLButtonElement;
    expect(refresh.disabled).toBe(true);
    fireEvent.click(refresh);
    expect(onRefresh).toHaveBeenCalledOnce();
  });
  it("omits recovery for fresh success but exposes it for an unresolved mutation", () => {
    const onRefresh = vi.fn();
    const element = (state: GoalReliabilityState) => <I18N_CONTEXT.Provider value={goalTestI18n("en")}>
      <GoalReliabilityNotice state={state} isRefreshing={false} mutationBlocked={state.kind === "mutation_unknown"} onRefresh={onRefresh} />
    </I18N_CONTEXT.Provider>;
    const { rerender, container } = render(element({ ...STATE, stale: false }));
    expect(screen.queryByRole("button")).toBeNull();
    rerender(element({ ...STATE, kind: "mutation_unknown" }));
    expect(container.querySelector('[data-resource-state="error"]')).toBeTruthy();
    expect(screen.getAllByRole("button")).toHaveLength(1);
    fireEvent.click(screen.getByRole("button"));
    expect(onRefresh).toHaveBeenCalledOnce();
  });
});
