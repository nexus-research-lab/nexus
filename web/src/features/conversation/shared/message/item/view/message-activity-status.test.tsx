// INPUT: 活动状态、轮次计时源与模型旁白。
// OUTPUT: 证明运行行显示“动效 + 趣味动词 + 已用时长”，关键阶段与无计时源时保留真实文案。
// POS: 消息活动行 DOM 行为测试；动词词表与计时节奏由 message-activity-clock 持有。

import { act, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { MessageActivityClockProvider } from "./message-activity-clock";
import { LocalizedMessageActivityStatus } from "./message-activity-status";

const NOW = Date.UTC(2026, 10, 23, 8, 0, 0);
const LITERAL_THINKING = /^(正在思考|Thinking)$/;

function withClock(node: ReactNode, startedAt: number | null = NOW - 12_000) {
  return (
    <I18nProvider>
      <MessageActivityClockProvider roundId="round-1" startedAt={startedAt}>
        {node}
      </MessageActivityClockProvider>
    </I18nProvider>
  );
}

function labelText(container: HTMLElement): string {
  return container.querySelector(".ui-status-swap")?.textContent ?? "";
}

describe("LocalizedMessageActivityStatus", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("replaces the generic thinking label with a playful verb and a live elapsed counter", () => {
    const { container } = render(withClock(<LocalizedMessageActivityStatus state="thinking" />));

    expect(labelText(container)).not.toMatch(LITERAL_THINKING);
    expect(container.querySelector("[data-loading-orb]")).not.toBeNull();
    expect(screen.getByText("12s")).toBeTruthy();

    act(() => {
      vi.advanceTimersByTime(50_000);
    });
    expect(screen.getByText("1m 2s")).toBeTruthy();
  });

  it("keeps the verb stable when the row remounts in another position", () => {
    const first = render(withClock(<LocalizedMessageActivityStatus state="thinking" />));
    const verb = labelText(first.container);
    first.unmount();

    const second = render(withClock(<LocalizedMessageActivityStatus state="replying" />));
    expect(labelText(second.container)).toBe(verb);
  });

  it("lets model narration win over the playful verb", () => {
    const { container } = render(
      withClock(<LocalizedMessageActivityStatus label="正在比对两份配置" state="thinking" />),
    );
    expect(labelText(container)).toBe("正在比对两份配置");
    expect(screen.getByText("12s")).toBeTruthy();
  });

  it("keeps waiting states literal and untimed", () => {
    const { container } = render(withClock(<LocalizedMessageActivityStatus state="waiting_permission" />));
    expect(labelText(container)).toMatch(/^(等待确认|Waiting for approval)$/);
    expect(container.querySelector("[data-message-activity-elapsed]")).toBeNull();
  });

  it("falls back to the literal stage without a round clock", () => {
    const { container } = render(
      <I18nProvider>
        <LocalizedMessageActivityStatus state="thinking" />
      </I18nProvider>,
    );
    expect(labelText(container)).toMatch(LITERAL_THINKING);
    expect(container.querySelector("[data-message-activity-elapsed]")).toBeNull();
  });
});
