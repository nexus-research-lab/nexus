import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("General settings owns the desktop app version and log export surface", async () => {
  const [sectionSource, desktopSource, controllerSource] = await Promise.all([
    readFile(
      path.join(
        webRoot,
        "src/features/settings/general/settings-general-section.tsx",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "src/features/settings/general/sections/settings-desktop-section.tsx",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "src/features/settings/general/use-desktop-settings.ts",
      ),
      "utf8",
    ),
  ]);

  assert.match(
    sectionSource,
    /section === "general"[\s\S]*<SettingsDesktopSection \/>[\s\S]*<SettingsGeneralBehaviorContent \/>/,
  );
  assert.doesNotMatch(sectionSource, /SettingsSystemSection/);
  assert.match(
    sectionSource,
    /section === "workspace" \? <SettingsWorkspaceSection \/> : null/,
  );
  assert.match(desktopSource, /controller\.versionDescription/);
  assert.match(desktopSource, /controller\.exportLogs/);
  assert.match(controllerSource, /getDesktopAppVersion\(\)/);
  assert.match(controllerSource, /exportDesktopLogs\(\)/);
});
