// INPUT: 有效、越界和 Room 分组上下文占用快照。
// OUTPUT: 验证安全百分比、语义风险 tone 与最高占用摘要。
// POS: Composer 上下文用量纯模型回归测试。

import { describe, expect, it } from "vitest";

import {
  projectComposerContextUsage,
  projectContextUsage,
} from "./composer-context-usage-model";

describe("composer context usage model", () => {
  it("clamps protocol percentages and maps semantic risk tones", () => {
    expect(projectContextUsage({ max_tokens: 100, percentage: 79.5, total_tokens: 80 }))
      .toMatchObject({ percentage: 80, tone: "warning" });
    expect(projectContextUsage({ max_tokens: 100, percentage: 97, total_tokens: 97 }))
      .toMatchObject({ percentage: 97, tone: "danger" });
    expect(projectContextUsage({ max_tokens: 100, percentage: 120, total_tokens: 120 }))
      .toMatchObject({ percentage: 100, tone: "danger" });
  });

  it("uses the highest valid Room snapshot for the summary", () => {
    const projection = projectComposerContextUsage({
      items: [
        {
          agentId: "agent-1",
          name: "Researcher",
          usage: { max_tokens: 100, percentage: 42, total_tokens: 42 },
        },
        {
          agentId: "agent-2",
          name: "Writer",
          usage: { max_tokens: 100, percentage: 86, total_tokens: 86 },
        },
      ],
      usage: null,
    });

    expect(projection?.grouped).toBe(true);
    expect(projection?.summary).toMatchObject({ percentage: 86, tone: "warning" });
  });
});
