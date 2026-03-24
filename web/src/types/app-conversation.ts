export interface AppConversationMessage {
  body: string;
  created_at: number;
  id: string;
  role: "app" | "user";
}

export interface AppConversationState {
  messages: AppConversationMessage[];
  clear_messages: () => void;
  submit_prompt: (prompt: string) => void;
}
