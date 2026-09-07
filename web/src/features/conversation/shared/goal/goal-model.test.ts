// INPUT: Whole-budget text, lifecycle states, usage facts and binding snapshots.
// OUTPUT: Regression evidence for safe budgets, localized states and clear confirmation identity.
// POS: Goal presentation policy tests; mutation transport remains outside this model.
import { describe, expect, it } from "vitest";
import type { Goal, GoalExecutionBinding } from "@/types/conversation/goal";
import {
  buildGoalStatusStripModel,
  buildGoalControllerProjection,
  parseGoalBudgetInput,
  resolveGoalClearDisabledReason,
} from "./goal-model";
import { ACTIVE_GOAL, goalTestI18n } from "./goal.test-support";

const i18n = goalTestI18n("en");
function status(goal: Goal, isGenerating = false) {
  return buildGoalStatusStripModel({
    ...i18n, goal, canResume: true, continuationHold: null, isGenerating,
  });
}

describe("Goal presentation policy", () => {
  it.each(["0", "-10", "+10", "100.5", "100abc", "1e3", "1,000", "1 000", "9007199254740992", "Infinity"])(
    "rejects the complete invalid budget %s without truncation or removal", (value) => {
      expect(parseGoalBudgetInput(value)).toEqual({ valid: false });
    },
  );
  it("distinguishes no limit from a valid whole budget, including the exact safe upper bound", () => {
    expect(parseGoalBudgetInput(" \n ")).toEqual({ valid: true, value: null });
    expect(parseGoalBudgetInput(" 00100 ")).toEqual({ valid: true, value: 100 });
    expect(parseGoalBudgetInput("9007199254740991")).toEqual({ valid: true, value: Number.MAX_SAFE_INTEGER });
  });
  it("keeps lifecycle, live execution and suspended continuation distinct", () => {
    expect(status(ACTIVE_GOAL).statusLabel).toBe("Active");
    expect(status(ACTIVE_GOAL, true).statusLabel).toBe("Executing");
    expect(status({ ...ACTIVE_GOAL, continuation_state: "suspended" }).statusLabel).toBe("Auto-continue stopped");
    expect(status({ ...ACTIVE_GOAL, status: "paused", continuation_state: "suspended" }).statusLabel).toBe("Paused");
    expect(status({ ...ACTIVE_GOAL, empty_progress_count: 999 }).attentionMessage).toBeNull();
  });
  it.each(["future", "constructor", "__proto__"])("handles unknown wire states %s conservatively", (state) => {
    const projected = status({ ...ACTIVE_GOAL, status: state as Goal["status"] });
    expect(projected.statusLabel).toBe("Status unknown");
    expect(projected.tone).toBe("idle");
    expect(projected.actions).toEqual(["refresh"]);
    const binding = { state } as GoalExecutionBinding;
    expect(resolveGoalClearDisabledReason(binding, i18n.t)).toBeTruthy();
    expect(buildGoalStatusStripModel({ ...i18n, goal: ACTIVE_GOAL, canResume: false, isGenerating: false,
      continuationHold: null, executionBinding: binding }).bindingBadge?.state).toBe("unavailable");
  });
  it("preserves estimated actual usage and hides unfinished completed totals", () => {
    expect(status(ACTIVE_GOAL).usageLabel).toBe("2,500 tokens");
    expect(status({ ...ACTIVE_GOAL, usage: { input_tokens: 100, output_tokens: 20, reasoning_tokens: 30 } }).usageLabel).toBe("≈130 tokens");
    expect(status({ ...ACTIVE_GOAL, status: "complete" }).usageLabel).toBeNull();
    expect(status({ ...ACTIVE_GOAL, status: "complete", usage_finalized: true }).usageLabel).toBe("2,500 tokens");
  });
  it("rejects a clear confirmation whose displayed objective changed, preserving ordinary progress refreshes", () => {
    const input = { t: i18n.t, dialog: { kind: "clear" as const, goal: ACTIVE_GOAL }, draft: null,
      executionBinding: { state: "standalone" as const }, phase: null };
    expect(buildGoalControllerProjection({ ...input, goal: { ...ACTIVE_GOAL, objective: "New objective" } }).dialog.kind).toBe("none");
    expect(buildGoalControllerProjection({ ...input, goal: { ...ACTIVE_GOAL, version: 2 } }).dialog.kind).toBe("clear");
    expect(buildGoalControllerProjection({ ...input, goal: ACTIVE_GOAL, executionBinding: { state: "confirmed" } }).dialog.kind).toBe("none");
  });
});
