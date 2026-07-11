import { useCallback, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { Message } from "@/types";

import { dedupeMessagesById } from "./message-collection-model";

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
        return dedupeMessagesById(nextMessages);
      });
    },
    [],
  );
  return { messages, setMessages };
}
