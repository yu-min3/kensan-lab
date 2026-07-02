package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// ErrConflict は楽観ロック失敗（他クライアントが先に書き込んだ）を表す。
var ErrConflict = errors.New("conflict: file changed since last read")

// writeMu はプロセス内の書き込みを直列化する。
// kensan backend は単一インスタンス運用が前提（unification-plan.md）。
// Claude Code / VSCode との競合は mtime 楽観ロックで検出する。
var writeMu sync.Mutex

var updatedRe = regexp.MustCompile(`(?m)^updated:\s*\S.*$`)

// Create は新規ファイルを書く。既存なら ErrConflict。
func (w *Workspace) Create(rel string, content []byte) error {
	abs, err := w.Abs(rel)
	if err != nil {
		return err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	if _, err := os.Stat(abs); err == nil {
		return fmt.Errorf("%w: %s already exists", ErrConflict, rel)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, content, 0o644)
}

// Write は既存ファイルを更新する。
// baseMTime が非ゼロの場合、現在の mtime と一致しなければ ErrConflict（楽観ロック）。
func (w *Workspace) Write(rel string, content []byte, baseMTime time.Time) error {
	abs, err := w.Abs(rel)
	if err != nil {
		return err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !baseMTime.IsZero() && !info.ModTime().Equal(baseMTime) {
		return fmt.Errorf("%w: %s", ErrConflict, rel)
	}
	return os.WriteFile(abs, content, 0o644)
}

// Delete はファイルを削除する。
func (w *Workspace) Delete(rel string) error {
	abs, err := w.Abs(rel)
	if err != nil {
		return err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	return os.Remove(abs)
}

// Mutate は read-modify-write を 1 つの書き込みロック内で行う。
// fn が返した新しい内容を書き戻す。fn 実行中に他プロセス（Claude 等）が
// 書き込んだ場合の検出は呼び出し側のセマンティクス（行内容の一致確認等）で行う。
func (w *Workspace) Mutate(rel string, fn func(content []byte, exists bool) ([]byte, error)) error {
	abs, err := w.Abs(rel)
	if err != nil {
		return err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	content, err := os.ReadFile(abs)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	out, err := fn(content, exists)
	if err != nil {
		return err
	}
	if out == nil {
		return nil // 変更なし
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, out, 0o644)
}

// TouchUpdated は frontmatter の updated フィールドを今日の日付に書き換える。
// updated が無い・frontmatter が無い場合はそのまま返す（寛容設計）。
func TouchUpdated(content []byte, today time.Time) []byte {
	src, ok := frontmatterBlock(content)
	if !ok {
		return content
	}
	if !updatedRe.Match(src) {
		return content
	}
	newSrc := updatedRe.ReplaceAll(src, []byte("updated: "+today.Format("2006-01-02")))
	// frontmatterBlock は先頭デリミタ以降の YAML 部分を返すので、位置を合わせて差し替える
	delim := 4 // "---\n"
	if len(content) >= 5 && content[3] == '\r' {
		delim = 5 // "---\r\n"
	}
	out := make([]byte, 0, len(content)+len(newSrc)-len(src))
	out = append(out, content[:delim]...)
	out = append(out, newSrc...)
	out = append(out, content[delim+len(src):]...)
	return out
}
