import { describe, expect, it } from "vitest";

import {
  hasOlderHistory,
  planNewerHistoryRequest,
  planOlderHistoryRequest,
} from "./conversation-history-model";

const base = {
  activeSessionKey: "agent:amy:ws:dm:test",
  hasMore: true,
  identity: null,
  isLoading: false,
};

describe("continuous window history requests", () => {
  it("continues from the server cursor while it still points at the resident head", () => {
    const request = planOlderHistoryRequest({
      ...base,
      cursor: { before_round_id: "round-10", before_round_timestamp: 1_000 },
      residentRoundIds: ["round-10", "round-11"],
    });
    expect(request?.query).toMatchObject({
      before_round_id: "round-10",
      before_round_timestamp: 1_000,
    });
  });

  it("reads the page adjacent to the resident head after eviction or a jump moved it", () => {
    const request = planOlderHistoryRequest({
      ...base,
      cursor: { before_round_id: "round-2", before_round_timestamp: 200 },
      residentRoundIds: ["round-40", "round-41"],
    });
    expect(request?.query.before_round_id).toBe("round-40");
    expect(request?.query.before_round_timestamp).toBeUndefined();
  });

  it("stops at the confirmed session start and reopens once that round is evicted", () => {
    const cursor = {
      before_round_id: null,
      before_round_timestamp: null,
      start_round_id: "round-1",
    };
    expect(planOlderHistoryRequest({
      ...base,
      cursor,
      hasMore: false,
      residentRoundIds: ["round-1", "round-2"],
    })).toBeNull();
    expect(hasOlderHistory("round-1", false, ["round-1", "round-2"])).toBe(false);

    expect(planOlderHistoryRequest({
      ...base,
      cursor,
      hasMore: false,
      residentRoundIds: ["round-5", "round-6"],
    })?.query.before_round_id).toBe("round-5");
    expect(hasOlderHistory("round-1", false, ["round-5", "round-6"])).toBe(true);
  });

  it("extends forward only while the window is detached from latest", () => {
    const context = { activeSessionKey: base.activeSessionKey, identity: null, isLoading: false };
    expect(planNewerHistoryRequest({ ...context, gapRoundId: null })).toBeNull();
    expect(planNewerHistoryRequest({ ...context, gapRoundId: "round-9" })?.query)
      .toMatchObject({ around_round_id: "round-9" });
  });
});
