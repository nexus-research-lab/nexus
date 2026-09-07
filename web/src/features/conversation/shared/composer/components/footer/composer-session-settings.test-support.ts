// INPUT: 测试覆盖的 Session 设置控制器字段。
// OUTPUT: 完整、可覆盖的控制器夹具和可检查的命令 spy。
// POS: Footer 控件与恢复面测试共享夹具，不进入产品运行时。

import { vi } from "vitest";
import type { ComposerSessionSettingsController } from "../../controller/use-composer-session-settings";

export function makeController(overrides: Partial<ComposerSessionSettingsController> = {}): ComposerSessionSettingsController {
  const target = { agentId: "nova", name: "Nova", sessionKey: "session-a" };
  return {
    busy: false, connectors: [], connectorsFailure: null, connectorsLoading: false, enabledConnectorIds: [],
    ensureTargetsLoaded: vi.fn(async () => undefined),
    hasModelOverride: true, hasPermissionOverride: true,
    inheritedModel: "base", inheritedPermissionMode: "default", inheritedProvider: "provider",
    isDangerousPermission: false, modelBusy: false, modelLabel: "Advanced", permissionLabel: "Default",
    providerOptions: {
      default_provider: null, default_model: null, default_selection: null,
      default_image_provider: null, default_image_model: null, default_image_selection: null,
      background_items: [], image_items: [], vision_items: [],
      items: [
        { provider: "provider", display_name: "Provider", models: [
          { model_id: "base", display_name: "Base", is_default: true },
          { model_id: "advanced", display_name: "Advanced", is_default: false },
        ] },
        { provider: "other", display_name: "Other provider with a long name", models: [
          { model_id: "base", display_name: "Base with a long name", is_default: true },
        ] },
      ],
    },
    providerOptionsLoading: false, providerFailure: null,
    resetTarget: vi.fn(), saving: false, selectTarget: vi.fn(),
    scope: { initialTargetId: target.agentId, runtimeKind: "nxs", targets: [target] },
    settings: { provider: "provider", model: "advanced", permission_mode: "plan", connector_ids: null },
    settingsLoading: false, settingsReadFailure: null, mutationFailure: null,
    target, targetViews: [{ target, busy: false, modelLabel: "Advanced" }],
    retryConnectors: vi.fn(), retryProviderOptions: vi.fn(), retrySessionSettings: vi.fn(async () => undefined),
    resetModel: vi.fn(async () => undefined), updateModel: vi.fn(async () => undefined),
    resetPermission: vi.fn(async () => undefined), updatePermission: vi.fn(async () => undefined),
    toggleConnector: vi.fn(async () => undefined),
    ...overrides,
  };
}
