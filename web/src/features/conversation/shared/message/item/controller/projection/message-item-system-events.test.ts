// INPUT: runtime compact_boundary 系统消息（nxs camelCase / Claude Code snake_case 元数据）。
// OUTPUT: 压缩前后 token 与摘要只在投影入口读取一次。
// POS: 系统事件投影回归。
import { expect, it } from "vitest";
import type { Message } from "@/types/conversation/message/entity";
import { buildSystemEventBlocks } from "./message-item-system-events";

const boundary = (metadata: Record<string, unknown>) => ({
  message_id: "cb", role: "system", content: "Conversation compacted", timestamp: 1,
  metadata: { subtype: "compact_boundary", ...metadata },
}) as unknown as Message;

it("projects compaction tokens and summary from both runtime shapes", () => {
  const [nxs] = buildSystemEventBlocks([boundary({ compact_metadata: { preTokens: 9000, postTokens: 900 }, summary: "  进行中的 Q3 复盘  " })], true);
  expect(nxs).toMatchObject({ compact_pre_tokens: 9000, compact_post_tokens: 900, compact_summary: "进行中的 Q3 复盘" });
  const [claude] = buildSystemEventBlocks([boundary({ compact_metadata: { pre_tokens: 5000 } })], true);
  expect(claude).toMatchObject({ compact_pre_tokens: 5000, compact_post_tokens: null, compact_summary: null });
});

it("leaves host run outcomes to the trigger divider", () => {
  const outcome = { message_id: "host_outcome_r", role: "system", content: "quiet", timestamp: 1,
    metadata: { subtype: "host_run_outcome", outcome: "quiet" } } as unknown as Message;
  expect(buildSystemEventBlocks([outcome], true)).toEqual([]);
});
