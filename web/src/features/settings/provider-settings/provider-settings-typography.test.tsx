// INPUT: Provider 标识、详情头、能力行和空模型目录。
// OUTPUT: 证明 Provider 使用共享 Typography、Badge 与 Shape 所有者。
// POS: Provider 视图合同测试；数据刷新和 mutation 由 controller/action 测试负责。

import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { CapabilitySwitch } from "./components/provider-settings-capability-switch";
import { ProviderSettingsDetailHeader } from "./components/provider-settings-detail-header";
import { ProviderIcon } from "./components/provider-settings-icon";
import { ProviderSettingsModelList } from "./components/provider-settings-model-list";

function renderWithI18n(children: ReactNode) {
  return render(
    <I18N_CONTEXT.Provider
      value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key) => key,
      }}
    >
      {children}
    </I18N_CONTEXT.Provider>,
  );
}

describe("Provider settings typography", () => {
  it("uses semantic identity, status, and graphical-initial roles", () => {
    renderWithI18n(
      <>
        <ProviderIcon name="Custom Provider" size="md" />
        <ProviderSettingsDetailHeader
          detailTitle="Custom Provider"
          enabled
          hasSelectedRecord
          isApiFormatConfigurable
          isEditing={false}
          onEnabledChange={vi.fn()}
          onTestSelection={vi.fn()}
          pendingAction={null}
          selectedCanManage
          testModelOptions={[]}
        />
      </>,
    );

    expect(screen.getByRole("heading", { name: "Custom Provider" }).className).toContain("ui-type-page-title");
    expect(screen.getByText("settings.providers.status_active").className).toContain("rounded-full");
    expect(screen.getByText("settings.providers.status_active").className).toContain("var(--success)");
    expect(screen.getByText("CP").className).toContain("ui-type-control");
    expect(screen.getByText("CP").className).toContain("radius-control-md");
  });

  it("keeps model headings, empty state, and capability labels on semantic roles", () => {
    renderWithI18n(
      <>
        <CapabilitySwitch checked label="Vision" onChange={vi.fn()} />
        <ProviderSettingsModelList
          displayedModels={[]}
          hasModelsEndpoint={false}
          isApiFormatConfigurable
          isEditing={false}
          modelQuery=""
          onDefaultModelDisableAttempt={vi.fn()}
          onFetchModels={vi.fn()}
          onModelOptions={vi.fn()}
          onModelQueryChange={vi.fn()}
          onOpenAddModel={vi.fn()}
          onRequestDeleteModel={vi.fn()}
          onSetDefaultModel={vi.fn()}
          onToggleModel={vi.fn()}
          pendingAction={null}
          selectedCanManage
          selectedRecord={null}
        />
      </>,
    );

    expect(screen.getByText("Vision").className).toContain("ui-type-control");
    expect(screen.getByRole("heading", { name: "settings.providers.models" }).className).toContain("ui-type-section-title");
    expect(screen.getByText("settings.providers.models_after_save").className).toContain("ui-type-supporting");
  });
});
