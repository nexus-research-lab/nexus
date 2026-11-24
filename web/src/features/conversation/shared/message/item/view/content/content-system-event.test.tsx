// INPUT: Current and expired Provider retry events.
// OUTPUT: Countdown timers stop at the deadline and never run for historical retries; compaction rows report saved tokens.
// POS: System event timer lifecycle regression.
import { act, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ContentSystemEvent } from "./content-system-event";
it("stops the countdown timer and skips expired history", () => {
  vi.useFakeTimers();
  vi.setSystemTime(10000);
  const interval = vi.spyOn(window, "setInterval");
  try {
    const view = (timestamp: number) => <I18nProvider><ContentSystemEvent block={{ type: "system_event", source_message_id: "retry-message", subtype: "api_retry", icon: "retry", tone: "neutral", label: "Retry", content: "Transient error", timestamp, retry_delay_ms: 1000 }} /></I18nProvider>;
    const { rerender, unmount } = render(view(10000));
    expect(interval).toHaveBeenCalledTimes(1);
    act(() => vi.advanceTimersByTime(1000));
    expect(vi.getTimerCount()).toBe(0);
    rerender(view(1000));
    expect(interval).toHaveBeenCalledTimes(1);
    unmount();
  } finally { interval.mockRestore(); vi.useRealTimers(); }
});

it("shows how many tokens a compaction saved instead of the SDK text", () => {
  const block = { type: "system_event" as const, source_message_id: "compact", subtype: "compact_boundary", icon: "status" as const, tone: "neutral" as const, label: "x", content: "Conversation compacted", timestamp: 1 };
  const { rerender } = render(<I18nProvider><ContentSystemEvent block={{ ...block, compact_pre_tokens: 120_000, compact_post_tokens: 15_600 }} /></I18nProvider>);
  expect(screen.getByText(/104\.4K/)).toBeTruthy();
  expect(screen.queryByText("Conversation compacted")).toBeNull();
  rerender(<I18nProvider><ContentSystemEvent block={{ ...block, compact_pre_tokens: 120_000 }} /></I18nProvider>);
  expect(screen.getByText(/120K/)).toBeTruthy();
});
