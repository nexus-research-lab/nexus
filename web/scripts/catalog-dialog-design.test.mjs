import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

test("Composer catalogs use plain directory chrome instead of explanatory cards", async () => {
  const [loopDialog, loopItem, workgraphDialog, composerZh] = await Promise.all([
    read("web/src/features/conversation/shared/composer/components/loop-picker/loop-picker-dialog.tsx"),
    read("web/src/features/conversation/shared/composer/components/loop-picker/loop-picker-item.tsx"),
    read("web/src/features/conversation/shared/composer/components/workgraph-distillation-picker/workgraph-distillation-picker-dialog.tsx"),
    read("web/src/shared/i18n/catalog/zh/conversation.ts"),
  ]);

  for (const source of [loopDialog, workgraphDialog]) {
    assert.match(source, /<UiDialogHeader[\s\S]*appearance="plain"/);
    assert.doesNotMatch(source, /subtitle=|GitBranchPlus|Repeat2/);
  }
  assert.doesNotMatch(loopItem, /<UiButton|rounded-\[6px\]|surface-interactive-hover-background\) px-2/);
  assert.doesNotMatch(composerZh, /loop_picker_subtitle|workgraph_picker_subtitle/);
});

test("Attachment and Mermaid previews keep only meaningful visible chrome", async () => {
  const [attachmentPreview, mermaidPreview, composerZh] = await Promise.all([
    read("web/src/features/conversation/shared/composer/attachments/composer-attachment-preview-dialog.tsx"),
    read("web/src/shared/ui/markdown/mermaid/mermaid-preview-dialog.tsx"),
    read("web/src/shared/i18n/catalog/zh/conversation.ts"),
  ]);

  assert.match(attachmentPreview, /<UiDialogHeader[\s\S]*appearance="plain"/);
  assert.doesNotMatch(attachmentPreview, /"composer\.(image|text)_preview"/);
  assert.doesNotMatch(composerZh, /"composer\.(image|text)_preview":/);
  assert.match(mermaidPreview, /<h2 className="sr-only"/);
  assert.doesNotMatch(mermaidPreview, /<UiDialogHeader/);
});

function read(relativePath) {
  return readFile(path.join(repositoryRoot, relativePath), "utf8");
}
