package protocol

import (
	"slices"
	"sync"
	"testing"
)

// 并发受理也必须各自拿到不重复的游标，且每个受理方看到的值严格递增。
func TestNewInputSeqIsUniqueAndStrictlyIncreasing(t *testing.T) {
	const workers, perWorker = 8, 2000
	minted := make([][]int64, workers)
	var group sync.WaitGroup
	for worker := range workers {
		group.Go(func() {
			values := make([]int64, perWorker)
			for index := range values {
				values[index] = NewInputSeq()
			}
			minted[worker] = values
		})
	}
	group.Wait()

	all := make([]int64, 0, workers*perWorker)
	for worker, values := range minted {
		if !slices.IsSorted(values) {
			t.Fatalf("worker %d 拿到的游标没有递增", worker)
		}
		all = append(all, values...)
	}
	slices.Sort(all)
	if unique := slices.Compact(slices.Clone(all)); len(unique) != len(all) {
		t.Fatalf("并发铸造出现重复游标: %d 个值里只有 %d 个不同", len(all), len(unique))
	}
	const maxSafeInteger = 1<<53 - 1
	if all[0] <= 0 || all[len(all)-1] > maxSafeInteger {
		t.Fatalf("游标必须是正的 JS 安全整数: min=%d max=%d", all[0], all[len(all)-1])
	}
}

func TestChatAckCarriesInputSeqOnlyForCommittedRows(t *testing.T) {
	committed := NewChatAckEvent("session", "request", "client", "round", "msg_user_1", 42, true, nil)
	if committed.Data[InputSeqField] != int64(42) {
		t.Fatalf("已落盘输入的 ACK 必须回传受理游标: %+v", committed.Data)
	}
	queued := NewChatAckEvent("session", "request", "client", "round", "msg_user_1", 0, false, nil)
	if _, exists := queued.Data[InputSeqField]; exists {
		t.Fatalf("尚未成为历史行的输入不应带游标: %+v", queued.Data)
	}
}
