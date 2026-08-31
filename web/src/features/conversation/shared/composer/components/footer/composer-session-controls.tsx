"use client";

/**
 * INPUT: 当前 Composer 可配置的 Session 目标、Agent 默认值与 runtime 类型。
 * OUTPUT: DM 直接配置当前 Session；Room 统一权限并按 Agent 配置模型。
 * POS: Composer Footer 内的会话运行时设置入口，不写回 Agent 默认配置。
 */

import {
  ChevronDown,
  ShieldCheck,
} from "lucide-react";
import {
  useEffect,
  useRef,
  useState,
} from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import type {
  ComposerSessionSettingsController,
} from "../../controller/use-composer-session-settings";
import { ComposerRoomModelControl } from "./composer-room-model-control";
import {
  buildResetSessionSettingItem,
  buildSessionModelItems,
  buildSessionPermissionItems,
  decodeSessionModelValue,
  RESET_SESSION_SETTING_VALUE,
} from "./composer-session-control-options";

interface ComposerSessionControlsProps {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  slot: "leading" | "trailing";
}

export function ComposerSessionControls({
  controller,
  disabled,
  slot,
}: ComposerSessionControlsProps) {
  const { scope } = controller;
  if (!scope || !controller.target) {
    return null;
  }
  if (scope.targets.length > 1) {
    if (slot === "leading") {
      return (
        <div className="flex min-w-0 items-center gap-1">
          <ComposerPermissionControl
            controller={controller}
            disabled={disabled}
            roomWide
          />
        </div>
      );
    }
    return (
      <ComposerRoomModelControl
        controller={controller}
        disabled={disabled}
        triggerClassName={SESSION_CONTROL_BUTTON_CLASS_NAME}
      />
    );
  }
  if (slot === "leading") {
    return (
      <div className="flex min-w-0 items-center gap-1">
        <ComposerPermissionControl
          controller={controller}
          disabled={disabled}
        />
      </div>
    );
  }
  return (
    <ComposerModelControl
      controller={controller}
      disabled={disabled}
    />
  );
}

function ComposerPermissionControl({
  controller,
  disabled,
  roomWide = false,
}: {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  roomWide?: boolean;
}) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  useEffect(() => {
    if (disabled || controller.busy) {
      setIsOpen(false);
    }
  }, [controller.busy, disabled]);
  const permissionItems = buildSessionPermissionItems(controller, t);
  const resetItem = buildResetSessionSettingItem(
    !controller.hasPermissionOverride,
    t,
  );
  return (
    <div className="min-w-0">
      <button
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("composer.session_permission")}
        className={cn(
          SESSION_CONTROL_BUTTON_CLASS_NAME,
          controller.isDangerousPermission && "text-(--destructive)",
        )}
        disabled={disabled || controller.busy}
        onClick={() => setIsOpen((current) => !current)}
        title={t("composer.session_permission")}
        type="button"
      >
        <ShieldCheck className="h-3.5 w-3.5 shrink-0" />
        <span className="nexus-chat-composer-session-permission-label max-w-24 truncate">
          {controller.permissionLabel}
        </span>
        <ChevronDown className="h-3 w-3 shrink-0" />
      </button>
      <UiActionMenu
        anchorRef={buttonRef}
        ariaLabel={t("composer.session_permission")}
        footerItems={[resetItem]}
        isOpen={isOpen && !disabled && !controller.busy}
        items={permissionItems}
        minWidth={SESSION_PERMISSION_MENU_WIDTH}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => {
          if (value === RESET_SESSION_SETTING_VALUE) {
            void controller.resetPermission();
            return;
          }
          void controller.updatePermission(
            !roomWide && value === controller.inheritedPermissionMode
              ? ""
              : value,
          );
        }}
        placement="top"
      />
    </div>
  );
}

function ComposerModelControl({
  controller,
  disabled,
}: {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
}) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  useEffect(() => {
    if (disabled || controller.modelBusy) {
      setIsOpen(false);
    }
  }, [controller.modelBusy, disabled]);
  const modelItems = buildSessionModelItems(controller);
  const resetItem = buildResetSessionSettingItem(
    !controller.hasModelOverride,
    t,
  );
  return (
    <div className="min-w-0">
      <button
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("composer.session_model")}
        className={cn(
          SESSION_CONTROL_BUTTON_CLASS_NAME,
          "max-w-44 text-(--text-default)",
        )}
        disabled={disabled || controller.modelBusy}
        onClick={() => setIsOpen((current) => !current)}
        title={t("composer.session_model")}
        type="button"
      >
        <span className="truncate">{controller.modelLabel}</span>
        <ChevronDown className="h-3 w-3 shrink-0" />
      </button>
      <UiActionMenu
        align="end"
        anchorRef={buttonRef}
        ariaLabel={t("composer.session_model")}
        density="compact"
        footerItems={[resetItem]}
        isOpen={isOpen && !disabled && !controller.modelBusy}
        items={modelItems}
        minWidth={SESSION_MODEL_MENU_WIDTH}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => {
          if (value === RESET_SESSION_SETTING_VALUE) {
            void controller.resetModel();
            return;
          }
          const [provider, model] = decodeSessionModelValue(value);
          if (
            provider === controller.inheritedProvider
            && model === controller.inheritedModel
          ) {
            void controller.resetModel();
            return;
          }
          void controller.updateModel(provider, model);
        }}
        placement="top"
      />
    </div>
  );
}

const SESSION_PERMISSION_MENU_WIDTH = 288;
const SESSION_MODEL_MENU_WIDTH = 256;

const SESSION_CONTROL_BUTTON_CLASS_NAME = cn(
  "inline-flex h-7 min-w-0 items-center gap-1.5 rounded-[8px] px-1.5",
  "text-xs font-medium text-(--text-soft) transition-colors",
  "hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  "disabled:pointer-events-none disabled:opacity-(--disabled-opacity)",
);
