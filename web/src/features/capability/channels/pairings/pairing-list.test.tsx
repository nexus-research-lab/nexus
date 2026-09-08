// INPUT: 已分组的配对、Agent 目录和更新/删除/复制动作。
// OUTPUT: 证明配对行复用 Panel/Typography，并保持状态动作与技术详情行为。
// POS: 配对列表 DOM 合同；筛选、分组和写入恢复由 model/controller 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { PairingView } from "@/lib/api/capability/channel-api";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { Agent } from "@/types/agent/agent";

import { PairingList } from "./pairing-list";

const AGENT = {
  agent_id: "agent-1",
  created_at: 1,
  name: "Nexus",
  options: {},
  status: "idle",
  workspace_path: "/workspace/nexus",
} satisfies Agent;

const PAIRING = {
  agent_id: AGENT.agent_id,
  agent_name: AGENT.name,
  channel_type: "telegram",
  chat_type: "group",
  created_at: "2026-09-01T10:00:00Z",
  external_name: "Design Team",
  external_ref: "chat-42",
  pairing_id: "pairing-1",
  session_key: "telegram:group:chat-42",
  source: "ingress",
  status: "active",
  updated_at: "2026-09-03T10:00:00Z",
} satisfies PairingView;

describe("PairingList", () => {
  it("distinguishes same-name targets while updates keep the exact selected Agent ID", async () => {
    const user = userEvent.setup();
    const other = { ...AGENT, agent_id: "agent-2", created_at: 2 };
    const item = { ...PAIRING, agent_id: other.agent_id };
    const onUpdatePairing = vi.fn();
    const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES.zh[key]);
    const view = (agents: Agent[]) => <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t }}>
      <PairingList agents={agents} busy={false} groups={[{ agent_id: other.agent_id, agent_name: other.name, items: [item] }]} pendingItems={[]} onCopySessionKey={vi.fn()} onDeletePairing={vi.fn()} onUpdatePairing={onUpdatePairing} />
    </I18N_CONTEXT.Provider>;
    const { rerender } = render(view([AGENT, other]));
    expect(screen.getByRole("button", { name: "选择配对处理智能体" }).textContent).toContain("2 · Nexus");
    rerender(view([AGENT]));
    expect(screen.getByRole("button", { name: "选择配对处理智能体" }).textContent).toContain("当前智能体不可用");
    expect(onUpdatePairing).not.toHaveBeenCalled();
    rerender(view([other, AGENT]));
    await user.click(screen.getByRole("button", { name: "选择配对处理智能体" }));
    await user.click(screen.getByRole("option", { name: "1 · Nexus" }));
    expect(onUpdatePairing).toHaveBeenCalledWith(item, { agent_id: AGENT.agent_id });
  });

  it("renders semantic row text and dispatches status and delete actions", async () => {
    const user = userEvent.setup();
    const onDeletePairing = vi.fn();
    const onUpdatePairing = vi.fn();
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <PairingList
          agents={[AGENT]}
          busy={false}
          groups={[{
            agent_id: AGENT.agent_id,
            agent_name: AGENT.name,
            items: [PAIRING],
          }]}
          onCopySessionKey={vi.fn()}
          onDeletePairing={onDeletePairing}
          onUpdatePairing={onUpdatePairing}
          pendingItems={[]}
        />
      </I18N_CONTEXT.Provider>,
    );

    expect(screen.getByRole("heading", { name: AGENT.name }).className)
      .toContain("ui-type-section-title");
    expect(screen.getByText(PAIRING.external_name).className)
      .toContain("ui-type-control");
    expect(screen.getByText(PAIRING.external_ref).className)
      .toContain("ui-type-code");
    expect(container.querySelector("section.surface-radius-sm")).toBeTruthy();

    await user.click(screen.getByText("处理智能体"));
    expect(screen.getByRole("listbox", { name: "选择配对处理智能体" })).toBeTruthy();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();

    await user.click(screen.getByRole("button", { name: "停用" }));
    expect(onUpdatePairing).toHaveBeenCalledWith(PAIRING, {
      status: "disabled",
    });

    await user.click(screen.getByRole("button", { name: "删除" }));
    expect(onDeletePairing).toHaveBeenCalledWith(PAIRING);
  });

  it("keeps technical identities behind the expandable details row", async () => {
    const user = userEvent.setup();
    const onCopySessionKey = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <PairingList
          agents={[AGENT]}
          busy={false}
          groups={[{
            agent_id: AGENT.agent_id,
            agent_name: AGENT.name,
            items: [PAIRING],
          }]}
          onCopySessionKey={onCopySessionKey}
          onDeletePairing={vi.fn()}
          onUpdatePairing={vi.fn()}
          pendingItems={[]}
        />
      </I18N_CONTEXT.Provider>,
    );

    await user.click(screen.getByText("技术详情"));
    expect(screen.getByText(PAIRING.session_key).className)
      .toContain("ui-type-code");

    await user.click(screen.getByRole("button", {
      name: "复制 IM session key",
    }));
    expect(onCopySessionKey).toHaveBeenCalledWith(PAIRING);
  });
});
