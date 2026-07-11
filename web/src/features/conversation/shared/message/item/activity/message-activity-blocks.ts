import type { ContentBlock } from "@/types/conversation/message/content";

export function findLastActivityBlock<Block extends ContentBlock>(
  content: readonly ContentBlock[],
  matches: (block: ContentBlock, index: number) => block is Block,
): Block | null {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (!block) {
      continue;
    }
    if (matches(block, index)) {
      return block;
    }
  }
  return null;
}
