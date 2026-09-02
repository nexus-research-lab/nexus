import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const srcRoot = path.join(webRoot, "src");

// 迁移债务只允许缩小。保留在这里不表示这些依赖符合新规范。
const SHARED_UPWARD_IMPORT_BASELINE = new Set([
  "src/shared/ui/display/qr-code.tsx -> @/hooks/ui/use-resettable-state",
  "src/shared/ui/markdown/code/code-block-content.tsx -> @/hooks/ui/use-copy-to-clipboard",
  "src/shared/ui/markdown/core/markdown-components.tsx -> @/features/conversation/shared/message/agent-mention-chip",
  "src/shared/ui/markdown/core/markdown-components.tsx -> @/features/conversation/shared/slash-command-presentation",
  "src/shared/ui/markdown/core/markdown-components.tsx -> @/features/conversation/shared/slash-command-token",
  "src/shared/ui/markdown/mermaid/mermaid-preview-dialog.tsx -> @/hooks/ui/use-resettable-state",
  "src/shared/ui/markdown/mermaid/mermaid-view.tsx -> @/hooks/ui/clipboard",
  "src/shared/ui/markdown/streaming/use-smooth-streaming-markdown-content.ts -> @/hooks/ui/use-prefers-reduced-motion",
  "src/shared/ui/markdown/workspace/use-markdown-workspace-files.ts -> @/store/agent",
  "src/shared/ui/markdown/workspace/use-markdown-workspace-files.ts -> @/store/workspace-files",
  "src/shared/ui/mention/mention-target-popover.tsx -> @/hooks/ui/use-resettable-state",
  "src/shared/ui/workspace/controls/conversation-tabs/use-conversation-tabs-controller.ts -> @/store/room-navigation",
  "src/shared/ui/workspace/controls/workspace-conversation-tabs.tsx -> @/store/room-navigation",
]);

const HOOK_UPWARD_IMPORT_BASELINE = new Set([
  "src/hooks/agent/use-agent-conversation.ts -> @/features/conversation/shared/execution/workgraph-distillation-intent",
]);

async function collectTypeScriptFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map(async (entry) => {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      return collectTypeScriptFiles(target);
    }
    return /\.tsx?$/.test(entry.name) ? [target] : [];
  }));
  return nested.flat();
}

function collectImports(source) {
  return Array.from(
    source.matchAll(/(?:from\s+|import\s*\()["']([^"']+)["']/g),
    (match) => match[1],
  );
}

test("shared and global hooks do not gain new upward business dependencies", async () => {
  const files = await collectTypeScriptFiles(srcRoot);
  const currentSharedDebt = new Set();
  const currentHookDebt = new Set();

  for (const file of files) {
    const relativePath = path.relative(webRoot, file);
    const source = await readFile(file, "utf8");
    for (const importedPath of collectImports(source)) {
      const identity = `${relativePath} -> ${importedPath}`;
      if (
        relativePath.startsWith("src/shared/")
        && /^@\/(?:app|pages|widgets|features|entities|hooks|store)(?:\/|$)/.test(importedPath)
      ) {
        currentSharedDebt.add(identity);
      }
      if (
        relativePath.startsWith("src/hooks/")
        && /^@\/(?:app|pages|widgets|features)(?:\/|$)/.test(importedPath)
      ) {
        currentHookDebt.add(identity);
      }
    }
  }

  assert.deepEqual(currentSharedDebt, SHARED_UPWARD_IMPORT_BASELINE);
  assert.deepEqual(currentHookDebt, HOOK_UPWARD_IMPORT_BASELINE);
});
