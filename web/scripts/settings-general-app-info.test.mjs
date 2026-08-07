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

test("Workspace settings chooses a native folder before confirming migration", async () => {
  const [
    sectionSource,
    controllerSource,
    bridgeSource,
    macOSSource,
    windowsSource,
    macOSBridgeScript,
    windowsBridgeScript,
  ] = await Promise.all([
    readFile(
      path.join(
        webRoot,
        "src/features/settings/general/sections/settings-workspace-section.tsx",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "src/features/settings/general/use-workspace-settings.ts",
      ),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/lib/desktop-bridge/desktop-bridge.ts"),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "../desktop/macos/Sources/NexusDesktop/Bridge/DesktopBridgeHandler.swift",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "../desktop/windows/Nexus.Desktop/Bridge/DesktopBridgeHandler.cs",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "../desktop/macos/Sources/NexusDesktop/Bridge/DesktopBridgeScript.swift",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "../desktop/windows/Nexus.Desktop/Bridge/DesktopBridgeScript.cs",
      ),
      "utf8",
    ),
  ]);

  assert.match(sectionSource, /controller\.selectDirectory\(\)/);
  assert.match(controllerSource, /chooseDesktopStateRoot\(/);
  assert.match(
    controllerSource,
    /if \(!result\.cancelled && selectedPath\)[\s\S]*replaceWorkspaceDraft\(current, selectedPath\)/,
  );
  assert.match(bridgeSource, /"app\.choose_state_root"/);
  assert.match(macOSSource, /case "app\.choose_state_root"/);
  assert.match(macOSSource, /panel\.canChooseFiles = false/);
  assert.match(macOSSource, /panel\.canChooseDirectories = true/);
  assert.match(windowsSource, /"app\.choose_state_root" => ChooseStateRoot\(payload\)/);
  assert.match(windowsSource, /OpenFolderDialog dialog = new\(\)/);
  assert.match(macOSBridgeScript, /request\.kind !== "app\.choose_state_root"/);
  assert.match(windowsBridgeScript, /request\.kind !== "app\.choose_state_root"/);
});

test("Emotion system is opt-in and persists through general preferences", async () => {
  const [typesSource, runtimeOptionsSource, modelSource, controllerSource, sectionSource] =
    await Promise.all([
      "src/types/settings/preferences.ts",
      "src/config/runtime-options.ts",
      "src/features/settings/general/model/settings-preferences-model.ts",
      "src/features/settings/general/use-general-settings-controller.ts",
      "src/features/settings/general/sections/settings-general-behavior-section.tsx",
    ].map((file) => readFile(path.join(webRoot, file), "utf8")));

  assert.match(typesSource, /emotion_enabled\?: boolean/);
  assert.match(runtimeOptionsSource, /DEFAULT_EMOTION_ENABLED = false/);
  assert.match(modelSource, /emotion_enabled: preferences\.emotion_enabled/);
  assert.match(controllerSource, /emotion_enabled: checked/);
  assert.match(sectionSource, /checked=\{emotionEnabled\}/);
  assert.match(sectionSource, /onChange=\{onEmotionEnabledChange\}/);
});
