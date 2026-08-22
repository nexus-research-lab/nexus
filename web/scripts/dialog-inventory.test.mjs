import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));
const sourceRoot = path.join(repositoryRoot, "web/src");
const dialogPrimitiveRoot = path.join(sourceRoot, "shared/ui/dialog");

test("dialog inventory stays explicit when a modal or dialog overlay is added", async () => {
  const files = await collectTsxFiles(sourceRoot);
  let modalRoots = 0;
  const dialogOverlays = [];

  for (const file of files) {
    if (file.startsWith(dialogPrimitiveRoot)) continue;
    const source = await readFile(file, "utf8");
    const roots = source.match(/<(?:UiDialogBackdrop|ConfirmDialog|PromptDialog)\b/g) ?? [];
    modalRoots += roots.length;
    if (/(?:role="dialog"|aria-modal="true")/.test(source)) {
      dialogOverlays.push(path.relative(repositoryRoot, file));
    }
  }

  assert.equal(modalRoots, 59);
  assert.deepEqual(dialogOverlays.sort(), [
    "web/src/features/conversation/room/surface/history/room-history-menu.tsx",
    "web/src/features/conversation/room/surface/mobile/room-mobile-conversation-switcher.tsx",
    "web/src/features/conversation/shared/composer/components/footer/composer-room-model-control.tsx",
    "web/src/features/conversation/shared/message/item/view/assistant/assistant-message-stats.tsx",
    "web/src/shared/ui/icon-picker/icon-picker-popover.tsx",
  ]);

  const specification = await readFile(
    path.join(repositoryRoot, "docs/specs/dialog-design-spec.md"),
    "utf8",
  );
  assert.match(specification, /59 个产品模态根/);
  assert.match(specification, /5 个 dialog 型浮层/);
  assert.match(specification, /共 \*\*64 个\*\*/);
});

async function collectTsxFiles(directory) {
  const files = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...await collectTsxFiles(target));
    } else if (entry.isFile() && entry.name.endsWith(".tsx")) {
      files.push(target);
    }
  }
  return files;
}
