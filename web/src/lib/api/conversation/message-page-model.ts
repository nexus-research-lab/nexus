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
