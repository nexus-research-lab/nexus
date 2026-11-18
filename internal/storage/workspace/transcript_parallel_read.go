// INPUT: 已打开的 transcript 文件与其快照长度。
// OUTPUT: 与顺序解析逐条一致（含行号）的 transcript 条目；大文件按换行边界分块并行解析。
// POS: 冷启动或改写后完整解析长 transcript 的 CPU 并行入口。
package workspace

import (
	"bytes"
	"context"
	"io"
	"math"
	"os"
	"runtime"
	"sync"
)

const (
	// 小文件的分块与调度开销超过并行收益，仍顺序解析。
	transcriptParallelMinBytes   = 2 << 20
	transcriptParallelChunkBytes = 1 << 20
	transcriptParallelMaxWorkers = 8
)

// readTranscriptEntriesParallelContext 把 [0,size) 切成以换行结尾的分块并行解析，
// 最后一块读到 EOF，与顺序读取看到的字节完全相同。每块按本地行号解析，再加上前面
// 各块的行数，得到与顺序解析相同的 Index。
func readTranscriptEntriesParallelContext(ctx context.Context, file *os.File, size int64) ([]transcriptEntry, int, error) {
	workers := min(runtime.GOMAXPROCS(0), transcriptParallelMaxWorkers, int(size/transcriptParallelChunkBytes))
	if size < transcriptParallelMinBytes || workers < 2 {
		return readTranscriptEntriesReaderContext(ctx, file, 0)
	}
	return readTranscriptEntriesChunksContext(ctx, file, size, workers)
}

func readTranscriptEntriesChunksContext(ctx context.Context, file *os.File, size int64, workers int) ([]transcriptEntry, int, error) {
	bounds, err := transcriptChunkBounds(file, size, workers)
	if err != nil {
		return nil, 0, err
	}
	type chunkResult struct {
		entries []transcriptEntry
		lines   int
		err     error
	}
	results := make([]chunkResult, len(bounds)-1)
	var wait sync.WaitGroup
	for chunk := range results {
		wait.Add(1)
		go func() {
			defer wait.Done()
			start, end := bounds[chunk], bounds[chunk+1]
			if chunk == len(results)-1 {
				end = math.MaxInt64
			}
			entries, lines, err := readTranscriptEntriesReaderContext(ctx, io.NewSectionReader(file, start, end-start), 0)
			results[chunk] = chunkResult{entries: entries, lines: lines, err: err}
		}()
	}
	wait.Wait()
	total := 0
	for _, result := range results {
		if result.err != nil {
			return nil, 0, result.err
		}
		total += len(result.entries)
	}
	entries := make([]transcriptEntry, 0, total)
	offset := 0
	for _, result := range results {
		for _, entry := range result.entries {
			entry.Index += offset
			entries = append(entries, entry)
		}
		offset += result.lines
	}
	return entries, offset, ctx.Err()
}

// transcriptChunkBounds 返回分块起点：每个内部边界都紧跟在一个换行之后，
// 因而没有行被切开；找不到换行的区段并入后一块。
func transcriptChunkBounds(file *os.File, size int64, workers int) ([]int64, error) {
	bounds := []int64{0}
	buffer := make([]byte, 64<<10)
	for worker := 1; worker < workers; worker++ {
		position := max(size*int64(worker)/int64(workers), bounds[len(bounds)-1])
		boundary := int64(-1)
		for position < size {
			n, err := file.ReadAt(buffer[:min(int64(len(buffer)), size-position)], position)
			if index := bytes.IndexByte(buffer[:n], '\n'); index >= 0 {
				boundary = position + int64(index) + 1
				break
			}
			if err != nil && err != io.EOF {
				return nil, err
			}
			if n == 0 {
				break
			}
			position += int64(n)
		}
		if boundary < 0 || boundary >= size {
			break
		}
		if boundary > bounds[len(bounds)-1] {
			bounds = append(bounds, boundary)
		}
	}
	return append(bounds, size), nil
}
