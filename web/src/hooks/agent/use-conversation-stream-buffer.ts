import {
  Dispatch,
  SetStateAction,
  startTransition,
  useCallback,
  useEffect,
  useRef,
} from "react";
import { Message, StreamMessage } from "@/types";
import { apply_stream_message } from "./message-helpers";

export function useConversationStreamBuffer(
  set_messages: Dispatch<SetStateAction<Message[]>>,
): (payload: StreamMessage) => void {
  const stream_buffer_ref = useRef<StreamMessage[]>([]);
  const stream_raf_ref = useRef<number | null>(null);

  const flush_stream_buffer = useCallback(() => {
    stream_raf_ref.current = null;
    const payloads = stream_buffer_ref.current;
    if (payloads.length === 0) {
      return;
    }
    stream_buffer_ref.current = [];

    startTransition(() => {
      set_messages((prev) => {
        let next = prev;
        for (const payload of payloads) {
          next = apply_stream_message(next, payload);
        }
        return next;
      });
    });
  }, [set_messages]);

  useEffect(() => {
    return () => {
      if (stream_raf_ref.current !== null) {
        cancelAnimationFrame(stream_raf_ref.current);
        stream_raf_ref.current = null;
      }
    };
  }, []);

  return useCallback(
    (payload: StreamMessage) => {
      stream_buffer_ref.current.push(payload);
      if (stream_raf_ref.current === null) {
        stream_raf_ref.current = requestAnimationFrame(flush_stream_buffer);
      }
    },
    [flush_stream_buffer],
  );
}
