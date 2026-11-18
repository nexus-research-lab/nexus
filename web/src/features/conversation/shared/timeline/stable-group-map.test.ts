// INPUT: 流式帧前后两次分组投影。
// OUTPUT: 只有逐元素同一引用的分组复用原数组。
// POS: memo 视图跳过未变化轮次的结构共享回归。
import { expect, it } from "vitest";

import { stabilizeGroupMap } from "./stable-group-map";

it("reuses unchanged groups and keeps changed or new ones", () => {
  const finished = { id: "finished" };
  const streamingBefore = { id: "streaming", text: "a" };
  const streamingAfter = { id: "streaming", text: "ab" };
  const previous = new Map([["done", [finished]], ["live", [streamingBefore]], ["empty", [] as object[]]]);
  const next = stabilizeGroupMap(previous, new Map([
    ["done", [finished]],
    ["live", [streamingAfter]],
    ["empty", []],
    ["new", [finished]],
  ]));
  expect(next.get("done")).toBe(previous.get("done"));
  expect(next.get("empty")).toBe(previous.get("empty"));
  expect(next.get("live")).not.toBe(previous.get("live"));
  expect(next.get("live")?.[0]).toBe(streamingAfter);
  expect(next.get("new")).toEqual([finished]);
  expect(stabilizeGroupMap(new Map([["done", [finished, finished]]]), new Map([["done", [finished]]])).get("done")).toHaveLength(1);
});
