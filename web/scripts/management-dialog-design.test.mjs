import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

test("Skill management dialogs keep instructions and implementation detail out of primary chrome", async () => {
  const [importDialog, importGuide, sourceManager, previewDialog, previewModel, capabilityZh] = await Promise.all([
    read("web/src/features/capability/skills/import/skill-import-dialog.tsx"),
    read("web/src/features/capability/skills/import/skill-import-guide.tsx"),
    read("web/src/features/capability/skills/external/skill-source-manager-dialog.tsx"),
    read("web/src/features/capability/skills/external/external-skill-preview-dialog.tsx"),
    read("web/src/features/capability/skills/external/external-skill-model.ts"),
    read("web/src/shared/i18n/catalog/zh/capability.ts"),
  ]);

  for (const source of [importDialog, sourceManager, previewDialog]) {
    assert.match(source, /<UiDialogHeader[\s\S]*appearance="plain"/);
    assert.doesNotMatch(source, /<UiDialogHeader[\s\S]*subtitle=/);
  }
  assert.match(importGuide, /<details className="group">/);
  assert.match(sourceManager, /<ConfirmDialog/);
  assert.doesNotMatch(sourceManager, /window\.confirm|sourceKindDescription|skill_source_description/);
  assert.doesNotMatch(previewDialog, /UiSeededAvatar|model\.subtitle/);
  assert.doesNotMatch(previewModel, /ExternalSkillPreviewModel \{[\s\S]*subtitle:/);
  assert.doesNotMatch(capabilityZh, /skills_import_subtitle|skill_sources_description|skill_source_description\./);
});

test("Scheduled dialogs use task facts instead of subtitles, icons, and internal header IDs", async () => {
  const [taskDialog, attentionDialog, historyDialog, historyContent, capabilityZh] = await Promise.all([
    read("web/src/features/capability/scheduled/dialog/scheduled-task-dialog.tsx"),
    read("web/src/features/capability/scheduled/board/scheduled-task-attention-dialog.tsx"),
    read("web/src/features/capability/scheduled/history/scheduled-task-run-history-dialog.tsx"),
    read("web/src/features/capability/scheduled/history/view/scheduled-task-run-history-content.tsx"),
    read("web/src/shared/i18n/catalog/zh/capability.ts"),
  ]);

  for (const source of [taskDialog, attentionDialog, historyDialog]) {
    assert.match(source, /<UiDialogHeader[\s\S]*appearance="plain"/);
    assert.doesNotMatch(source, /<UiDialogHeader[\s\S]*subtitle=/);
  }
  assert.doesNotMatch(attentionDialog, /CircleAlert|Link2Off|ShieldAlert|原任务的名称|>\s*当前状态\s*</);
  assert.doesNotMatch(historyDialog, /Job ID:|\{activeTask\.name\} 运行历史/);
  assert.match(historyContent, /任务运行后，记录会显示在这里。/);
  assert.doesNotMatch(capabilityZh, /scheduled_dialog_(new|edit)_subtitle/);
});

function read(relativePath) {
  return readFile(path.join(repositoryRoot, relativePath), "utf8");
}
