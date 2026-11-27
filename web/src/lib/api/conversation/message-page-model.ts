import type {
  ApiConversationMessagePage,
  ConversationMessagePage,
  ConversationMessagesQuery,
} from "@/types/conversation/history";

type QueryValueResolver = (
  options: ConversationMessagesQuery,
) => string | null;

interface QueryFieldRule {
  key: string;
  resolve: QueryValueResolver;
}

const MESSAGE_QUERY_FIELD_RULES: readonly QueryFieldRule[] = [
  {key: "limit", resolve: (options) => serializePositiveNumber(options.limit)},
  {key: "before_round_id", resolve: (options) => options.before_round_id || null},
  {
    key: "before_round_timestamp",
    resolve: (options) => serializePositiveNumber(options.before_round_timestamp),
  },
  {key: "around_round_id", resolve: (options) => options.around_round_id || null},
  {
    key: "around_limit",
    resolve: (options) => serializePositiveNumber(options.around_limit),
  },
];

/**
 * 历史页与轮次导航的请求超时。会话第一次被读取时，后端在请求里同步回填整段历史（大群聊可达数十秒），
 * 默认 30s 会在回填完成前放弃并报加载失败；服务端写超时是 6 分钟，这里留在它之内。
 */
export const CONVERSATION_HISTORY_REQUEST_TIMEOUT_MS = 120_000;

function serializePositiveNumber(
  value: number | null | undefined,
): string | null {
  return value && value > 0 ? String(value) : null;
}

export function buildConversationMessagesQuerySuffix(
  options: ConversationMessagesQuery,
  initialEntries: ReadonlyArray<readonly [string, string]> = [],
): string {
  const params = new URLSearchParams();
  for (const [key, value] of initialEntries) {
    params.set(key, value);
  }
  for (const rule of MESSAGE_QUERY_FIELD_RULES) {
    const value = rule.resolve(options);
    if (value) {
      params.set(rule.key, value);
    }
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

export function normalizeConversationMessagePage(
  page: ApiConversationMessagePage,
): ConversationMessagePage {
  return {
    has_more: page.has_more ?? false,
    items: page.items ?? [],
    next_before_round_id: page.next_before_round_id ?? null,
    next_before_round_timestamp: page.next_before_round_timestamp ?? null,
  };
}
