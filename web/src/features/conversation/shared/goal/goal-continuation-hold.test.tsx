// INPUT: Current DM/Room permissions, member identity and locale changes.
// OUTPUT: Evidence that continuation explanations localize without changing target or permission rules.
// POS: Pure hold policy and DM hook tests; no Goal mutation or runtime is started.
import type { ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { useDmGoalController } from "@/features/conversation/room/dm/panel/controller/use-dm-goal-controller";
import { ROOM_GOAL_MEMBERS } from "@/features/conversation/room/group/chat/room-goal.test-support";
import { goalContinuationHoldForPermission, goalContinuationHoldForRoomTarget } from "./goal-continuation-hold";
import { goalTestI18n } from "./goal.test-support";

describe("Goal continuation holds", () => {
  it.each(["en", "zh"] as const)("preserves target and host takeover rules in %s", (locale) => {
    const { t } = goalTestI18n(locale);
    expect(goalContinuationHoldForPermission(null, "default", t)).toBeNull();
    expect(goalContinuationHoldForPermission(" ", " plan ", t)?.detail)
      .toBe(t("goal.hold_plan_detail", { name: t("agent.name_fallback") }));
    expect(goalContinuationHoldForRoomTarget([], "", false, t)).toBeNull();
    expect(goalContinuationHoldForRoomTarget([ROOM_GOAL_MEMBERS[1]], "", true, t)?.label).toBe(t("goal.hold_plan_label"));
    expect(goalContinuationHoldForRoomTarget(ROOM_GOAL_MEMBERS, "alpha", false, t)).toBeNull();
    expect(goalContinuationHoldForRoomTarget(ROOM_GOAL_MEMBERS, "beta", false, t)?.detail)
      .toBe(t("goal.hold_plan_detail", { name: "Beta" }));
    expect(goalContinuationHoldForRoomTarget(ROOM_GOAL_MEMBERS, "missing", false, t)?.label).toBe(t("goal.hold_owner_label"));
    expect(goalContinuationHoldForRoomTarget(ROOM_GOAL_MEMBERS, "", true, t)?.label).toBe(t("goal.hold_target_label"));
  });
  it("updates DM hold language in place while keeping the refresh sequence", () => {
    let locale: "en" | "zh" = "en";
    function Wrapper({ children }: { children: ReactNode }) {
      return <I18N_CONTEXT.Provider value={goalTestI18n(locale)}>{children}</I18N_CONTEXT.Provider>;
    }
    const { result, rerender } = renderHook(() => useDmGoalController({ agentName: "Alpha", permissionMode: "plan" }), { wrapper: Wrapper });
    expect(result.current.continuationHold?.label).toBe("Paused by Plan mode");
    act(() => result.current.refresh());
    locale = "zh"; rerender();
    expect(result.current.continuationHold?.label).toBe("Plan 模式暂停");
    expect(result.current.refreshSequence).toBe(1);
  });
});
