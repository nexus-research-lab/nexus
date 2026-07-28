import {
  Dispatch,
  SetStateAction,
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

    // RAF 已经把同一可见帧内的 token 合成一次提交。这里不能再降为
    // transition，否则重 Markdown 工作会推迟/合并中间帧，表现成整段刷出。
    setMessages((prev) => {
      let next = prev;
      for (const payload of payloads) {
        next = applyStreamMessage(next, payload);
      }
      return next;
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
