import { useEffect, useMemo, useState } from "react";
import { ArrowRight, Sparkles, Waypoints } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getRoomContexts, listRooms } from "@/lib/room-api";
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

export function RoomsPage() {
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

  const latest_room = useMemo(() => (
    sort_rooms_desc(rooms).find((room) => room.room.room_type !== "dm") ?? null
  ), [rooms]);

  useEffect(() => {
    if (is_loading || !latest_room) {
      return;
    }

    let is_cancelled = false;
    const target_room = latest_room;

    async function open_latest_room() {
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

    void open_latest_room();

    return () => {
      is_cancelled = true;
    };
  }, [is_loading, latest_room, navigate]);

  if (is_loading) {
    return <AppLoadingScreen />;
  }

  return (
    <AppStage active_rail_item="rooms">
      <div className="relative flex min-h-0 flex-1 items-center justify-center px-4 py-6 sm:px-6">
        <section className="workspace-card max-w-xl rounded-[32px] px-6 py-6 text-center sm:px-8">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[20px] bg-white/12 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.14)]">
            <Waypoints className="h-6 w-6 text-slate-900/78" />
          </div>
          <h1 className="mt-5 text-[28px] font-black tracking-[-0.05em] text-slate-950/90">
            Rooms
          </h1>
          <p className="mt-3 text-sm leading-6 text-slate-700/60">
            当前还没有可恢复的多人协作。你可以先从首页描述任务，让 Nexus 帮你组织新的协作空间。
          </p>
          <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
            <button
              className="workspace-chip inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold text-slate-900/82"
              onClick={() => navigate(AppRouteBuilders.launcher_app())}
              type="button"
            >
              <Sparkles className="h-4 w-4" />
              打开 Nexus
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
