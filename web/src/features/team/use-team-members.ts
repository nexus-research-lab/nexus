// INPUT: 当前远程登录身份与在线 Room 可用状态。
// OUTPUT: 排除当前用户的可邀请 Team 真人目录。
// POS: 在线建群表单的人类成员目录资源。
import { useEffect, useState } from "react";

import {
  listControlMemberDirectoryApi,
  type ControlMemberDirectoryEntry,
} from "@/lib/api/account/control-api";
import { useAuth } from "@/shared/auth/auth-context";

export function useTeamMembers(enabled: boolean) {
  const { status } = useAuth();
  const userId = status?.control_user_id ?? status?.user_id;
  const [members, setMembers] = useState<ControlMemberDirectoryEntry[]>([]);

  useEffect(() => {
    if (!enabled) {
      setMembers([]);
      return;
    }
    let active = true;
    void listControlMemberDirectoryApi().then((items) => {
      if (active) setMembers(items.filter((item) => item.user_id !== userId));
    }).catch(() => {
      if (active) setMembers([]);
    });
    return () => { active = false; };
  }, [enabled, userId, status?.organization_id]);

  return members;
}
