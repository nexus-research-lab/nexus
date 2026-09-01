// INPUT: 已连接 MCP 的 tools/list 快照、读取状态与可用性。
// OUTPUT: 保留服务端原始工具标题、描述、参数与只读 annotation 的统一目录。
// POS: 固定 Connector 与自定义 MCP 共用的工具展示；不读取或展示 Prompts/Resources。
import { Code2, RotateCcw, Wrench } from "lucide-react";

import type { ResourceFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiPanel } from "@/shared/ui/panel";
import type {
  CustomMCPTool,
  CustomMCPToolCatalog,
} from "@/types/capability/connector";

interface MCPToolsSectionProps {
  available: boolean;
  unavailableMessage?: string;
  catalog: CustomMCPToolCatalog | null;
  description?: string;
  failure: ResourceFailure | null;
  loading: boolean;
  onRetry: () => void;
}

export function MCPToolsSection({
  available,
  unavailableMessage,
  catalog,
  description,
  failure,
  loading,
  onRetry,
}: MCPToolsSectionProps) {
  const { t } = useI18n();
  const toolCount = catalog?.tools.length ?? 0;
  return (
    <section className="py-5">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h2 className="text-base font-medium text-(--text-strong)">
            {t("capability.custom_mcp_tools")}
            {catalog?.inspection_state === "connected" ? (
              <span className="ml-2 text-xs font-normal tabular-nums text-(--text-soft)">
                {toolCount}
              </span>
            ) : null}
          </h2>
          <p className="mt-1 text-xs text-(--text-muted)">
            {description || t("capability.custom_mcp_tools_description")}
          </p>
        </div>
        {failure ? (
          <UiButton onClick={onRetry} size="sm" type="button" variant="text">
            <RotateCcw className="h-3.5 w-3.5" />
            {t("state.retry")}
          </UiButton>
        ) : null}
      </div>
      {!available ? (
        <MCPToolsMessage
          title={unavailableMessage || t("capability.custom_mcp_tools_disabled")}
        />
      ) : loading && !catalog ? (
        <UiResourceState
          className="mt-3 min-h-36"
          size="sm"
          state="loading"
          title={t("capability.custom_mcp_tools_loading")}
        />
      ) : failure ? (
        <UiResourceState
          className="mt-3 min-h-36"
          impact={t("capability.custom_mcp_tools_load_failed_impact")}
          size="sm"
          state="error"
          title={t("capability.custom_mcp_tools_load_failed")}
        />
      ) : catalog?.inspection_state === "runtime_only" ? (
        <MCPToolsMessage
          title={t("capability.custom_mcp_tools_runtime_only")}
        />
      ) : catalog && !catalog.supports_tools ? (
        <MCPToolsMessage
          title={t("capability.custom_mcp_tools_unsupported")}
        />
      ) : catalog && catalog.tools.length === 0 ? (
        <MCPToolsMessage
          title={t("capability.custom_mcp_tools_empty")}
        />
      ) : catalog ? (
        <UiPanel
          className="mt-3 divide-y divide-(--divider-subtle-color)"
          padding="none"
          radius="md"
          variant="inset"
        >
          {catalog.tools.map((tool) => (
            <MCPToolRow key={tool.name} tool={tool} />
          ))}
        </UiPanel>
      ) : null}
    </section>
  );
}

function MCPToolsMessage({ title }: { title: string }) {
  return (
    <div className="mt-3 flex min-h-28 items-center border-y border-(--divider-subtle-color) px-1 text-sm text-(--text-muted)">
      {title}
    </div>
  );
}

function MCPToolRow({ tool }: { tool: CustomMCPTool }) {
  const { t } = useI18n();
  return (
    <UiListRow
      className="min-h-[68px] rounded-none py-3"
      leading={(
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] border border-(--divider-subtle-color) bg-(--surface-panel-background)">
          <Wrench className="h-4 w-4 text-(--icon-default)" />
        </span>
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h3 className="font-medium text-(--text-strong)">{tool.title}</h3>
          {tool.title !== tool.name ? (
            <code className="text-2xs text-(--text-soft)">{tool.name}</code>
          ) : null}
          {tool.read_only ? (
            <UiBadge>{t("capability.custom_mcp_tool_read_only")}</UiBadge>
          ) : null}
        </div>
        {tool.description ? (
          <p className="mt-1 text-sm leading-5 text-(--text-muted)">
            {tool.description}
          </p>
        ) : null}
        {tool.arguments.length > 0 ? (
          <div className="mt-2 flex flex-wrap items-center gap-1.5">
            <Code2 className="h-3.5 w-3.5 text-(--icon-muted)" />
            {tool.arguments.map((argument) => (
              <code
                className="rounded-[5px] bg-(--surface-interactive-hover-background) px-1.5 py-0.5 text-2xs text-(--text-muted)"
                key={argument.name}
                title={argument.description}
              >
                {argument.name}{argument.required ? " *" : ""}
              </code>
            ))}
          </div>
        ) : null}
      </div>
    </UiListRow>
  );
}
