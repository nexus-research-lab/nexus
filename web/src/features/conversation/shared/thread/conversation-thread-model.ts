import type { Message } from "@/types/conversation/message";

export interface ConversationThreadRound {
  messages: Message[];
  roundId: string;
}
