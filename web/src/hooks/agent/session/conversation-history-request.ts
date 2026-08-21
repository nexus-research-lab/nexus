/**
 * INPUT: 可返回 deferred indexing 状态的消息分页请求与取消信号。
 * OUTPUT: 等待索引完成后的完整消息页，或把取消原样传播给调用方。
 * POS: Agent Session 首屏、分页与目标轮次历史请求共享的建索引等待边界。
 */
import type { ConversationMessagePage } from "@/types/conversation/history";

interface ConversationHistoryPageRequest {
  loadPage: () => Promise<ConversationMessagePage>;
  signal?: AbortSignal;
}

export async function requestConversationHistoryPageUntilReady({
  loadPage,
  signal,
}: ConversationHistoryPageRequest): Promise<ConversationMessagePage> {
  for (;;) {
    const page = await loadPage();
    if (!page.indexing) {
      return page;
    }
    await waitForConversationHistoryIndex(page.retry_after_ms, signal);
  }
}

function waitForConversationHistoryIndex(
  retryAfterMs: number,
  signal?: AbortSignal,
): Promise<void> {
  if (signal?.aborted) {
    return Promise.reject(new DOMException("Aborted", "AbortError"));
  }
  const delay = Math.min(Math.max(retryAfterMs, 100), 5_000);
  return new Promise((resolve, reject) => {
    const timeout = globalThis.setTimeout(() => {
      signal?.removeEventListener("abort", handleAbort);
      resolve();
    }, delay);
    const handleAbort = (): void => {
      globalThis.clearTimeout(timeout);
      reject(new DOMException("Aborted", "AbortError"));
    };
    signal?.addEventListener("abort", handleAbort, { once: true });
  });
}
