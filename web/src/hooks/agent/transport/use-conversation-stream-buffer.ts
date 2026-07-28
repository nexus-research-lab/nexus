/**
 * INPUT: WebSocket 顺序到达的 stream event、最新活动 session_key 与完整 message 快照提交边界。
 * OUTPUT: 仅向当前会话应用的同帧 stream 合批、完整快照前同步 flush 与 live 首帧标记清理。
 * POS: Agent transport 的会话隔离有序流缓冲；旧 patch 不得越界，首批正文必须先进入平滑 backlog。
 */
import {
  Dispatch,
  RefObject,
  SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useRef,
} from "react";
import type { Message } from "@/types/conversation/message/entity";
import type { StreamMessage } from "@/types/conversation/message/event";
import {
  clearLiveStreamRevealMarkers,
  hasVisibleSmoothRevealContent,
} from "@/lib/conversation/live-stream-reveal";
import { applyStreamMessage } from "../message/stream-message-reducer";

export interface ConversationStreamBuffer {
  enqueueStreamPayload: (payload: StreamMessage) => void;
  flushStreamPayloads: () => void;
  settleLiveMessageSnapshot: (message: Message) => void;
}

/** 保持 batch 内到达顺序，并在提交时拒绝已经失效的会话批次。 */
export function applyStreamPayloadBatchForActiveSession(
  messages: Message[],
  payloads: StreamMessage[],
  bufferedSessionKey: string | null,
  activeSessionKey: string | null,
): Message[] {
  if (
    !bufferedSessionKey
    || bufferedSessionKey !== activeSessionKey
  ) {
    return messages;
  }

  let next = messages;
  for (const payload of payloads) {
    if (payload.session_key === bufferedSessionKey) {
      next = applyStreamMessage(next, payload);
    }
  }
  return next;
}

export function useConversationStreamBuffer(
  setMessages: Dispatch<SetStateAction<Message[]>>,
  activeSessionKeyRef: RefObject<string | null>,
): ConversationStreamBuffer {
  const streamBufferRef = useRef<StreamMessage[]>([]);
  const streamRafRef = useRef<number | null>(null);
  const revealClearIdsRef = useRef<Set<string>>(new Set());
  const revealClearRafRef = useRef<number | null>(null);

  const scheduleRevealMarkerClear = useCallback((messageIds: string[]) => {
    messageIds.forEach((messageId) => revealClearIdsRef.current.add(messageId));
    if (
      revealClearIdsRef.current.size === 0
      || revealClearRafRef.current !== null
    ) {
      return;
    }
    // 第一帧先让 Markdown hook 读取本地标记并从空正文建立 backlog；
    // 下一帧只移除标记，已经挂载的 hook 会继续单调追赶目标内容。
    revealClearRafRef.current = requestAnimationFrame(() => {
      revealClearRafRef.current = null;
      const messageIdsToClear = revealClearIdsRef.current;
      revealClearIdsRef.current = new Set();
      setMessages((messages) => clearLiveStreamRevealMarkers(
        messages,
        messageIdsToClear,
      ));
    });
  }, [setMessages]);

  const flushStreamBuffer = useCallback(() => {
    if (streamRafRef.current !== null) {
      cancelAnimationFrame(streamRafRef.current);
      streamRafRef.current = null;
    }
    const bufferedPayloads = streamBufferRef.current;
    streamBufferRef.current = [];
    const activeSessionKey = activeSessionKeyRef.current;
    const payloads = activeSessionKey
      ? bufferedPayloads.filter(
        (payload) => payload.session_key === activeSessionKey,
      )
      : [];
    if (payloads.length === 0) {
      return;
    }

    // RAF 已经把同一可见帧内的 token 合成一次提交。这里不能再降为
    // transition，否则重 Markdown 工作会推迟/合并中间帧，表现成整段刷出。
    setMessages((prev) => applyStreamPayloadBatchForActiveSession(
      prev,
      payloads,
      activeSessionKey,
      activeSessionKeyRef.current,
    ));
    scheduleRevealMarkerClear(
      payloads
        .filter((payload) => hasVisibleSmoothRevealContent(
          payload.content_block,
        ))
        .map((payload) => payload.message_id),
    );
  }, [activeSessionKeyRef, scheduleRevealMarkerClear, setMessages]);

  useEffect(() => {
    return () => {
      if (streamRafRef.current !== null) {
        cancelAnimationFrame(streamRafRef.current);
        streamRafRef.current = null;
      }
      if (revealClearRafRef.current !== null) {
        cancelAnimationFrame(revealClearRafRef.current);
        revealClearRafRef.current = null;
      }
      streamBufferRef.current = [];
      revealClearIdsRef.current.clear();
    };
  }, []);

  const enqueueStreamPayload = useCallback(
    (payload: StreamMessage) => {
      streamBufferRef.current.push(payload);
      if (streamRafRef.current === null) {
        streamRafRef.current = requestAnimationFrame(flushStreamBuffer);
      }
    },
    [flushStreamBuffer],
  );

  const settleLiveMessageSnapshot = useCallback((message: Message) => {
    if (
      message.role === "assistant"
      && message.content.some(hasVisibleSmoothRevealContent)
    ) {
      scheduleRevealMarkerClear([message.message_id]);
    }
  }, [scheduleRevealMarkerClear]);

  return useMemo(
    () => ({
      enqueueStreamPayload,
      flushStreamPayloads: flushStreamBuffer,
      settleLiveMessageSnapshot,
    }),
    [
      enqueueStreamPayload,
      flushStreamBuffer,
      settleLiveMessageSnapshot,
    ],
  );
}
