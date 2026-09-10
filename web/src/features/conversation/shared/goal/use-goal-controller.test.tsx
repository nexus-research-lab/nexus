// INPUT: Authoritative scoped resource snapshots, lifecycle replies and UI commands.
// OUTPUT: Evidence for scoped drafts/confirmations, safe budget requests and localized panel composition.
// POS: Controller and real GoalPanel integration tests; the resource adapter is isolated from HTTP.
import type { FormEvent, ReactNode } from "react";
import { act, fireEvent, render, renderHook, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { Locale } from "@/shared/i18n/messages";
import { clearGoalApi, updateGoalApi } from "@/lib/api/conversation/goal-api";
import { GoalPanel } from "./goal-panel";
import { useGoalController } from "./use-goal-controller";
import { useGoalResource } from "./use-goal-resource";
import { ACTIVE_GOAL, goalTestI18n } from "./goal.test-support";

vi.mock("./use-goal-resource", () => ({ useGoalResource: vi.fn() }));
vi.mock("@/lib/api/conversation/goal-api", () => ({
  clearGoalApi: vi.fn(), updateGoalApi: vi.fn(), pauseGoalApi: vi.fn(), resumeGoalApi: vi.fn(),
}));

let resource: ReturnType<typeof useGoalResource>;
let locale: Locale;
function Wrapper({ children }: { children: ReactNode }) {
  return <I18N_CONTEXT.Provider value={goalTestI18n(locale)}>{children}</I18N_CONTEXT.Provider>;
}
function submitEvent() { return { preventDefault: vi.fn() } as unknown as FormEvent; }
function renderController() {
  return renderHook(({ sessionKey }) => useGoalController({ disabled: false, sessionKey }), {
    initialProps: { sessionKey: ACTIVE_GOAL.session_key }, wrapper: Wrapper,
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  locale = "en";
  resource = {
    goal: ACTIVE_GOAL, executionBinding: { state: "standalone" }, isLoading: false,
    mutationBlockReason: null, mutationsBlocked: false, ownerScopeGeneration: 1,
    phase: null, refresh: vi.fn(async () => {}), reliability: null,
    runCommand: vi.fn(async (_input, command) => ({ ok: true, goal: await command(resource.goal!.id) })),
  };
  vi.mocked(useGoalResource).mockImplementation(() => resource);
  vi.mocked(updateGoalApi).mockResolvedValue(ACTIVE_GOAL);
});

describe("Goal controller interaction scope", () => {
  it.each(["100.5", "100abc", "0", "9007199254740992"])("never dispatches invalid budget %s", async (budget) => {
    const { result } = renderController();
    act(() => result.current.actions.startEditing());
    act(() => result.current.actions.setBudget(budget));
    await act(() => result.current.actions.submit(submitEvent()));
    expect(resource.runCommand).not.toHaveBeenCalled();
    expect(updateGoalApi).not.toHaveBeenCalled();
    expect(result.current.draft?.budget).toBe(budget);
  });
  it.each([
    { current: 10000, input: "", expected: null },
    { current: null, input: "", expected: undefined },
    { current: 10000, input: " 012000 ", expected: 12000 },
  ])("preserves explicit removal and omission semantics: %j", async ({ current, input, expected }) => {
    resource.goal = { ...ACTIVE_GOAL, token_budget: current };
    const { result } = renderController();
    act(() => result.current.actions.startEditing());
    act(() => result.current.actions.setBudget(input));
    await act(() => result.current.actions.submit(submitEvent()));
    expect(updateGoalApi).toHaveBeenCalledWith(ACTIVE_GOAL.id, {
      objective: ACTIVE_GOAL.objective, token_budget: expected,
    });
    expect(result.current.draft).toBeNull();
  });
  it.each(["session", "owner", "goal"])("clears old UI state on %s changes without restoring it when the old target returns", (kind) => {
    const { result, rerender } = renderController();
    act(() => { result.current.actions.startEditing(); result.current.actions.startClearing(); });
    expect(result.current.draft).not.toBeNull();
    expect(result.current.dialog.kind).toBe("clear");
    const oldResource = resource;
    resource = { ...resource, ownerScopeGeneration: kind === "owner" ? 2 : 1,
      goal: { ...ACTIVE_GOAL, id: kind === "goal" ? "goal-2" : ACTIVE_GOAL.id,
        session_key: kind === "session" ? "session-2" : ACTIVE_GOAL.session_key } };
    rerender({ sessionKey: resource.goal!.session_key });
    expect(result.current.draft).toBeNull();
    expect(result.current.dialog.kind).toBe("none");
    resource = oldResource;
    rerender({ sessionKey: ACTIVE_GOAL.session_key });
    expect(result.current.draft).toBeNull();
    expect(result.current.dialog.kind).toBe("none");
  });
  it.each(["objective", "binding", "busy"])("consumes a confirmation invalidated by %s changes", (kind) => {
    const { result, rerender } = renderController();
    act(() => result.current.actions.startClearing());
    const before = resource;
    resource = { ...resource,
      goal: kind === "objective" ? { ...ACTIVE_GOAL, objective: "Replacement objective" } : ACTIVE_GOAL,
      executionBinding: { state: kind === "binding" ? "confirmed" : "standalone" },
      isLoading: kind === "busy",
    };
    rerender({ sessionKey: ACTIVE_GOAL.session_key });
    expect(result.current.dialog.kind).toBe("none");
    resource = before;
    rerender({ sessionKey: ACTIVE_GOAL.session_key });
    expect(result.current.dialog.kind).toBe("none");
    act(() => result.current.actions.confirmDialog());
    expect(clearGoalApi).not.toHaveBeenCalled();
  });
  it("keeps draft and confirmation through language/progress updates, and clears a committed draft", () => {
    const { result, rerender } = renderController();
    act(() => { result.current.actions.startEditing(); result.current.actions.startClearing(); });
    act(() => result.current.actions.setObjective("Unsaved text"));
    locale = "zh";
    resource = { ...resource, goal: { ...ACTIVE_GOAL, version: 2, usage: { actual_tokens: 4000 } } };
    rerender({ sessionKey: ACTIVE_GOAL.session_key });
    expect(result.current.dialog.kind).toBe("clear");
    expect(result.current.draft?.objective).toBe("Unsaved text");
    resource = { ...resource, reliability: {
      kind: "mutation_applied", operation: "update", access: null, detail: "", stale: true,
      sessionKey: ACTIVE_GOAL.session_key, ownerScopeGeneration: 1, blocksMutations: false,
    } };
    rerender({ sessionKey: ACTIVE_GOAL.session_key });
    expect(result.current.draft).toBeNull();
  });
  it("distinguishes read recovery from saving while preserving a locked unknown-result draft", async () => {
    const { result, rerender } = renderController();
    act(() => result.current.actions.startEditing());
    resource = { ...resource, isLoading: true };
    rerender({ sessionKey: ACTIVE_GOAL.session_key });
    expect(result.current.loadingLabel).toBe(goalTestI18n("en").t("state.reload_check"));
    expect(result.current.pendingAction).toBeNull();
    resource = { ...resource, isLoading: false, mutationsBlocked: true, mutationBlockReason: "unknown_mutation" };
    rerender({ sessionKey: ACTIVE_GOAL.session_key });
    await act(() => result.current.actions.submit(submitEvent()));
    expect(resource.runCommand).not.toHaveBeenCalled();
    expect(result.current.draft?.objective).toBe(ACTIVE_GOAL.objective);
    act(() => result.current.actions.cancelEditing());
    expect(result.current.draft).toBeNull();
  });
  it("does not clear a newer draft when an older command promise finishes", async () => {
    let complete!: (value: Awaited<ReturnType<typeof resource.runCommand>>) => void;
    resource.runCommand = vi.fn(() => new Promise<Awaited<ReturnType<typeof resource.runCommand>>>((resolve) => { complete = resolve; }));
    const { result, rerender } = renderController();
    act(() => result.current.actions.startEditing());
    let submission!: Promise<void>;
    act(() => { submission = result.current.actions.submit(submitEvent()); });
    resource = { ...resource, goal: { ...ACTIVE_GOAL, id: "new-goal", session_key: "new-session" } };
    rerender({ sessionKey: "new-session" });
    act(() => result.current.actions.startEditing());
    act(() => result.current.actions.setObjective("New draft"));
    await act(async () => { complete({ ok: true, goal: ACTIVE_GOAL }); await submission; });
    expect(result.current.draft?.objective).toBe("New draft");
  });
});

describe("GoalPanel composition", () => {
  it("localizes clear confirmation and submits only the current valid confirmation", async () => {
    render(<Wrapper><GoalPanel sessionKey={ACTIVE_GOAL.session_key} /></Wrapper>);
    expect(screen.getByText("Session Goal")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Clear" }));
    const dialog = screen.getByRole("dialog", { name: "Clear the current Goal?" });
    expect(dialog.textContent).toContain(ACTIVE_GOAL.objective);
    await act(async () => { fireEvent.click(within(dialog).getByRole("button", { name: "Clear" })); });
    expect(clearGoalApi).toHaveBeenCalledWith(ACTIVE_GOAL.id);
  });
  it("shows recovery once while editing and restores the lane after closing", () => {
    resource.reliability = { kind: "binding_failed", access: null, detail: "internal diagnostic", operation: null,
      stale: true, sessionKey: ACTIVE_GOAL.session_key, ownerScopeGeneration: 1, blocksMutations: false };
    render(<Wrapper><GoalPanel sessionKey={ACTIVE_GOAL.session_key} /></Wrapper>);
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    const notices = () => document.querySelectorAll('[data-goal-reliability-kind="binding_failed"]');
    expect(notices()).toHaveLength(1);
    expect(screen.getByRole("dialog", { name: "Edit Goal" }).contains(notices()[0])).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(notices()).toHaveLength(1);
  });
});
