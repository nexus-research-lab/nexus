import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const repositoryRoot = path.dirname(webRoot);
const reviewRoot = path.join(repositoryRoot, "docs/reviews/failure-recovery-visuals");
const stories = [
  "feedback-not-applied",
  "feedback-accepted",
  "feedback-committed-refresh",
  "feedback-outcome-unknown",
  "resource-load-failed",
  "resource-stale-snapshot",
  "conversation-delivery-unknown",
  "editor-conflict",
  "editor-outcome-unknown",
  "provider-persist-unknown",
  "destructive-outcome-unknown",
];

test("failure visual review uses production surfaces without joining the app build", async () => {
  const [gallery, capture, viteConfig] = await Promise.all([
    readFile(path.join(webRoot, "visual-review/failure-recovery/main.tsx"), "utf8"),
    readFile(path.join(webRoot, "scripts/capture-failure-recovery-visuals.py"), "utf8"),
    readFile(path.join(webRoot, "vite.config.ts"), "utf8"),
  ]);

  for (const component of [
    "ConversationReliabilityNotice",
    "ReadResourceReliabilityNotice",
    "TextFileEditorReliability",
    "UiResourceState",
    "ConfirmDialog",
    "FeedbackBanner",
  ]) {
    assert.match(gallery, new RegExp(component));
  }
  assert.doesNotMatch(gallery, /requestApi|fetch\(|\/nexus\/v1/);
  assert.doesNotMatch(viteConfig, /visual-review\/failure-recovery/);

  for (const story of stories) {
    assert.ok(gallery.includes(`"${story}"`), `gallery missing ${story}`);
    assert.ok(capture.includes(`"${story}"`), `capture script missing ${story}`);
  }
});

test("desktop and mobile review images cover every declared failure style", async () => {
  for (const viewport of ["desktop", "mobile"]) {
    for (const story of stories) {
      const image = await stat(path.join(reviewRoot, `${viewport}--${story}.png`));
      assert.ok(image.size > 10_000, `${viewport} ${story} screenshot is unexpectedly small`);
    }
  }
  for (const viewport of ["desktop", "mobile"]) {
    const contactSheet = await stat(path.join(reviewRoot, `contact-sheet--${viewport}.png`));
    assert.ok(contactSheet.size > 50_000, `${viewport} contact sheet is unexpectedly small`);
  }
});
