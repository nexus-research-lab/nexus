// INPUT: context_usage WebSocket 载荷（含/不含分类明细、含坏项）。
// OUTPUT: 验证解码边界保留合法分类、丢弃坏项，且不因分类缺失拒绝快照。
// POS: 上下文分类明细从后端到 Composer 弹层的协议解码回归测试。

import { describe, expect, it } from "vitest";

import { parseContextUsageData } from "./session-event-data";

describe("parseContextUsageData", () => {
  it("keeps valid categories and the deferred flag", () => {
    expect(parseContextUsageData({
      categories: [
        { name: "System prompt", tokens: 3200 },
        { deferred: true, name: "MCP tools", tokens: 800 },
        { name: "", tokens: 5 },
        { name: "Broken", tokens: -1 },
        "junk",
      ],
      max_tokens: 1_000_000,
      percentage: 2,
      total_tokens: 17_900,
    })).toEqual({
      categories: [
        { name: "System prompt", tokens: 3200 },
        { deferred: true, name: "MCP tools", tokens: 800 },
      ],
      max_tokens: 1_000_000,
      percentage: 2,
      total_tokens: 17_900,
    });
  });

  it("omits categories when the runtime sends none", () => {
    expect(parseContextUsageData({ max_tokens: 10, percentage: 10, total_tokens: 1 }))
      .toEqual({ max_tokens: 10, percentage: 10, total_tokens: 1 });
  });
});
