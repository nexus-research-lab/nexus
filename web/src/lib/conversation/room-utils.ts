/**
 * Room 排序与时间戳工具
 *
 * [OUTPUT]: 对外提供 get_room_timestamp, sort_rooms_by_recency
 * [POS]: lib 模块，被 pages 消费
 */

import { RoomAggregate } from "@/types/conversation/room";

/** 获取 Room 最近活动时间戳 */
export function get_room_timestamp(room: RoomAggregate): number {
  return new Date(
    room.room.updated_at ?? room.room.created_at ?? 0,
  ).getTime();
}

/** 按最近活动时间降序排列 */
export function sort_rooms_by_recency(rooms: RoomAggregate[]): RoomAggregate[] {
  return [...rooms].sort((a, b) => get_room_timestamp(b) - get_room_timestamp(a));
}
