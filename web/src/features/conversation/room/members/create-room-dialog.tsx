/**
 * 创建 Room 弹窗
 *
 * 复用 dialog-shell 设计系统，与 AgentOptions / SkillDetailDialog 风格统一。
 * 使用 createPortal 渲染到 document.body，确保全页面居中显示。
 */

"use client";

import { useCallback, useEffect, useMemo } from "react";
import { Check, Crown, Hash, Plus, Search } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { getAvailableSkillsApi } from "@/lib/api/skill-api";
import { cn } from "@/lib/utils";
import { ROOM_ICON_ID_END, ROOM_ICON_ID_START } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/avatar";
import {
  UiDialogBackdrop,
  UiDialogCloseButton,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  getDialogActionClassName,
} from "@/shared/ui/dialog/dialog-styles";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import { UiMultiSelectMenu, UiSelectMenu } from "@/shared/ui/select-menu";
import type { SkillInfo } from "@/types/capability/skill";

interface RoomMemberAgentOption {
  agent_id: string;
  name: string;
  avatar?: string | null;
  status?: string;
  headline?: string | null;
  description?: string | null;
}

interface CreateRoomDialogProps {
  agents: RoomMemberAgentOption[];
  isOpen: boolean;
  isCreating?: boolean;
  mode?: "create" | "manage";
  dialogTitle?: string;
  dialogSubtitle?: string;
  confirmLabel?: string;
  initialName?: string;
  initialAvatar?: string;
  initialSelectedAgentIds?: string[];
  initialRoomSkillNames?: string[];
  initialHostAgentId?: string | null;
  initialHostAutoReplyEnabled?: boolean;
  initialPrivateMessagesEnabled?: boolean;
  onCancel: () => void;
  onConfirm: (
    agentIds: string[],
    name: string,
    avatar?: string,
    skillNames?: string[],
    hostAgentId?: string | null,
    hostAutoReplyEnabled?: boolean,
    privateMessagesEnabled?: boolean,
  ) => void;
}

interface RoomSkillsState {
  error: string | null;
  items: SkillInfo[];
  loading: boolean;
}

