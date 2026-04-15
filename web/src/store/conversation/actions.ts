/**
 * Conversation Store Actions
 *
 * [INPUT]: 依赖 @/types, @/lib/agent-api
 * [OUTPUT]: 对外提供 conversation 元数据同步 actions
 * [POS]: store/conversation 模块的操作函数
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { Conversation, ConversationStoreState } from '@/types/conversation';
import { get_conversations } from "@/lib/agent-api";
import { is_legacy_launcher_app_session_key } from "@/lib/session-key";

type ConversationStoreSetter = (
  update:
    | Partial<ConversationStoreState>
    | ((state: ConversationStoreState) => Partial<ConversationStoreState>)
) => void;

function dedupe_conversations_by_session_key(
  conversations: Conversation[],
): Conversation[] {
  const unique_conversations = new Map<string, Conversation>();
  for (const conversation of conversations) {
    if (is_legacy_launcher_app_session_key(conversation.session_key)) {
      continue;
    }
    const existing_conversation = unique_conversations.get(conversation.session_key);
    if (!existing_conversation) {
      unique_conversations.set(conversation.session_key, conversation);
      continue;
    }

    // 中文注释：同一 session_key 必须只保留一条。
    // 冲突时优先使用最近活跃的记录，避免首页和 Launcher 出现重复 key。
    if (conversation.last_activity_at >= existing_conversation.last_activity_at) {
      unique_conversations.set(conversation.session_key, conversation);
    }
  }
  return Array.from(unique_conversations.values());
}

export const sync_conversation_snapshot_action = (
  set: ConversationStoreSetter
) => (
  key: string,
  patch: Partial<Pick<Conversation, 'message_count' | 'last_activity_at' | 'session_id'>>
): void => {
  set((state) => {
    const idx = state.conversations.findIndex((c) => c.session_key === key);
    if (idx === -1) return { error: null };

    const current = state.conversations[idx];
    const next_last_activity_at = patch.last_activity_at ?? current.last_activity_at;
    const next_message_count = patch.message_count ?? current.message_count;
    const next_session_id = patch.session_id ?? current.session_id;
    const has_changed =
      current.last_activity_at !== next_last_activity_at ||
      current.message_count !== next_message_count ||
      current.session_id !== next_session_id;

    // 中文注释：流式过程中会高频同步快照，同值更新必须直接短路，避免触发无意义重渲染。
    if (!has_changed) {
      return { error: null };
    }

    const patched: Conversation = {
      ...current,
      ...patch,
    };
    const activity_changed =
      patch.last_activity_at !== undefined &&
      patch.last_activity_at !== current.last_activity_at;

    let updated_conversations: Conversation[];
    if (activity_changed) {
      updated_conversations = [
        patched,
        ...state.conversations.slice(0, idx),
        ...state.conversations.slice(idx + 1),
      ];
    } else {
      updated_conversations = state.conversations.map((c, i) => (i === idx ? patched : c));
    }

    return { conversations: updated_conversations, error: null };
  });
};

export const load_conversations_from_server_action = (
  set: ConversationStoreSetter,
) => async (): Promise<void> => {
  try {
    set({ loading: true, error: null });

    const conversations = await get_conversations();

    if (conversations && Array.isArray(conversations)) {
      const sorted_conversations = dedupe_conversations_by_session_key(conversations)
        .sort((a, b) => b.last_activity_at - a.last_activity_at);
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

export const clear_all_conversations_action = (
  set: ConversationStoreSetter
) => (): void => {
  set({
    conversations: [],
    error: null,
  });
};
