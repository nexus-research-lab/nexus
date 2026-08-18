/**
 * INPUT: 历史、实时和乐观消息状态更新。
 * OUTPUT: 去重且按完整 round/估算字节受限的 React 消息集合。
 * POS: Agent 会话消息数组的唯一内存预算入口。
 */
import { useCallback, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { Message } from "@/types/conversation/message/entity";

import { dedupeMessagesById } from "./message-collection-model";
import { boundLoadedMessages } from "./message-window-model";

export function useAgentMessageCollection(): {
  messages: Message[];
  setMessages: Dispatch<SetStateAction<Message[]>>;
} {
  const [messages, setMessagesState] = useState<Message[]>([]);
  const setMessages = useCallback<Dispatch<SetStateAction<Message[]>>>(
    (nextState) => {
      setMessagesState((currentMessages) => {
        const nextMessages = typeof nextState === "function"
          ? nextState(currentMessages)
          : nextState;
        return boundLoadedMessages(dedupeMessagesById(nextMessages), {
          preference: "latest",
        });
      });
    },
    [],
  );
  return { messages, setMessages };
}
