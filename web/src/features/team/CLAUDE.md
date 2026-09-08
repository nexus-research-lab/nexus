# Team

- `use-team-bootstrap.ts` 只判断当前登录用户是否有 Relay Team，并为聊天目录提供默认 General Room。
- `use-team-room.ts` 负责快照、差量游标、WSS 水位/换代提示和真人消息提交；WSS 不承载消息正文，提交成功后仍从旧游标走 difference 再前进。
- `team-stream-event.ts` 只校验当前 stream/epoch 的水位提示与显式换代事件。
- Relay 是共享消息权威源；浏览器只访问同源 Nexus gateway，不接触 Control 或 Relay token。
