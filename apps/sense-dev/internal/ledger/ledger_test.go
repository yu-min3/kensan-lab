package ledger

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type payload struct {
	Task string `json:"task"`
	N    int    `json:"n"`
}

func open(t *testing.T, dir string) *Ledger {
	t.Helper()
	l, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func appendN(t *testing.T, l *Ledger, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		if _, err := l.Append("task.state", payload{Task: "T011", N: i}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
}

func replayKinds(t *testing.T, l *Ledger) []Record {
	t.Helper()
	_, recs, err := l.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	return recs
}

// 再起動しても同じ台帳が戻ること。これが T011 の本体。
func TestReopenRestoresSameLedger(t *testing.T) {
	dir := t.TempDir()
	l := open(t, dir)
	appendN(t, l, 3)
	if got := l.Seq(); got != 3 {
		t.Fatalf("Seq = %d, want 3", got)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	again := open(t, dir)
	if got := again.Seq(); got != 3 {
		t.Fatalf("reopened Seq = %d, want 3", got)
	}
	recs := replayKinds(t, again)
	if len(recs) != 3 {
		t.Fatalf("replayed %d records, want 3", len(recs))
	}
	var p payload
	if err := json.Unmarshal(recs[2].Data, &p); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if p.N != 3 || p.Task != "T011" {
		t.Fatalf("last record = %+v, want {T011 3}", p)
	}
}

// 追記の途中で落ちた状態。末尾の欠けた行だけが捨てられ、それ以外は残る。
func TestRecoversFromPartialTrailingWrite(t *testing.T) {
	dir := t.TempDir()
	l := open(t, dir)
	appendN(t, l, 3)
	_ = l.Close()

	path := filepath.Join(dir, journalName)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	// 3 行目を途中で切る。
	if err := os.WriteFile(path, b[:len(b)-20], fileMode); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	again := open(t, dir)
	if got := again.Seq(); got != 2 {
		t.Fatalf("Seq after recovery = %d, want 2", got)
	}
	if got := again.Recovered(); got != 1 {
		t.Fatalf("Recovered = %d, want 1", got)
	}
	if got := len(replayKinds(t, again)); got != 2 {
		t.Fatalf("replayed %d records, want 2", got)
	}

	// 切り捨てた後も追記でき、seq が続くこと。
	seq, err := again.Append("task.state", payload{Task: "T011", N: 3})
	if err != nil {
		t.Fatalf("Append after recovery: %v", err)
	}
	if seq != 3 {
		t.Fatalf("seq after recovery = %d, want 3", seq)
	}
}

// 中身だけ書き換えられた行は checksum で落ちる。
func TestRejectsTamperedRecord(t *testing.T) {
	dir := t.TempDir()
	l := open(t, dir)
	appendN(t, l, 2)
	_ = l.Close()

	path := filepath.Join(dir, journalName)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	// 2 行目の kind を別物に差し替える（長さは変えない）。
	tampered := []byte(string(b))
	idx := lastIndexOf(tampered, []byte(`"kind":"task.state"`))
	if idx < 0 {
		t.Fatal("kind not found in journal")
	}
	copy(tampered[idx:], []byte(`"kind":"task.stateX`))
	if err := os.WriteFile(path, tampered, fileMode); err != nil {
		t.Fatalf("write journal: %v", err)
	}

	again := open(t, dir)
	if got := again.Seq(); got != 1 {
		t.Fatalf("Seq = %d, want 1 (tampered record dropped)", got)
	}
}

// checkpoint は snapshot へ畳み、journal を空にする。
func TestCheckpointFoldsJournal(t *testing.T) {
	dir := t.TempDir()
	l := open(t, dir)
	appendN(t, l, 4)
	if err := l.Checkpoint(map[string]any{"done": 4}); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if got := len(replayKinds(t, l)); got != 0 {
		t.Fatalf("journal has %d records after checkpoint, want 0", got)
	}
	if _, err := l.Append("task.state", payload{Task: "T011", N: 5}); err != nil {
		t.Fatalf("Append after checkpoint: %v", err)
	}
	_ = l.Close()

	again := open(t, dir)
	if got := again.Seq(); got != 5 {
		t.Fatalf("Seq = %d, want 5", got)
	}
	state, recs, err := again.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("replayed %d records, want 1", len(recs))
	}
	var st map[string]int
	if err := json.Unmarshal(state, &st); err != nil {
		t.Fatalf("state: %v", err)
	}
	if st["done"] != 4 {
		t.Fatalf("snapshot state = %v, want done=4", st)
	}
}

// 書き手は 1 つだけ。
func TestSingleWriter(t *testing.T) {
	dir := t.TempDir()
	first := open(t, dir)
	_ = first

	second, err := Open(dir)
	if err == nil {
		_ = second.Close()
		t.Fatal("second Open succeeded, want ErrLocked")
	}
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v, want ErrLocked", err)
	}
}

// backup → 進める → restore で、backup 時点へ戻る。
func TestBackupRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	backup := filepath.Join(t.TempDir(), "backup")

	l := open(t, dir)
	appendN(t, l, 2)
	if err := l.Backup(backup); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	appendN(t, l, 3) // backup 後に進める
	if got := l.Seq(); got != 5 {
		t.Fatalf("Seq = %d, want 5", got)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := Restore(backup, dir); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	restored := open(t, dir)
	if got := restored.Seq(); got != 2 {
		t.Fatalf("Seq after restore = %d, want 2", got)
	}
	if got := len(replayKinds(t, restored)); got != 2 {
		t.Fatalf("replayed %d records, want 2", got)
	}
}

// 読めない schema version を黙って受け入れない。
func TestRejectsUnknownSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	l := open(t, dir)
	if err := l.Checkpoint(map[string]any{"n": 1}); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	_ = l.Close()

	path := filepath.Join(dir, snapshotName)
	var snap map[string]any
	b, _ := os.ReadFile(path)
	_ = json.Unmarshal(b, &snap)
	snap["schema_version"] = SchemaVersion + 1
	b, _ = json.Marshal(snap)
	if err := os.WriteFile(path, b, fileMode); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	if _, err := Open(dir); !errors.Is(err, ErrSchema) {
		t.Fatalf("err = %v, want ErrSchema", err)
	}
}

// 空のディレクトリからでも開けて、そこから積める。
func TestOpenFreshDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "ledger")
	l := open(t, dir)
	if got := l.Seq(); got != 0 {
		t.Fatalf("Seq = %d, want 0", got)
	}
	if _, err := l.Append("task.state", payload{Task: "T011", N: 1}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != dirMode {
		t.Fatalf("dir mode = %04o, want %04o", info.Mode().Perm(), dirMode)
	}
}

func lastIndexOf(haystack, needle []byte) int {
	for i := len(haystack) - len(needle); i >= 0; i-- {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return i
		}
	}
	return -1
}
