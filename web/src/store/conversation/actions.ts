/**
 * Conversation Store Actions
 *
 * [INPUT]: 依赖 @/types, @/lib/agent-api
 * [OUTPUT]: 对外提供 conversation CRUD actions
 * [POS]: store/conversation 模块的操作函数
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { CreateConversationParams, Conversation, ConversationStoreState, UpdateConversationParams } from '@/types/conversation';
import { createDefaultConversation } from './utils';
import {
  createConversation,
  deleteConversation,
  getConversations,
  updateConversation,
} from "@/lib/agent-api";

type ConversationStoreSetter = (
  update:
    | Partial<ConversationStoreState>
    | ((state: ConversationStoreState) => Partial<ConversationStoreState>)
) => void;

export const createConversationAction = (
  set: ConversationStoreSetter,
) => async (params?: CreateConversationParams): Promise<string> => {
  const new_conversation = createDefaultConversation(params);

  try {
    const created = await createConversation(new_conversation.session_key, {
      title: params?.title,
      agent_id: params?.agent_id,
    });

    set((state) => ({
      conversations: [created, ...state.conversations.filter((item) => item.session_key !== new_conversation.session_key)],
      error: null,
    }));
    console.debug('[ConversationStore] Conversation synced:', created.session_key);
    return created.session_key;
  } catch (error) {
    console.error('[ConversationStore] Failed to sync conversation:', error);
    set((state) => ({
      conversations: [new_conversation, ...state.conversations],
      error: null,
    }));
    return new_conversation.session_key;
  }
};

export const deleteConversationAction = (
  set: ConversationStoreSetter,
) => async (key: string): Promise<void> => {
  try {
    await deleteConversation(key);

    set((state) => {
      const next_conversations = state.conversations.filter((item) => item.session_key !== key);
      const next_current_conversation_id = state.current_conversation_id === key
        ? (next_conversations[0]?.session_key || null)
        : state.current_conversation_id;

      return {
        conversations: next_conversations,
        current_conversation_id: next_current_conversation_id,
        error: null,
      };
    });
  } catch (error) {
    console.error('[ConversationStore] Failed to delete conversation:', error);
    set(() => ({ error: 'Failed to delete conversation' }));
  }
};

export const updateConversationAction = (
  set: ConversationStoreSetter
) => async (key: string, params: UpdateConversationParams): Promise<void> => {
  try {
    await updateConversation(key, params);

    set((state) => ({
      conversations: state.conversations.map((conversation) =>
        conversation.session_key === key
          ? {
            ...conversation,
            ...(params.title && { title: params.title }),
            last_activity_at: Date.now(),
          }
          : conversation
      ),
      error: null,
    }));
  } catch (error) {
    console.error('[ConversationStore] Failed to update conversation:', error);
    set(() => ({ error: 'Failed to sync update with server' }));
  }
};

export const setCurrentConversationAction = (
  set: ConversationStoreSetter
) => (key: string | null): void => {
  set({ current_conversation_id: key, error: null });
};

export const syncConversationSnapshotAction = (
  set: ConversationStoreSetter
) => (
  key: string,
  patch: Partial<Pick<Conversation, 'message_count' | 'last_activity_at' | 'session_id'>>
): void => {
  set((state) => {
    const updated_conversations = state.conversations.map((conversation) =>
      conversation.session_key === key
        ? {
          ...conversation,
          ...patch,
        }
        : conversation
    );

    updated_conversations.sort((left, right) => right.last_activity_at - left.last_activity_at);
    return {
      conversations: updated_conversations,
      error: null,
    };
  });
};

export const getConversationAction = (get: () => ConversationStoreState) => (key: string): Conversation | undefined => {
  return get().conversations.find((item) => item.session_key === key);
};

export const loadConversationsFromServerAction = (
  set: ConversationStoreSetter,
) => async (): Promise<void> => {
  try {
    set({ loading: true, error: null });

    const conversations = await getConversations();

    if (conversations && Array.isArray(conversations)) {
      const sorted_conversations = [...conversations].sort((a, b) => b.last_activity_at - a.last_activity_at);
      console.debug(`[ConversationStore] Loaded ${sorted_conversations.length} conversations`);
      set({ conversations: sorted_conversations, loading: false, error: null });
    } else {
      set({ loading: false, error: 'Invalid response format' });
    }
  } catch (err) {
    console.error('[ConversationStore] Failed to load conversations:', err);
    set({
      loading: false,
      error: err instanceof Error ? err.message : 'Unknown error',
    });
  }
};

export const clearAllConversationsAction = (
  set: ConversationStoreSetter
) => (): void => {
  set({
    conversations: [],
    current_conversation_id: null,
    error: null,
  });
};
