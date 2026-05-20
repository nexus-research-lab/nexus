import { read_markdown_fence_marker } from "./markdown-fence";

export type MarkdownStreamBlockState = "revealed" | "streaming";

export interface MarkdownStreamBlock {
  content: string;
  start_offset: number;
  state: MarkdownStreamBlockState;
}

interface MarkdownRawBlock {
  content: string;
  start_offset: number;
}

function get_lines_with_endings(content: string): string[] {
  return content.match(/[^\n]*(?:\n|$)/g)?.filter((line) => line.length > 0) ?? [];
}

function is_blank_line(line: string): boolean {
  return line.trim().length === 0;
}

function is_standalone_block_line(line: string): boolean {
  return /^ {0,3}#{1,6}\s+\S/.test(line) || /^ {0,3}(?:-{3,}|\*{3,}|_{3,})\s*$/.test(line);
}

function split_markdown_raw_blocks(content: string): MarkdownRawBlock[] {
  const blocks: MarkdownRawBlock[] = [];
  const buffer: string[] = [];
  let block_start_offset = 0;
  let cursor_offset = 0;
  let open_fence: { marker: "`" | "~"; length: number } | null = null;

  const flush_buffer = () => {
    if (buffer.length === 0) {
      block_start_offset = cursor_offset;
      return;
    }

    blocks.push({
      content: buffer.join(""),
      start_offset: block_start_offset,
    });
    buffer.length = 0;
    block_start_offset = cursor_offset;
  };

  for (const line of get_lines_with_endings(content)) {
    const fence_marker = read_markdown_fence_marker(line);

    buffer.push(line);
    cursor_offset += line.length;

    if (open_fence) {
      if (
        fence_marker &&
        fence_marker.marker === open_fence.marker &&
        fence_marker.length >= open_fence.length
      ) {
        open_fence = null;
        flush_buffer();
      }
      continue;
    }

    if (fence_marker) {
      open_fence = fence_marker;
      continue;
    }

    if (is_blank_line(line) || (buffer.length === 1 && is_standalone_block_line(line))) {
      flush_buffer();
    }
  }

  flush_buffer();
  return blocks;
}

export function split_streaming_markdown_blocks(content: string): MarkdownStreamBlock[] {
  const raw_blocks = split_markdown_raw_blocks(content);
  const tail_index = raw_blocks.length - 1;

  return raw_blocks.map((block, index) => ({
    ...block,
    state: index === tail_index ? "streaming" : "revealed",
  }));
}
