package runtime

import "time"

// RoundIdleAbortTimeout 是 round 空闲后中断/断连的宽限：manager 会话空闲回收与
// exec 轮次执行共用，故定义在 runtime 核心并导出给 exec 引用。
const RoundIdleAbortTimeout = 5 * time.Second
