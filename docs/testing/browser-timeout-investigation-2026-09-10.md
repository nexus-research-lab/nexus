# Browser 连续超时调查与修复设计

状态：原始日志与安装代码已复核；首次故障的精确阻塞点尚未确定，调查未闭环。以下初始修复设计保留为 **non-normative** 调查记录；本次实现的协议6行为以 `docs/specs/browser-spec.md` 为准。历史事故首次阻塞点仍未被回溯证明。

## 原始证据复核与结论边界

- Chrome Profile 1 的 Secure Preferences 指向 `/Applications/Nexus.app/Contents/Resources/Nexus Browser Extension`；当前安装版本为 0.8.4，`background.js` 和 `cursor.js` 与当前仓库逐字节一致。此项证明当前安装副本一致，不单独证明事发后从未更换文件。
- 原始会话 `0e61cee1-7c60-4100-8e0f-9bb86734923f.jsonl` 第20、26、39、44行分别记录导航成功、截图成功、scroll 超时和 evaluate 超时。第50、72、101行记录三次 connected=true。另一份 `d61ec06b-b050-4001-9612-8ac90bae2b6e.jsonl` 复用了相同 tool_use_id，不是第二次独立事故。
- sidecar 日志第582行的模型 tool_use message_stop 时间为02:50:00.903528，首条 scroll 结果为02:51:30.912097，相差约90.009秒；这是接近调用开始的时间标记，不是扩展实际开始执行时间。随后 evaluate 的 message_stop 为02:51:36.498505，结果为02:53:06.501759，相差约90.003秒。错误文本来自宿主等待回执的 deadline 分支。
- `timeout_ms=5000` 只传到扩展 Runtime.evaluate 的 timeout 参数；宿主仍统一使用90秒预算。这个参数不覆盖排队、tabs.get 或 debugger attach，不能根据5秒未返回推定页面脚本执行了90秒。
- **纠正健康判断：status 在 Service.Execute 内直接返回宿主 Status，不向扩展发请求。connected=true 只说明宿主保留连接对象，不证明收到 pong，更不证明执行器正常。** handler 不记录 browser.pong 接收事实；历史日志不能排除连接或扩展消费链故障。
- 在已检查的 Chrome 用户数据根没有发现 chrome_debug.log；当天 DiagnosticReports 中没有找到名称匹配 Chrome/Nexus 的崩溃报告。源码没有命令接收、出队、Chrome API阶段持久日志。这些缺失不能证明浏览器没有异常。
- 从当前源码提取原始 handleMessage，注入永不完成的 scroll 后，独立验证得到 entered=[scroll]、sent=[browser.pong]，list_tabs 未进入 execute。**这证明全局队列缺陷可复现，不证明事发时一定停在该分支。**

目前直接确定的是：首次可见失败为 scroll，宿主连续未获得有效回执，后续14次动作失败并包含不依赖页面脚本的标签查询；收尾也失败。首次命令是否到达扩展、是否出队、是否进入 getLayoutMetrics / 光标补注入 / 输入派发，以及是否存在队列拒绝，均缺少历史直接证据。不能把任一候选写成已确认事故根因。

闭环取证需要同一 command ID 贯穿宿主 send-start/send-end、扩展 receive/queue-start、每个 Chrome API start/end/error、result-send、宿主 result-receive/timeout；同时记录连接代次和单调耗时。扩展阶段记录需独立于动作队列传回宿主持久化。复现时只在新建测试页执行导航、截图、scroll；首条超时后保留现场，按最后一个有开始无结束的阶段定位，不继续堆积重试。仅增加最终超时日志无法填补该缺口。

## 记录证据

2026-09-10，Asia/Shanghai，browser-scout 的同一回合：

- 02:48:56 导航成功，02:49:03 截图成功；Chrome 扩展 0.8.4，协议5。
- 02:51:30 起，scroll、evaluate、reload、navigate、list_tabs、close_tab、close_session、find_tab、cdp、attach_active 共14次工具结果报告宿主 deadline 超时，最后一条03:13:25。
- 02:53:12、02:58:09、03:04:44 的 status 返回 connected=true。
- 03:14:05 sidecar 日志记录 finalize_round 超时（宿主预算15秒）。普通命令预算90秒；evaluate 的 timeout_ms=5000 未使整条工具请求在5秒完成。
- 去重依据 tool_use_id；历史分支 transcript 包含同一批调用，不能重复计数。

