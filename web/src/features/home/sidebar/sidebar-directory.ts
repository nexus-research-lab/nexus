import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";
import { refreshHomeDirectory, useHomeDirectory } from "../home-directory-resource";

export interface SidebarDirectoryState {
  agents: LauncherAgentSummary[];
  conversations: LauncherConversationSummary[];
  hasError: boolean;
  isLoading: boolean;
  refreshDirectory: () => void;
  rooms: LauncherRoomSummary[];
}

export function useSidebarDirectory(): SidebarDirectoryState {
  const directory = useHomeDirectory();

  return {
    ...directory,
    refreshDirectory: refreshHomeDirectory,
  };
}
