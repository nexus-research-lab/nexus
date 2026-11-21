/**
 * INPUT: 历史、实时和乐观消息状态更新，以及阅读视口所在的窗口端。
 * OUTPUT: 去重且按完整 round/估算字节受限的 React 消息窗口与其断点。
 * POS: Agent 会话消息数组的唯一内存预算入口。
 */
import { useCallback, useRef, useState } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import type { Message } from "@/types/conversation/message/entity";

import { dedupeMessagesById } from "./message-collection-model";
import {
  boundMessageWindow,
  type LoadedMessageWindow,
  type MessageWindowEdge,
} from "./message-window-model";

export type MessageWindowUpdater = (
  current: LoadedMessageWindow,
) => LoadedMessageWindow;

const EMPTY_WINDOW: LoadedMessageWindow = {
  gapAfterRoundId: null,
  messages: [],
};

export function useAgentMessageCollection(): {
  gapAfterRoundId: string | null;
  messages: Message[];
  setEvictionEdge: (edge: MessageWindowEdge) => void;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  updateMessageWindow: (updater: MessageWindowUpdater) => void;
  windowRef: RefObject<LoadedMessageWindow>;
} {
  const [window, setWindow] = useState<LoadedMessageWindow>(EMPTY_WINDOW);
  // 历史请求在异步边界读取最近一次提交的窗口，用它规划下一页。
  const windowRef = useRef(window);
  windowRef.current = window;
  // 实时写入超出预算时让出远离阅读视口的一端；该选择不参与渲染。
  const evictionEdgeRef = useRef<MessageWindowEdge>("head");

  const setEvictionEdge = useCallback((edge: MessageWindowEdge) => {
    evictionEdgeRef.current = edge;
  }, []);

  const updateMessageWindow = useCallback((updater: MessageWindowUpdater) => {
    setWindow((current) => {
      const next = updater(current);
      const bounded = boundMessageWindow(
        { ...next, messages: dedupeMessagesById(next.messages) },
        { evict: evictionEdgeRef.current },
      );
      return (
        bounded.messages === current.messages
        && bounded.gapAfterRoundId === current.gapAfterRoundId
      )
        ? current
        : bounded;
    });
  }, []);

  const setMessages = useCallback<Dispatch<SetStateAction<Message[]>>>(
    (nextState) => {
      updateMessageWindow((current) => ({
        gapAfterRoundId: current.gapAfterRoundId,
        messages: typeof nextState === "function"
          ? nextState(current.messages)
          : nextState,
      }));
    },
    [updateMessageWindow],
  );

  return {
    gapAfterRoundId: window.gapAfterRoundId,
    messages: window.messages,
    setEvictionEdge,
    setMessages,
    updateMessageWindow,
    windowRef,
  };
}
