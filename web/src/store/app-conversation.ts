import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJSONStorage } from "@/lib/browser-storage";
import { AppConversationMessage, AppConversationState } from "@/types/app-conversation";

function create_message(role: AppConversationMessage["role"], body: string): AppConversationMessage {
  return {
    id: `${role}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    role,
    body,
    created_at: Date.now(),
  };
}

function build_app_reply(prompt: string): string {
  const normalized_prompt = prompt.trim().toLowerCase();

  if (normalized_prompt.includes("恢复") || normalized_prompt.includes("继续")) {
    return "我会优先帮你恢复最近相关的协作，再判断应该回到哪一个 room 继续推进。";
  }

  if (normalized_prompt.includes("创建") || normalized_prompt.includes("新建")) {
    return "我会先整理这次任务需要的成员、上下文和目标，再把它落成一个真正承载执行的 room。";
  }

  if (normalized_prompt.includes("成员") || normalized_prompt.includes("联系人") || normalized_prompt.includes("邀请")) {
    return "我会先帮你判断需要哪些成员参与，再决定是发起 1v1，还是组织成一个多人 room。";
  }

  return "我已经接住这条系统级意图。下一步我会围绕它组织协作结构、整理上下文，并把你带到合适的 room。";
}

export const useAppConversationStore = create<AppConversationState>()(
  persist(
    (set) => ({
      messages: [],

      clear_messages: () => {
        set({ messages: [] });
      },

      submit_prompt: (prompt: string) => {
        const trimmed_prompt = prompt.trim();
        if (!trimmed_prompt) {
          return;
        }

        set((state) => ({
          messages: [
            ...state.messages,
            create_message("user", trimmed_prompt),
            create_message("app", build_app_reply(trimmed_prompt)),
          ],
        }));
      },
    }),
    {
      name: "agent-ui-app-conversation",
      storage: createBrowserJSONStorage(),
      partialize: (state) => ({
        messages: state.messages,
      }),
    },
  ),
);
