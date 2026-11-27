// INPUT: Current scoped Goal snapshots, member names/permissions and language changes.
// OUTPUT: Evidence that Room presentation reads the canonical Goal without a delayed local mirror.
// POS: Real Room/Goal panel DOM composition with offline resource snapshots.
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { ACTIVE_GOAL, goalTestI18n } from "@/features/conversation/shared/goal/goal.test-support";
import { useGoalResource } from "@/features/conversation/shared/goal/use-goal-resource";
import { RoomGoalPanel } from "./room-goal-panel";
import { ROOM_GOAL_MEMBERS } from "./room-goal.test-support";

vi.mock("@/features/conversation/shared/goal/use-goal-resource", () => ({ useGoalResource: vi.fn() }));
let resource: ReturnType<typeof useGoalResource>;
const onGoalChange = vi.fn();
function view(locale: "en" | "zh" = "en", members = ROOM_GOAL_MEMBERS, sessionKey = "session-1") {
  return <I18N_CONTEXT.Provider value={goalTestI18n(locale)}>
    <RoomGoalPanel activityKey={1} isLoading={false} isMobileLayout={false} onGoalChange={onGoalChange}
      roomHostAgentId="alpha" roomHostAutoReplyEnabled roomMembers={members} sessionKey={sessionKey} />
  </I18N_CONTEXT.Provider>;
}
beforeEach(() => {
  vi.clearAllMocks();
  resource = {
    goal: { ...ACTIVE_GOAL, metadata: { room_goal_lead_agent_id: "beta" } },
    executionBinding: { state: "standalone" }, isLoading: true, mutationBlockReason: null,
    mutationsBlocked: false, ownerScopeGeneration: 1, phase: null, refresh: vi.fn(async () => {}),
    reliability: null, runCommand: vi.fn(async () => ({ ok: false, goal: null })),
  };
  vi.mocked(useGoalResource).mockImplementation(() => resource);
});

describe("RoomGoalPanel", () => {
  it("uses the loaded Goal owner immediately while a refresh has withheld onGoalChange", () => {
    render(view());
    expect(onGoalChange).not.toHaveBeenCalled();
    expect(screen.getByText("Owner Beta")).toBeTruthy();
    expect(screen.queryByText("Owner Alpha")).toBeNull();
    expect(screen.getByText("Paused by Plan mode")).toBeTruthy();
  });
  it("switches current Goal, permission and owner context without retaining another session's presentation", () => {
    const { rerender } = render(view());
    resource = { ...resource, goal: { ...ACTIVE_GOAL, session_key: "session-2", metadata: { room_goal_lead_agent_id: "alpha" } } };
    rerender(view("en", ROOM_GOAL_MEMBERS, "session-2"));
    expect(screen.getByText("Owner Alpha")).toBeTruthy();
    expect(screen.queryByText("Owner Beta")).toBeNull();
    expect(screen.queryByText("Paused by Plan mode")).toBeNull();
    resource = { ...resource, ownerScopeGeneration: 2, goal: null };
    rerender(view("en", ROOM_GOAL_MEMBERS, "session-2"));
    expect(screen.queryByText("Owner Alpha")).toBeNull();
  });
  it("keeps the locked-state refresh on the shared controller instead of maintaining a second Goal request", () => {
    resource.isLoading = false; resource.mutationsBlocked = true; resource.mutationBlockReason = "stale_read"; render(view());
    vi.mocked(resource.refresh).mockClear();
    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
    expect(resource.refresh).toHaveBeenCalledOnce();
    expect(onGoalChange).toHaveBeenCalledWith(resource.goal);
  });
});
