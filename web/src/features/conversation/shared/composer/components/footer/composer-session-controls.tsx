"use client";

/**
 * INPUT: 当前 Composer 可配置的 Session 目标、Agent 默认值与 runtime 类型。
 * OUTPUT: DM 直接配置当前 Session 模型/思考强度与权限；Room 统一权限并按 Agent 配置模型与强度。
 * POS: Composer Footer 会话设置入口；临时菜单按精确 Session 重置，模型宽度/选择与 Room 共用 owner，不写回 Agent 默认配置。
 */

import {
  ChevronDown,
  ShieldCheck,
} from "lucide-react";
import {
  useEffect,
  useRef,
} from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiButton } from "@/shared/ui/button/button";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import type {
  ComposerSessionSettingsController,
} from "../../controller/use-composer-session-settings";
import { ComposerModelPicker } from "./composer-model-picker";
import { ComposerRoomModelControl } from "./composer-room-model-control";
import {
  buildResetSessionSettingItem,
  buildSessionPermissionItems,
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
  const roomWide = scope.targets.length > 1;
  if (slot === "leading") {
    return (
      <div className="flex min-w-0 items-center gap-1">
        <ComposerPermissionControl
          controller={controller}
          disabled={disabled}
          roomWide={roomWide}
        />
      </div>
    );
  }
  if (roomWide) {
    return <ComposerRoomModelControl controller={controller} disabled={disabled} />;
  }
  return <ComposerModelPicker controller={controller} disabled={disabled} />;
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
  const [isOpen, setIsOpen] = useResettableState(false, controller.target?.sessionKey);
  useEffect(() => {
    if (disabled || controller.busy) {
      setIsOpen(false);
    }
  }, [controller.busy, disabled, setIsOpen]);
  const permissionItems = buildSessionPermissionItems(controller, t);
  const resetItem = buildResetSessionSettingItem(
    !controller.hasPermissionOverride,
    t,
  );
  return (
    <div className="min-w-0">
      <UiButton
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("composer.session_permission")}
        className="min-w-0"
        disabled={disabled || controller.busy}
        onClick={() => setIsOpen((current) => !current)}
        size="xs"
        tone={controller.isDangerousPermission ? "danger" : "default"}
        variant="text"
      >
        <ShieldCheck className="h-3.5 w-3.5 shrink-0" />
        <span className="nexus-chat-composer-session-permission-label max-w-24 truncate">
          {controller.permissionLabel}
        </span>
        <ChevronDown className="h-3 w-3 shrink-0" />
      </UiButton>
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

const SESSION_PERMISSION_MENU_WIDTH = 288;
