// INPUT: 同一 Agent 的两个执行轮、后到/空白姓名与当前语言。
// OUTPUT: 证明姓名仅影响显示，Thread/停止仍作用于精确执行轮且外壳保持稳定。
// POS: 真实 Room 卡片装配 DOM 回归，不校验像素或运行时业务。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import { ThreadControlContext, type ThreadTarget } from "../group-thread-state";
import { GroupRoundCardGroup } from "./group-round-card-group";

const slots: RoomPendingAgentSlotState[] = [1, 2].map((index) => ({
  agent_id: "internal-agent", agent_round_id: `execution-${index}`,
  msg_id: `slot-${index}`, round_id: "root", timestamp: index,
  status: "pending", index,
}));

describe("Room card display identity", () => {
  it.each(["zh", "en"] as const)("keeps exact actions and shell identity while labels change in %s", (locale) => {
    const onStop = vi.fn();
    const openThread = vi.fn();
    const closeThread = vi.fn();
    const view = (language: Locale, names: Record<string, string>, activeThread: ThreadTarget | null = null) => (
      <I18N_CONTEXT.Provider value={{ locale: language, setLocale: vi.fn(), t: (key) => MESSAGES[language][key] }}>
        <ThreadControlContext.Provider value={{ activeThread, closeThread, openThread }}>
          <GroupRoundCardGroup
            agentAvatarMap={{}} agentNameMap={names} messages={[]}
            pendingPermissions={[]} pendingSlots={slots} roomAgentExecutionStates={[]}
            onPermissionResponse={() => true} onStopAgentRound={onStop}
            roundId="root" stoppingAgentRoundIds={[]}
          />
        </ThreadControlContext.Provider>
      </I18N_CONTEXT.Provider>
    );
    const { container, rerender } = render(view(locale, {}));
    expect(container.textContent).toContain(MESSAGES[locale]["agent.name_fallback"]);
    expect(container.textContent).not.toContain("internal-agent");
    const shells = Array.from(container.querySelectorAll("[data-room-agent-execution-shell]"));
    expect(shells).toHaveLength(2);
    fireEvent.click(screen.getAllByRole("button", { name: MESSAGES[locale]["room.thread_open"] })[0]);
    expect(openThread).toHaveBeenCalledWith("root", "internal-agent", "execution-1");
    fireEvent.click(screen.getAllByRole("button", { name: MESSAGES[locale]["room.agent_stop_action"] })[1]);
    expect(onStop).toHaveBeenCalledWith("execution-2");

    rerender(view(locale, { "internal-agent": " Nova " }, { roundId: "root", agentId: "internal-agent", agentRoundId: "execution-1" }));
    expect(container.textContent).toContain("Nova");
    const currentShells = container.querySelectorAll("[data-room-agent-execution-shell]");
    shells.forEach((shell, index) => expect(currentShells[index]).toBe(shell));
    fireEvent.click(screen.getByRole("button", { name: MESSAGES[locale]["room.thread_close"] }));
    expect(closeThread).toHaveBeenCalledOnce();

    const nextLocale = locale === "zh" ? "en" : "zh";
    rerender(view(nextLocale, { "internal-agent": "  " }));
    expect(container.textContent).toContain(MESSAGES[nextLocale]["agent.name_fallback"]);
    expect(container.textContent).not.toContain("Nova");
    expect(openThread).toHaveBeenCalledOnce();
    expect(onStop).toHaveBeenCalledOnce();
  });
});
