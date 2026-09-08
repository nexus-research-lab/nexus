// INPUT: Whether a mounted view currently displays relative timestamps.
// OUTPUT: A current clock sampled on visible minute boundaries and immediately on foreground return.
// POS: Presentation-only React clock; no requests, persistence or task-status polling.

import { useEffect, useState } from "react";

const MINUTE_MS = 60_000;

export function useMinuteClock(enabled: boolean): number {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    if (!enabled) return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const update = () => {
      clearTimeout(timer);
      if (document.visibilityState === "hidden") return;
      const current = Date.now();
      setNow(current);
      timer = setTimeout(update, MINUTE_MS - current % MINUTE_MS);
    };
    document.addEventListener("visibilitychange", update);
    update();
    return () => {
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", update);
    };
  }, [enabled]);
  return now;
}
