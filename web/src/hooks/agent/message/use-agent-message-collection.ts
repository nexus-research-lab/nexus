/**
 * INPUT: 历史、实时和乐观消息状态更新，以及阅读视口所在的窗口端。
 * OUTPUT: 去重且按完整 round/估算字节受限的 React 消息窗口与其断点；windowRef 随每次写入同步前进。
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
  // windowRef 是窗口的唯一真相，写入时同步前进，React state 只驱动渲染。历史游标在页面
  // 落地时同步提交；若窗口要等下一次 render 才前进，React 渲染前触发的边界检查就会拿
  // 新游标配旧窗口头部，判定游标失效，再按旧头部把刚取回的同一页重复请求一次。
  const windowRef = useRef(window);
  // 实时写入超出预算时让出远离阅读视口的一端；该选择不参与渲染。
  const evictionEdgeRef = useRef<MessageWindowEdge>("head");

  const setEvictionEdge = useCallback((edge: MessageWindowEdge) => {
    evictionEdgeRef.current = edge;
  }, []);

  const updateMessageWindow = useCallback((updater: MessageWindowUpdater) => {
    const current = windowRef.current;
    const next = updater(current);
    const bounded = boundMessageWindow(
      { ...next, messages: dedupeMessagesById(next.messages) },
      { evict: evictionEdgeRef.current },
    );
    if (
      bounded.messages === current.messages
      && bounded.gapAfterRoundId === current.gapAfterRoundId
    ) {
      return;
    }
    windowRef.current = bounded;
    setWindow(bounded);
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
