import { describe, expect, it } from "vitest";

import { projectComposerRuntime } from "./composer-view-projections";

describe("projectComposerRuntime", () => {
  it("keeps a phase-active session busy while loading is temporarily false", () => {
    const projection = projectComposerRuntime({
      isLoading: false,
      queueItemCount: 0,
      runtimePhase: "streaming",
    });

    expect(projection.sessionBusy).toBe(true);
    expect(projection.activity).toBe("replying");
    expect(projection.canStopGeneration).toBe(true);
  });

  // isLoading is derived from phase !== "idle", so a live runtime always pairs
  // isLoading=true with its phase. Stop must stay available for that pairing.
  it.each(["streaming", "awaiting_permission", "compacting"] as const)(
    "offers stop while the runtime is %s with loading set", (runtimePhase) => {
      const projection = projectComposerRuntime({
        isLoading: true,
        queueItemCount: 0,
        runtimePhase,
      });

      expect(projection.canStopGeneration).toBe(true);
    },
  );

  it("withholds stop only while the request is still being sent", () => {
    const projection = projectComposerRuntime({
      isLoading: true,
      queueItemCount: 0,
      runtimePhase: "sending",
    });

    expect(projection.canStopGeneration).toBe(false);
    expect(projection.sessionBusy).toBe(true);
  });

  it("treats an awaiting permission phase as busy for queue routing", () => {
    const projection = projectComposerRuntime({
      isLoading: false,
      queueItemCount: 0,
      runtimePhase: "awaiting_permission",
    });

    expect(projection.sessionBusy).toBe(true);
    expect(projection.isAwaitingPermission).toBe(true);
  });
});
