import {
  Dispatch,
  SetStateAction,
  startTransition,
  useCallback,
  useEffect,
  useRef,
} from "react";
import type { Message } from "@/types/conversation/message/entity";
import type { StreamMessage } from "@/types/conversation/message/event";
import { applyStreamMessage } from "../message/stream-message-reducer";

export function useConversationStreamBuffer(
  setMessages: Dispatch<SetStateAction<Message[]>>,
): (payload: StreamMessage) => void {
  const streamBufferRef = useRef<StreamMessage[]>([]);
  const streamRafRef = useRef<number | null>(null);

  const flushStreamBuffer = useCallback(() => {
    streamRafRef.current = null;
    const payloads = streamBufferRef.current;
    if (payloads.length === 0) {
      return;
    }
    streamBufferRef.current = [];

    startTransition(() => {
      setMessages((prev) => {
        let next = prev;
        for (const payload of payloads) {
          next = applyStreamMessage(next, payload);
        }
        return next;
      });
    });
  }, [setMessages]);

  useEffect(() => {
    return () => {
      if (streamRafRef.current !== null) {
        cancelAnimationFrame(streamRafRef.current);
        streamRafRef.current = null;
      }
    };
  }, []);

  return useCallback(
    (payload: StreamMessage) => {
      streamBufferRef.current.push(payload);
      if (streamRafRef.current === null) {
        streamRafRef.current = requestAnimationFrame(flushStreamBuffer);
      }
    },
    [flushStreamBuffer],
  );
}
