// Package ledger は controller の台帳を file だけで保つ。
//
// 構成は snapshot + journal の 2 段。書き込みは journal への追記が先で、
// snapshot は checkpoint のときだけ atomic rename で置き換える。復元は
// 「snapshot を読み、その seq より後の journal を再生する」だけで済む。
//
// 落ちるのは常に追記の途中なので、journal の末尾 1 行が切れている状態を
// 正常系として扱う。各行が自分の長さと checksum を持つので、途中で切れた行は
// 検出して捨てられる。捨てた分は controller が再実行すればよい（外部操作は
// intent を先に永続化する前提）。
//
// 書き手は 1 プロセスだけ。flock で二重起動を弾く。
package ledger

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

// SchemaVersion は snapshot と journal の両方に入る。読めない版は開かない。
const SchemaVersion = 1

const (
	snapshotName = "snapshot.json"
	journalName  = "journal.jsonl"
	lockName     = "ledger.lock"
	dirMode      = 0o700
	fileMode     = 0o600
)

var (
	// ErrLocked は別のプロセスが既に台帳を持っているとき返る。
	ErrLocked = errors.New("ledger: another writer owns this ledger")
	// ErrSchema は schema version が合わないとき返る。黙って移行しない。
	ErrSchema = errors.New("ledger: unsupported schema version")
)

// Record は journal の 1 行。Data は任意の JSON をそのまま持つ。
type Record struct {
	Seq  uint64          `json:"seq"`
	At   time.Time       `json:"at"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data,omitempty"`
	Sum  string          `json:"sum"`
}

// Snapshot は checkpoint 時点の台帳の中身。
type Snapshot struct {
	SchemaVersion int             `json:"schema_version"`
	Seq           uint64          `json:"seq"`
	At            time.Time       `json:"at"`
	State         json.RawMessage `json:"state"`
}

// Ledger は 1 ディレクトリ ＝ 1 台帳。
type Ledger struct {
	dir  string
	lock *os.File
	mu   sync.Mutex

	journal *os.File
	seq     uint64
	snap    Snapshot

	// Recovered は Open 時に切り捨てた壊れた末尾行の数。0 でないことを
	// 障害の記録として残せるよう外から読めるようにしてある。
	recovered int
}

// Open は台帳を開き、必要なら壊れた journal 末尾を切り捨てて復元する。
func Open(dir string) (*Ledger, error) {
	if dir == "" || !filepath.IsAbs(dir) || filepath.Clean(dir) == "/" {
		return nil, errors.New("ledger: dir must be an absolute non-root directory")
	}
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("ledger: dir must be a real directory")
	}

	lock, err := os.OpenFile(filepath.Join(dir, lockName), os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("%w: %v", ErrLocked, err)
	}

	l := &Ledger{dir: dir, lock: lock}
	if err := l.loadSnapshot(); err != nil {
		l.Close()
		return nil, err
	}
	if err := l.recoverJournal(); err != nil {
		l.Close()
		return nil, err
	}
	journal, err := os.OpenFile(l.path(journalName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, fileMode)
	if err != nil {
		l.Close()
		return nil, err
	}
	l.journal = journal
	return l, nil
}

// Close は lock を返し、journal を閉じる。
func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	var firstErr error
	if l.journal != nil {
		if err := l.journal.Close(); err != nil {
			firstErr = err
		}
		l.journal = nil
	}
	if l.lock != nil {
		if err := syscall.Flock(int(l.lock.Fd()), syscall.LOCK_UN); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := l.lock.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		l.lock = nil
	}
	return firstErr
}

// Seq は最後に確定した記録の番号。
func (l *Ledger) Seq() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.seq
}

// Recovered は Open 時に切り捨てた壊れた末尾行の数。
func (l *Ledger) Recovered() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.recovered
}

// Append は 1 件を journal へ追記して fsync する。返るのは確定した seq。
func (l *Ledger) Append(kind string, data any) (uint64, error) {
	if kind == "" {
		return 0, errors.New("ledger: kind is required")
	}
	raw, err := marshalData(data)
	if err != nil {
		return 0, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.journal == nil {
		return 0, errors.New("ledger: closed")
	}

	rec := Record{Seq: l.seq + 1, At: time.Now().UTC(), Kind: kind, Data: raw}
	rec.Sum = checksum(rec)
	line, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}
	line = append(line, '\n')
	if _, err := l.journal.Write(line); err != nil {
		return 0, err
	}
	// fsync してから seq を進める。ここで落ちたら次の Open が末尾を切り捨てる。
	if err := l.journal.Sync(); err != nil {
		return 0, err
	}
	l.seq = rec.Seq
	return rec.Seq, nil
}

// Replay は snapshot の state と、その後に追記された記録を順に返す。
func (l *Ledger) Replay() (json.RawMessage, []Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	recs, _, err := readJournal(l.path(journalName), l.snap.Seq)
	if err != nil {
		return nil, nil, err
	}
	return l.snap.State, recs, nil
}

