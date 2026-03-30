import type { RoomAggregate } from "@/types/room";

/** 按最近活跃时间降序排列 Room 列表。 */
export function sort_rooms_by_recency(rooms: RoomAggregate[]): RoomAggregate[] {
  return [...rooms].sort((a, b) => {
    const ta = a.room.updated_at ?? a.room.created_at ?? "";
    const tb = b.room.updated_at ?? b.room.created_at ?? "";
    return tb.localeCompare(ta);
  });
}
