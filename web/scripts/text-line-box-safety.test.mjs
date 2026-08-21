import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));
const webSourceRoot = path.join(repositoryRoot, "web/src");

test("single-line clipping never uses a descender-tight line box", () => {
  const violations = sourceFiles(webSourceRoot).flatMap((filePath) => (
    fs.readFileSync(filePath, "utf8")
      .split("\n")
      .flatMap((line, index) => (
        line.includes("leading-none")
        && (line.includes("truncate") || line.includes("overflow-hidden"))
          ? [`${path.relative(repositoryRoot, filePath)}:${index + 1}`]
          : []
      ))
  ));

  assert.deepEqual(violations, []);
});

test("known nested single-line surfaces keep descender-safe inherited leading", () => {
  const mentionChip = read("web/src/features/conversation/shared/message/agent-mention-chip.tsx");
  const slashToken = read("web/src/features/conversation/shared/slash-command-token.tsx");
  const assistantStats = read("web/src/features/conversation/shared/message/item/view/assistant/assistant-message-stats.tsx");
  const roomSwitcher = read("web/src/features/conversation/room/surface/room-agent-switcher.tsx");
  const taskStrip = read("web/src/shared/ui/workspace/surface/workspace-task-strip.tsx");

  assert.match(mentionChip, /text-\[0\.9em\] font-medium leading-tight/);
  assert.match(slashToken, /text-\[0\.9em\] font-medium leading-tight/);
  assert.match(assistantStats, /gap-1\.5 leading-tight text-/);
  assert.match(assistantStats, /overflow-hidden whitespace-nowrap leading-tight/);
  assert.match(roomSwitcher, /truncate text-left text-compact font-semibold leading-normal/);
  assert.match(taskStrip, /truncate text-xs font-medium leading-normal/);
});

function read(relativePath) {
  return fs.readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

function sourceFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      return sourceFiles(entryPath);
    }
    return /\.(?:ts|tsx)$/.test(entry.name) ? [entryPath] : [];
  });
}
