// INPUT: 进程重启后的长 DM 会话、小会话与不适合预热的会话。
// OUTPUT: 预热建立投影检查点，之后追加一轮的刷新沿用同一解析代系，结果与未预热时一致。
// POS: 轮次开始时后台预热历史的回归。
package workspace

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func transcriptCacheLineageForTest(t *testing.T, f *longSessionFixture) (uint64, bool) {
	t.Helper()
	path, _, ok := f.history.transcriptPrewarmTarget(context.Background(), f.workspacePath, *f.session.SessionID)
	if !ok {
		t.Fatal("transcript target missing")
	}
	f.history.cache.mu.RLock()
	defer f.history.cache.mu.RUnlock()
	entry, exists := f.history.cache.entries[path]
	return entry.Lineage, exists && entry.Projection != nil
}

func TestPrewarmTranscriptHistoryMakesFirstRefreshIncremental(t *testing.T) {
	f := newLongSessionFixture(t, 160, 8*1024)
	f.readFirstPage(t)
	simulateHistoryProcessRestart(t, f)

	if !f.history.prewarmTranscriptHistory(context.Background(), f.workspacePath, f.session) {
		t.Fatal("prewarm did not build a projection checkpoint")
	}
	lineage, cached := transcriptCacheLineageForTest(t, f)
	if !cached || lineage == 0 {
		t.Fatal("prewarm left no projection in the parse cache")
	}
	// 缓存已热时再次预热不会重新解析。
	if !f.history.prewarmTranscriptHistory(context.Background(), f.workspacePath, f.session) {
		t.Fatal("second prewarm reported no checkpoint")
	}
	if again, _ := transcriptCacheLineageForTest(t, f); again != lineage {
		t.Fatal("prewarm re-parsed a warm transcript")
	}

	f.appendRound(t)
	got := f.readFirstPage(t)
	if after, _ := transcriptCacheLineageForTest(t, f); after != lineage {
		t.Fatalf("refresh after prewarm re-parsed the transcript: lineage %d -> %d", lineage, after)
	}

	// 与重启后丢弃读模型、不预热完整重建的结果一致。
	simulateHistoryProcessRestart(t, f)
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	if err := access.ReadModel.deleteScope(context.Background(), access.Scope); err != nil {
		t.Fatal(err)
	}
	if want := f.readFirstPage(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("prewarmed refresh diverged from a cold rebuild\ngot  %v\nwant %v", got, want)
	}
}

func TestPrewarmTranscriptHistoryRunsInBackground(t *testing.T) {
	f := newLongSessionFixture(t, 160, 8*1024)
	simulateHistoryProcessRestart(t, f)
	f.history.ForOwner("").PrewarmTranscriptHistory(f.workspacePath, f.session)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, cached := transcriptCacheLineageForTest(t, f); cached {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background prewarm did not populate the cache")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for {
		f.history.cache.mu.RLock()
		running := len(f.history.cache.prewarming)
		f.history.cache.mu.RUnlock()
		if running == 0 && len(transcriptPrewarmSlots) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("background prewarm did not release its slot")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPrewarmTranscriptHistorySkipsSmallAndIneligibleSessions(t *testing.T) {
	f := newLongSessionFixture(t, 4, 64)
	if f.history.prewarmTranscriptHistory(context.Background(), f.workspacePath, f.session) {
		t.Fatal("small transcript was prewarmed")
	}
	if _, cached := transcriptCacheLineageForTest(t, f); cached {
		t.Fatal("small transcript entered the cache")
	}
	empty := ""
	for name, session := range map[string]protocol.Session{
		"no session id": {SessionKey: f.session.SessionKey},
		"empty id":      {SessionKey: f.session.SessionKey, SessionID: &empty},
		"segmented":     {SessionKey: f.session.SessionKey, SessionID: f.session.SessionID, Options: map[string]any{protocol.OptionRuntimeSegmentedTranscript: true}},
	} {
		if transcriptPrewarmEligible(session) {
			t.Fatalf("%s session is eligible for prewarm", name)
		}
	}
	if !transcriptPrewarmEligible(f.session) {
		t.Fatal("plain DM session is not eligible")
	}
}
