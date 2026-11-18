// INPUT: 含空行、坏行、CRLF、超长行、无结尾换行与无 uuid 行的 JSONL。
// OUTPUT: 任意分块数的并行解析与顺序解析的条目、行号和下一行号完全一致。
// POS: transcript 分块并行解析的差分回归。
package workspace

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTranscriptParallelReadMatchesSequential(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	pieces := func() []string {
		lines := []string{}
		for i := 0; i < 200; i++ {
			switch random.Intn(9) {
			case 0:
				lines = append(lines, "")
			case 1:
				lines = append(lines, "   \t")
			case 2:
				lines = append(lines, `{"uuid":`)
			case 3:
				lines = append(lines, fmt.Sprintf(`{"type":"user","message":{"content":"no uuid %d"}}`, i))
			case 4:
				lines = append(lines, fmt.Sprintf(`{"uuid":"crlf-%d","type":"user"}`+"\r", i))
			case 5:
				lines = append(lines, fmt.Sprintf(`{"uuid":"long-%d","type":"assistant","message":{"content":[{"type":"text","text":%q}]}}`, i, strings.Repeat("x", random.Intn(5000))))
			default:
				lines = append(lines, fmt.Sprintf(`  {"uuid":"u-%d","sessionId":"s","type":"user","message":{"content":"q%d"}}  `, i, i))
			}
		}
		return lines
	}
	for _, trailing := range []string{"\n", "", "\n\n"} {
		content := strings.Join(pieces(), "\n") + trailing
		path := filepath.Join(t.TempDir(), "transcript.jsonl")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		want, wantNext, err := readTranscriptEntriesReaderContext(context.Background(), file, 0)
		if err != nil {
			t.Fatal(err)
		}
		size := int64(len(content))
		for _, workers := range []int{2, 3, 7, 16, 64, 1000} {
			got, gotNext, err := readTranscriptEntriesChunksContext(context.Background(), file, size, workers)
			if err != nil {
				t.Fatal(err)
			}
			if gotNext != wantNext || !reflect.DeepEqual(got, want) {
				t.Fatalf("trailing=%q workers=%d: next %d want %d, entries %d want %d", trailing, workers, gotNext, wantNext, len(got), len(want))
			}
		}
		// 快照长度之后追加的字节由最后一块读到，与顺序读取 EOF 的行为一致。
		appended := content + `{"uuid":"late","type":"user"}` + "\n"
		if err = os.WriteFile(path, []byte(appended), 0o600); err != nil {
			t.Fatal(err)
		}
		lateWant, lateNext, _ := readTranscriptEntriesReaderContext(context.Background(), mustSeekStart(t, file), 0)
		lateGot, lateGotNext, err := readTranscriptEntriesChunksContext(context.Background(), file, size, 5)
		if err != nil || lateGotNext != lateNext || !reflect.DeepEqual(lateGot, lateWant) {
			t.Fatalf("trailing=%q: late append diverged: err=%v", trailing, err)
		}
		_ = file.Close()
	}
}

func TestTranscriptParallelReadHonorsContext(t *testing.T) {
	content := strings.Repeat(`{"uuid":"u","type":"user"}`+"\n", 1000)
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = readTranscriptEntriesChunksContext(ctx, file, int64(len(content)), 4); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func mustSeekStart(t *testing.T, file *os.File) *os.File {
	t.Helper()
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	return file
}
