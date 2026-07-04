import { useEffect, useRef, useState } from "react";

import { getSessionRoundIndexApi } from "@/lib/api/agent-api";
import type { SessionRoundIndexItem } from "@/types/conversation/room";

export function useSessionRoundIndex(sessionKey: string | null) {
  const [items, setItems] = useState<SessionRoundIndexItem[]>([]);
  const requestIdRef = useRef(0);

  useEffect(() => {
    requestIdRef.current += 1;
    const requestId = requestIdRef.current;
    const normalizedSessionKey = sessionKey?.trim() ?? "";
    if (!normalizedSessionKey) {
      setItems([]);
      return;
    }

    void getSessionRoundIndexApi(normalizedSessionKey)
      .then((nextItems) => {
        if (requestIdRef.current !== requestId) {
          return;
        }
        setItems(nextItems);
      })
      .catch((error) => {
        if (requestIdRef.current !== requestId) {
          return;
        }
        console.error("[useSessionRoundIndex] 加载 session round 索引失败:", error);
        setItems([]);
      });
  }, [sessionKey]);

  return items;
}
