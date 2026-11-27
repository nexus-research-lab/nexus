// INPUT: Unicode names/text with and without Intl.Segmenter support.
// OUTPUT: Full graphemes normally and valid code points on older runtimes.
// POS: Shared character boundary regression, independent of animation/stream timing.

import { afterEach, expect, it, vi } from "vitest";

afterEach(() => { vi.unstubAllGlobals(); vi.resetModules(); });

it("falls back to complete code points when segmentation is unavailable", async () => {
  vi.stubGlobal("Intl", {});
  const { splitTextGraphemes } = await import("./text-graphemes");
  expect(splitTextGraphemes("𠮷👍")).toEqual(["𠮷", "👍"]);
});
