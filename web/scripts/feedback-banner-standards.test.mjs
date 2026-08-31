import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("feedback persistence follows severity and actionability", async () => {
  const { projectFeedbackBanner } = await server.ssrLoadModule(
    "/src/shared/ui/feedback/feedback-banner-model.ts",
  );

  assert.equal(projectFeedbackBanner("error", false).autoDismissMs, null);
  assert.equal(projectFeedbackBanner("warning", false).autoDismissMs, null);
  assert.equal(projectFeedbackBanner("success", false).autoDismissMs, 4000);
  assert.equal(projectFeedbackBanner("info", false).autoDismissMs, 5000);
  assert.equal(projectFeedbackBanner("success", true).autoDismissMs, null);
  assert.equal(projectFeedbackBanner("info", true).autoDismissMs, null);
});

test("error and warning feedback require recovery facts without audit labels", async () => {
  const banner = await read("src/shared/ui/feedback/feedback-banner.tsx");
  const contract = await read(
    "src/shared/ui/feedback/feedback-banner-contract.ts",
  );
  const resourceState = await read("src/shared/ui/display/resource-state.tsx");

  assert.match(
    contract,
    /interface FeedbackBannerRecoveryProps[\s\S]*impact: string;[\s\S]*nextStep: string;[\s\S]*tone: "error" \| "warning";/,
  );
  assert.doesNotMatch(banner, /state\.existing_data|state\.next_step|<dl|<dt/);
  assert.doesNotMatch(resourceState, /state\.existing_data|state\.next_step|<dl|<dt/);
  assert.match(resourceState, /data-resource-state-impact/);
  assert.match(resourceState, /data-resource-state-next-step/);
});

test("urgency, recovery color, and mobile layout are independent from error tone", async () => {
  const banner = await read("src/shared/ui/feedback/feedback-banner.tsx");
  const viewport = await read(
    "src/shared/ui/feedback/feedback-banner-viewport.tsx",
  );
  const resourceState = await read("src/shared/ui/display/resource-state.tsx");

  assert.match(banner, /urgency = "polite"/);
  assert.match(banner, /urgency === "assertive" \? "alert" : "status"/);
  assert.doesNotMatch(banner, /tone === "error" \? "alert"/);
  assert.match(
    banner,
    /tone=\{action\.tone === "danger" \? "danger" : "primary"\}/,
  );
  assert.doesNotMatch(banner, /tone === "error" \? "danger"/);
  assert.match(viewport, /left-3 right-3/);
  assert.match(viewport, /sm:left-auto/);
  assert.match(banner, /break-words/);
  assert.match(banner, /max-h-\[calc\(100dvh-/);
  assert.match(banner, /overflow-y-auto/);
  assert.match(banner, /motion-reduce:transition-none/);
  assert.match(resourceState, /sm:flex-row/);
  assert.match(resourceState, /motion-reduce:animate-none/);
  assert.match(resourceState, /urgency = "polite"/);
  assert.doesNotMatch(resourceState, /failure \? "alert"/);
});

test("auto-dismiss follows content identity without callback churn", async () => {
  const banner = await read("src/shared/ui/feedback/feedback-banner.tsx");

  assert.match(banner, /const onDismissRef = useRef\(onDismiss\)/);
  assert.match(
    banner,
    /\[canAutoDismiss, impact, message, nextStep, presentation\.autoDismissMs, title\]/,
  );
  assert.match(
    banner,
    /canAutoDismiss = Boolean\(onDismiss\)[\s\S]*&& !impact[\s\S]*&& !nextStep/,
  );
  assert.doesNotMatch(
    banner,
    /\[message, onDismiss, presentation\.autoDismissMs, title\]/,
  );
});

test("every global feedback caller supplies the strict recovery contract", async () => {
  const files = await findSourceFiles(path.join(webRoot, "src"));
  const callers = [];
  for (const file of files) {
    const source = await readFile(file, "utf8");
    if (
      source.includes("<FeedbackBannerViewport")
      && !file.endsWith("feedback-banner-viewport.tsx")
    ) {
      callers.push({ file, source });
    }
  }

  assert.ok(callers.length > 0);
  for (const caller of callers) {
    const usesAdapter = caller.source.includes("completeFeedbackBanner(");
    const suppliesFacts = caller.source.includes("impact:")
      && caller.source.includes("nextStep:");
    const passesStrictControllerFeedback = caller.source.includes(
      "item={controller.feedback}",
    ) || caller.source.includes("...controller.feedback.value")
      || caller.source.includes("item={controller.state.feedback ?? feedback ?? null}");
    assert.ok(
      usesAdapter || suppliesFacts || passesStrictControllerFeedback,
      `${path.relative(webRoot, caller.file)} must supply impact and nextStep`,
    );
  }
});

test("recovery fallback copy stays aligned in both locales", async () => {
  const zh = await read("src/shared/i18n/catalog/zh/core.ts");
  const en = await read("src/shared/i18n/catalog/en/core.ts");
  for (const key of [
    "feedback.unconfirmed_impact",
    "feedback.unconfirmed_next_step",
    "feedback.processing_impact",
    "feedback.processing_next_step",
    "feedback.partial_impact",
    "feedback.partial_next_step",
  ]) {
    assert.ok(zh.includes(`"${key}"`), `zh catalog is missing ${key}`);
    assert.ok(en.includes(`"${key}"`), `en catalog is missing ${key}`);
  }
});

async function findSourceFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(entries.map(async (entry) => {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) return findSourceFiles(target);
    return entry.isFile() && entry.name.endsWith(".tsx") ? [target] : [];
  }));
  return files.flat();
}

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
