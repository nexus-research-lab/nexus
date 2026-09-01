/**
 * Conversation Store Actions
 *
 * [INPUT]: 依赖 @/types、Session API 与当前 owner scope revision
 * [OUTPUT]: 对外提供 conversation 元数据同步 actions，迟到的旧 owner 读取不得提交
 * [POS]: store/conversation 模块的操作函数
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { Conversation, ConversationStoreState } from '@/types/conversation/conversation';
import { getConversations } from "@/lib/api/conversation/session-api";
import { getErrorMessage } from "@/lib/error-message";

type ConversationStoreSetter = (
  update:
    | Partial<ConversationStoreState>
    | ((state: ConversationStoreState) => Partial<ConversationStoreState>)
) => void;

function dedupeConversationsBySessionKey(
  conversations: Conversation[],
): Conversation[] {
  const uniqueConversations = new Map<string, Conversation>();
  for (const conversation of conversations) {
    const existingConversation = uniqueConversations.get(conversation.session_key);
    if (!existingConversation) {
      uniqueConversations.set(conversation.session_key, conversation);
      continue;
    }

    // 同一 sessionKey 必须只保留一条。
    // 冲突时优先使用最近活跃的记录，避免首页和 Launcher 出现重复 key。
    if (conversation.last_activity_at >= existingConversation.last_activity_at) {
      uniqueConversations.set(conversation.session_key, conversation);
    }
  }
  return Array.from(uniqueConversations.values());
}

export const syncConversationSnapshotAction = (
  set: ConversationStoreSetter
) => (
  key: string,
  patch: Partial<Pick<Conversation, 'last_activity_at' | 'session_id'>>
): void => {
  set((state) => {
    const idx = state.conversations.findIndex((c) => c.session_key === key);
    if (idx === -1) return { error: null };

    const current = state.conversations[idx];
    const nextLastActivityAt = patch.last_activity_at ?? current.last_activity_at;
    const nextSessionId = patch.session_id ?? current.session_id;
    const hasChanged =
      current.last_activity_at !== nextLastActivityAt ||
      current.session_id !== nextSessionId;

    // 流式过程中会高频同步快照，同值更新必须直接短路，避免触发无意义重渲染。
    if (!hasChanged) {
      return { error: null };
    }

    const patched: Conversation = {
      ...current,
      ...patch,
    };
    const activityChanged =
      patch.last_activity_at !== undefined &&
      patch.last_activity_at !== current.last_activity_at;

    let updatedConversations: Conversation[];
    if (activityChanged) {
      updatedConversations = [
        patched,
        ...state.conversations.slice(0, idx),
        ...state.conversations.slice(idx + 1),
      ];
    } else {
      updatedConversations = state.conversations.map((c, i) => (i === idx ? patched : c));
    }

    return { conversations: updatedConversations, error: null };
  });
};

export const loadConversationsFromServerAction = (
  set: ConversationStoreSetter,
  getOwnerScopeRevision: () => number,
) => async (): Promise<void> => {
  const ownerScopeRevision = getOwnerScopeRevision();
  try {
    set({ loading: true, error: null });

    const conversations = await getConversations();
    if (getOwnerScopeRevision() !== ownerScopeRevision) {
      return;
    }

    if (conversations && Array.isArray(conversations)) {
      const sortedConversations = dedupeConversationsBySessionKey(conversations)
        .sort((a, b) => b.last_activity_at - a.last_activity_at);
      console.debug(`[ConversationStore] Loaded ${sortedConversations.length} conversations`);
      set({ conversations: sortedConversations, loading: false, error: null });
    } else {
      set({ loading: false, error: 'Invalid response format' });
    }
  } catch (err) {
    if (getOwnerScopeRevision() !== ownerScopeRevision) {
      return;
    }
    console.error('[ConversationStore] Failed to load conversations:', err);
    set({
      loading: false,
      error: getErrorMessage(err, "会话列表暂时无法更新"),
    });
  }
};

export const clearAllConversationsAction = (
  set: ConversationStoreSetter
) => (): void => {
  set({
    conversations: [],
    error: null,
  });
};
