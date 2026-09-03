// INPUT: 已分组的配对、Agent 目录和更新/删除/复制动作。
// OUTPUT: 证明配对行复用 Panel/Typography，并保持状态动作与技术详情行为。
// POS: 配对列表 DOM 合同；筛选、分组和写入恢复由 model/controller 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { PairingView } from "@/lib/api/capability/channel-api";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
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
