import { describe, expect, it } from "vitest";

import {
  resolveWindowEdgeLoad,
  resolveWindowEvictionEdge,
  type WindowEdgeContext,
} from "./continuous-window-model";

const reading: WindowEdgeContext = {
  clientHeight: 600,
  direction: "up",
  firstVisibleIndex: 10,
  gapIndex: null,
  lastVisibleIndex: 12,
  newerUnavailable: false,
  olderUnavailable: false,
  scrollHeight: 20_000,
  scrollTop: 8_000,
};

describe("continuous window edge loading", () => {
  it("prefetches the older page within one viewport of the window start", () => {
    expect(resolveWindowEdgeLoad(reading)).toBeNull();
    expect(resolveWindowEdgeLoad({ ...reading, scrollTop: 500 })).toBe("older");
    expect(resolveWindowEdgeLoad({ ...reading, olderUnavailable: true, scrollTop: 500 })).toBeNull();
  });

  it("extends forward near the gap and reattaches to latest once the reader passed it", () => {
    const detached = { ...reading, direction: "down" as const, gapIndex: 20 };
    expect(resolveWindowEdgeLoad(detached)).toBeNull();
    expect(resolveWindowEdgeLoad({ ...detached, lastVisibleIndex: 18 })).toBe("newer");
    expect(resolveWindowEdgeLoad({ ...detached, firstVisibleIndex: 21, lastVisibleIndex: 22 }))
      .toBe("latest");
    expect(resolveWindowEdgeLoad({ ...detached, lastVisibleIndex: 18, newerUnavailable: true }))
      .toBeNull();
  });

  it("follows the reading direction when both edges are within reach", () => {
    const short = { ...reading, gapIndex: 12, scrollTop: 100 };
    expect(resolveWindowEdgeLoad({ ...short, direction: "up" })).toBe("older");
    expect(resolveWindowEdgeLoad({ ...short, direction: "down" })).toBe("newer");
  });

  it("evicts the edge farther from the reading viewport and never the latest end while following", () => {
    const top = { clientHeight: 600, followingLatest: false, scrollHeight: 20_000, scrollTop: 0 };
    expect(resolveWindowEvictionEdge(top)).toBe("tail");
    expect(resolveWindowEvictionEdge({ ...top, scrollTop: 19_400 })).toBe("head");
    expect(resolveWindowEvictionEdge({ ...top, followingLatest: true })).toBe("head");
  });
});
