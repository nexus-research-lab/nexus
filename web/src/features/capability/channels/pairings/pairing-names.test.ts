// INPUT: 当前目录、旧配对姓名、缺名对象与精确筛选身份。
// OUTPUT: 证明分组随当前姓名/语言更新，原始配对和筛选身份不变。
// POS: 配对分组名称回归，不定义资源选择或外部对象 ID 的展示策略。

import { describe, expect, it } from "vitest";
import type { PairingView } from "@/lib/api/capability/channel-api";
import { MESSAGES } from "@/shared/i18n/messages";
import type { Agent } from "@/types/agent/agent";
import { filterPairings, groupPairings } from "./pairing-model";

const agent: Agent = { agent_id: "internal-agent", name: " Nova ", created_at: 1, options: {}, status: "idle", workspace_path: "/workspace/nova" };
const pairing: PairingView = {
  agent_id: agent.agent_id, agent_name: "Old name", channel_type: "telegram", chat_type: "group",
  pairing_id: "pairing-1", external_ref: "chat-42", source: "ingress", status: "active",
  session_key: "telegram:group:chat-42",
  created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
};

describe("pairing group names", () => {
  it.each(["zh", "en"] as const)("uses current names, historical names and safe fallback in %s", (locale) => {
    const localization = { locale, t: (key: keyof typeof MESSAGES.zh) => MESSAGES[locale][key] };
    const second = { ...pairing, pairing_id: "pairing-2", external_ref: "chat-43" };
    const result = groupPairings([pairing, second], [agent], localization);
    expect(result).toHaveLength(1);
    expect(result[0].agent_name).toBe("Nova");
    expect(result[0].agent_id).toBe(agent.agent_id);
    expect(result[0].items[0]).toBe(pairing);
    expect(result[0].items[1]).toBe(second);
    expect(groupPairings([pairing], [], localization)[0].agent_name).toBe("Old name");
    const unnamed = { ...pairing, agent_name: "  " };
    expect(groupPairings([unnamed], [], localization)[0].agent_name).toBe(localization.t("agent.name_fallback"));
    expect(filterPairings([pairing, { ...second, agent_id: "another-agent" }], { agentId: agent.agent_id, channel: "", query: "", status: "" })).toEqual([pairing]);
    expect(pairing.agent_name).toBe("Old name");
  });
});
