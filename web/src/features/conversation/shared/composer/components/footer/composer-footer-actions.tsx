// INPUT: Composer 附件/目录/Goal/Plan/WorkGraph 动作与 Connector 只读目录。
// OUTPUT: 主动作菜单与受控 Plan/Goal/Connector 勾选项；每行只有一个原生激活入口。
// POS: Composer Footer 动作入口；Connector 读取失败由外层可靠性面统一展示。
import type { RefObject } from "react";
import {
  FolderPlus,
  Loader2,
  Lightbulb,
  Paperclip,
  Plus,
  Target,
  GitBranchPlus,
} from "lucide-react";

import { ConnectorIcon } from "@/features/capability/connectors/connector-icon";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import type { ComposerSessionSettingsController } from "../../controller/use-composer-session-settings";
import type { ComposerLocalDirectoriesController } from "../../controller/use-composer-local-directories";

type ComposerActionValue = "attachment" | "directory" | "goal" | "workgraph" | "plan";

interface ComposerFooterActionsProps {
  actionButtonRef: RefObject<HTMLButtonElement | null>;
  canCreateGoal: boolean;
  canUsePlan: boolean;
  isPlanMode: boolean;
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
  onPlanToggle: () => void;
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
  canUsePlan,
  isPlanMode,
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
  onPlanToggle,
  onWorkGraphDistillationsSelect,
  onLocalDirectorySelect,
  sessionSettingsController,
  sessionSettingsDisabled,
}: ComposerFooterActionsProps) {
  const { t } = useI18n();
  const items = buildActionItems({
    canCreateGoal,
    canUsePlan,
    isPlanMode,
    canUseLocalDirectories: localDirectoriesController.available,
    canUseWorkGraphDistillations,
    isGoalCreating,
    isGoalMode,
    isLocalDirectoryBusy:
      sessionSettingsDisabled
      || localDirectoriesController.loading
      || localDirectoriesController.saving
      || Boolean(localDirectoriesController.failure?.blocksMutation),
    isPreparingAttachments,
    labels: {
      attachment: t("composer.add_attachment"),
      directory: t("composer.add_local_directory"),
      goal: t("composer.start_goal"),
      plan: t("composer.plan_mode"),
      workgraph: t("composer.open_workgraph_distillations"),
    },
  });
  const commands = new Map<string, () => void>([
    ["attachment", onAttachmentSelect],
    ["directory", onLocalDirectorySelect],
    ["workgraph", onWorkGraphDistillationsSelect],
    ["goal", () => onGoalToggle(!isGoalMode)],
    ["plan", onPlanToggle],
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
      loading: t("composer.connectors_loading"),
    },
  });

  return (
    <div className="shrink-0">
      <UiIconButton
        ref={actionButtonRef}
        aria-expanded={isActionMenuOpen}
        aria-haspopup="menu"
        aria-label={t("composer.open_actions")}
        onClick={onActionMenuToggle}
        size="md"
        tooltip={t("composer.open_actions")}
        variant="ghost"
      >
        <Plus className="h-4 w-4" />
      </UiIconButton>
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
  labels: Pick<Record<"enable" | "enabled" | "loading", string>, "loading">;
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
      checked: active,
      value: `connector:${connector.connector_id}`,
    };
  });
}

function buildActionItems({
  canCreateGoal,
  canUsePlan,
  isPlanMode,
  canUseLocalDirectories,
  canUseWorkGraphDistillations,
  isGoalCreating,
  isGoalMode,
  isLocalDirectoryBusy,
  isPreparingAttachments,
  labels,
}: {
  canCreateGoal: boolean;
  canUsePlan: boolean;
  isPlanMode: boolean;
  canUseLocalDirectories: boolean;
  canUseWorkGraphDistillations: boolean;
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
        icon: <GitBranchPlus className="h-4 w-4 text-(--icon-muted)" />,
        label: labels.workgraph,
        value: "workgraph",
      },
      visible: canUseWorkGraphDistillations,
    },
    {
      item: {
        active: isGoalMode,
        disabled: !canCreateGoal || isGoalCreating || isPlanMode,
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
        checked: isGoalMode,
        value: "goal",
      },
      visible: true,
    },
    {
      item: {
        active: isPlanMode,
        checked: isPlanMode,
        disabled: isGoalMode || isGoalCreating || isPreparingAttachments,
        icon: <Lightbulb className="h-4 w-4 text-(--icon-muted)" />,
        label: labels.plan,
        value: "plan",
      },
      visible: canUsePlan,
    },
  ];
  return candidates
    .filter((candidate) => candidate.visible)
    .map((candidate) => candidate.item);
}
