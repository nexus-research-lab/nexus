// INPUT: 通用 resource_changed 失效事件与其他事件。
// OUTPUT: 证明资源名与 Agent 作用域都命中才触发刷新。
// POS: resource-change 纯匹配回归。

import { describe, expect, it } from "vitest";

import type { EventMessage } from "@/types/generated/protocol";

import { isResourceChange } from "./resource-change";

function event(eventType: string, data: Record<string, unknown>): EventMessage {
  return { event_type: eventType, data } as unknown as EventMessage;
}

describe("isResourceChange", () => {
  it("matches the named resource only for resource_changed events", () => {
    expect(isResourceChange(event("resource_changed", { resource: "channel_control" }), "channel_control")).toBe(true);
    expect(isResourceChange(event("resource_changed", { resource: "agent_private_domain" }), "channel_control")).toBe(false);
    expect(isResourceChange(event("directory_changed", { resource: "channel_control" }), "channel_control")).toBe(false);
  });

  it("requires the Agent to be listed when an Agent scope is given", () => {
    const changed = event("resource_changed", { resource: "agent_private_domain", agent_ids: ["alpha", "beta"] });
    expect(isResourceChange(changed, "agent_private_domain", "beta")).toBe(true);
    expect(isResourceChange(changed, "agent_private_domain", "gamma")).toBe(false);
    expect(isResourceChange(event("resource_changed", { resource: "agent_private_domain" }), "agent_private_domain", "alpha")).toBe(false);
  });
});
