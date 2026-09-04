// INPUT: 最近 DM/Room 条目、共享 i18n 与导航回调。
// OUTPUT: 验证最近入口复用 Button 胶囊合同、稳定类型标识和原有导航行为。
// POS: Launcher Hero 最近入口组件回归测试。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";

import { LauncherRecentEntries } from "./launcher-recent-entries";

describe("LauncherRecentEntries", () => {
  beforeEach(() => {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, "zh");
  });

  it("uses shared pill actions without model-owned inline colors", async () => {
    const user = userEvent.setup();
    const onHandoff = vi.fn();
    const onOpen = vi.fn();
    const dmEntry = {
      agent_id: "agent-1",
      key: "dm-1",
      label: "Researcher",
      last_activity_at: 2,
      type: "dm" as const,
    };
    const roomEntry = {
      conversation_id: "conversation-1",
      key: "room-1",
      label: "Design review",
      last_activity_at: 1,
      room_id: "room-1",
      type: "room" as const,
    };

    render(
      <I18nProvider>
        <LauncherRecentEntries
          handoffLabel="交给 Nexus"
          initialPrompt="整理今天的工作"
          onHandoff={onHandoff}
          onOpen={onOpen}
          recentEntries={[dmEntry, roomEntry]}
        />
      </I18nProvider>,
    );

    const dmButton = screen.getByRole("button", { name: "私聊 Researcher" });
    const roomButton = screen.getByRole("button", { name: "房间 Design review" });
    for (const button of [dmButton, roomButton]) {
      expect(button.className).toContain("min-h-8");
      expect(button.className).toContain("rounded-full");
      expect(button.getAttribute("style")).toBeNull();
    }
    expect(dmButton.querySelector("svg")).toBeTruthy();
    expect(roomButton.textContent).toBe("#Desig…view");

    await user.click(dmButton);
    expect(onOpen).toHaveBeenCalledWith(dmEntry);
    await user.click(screen.getByRole("button", { name: "交给 Nexus" }));
    expect(onHandoff).toHaveBeenCalledWith("整理今天的工作");
  });
});
