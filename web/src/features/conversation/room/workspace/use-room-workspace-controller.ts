/**
 * Room Workspace 控制器
 *
 * 统一管理 workspace 页面中的目录上下文、成员切换、文件操作与错误状态。
 */

import { ChangeEvent, MouseEvent, RefObject, useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  create_workspace_entry_api,
  delete_workspace_entry_api,
  rename_workspace_entry_api,
  upload_workspace_file_api,
} from "@/lib/api/agent-manage-api";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

export interface WorkspaceContextMenuState {
  position: { x: number; y: number } | null;
  entry: WorkspaceFileEntry | null;
}

export type WorkspacePromptState =
  | { mode: "create-file"; default_value: string; parent_path: string | null }
  | { mode: "create-directory"; default_value: string; parent_path: string | null }
  | { mode: "rename"; entry: WorkspaceFileEntry; default_value: string }
  | null;

interface UseRoomWorkspaceControllerOptions {
  active_workspace_path: string | null;
  agent_id: string;
  is_dm: boolean;
  on_open_workspace_file: (path: string | null) => void;
  file_input_ref: RefObject<HTMLInputElement | null>;
}

export function get_parent_directory_path(path: string): string | null {
  const last_slash_index = path.lastIndexOf("/");
  if (last_slash_index === -1) {
    return null;
  }
  return path.slice(0, last_slash_index);
}

export function join_workspace_path(parent_path: string | null, name: string): string {
  return parent_path ? `${parent_path}/${name}` : name;
}

export function get_renamed_active_path(
  active_path: string | null,
  old_path: string,
  new_path: string,
): string | null {
  if (!active_path) {
    return null;
  }
  if (active_path === old_path) {
    return new_path;
  }
  if (active_path.startsWith(`${old_path}/`)) {
    return `${new_path}${active_path.slice(old_path.length)}`;
  }
  return null;
}

export function is_workspace_path_affected(
  active_path: string | null,
  target_path: string,
): boolean {
  if (!active_path) {
    return false;
  }
  return active_path === target_path || active_path.startsWith(`${target_path}/`);
}

export function resolve_workspace_menu_position(
  event: MouseEvent,
  menu_height: number,
): { x: number; y: number } {
  const menu_width = 180;
  return {
    x: Math.min(event.clientX, window.innerWidth - menu_width),
    y: Math.min(event.clientY, window.innerHeight - menu_height),
  };
}

