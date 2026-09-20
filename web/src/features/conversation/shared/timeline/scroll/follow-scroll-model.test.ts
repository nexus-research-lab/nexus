import { describe, expect, it } from "vitest";

import {
  resolveConversationViewportSizeRevision,
} from "./follow-scroll-model";

describe("conversation viewport size revision", () => {
  it("treats a thread side-panel width change as a viewport resize", () => {
    expect(resolveConversationViewportSizeRevision(
      { height: 600, width: 900 },
      { height: 600, width: 540 },
    )).toEqual({
      baseline: { height: 600, width: 540 },
      changed: true,
    });
  });
});
