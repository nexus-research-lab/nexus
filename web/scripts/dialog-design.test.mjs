import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

test("shared decision dialogs do not manufacture warning prose or decorative chrome", async () => {
  const [dialogSource, frameSource, modelSource, uiContract] = await Promise.all([
    read("web/src/shared/ui/dialog/decision/decision-dialog.tsx"),
    read("web/src/shared/ui/dialog/decision/decision-dialog-frame.tsx"),
    read("web/src/shared/ui/dialog/decision/decision-dialog-model.ts"),
    read("web/src/shared/ui/CLAUDE.md"),
  ]);

  assert.doesNotMatch(dialogSource, /AlertTriangle|DIALOG_HEADER_ICON_CLASS_NAME|getDialogNote/);
  assert.doesNotMatch(dialogSource, /<UiDialogHeader/);
  assert.match(dialogSource, /className="absolute right-3 top-3 z-10"/);
  assert.match(dialogSource, /whitespace-pre-wrap text-sm leading-6/);
  assert.doesNotMatch(modelSource, /请确认是否继续|不可恢复|subtitle|noteTone|iconStyle/);
  assert.match(frameSource, /!border-t-0 !bg-transparent/);
  assert.match(uiContract, /不得自动补风险套话、装饰警告图标或消息卡片/);
});

function read(relativePath) {
  return readFile(path.join(repositoryRoot, relativePath), "utf8");
}