export function useRoomWorkspaceController(
  {
    active_workspace_path,
    agent_id,
    is_dm,
    on_open_workspace_file,
    file_input_ref,
  }: UseRoomWorkspaceControllerOptions) {
  const [selected_agent_id, set_selected_agent_id] = useState(agent_id);
  const [is_uploading, set_is_uploading] = useState(false);
  const [is_loading_files, set_is_loading_files] = useState(false);
  const [error_message, set_error_message] = useState<string | null>(null);
  const [context_menu, set_context_menu] = useState<WorkspaceContextMenuState>({
    position: null,
    entry: null,
  });
  const [prompt_state, set_prompt_state] = useState<WorkspacePromptState>(null);
  const [delete_target, set_delete_target] = useState<WorkspaceFileEntry | null>(null);
  const [focused_directory_path, set_focused_directory_path] = useState<string | null>(null);
  const [upload_target_directory, set_upload_target_directory] = useState<string | null>(null);

  const files_by_agent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const refresh_files = useWorkspaceFilesStore((state) => state.refresh_files);
  const clear_workspace_agent = useWorkspaceFilesStore((state) => state.clear_agent);

  const previous_view_agent_id_ref = useRef<string>(is_dm ? agent_id : selected_agent_id);
  const view_agent_id = is_dm ? agent_id : selected_agent_id;
  const files = useMemo(() => files_by_agent[view_agent_id] || [], [files_by_agent, view_agent_id]);

  useEffect(() => {
    set_selected_agent_id(agent_id);
  }, [agent_id]);

  useEffect(() => {
    const previous_view_agent_id = previous_view_agent_id_ref.current;
    previous_view_agent_id_ref.current = view_agent_id;

    if (previous_view_agent_id !== view_agent_id) {
      on_open_workspace_file(null);
      set_focused_directory_path(null);
    }

    let ignore = false;

    const load_workspace_files = async () => {
      set_is_loading_files(true);
      set_error_message(null);
      try {
        await refresh_files(view_agent_id);
      } catch (error) {
        if (ignore) {
          return;
        }
        clear_workspace_agent(view_agent_id);
        set_error_message(error instanceof Error ? error.message : "加载文件列表失败");
      } finally {
        if (!ignore) {
          set_is_loading_files(false);
        }
      }
    };

    void load_workspace_files();

    return () => {
      ignore = true;
    };
  }, [clear_workspace_agent, on_open_workspace_file, refresh_files, view_agent_id]);

  useEffect(() => {
    set_focused_directory_path(get_parent_directory_path(active_workspace_path ?? ""));
  }, [active_workspace_path, view_agent_id]);

  const handle_click_file = useCallback((path: string) => {
    set_focused_directory_path(get_parent_directory_path(path));
    on_open_workspace_file(path);
  }, [on_open_workspace_file]);

  const handle_click_directory = useCallback((path: string) => {
    set_focused_directory_path(path);
  }, []);

  const handle_upload_click = useCallback((directory_path?: string | null) => {
    set_upload_target_directory(directory_path ?? focused_directory_path);
    file_input_ref.current?.click();
  }, [file_input_ref, focused_directory_path]);

  const handle_file_select = useCallback(async (event: ChangeEvent<HTMLInputElement>) => {
    const selected_files = event.target.files;
    if (!selected_files || selected_files.length === 0) {
      return;
    }

    set_is_uploading(true);
    set_error_message(null);
    try {
      for (const file of Array.from(selected_files)) {
        const target_directory = upload_target_directory ? `${upload_target_directory}/` : undefined;
        await upload_workspace_file_api(view_agent_id, file, target_directory);
      }
      await refresh_files(view_agent_id);
    } catch (error) {
      set_error_message(error instanceof Error ? error.message : "上传文件失败");
    } finally {
      set_is_uploading(false);
      set_upload_target_directory(null);
      if (file_input_ref.current) {
        file_input_ref.current.value = "";
      }
    }
  }, [file_input_ref, refresh_files, upload_target_directory, view_agent_id]);

  const open_create_prompt = useCallback((entry_type: "file" | "directory", parent_path?: string | null) => {
    set_prompt_state(
      entry_type === "file"
        ? {mode: "create-file", default_value: "untitled.txt", parent_path: parent_path ?? focused_directory_path}
        : {mode: "create-directory", default_value: "new-folder", parent_path: parent_path ?? focused_directory_path},
    );
  }, [focused_directory_path]);

  const open_rename_prompt = useCallback((entry: WorkspaceFileEntry) => {
    set_prompt_state({
      mode: "rename",
      entry,
      default_value: entry.name,
    });
  }, []);

  const handle_prompt_confirm = useCallback(async (value: string) => {
    const normalized_name = value.trim();
    if (!prompt_state || !normalized_name) {
      return;
    }

    set_error_message(null);
    try {
      if (prompt_state.mode === "rename") {
        if (normalized_name === prompt_state.entry.name) {
          set_prompt_state(null);
          return;
        }

        const renamed_entry = await rename_workspace_entry_api(
          view_agent_id,
          prompt_state.entry.path,
          join_workspace_path(get_parent_directory_path(prompt_state.entry.path), normalized_name),
        );
        await refresh_files(view_agent_id);

        const next_active_path = get_renamed_active_path(
          active_workspace_path,
          prompt_state.entry.path,
          renamed_entry.new_path,
        );
        if (next_active_path) {
          on_open_workspace_file(next_active_path);
        }
        if (is_workspace_path_affected(focused_directory_path, prompt_state.entry.path)) {
          set_focused_directory_path(
            get_renamed_active_path(focused_directory_path, prompt_state.entry.path, renamed_entry.new_path),
          );
        }
      } else {
        const created_entry = await create_workspace_entry_api(
          view_agent_id,
          join_workspace_path(prompt_state.parent_path, normalized_name),
          prompt_state.mode === "create-file" ? "file" : "directory",
        );
        await refresh_files(view_agent_id);

        if (prompt_state.mode === "create-file") {
          on_open_workspace_file(created_entry.path);
          set_focused_directory_path(get_parent_directory_path(created_entry.path));
        } else {
          set_focused_directory_path(created_entry.path);
        }
      }
      set_prompt_state(null);
    } catch (error) {
      set_error_message(error instanceof Error ? error.message : "工作区操作失败");
    }
  }, [active_workspace_path, focused_directory_path, on_open_workspace_file, prompt_state, refresh_files, view_agent_id]);

  const handle_confirm_delete = useCallback(async () => {
    if (!delete_target) {
      return;
    }

    set_error_message(null);
    try {
      await delete_workspace_entry_api(view_agent_id, delete_target.path);
      await refresh_files(view_agent_id);
      if (is_workspace_path_affected(active_workspace_path, delete_target.path)) {
        on_open_workspace_file(null);
      }
      if (is_workspace_path_affected(focused_directory_path, delete_target.path)) {
        set_focused_directory_path(get_parent_directory_path(delete_target.path));
      }
      set_delete_target(null);
    } catch (error) {
      set_error_message(error instanceof Error ? error.message : "删除失败");
    }
  }, [active_workspace_path, delete_target, focused_directory_path, on_open_workspace_file, refresh_files, view_agent_id]);

  const handle_context_menu = useCallback((event: MouseEvent, entry: WorkspaceFileEntry) => {
    set_context_menu({
      position: resolve_workspace_menu_position(event, entry.is_dir ? 178 : 102),
      entry,
    });
  }, []);

  const handle_root_context_menu = useCallback((event: MouseEvent) => {
    event.preventDefault();
    set_context_menu({
      position: resolve_workspace_menu_position(event, 106),
      entry: null,
    });
  }, []);

  const close_context_menu = useCallback(() => {
    set_context_menu({position: null, entry: null});
  }, []);

  return {
    view_agent_id,
    files,
    selected_agent_id,
    set_selected_agent_id,
    is_uploading,
    is_loading_files,
    error_message,
    clear_error_message: () => set_error_message(null),
    context_menu,
    prompt_state,
    delete_target,
    focused_directory_path,
    current_directory_label: focused_directory_path ?? "/",
    handle_click_file,
    handle_click_directory,
    handle_upload_click,
    handle_file_select,
    open_create_prompt,
    open_rename_prompt,
    handle_prompt_confirm,
    handle_confirm_delete,
    handle_context_menu,
    handle_root_context_menu,
    close_context_menu,
    set_delete_target,
    set_prompt_state,
  };
}
