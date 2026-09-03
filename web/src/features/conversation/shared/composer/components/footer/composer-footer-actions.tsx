// INPUT: Composer 附件/目录/Goal/Loop/WorkGraph 动作与 Connector 只读目录。
// OUTPUT: 主动作菜单及已加载的 Session Connector 开关项。
// POS: Composer Footer 动作入口；Connector 读取失败由外层可靠性面统一展示。
import type { ReactNode, RefObject } from "react";
import {
  Check,
  FolderPlus,
  Loader2,
  Paperclip,
  Plus,
  Repeat2,
  Target,
  GitBranchPlus,
} from "lucide-react";

import { ConnectorIcon } from "@/features/capability/connectors/connector-icon";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import type { ComposerSessionSettingsController } from "../../controller/use-composer-session-settings";
import type { ComposerLocalDirectoriesController } from "../../controller/use-composer-local-directories";

type ComposerActionValue = "attachment" | "directory" | "goal" | "loop" | "workgraph";

interface ComposerFooterActionsProps {
  actionButtonRef: RefObject<HTMLButtonElement | null>;
  canCreateGoal: boolean;
  canUseLoop: boolean;
  canUseWorkGraphDistillations: boolean;
  isActionMenuOpen: boolean;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isPreparingAttachments: boolean;
  localDirectoriesController: ComposerLocalDirectoriesController;
  onActionMenuClose: () => void;
  onActionMenuToggle: () => void;
  onAttachmentSelect: () => void;
  onGoalToggle: (checked: boolean) => void;
  onLoopSelect: () => void;
  onWorkGraphDistillationsSelect: () => void;
  onLocalDirectorySelect: () => void;
  sessionSettingsController: ComposerSessionSettingsController;
  sessionSettingsDisabled: boolean;
}

interface VisibleActionItem {
  item: UiActionMenuItem;
  visible: boolean;
}

export function ComposerFooterActions({
  actionButtonRef,
  canCreateGoal,
  canUseLoop,
  canUseWorkGraphDistillations,
  isActionMenuOpen,
  isGoalCreating,
  isGoalMode,
  isPreparingAttachments,
  localDirectoriesController,
  onActionMenuClose,
  onActionMenuToggle,
  onAttachmentSelect,
  onGoalToggle,
  onLoopSelect,
  onWorkGraphDistillationsSelect,
  onLocalDirectorySelect,
  sessionSettingsController,
  sessionSettingsDisabled,
}: ComposerFooterActionsProps) {
  const { t } = useI18n();
  const items = buildActionItems({
    canCreateGoal,
    canUseLocalDirectories: localDirectoriesController.available,
    canUseLoop,
    canUseWorkGraphDistillations,
    goalSwitch: (
      <span
        onClick={(event) => event.stopPropagation()}
        onKeyDown={(event) => event.stopPropagation()}
        role="presentation"
      >
        <GlassSwitch
          aria-label={t("composer.start_goal")}
          checked={isGoalMode}
          disabled={!canCreateGoal || isGoalCreating}
          onChange={onGoalToggle}
          size="xs"
        />
      </span>
    ),
    isGoalCreating,
    isGoalMode,
    isLocalDirectoryBusy:
      sessionSettingsDisabled
      || localDirectoriesController.loading
      || localDirectoriesController.saving,
    isPreparingAttachments,
    labels: {
      attachment: t("composer.add_attachment"),
      directory: t("composer.add_local_directory"),
      goal: t("composer.start_goal"),
      loop: t("composer.insert_loop"),
      workgraph: t("composer.open_workgraph_distillations"),
    },
  });
  const commands = new Map<string, () => void>([
    ["attachment", onAttachmentSelect],
    ["directory", onLocalDirectorySelect],
    ["loop", onLoopSelect],
    ["workgraph", onWorkGraphDistillationsSelect],
    ["goal", () => onGoalToggle(!isGoalMode)],
  ]);
  for (const connector of sessionSettingsController.connectors) {
    commands.set(`connector:${connector.connector_id}`, () => {
      void sessionSettingsController.toggleConnector(connector.connector_id);
    });
  }
  const connectorItems = buildConnectorItems({
    controller: sessionSettingsController,
    disabled: sessionSettingsDisabled,
    labels: {
      enable: t("composer.connector_enable"),
      enabled: t("composer.connector_enabled"),
      loading: t("composer.connectors_loading"),
    },
  });

  return (
    <div className="shrink-0">
      <button
        ref={actionButtonRef}
        aria-expanded={isActionMenuOpen}
        aria-haspopup="menu"
        aria-label={t("composer.open_actions")}
        className="inline-flex h-8 w-8 items-center justify-center rounded-[8px] text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
        onClick={onActionMenuToggle}
        type="button"
      >
        <Plus className="h-4 w-4" />
      </button>
      <UiActionMenu
        anchorRef={actionButtonRef}
        ariaLabel={t("composer.open_actions")}
        density="compact"
        isOpen={isActionMenuOpen}
        footerItems={connectorItems}
        items={items}
        onClose={onActionMenuClose}
        onSelect={(value) => commands.get(value)?.()}
        placement="top"
      />
    </div>
  );
}

