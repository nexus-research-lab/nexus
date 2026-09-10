// INPUT: 同名/缺名目录、成员子集与暂时缺失的当前绑定。
// OUTPUT: 证明显示序号在排序/筛选间稳定，目录不扩大资格或改变 value。
// POS: Agent 选择显示规则的跨领域回归。

import { describe, expect, it } from "vitest";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { buildAgentSelectionOptions, includeUnavailableAgentSelection } from "./agent-selection-options";

describe("Agent selection presentation", () => {
  it.each(["zh", "en"] as const)("keeps scoped labels and exact IDs stable in %s", (locale) => {
    const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
    const a = { agent_id: "agent-a", name: " Nova ", created_at: 2 };
    const b = { agent_id: "agent-b", name: "Nova", created_at: 1 };
    const directory = [a, b, { agent_id: "unrelated", name: "Pixel", created_at: 0 }];
    const options = buildAgentSelectionOptions(directory, t);
    expect(options).toEqual([{ value: a.agent_id, label: "2 · Nova" }, { value: b.agent_id, label: "1 · Nova" }, { value: "unrelated", label: "Pixel" }]);
    expect(buildAgentSelectionOptions([...directory].reverse(), t)).toEqual([...options].reverse());
    expect(buildAgentSelectionOptions([{ agent_id: a.agent_id }], t, directory)).toEqual([options[0]]);
    expect(buildAgentSelectionOptions([{ agent_id: "scoped", name: "Context name" }], t, [{ agent_id: "scoped", name: "  " }])).toEqual([{ value: "scoped", label: "Context name" }]);
    const unnamed = buildAgentSelectionOptions([{ agent_id: "z", name: "  " }, { agent_id: "a" }], t);
    expect(unnamed.map((option) => option.label)).toEqual([`2 · ${t("agent.name_fallback")}`, `1 · ${t("agent.name_fallback")}`]);
    expect(a.name).toBe(" Nova ");
  });

  it("keeps a missing current value disabled and restores it when its entry returns", () => {
    const t = (key: keyof typeof MESSAGES.zh) => MESSAGES.zh[key];
    const available = [{ value: "available", label: "Nova" }];
    expect(includeUnavailableAgentSelection(available, "missing", t)).toEqual([
      { value: "missing", label: "当前智能体不可用", disabled: true }, ...available,
    ]);
    expect(includeUnavailableAgentSelection(available, "available", t)).toBe(available);
    expect(includeUnavailableAgentSelection(available, "", t)).toBe(available);
    expect(available).toEqual([{ value: "available", label: "Nova" }]);
  });
});
