/**
 * INPUT: Home 全局目录快照与刷新命令。
 * OUTPUT: 聊天/联系人侧栏共用的窄目录接口。
 * POS: Home 目录资源到侧栏控制器之间的只读投影边界。
 */
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";
import {
  reconcileHomeDirectory,
  refreshHomeDirectory,
  useHomeDirectory,
} from "../home-directory-resource";

export interface SidebarDirectoryState {
  agents: LauncherAgentSummary[];
  conversations: LauncherConversationSummary[];
  hasError: boolean;
  hasLoaded: boolean;
  isLoading: boolean;
  reconcileRoomTarget: (roomId: string) => Promise<boolean>;
  refreshDirectory: () => void;
  rooms: LauncherRoomSummary[];
}

export function useSidebarDirectory(): SidebarDirectoryState {
  const directory = useHomeDirectory();

  return {
    ...directory,
    reconcileRoomTarget: async (roomId: string) => {
      const refreshed = await reconcileHomeDirectory();
      return refreshed.rooms.some((room) => room.id === roomId);
    },
    refreshDirectory: refreshHomeDirectory,
  };
}
