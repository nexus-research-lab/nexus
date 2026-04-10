# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_control_registry.py
# @Date   ：2026/04/10 14:43
# @Author ：leemysw
# 2026/04/10 14:43   Create
# =====================================================

"""会话绑定与控制权注册表。"""

from __future__ import annotations

from typing import Any, Dict, Set

from agent.service.channels.message_sender import MessageSender


class SessionControlRegistry:
    """维护 session 的绑定集合与单控制端。"""

    def __init__(self) -> None:
        self._session_bindings: Dict[str, Dict[int, Dict[str, Any]]] = {}
        self._sender_sessions: Dict[int, Set[str]] = {}
        self._sender_by_id: Dict[int, MessageSender] = {}
        self._controller_sender_ids: Dict[str, int] = {}
        self._bind_sequence = 0

    def bind_session(
        self,
        session_key: str,
        sender: MessageSender,
        client_id: str,
        request_control: bool,
    ) -> dict[str, Any]:
        """绑定 sender 到 session，并按需要抢占控制权。"""
        if not session_key or self._is_sender_closed(sender):
            return self.get_session_snapshot(session_key)

        sender_id = id(sender)
        bindings = self._session_bindings.setdefault(session_key, {})
        self._sender_by_id[sender_id] = sender
        self._sender_sessions.setdefault(sender_id, set()).add(session_key)
        self._bind_sequence += 1
        bindings[sender_id] = {
            "client_id": client_id or f"sender:{sender_id}",
            "bind_order": self._bind_sequence,
        }
        self._prune_closed_bindings(session_key)

        controller_sender_id = self._controller_sender_ids.get(session_key)
        if (
            request_control
            or controller_sender_id is None
            or controller_sender_id not in bindings
        ):
            self._controller_sender_ids[session_key] = sender_id

        return self.get_session_snapshot(session_key)

    def unbind_session(
        self,
        session_key: str,
        sender: MessageSender,
    ) -> dict[str, Any]:
        """解绑某个 sender 对指定 session 的绑定。"""
        if not session_key:
            return self.get_session_snapshot(session_key)

        self._remove_binding(session_key, id(sender))
        self._prune_closed_bindings(session_key)
        return self.get_session_snapshot(session_key)

    def unregister_sender(self, sender: MessageSender) -> tuple[str, ...]:
        """移除某个 sender 持有的全部 session 绑定。"""
        sender_id = id(sender)
        session_keys = list(self._sender_sessions.pop(sender_id, set()))
        changed_session_keys: list[str] = []
        for session_key in session_keys:
            self._remove_binding(session_key, sender_id)
            self._prune_closed_bindings(session_key)
            changed_session_keys.append(session_key)
        self._sender_by_id.pop(sender_id, None)
        return tuple(changed_session_keys)

    def has_bindings(self, session_key: str) -> bool:
        """判断 session 是否存在任意活跃绑定。"""
        self._prune_closed_bindings(session_key)
        return bool(self._session_bindings.get(session_key))

    def is_bound(self, session_key: str, sender: MessageSender) -> bool:
        """判断某个 sender 是否已经绑定到 session。"""
        self._prune_closed_bindings(session_key)
        return id(sender) in self._session_bindings.get(session_key, {})

    def is_session_controller(self, session_key: str, sender: MessageSender) -> bool:
        """判断某个 sender 是否是 session 当前控制端。"""
        controller = self.resolve_controller_sender(session_key)
        return controller is sender

    def resolve_controller_sender(self, session_key: str) -> MessageSender | None:
        """解析当前 session 的控制端 sender。"""
        self._prune_closed_bindings(session_key)
        sender_id = self._controller_sender_ids.get(session_key)
        if sender_id is None:
            return None
        return self._sender_by_id.get(sender_id)

    def resolve_session_senders(self, session_key: str) -> tuple[MessageSender, ...]:
        """解析当前 session 的全部绑定 sender。"""
        self._prune_closed_bindings(session_key)
        bindings = self._session_bindings.get(session_key, {})
        return tuple(
            sender
            for sender_id in bindings
            if (sender := self._sender_by_id.get(sender_id)) is not None
        )

    def get_session_snapshot(self, session_key: str) -> dict[str, Any]:
        """返回当前 session 的控制权快照。"""
        self._prune_closed_bindings(session_key)
        bindings = self._session_bindings.get(session_key, {})
        controller_sender_id = self._controller_sender_ids.get(session_key)
        controller_binding = (
            bindings.get(controller_sender_id)
            if controller_sender_id is not None
            else None
        )
        bound_client_count = len(bindings)
        return {
            "controller_client_id": (
                controller_binding.get("client_id")
                if controller_binding is not None
                else None
            ),
            "observer_count": max(bound_client_count - 1, 0),
            "bound_client_count": bound_client_count,
        }

    def _prune_closed_bindings(self, session_key: str) -> None:
        """清理已关闭 sender，并保证 session 始终只有一个控制端。"""
        bindings = self._session_bindings.get(session_key)
        if not bindings:
            self._session_bindings.pop(session_key, None)
            self._controller_sender_ids.pop(session_key, None)
            return

        closed_sender_ids = [
            sender_id
            for sender_id in list(bindings.keys())
            if self._is_sender_closed(self._sender_by_id.get(sender_id))
        ]
        for sender_id in closed_sender_ids:
            self._remove_binding(session_key, sender_id)

        bindings = self._session_bindings.get(session_key)
        if not bindings:
            self._session_bindings.pop(session_key, None)
            self._controller_sender_ids.pop(session_key, None)
            return

        controller_sender_id = self._controller_sender_ids.get(session_key)
        if controller_sender_id not in bindings:
            self._promote_controller(session_key)

    def _remove_binding(self, session_key: str, sender_id: int) -> None:
        """移除一条 session 绑定，并同步 sender 反向索引。"""
        bindings = self._session_bindings.get(session_key)
        if bindings is None:
            return

        bindings.pop(sender_id, None)
        if not bindings:
            self._session_bindings.pop(session_key, None)

        sender_sessions = self._sender_sessions.get(sender_id)
        if sender_sessions is not None:
            sender_sessions.discard(session_key)
            if not sender_sessions:
                self._sender_sessions.pop(sender_id, None)
                self._sender_by_id.pop(sender_id, None)

        if self._controller_sender_ids.get(session_key) == sender_id:
            self._controller_sender_ids.pop(session_key, None)

    def _promote_controller(self, session_key: str) -> None:
        """从剩余绑定里晋升最新绑定的连接为控制端。"""
        bindings = self._session_bindings.get(session_key)
        if not bindings:
            self._controller_sender_ids.pop(session_key, None)
            return

        promoted_sender_id = max(
            bindings.items(),
            key=lambda item: int(item[1].get("bind_order", 0)),
        )[0]
        self._controller_sender_ids[session_key] = promoted_sender_id

    @staticmethod
    def _is_sender_closed(sender: MessageSender | None) -> bool:
        """判断 sender 是否已关闭。"""
        if sender is None:
            return True
        return bool(getattr(sender, "is_closed", False))