来源：本机 .nexus/app/logs/sidecar-2026-09-10.log 和 browser-scout runtime transcript。仓库不复制原始会话、截图或页面内容。

## 确定缺陷与未知边界

1. BrowserClient.handleMessage 用一条全局 Promise 链执行所有命令。一个未完成任务会阻塞其他页面、Session、只读查询和收尾。ping 在队列外响应；宿主 Status 仅检查连接对象，不能证明执行器健康。
2. BrowserController.command 的 debugger attach/sendCommand 没有扩展侧期限。scroll 的 Page.getLayoutMetrics、可见指针补注入和 Input.dispatchMouseEvent 都可能停住；其中 sendCursorMove 有1.5秒保护，但补注入 executeScript 没有。
3. closeSocket/reconcile 没有取消旧队列或使旧命令失效。重连不等于执行恢复。
4. service.sendCommand 超时只删除等待回执，不通知扩展取消。旧队列恢复后仍可能执行早已超时的点击、导航、关闭等动作。
5. finalizeRound 的 hideCursor/debugger detach 等等待同样可能卡住。不能把视觉指针清理作为释放操作的无界前置条件。
6. handler 的写锁等待不响应调用 context；旧连接回执/事件入口缺少显式连接身份校验，修复命令代次时必须一起核对。

7. 发送结果若抛错，catch中的错误回执发送再次抛错，会让queue永久rejected；后续仅.then不会恢复。响应发送失败必须与执行结果分离，不能毒化调度器。

现有记录没有扩展内部 Chrome API 的开始/结束阶段，**不能证明本次首次 scroll 究竟卡在 getLayoutMetrics、脚本补注入还是鼠标派发**。不能把可复现缺陷冒充本次唯一根因。

## 复现与现有测试

- 使用当前源码中的 handleMessage，令首个scroll永不返回，再提交list_tabs和ping：仅scroll进入执行、pong正常返回、list_tabs未开始。复现了“已连接但所有动作超时”的状态。
- 使用当前源码中的scroll/moveCursor，令布局读取成功、指针接收端不存在、补注入executeScript永不返回：Input.dispatchMouseEvent从未发出，scroll不结束。
- 当前 scripts/desktop/browser-extension.test.mjs：9/9通过；它未覆盖永久挂起、截止、取消或重连后旧任务复活，不能证明这些边界正确。

## 建议修复顺序

### 1. 可见指针降级与阶段诊断

给指针发送、补注入、隐藏设置共享的短总预算。视觉反馈失败就降级到原有CDP动作；不能反复获取一份新预算。迟到的注入结果不能继续触发输入。每个命令记录 request ID、action、连接代次、排队/执行阶段、Chrome方法名称与耗时，不记录页面正文、URL参数或脚本内容。

这一步能移除一个确定的无界路径，但不能单独宣称解决整个执行器问题。

### 2. 明确命令生命周期

宿主与扩展同时升级协议。命令携带精确连接代次、ID、剩余预算；宿主取消或超时向原连接发送独立cancel消息，使用独立短context，不能复用已取消context，也不能交给重连后的连接。取消和健康信息不进入动作队列。

插件维护 queued/running/completed/cancelled/unknown；入队、出队、每个等待返回后和下一项副作用前检查。排队取消保证不会开始；已发往Chrome但尚无结果的操作只能标unknown，不能称作未执行、自动重放或伪造成功。batch继承总预算，finalize也遵守同一身份和取消边界。

跨机器时钟不保证一致：不要直接比较两端Date.now。接收后用本地单调计时消耗预算，主动取消补足宿主生命周期；明确承认传输中的取消无法撤销已发出的原生副作用。

### 3. 解除全局阻塞，同时隔离未知动作

使用显式、有容量上限的调度器替代不可取消的Promise链。只读健康/标签目录不被页面动作堵住；同一受控标签/租约的冲突动作保持顺序，其他页面可以继续。Session级创建/关闭与其标签动作也要保持冲突关系，不能简单全部并行。

如果Chrome原生调用未返回，对受影响标签隔离并尝试有界debugger detach；在确认释放或用户明确恢复前，不启动同一标签的新副作用。不能仅Promise.race超时就放行下一条，或仅清空queue引用：旧JavaScript任务仍可能恢复执行。

