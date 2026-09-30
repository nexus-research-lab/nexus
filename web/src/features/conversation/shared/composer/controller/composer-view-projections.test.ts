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
