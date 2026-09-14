// INPUT: Current and expired Provider retry events.
// OUTPUT: Countdown timers stop at the deadline and never run for historical retries.
// POS: System event timer lifecycle regression.
import { act, render } from "@testing-library/react";
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
