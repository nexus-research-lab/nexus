// INPUT: Menu and typed Slash actions against the same Composer draft.
// OUTPUT: Both entrances select the same hidden Plan draft mode, preserving the task text.
// POS: Planning entry behavior regression for the shared Slash controller.
import { createRef } from "react";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { CommandCatalogData, CommandDescriptor } from "@/types/generated/protocol";
import { useComposerSlashCommand } from "./use-composer-slash-command";

import { useComposerDraft } from "./controller/use-composer-draft";
import { resetComposerDraftOwnerScope } from "./composer-draft-store";

beforeEach(resetComposerDraftOwnerScope);

const command = { name: "plan", enabled: true, execution: "runtime" } as CommandDescriptor;

function useEntry(scope: string) {
  const draft = useComposerDraft(scope);
  const { input, inputMode } = draft.state;
  const { setInput } = draft;
  const slash = useComposerSlashCommand({
    catalog: { commands: [command], status: "ready" } as CommandCatalogData,
    input, setInput, isGoalMode: false, isPlanMode: inputMode === "plan", onPlanToggle: draft.togglePlan, runtimeKind: "nxs", textareaRef: createRef(),
  });
  return { input, setInput, draft, ...slash };
}

describe("Plan entry", () => {
  it("uses the same draft for menu activation, typed Slash and cancellation", () => {
    const { result } = renderHook(() => useEntry("session-plan"), { wrapper: ({ children }) =>
      <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider>,
    });
    act(() => result.current.setInput("improve search"));
    act(() => result.current.togglePlanInput());
    expect(result.current.input).toBe("improve search");
    expect(result.current.isPlanMode).toBe(true);
    act(() => result.current.togglePlanInput());
    expect(result.current.input).toBe("improve search");
    act(() => { result.current.setInput("/plan"); result.current.updateForInput("/plan"); });
    act(() => result.current.selectCommand(command));
    expect(result.current.input).toBe("");
    expect(result.current.isPlanMode).toBe(true);
    act(() => result.current.togglePlanInput());
    expect(result.current.input).toBe("");
  });
  it("isolates Plan by Session and restores the mode with a rejected draft", () => {
    const { result, rerender } = renderHook(({ scope }) => useEntry(scope), {
      initialProps: { scope: "session-a" },
      wrapper: ({ children }) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider>,
    });
    act(() => result.current.setInput("/plan improve search"));
    expect(result.current.input).toBe("improve search");
    expect(result.current.isPlanMode).toBe(true);
    rerender({ scope: "session-b" });
    expect(result.current.isPlanMode).toBe(false);
    expect(result.current.input).toBe("");
    rerender({ scope: "session-a" });
    expect(result.current.isPlanMode).toBe(true);
    const snapshot = result.current.draft.state;
    act(() => { result.current.draft.claimMessageSubmission(); });
    expect(result.current.isPlanMode).toBe(false);
    act(() => { result.current.draft.restoreFailedMessageSubmission(snapshot); });
    expect(result.current.isPlanMode).toBe(true);
    expect(result.current.input).toBe("improve search");
  });

});