function buildConnectorItems({
  controller,
  disabled,
  labels,
}: {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  labels: Record<"enable" | "enabled" | "loading", string>;
}): UiActionMenuItem[] {
  if (controller.connectorsLoading && controller.connectors.length === 0) {
    return [{
      disabled: true,
      icon: (
        <Loader2
          className={getUiSpinnerClassName({ size: "md", tone: "muted" })}
        />
      ),
      label: labels.loading,
      value: "connectors:loading",
    }];
  }
  return controller.connectors.map((connector) => {
    const active = controller.enabledConnectorIds.includes(
      connector.connector_id,
    );
    return {
      active,
      description: active ? labels.enabled : labels.enable,
      disabled: disabled
        || controller.busy
        || controller.connectorsLoading
        || Boolean(controller.connectorsFailure),
      icon: (
        <ConnectorIcon
          icon={connector.icon}
          size="sm"
          title={connector.title}
        />
      ),
      label: connector.title,
      trailing: active
        ? <Check className="h-3.5 w-3.5 text-(--text-strong)" />
        : undefined,
      value: `connector:${connector.connector_id}`,
    };
  });
}

function buildActionItems({
  canCreateGoal,
  canUseLocalDirectories,
  canUseLoop,
  canUseWorkGraphDistillations,
  goalSwitch,
  isGoalCreating,
  isGoalMode,
  isLocalDirectoryBusy,
  isPreparingAttachments,
  labels,
}: {
  canCreateGoal: boolean;
  canUseLocalDirectories: boolean;
  canUseLoop: boolean;
  canUseWorkGraphDistillations: boolean;
  goalSwitch: ReactNode;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isLocalDirectoryBusy: boolean;
  isPreparingAttachments: boolean;
  labels: Record<ComposerActionValue, string>;
}): UiActionMenuItem[] {
  const candidates: VisibleActionItem[] = [
    {
      item: {
        disabled: isGoalMode || isLocalDirectoryBusy,
        icon: <FolderPlus className="h-4 w-4 text-(--icon-muted)" />,
        label: labels.directory,
        value: "directory",
      },
      visible: canUseLocalDirectories,
    },
    {
      item: {
        disabled: isGoalMode || isPreparingAttachments,
        icon: <Paperclip className="h-4 w-4 text-(--icon-muted)" />,
        label: labels.attachment,
        value: "attachment",
      },
      visible: true,
    },
    {
      item: {
        icon: <Repeat2 className="h-4 w-4 text-(--icon-muted)" />,
        label: labels.loop,
        value: "loop",
      },
      visible: canUseLoop,
    },
    {
      item: {
        icon: <GitBranchPlus className="h-4 w-4 text-(--icon-muted)" />,
        label: labels.workgraph,
        value: "workgraph",
      },
      visible: canUseWorkGraphDistillations,
    },
    {
      item: {
        active: isGoalMode,
        disabled: !canCreateGoal || isGoalCreating,
        icon: (
          <Target
            className={
              isGoalMode
                ? "h-4 w-4 text-(--brand-action)"
                : "h-4 w-4 text-(--icon-muted)"
            }
          />
        ),
        label: labels.goal,
        tone: isGoalMode ? "primary" : "default",
        trailing: goalSwitch,
        value: "goal",
      },
      visible: true,
    },
  ];
  return candidates
    .filter((candidate) => candidate.visible)
    .map((candidate) => candidate.item);
}
