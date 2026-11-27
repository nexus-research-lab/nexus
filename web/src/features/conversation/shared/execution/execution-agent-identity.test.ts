// INPUT: Agent/Subagent 节点身份、可缺失的名称与当前语言。
// OUTPUT: 证明通称不污染身份、头像种子或无负责人的语义。
// POS: Execution 展示身份回归；不改变图拓扑或运行身份。

import { describe, expect, it } from "vitest";
import { MESSAGES } from "@/shared/i18n/messages";
import type { ExecutionGraphNodeView } from "@/types/conversation/execution";
import { resolveExecutionAgent, resolveExecutionGraphNodeAgent } from "./execution-process-model";

const node: ExecutionGraphNodeView = { id: "node", kind: "subagent", visibility: "nested", work_item_id: "work", position: 0, agent_id: "runtime-agent", subject_id: "child-identity" };

describe("Execution display identities", () => {
  it.each(["zh", "en"] as const)("uses current role labels while keeping exact IDs and unassigned state in %s", (locale) => {
    const t = (key: keyof typeof MESSAGES.zh) => MESSAGES[locale][key];
    expect(resolveExecutionAgent({}, undefined, t)).toBeNull();
    expect(resolveExecutionAgent({}, "owner-id", t)).toEqual({ id: "owner-id", avatar: null, name: MESSAGES[locale]["agent.name_fallback"], nameIsFallback: true });
    const child = resolveExecutionGraphNodeAgent({}, node, null, t)!;
    expect(child.id).toBe("subagent:child-identity");
    expect(child.name).toBe(MESSAGES[locale]["agent.subagent_name_fallback"]);
    expect(child.name).not.toContain("runtime-agent");
    const renamed = resolveExecutionGraphNodeAgent({}, { ...node, name: " Research " }, null, t)!;
    expect(renamed.name).toBe("Research");
    expect(renamed.avatar).toBe(child.avatar);
    expect(renamed.id).toBe(child.id);
  });
});
