import type { Message } from "@/types/conversation/message/entity";

export interface ConversationThreadRound {
  messages: Message[];
  roundId: string;
}