// Checkpoint は今の state を snapshot として atomic に書き、journal を空にする。
// snapshot の入れ替えが先、journal の切り詰めが後。逆にすると、間で落ちたときに
// 記録が消える。
func (l *Ledger) Checkpoint(state any) error {
	raw, err := marshalData(state)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.journal == nil {
		return errors.New("ledger: closed")
	}

	snap := Snapshot{SchemaVersion: SchemaVersion, Seq: l.seq, At: time.Now().UTC(), State: raw}
	if err := writeJSONAtomic(l.path(snapshotName), snap); err != nil {
		return err
	}
	if err := l.journal.Truncate(0); err != nil {
		return err
	}
	if _, err := l.journal.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := l.journal.Sync(); err != nil {
		return err
	}
	l.snap = snap
	return nil
}

// Backup は snapshot と journal を dst へ複製する。台帳は開いたまま使える。
func (l *Ledger) Backup(dst string) error {
	if dst == "" || !filepath.IsAbs(dst) {
		return errors.New("ledger: backup dir must be absolute")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.journal != nil {
		if err := l.journal.Sync(); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dst, dirMode); err != nil {
		return err
	}
	// backup は自己完結させる。checkpoint 前でも snapshot を必ず 1 つ書き、
	// journal は無ければ空で作る。restore 側が「片方だけ古い」状態を作らないため。
	if err := writeJSONAtomic(filepath.Join(dst, snapshotName), l.snap); err != nil {
		return err
	}
	if err := copyFileOrEmpty(l.path(journalName), filepath.Join(dst, journalName)); err != nil {
		return err
	}
	return syncDir(dst)
}

// Restore は backup の中身で台帳を置き換える。呼ぶ側が Close してから使う。
// 途中で落ちても元の台帳が壊れないよう、両方を temp 経由の rename で入れ替える。
func Restore(src, dir string) error {
	if !filepath.IsAbs(src) || !filepath.IsAbs(dir) {
		return errors.New("ledger: restore paths must be absolute")
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	var snap Snapshot
	if err := readJSON(filepath.Join(src, snapshotName), &snap); err != nil {
		return err
	}
	if snap.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: backup has %d", ErrSchema, snap.SchemaVersion)
	}
	// 両方を必ず置き換える。片方だけ残すと snapshot と journal がずれる。
	for _, name := range []string{snapshotName, journalName} {
		from := filepath.Join(src, name)
		tmp := filepath.Join(dir, "."+name+".restore")
		if err := copyFileOrEmpty(from, tmp); err != nil {
			return err
		}
		if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	return syncDir(dir)
}

func (l *Ledger) path(name string) string { return filepath.Join(l.dir, name) }

func (l *Ledger) loadSnapshot() error {
	var snap Snapshot
	err := readJSON(l.path(snapshotName), &snap)
	if errors.Is(err, os.ErrNotExist) {
		l.snap = Snapshot{SchemaVersion: SchemaVersion}
		return nil
	}
	if err != nil {
		return err
	}
	if snap.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: snapshot has %d", ErrSchema, snap.SchemaVersion)
	}
	l.snap = snap
	l.seq = snap.Seq
	return nil
}

// recoverJournal は journal を頭から検査し、最初に壊れた行より後ろを切り捨てる。
func (l *Ledger) recoverJournal() error {
	path := l.path(journalName)
	recs, goodBytes, err := readJournal(path, l.snap.Seq)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if goodBytes < info.Size() {
		if err := os.Truncate(path, goodBytes); err != nil {
			return err
		}
		l.recovered++
	}
	if n := len(recs); n > 0 {
		l.seq = recs[n-1].Seq
	} else {
		l.seq = l.snap.Seq
	}
	return nil
}

// readJournal は after より後の記録を順に返す。2 つ目の戻り値は、最後に
// 健全だった位置（バイト数）。壊れた行以降は読まない。
func readJournal(path string, after uint64) ([]Record, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var (
		recs  []Record
		good  int64
		prev  = after
		sc    = bufio.NewScanner(f)
		limit = 16 << 20 // 1 行の上限。巨大な行で落ちないように。
	)
	sc.Buffer(make([]byte, 0, 64<<10), limit)
	for sc.Scan() {
		line := sc.Bytes()
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			break // 途中で切れた行。ここから先は捨てる。
		}
		if rec.Sum != checksum(rec) {
			break
		}
		if rec.Seq != prev+1 {
			break // 欠番や巻き戻りは信用しない。
		}
		prev = rec.Seq
		recs = append(recs, rec)
		good += int64(len(line)) + 1
	}
	if err := sc.Err(); err != nil && !errors.Is(err, bufio.ErrTooLong) {
		return nil, 0, err
	}
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].Seq < recs[j].Seq })
	return recs, good, nil
}

// checksum は Sum を除いた内容から取る。行が途中で切れれば必ず一致しない。
func checksum(rec Record) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\n%s\n%s\n", rec.Seq, rec.At.UTC().Format(time.RFC3339Nano), rec.Kind)
	h.Write(rec.Data)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func marshalData(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	if raw, ok := v.(json.RawMessage); ok {
		return raw, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func readJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func writeJSONAtomic(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(fileMode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncDir(dir)
}

// copyFileOrEmpty は src が無ければ空ファイルを作る。台帳は checkpoint 前でも
// backup / restore できる必要があるため。
func copyFileOrEmpty(src, dst string) error {
	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
		if err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
