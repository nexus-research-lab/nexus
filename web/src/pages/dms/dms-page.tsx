import { useEffect, useMemo, useState } from "react";
import { ArrowRight, MessageCircleMore, Users } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { listRooms, getRoomContexts } from "@/lib/room-api";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { AppStage } from "@/shared/ui/app-stage";
import { RoomAggregate } from "@/types/room";

function sort_rooms_desc(rooms: RoomAggregate[]) {
  return [...rooms].sort((left, right) => {
    const left_timestamp = new Date(left.room.updated_at ?? left.room.created_at ?? 0).getTime();
    const right_timestamp = new Date(right.room.updated_at ?? right.room.created_at ?? 0).getTime();
    return right_timestamp - left_timestamp;
  });
}

export function DmsPage() {
  const navigate = useNavigate();
  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [is_loading, set_is_loading] = useState(true);

  useEffect(() => {
    let is_cancelled = false;

    async function bootstrap() {
      try {
        const next_rooms = await listRooms(200);
        if (is_cancelled) {
          return;
        }
        set_rooms(next_rooms);
      } finally {
        if (!is_cancelled) {
          set_is_loading(false);
        }
      }
    }

    void bootstrap();

    return () => {
      is_cancelled = true;
    };
  }, []);

  const latest_dm_room = useMemo(() => (
    sort_rooms_desc(rooms).find((room) => room.room.room_type === "dm") ?? null
  ), [rooms]);

  useEffect(() => {
    if (is_loading || !latest_dm_room) {
      return;
    }

    let is_cancelled = false;
    const target_room = latest_dm_room;

    async function open_latest_dm() {
      const contexts = await getRoomContexts(target_room.room.id);
      if (is_cancelled) {
        return;
      }

      if (contexts[0]?.conversation?.id) {
        navigate(
          AppRouteBuilders.room_conversation(
            target_room.room.id,
            contexts[0].conversation.id,
          ),
          { replace: true },
        );
        return;
      }

      navigate(AppRouteBuilders.room(target_room.room.id), { replace: true });
    }

    void open_latest_dm();

    return () => {
      is_cancelled = true;
    };
  }, [is_loading, latest_dm_room, navigate]);

  if (is_loading) {
    return <AppLoadingScreen />;
  }

  return (
    <AppStage active_rail_item="dms">
      <div className="relative flex min-h-0 flex-1 items-center justify-center px-4 py-6 sm:px-6">
        <section className="workspace-card max-w-xl rounded-[32px] px-6 py-6 text-center sm:px-8">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[20px] bg-white/12 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.14)]">
            <MessageCircleMore className="h-6 w-6 text-slate-900/78" />
          </div>
          <h1 className="mt-5 text-[28px] font-black tracking-[-0.05em] text-slate-950/90">
            Direct Messages
          </h1>
          <p className="mt-3 text-sm leading-6 text-slate-700/60">
            当前还没有可恢复的 1v1 协作。先去成员网络选择成员，或者从首页唤起新的系统协作。
          </p>
          <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
            <button
              className="workspace-chip inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold text-slate-900/82"
              onClick={() => navigate(AppRouteBuilders.contacts())}
              type="button"
            >
              <Users className="h-4 w-4" />
              成员网络
            </button>
            <button
              className="workspace-chip inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold text-slate-900/82"
              onClick={() => navigate(AppRouteBuilders.launcher())}
              type="button"
            >
              回到首页
              <ArrowRight className="h-4 w-4" />
            </button>
          </div>
        </section>
      </div>
    </AppStage>
  );
}