所有原生调用和后续副作用通过带命令context的适配层执行。catch/finally中的降级、清理也必须遵守取消；等待事件/定时器需注销，不能在旧回调里改写新租约。

### 4. 重连和健康状态

断线即废止旧连接的排队命令和执行权限；新连接不得复用旧队列。回执、事件、租约变更都校验连接代次。将“连接在线”与“执行忙碌/停滞/需恢复”分开；状态读取不能绕过实际故障只报告connected=true。

handler写锁改为可取消等待，获得锁后再核对context；取消发送失败不自动重放原动作。

## 必须通过的回归

- 指针补注入/隐藏永不返回时，视觉路径有界降级；取消后不发鼠标事件。
- 第一条Chrome调用永久挂起：健康读取和另一页面继续，同页冲突动作受阻。
- 第二条命令排队后取消/过期，释放第一条后第二条Chrome调用次数仍为0。
- 运行中超时，Chrome迟到返回后不继续下一项输入/导航/关闭，不回写新代次状态。
- 旧连接断开再连接：旧队列、回执、事件不能进入新连接。
- batch与finalize保留总预算；收尾未知不被记录为清理成功。
- 写锁等待期间context取消：及时退出，解除锁后不补发。
- 成功/失败回执发送均抛错时，调度器仍接受后续独立请求；拖拽按下后取消不能遗留未经处理的输入状态。
- 超时和成功回执竞态有唯一终态，错误文案明确结果未知；无副作用自动重试。

只有上述故障注入回归和目标Go包测试通过，才能把此设计升级为已实现合同。初次调查时未修改插件或服务运行逻辑；后续实现见文末。仍未重载用户扩展或关闭其页面。


## 本次实现与验证范围

宿主与扩展现已实现协议6的剩余总预算、独立取消、连接身份校验、阶段记录、执行健康；扩展改为有容量上限的可推进顺序队列，标签目录读取独立，命令作用域的 Chrome 适配器拒绝迟到延续，光标路径有界降级。发生未知结果时隔离整个关联 Session 和已知标签，尝试有界 detach，用户核对后通过重新加载扩展恢复。当前采用保守的会话隔离，未实现初始设计设想的完整按标签并行调度或自动恢复。

新增回归覆盖挂起后的目录查询、排队取消、跨会话推进、迟到状态写入、断线旧任务、回执发送异常、光标补注入与隐藏挂起、取消后的鼠标输入、拖拽按下后detach以及导航监听清理；Go 回归覆盖原连接取消、迟到回执、旧连接观测拒绝与可取消写锁。测试为故障注入，不冒充真实 Chrome 现场复现。安装目录尚未更新，运行中的用户扩展尚未重载。


验证结果：扩展行为测试20/20通过（含新增11项故障回归）；Browser service/handler 的 Go race 测试及 MCP 包编译通过；三个目标包 go vet、架构依赖检查与 diff 空白检查通过。未执行全量 Go 测试，未用历史日志声称已识别首次原生卡点。


## 0.8.5 安装后的真实复测（10:52—10:55）

会话5996cbe2-3790-4a70-b24d-40c7a50f173b的新增记录（不是其前部复用的旧历史）：10:52:32后台导航成功、10:52:39截图成功；10:53:48 scroll明确返回 `Browser API timed out: Input.dispatchMouseEvent`。后续修改动作立即返回 recovery_required，10:54:21的全局标签目录正常返回；因此全局阻塞已被隔离，但原生滚轮本身仍失败。创建结果明确为 active=false。

0.8.6 在真实键鼠及原始Input.*动作之前使用Page.bringToFront激活目标页，再进行光标与输入派发；激活失败不发输入。Chrome协议仓库曾记录非活动页Input.dispatchMouseEvent无响应（https://github.com/ChromeDevTools/devtools-protocol/issues/89），Page.bringToFront的官方语义为激活标签（https://chromedevtools.github.io/devtools-protocol/1-3/Page/#method-bringToFront）。后台状态是当前有证据支持的触发条件，真实修复效果仍以安装后同链路测试为准。

同时修正阶段日志装配：原先默认slog只进入sidecar stderr，没有进入Nexus的持久化sidecar日志；现由AppServices显式注入现有文件logger。新增回归验证激活先于输入、激活失败不发输入，以及阶段信息写入配置的logger且不泄漏参数。
