// INPUT: 含引号、控制字符、NUL、多字节与非法 UTF-8 的身份键，以及分两次并入同一 scope 的键集合。
// OUTPUT: 每个键都能按原值查到，前缀相同的键互不混淆，重复并入不出错，替换后旧键消失。
// POS: 尾部身份键批量写入的回归：写入走 JSON 编码，必须与按参数绑定的查询逐字节一致。
package workspace

import (
	"context"
	"testing"
)

func TestHistoryReadTailKeysRoundTripVerbatim(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sharedHistoryReadModel(root).database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeHistoryReadModelForRestart(root) })

	const scope = "scope"
	stored := []string{"plain", `quote"back\slash`, "tab\tline\n", "a\x00b", "é漢字😀", "\u2028", "\xff\xfe", "a\xffb"}
	absent := []string{"a", "a\x00", "a\x00c", "\ufffd\ufffd", "a\ufffdb", "PLAIN"}
	write := func(replace bool, kind int, values ...string) {
		t.Helper()
		keys := make(historyTailKeys)
		for _, value := range values {
			keys.add(kind, value)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if replace {
			err = replaceHistoryReadTailKeys(ctx, tx, scope, keys)
		} else {
			err = insertHistoryReadTailKeys(ctx, tx, scope, keys)
		}
		if err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	contains := func(kind int, value string) bool {
		t.Helper()
		keys := make(historyTailKeys)
		keys.add(kind, value)
		found, err := historyReadTailKeysContain(ctx, db, scope, keys)
		if err != nil {
			t.Fatal(err)
		}
		return found
	}

	write(true, historyTailKeyMessage, stored[:4]...)
	write(false, historyTailKeyMessage, stored[2:]...)
	for _, value := range stored {
		if !contains(historyTailKeyMessage, value) {
			t.Errorf("key %q was stored but is not found", value)
		}
		if contains(historyTailKeyGroup, value) {
			t.Errorf("key %q leaked into another kind", value)
		}
	}
	for _, value := range absent {
		if contains(historyTailKeyMessage, value) {
			t.Errorf("key %q was never stored but is found", value)
		}
	}

	write(true, historyTailKeyGroup, "next")
	if contains(historyTailKeyMessage, "plain") || !contains(historyTailKeyGroup, "next") {
		t.Fatal("replacing the scope's keys must drop the previous set")
	}
}
