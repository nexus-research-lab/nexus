// INPUT: 不完整参与者目录、精确事件来源/收件人和当前语言。
// OUTPUT: 证明身份不变、名称安全降级与自己的语义不依赖目录完整性。
// POS: 私域事件展示投影回归；不改变原文、路由或文件来源。

import { describe, expect, it } from "vitest";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { AgentPrivateEvent, AgentPrivateThread } from "@/types/agent/private-domain";
import { buildPrivateTimelineBody } from "./agent-private-domain-timeline-model";
import type { PrivateDomainLocalization } from "../agent-private-domain-thread-model";

const thread: AgentPrivateThread = { thread_id: "thread", agent_id: "owner", participants: [], participant_agent_ids: ["owner", "peer-id"], peer_agent_ids: ["peer-id"], scope: "direct", message_count: 1 };
const event: AgentPrivateEvent = { message_id: "event", thread_id: "thread", direction: "incoming", source_agent_id: "peer-id", recipients: ["owner", "missing-id"], participants: [], reply_route: { mode: "private" }, timestamp: 1000, content: "原文 peer-id 不得全局替换" };
function project(locale: Locale, overrides: Partial<AgentPrivateEvent> = {}) {
  const localization: PrivateDomainLocalization = { locale, t: (key, params) => Object.entries(params ?? {}).reduce((value, [name, replacement]) => value.replaceAll(`{${name}}`, String(replacement)), MESSAGES[locale][key]) };
  return buildPrivateTimelineBody({ agentId: "owner", events: [{ ...event, ...overrides }], isLoading: false, localization, thread }).events[0];
}

describe("private participant presentation", () => {
  it.each(["zh", "en"] as const)("localizes missing source and recipient names while preserving identity in %s", (locale) => {
    const result = project(locale);
    expect(result.sourceName).toBe(MESSAGES[locale]["agent.name_fallback"]);
    expect(result.routeLabel).toContain(MESSAGES[locale]["agent.name_fallback"]);
    expect(result.routeLabel).not.toContain("missing-id");
    expect(result.sourceAgentId).toBe("peer-id");
    expect(result.content).toBe(event.content);
    expect(result.id).toBe("event");
  });
  it("identifies self from the event even when participant metadata is absent", () => {
    const result = project("zh", { source_agent_id: "owner", direction: "self" });
    expect(result.sourceName).toBe(MESSAGES.zh["agent_options.contact.self"]);
    expect(result.sourceAgentId).toBe("owner");
  });
  it("uses current real names but leaves missing or blank recipients generic", () => {
    const result = project("en", { participants: [{ agent_id: "peer-id", name: " Nova " }, { agent_id: "missing-id", name: "  " }] });
    expect(result.sourceName).toBe("Nova");
    expect(result.routeLabel).not.toContain("missing-id");
    expect(project("en", { participants: [{ agent_id: "peer-id", name: "Pixel" }] }).sourceName).toBe("Pixel");
  });
});