const MAX_MEMBERS = 10;
const EMPTY_STRING_LIST: string[] = [];
const STRING_LIST_SIGNATURE_SEPARATOR = "\x1f";
export function CreateRoomDialog({
  agents,
  isOpen: isOpen,
  isCreating: isCreating = false,
  mode = "create",
  dialogTitle: dialogTitle,
  dialogSubtitle: dialogSubtitle,
  confirmLabel: confirmLabel,
  initialName: initialName = "",
  initialAvatar: initialAvatar = "",
  initialSelectedAgentIds: initialSelectedAgentIds,
  initialRoomSkillNames: initialRoomSkillNames,
  initialHostAgentId: initialHostAgentId = null,
  initialHostAutoReplyEnabled: initialHostAutoReplyEnabled = false,
  initialPrivateMessagesEnabled: initialPrivateMessagesEnabled = false,
  onCancel: onCancel,
  onConfirm: onConfirm,
}: CreateRoomDialogProps) {
  const { t } = useI18n();
  const [roomSkillsState, setRoomSkillsState] = useResettableState<RoomSkillsState>(
    { error: null, items: [], loading: isOpen },
    isOpen ? "open" : "closed",
  );
  const {
    error: roomSkillError,
    items: availableRoomSkills,
    loading: isLoadingRoomSkills,
  } = roomSkillsState;
  const normalizedInitialSelectedIds = initialSelectedAgentIds ?? EMPTY_STRING_LIST;
  const normalizedInitialRoomSkillNames = initialRoomSkillNames ?? EMPTY_STRING_LIST;
  // 数组 props 往往每次 render 都是新引用，依赖内容签名，
  // 避免弹窗打开时因默认空数组或父层重建数组而反复 setState。
  const initialSelectedIdsSignature = useMemo(
    () => normalizedInitialSelectedIds.join(STRING_LIST_SIGNATURE_SEPARATOR),
    [normalizedInitialSelectedIds],
  );
  const stableInitialSelectedIds = useMemo(
    () =>
      initialSelectedIdsSignature === ""
        ? []
        : initialSelectedIdsSignature.split(STRING_LIST_SIGNATURE_SEPARATOR),
    [initialSelectedIdsSignature],
  );
  const initialRoomSkillNamesSignature = useMemo(
    () => normalizedInitialRoomSkillNames.join(STRING_LIST_SIGNATURE_SEPARATOR),
    [normalizedInitialRoomSkillNames],
  );
  const stableInitialRoomSkillNames = useMemo(
    () =>
      initialRoomSkillNamesSignature === ""
        ? []
        : initialRoomSkillNamesSignature.split(STRING_LIST_SIGNATURE_SEPARATOR),
    [initialRoomSkillNamesSignature],
  );
  const dialogResetKey = [
    isOpen ? "open" : "closed",
    initialName,
    initialAvatar,
    initialSelectedIdsSignature,
    initialRoomSkillNamesSignature,
    initialHostAgentId?.trim() ?? "",
    String(initialHostAutoReplyEnabled),
    String(initialPrivateMessagesEnabled),
  ].join("\x1e");
  const [searchQuery, setSearchQuery] = useResettableState("", dialogResetKey);
  const [selectedIds, setSelectedIds] = useResettableState<string[]>(stableInitialSelectedIds, dialogResetKey);
  const [roomName, setRoomName] = useResettableState(initialName, dialogResetKey);
  const [selectedAvatar, setSelectedAvatar] = useResettableState(initialAvatar, dialogResetKey);
  const [selectedRoomSkillNames, setSelectedRoomSkillNames] = useResettableState<string[]>(
    stableInitialRoomSkillNames,
    dialogResetKey,
  );
  const [roomSkillQuery, setRoomSkillQuery] = useResettableState("", dialogResetKey);
  const [selectedHostAgentId, setSelectedHostAgentId] = useResettableState<string>(
    initialHostAgentId?.trim() ?? "",
    dialogResetKey,
  );
  const [hostAutoReplyEnabled, setHostAutoReplyEnabled] = useResettableState(
    initialHostAutoReplyEnabled,
    dialogResetKey,
  );
  const [privateMessagesEnabled, setPrivateMessagesEnabled] = useResettableState(
    initialPrivateMessagesEnabled,
    dialogResetKey,
  );

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    let isCancelled = false;
    getAvailableSkillsApi({scope: "room"})
      .then((items) => {
        if (!isCancelled) {
          setRoomSkillsState({ error: null, items, loading: false });
        }
      })
      .catch((error: unknown) => {
        if (!isCancelled) {
          setRoomSkillsState({
            error: error instanceof Error ? error.message : t("room.skills_load_error"),
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      isCancelled = true;
    };
  }, [isOpen, setRoomSkillsState, t]);

  // 搜索过滤
  const filteredAgents = useMemo(() => {
    if (!searchQuery.trim()) return agents;
    const q = searchQuery.toLowerCase();
    return agents.filter((a) => a.name.toLowerCase().includes(q));
  }, [agents, searchQuery]);

  // 已选中的 Agent 对象列表
  const selectedIdSet = useMemo(() => new Set(selectedIds), [selectedIds]);
  const selectedAgents = useMemo(
    () => agents.filter((a) => selectedIdSet.has(a.agent_id)),
    [agents, selectedIdSet],
  );

  if (
    selectedIds.length === 0 &&
    (selectedHostAgentId || hostAutoReplyEnabled)
  ) {
    setSelectedHostAgentId("");
    setHostAutoReplyEnabled(false);
  } else if (selectedHostAgentId && !selectedIds.includes(selectedHostAgentId)) {
    setSelectedHostAgentId("");
    setHostAutoReplyEnabled(false);
  }

  const filteredRoomSkills = useMemo(() => {
    const query = roomSkillQuery.trim().toLowerCase();
    if (!query) {
      return availableRoomSkills;
    }
    return availableRoomSkills.filter((skill) =>
      skill.name.toLowerCase().includes(query)
      || skill.title.toLowerCase().includes(query)
      || skill.description.toLowerCase().includes(query),
    );
  }, [availableRoomSkills, roomSkillQuery]);
  const roomSkillOptions = useMemo(
    () => filteredRoomSkills.map((skill) => ({
      value: skill.name,
      label: skill.name,
      description: skill.description || skill.title,
    })),
    [filteredRoomSkills],
  );

  const toggleAgent = useCallback((agentId: string) => {
    setSelectedIds((prev) => {
      if (prev.includes(agentId)) {
        return prev.filter((id) => id !== agentId);
      }
      if (prev.length >= MAX_MEMBERS) return prev;
      return [...prev, agentId];
    });
  }, [setSelectedIds]);

  const handleChangeHostAgent = useCallback((agentId: string) => {
    setSelectedHostAgentId(agentId);
    if (!agentId) {
      setHostAutoReplyEnabled(false);
    }
  }, [setHostAutoReplyEnabled, setSelectedHostAgentId]);

  const handleCreate = useCallback(() => {
    if (selectedIds.length === 0 || !roomName.trim()) return;
    onConfirm(
      selectedIds,
      roomName.trim(),
      selectedAvatar || undefined,
      selectedRoomSkillNames,
      selectedHostAgentId || null,
      hostAutoReplyEnabled && selectedHostAgentId !== "",
      privateMessagesEnabled,
    );
  }, [hostAutoReplyEnabled, onConfirm, privateMessagesEnabled, roomName, selectedAvatar, selectedHostAgentId, selectedIds, selectedRoomSkillNames]);

  if (!isOpen) return null;

  const canCreate = selectedIds.length > 0 && roomName.trim().length > 0 && !isCreating;
  const resolvedDialogTitle = dialogTitle ?? (mode === "manage" ? t("room.manage_dialog_title") : t("room.create_dialog_title"));
  const resolvedDialogSubtitle = dialogSubtitle ?? (mode === "manage" ? t("room.manage_dialog_subtitle") : t("room.create_dialog_subtitle"));
  const resolvedConfirmLabel = confirmLabel ?? (mode === "manage" ? t("common.save") : t("room.create_action"));

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9998]"
        labelledBy="create-room-dialog-title"
        onClose={onCancel}
      >
        <UiDialogShell
          className="max-h-[min(80vh,720px)] pointer-events-auto"
          size="lg"
        >
          <UiDialogHeader>
            <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
              <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-11 w-11 rounded-[16px] text-primary")}>
                <Hash className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <h2
                  className="dialog-title truncate"
                  id="create-room-dialog-title"
                >
                  {resolvedDialogTitle}
                </h2>
                <p className="dialog-subtitle truncate">
                  {resolvedDialogSubtitle}
                </p>
              </div>
            </div>
            <UiDialogCloseButton onClose={onCancel} />
          </UiDialogHeader>

          {/* 内容区：成员管理 + 底部 Room Skill 标签行 */}
          <div className="dialog-body flex min-h-0 flex-col gap-4 overflow-y-auto">
            <div className="flex min-h-0 gap-5">
              {/* 左栏：房间信息 */}
              <div className="flex min-h-0 w-60 shrink-0 flex-col gap-3">
                <p className="dialog-label">
                  {t("room.settings_title")}
                </p>
                <div className="flex flex-col gap-3">
                  <div className="flex items-center gap-3">
                    <UiRoomAvatar
                      avatar={selectedAvatar}
                      className="h-11 w-11 rounded-[14px]"
                      members={[]}
                      roomId={roomName}
                      title={roomName || resolvedDialogTitle}
                    />
                    <input
                      aria-label={t("room.settings_title")}
                      className="dialog-input min-w-0 flex-1 rounded-xl px-3 py-2 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none"
                      data-autofocus="true"
                      maxLength={64}
                      onChange={(e) => setRoomName(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" && canCreate) {
                          handleCreate();
                        }
                      }}
                      placeholder={t("room.name_required_placeholder")}
                      required
                      type="text"
                      value={roomName}
                    />
                  </div>
                  <IconPicker
                    className="mt-3"
                    disabled={isCreating}
                    iconFamily="room"
                    layout="row"
                    iconSize="sm"
                    maxIcons={ROOM_ICON_ID_END - ROOM_ICON_ID_START + 1}
                    onSelect={setSelectedAvatar}
                    showClear={false}
                    startIconId={ROOM_ICON_ID_START}
                    value={selectedAvatar}
                  />
                </div>

                <div className="flex flex-col gap-2">
                  <div className="flex items-center gap-2">
                    <div className="flex shrink-0 items-center gap-1.5 text-[11px] font-semibold text-(--text-muted)">
                      <Crown className="h-3.5 w-3.5 text-primary" />
                      <span>群主</span>
                    </div>
                    <UiSelectMenu
                      ariaLabel="选择 Room 群主"
                      className="min-w-0 flex-1"
                      disabled={selectedAgents.length === 0 || isCreating}
                      onChange={handleChangeHostAgent}
                      options={[
                        { value: "", label: "未设置" },
                        ...selectedAgents.map((agent) => ({
                          value: agent.agent_id,
                          label: agent.name,
                        })),
                      ]}
                      size="sm"
                      surface="dialog"
                      value={selectedHostAgentId}
                    />
                  </div>
                  <label className="mt-1.5 flex items-center gap-2 px-0.5 text-[11px] font-medium text-(--text-default)">
                    <input
                      checked={hostAutoReplyEnabled}
                      className="h-3.5 w-3.5 shrink-0 accent-[var(--primary)] disabled:cursor-not-allowed disabled:opacity-55"
                      disabled={!selectedHostAgentId || isCreating}
                      onChange={(event) => setHostAutoReplyEnabled(event.target.checked)}
                      type="checkbox"
                    />
                    <span className="min-w-0 truncate">
                      未 @ 时由群主接管，可回答或委派
                    </span>
                  </label>
                  <label className="flex items-center gap-2 px-0.5 text-[11px] font-medium text-(--text-default)">
                    <input
                      checked={privateMessagesEnabled}
                      className="h-3.5 w-3.5 shrink-0 accent-[var(--primary)] disabled:cursor-not-allowed disabled:opacity-55"
                      disabled={isCreating}
                      onChange={(event) => setPrivateMessagesEnabled(event.target.checked)}
                      type="checkbox"
                    />
                    <span className="min-w-0 truncate">
                      允许成员私信协作
                    </span>
                  </label>
                </div>

              </div>

              {/* 右栏：Agent 列表 */}
              <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
                {/* 搜索框 */}
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-(--text-soft)" />
                  <input
                    aria-label={t("room.search_agent_placeholder")}
                    className="dialog-input w-full rounded-xl py-2 pl-8 pr-3 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none"
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder={t("room.search_agent_placeholder")}
                    type="text"
                    value={searchQuery}
                  />
                </div>

                <p className="dialog-label">
                  {t("room.all_agents", { count: filteredAgents.length })}
                </p>

                <div className="flex max-h-[min(36vh,360px)] min-h-0 flex-col overflow-hidden rounded-[16px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_84%,transparent)] px-2 py-2">
                  <div
                    className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto pr-1"
                    data-room-member-selection-list="true"
                  >
                    {filteredAgents.map((agent) => {
                      const isSelected = selectedIdSet.has(agent.agent_id);
                      const actionLabel = isSelected
                        ? t("room.agent_select_remove", { name: agent.name })
                        : t("room.agent_select_add", { name: agent.name });
                      return (
                        <button
                          aria-label={actionLabel}
                          aria-pressed={isSelected}
                          key={agent.agent_id}
                          className={cn(
                            "flex w-full cursor-pointer items-center gap-3 rounded-[14px] border px-3 py-1.5 text-left transition-[background,border-color] duration-(--motion-duration-normal)",
                            isSelected
                              ? "border-[color:color-mix(in_srgb,var(--primary)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_13%,transparent)]"
                              : "border-[color:color-mix(in_srgb,var(--divider-subtle-color)_58%,transparent)] bg-transparent hover:border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color))] hover:bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)]",
                          )}
                          onClick={() => toggleAgent(agent.agent_id)}
                          title={actionLabel}
                          type="button"
                        >
                          <UiAgentAvatar avatar={agent.avatar} name={agent.name} size="sm" />

                          <div className="min-w-0 flex-1">
                            <p className="truncate text-sm font-semibold text-(--text-strong)">
                              {agent.name}
                            </p>
                          </div>

                          <div
                            className={cn(
                              "pointer-events-none flex h-6 w-6 shrink-0 items-center justify-center rounded-full transition-all",
                              isSelected
                                ? "bg-primary text-white"
                                : "border border-(--surface-interactive-hover-border) text-(--text-soft)",
                            )}
                          >
                            {isSelected ? (
                              <Check className="h-3 w-3" />
                            ) : (
                              <Plus className="h-3 w-3" />
                            )}
                          </div>
                        </button>
                      );
                    })}
                  </div>
                </div>
              </div>
            </div>

            <div className="shrink-0 space-y-2">
              <p className="dialog-label">
                {t("room.skills_label")}
              </p>
              <UiMultiSelectMenu
                ariaLabel={t("room.skills_label")}
                disabled={isCreating}
                emptyText={t("room.skills_empty")}
                errorText={roomSkillError}
                isLoading={isLoadingRoomSkills}
                loadingText={t("room.skills_loading")}
                onChange={setSelectedRoomSkillNames}
                onQueryChange={setRoomSkillQuery}
                options={roomSkillOptions}
                placement="top"
                placeholder={t("room.skills_none")}
                query={roomSkillQuery}
                searchPlaceholder={t("agent_options.skills.search_placeholder")}
                surface="dialog"
                value={selectedRoomSkillNames}
              />
            </div>
          </div>

          {/* 底部栏 — 与 AgentOptions footer 一致 */}
          <div className="dialog-footer justify-end gap-3">
            {/* 操作按钮 */}
            <button
              className={getDialogActionClassName("default")}
              onClick={onCancel}
              type="button"
            >
              {t("common.cancel")}
            </button>
            <button
              className={getDialogActionClassName(canCreate ? "primary" : "default")}
              disabled={!canCreate}
              onClick={handleCreate}
              type="button"
            >
              {isCreating ? t("room.creating_action") : resolvedConfirmLabel}
            </button>
          </div>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
